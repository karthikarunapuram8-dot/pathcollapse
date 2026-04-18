package drift

import (
	"testing"
	"time"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
)

// buildGraph is a test helper that creates a graph with the given nodes and edges.
func buildGraph(t *testing.T, nodes []*model.Node, edges []*model.Edge) *graph.Graph {
	t.Helper()
	g := graph.New()
	for _, n := range nodes {
		if err := g.AddNode(n); err != nil {
			t.Fatalf("AddNode %s: %v", n.ID, err)
		}
	}
	for _, e := range edges {
		if err := g.AddEdge(e); err != nil {
			t.Fatalf("AddEdge %s: %v", e.ID, err)
		}
	}
	return g
}

func TestCompareSnapshots_TimestampsStoredAsProvided(t *testing.T) {
	g := graph.New()
	oldAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newAt := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	rep := CompareSnapshots(g, g, oldAt, newAt)
	if !rep.OldSnapshotAt.Equal(oldAt) {
		t.Errorf("OldSnapshotAt: want %v, got %v", oldAt, rep.OldSnapshotAt)
	}
	if !rep.NewSnapshotAt.Equal(newAt) {
		t.Errorf("NewSnapshotAt: want %v, got %v", newAt, rep.NewSnapshotAt)
	}
}

func TestCompareSnapshots_ZeroTimestampsAllowed(t *testing.T) {
	g := graph.New()
	rep := CompareSnapshots(g, g, time.Time{}, time.Time{})
	if !rep.OldSnapshotAt.IsZero() || !rep.NewSnapshotAt.IsZero() {
		t.Error("zero timestamps should be stored as zero, not overwritten")
	}
}

func TestCompareSnapshots_DetectsAddedNode(t *testing.T) {
	n := model.NewNode("u1", model.NodeUser, "Alice")
	old := graph.New()
	new_ := buildGraph(t, []*model.Node{n}, nil)

	rep := CompareSnapshots(old, new_, time.Time{}, time.Time{})
	if rep.NodesAdded != 1 {
		t.Errorf("expected 1 added node, got %d", rep.NodesAdded)
	}
	if rep.NodesRemoved != 0 {
		t.Errorf("expected 0 removed nodes, got %d", rep.NodesRemoved)
	}
}

func TestCompareSnapshots_DetectsRemovedNode(t *testing.T) {
	n := model.NewNode("u1", model.NodeUser, "Alice")
	old := buildGraph(t, []*model.Node{n}, nil)
	new_ := graph.New()

	rep := CompareSnapshots(old, new_, time.Time{}, time.Time{})
	if rep.NodesRemoved != 1 {
		t.Errorf("expected 1 removed node, got %d", rep.NodesRemoved)
	}
	if rep.NodesAdded != 0 {
		t.Errorf("expected 0 added nodes, got %d", rep.NodesAdded)
	}
}

func TestCompareSnapshots_DetectsAddedEdge(t *testing.T) {
	u := model.NewNode("u1", model.NodeUser, "Alice")
	g1 := model.NewNode("g1", model.NodeGroup, "Admins")
	e := model.NewEdge("e1", model.EdgeMemberOf, "u1", "g1")

	old := buildGraph(t, []*model.Node{u, g1}, nil)
	new_ := buildGraph(t, []*model.Node{u, g1}, []*model.Edge{e})

	rep := CompareSnapshots(old, new_, time.Time{}, time.Time{})
	if rep.EdgesAdded != 1 {
		t.Errorf("expected 1 added edge, got %d", rep.EdgesAdded)
	}
}

func TestCompareSnapshots_ClassifiesTier0Membership(t *testing.T) {
	u := model.NewNode("u1", model.NodeUser, "Alice")
	g1 := model.NewNode("g1", model.NodeGroup, "Domain Admins")
	g1.Tags = []string{model.TagTier0}
	e := model.NewEdge("e1", model.EdgeMemberOf, "u1", "g1")

	old := buildGraph(t, []*model.Node{u, g1}, nil)
	new_ := buildGraph(t, []*model.Node{u, g1}, []*model.Edge{e})

	rep := CompareSnapshots(old, new_, time.Time{}, time.Time{})
	if len(rep.Items) != 1 {
		t.Fatalf("expected 1 drift item, got %d", len(rep.Items))
	}
	if rep.Items[0].Category != DriftNewPrivilegedMembership {
		t.Errorf("expected category %s, got %s", DriftNewPrivilegedMembership, rep.Items[0].Category)
	}
	if rep.Items[0].Severity != "high" {
		t.Errorf("expected severity high, got %s", rep.Items[0].Severity)
	}
}

