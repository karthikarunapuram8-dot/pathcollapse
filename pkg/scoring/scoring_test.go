package scoring

import (
	"testing"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
)

type fakeGraph struct {
	nodes map[string]*model.Node
}

func (f *fakeGraph) GetNode(id string) *model.Node { return f.nodes[id] }

func buildPath(edges ...*model.Edge) graph.Path {
	nodes := make([]*model.Node, len(edges)+1)
	for i := range nodes {
		nodes[i] = model.NewNode("x", model.NodeUser, "x")
	}
	return graph.Path{Nodes: nodes, Edges: edges}
}

func TestScorePath_EmptyPath(t *testing.T) {
	p := graph.Path{}
	score := ScorePath(p, &fakeGraph{}, DefaultConfig())
	if score != 0 {
		t.Fatalf("empty path score should be 0, got %f", score)
	}
}

func TestScorePath_Tier0Target(t *testing.T) {
	target := model.NewNode("dc", model.NodeComputer, "DC")
	target.Tags = []string{model.TagTier0}

	fg := &fakeGraph{nodes: map[string]*model.Node{"dc": target}}

	e := model.NewEdge("e1", model.EdgeAdminTo, "u1", "dc")
	e.Confidence = 1.0
	e.Exploitability = 1.0
	e.Detectability = 0.0
	e.BlastRadius = 1.0

	startNode := model.NewNode("u1", model.NodeUser, "u1")
	p := graph.Path{Nodes: []*model.Node{startNode, target}, Edges: []*model.Edge{e}}

	score := ScorePath(p, fg, DefaultConfig())
	// Max possible: 0.30 + 0.20 + 0.20 + 0.15 + 0.15 = 1.0
	if score < 0.95 {
		t.Fatalf("expected high score for tier0 full-confidence path, got %f", score)
	}
}

func TestRankPaths(t *testing.T) {
	fg := &fakeGraph{nodes: map[string]*model.Node{}}

	lowE := model.NewEdge("e1", model.EdgeMemberOf, "u", "g")
	lowE.Confidence = 0.1
	lowE.Exploitability = 0.1
	lowE.Detectability = 1.0
	lowE.BlastRadius = 0.1

	highE := model.NewEdge("e2", model.EdgeAdminTo, "u", "g")
	highE.Confidence = 1.0
	highE.Exploitability = 1.0
	highE.Detectability = 0.0
	highE.BlastRadius = 1.0

	paths := []graph.Path{buildPath(lowE), buildPath(highE)}
	ranked := RankPaths(paths, fg, DefaultConfig())

	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked paths, got %d", len(ranked))
	}
	if ranked[0].Score <= ranked[1].Score {
		t.Fatalf("paths not sorted descending: %f <= %f", ranked[0].Score, ranked[1].Score)
	}
}

func TestDefaultConfigWeights(t *testing.T) {
	cfg := DefaultConfig()
	sum := cfg.TargetCriticalityWeight + cfg.ConfidenceWeight +
		cfg.ExploitabilityWeight + cfg.DetectabilityWeight + cfg.BlastRadiusWeight
	if sum < 0.999 || sum > 1.001 {
		t.Fatalf("weights should sum to 1.0, got %f", sum)
	}
}
