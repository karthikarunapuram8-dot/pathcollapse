package confidence

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// RefitResult is the output of Refit. It bundles the fitted calibrator with
// the diagnostic metrics a caller needs to decide whether to adopt the new
// calibration. See docs/confidence.md §7.
type RefitResult struct {
	// LabeledCount is the number of shadow entries that had IsLabeled()==true.
	LabeledCount int `json:"labeled_count"`

	// Regime is the calibrator's training-size regime.
	Regime Regime `json:"regime"`

	// Brier is the mean squared error between predicted and observed labels.
	// Lower is better. Baseline (static 0.85): (0.85 - meanY)^2 + Var(Y).
	Brier float64 `json:"brier"`

	// BrierBaseline is the Brier score a constant-0.85 predictor would produce
	// on this same data, for head-to-head comparison.
	BrierBaseline float64 `json:"brier_baseline"`

	// ECE is the Expected Calibration Error: weighted mean over deciles of
	// |mean_predicted - mean_observed|. Target < 0.05 at ≥500 labels.
	ECE float64 `json:"ece"`

	// Buckets is the reliability diagram: predicted-vs-observed by decile.
	Buckets []ReliabilityBucket `json:"buckets"`

	// Calibrator is the fitted isotonic model. Serialize via SaveCalibrator.
	Calibrator *IsotonicCalibrator `json:"-"`
}

// ReliabilityBucket is one row of the reliability diagram.
type ReliabilityBucket struct {
	Min           float64 `json:"min"`
	Max           float64 `json:"max"`
	Count         int     `json:"count"`
	MeanPredicted float64 `json:"mean_predicted"`
	MeanObserved  float64 `json:"mean_observed"`
}

// Refit fits an IsotonicCalibrator on the labeled entries and computes
// calibration diagnostics. Returns an error only if the input cannot be
// turned into at least one labeled outcome.
func Refit(entries []ShadowEntry) (*RefitResult, error) {
	outcomes := make([]LabeledOutcome, 0, len(entries))
	for _, e := range entries {
		y, ok := e.Label()
		if !ok {
			continue
		}
		outcomes = append(outcomes, LabeledOutcome{Raw: e.Raw, Y: y})
	}
	if len(outcomes) == 0 {
		return nil, fmt.Errorf("confidence: refit requires at least one labeled outcome, got %d entries with labels", 0)
	}

	ic := NewIsotonicCalibrator()
	if err := ic.Fit(outcomes); err != nil {
		return nil, fmt.Errorf("confidence: fit isotonic: %w", err)
	}

	brier := computeBrier(outcomes, ic)
	brierBase := computeBrierBaseline(outcomes, 0.85)
	buckets := computeReliabilityBuckets(outcomes, ic, 10)
	ece := computeECE(buckets, len(outcomes))

	return &RefitResult{
		LabeledCount:  len(outcomes),
		Regime:        ic.Regime(),
		Brier:         brier,
		BrierBaseline: brierBase,
		ECE:           ece,
		Buckets:       buckets,
		Calibrator:    ic,
	}, nil
}

// ── Metrics ────────────────────────────────────────────────────────────────

func computeBrier(outcomes []LabeledOutcome, cal Calibrator) float64 {
	var sum float64
	for _, o := range outcomes {
		p := cal.Apply(o.Raw)
		d := p - o.Y
		sum += d * d
	}
	return sum / float64(len(outcomes))
}

func computeBrierBaseline(outcomes []LabeledOutcome, constant float64) float64 {
	var sum float64
	for _, o := range outcomes {
		d := constant - o.Y
		sum += d * d
	}
	return sum / float64(len(outcomes))
}

// computeReliabilityBuckets groups outcomes by predicted-probability decile
// and reports per-bucket mean predicted vs mean observed. Empty buckets are
// dropped from the result.
func computeReliabilityBuckets(outcomes []LabeledOutcome, cal Calibrator, nBuckets int) []ReliabilityBucket {
	type datum struct{ p, y float64 }
	data := make([]datum, len(outcomes))
	for i, o := range outcomes {
		data[i] = datum{p: cal.Apply(o.Raw), y: o.Y}
	}
	sort.Slice(data, func(i, j int) bool { return data[i].p < data[j].p })

	buckets := make([]ReliabilityBucket, 0, nBuckets)
	width := 1.0 / float64(nBuckets)
	for i := 0; i < nBuckets; i++ {
		lo := float64(i) * width
		hi := lo + width
		if i == nBuckets-1 {
			hi = 1.0 + 1e-9 // include exactly-1.0 in the last bucket
		}
		var sumP, sumY float64
		count := 0
		for _, d := range data {
			if d.p >= lo && d.p < hi {
				sumP += d.p
				sumY += d.y
				count++
			}
		}
		if count == 0 {
			continue
		}
		buckets = append(buckets, ReliabilityBucket{
			Min:           lo,
			Max:           math.Min(hi, 1.0),
			Count:         count,
			MeanPredicted: sumP / float64(count),
			MeanObserved:  sumY / float64(count),
		})
	}
	return buckets
}

