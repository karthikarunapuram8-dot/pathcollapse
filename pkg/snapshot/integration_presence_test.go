package snapshot_test

import (
	"testing"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/confidence"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/controls"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/scoring"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/snapshot"
)

// TestPresence_DrivesTemporalStabilityThroughOptimizer verifies the full
// chain: snapshots stored → Presence indexes them → controls.Optimize uses
// it as SnapshotProvider → T(e) reflects the stored history.
//
// The two scenarios (stable edge vs never-seen-before edge) must produce
// materially different TemporalStability values for the same recommendation.
func TestPresence_DrivesTemporalStabilityThroughOptimizer(t *testing.T) {
	build := func() *graph.Graph {
		g := graph.New()
		alice := model.NewNode("alice", model.NodeUser, "Alice")
		admins := model.NewNode("admins", model.NodeGroup, "Admins")
		admins.Tags = []string{model.TagTier1}
		dc := model.NewNode("dc", model.NodeComputer, "DC")
		dc.Tags = []string{model.TagTier0}
		g.AddNode(alice)
		g.AddNode(admins)
		g.AddNode(dc)
		g.AddEdge(model.NewEdge("e1", model.EdgeMemberOf, "alice", "admins"))
		g.AddEdge(model.NewEdge("e2", model.EdgeAdminTo, "admins", "dc"))
		return g
	}

	runOptimize := func(t *testing.T, p *snapshot.Presence) []controls.ControlRecommendation {
		t.Helper()
		g := build()
		paths := g.FindPaths("alice", "dc", graph.DefaultPathOptions())
		scored := scoring.RankPaths(paths, g, scoring.DefaultConfig())
		opt := controls.DefaultOptimizerConfig()
		opt.Confidence = &controls.ConfidenceOptions{
			ScoringCfg: scoring.DefaultConfig(),
			Snapshots:  p,
			Config:     confidence.DefaultConfig(),
		}
		return controls.Optimize(scored, g, opt)
	}

	// ── Scenario A: edges have been present across 4 snapshots. ──────────
	stable := openTempStore(t)
	for i := 0; i < 4; i++ {
		if _, err := stable.Save("snap", build()); err != nil {
			t.Fatal(err)
		}
	}
	pStable, err := snapshot.NewPresence(stable, 8)
	if err != nil {
		t.Fatal(err)
	}
	if pStable.Window() != 4 {
		t.Fatalf("stable window: got %d want 4", pStable.Window())
	}
	stableRecs := runOptimize(t, pStable)

	// ── Scenario B: snapshots exist but none contain the edges we're
	//                about to recommend collapsing (drift/new exposure). ──
	drift := openTempStore(t)
	for i := 0; i < 4; i++ {
		// Different graph with unrelated edges.
		g := buildGraphWithEdges(t, [][3]string{{"xx", "yy", "member_of"}})
		if _, err := drift.Save("drift", g); err != nil {
			t.Fatal(err)
		}
	}
	pDrift, err := snapshot.NewPresence(drift, 8)
	if err != nil {
		t.Fatal(err)
	}
	driftRecs := runOptimize(t, pDrift)

	// Both optimisations should surface at least one rec with a Breakdown.
	if len(stableRecs) == 0 || stableRecs[0].Breakdown == nil {
		t.Fatal("stable scenario: expected rec with breakdown")
	}
	if len(driftRecs) == 0 || driftRecs[0].Breakdown == nil {
		t.Fatal("drift scenario: expected rec with breakdown")
	}

	stableT := stableRecs[0].Breakdown.TemporalStability
	driftT := driftRecs[0].Breakdown.TemporalStability

	// Stable presence should yield materially higher T than the drift
	// scenario. Using a 0.10 absolute-delta threshold rather than an exact
	// number to survive future prior-weight retuning.
	if stableT-driftT < 0.10 {
		t.Errorf("expected stable T to exceed drift T by ≥0.10; got stable=%v drift=%v",
			stableT, driftT)
	}
}
