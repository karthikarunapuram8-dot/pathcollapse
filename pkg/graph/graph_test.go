package graph

import (
	"errors"
	"fmt"
	"testing"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
)

// buildLinearGraph creates: u0 -[e01]-> u1 -[e12]-> u2
func buildLinearGraph(t *testing.T) *Graph {
	t.Helper()
	g := New()
	for i := 0; i < 3; i++ {
		n := model.NewNode(nodeID(i), model.NodeUser, nodeID(i))
		if err := g.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}
	addEdge(t, g, "e01", model.EdgeMemberOf, "u0", "u1")
	addEdge(t, g, "e12", model.EdgeMemberOf, "u1", "u2")
	return g
}

func nodeID(i int) string {
	return []string{"u0", "u1", "u2", "u3", "u4"}[i]
}

func addEdge(t *testing.T, g *Graph, id string, typ model.EdgeType, src, tgt string) {
	t.Helper()
	e := model.NewEdge(id, typ, src, tgt)
	if err := g.AddEdge(e); err != nil {
		t.Fatalf("AddEdge %s: %v", id, err)
	}
}

func TestAddGetNode(t *testing.T) {
	g := New()
	n := model.NewNode("n1", model.NodeUser, "Alice")
	if err := g.AddNode(n); err != nil {
		t.Fatal(err)
	}
	got := g.GetNode("n1")
	if got == nil || got.Name != "Alice" {
		t.Fatalf("expected Alice, got %v", got)
	}
}

func TestAddEdgeMissingNode(t *testing.T) {
	g := New()
	g.AddNode(model.NewNode("a", model.NodeUser, "A"))
	e := model.NewEdge("e1", model.EdgeMemberOf, "a", "missing")
	if err := g.AddEdge(e); err == nil {
		t.Fatal("expected error for missing target node")
	}
}

func TestSentinelErrors(t *testing.T) {
	g := New()

	if err := g.AddNode(nil); !errors.Is(err, ErrNilNode) {
		t.Fatalf("nil node: expected ErrNilNode, got %v", err)
	}
	if err := g.AddNode(model.NewNode("", model.NodeUser, "x")); !errors.Is(err, ErrEmptyNodeID) {
		t.Fatalf("empty node ID: expected ErrEmptyNodeID, got %v", err)
	}
	if err := g.AddEdge(nil); !errors.Is(err, ErrNilEdge) {
		t.Fatalf("nil edge: expected ErrNilEdge, got %v", err)
	}
	if err := g.AddEdge(model.NewEdge("", model.EdgeMemberOf, "x", "y")); !errors.Is(err, ErrEmptyEdgeID) {
		t.Fatalf("empty edge ID: expected ErrEmptyEdgeID, got %v", err)
	}

	g.AddNode(model.NewNode("src", model.NodeUser, "src"))
	missingTarget := model.NewEdge("e1", model.EdgeMemberOf, "src", "no-such-target")
	if err := g.AddEdge(missingTarget); !errors.Is(err, ErrTargetMissing) {
		t.Fatalf("missing target: expected ErrTargetMissing, got %v", err)
	}

	missingSource := model.NewEdge("e2", model.EdgeMemberOf, "no-such-source", "src")
	if err := g.AddEdge(missingSource); !errors.Is(err, ErrSourceMissing) {
		t.Fatalf("missing source: expected ErrSourceMissing, got %v", err)
	}
}

func TestRemoveNode(t *testing.T) {
	g := buildLinearGraph(t)
	g.RemoveNode("u1")
	if g.GetNode("u1") != nil {
		t.Fatal("node should be removed")
	}
	// Edges touching u1 should be gone.
	if g.GetEdge("e01") != nil || g.GetEdge("e12") != nil {
		t.Fatal("incident edges should be removed")
	}
}

func TestNeighbors(t *testing.T) {
	g := buildLinearGraph(t)
	fwd := g.Neighbors("u1")
	if len(fwd) != 1 || fwd[0].ID != "e12" {
		t.Fatalf("expected 1 forward edge from u1, got %v", fwd)
	}
	rev := g.ReverseNeighbors("u1")
	if len(rev) != 1 || rev[0].ID != "e01" {
		t.Fatalf("expected 1 reverse edge to u1, got %v", rev)
	}
}

func TestFindPaths(t *testing.T) {
	g := buildLinearGraph(t)
	paths := g.FindPaths("u0", "u2", DefaultPathOptions())
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0].Len() != 2 {
		t.Fatalf("expected 2-edge path, got %d", paths[0].Len())
	}
}