// computeECE is Σ (bucket.Count / N) * |mean_pred - mean_obs|.
func computeECE(buckets []ReliabilityBucket, total int) float64 {
	if total == 0 {
		return 0
	}
	var sum float64
	for _, b := range buckets {
		w := float64(b.Count) / float64(total)
		sum += w * math.Abs(b.MeanPredicted-b.MeanObserved)
	}
	return sum
}

// ── Persistence ────────────────────────────────────────────────────────────

// calibratorFile is the on-disk representation of a fitted IsotonicCalibrator.
type calibratorFile struct {
	Version         int                 `json:"version"`
	Breakpoints     []float64           `json:"breakpoints"`
	Values          []float64           `json:"values"`
	NTrainingLabels int                 `json:"n_training_labels"`
	Regime          Regime              `json:"regime"`
	FitTime         time.Time           `json:"fit_time"`
	Metrics         calibratorFileStats `json:"metrics,omitempty"`
}

type calibratorFileStats struct {
	Brier         float64 `json:"brier"`
	BrierBaseline float64 `json:"brier_baseline"`
	ECE           float64 `json:"ece"`
}

const calibratorFileVersion = 1

// DefaultCalibratorPath returns ~/.pathcollapse/calibrator.json.
func DefaultCalibratorPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("confidence: locate home dir: %w", err)
	}
	return filepath.Join(home, ".pathcollapse", "calibrator.json"), nil
}

// SaveCalibrator persists r's fitted calibrator + diagnostic metadata to
// path. The parent directory is created if missing.
func SaveCalibrator(path string, r *RefitResult) error {
	if r == nil || r.Calibrator == nil {
		return fmt.Errorf("confidence: SaveCalibrator requires a non-nil fitted RefitResult")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("confidence: create calibrator dir: %w", err)
	}
	cf := calibratorFile{
		Version:         calibratorFileVersion,
		Breakpoints:     append([]float64(nil), r.Calibrator.breakpoints...),
		Values:          append([]float64(nil), r.Calibrator.values...),
		NTrainingLabels: r.Calibrator.n,
		Regime:          r.Regime,
		FitTime:         time.Now().UTC(),
		Metrics: calibratorFileStats{
			Brier:         r.Brier,
			BrierBaseline: r.BrierBaseline,
			ECE:           r.ECE,
		},
	}
	b, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("confidence: marshal calibrator: %w", err)
	}
	// Atomic-replace: write to a temp file and rename. Prevents a partial
	// write from corrupting the calibrator that `--confidence on` loads.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("confidence: write calibrator: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("confidence: rename calibrator: %w", err)
	}
	return nil
}

// LoadCalibrator reads a previously-saved calibrator. Returns
// (nil, nil) when the file doesn't exist — callers interpret as cold-start
// and use IdentityCalibrator. A corrupt file returns an error so the
// caller can decide to fail closed or warn and fall back.
func LoadCalibrator(path string) (Calibrator, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("confidence: read calibrator: %w", err)
	}
	var cf calibratorFile
	if err := json.Unmarshal(b, &cf); err != nil {
		return nil, fmt.Errorf("confidence: parse calibrator: %w", err)
	}
	if cf.Version != calibratorFileVersion {
		return nil, fmt.Errorf("confidence: calibrator file version %d unsupported (expected %d)", cf.Version, calibratorFileVersion)
	}
	if len(cf.Breakpoints) != len(cf.Values) {
		return nil, fmt.Errorf("confidence: calibrator has mismatched breakpoints (%d) and values (%d)", len(cf.Breakpoints), len(cf.Values))
	}
	ic := &IsotonicCalibrator{
		breakpoints: cf.Breakpoints,
		values:      cf.Values,
		n:           cf.NTrainingLabels,
	}
	return ic, nil
}
