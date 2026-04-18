package snapshot

import (
	"fmt"

	"github.com/karunapuram/pathcollapse/pkg/model"
)

// Presence answers "in what fraction of the N most recent snapshots did this
// edge exist?" for arbitrary (source, target, type) triples. It satisfies
// confidence.SnapshotProvider without importing that package (structural).
//
// Build once per CLI invocation via NewPresence and share across all
// recommendation scoring — building cost is one JSON unmarshal per snapshot
// in the window; query cost is a single map lookup per edge.
//
// Presence is safe for concurrent readers after construction. It is not
// safe for concurrent construction.
type Presence struct {
	// edgeSets[i] holds the edge triples present in the i-th most recent
	// snapshot (i = 0 is newest).
	edgeSets []map[presenceKey]struct{}
}

type presenceKey struct {
	Source string
	Target string
	Type   model.EdgeType
}

// NewPresence loads up to `window` most recent snapshots from s and builds
// an in-memory edge-presence index.
//
// A window ≤ 0 is treated as "no history" — the returned Presence will
// report ok=false for every query, matching the cold-start contract in
// docs/confidence.md §4.4.
//
// Corrupt / unreadable snapshots are skipped with no error so a single bad
// row doesn't disable the whole signal; the snapshot's contribution simply
// drops out of the denominator.
func NewPresence(s *Store, window int) (*Presence, error) {
	if s == nil {
		return nil, fmt.Errorf("snapshot: NewPresence requires a non-nil Store")
	}
	p := &Presence{}
	if window <= 0 {
		return p, nil
	}

	snaps, err := s.List()
	if err != nil {
		return nil, fmt.Errorf("snapshot: list for presence: %w", err)
	}
	if len(snaps) > window {
		snaps = snaps[:window]
	}

	p.edgeSets = make([]map[presenceKey]struct{}, 0, len(snaps))
	for _, meta := range snaps {
		g, _, err := s.Load(meta.ID)
		if err != nil {
			continue // skip corrupt row, documented above
		}
		set := make(map[presenceKey]struct{}, g.EdgeCount())
		for _, e := range g.Edges() {
			set[presenceKey{Source: e.Source, Target: e.Target, Type: e.Type}] = struct{}{}
		}
		p.edgeSets = append(p.edgeSets, set)
	}

	return p, nil
}

// Window returns the actual number of snapshots indexed. May be less than
// the requested window if fewer snapshots exist.
func (p *Presence) Window() int {
	if p == nil {
		return 0
	}
	return len(p.edgeSets)
}

// EdgePresence returns the fraction of the min(requested, indexed) most
// recent snapshots in which an edge matching (source, target, etype) appears.
//
// Returns (0.5, false) when fewer than 2 snapshots are indexed — this is the
// cold-start contract: the caller's temporal factor T(e) should fall through
// to a non-informative prior rather than trust a one-snapshot data point.
func (p *Presence) EdgePresence(source, target string, etype model.EdgeType, requested int) (float64, bool) {
	if p == nil || len(p.edgeSets) < 2 {
		return 0.5, false
	}
	n := len(p.edgeSets)
	if requested > 0 && requested < n {
		n = requested
	}
	if n < 2 {
		return 0.5, false
	}

	key := presenceKey{Source: source, Target: target, Type: etype}
	hits := 0
	for i := 0; i < n; i++ {
		if _, ok := p.edgeSets[i][key]; ok {
			hits++
		}
	}
	return float64(hits) / float64(n), true
}
