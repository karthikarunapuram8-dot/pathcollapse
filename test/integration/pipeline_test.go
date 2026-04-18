package integration_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/karthikarunapuram8-dot/pathcollapse/internal/testdata"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/controls"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/reporting"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/scoring"
)

func TestFullPipeline(t *testing.T) {
	g := testdata.EnterpriseAD()

	if g.NodeCount() == 0 {
		t.Fatal("fixture graph has no nodes")
	}
	if g.EdgeCount() == 0 {
		t.Fatal("fixture graph has no edges")
	}

	// Collect all paths to tier-0 targets.
	opts := graph.DefaultPathOptions()
	var allPaths []graph.Path
	for _, tgt := range g.Nodes() {
		if !tgt.HasTag(model.TagTier0) {
			continue
		}
		for _, src := range g.Nodes() {
			if src.ID == tgt.ID {
				continue
			}
			found := g.FindPaths(src.ID, tgt.ID, opts)
			allPaths = append(allPaths, found...)
		}
	}

	if len(allPaths) == 0 {
		t.Fatal("no paths found to tier-0 targets; fixture may be missing tier0 tags or edges")
	}

	// Score and rank paths.
	cfg := scoring.DefaultConfig()
	scored := scoring.RankPaths(allPaths, g, cfg)
	if len(scored) == 0 {
		t.Fatal("scoring returned no ranked paths")
	}
	if scored[0].Score <= 0 {
		t.Errorf("top scored path has non-positive score: %f", scored[0].Score)
	}

	// Compute breakpoints via greedy set-cover.
	recs := controls.Optimize(scored, g, controls.DefaultOptimizerConfig())
	if len(recs) == 0 {
		t.Fatal("optimizer returned no control recommendations")
	}
	for i, rec := range recs {
		if rec.PathsRemoved <= 0 {
			t.Errorf("recommendation %d collapses 0 paths", i)
		}
		if rec.Change.Description == "" {
			t.Errorf("recommendation %d has empty description", i)
		}
	}

	// Render a markdown report and verify expected sections.
	rep := reporting.BuildReport(g, scored, recs)
	var buf bytes.Buffer
	r := reporting.New(reporting.FormatMarkdown)
	if err := r.Render(&buf, rep); err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Fatal("rendered report is empty")
	}

	for _, want := range []string{
		"PathCollapse Analysis Report",
		"Top Risk Paths",
		"Control Recommendations",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing section %q", want)
		}
	}
}

func TestDriftDetection(t *testing.T) {
	base := testdata.EnterpriseAD()

	// Clone by rebuilding from scratch and add a new tier-0 membership edge.
	modified := testdata.EnterpriseAD()
	extraEdge := model.NewEdge("e-test-drift", model.EdgeMemberOf, "usr-charlie", "grp-domain-admins")
	extraEdge.Confidence = 1.0
	extraEdge.Exploitability = 0.9
	extraEdge.BlastRadius = 0.9
	_ = modified.AddEdge(extraEdge)

	if modified.EdgeCount() <= base.EdgeCount() {
		t.Fatalf("modified graph should have more edges: base=%d modified=%d",
			base.EdgeCount(), modified.EdgeCount())
	}
}

func TestScoringWeightsSum(t *testing.T) {
	cfg := scoring.DefaultConfig()
	total := cfg.TargetCriticalityWeight +
		cfg.ConfidenceWeight +
		cfg.ExploitabilityWeight +
		cfg.DetectabilityWeight +
		cfg.BlastRadiusWeight

	const epsilon = 1e-9
	if total < 1.0-epsilon || total > 1.0+epsilon {
		t.Errorf("scoring weights sum to %.4f, want 1.0", total)
	}
}
