package subcmd

import (
	"testing"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/scoring"
)

func TestGatherTopPathsRanksBeforeLimiting(t *testing.T) {
	g := graph.New()

	nodes := []*model.Node{
		model.NewNode("low-src", model.NodeUser, "low-src"),
		model.NewNode("low-hop", model.NodeGroup, "low-hop"),
		model.NewNode("a-tier0", model.NodeComputer, "A-Tier0"),
		model.NewNode("high-src", model.NodeUser, "high-src"),
		model.NewNode("high-hop", model.NodeGroup, "high-hop"),
		model.NewNode("z-tier0", model.NodeComputer, "Z-Tier0"),
	}
	nodes[2].Tags = []string{model.TagTier0}
	nodes[5].Tags = []string{model.TagTier0}

	for _, n := range nodes {
		if err := g.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}

	low1 := model.NewEdge("low-1", model.EdgeMemberOf, "low-src", "low-hop")
	low1.Confidence = 0.1
	low1.Exploitability = 0.1
	low2 := model.NewEdge("low-2", model.EdgeAdminTo, "low-hop", "a-tier0")
	low2.Confidence = 0.1
	low2.Exploitability = 0.1

	high1 := model.NewEdge("high-1", model.EdgeMemberOf, "high-src", "high-hop")
	high1.Confidence = 1.0
	high1.Exploitability = 1.0
	high2 := model.NewEdge("high-2", model.EdgeAdminTo, "high-hop", "z-tier0")
	high2.Confidence = 1.0
	high2.Exploitability = 1.0

	for _, e := range []*model.Edge{low1, low2, high1, high2} {
		if err := g.AddEdge(e); err != nil {
			t.Fatal(err)
		}
	}

	scored := gatherTopPaths(g, scoring.DefaultConfig(), 1)
	if len(scored) != 1 {
		t.Fatalf("expected 1 scored path, got %d", len(scored))
	}
	if got := scored[0].Path.Target().ID; got != "z-tier0" {
		t.Fatalf("expected highest-risk path to be kept, got target %q", got)
	}
}