func TestCompareSnapshots_ClassifiesDelegation(t *testing.T) {
	src := model.NewNode("svc1", model.NodeServiceAccount, "svc-sql")
	tgt := model.NewNode("dc1", model.NodeComputer, "DC01")
	e := model.NewEdge("e1", model.EdgeCanDelegateTo, "svc1", "dc1")

	old := buildGraph(t, []*model.Node{src, tgt}, nil)
	new_ := buildGraph(t, []*model.Node{src, tgt}, []*model.Edge{e})

	rep := CompareSnapshots(old, new_, time.Time{}, time.Time{})
	if len(rep.Items) != 1 {
		t.Fatalf("expected 1 drift item, got %d", len(rep.Items))
	}
	if rep.Items[0].Category != DriftDangerousDelegation {
		t.Errorf("expected category %s, got %s", DriftDangerousDelegation, rep.Items[0].Category)
	}
}

func TestCompareSnapshots_ClassifiesTrustExpansion(t *testing.T) {
	trust := model.NewNode("tr1", model.NodeTrust, "Partner")
	da := model.NewNode("g1", model.NodeGroup, "Domain Admins")
	e := model.NewEdge("e1", model.EdgeTrustedBy, "tr1", "g1")

	old := buildGraph(t, []*model.Node{trust, da}, nil)
	new_ := buildGraph(t, []*model.Node{trust, da}, []*model.Edge{e})

	rep := CompareSnapshots(old, new_, time.Time{}, time.Time{})
	if len(rep.Items) != 1 {
		t.Fatalf("expected 1 drift item, got %d", len(rep.Items))
	}
	if rep.Items[0].Category != DriftTrustExpansion {
		t.Errorf("expected category %s, got %s", DriftTrustExpansion, rep.Items[0].Category)
	}
	if rep.Items[0].Severity != "medium" {
		t.Errorf("expected severity medium, got %s", rep.Items[0].Severity)
	}
}

func TestCompareSnapshots_ClassifiesCertTemplate(t *testing.T) {
	u := model.NewNode("u1", model.NodeUser, "Alice")
	ct := model.NewNode("ct1", model.NodeCertTemplate, "UserCert")
	e := model.NewEdge("e1", model.EdgeCanEnrollIn, "u1", "ct1")

	old := buildGraph(t, []*model.Node{u, ct}, nil)
	new_ := buildGraph(t, []*model.Node{u, ct}, []*model.Edge{e})

	rep := CompareSnapshots(old, new_, time.Time{}, time.Time{})
	if len(rep.Items) != 1 {
		t.Fatalf("expected 1 drift item, got %d", len(rep.Items))
	}
	if rep.Items[0].Category != DriftCertTemplateChange {
		t.Errorf("expected category %s, got %s", DriftCertTemplateChange, rep.Items[0].Category)
	}
}

func TestCompareSnapshots_BenignEdgeProducesNoDriftItem(t *testing.T) {
	u := model.NewNode("u1", model.NodeUser, "Alice")
	g1 := model.NewNode("g1", model.NodeGroup, "Finance") // no tier0 tag
	e := model.NewEdge("e1", model.EdgeMemberOf, "u1", "g1")

	old := buildGraph(t, []*model.Node{u, g1}, nil)
	new_ := buildGraph(t, []*model.Node{u, g1}, []*model.Edge{e})

	rep := CompareSnapshots(old, new_, time.Time{}, time.Time{})
	if rep.EdgesAdded != 1 {
		t.Errorf("expected 1 edge added, got %d", rep.EdgesAdded)
	}
	if len(rep.Items) != 0 {
		t.Errorf("expected no drift items for benign membership, got %d", len(rep.Items))
	}
}

func TestCompareSnapshots_IdenticalGraphsNoDrift(t *testing.T) {
	u := model.NewNode("u1", model.NodeUser, "Alice")
	g1 := model.NewNode("g1", model.NodeGroup, "Domain Admins")
	g1.Tags = []string{model.TagTier0}
	e := model.NewEdge("e1", model.EdgeMemberOf, "u1", "g1")

	g := buildGraph(t, []*model.Node{u, g1}, []*model.Edge{e})
	rep := CompareSnapshots(g, g, time.Time{}, time.Time{})

	if rep.NodesAdded != 0 || rep.NodesRemoved != 0 || rep.EdgesAdded != 0 || rep.EdgesRemoved != 0 {
		t.Errorf("identical graphs should produce zero drift: %+v", rep)
	}
	if len(rep.Items) != 0 {
		t.Errorf("identical graphs should produce no items, got %d", len(rep.Items))
	}
}
