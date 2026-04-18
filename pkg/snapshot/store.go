// Package snapshot persists graph snapshots to a local SQLite database and
// provides diff operations via the drift engine.
package snapshot

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/drift"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
)

const defaultDBDir = ".pathcollapse"
const dbName = "snapshots.db"

// Snapshot is a named, timestamped graph snapshot stored in the database.
type Snapshot struct {
	ID        int64
	Name      string
	SavedAt   time.Time
	NodeCount int
	EdgeCount int
}

// Store is a SQLite-backed snapshot store.
type Store struct {
	db *sql.DB
}

// graphBlob is used to serialise/deserialise graph state.
type graphBlob struct {
	Nodes []model.Node `json:"nodes"`
	Edges []model.Edge `json:"edges"`
}

// DefaultDBPath returns ~/.pathcollapse/snapshots.db.
func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("snapshot: locate home dir: %w", err)
	}
	return filepath.Join(home, defaultDBDir, dbName), nil
}

// Open opens (or creates) the snapshot database at dbPath.
func Open(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("snapshot: create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("snapshot: open db: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS snapshots (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		name      TEXT NOT NULL,
		saved_at  TEXT NOT NULL,
		blob      BLOB NOT NULL,
		node_count INTEGER NOT NULL DEFAULT 0,
		edge_count INTEGER NOT NULL DEFAULT 0
	)`)
	return err
}

// Save serialises g and stores it under the given name.
func (s *Store) Save(name string, g *graph.Graph) (int64, error) {
	blob, err := marshalGraph(g)
	if err != nil {
		return 0, err
	}
	res, err := s.db.Exec(
		`INSERT INTO snapshots (name, saved_at, blob, node_count, edge_count) VALUES (?, ?, ?, ?, ?)`,
		name,
		time.Now().UTC().Format(time.RFC3339),
		blob,
		g.NodeCount(),
		g.EdgeCount(),
	)
	if err != nil {
		return 0, fmt.Errorf("snapshot: save: %w", err)
	}
	return res.LastInsertId()
}

// List returns metadata for all stored snapshots, newest first.
func (s *Store) List() ([]Snapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, name, saved_at, node_count, edge_count FROM snapshots ORDER BY id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("snapshot: list: %w", err)
	}
	defer rows.Close()

	var snaps []Snapshot
	for rows.Next() {
		var snap Snapshot
		var savedAt string
		if err := rows.Scan(&snap.ID, &snap.Name, &savedAt, &snap.NodeCount, &snap.EdgeCount); err != nil {
			return nil, fmt.Errorf("snapshot: scan row: %w", err)
		}
		snap.SavedAt, _ = time.Parse(time.RFC3339, savedAt)
		snaps = append(snaps, snap)
	}
	return snaps, rows.Err()
}

// Load retrieves and deserialises the graph for the snapshot with the given ID.
func (s *Store) Load(id int64) (*graph.Graph, *Snapshot, error) {
	var snap Snapshot
	var blob []byte
	var savedAt string
	err := s.db.QueryRow(
		`SELECT id, name, saved_at, blob, node_count, edge_count FROM snapshots WHERE id = ?`, id,
	).Scan(&snap.ID, &snap.Name, &savedAt, &blob, &snap.NodeCount, &snap.EdgeCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("snapshot: id %d not found", id)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot: load id %d: %w", id, err)
	}
	snap.SavedAt, _ = time.Parse(time.RFC3339, savedAt)

	g, err := unmarshalGraph(blob)
	if err != nil {
		return nil, nil, err
	}
	return g, &snap, nil
}

// LoadByName retrieves the most recent snapshot with the given name.
func (s *Store) LoadByName(name string) (*graph.Graph, *Snapshot, error) {
	var snap Snapshot
	var blob []byte
	var savedAt string
	err := s.db.QueryRow(
		`SELECT id, name, saved_at, blob, node_count, edge_count FROM snapshots WHERE name = ? ORDER BY id DESC LIMIT 1`, name,
	).Scan(&snap.ID, &snap.Name, &savedAt, &blob, &snap.NodeCount, &snap.EdgeCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("snapshot: name %q not found", name)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot: load name %q: %w", name, err)
	}
	snap.SavedAt, _ = time.Parse(time.RFC3339, savedAt)

	g, err := unmarshalGraph(blob)
	if err != nil {
		return nil, nil, err
	}
	return g, &snap, nil
}

// Diff compares two stored snapshots by ID and returns a drift report.
func (s *Store) Diff(oldID, newID int64) (*drift.DriftReport, *Snapshot, *Snapshot, error) {
	oldG, oldSnap, err := s.Load(oldID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("snapshot diff: old snapshot: %w", err)
	}
	newG, newSnap, err := s.Load(newID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("snapshot diff: new snapshot: %w", err)
	}
	rep := drift.CompareSnapshots(oldG, newG, oldSnap.SavedAt, newSnap.SavedAt)
	return rep, oldSnap, newSnap, nil
}

// Prune deletes snapshots older than maxAge, keeping at least keepMin entries.
// Returns the number of rows deleted.
func (s *Store) Prune(maxAge time.Duration, keepMin int) (int64, error) {
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339)
	res, err := s.db.Exec(`
		DELETE FROM snapshots
		WHERE saved_at < ?
		  AND id NOT IN (SELECT id FROM snapshots ORDER BY id DESC LIMIT ?)
	`, cutoff, keepMin)
	if err != nil {
		return 0, fmt.Errorf("snapshot: prune: %w", err)
	}
	return res.RowsAffected()
}

// marshalGraph serialises a graph to JSON bytes.
func marshalGraph(g *graph.Graph) ([]byte, error) {
	nodes := g.Nodes()
	edges := g.Edges()

	// Dereference pointers for clean serialisation.
	nodeVals := make([]model.Node, len(nodes))
	for i, n := range nodes {
		nodeVals[i] = *n
	}
	edgeVals := make([]model.Edge, len(edges))
	for i, e := range edges {
		edgeVals[i] = *e
	}

	b, err := json.Marshal(graphBlob{Nodes: nodeVals, Edges: edgeVals})
	if err != nil {
		return nil, fmt.Errorf("snapshot: marshal graph: %w", err)
	}
	return b, nil
}

// unmarshalGraph deserialises a graph from JSON bytes.
func unmarshalGraph(data []byte) (*graph.Graph, error) {
	var blob graphBlob
	if err := json.Unmarshal(data, &blob); err != nil {
		return nil, fmt.Errorf("snapshot: unmarshal graph: %w", err)
	}
	g := graph.New()
	for i := range blob.Nodes {
		if err := g.AddNode(&blob.Nodes[i]); err != nil {
			return nil, fmt.Errorf("snapshot: restore node %q: %w", blob.Nodes[i].ID, err)
		}
	}
	for i := range blob.Edges {
		_ = g.AddEdge(&blob.Edges[i]) // skip edges with missing endpoints
	}
	return g, nil
}
