package confidence

import (
	"math"
	"testing"
	"time"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/scoring"
)

// ── aggregation primitives ─────────────────────────────────────────────────

func TestSigmaLogitRoundTrip(t *testing.T) {
	for _, p := range []float64{0.01, 0.1, 0.3, 0.5, 0.7, 0.9, 0.99} {
		got := sigma(logit(p))
		if math.Abs(got-p) > 1e-9 {
			t.Fatalf("round-trip p=%v: got %v", p, got)
		}
	}
}

func TestClipBounds(t *testing.T) {
	if clip(-0.5, 1e-3) != 1e-3 {
		t.Fatal("clip negative failed")
	}
	if clip(1.5, 1e-3) != 1-1e-3 {
		t.Fatal("clip above one failed")
	}
	if clip(0.42, 1e-3) != 0.42 {
		t.Fatal("clip in-range changed value")
	}
}

// ── E(e) evidence quality ──────────────────────────────────────────────────

func TestEvidenceQualitySingleSourceFloors(t *testing.T) {
	cfg := DefaultConfig()
	e := model.NewEdge("e1", model.EdgeMemberOf, "a", "b") // Confidence=1.0
	e.Evidence = []model.EvidenceRef{{Source: "bloodhound"}}

	got := evidenceQuality(e, cfg)
	if got <= 0 || got >= 1 {
		t.Fatalf("expected single-source evidence in (0, 1), got %v", got)
	}
	// Hand-computed: 1.0^0.5 * 0.05^0.3 * 1.0^0.2 ≈ 0.4170
	want := math.Pow(0.05, 0.3)
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("single-source evidence: got %v want %v", got, want)
	}
}

func TestEvidenceQualityMultiSourceRises(t *testing.T) {
	cfg := DefaultConfig()
	e := model.NewEdge("e1", model.EdgeMemberOf, "a", "b")
	e.Evidence = []model.EvidenceRef{
		{Source: "bloodhound"},
		{Source: "csv_local_admin"},
		{Source: "yaml_analyst"},
	}

	got := evidenceQuality(e, cfg)
	// Must exceed single-source floor.
	single := math.Pow(0.05, 0.3)
	if got <= single {
		t.Fatalf("multi-source did not raise evidence above single-source: %v vs %v", got, single)
	}
}

func TestEvidenceQualityPreconditionFailureCrashes(t *testing.T) {
	cfg := DefaultConfig()
	e := model.NewEdge("e1", model.EdgeCanDelegateTo, "a", "b")
	e.Evidence = []model.EvidenceRef{{Source: "bloodhound"}, {Source: "yaml_analyst"}}
	e.Preconditions = []model.Precondition{
		{Description: "needs TGT", Satisfied: false},
		{Description: "needs SPN", Satisfied: true},
	}

	got := evidenceQuality(e, cfg)
	// sPre = 0.5 → factor 0.5^0.2 ≈ 0.87
	noPre := model.NewEdge("e2", model.EdgeCanDelegateTo, "a", "b")
	noPre.Evidence = e.Evidence
	gotNoPre := evidenceQuality(noPre, cfg)
	if got >= gotNoPre {
		t.Fatalf("unsatisfied preconditions should lower E: with=%v without=%v", got, gotNoPre)
	}
}

// ── R(e, G) structural robustness ──────────────────────────────────────────

// buildLinearGraph returns a graph with A→B→C (single path). Removing the
// only edge must yield R = 1.
func buildLinearGraph(t *testing.T) (*graph.Graph, *model.Edge, graph.Path) {
	t.Helper()
	g := graph.New()
	for _, id := range []string{"A", "B", "C"} {
		if err := g.AddNode(model.NewNode(id, model.NodeUser, id)); err != nil {
			t.Fatal(err)
		}
	}
	e1 := model.NewEdge("e1", model.EdgeMemberOf, "A", "B")
	e2 := model.NewEdge("e2", model.EdgeMemberOf, "B", "C")
	for _, e := range []*model.Edge{e1, e2} {
		if err := g.AddEdge(e); err != nil {
			t.Fatal(err)
		}
	}
	paths := g.FindPaths("A", "C", graph.PathOptions{MaxDepth: 5})
	if len(paths) != 1 {
		t.Fatalf("expected 1 path in linear graph, got %d", len(paths))
	}
	return g, e1, paths[0]
}

func TestStructuralRobustnessLinearGraph(t *testing.T) {
	g, e1, p := buildLinearGraph(t)
	got := structuralRobustness(e1, []graph.Path{p}, g, scoring.DefaultConfig(), DefaultConfig())
	if got < 0.99 {
		t.Fatalf("linear graph robustness: got %v, expected ~1.0", got)
	}
}

