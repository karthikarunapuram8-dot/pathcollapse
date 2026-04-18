package confidence

import (
	"fmt"
	"time"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/scoring"
)

// ScoreEdgeInput bundles the per-edge inputs Score needs. The caller is
// expected to build this once per recommendation from the optimizer's
// existing state; see controls.Optimize integration notes in
// docs/confidence.md §8.
type ScoreEdgeInput struct {
	// Edge being severed.
	Edge *model.Edge

	// AffectedPaths is the set of paths the recommendation claims to
	// collapse. Typically rec.AffectedPaths from controls.ControlRecommendation.
	AffectedPaths []graph.Path

	// CandidateIndex groups interchangeable candidates for K(c). Nil is
	// permitted and treated as "unique coverage" (K = 1).
	CandidateIndex *CandidateIndex
}

// Deps bundles external dependencies.
type Deps struct {
	Graph      *graph.Graph
	ScoringCfg scoring.ScoringConfig
	Snapshots  SnapshotProvider // may be nil in cold start
	Calibrator Calibrator       // nil → IdentityCalibrator
	Now        time.Time        // defaults to time.Now().UTC() when zero
}

// ScoreEdge runs the five-factor decomposition, aggregates in log-odds
// space, and applies calibration. It never mutates its inputs.
//
// Returns the calibrated probability, the full breakdown, the calibrator's
// regime, and any error that stopped computation.
//
// Error cases are narrow: nil edge, nil graph. Missing-but-recoverable
// inputs (no snapshots, no candidate index, empty covered-paths) are handled
// with documented fallbacks, not errors — see individual factor docstrings.
func ScoreEdge(in ScoreEdgeInput, deps Deps, cfg Config) (float64, Breakdown, Regime, error) {
	if in.Edge == nil {
		return 0, Breakdown{}, RegimeColdStart, fmt.Errorf("confidence: ScoreEdge requires a non-nil Edge")
	}
	if deps.Graph == nil {
		return 0, Breakdown{}, RegimeColdStart, fmt.Errorf("confidence: ScoreEdge requires a non-nil Graph")
	}

	now := deps.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	cal := deps.Calibrator
	if cal == nil {
		cal = IdentityCalibrator{}
	}

	b := Breakdown{
		Evidence:              evidenceQuality(in.Edge, cfg),
		Robustness:            structuralRobustness(in.Edge, in.AffectedPaths, deps.Graph, deps.ScoringCfg, cfg),
		Safety:                operationalSafety(in.Edge, deps.Graph),
		TemporalStability:     temporalStability(in.Edge, deps.Snapshots, cfg, now),
		CoverageConcentration: coverageConcentration(in.Edge.ID, in.CandidateIndex),
	}
	b.Raw = aggregate(b, cfg)
	b.Final = cal.Apply(b.Raw)

	return b.Final, b, cal.Regime(), nil
}

// BuildCandidateIndex is a convenience constructor that mirrors the scan in
// pkg/controls/controls.go:92–101. Callers that already have the candidate
// map can call CandidateIndex.Register directly and avoid walking paths
// twice.
func BuildCandidateIndex(pathsByEdge map[string][]int) *CandidateIndex {
	ci := NewCandidateIndex()
	for edgeID, idxs := range pathsByEdge {
		sorted := make([]int, len(idxs))
		copy(sorted, idxs)
		sortInts(sorted)
		ci.Register(edgeID, sorted)
	}
	return ci
}

// sortInts is a dependency-free ascending sort used only by BuildCandidateIndex.
func sortInts(a []int) {
	// Insertion sort is fine here — path-idx lists are small (O(paths/edge)).
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}
