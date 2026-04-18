package confidence

import (
	"fmt"
	"sort"
)

// Calibrator maps a raw aggregated probability C_raw to a calibrated final
// probability C. In cold-start deployments it is the identity; once enough
// labeled outcomes are collected it is refit via isotonic regression.
//
// See docs/confidence.md §5.2.
type Calibrator interface {
	// Apply maps C_raw ∈ [0, 1] to the calibrated final probability.
	Apply(raw float64) float64

	// Regime reports how many labeled outcomes this calibrator was fit
	// against, bucketed into cold_start / partial / calibrated.
	Regime() Regime
}

// IdentityCalibrator is the cold-start default: Apply(x) == x.
type IdentityCalibrator struct{}

func (IdentityCalibrator) Apply(raw float64) float64 { return raw }
func (IdentityCalibrator) Regime() Regime            { return RegimeColdStart }

// LabeledOutcome is one training example: the raw score the model predicted,
// and the observed outcome (1 = recommendation succeeded, 0 = did not).
//
// Y is defined in docs/confidence.md §2 as collapse ∧ no_regression. During
// bootstrap it may be collapse-only (see §9.1 limitation).
type LabeledOutcome struct {
	Raw float64
	Y   float64 // {0, 1}
}

// IsotonicCalibrator is a monotone step function fit from labeled outcomes
// via the Pool-Adjacent-Violators algorithm. See docs/confidence.md §5.2.
//
// After Fit, Apply performs a binary search over the breakpoints and returns
// the step value for the bucket containing raw.
type IsotonicCalibrator struct {
	// breakpoints is sorted ascending; values[i] is the calibrated output
	// for raw scores in [breakpoints[i], breakpoints[i+1]).
	breakpoints []float64
	values      []float64
	n           int // number of training examples
}

// NewIsotonicCalibrator returns a calibrator that defers to the identity
// until Fit is called.
func NewIsotonicCalibrator() *IsotonicCalibrator {
	return &IsotonicCalibrator{}
}

// Fit trains the calibrator on labeled outcomes. Returns an error if the
// input is empty; returns nil for any non-empty input (cold-start/partial
// regimes still fit cleanly, they are just under-determined).
func (ic *IsotonicCalibrator) Fit(outcomes []LabeledOutcome) error {
	if len(outcomes) == 0 {
		return fmt.Errorf("calibration: refusing to fit on empty outcome set")
	}

	// Sort ascending by Raw so PAV can walk in order.
	sorted := make([]LabeledOutcome, len(outcomes))
	copy(sorted, outcomes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Raw < sorted[j].Raw })

	// Pool-Adjacent-Violators: initialise blocks of size 1, merge while
	// adjacent means violate monotonicity.
	type block struct {
		sum    float64
		count  int
		rawMin float64 // smallest raw in the block
	}
	blocks := make([]block, len(sorted))
	for i, o := range sorted {
		blocks[i] = block{sum: o.Y, count: 1, rawMin: o.Raw}
	}

	for {
		merged := false
		for i := 0; i < len(blocks)-1; i++ {
			ma := blocks[i].sum / float64(blocks[i].count)
			mb := blocks[i+1].sum / float64(blocks[i+1].count)
			if ma > mb {
				// Violation — merge i+1 into i.
				blocks[i].sum += blocks[i+1].sum
				blocks[i].count += blocks[i+1].count
				blocks = append(blocks[:i+1], blocks[i+2:]...)
				merged = true
				break
			}
		}
		if !merged {
			break
		}
	}

	ic.breakpoints = make([]float64, len(blocks))
	ic.values = make([]float64, len(blocks))
	for i, b := range blocks {
		ic.breakpoints[i] = b.rawMin
		ic.values[i] = b.sum / float64(b.count)
	}
	ic.n = len(outcomes)
	return nil
}

// Apply returns the calibrated probability for raw. Before Fit, returns raw
// unchanged.
func (ic *IsotonicCalibrator) Apply(raw float64) float64 {
	if len(ic.breakpoints) == 0 {
		return raw
	}
	// Largest breakpoint ≤ raw.
	idx := sort.SearchFloat64s(ic.breakpoints, raw)
	if idx == 0 {
		return ic.values[0]
	}
	if idx >= len(ic.breakpoints) {
		return ic.values[len(ic.values)-1]
	}
	// SearchFloat64s returns the first index > raw; we want ≤ raw.
	if ic.breakpoints[idx] > raw {
		idx--
	}
	return ic.values[idx]
}

// Regime buckets the training set size.
func (ic *IsotonicCalibrator) Regime() Regime {
	switch {
	case ic.n < MinPartialLabels:
		return RegimeColdStart
	case ic.n < MinCalibratedLabels:
		return RegimePartial
	default:
		return RegimeCalibrated
	}
}
