package reporting

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/karunapuram/pathcollapse/pkg/confidence"
	"github.com/karunapuram/pathcollapse/pkg/controls"
	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
)

func buildTestReport(t *testing.T) (*Report, *graph.Graph) {
	t.Helper()
	g := graph.New()
	alice := model.NewNode("alice", model.NodeUser, "Alice")
	dc := model.NewNode("dc", model.NodeComputer, "DC01")
	dc.Tags = []string{model.TagTier0}
	g.AddNode(alice)
	g.AddNode(dc)
	e := model.NewEdge("e1", model.EdgeAdminTo, "alice", "dc")
	g.AddEdge(e)

	paths := g.FindPaths("alice", "dc", graph.DefaultPathOptions())
	cfg := scoring.DefaultConfig()
	scored := scoring.RankPaths(paths, g, cfg)

	recs := controls.Optimize(scored, g, controls.DefaultOptimizerConfig())

	rep := BuildReport(g, scored, recs)
	return rep, g
}

func TestRenderMarkdown(t *testing.T) {
	rep, _ := buildTestReport(t)
	r := New(FormatMarkdown)
	var buf bytes.Buffer
	if err := r.Render(&buf, rep); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "PathCollapse Analysis Report") {
		t.Error("expected report title in markdown output")
	}
	if !strings.Contains(out, "Top Risk Paths") {
		t.Error("expected top risk paths section")
	}
	if !strings.Contains(out, "Detection And Telemetry") {
		t.Error("expected detection and telemetry section")
	}
	if !strings.Contains(out, "ATT&CK techniques") {
		t.Error("expected ATT&CK details in markdown output")
	}
}

func TestRenderJSON(t *testing.T) {
	rep, _ := buildTestReport(t)
	r := New(FormatJSON)
	var buf bytes.Buffer
	if err := r.Render(&buf, rep); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if _, ok := out["node_count"]; !ok {
		t.Error("expected node_count field in JSON")
	}
	if _, ok := out["path_details"]; !ok {
		t.Error("expected path_details field in JSON")
	}
}

func TestExecutiveSummary_NoPaths(t *testing.T) {
	rep := &Report{GeneratedAt: time.Now()}
	var buf bytes.Buffer
	if err := ExecutiveSummary(&buf, rep); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No high-risk paths") {
		t.Error("expected no-paths message")
	}
}

func TestBuildReport(t *testing.T) {
	g := graph.New()
	g.AddNode(model.NewNode("x", model.NodeUser, "X"))
	rep := BuildReport(g, nil, nil)
	if rep.NodeCount != 1 {
		t.Fatalf("expected 1 node, got %d", rep.NodeCount)
	}
	if rep.GeneratedAt.IsZero() {
		t.Fatal("GeneratedAt should not be zero")
	}
	if len(rep.PathDetails) != 0 {
		t.Fatalf("expected no path details for empty report, got %d", len(rep.PathDetails))
	}
}

func TestBuildReport_PathDetails(t *testing.T) {
	rep, _ := buildTestReport(t)
	if len(rep.PathDetails) != len(rep.TopPaths) {
		t.Fatalf("expected one path detail per top path, got %d details for %d paths", len(rep.PathDetails), len(rep.TopPaths))
	}
	detail := rep.PathDetails[0]
	if detail.Detection.SigmaRule == "" {
		t.Fatal("expected Sigma rule in path detail")
	}
	if len(detail.Telemetry) == 0 {
		t.Fatal("expected telemetry requirements in path detail")
	}
}

