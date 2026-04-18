package snapshot_test

import (
	"testing"

	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
	"github.com/karunapuram/pathcollapse/pkg/snapshot"
)

// buildGraphWithEdges returns a graph containing the named edges. Each
// entry is a (src, tgt, type) triple; node IDs are deduplicated.
func buildGraphWithEdges(t *testing.T, edges [][3]string) *graph.Graph {
	t.Helper()
	g := graph.New()
	added := map[string]bool{}
	for _, e := range edges {
		for _, id := range []string{e[0], e[1]} {
			if !added[id] {
				if err := g.AddNode(model.NewNode(id, model.NodeUser, id)); err != nil {
					t.Fatal(err)
				}
				added[id] = true
			}
		}
	}
	for i, e := range edges {
		edge := model.NewEdge(
			t.Name()+"-edge-"+e[0]+"-"+e[1]+"-"+itoa(i),
			model.EdgeType(e[2]),
			e[0], e[1],
		)
		if err := g.AddEdge(edge); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

func itoa(i int) string {
	// Minimal itoa to avoid importing strconv.
	if i == 0 {
		return "0"
	}
	const digits = "0123456789"
	var buf [8]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = digits[i%10]
		i /= 10
	}
	return string(buf[pos:])
}

func TestNewPresence_RejectsNilStore(t *testing.T) {
	if _, err := snapshot.NewPresence(nil, 8); err == nil {
		t.Fatal("expected error on nil store")
	}
}

func TestNewPresence_ColdStartBelowTwoSnapshots(t *testing.T) {
	s := openTempStore(t)
	// Zero snapshots → Window() == 0, EdgePresence() → (0.5, false).
	p, err := snapshot.NewPresence(s, 8)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Window(); got != 0 {
		t.Fatalf("empty store Window(): got %d want 0", got)
	}
	frac, ok := p.EdgePresence("a", "b", model.EdgeMemberOf, 8)
	if ok {
		t.Errorf("expected ok=false for empty store, got frac=%v", frac)
	}
	if frac != 0.5 {
		t.Errorf("expected cold-start 0.5, got %v", frac)
	}

	// One snapshot still below threshold.
	g1 := buildGraphWithEdges(t, [][3]string{{"a", "b", "member_of"}})
	if _, err := s.Save("single", g1); err != nil {
		t.Fatal(err)
	}
	p, err = snapshot.NewPresence(s, 8)
	if err != nil {
		t.Fatal(err)
	}
	frac, ok = p.EdgePresence("a", "b", model.EdgeMemberOf, 8)
	if ok {
		t.Errorf("single snapshot should still be cold start: frac=%v", frac)
	}
}

func TestNewPresence_StablyPresentEdge(t *testing.T) {
	s := openTempStore(t)
	for i := 0; i < 4; i++ {
		g := buildGraphWithEdges(t, [][3]string{
			{"alice", "admins", "member_of"},
			{"admins", "dc", "admin_to"},
		})
		if _, err := s.Save("snap", g); err != nil {
			t.Fatal(err)
		}
	}

	p, err := snapshot.NewPresence(s, 8)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Window(); got != 4 {
		t.Fatalf("Window(): got %d want 4", got)
	}

	frac, ok := p.EdgePresence("alice", "admins", model.EdgeMemberOf, 8)
	if !ok {
		t.Fatal("expected ok=true with 4 snapshots indexed")
	}
	if frac != 1.0 {
		t.Errorf("stably present edge: frac=%v want 1.0", frac)
	}
}

func TestNewPresence_PartialPresence(t *testing.T) {
	s := openTempStore(t)

	// Oldest → newest: edge present, present, missing, present.
	// After reverse-sort by ID, the newest-first order is:
	//   newest = present, present, missing, oldest = present
	// So in the first 4 snapshots the edge appears in 3 → frac = 0.75.
	plans := [][][3]string{
		{{"a", "b", "member_of"}},
		{{"a", "b", "member_of"}},
		{{"x", "y", "member_of"}},
		{{"a", "b", "member_of"}},
	}
	for _, edges := range plans {
		g := buildGraphWithEdges(t, edges)
		if _, err := s.Save("snap", g); err != nil {
			t.Fatal(err)
		}
	}

	p, err := snapshot.NewPresence(s, 8)
	if err != nil {
		t.Fatal(err)
	}
	frac, ok := p.EdgePresence("a", "b", model.EdgeMemberOf, 8)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if frac != 0.75 {
		t.Errorf("partial presence: frac=%v want 0.75", frac)
	}
}

func TestNewPresence_RespectsRequestedWindow(t *testing.T) {
	s := openTempStore(t)

	// Insert order (oldest first): x-y, x-y, x-y, a-b, a-b, a-b.
	// List() returns newest-first, so the 3 newest snapshots contain a-b
	// and the 3 oldest contain x-y.
	plans := [][][3]string{
		{{"x", "y", "member_of"}},
		{{"x", "y", "member_of"}},
		{{"x", "y", "member_of"}},
		{{"a", "b", "member_of"}},
		{{"a", "b", "member_of"}},
		{{"a", "b", "member_of"}},
	}
	for _, edges := range plans {
		g := buildGraphWithEdges(t, edges)
		if _, err := s.Save("snap", g); err != nil {
			t.Fatal(err)
		}
	}

	p, err := snapshot.NewPresence(s, 8)
	if err != nil {
		t.Fatal(err)
	}

	// Full window (6): a-b present in 3 of 6 → 0.5.
	frac, ok := p.EdgePresence("a", "b", model.EdgeMemberOf, 8)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if frac != 0.5 {
		t.Errorf("full window a-b: frac=%v want 0.5", frac)
	}

	// Requested=3 restricts to the 3 newest → a-b present in all → 1.0.
	frac, ok = p.EdgePresence("a", "b", model.EdgeMemberOf, 3)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if frac != 1.0 {
		t.Errorf("narrow window a-b: frac=%v want 1.0", frac)
	}

	// And x-y is absent from the 3 newest → 0.0.
	frac, ok = p.EdgePresence("x", "y", model.EdgeMemberOf, 3)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if frac != 0.0 {
		t.Errorf("narrow window x-y: frac=%v want 0.0", frac)
	}
}

func TestNewPresence_TypeMismatchIsAbsent(t *testing.T) {
	s := openTempStore(t)
	for i := 0; i < 3; i++ {
		g := buildGraphWithEdges(t, [][3]string{{"a", "b", "member_of"}})
		if _, err := s.Save("snap", g); err != nil {
			t.Fatal(err)
		}
	}
	p, err := snapshot.NewPresence(s, 8)
	if err != nil {
		t.Fatal(err)
	}
	frac, ok := p.EdgePresence("a", "b", model.EdgeAdminTo, 8)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if frac != 0.0 {
		t.Errorf("type mismatch: frac=%v want 0.0", frac)
	}
}

// Sanity: Presence satisfies the interface confidence expects, without
// importing the confidence package here (structural typing).
func TestPresence_SatisfiesSnapshotProviderShape(t *testing.T) {
	var iface interface {
		EdgePresence(source, target string, etype model.EdgeType, window int) (float64, bool)
	} = (*snapshot.Presence)(nil)
	_ = iface
}
