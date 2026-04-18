package reasoning

import (
	"testing"

	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
)

func buildGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g := graph.New()
	alice := model.NewNode("alice", model.NodeUser, "alice")
	admins := model.NewNode("admins", model.NodeGroup, "Domain Admins")
	dc := model.NewNode("dc", model.NodeComputer, "DC01")
	dc.Tags = []string{model.TagTier0}

	for _, n := range []*model.Node{alice, admins, dc} {
		g.AddNode(n)
	}
	g.AddEdge(model.NewEdge("e1", model.EdgeMemberOf, "alice", "admins"))
	g.AddEdge(model.NewEdge("e2", model.EdgeAdminTo, "admins", "dc"))
	return g
}

func TestReachability(t *testing.T) {
	g := buildGraph(t)
	r := New(g, scoring.DefaultConfig())
	analyses := r.FindAndAnalyse("alice", "dc", ModeReachability, graph.DefaultPathOptions())
	if len(analyses) == 0 {
		t.Fatal("expected at least one path")
	}
	if !analyses[0].Realistic {
		t.Fatal("reachability path should be realistic")
	}
}

func TestPlausibility_UnsatisfiedPrecondition(t *testing.T) {
	g := buildGraph(t)
	// Mark e1 as having an unsatisfied precondition.
	e1 := g.GetEdge("e1")
	e1.Preconditions = []model.Precondition{
		{Description: "requires VPN", Satisfied: false},
	}
	r := New(g, scoring.DefaultConfig())
	analyses := r.FindAndAnalyse("alice", "dc", ModePlausibility, graph.DefaultPathOptions())
	if len(analyses) == 0 {
		t.Fatal("expected at least one analysis")
	}
	if analyses[0].Realistic {
		t.Fatal("path with unsatisfied precondition should not be realistic")
	}
	if len(analyses[0].MissingPreconditions) == 0 {
		t.Fatal("expected missing preconditions list")
	}
}

func TestDefensive_RemediationHints(t *testing.T) {
	g := buildGraph(t)
	r := New(g, scoring.DefaultConfig())
	analyses := r.FindAndAnalyse("alice", "dc", ModeDefensive, graph.DefaultPathOptions())
	if len(analyses) == 0 {
		t.Fatal("expected at least one analysis")
	}
	if len(analyses[0].RemediationHints) == 0 {
		t.Fatal("expected remediation hints in defensive mode")
	}
}

func TestAnalysePaths_Sorted(t *testing.T) {
	g := buildGraph(t)
	r := New(g, scoring.DefaultConfig())
	paths := g.FindPaths("alice", "dc", graph.DefaultPathOptions())
	if len(paths) == 0 {
		t.Skip("no paths found")
	}
	analyses := r.AnalysePaths(paths, ModeDefensive)
	for i := 1; i < len(analyses); i++ {
		if analyses[i].Score > analyses[i-1].Score {
			t.Fatal("analyses not sorted descending by score")
		}
	}
}