// TestRenderReproducibility verifies that rendering the same Report twice
// produces byte-identical output (no timestamp churn, random map iteration, etc.).
func TestRenderReproducibility(t *testing.T) {
	rep, _ := buildTestReport(t)
	// Pin GeneratedAt so the timestamp is deterministic across both renders.
	rep.GeneratedAt = time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	render := func() []byte {
		var buf bytes.Buffer
		if err := New(FormatMarkdown).Render(&buf, rep); err != nil {
			t.Fatalf("Render: %v", err)
		}
		return buf.Bytes()
	}

	first := render()
	second := render()
	if !bytes.Equal(first, second) {
		t.Errorf("renders are not byte-equal:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// ── Confidence surfacing ───────────────────────────────────────────────────

// buildConfidenceTestReport extends buildTestReport with confidence-enabled
// recommendations so we can verify the reporting surfaces them.
func buildConfidenceTestReport(t *testing.T) *Report {
	t.Helper()
	g := graph.New()
	alice := model.NewNode("alice", model.NodeUser, "Alice")
	dc := model.NewNode("dc", model.NodeComputer, "DC01")
	dc.Tags = []string{model.TagTier0}
	g.AddNode(alice)
	g.AddNode(dc)
	e := model.NewEdge("e1", model.EdgeAdminTo, "alice", "dc")
	e.Evidence = []model.EvidenceRef{{Source: "bloodhound"}, {Source: "csv"}}
	g.AddEdge(e)

	paths := g.FindPaths("alice", "dc", graph.DefaultPathOptions())
	scored := scoring.RankPaths(paths, g, scoring.DefaultConfig())

	optCfg := controls.DefaultOptimizerConfig()
	optCfg.Confidence = &controls.ConfidenceOptions{
		ScoringCfg: scoring.DefaultConfig(),
		Config:     confidence.DefaultConfig(),
	}
	recs := controls.Optimize(scored, g, optCfg)

	return BuildReport(g, scored, recs)
}

func TestBuildConfidenceSummary_EmptyWhenNoBreakdowns(t *testing.T) {
	s := BuildConfidenceSummary(nil)
	if s.HasConfidence {
		t.Fatal("expected HasConfidence=false for nil input")
	}
	if s.Count != 0 {
		t.Fatalf("expected Count=0, got %d", s.Count)
	}
}

func TestBuildConfidenceSummary_Aggregates(t *testing.T) {
	rep := buildConfidenceTestReport(t)
	s := rep.Confidence
	if !s.HasConfidence {
		t.Fatal("expected HasConfidence=true when recs carry Breakdown")
	}
	if s.Count == 0 {
		t.Fatal("expected non-zero scored count")
	}
	if s.Average <= 0 || s.Average > 1 {
		t.Fatalf("average out of range: %v", s.Average)
	}
	if s.Highest < s.Average || s.Lowest > s.Average {
		t.Fatalf("min/max/avg invariant broken: min=%v avg=%v max=%v",
			s.Lowest, s.Average, s.Highest)
	}
	if s.ColdStart+s.Partial+s.Calibrated != s.Count {
		t.Fatalf("regime counts do not sum to Count: %d+%d+%d != %d",
			s.ColdStart, s.Partial, s.Calibrated, s.Count)
	}
}

func TestTopDriversOrdering(t *testing.T) {
	b := &confidence.Breakdown{
		Evidence:              0.40,
		Robustness:            0.92,
		Safety:                0.10,
		TemporalStability:     0.88,
		CoverageConcentration: 0.50,
	}
	drivers := TopDrivers(b)
	if len(drivers) != 2 {
		t.Fatalf("expected 2 drivers, got %d: %v", len(drivers), drivers)
	}
	if !strings.HasPrefix(drivers[0], "robustness") {
		t.Errorf("expected robustness first, got %q", drivers[0])
	}
	if !strings.HasPrefix(drivers[1], "temporal") {
		t.Errorf("expected temporal second, got %q", drivers[1])
	}
}

func TestLowestDriver(t *testing.T) {
	b := &confidence.Breakdown{
		Evidence:              0.40,
		Robustness:            0.92,
		Safety:                0.10,
		TemporalStability:     0.88,
		CoverageConcentration: 0.50,
	}
	if got := LowestDriver(b); !strings.HasPrefix(got, "safety") {
		t.Errorf("expected safety lowest, got %q", got)
	}
	if got := LowestDriver(nil); got != "" {
		t.Errorf("nil breakdown should return empty, got %q", got)
	}
}

func TestRenderMarkdown_SurfacesConfidence(t *testing.T) {
	rep := buildConfidenceTestReport(t)
	var buf bytes.Buffer
	if err := New(FormatMarkdown).Render(&buf, rep); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Confidence Summary",
		"Average confidence",
		"Regime",
		"**Drivers**",
		"**Weakest factor**",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown output missing %q", want)
		}
	}
}

func TestRenderMarkdown_OmitsConfidenceWhenAbsent(t *testing.T) {
	rep, _ := buildTestReport(t) // no confidence enabled
	var buf bytes.Buffer
	if err := New(FormatMarkdown).Render(&buf, rep); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "Confidence Summary") {
		t.Error("confidence summary must not appear when no breakdowns present")
	}
	if strings.Contains(out, "**Drivers**") {
		t.Error("driver line must not appear when no breakdowns present")
	}
}

func TestRenderHTML_SurfacesConfidence(t *testing.T) {
	rep := buildConfidenceTestReport(t)
	var buf bytes.Buffer
	if err := New(FormatHTML).Render(&buf, rep); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Recommendation Confidence",
		"<th>Why</th>",
		"Calibration regime",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("html output missing %q", want)
		}
	}
}

func TestRenderJSON_IncludesConfidenceSummary(t *testing.T) {
	rep := buildConfidenceTestReport(t)
	var buf bytes.Buffer
	if err := New(FormatJSON).Render(&buf, rep); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	cs, ok := out["confidence_summary"].(map[string]any)
	if !ok {
		t.Fatal("expected confidence_summary object in JSON")
	}
	if cs["has_confidence"] != true {
		t.Errorf("expected has_confidence=true, got %v", cs["has_confidence"])
	}
	// Recommendation objects should carry confidence_breakdown.
	recs, _ := out["recommendations"].([]any)
	if len(recs) == 0 {
		t.Fatal("expected recommendations in JSON")
	}
	firstRec, _ := recs[0].(map[string]any)
	if _, ok := firstRec["confidence_breakdown"]; !ok {
		t.Error("expected confidence_breakdown on first recommendation")
	}
}