func TestFindPathsNone(t *testing.T) {
	g := buildLinearGraph(t)
	paths := g.FindPaths("u2", "u0", DefaultPathOptions())
	if len(paths) != 0 {
		t.Fatalf("expected no paths (reverse direction), got %d", len(paths))
	}
}

func TestFindPathsMaxDepth(t *testing.T) {
	g := buildLinearGraph(t)
	opts := DefaultPathOptions()
	opts.MaxDepth = 1
	paths := g.FindPaths("u0", "u2", opts)
	if len(paths) != 0 {
		t.Fatalf("expected no paths with maxDepth=1, got %d", len(paths))
	}
}

func TestConnectedComponents(t *testing.T) {
	g := buildLinearGraph(t)
	// Add an isolated node.
	g.AddNode(model.NewNode("isolated", model.NodeComputer, "ISO"))
	comps := g.ConnectedComponents()
	if len(comps) != 2 {
		t.Fatalf("expected 2 components, got %d", len(comps))
	}
}

func TestPrivilegeConcentration(t *testing.T) {
	g := New()
	for _, id := range []string{"u0", "u1", "dc"} {
		g.AddNode(model.NewNode(id, model.NodeUser, id))
	}
	// Both users are admin_to dc.
	addEdge(t, g, "e1", model.EdgeAdminTo, "u0", "dc")
	addEdge(t, g, "e2", model.EdgeAdminTo, "u1", "dc")
	conc := g.PrivilegeConcentration()
	if len(conc) == 0 || conc[0].Node.ID != "dc" {
		t.Fatalf("expected dc as most concentrated node, got %v", conc)
	}
	if conc[0].InboundPrivileged != 2 {
		t.Fatalf("expected 2 inbound privileged edges, got %d", conc[0].InboundPrivileged)
	}
}

func TestFilteredTraversal(t *testing.T) {
	g := buildLinearGraph(t)
	// Only allow member_of edges (both e01 and e12 qualify).
	paths := g.FilteredTraversal("u0", func(e *model.Edge) bool {
		return e.Type == model.EdgeMemberOf
	})
	if len(paths) == 0 {
		t.Fatal("expected at least one path via FilteredTraversal")
	}
}

// BenchmarkFindPaths_Chain10 measures path enumeration on a 10-node chain.
func BenchmarkFindPaths_Chain10(b *testing.B) { benchChain(b, 10) }

// BenchmarkFindPaths_Chain50 measures path enumeration on a 50-node chain.
func BenchmarkFindPaths_Chain50(b *testing.B) { benchChain(b, 50) }

// BenchmarkFindPaths_Chain100 measures path enumeration on a 100-node chain.
func BenchmarkFindPaths_Chain100(b *testing.B) { benchChain(b, 100) }

// BenchmarkFindPaths_FanOut measures path enumeration through a two-level fan-out
// graph (hub → 20 spokes → 5 leaves each).
func BenchmarkFindPaths_FanOut(b *testing.B) {
	g := New()
	const spokes, leaves = 20, 5
	g.AddNode(model.NewNode("hub", model.NodeGroup, "hub"))
	for s := 0; s < spokes; s++ {
		sid := intStr(s)
		g.AddNode(model.NewNode(sid, model.NodeGroup, sid))
		g.AddEdge(model.NewEdge("hs-"+sid, model.EdgeMemberOf, "hub", sid))
		for l := 0; l < leaves; l++ {
			lid := intStr(1000+s*leaves+l)
			g.AddNode(model.NewNode(lid, model.NodeUser, lid))
			g.AddEdge(model.NewEdge("sl-"+lid, model.EdgeMemberOf, sid, lid))
		}
	}
	opts := DefaultPathOptions()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		g.FindPaths("hub", intStr(1000), opts)
	}
}

func benchChain(b *testing.B, n int) {
	b.Helper()
	g := New()
	for i := 0; i < n; i++ {
		g.AddNode(model.NewNode(intStr(i), model.NodeUser, intStr(i)))
	}
	for i := 0; i < n-1; i++ {
		e := model.NewEdge("e"+intStr(i), model.EdgeMemberOf, intStr(i), intStr(i+1))
		g.AddEdge(e)
	}
	opts := DefaultPathOptions()
	opts.MaxDepth = n + 1
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		g.FindPaths(intStr(0), intStr(n-1), opts)
	}
}

func intStr(i int) string { return fmt.Sprintf("%d", i) }