// buildDiamondGraph returns a graph with two parallel paths A→B→D and A→C→D.
// Removing A→B still leaves A→C→D as a viable alternative, so R should be low.
func buildDiamondGraph(t *testing.T) (*graph.Graph, *model.Edge, []graph.Path) {
	t.Helper()
	g := graph.New()
	for _, id := range []string{"A", "B", "C", "D"} {
		if err := g.AddNode(model.NewNode(id, model.NodeUser, id)); err != nil {
			t.Fatal(err)
		}
	}
	edges := []*model.Edge{
		model.NewEdge("ab", model.EdgeMemberOf, "A", "B"),
		model.NewEdge("bd", model.EdgeMemberOf, "B", "D"),
		model.NewEdge("ac", model.EdgeMemberOf, "A", "C"),
		model.NewEdge("cd", model.EdgeMemberOf, "C", "D"),
	}
	for _, e := range edges {
		if err := g.AddEdge(e); err != nil {
			t.Fatal(err)
		}
	}
	paths := g.FindPaths("A", "D", graph.PathOptions{MaxDepth: 5})
	if len(paths) < 2 {
		t.Fatalf("expected ≥2 paths in diamond, got %d", len(paths))
	}
	// Return the A→B→D path and edge A→B.
	return g, edges[0], paths
}

func TestStructuralRobustnessDiamondGraphLow(t *testing.T) {
	g, ab, paths := buildDiamondGraph(t)

	// Filter to just the paths that contain ab.
	var covered []graph.Path
	for _, p := range paths {
		for _, e := range p.Edges {
			if e.ID == ab.ID {
				covered = append(covered, p)
				break
			}
		}
	}
	got := structuralRobustness(ab, covered, g, scoring.DefaultConfig(), DefaultConfig())
	// Alternative path exists; residual risk should be high → R should be low.
	if got > 0.5 {
		t.Fatalf("diamond graph robustness: got %v, expected low (<0.5)", got)
	}
}

// ── S(e, G) operational safety ─────────────────────────────────────────────

func TestOperationalSafetyTierMismatchIsSuspicious(t *testing.T) {
	g := graph.New()
	user := model.NewNode("user", model.NodeUser, "U")
	user.Tags = []string{model.TagTier2}
	dc := model.NewNode("dc", model.NodeComputer, "DC")
	dc.Tags = []string{model.TagTier0}
	_ = g.AddNode(user)
	_ = g.AddNode(dc)
	e := model.NewEdge("e", model.EdgeAdminTo, "user", "dc")
	e.BlastRadius = 0.2
	_ = g.AddEdge(e)

	got := operationalSafety(e, g)
	if got < 0.7 {
		t.Fatalf("tier-2 → tier-0 should be high-safety-to-sever: got %v", got)
	}
}

func TestOperationalSafetyTierAlignedIsConservative(t *testing.T) {
	g := graph.New()
	admin := model.NewNode("admin", model.NodeGroup, "Admins")
	admin.Tags = []string{model.TagTier1}
	srv := model.NewNode("srv", model.NodeComputer, "APPSRV")
	srv.Tags = []string{model.TagTier1}
	_ = g.AddNode(admin)
	_ = g.AddNode(srv)
	e := model.NewEdge("e", model.EdgeAdminTo, "admin", "srv")
	e.BlastRadius = 0.5
	_ = g.AddEdge(e)

	got := operationalSafety(e, g)
	if got > 0.6 {
		t.Fatalf("tier-aligned admin should be conservative-safety: got %v", got)
	}
}

// ── T(e) temporal stability ────────────────────────────────────────────────

type fakeSnapshots struct {
	frac float64
	ok   bool
}

func (f fakeSnapshots) EdgePresence(_, _ string, _ model.EdgeType, _ int) (float64, bool) {
	return f.frac, f.ok
}

func TestTemporalStabilityColdStart(t *testing.T) {
	cfg := DefaultConfig()
	e := model.NewEdge("e", model.EdgeMemberOf, "a", "b") // FirstSeen = now
	got := temporalStability(e, nil, cfg, time.Now().UTC())
	// No snapshot ⇒ 0.6*0.5 + 0.4*ageFactor≈0 → ~0.3.
	if got < 0.25 || got > 0.35 {
		t.Fatalf("cold-start T(e): expected ~0.3, got %v", got)
	}
}

func TestTemporalStabilityMaturedEdge(t *testing.T) {
	cfg := DefaultConfig()
	e := model.NewEdge("e", model.EdgeMemberOf, "a", "b")
	e.FirstSeen = time.Now().UTC().AddDate(0, 0, -180) // 180 days old
	snaps := fakeSnapshots{frac: 1.0, ok: true}

	got := temporalStability(e, snaps, cfg, time.Now().UTC())
	if got < 0.95 {
		t.Fatalf("matured edge T(e): expected ≥0.95, got %v", got)
	}
}

// ── K(c) coverage concentration ────────────────────────────────────────────

func TestCoverageConcentrationUniqueIsOne(t *testing.T) {
	ci := NewCandidateIndex()
	ci.Register("e1", []int{0, 1, 2})
	ci.Register("e2", []int{3, 4})

	if got := coverageConcentration("e1", ci); got != 1.0 {
		t.Fatalf("unique coverage K: got %v want 1.0", got)
	}
}

