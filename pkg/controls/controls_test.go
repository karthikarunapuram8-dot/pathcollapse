package controls

import (
	"testing"

	"github.com/karunapuram/pathcollapse/pkg/confidence"
	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
)

func buildScoredPaths(t *testing.T) ([]scoring.ScoredPath, *graph.Graph) {
	t.Helper()
	g := graph.New()
	alice := model.NewNode("alice", model.NodeUser, "alice")
	bob := model.NewNode("bob", model.NodeUser, "bob")
	admins := model.NewNode("admins", model.NodeGroup, "Domain Admins")
	dc := model.NewNode("dc", model.NodeComputer, "DC01")
	dc.Tags = []string{model.TagTier0}

	for _, n := range []*model.Node{alice, bob, admins, dc} {
		g.AddNode(n)
	}
	e1 := model.NewEdge("e1", model.EdgeMemberOf, "alice", "admins")
	e2 := model.NewEdge("e2", model.EdgeAdminTo, "admins", "dc")
	e3 := model.NewEdge("e3", model.EdgeMemberOf, "bob", "admins")
	for _, e := range []*model.Edge{e1, e2, e3} {
		g.AddEdge(e)
	}

	cfg := scoring.DefaultConfig()
	alicePaths := g.FindPaths("alice", "dc", graph.DefaultPathOptions())
	bobPaths := g.FindPaths("bob", "dc", graph.DefaultPathOptions())
	allPaths := append(alicePaths, bobPaths...)
	scored := scoring.RankPaths(allPaths, g, cfg)
	return scored, g
}

func TestOptimize_Basic(t *testing.T) {
	scored, g := buildScoredPaths(t)
	if len(scored) == 0 {
		t.Fatal("expected scored paths")
	}

	recs := Optimize(scored, g, DefaultOptimizerConfig())
	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation")
	}
}

func TestOptimize_GreedyPicksBestFirst(t *testing.T) {
	scored, g := buildScoredPaths(t)
	recs := Optimize(scored, g, DefaultOptimizerConfig())
	if len(recs) < 2 {
		t.Skip("fewer than 2 recommendations")
	}
	// The first recommendation should remove at least as many paths as subsequent.
	if recs[0].PathsRemoved < recs[1].PathsRemoved {
		t.Fatalf("greedy: first rec (%d paths) should remove >= second (%d paths)",
			recs[0].PathsRemoved, recs[1].PathsRemoved)
	}
}

func TestOptimize_Empty(t *testing.T) {
	recs := Optimize(nil, graph.New(), DefaultOptimizerConfig())
	if recs != nil {
		t.Fatal("empty input should return nil")
	}
}

func TestOptimize_MaxRecommendations(t *testing.T) {
	scored, g := buildScoredPaths(t)
	cfg := DefaultOptimizerConfig()
	cfg.MaxRecommendations = 1
	recs := Optimize(scored, g, cfg)
	if len(recs) > 1 {
		t.Fatalf("expected at most 1 recommendation, got %d", len(recs))
	}
}

func TestChangeForEdge_MemberOf(t *testing.T) {
	g := graph.New()
	g.AddNode(model.NewNode("u1", model.NodeUser, "Alice"))
	g.AddNode(model.NewNode("g1", model.NodeGroup, "Admins"))
	e := model.NewEdge("e1", model.EdgeMemberOf, "u1", "g1")
	c := changeForEdge(e, g)
	if c.Type != ChangeRemoveMember {
		t.Fatalf("expected ChangeRemoveMember, got %v", c.Type)
	}
}

func TestDifficultyForChange(t *testing.T) {
	tests := []struct {
		ct   ChangeType
		want Difficulty
	}{
		{ChangeRemoveMember, DifficultyLow},
		{ChangeRemoveEdge, DifficultyMedium},
		{ChangeModifyNode, DifficultyHigh},
	}
	for _, tc := range tests {
		if got := difficultyForChange(tc.ct); got != tc.want {
			t.Errorf("difficultyForChange(%v) = %v, want %v", tc.ct, got, tc.want)
		}
	}
}

// ── Confidence integration ─────────────────────────────────────────────────

func TestOptimize_LegacyConfidenceWhenDisabled(t *testing.T) {
	scored, g := buildScoredPaths(t)
	recs := Optimize(scored, g, DefaultOptimizerConfig())
	if len(recs) == 0 {
		t.Fatal("expected recommendations")
	}
	for i, r := range recs {
		if r.Confidence != LegacyConfidence {
			t.Errorf("rec[%d]: expected legacy %v, got %v", i, LegacyConfidence, r.Confidence)
		}
		if r.Breakdown != nil {
			t.Errorf("rec[%d]: Breakdown must be nil when confidence disabled", i)
		}
		if r.Regime != "" {
			t.Errorf("rec[%d]: Regime must be empty when confidence disabled, got %q", i, r.Regime)
		}
	}
}

func TestOptimize_ConfidenceEnabledReplacesLegacyConstant(t *testing.T) {
	scored, g := buildScoredPaths(t)

	cfg := DefaultOptimizerConfig()
	cfg.Confidence = &ConfidenceOptions{
		ScoringCfg: scoring.DefaultConfig(),
		Config:     confidence.DefaultConfig(),
	}

	recs := Optimize(scored, g, cfg)
	if len(recs) == 0 {
		t.Fatal("expected recommendations")
	}

	anyBreakdown := false
	anyDiverged := false
	for _, r := range recs {
		if r.Breakdown != nil {
			anyBreakdown = true
			// Every factor must land in [0, 1].
			b := r.Breakdown
			for name, v := range map[string]float64{
				"Evidence":              b.Evidence,
				"Robustness":            b.Robustness,
				"Safety":                b.Safety,
				"TemporalStability":     b.TemporalStability,
				"CoverageConcentration": b.CoverageConcentration,
				"Raw":                   b.Raw,
				"Final":                 b.Final,
			} {
				if v < 0 || v > 1 {
					t.Errorf("rec breakdown %s out of [0,1]: %v", name, v)
				}
			}
			if r.Confidence != b.Final {
				t.Errorf("rec.Confidence (%v) != Breakdown.Final (%v)", r.Confidence, b.Final)
			}
			if r.Regime != confidence.RegimeColdStart {
				t.Errorf("expected cold-start regime, got %q", r.Regime)
			}
		}
		if r.Confidence != LegacyConfidence {
			anyDiverged = true
		}
	}
	if !anyBreakdown {
		t.Fatal("expected at least one rec with Breakdown populated")
	}
	if !anyDiverged {
		t.Fatal("expected at least one rec to diverge from LegacyConfidence")
	}
}
