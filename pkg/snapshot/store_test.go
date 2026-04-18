package snapshot_test

import (
	"os"
	"testing"
	"time"

	"github.com/karthikarunapuram8-dot/pathcollapse/internal/testdata"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/snapshot"
)

func openTempStore(t *testing.T) *snapshot.Store {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "snapshots-*.db")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	f.Close()

	s, err := snapshot.Open(f.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSaveAndList(t *testing.T) {
	s := openTempStore(t)
	g := testdata.EnterpriseAD()

	id, err := s.Save("baseline", g)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	snaps, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].Name != "baseline" {
		t.Errorf("name: got %q, want %q", snaps[0].Name, "baseline")
	}
	if snaps[0].NodeCount != g.NodeCount() {
		t.Errorf("node_count: got %d, want %d", snaps[0].NodeCount, g.NodeCount())
	}
	if snaps[0].EdgeCount != g.EdgeCount() {
		t.Errorf("edge_count: got %d, want %d", snaps[0].EdgeCount, g.EdgeCount())
	}
}

func TestLoadByID(t *testing.T) {
	s := openTempStore(t)
	g := testdata.EnterpriseAD()

	id, err := s.Save("test", g)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, snap, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snap.Name != "test" {
		t.Errorf("name: got %q, want %q", snap.Name, "test")
	}
	if loaded.NodeCount() != g.NodeCount() {
		t.Errorf("node_count: got %d, want %d", loaded.NodeCount(), g.NodeCount())
	}
	if loaded.EdgeCount() != g.EdgeCount() {
		t.Errorf("edge_count: got %d, want %d", loaded.EdgeCount(), g.EdgeCount())
	}
}

func TestLoadByName(t *testing.T) {
	s := openTempStore(t)
	g := testdata.EnterpriseAD()

	if _, err := s.Save("named", g); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, snap, err := s.LoadByName("named")
	if err != nil {
		t.Fatalf("LoadByName: %v", err)
	}
	if snap.Name != "named" {
		t.Errorf("name: got %q, want %q", snap.Name, "named")
	}
	if loaded.NodeCount() != g.NodeCount() {
		t.Errorf("node_count mismatch")
	}
}

func TestLoadNotFound(t *testing.T) {
	s := openTempStore(t)
	_, _, err := s.Load(9999)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestDiff(t *testing.T) {
	s := openTempStore(t)
	g1 := testdata.EnterpriseAD()
	g2 := testdata.EnterpriseAD()

	id1, err := s.Save("snap-1", g1)
	if err != nil {
		t.Fatalf("Save snap-1: %v", err)
	}
	id2, err := s.Save("snap-2", g2)
	if err != nil {
		t.Fatalf("Save snap-2: %v", err)
	}

	rep, oldSnap, newSnap, err := s.Diff(id1, id2)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if oldSnap.ID != id1 {
		t.Errorf("old snap id: got %d, want %d", oldSnap.ID, id1)
	}
	if newSnap.ID != id2 {
		t.Errorf("new snap id: got %d, want %d", newSnap.ID, id2)
	}
	// Identical graphs should report no items
	if len(rep.Items) != 0 {
		t.Errorf("expected 0 drift items for identical snapshots, got %d", len(rep.Items))
	}
}

func TestPrune(t *testing.T) {
	s := openTempStore(t)
	g := testdata.EnterpriseAD()

	// Save 5 snapshots.
	for i := 0; i < 5; i++ {
		if _, err := s.Save("snap", g); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	// Negative maxAge pushes the cutoff into the future, so all snapshots
	// qualify for deletion; keepMin=2 preserves the two most recent.
	deleted, err := s.Prune(-time.Second, 2)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 3 {
		t.Errorf("expected 3 deleted, got %d", deleted)
	}

	snaps, _ := s.List()
	if len(snaps) != 2 {
		t.Errorf("expected 2 remaining, got %d", len(snaps))
	}
}

func TestPruneByAge(t *testing.T) {
	s := openTempStore(t)
	g := testdata.EnterpriseAD()

	for i := 0; i < 3; i++ {
		if _, err := s.Save("old", g); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	// With a large maxAge nothing should be pruned (all snapshots are recent).
	deleted, err := s.Prune(24*time.Hour, 0)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted for recent snapshots, got %d", deleted)
	}
}

func TestMultipleSavesSameName(t *testing.T) {
	s := openTempStore(t)
	g := testdata.EnterpriseAD()

	for i := 0; i < 3; i++ {
		if _, err := s.Save("weekly", g); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	// LoadByName should return the latest.
	_, snap, err := s.LoadByName("weekly")
	if err != nil {
		t.Fatalf("LoadByName: %v", err)
	}
	// Should have the highest ID (most recent).
	snaps, _ := s.List()
	if snap.ID != snaps[0].ID {
		t.Errorf("LoadByName returned id %d, want latest id %d", snap.ID, snaps[0].ID)
	}
}