func TestCoverageConcentrationTripletIsOneThird(t *testing.T) {
	ci := NewCandidateIndex()
	ci.Register("e1", []int{0, 1, 2})
	ci.Register("e2", []int{0, 1, 2})
	ci.Register("e3", []int{0, 1, 2})

	got := coverageConcentration("e1", ci)
	want := 1.0 / 3.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("triplet coverage K: got %v want %v", got, want)
	}
}

// ── Calibrator ─────────────────────────────────────────────────────────────

func TestIdentityCalibrator(t *testing.T) {
	c := IdentityCalibrator{}
	for _, x := range []float64{0, 0.3, 0.8, 1.0} {
		if c.Apply(x) != x {
			t.Fatalf("identity calibrator changed %v", x)
		}
	}
	if c.Regime() != RegimeColdStart {
		t.Fatal("identity calibrator should report cold start")
	}
}

func TestIsotonicCalibratorMonotone(t *testing.T) {
	ic := NewIsotonicCalibrator()
	outcomes := []LabeledOutcome{
		{Raw: 0.1, Y: 0}, {Raw: 0.2, Y: 0}, {Raw: 0.3, Y: 1},
		{Raw: 0.4, Y: 0}, // intentional violation — PAV should pool
		{Raw: 0.5, Y: 1}, {Raw: 0.7, Y: 1}, {Raw: 0.9, Y: 1},
	}
	if err := ic.Fit(outcomes); err != nil {
		t.Fatal(err)
	}

	prev := -1.0
	for _, x := range []float64{0.0, 0.15, 0.35, 0.55, 0.75, 0.95} {
		got := ic.Apply(x)
		if got < prev-1e-9 {
			t.Fatalf("non-monotone: f(%v)=%v < prev=%v", x, got, prev)
		}
		prev = got
	}
}

func TestIsotonicCalibratorRegimeBuckets(t *testing.T) {
	ic := NewIsotonicCalibrator()
	mk := func(n int) []LabeledOutcome {
		out := make([]LabeledOutcome, n)
		for i := range out {
			out[i] = LabeledOutcome{Raw: float64(i) / float64(n), Y: float64(i % 2)}
		}
		return out
	}
	_ = ic.Fit(mk(10))
	if ic.Regime() != RegimeColdStart {
		t.Fatal("10 outcomes should be cold_start")
	}
	_ = ic.Fit(mk(100))
	if ic.Regime() != RegimePartial {
		t.Fatal("100 outcomes should be partial")
	}
	_ = ic.Fit(mk(1000))
	if ic.Regime() != RegimeCalibrated {
		t.Fatal("1000 outcomes should be calibrated")
	}
}

// ── End-to-end: ScoreEdge on the diamond graph ─────────────────────────────

func TestScoreEdgeEndToEndDiamondGraph(t *testing.T) {
	g, ab, paths := buildDiamondGraph(t)
	var covered []graph.Path
	for _, p := range paths {
		for _, e := range p.Edges {
			if e.ID == ab.ID {
				covered = append(covered, p)
				break
			}
		}
	}
	ab.Evidence = []model.EvidenceRef{{Source: "bloodhound"}, {Source: "csv"}}
	ab.FirstSeen = time.Now().UTC().AddDate(0, 0, -60)

	ci := NewCandidateIndex()
	ci.Register(ab.ID, []int{0})

	final, b, regime, err := ScoreEdge(
		ScoreEdgeInput{Edge: ab, AffectedPaths: covered, CandidateIndex: ci},
		Deps{
			Graph:      g,
			ScoringCfg: scoring.DefaultConfig(),
			Snapshots:  fakeSnapshots{frac: 0.875, ok: true},
			Calibrator: IdentityCalibrator{},
			Now:        time.Now().UTC(),
		},
		DefaultConfig(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if regime != RegimeColdStart {
		t.Fatalf("expected cold start, got %s", regime)
	}
	if final <= 0 || final >= 1 {
		t.Fatalf("final probability out of bounds: %v", final)
	}
	// Sanity: diamond graph yields low R, so Breakdown.Robustness must be
	// materially below 1.0 — this is the signal the algorithm is supposed
	// to surface in an explainable report.
	if b.Robustness > 0.5 {
		t.Fatalf("diamond graph should yield low Robustness, got %v", b.Robustness)
	}

	// Counterfactual: swap in a high-R reading and confirm Final rises.
	// This verifies R actually influences the output in the expected direction.
	bHighR := b
	bHighR.Robustness = 0.95
	altRaw := aggregate(bHighR, DefaultConfig())
	if altRaw <= b.Raw {
		t.Fatalf("raising R should raise Raw; got raw=%v altRaw=%v", b.Raw, altRaw)
	}
}

func TestScoreEdgeRejectsNilInputs(t *testing.T) {
	_, _, _, err := ScoreEdge(ScoreEdgeInput{}, Deps{Graph: graph.New()}, DefaultConfig())
	if err == nil {
		t.Fatal("expected error on nil edge")
	}
	e := model.NewEdge("e", model.EdgeMemberOf, "a", "b")
	_, _, _, err = ScoreEdge(ScoreEdgeInput{Edge: e}, Deps{}, DefaultConfig())
	if err == nil {
		t.Fatal("expected error on nil graph")
	}
}
