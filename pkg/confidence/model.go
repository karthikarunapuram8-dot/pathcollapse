// Package confidence computes a calibrated probability that a recommended
// control change will collapse the exposure it targets without regressing
// legitimate operations.
//
// See docs/confidence.md for the algorithm specification. The public entry
// point is ScoreEdge.
package confidence

// Breakdown is the per-factor decomposition of the confidence score.
// All fields are in [0, 1]. See docs/confidence.md §4.
type Breakdown struct {
	Evidence              float64 `json:"evidence"`               // E(e) — §4.1
	Robustness            float64 `json:"robustness"`             // R(e, G) — §4.2
	Safety                float64 `json:"safety"`                 // S(e, G) — §4.3
	TemporalStability     float64 `json:"temporal_stability"`     // T(e) — §4.4
	CoverageConcentration float64 `json:"coverage_concentration"` // K(c) — §4.5

	// Raw is the pre-calibration logistic of the log-odds sum.
	Raw float64 `json:"raw"`

	// Final is the post-calibration probability returned to callers.
	Final float64 `json:"final"`
}

// Regime describes how much labeled data the active Calibrator was trained on.
// Consumers use this to decide whether to surface the score prominently or
// gate it behind an "experimental" flag in the UI.
type Regime string

const (
	// RegimeColdStart — fewer than MinPartialLabels labeled outcomes. The
	// calibrator is the identity map; Final == Raw.
	RegimeColdStart Regime = "cold_start"

	// RegimePartial — between MinPartialLabels and MinCalibratedLabels.
	// Use with caution; report alongside shadow-mode comparisons.
	RegimePartial Regime = "partial"

	// RegimeCalibrated — at least MinCalibratedLabels labeled outcomes.
	// Final is expected to be well-calibrated by Brier/ECE metrics.
	RegimeCalibrated Regime = "calibrated"
)

// Label-count thresholds for regime classification. See docs/confidence.md §8.1.
const (
	MinPartialLabels    = 50
	MinCalibratedLabels = 500
)

// Config holds tunable aggregation weights and factor parameters. Defaults
// come from docs/confidence.md §5.1 and must be re-fit against labeled data
// before production use (see §9.2).
type Config struct {
	// Log-odds aggregation weights. Order: [bias, E, R, S, T, K].
	Beta0 float64
	BetaE float64
	BetaR float64
	BetaS float64
	BetaT float64
	BetaK float64

	// ClipEpsilon bounds each factor to [ε, 1-ε] before taking logit
	// to prevent divergence on degenerate inputs. §5.1.
	ClipEpsilon float64

	// EvidenceSourceLambda controls corroboration decay in E(e). §4.1.
	// Higher = faster saturation with additional sources.
	EvidenceSourceLambda float64

	// RobustnessThreshold (τ) is the risk-score level below which a
	// residual path is considered "collapsed." §2.
	RobustnessThreshold float64

	// MaxResidualDepth caps path-search depth in R(e, G). §4.2.
	MaxResidualDepth int

	// TemporalHalflifeDays controls age saturation in T(e). §4.4.
	TemporalHalflifeDays float64

	// TemporalSnapshotWindow is the number of recent snapshots consulted
	// for edge-presence rate in T(e). §4.4.
	TemporalSnapshotWindow int
}

// DefaultConfig returns the prior-calibration weights from docs/confidence.md §5.1.
//
// These are informed priors, not learned weights. Deployments should run in
// shadow mode for 2–4 weeks and refit via calibration before trusting the
// score over analyst judgment.
func DefaultConfig() Config {
	return Config{
		Beta0: 0.0,
		BetaE: 1.2,
		BetaR: 1.6,
		BetaS: 1.1,
		BetaT: 0.6,
		BetaK: 0.8,
		// ε = 0.05 caps each factor's logit contribution to ±2.94.
		// Tighter (e.g. 1e-3 → ±6.9) lets a single saturating factor like
		// K (unique coverage) or a trivially high E dominate the sum; see
		// TestScoreEdgeEndToEndDiamondGraph for the regression this prevents.
		ClipEpsilon:            0.05,
		EvidenceSourceLambda:   0.7,
		RobustnessThreshold:    0.6,
		MaxResidualDepth:       5,
		TemporalHalflifeDays:   30.0,
		TemporalSnapshotWindow: 8,
	}
}
