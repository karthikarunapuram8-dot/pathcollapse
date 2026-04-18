package reporting

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

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
