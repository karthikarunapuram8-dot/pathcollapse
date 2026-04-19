package confidence

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// mkLabeled builds a ShadowEntry with the given raw score and binary label.
func mkLabeled(raw float64, y float64) ShadowEntry {
	collapsed := y == 1
	return ShadowEntry{
		EdgeID:            "e",
		Raw:               raw,
		ObservedCollapsed: &collapsed,
	}
}

// ── Refit ──────────────────────────────────────────────────────────────────

func TestRefit_RejectsEmpty(t *testing.T) {
	if _, err := Refit(nil); err == nil {
		t.Fatal("expected error on empty input")
	}
	if _, err := Refit([]ShadowEntry{{EdgeID: "unlabeled"}}); err == nil {
		t.Fatal("expected error when all entries are unlabeled")
	}
}

func TestRefit_BrierBeatsBaselineOnPerfectSeparation(t *testing.T) {
	// Raw scores perfectly separate collapsed from not-collapsed.
	// Isotonic regression should drive Brier toward 0.
	var entries []ShadowEntry
	for i := 0; i < 20; i++ {
		entries = append(entries, mkLabeled(0.1+float64(i)*0.01, 0))
	}
	for i := 0; i < 20; i++ {
		entries = append(entries, mkLabeled(0.6+float64(i)*0.01, 1))
	}

	r, err := Refit(entries)
	if err != nil {
		t.Fatal(err)
	}
	if r.LabeledCount != 40 {
		t.Errorf("expected 40 labeled, got %d", r.LabeledCount)
	}
	// Brier on perfect-separation isotonic fit approaches 0.
	if r.Brier > 0.1 {
		t.Errorf("expected Brier < 0.1 on separable data, got %v", r.Brier)
	}
	if r.Brier >= r.BrierBaseline {
		t.Errorf("expected Brier (%v) < BrierBaseline (%v)", r.Brier, r.BrierBaseline)
	}
}

func TestRefit_ECE_IsSmallOnCleanSignal(t *testing.T) {
	// Synthesize a dataset where p ≈ raw. Isotonic should reproduce this
	// and ECE should be small.
	var entries []ShadowEntry
	// For each raw bucket, generate outcomes whose mean-Y ≈ raw.
	for _, raw := range []float64{0.1, 0.3, 0.5, 0.7, 0.9} {
		positives := int(raw * 10)
		negatives := 10 - positives
		for i := 0; i < positives; i++ {
			entries = append(entries, mkLabeled(raw, 1))
		}
		for i := 0; i < negatives; i++ {
			entries = append(entries, mkLabeled(raw, 0))
		}
	}

	r, err := Refit(entries)
	if err != nil {
		t.Fatal(err)
	}
	if r.ECE > 0.15 {
		t.Errorf("expected ECE ≤ 0.15 on clean signal, got %v (buckets=%+v)", r.ECE, r.Buckets)
	}
}

func TestRefit_CountsLabeledOnly(t *testing.T) {
	entries := []ShadowEntry{
		mkLabeled(0.5, 1),
		{EdgeID: "unlabeled-1", Raw: 0.5},
		mkLabeled(0.8, 1),
		{EdgeID: "unlabeled-2", Raw: 0.9},
	}
	r, err := Refit(entries)
	if err != nil {
		t.Fatal(err)
	}
	if r.LabeledCount != 2 {
		t.Errorf("expected 2 labeled, got %d", r.LabeledCount)
	}
}

// ── Metric units ───────────────────────────────────────────────────────────

func TestComputeBrier(t *testing.T) {
	outcomes := []LabeledOutcome{{Raw: 0.5, Y: 1}, {Raw: 0.5, Y: 0}}
	// Identity calibrator: both predictions 0.5. Brier = (0.5-1)^2/2 + (0.5-0)^2/2 = 0.25
	got := computeBrier(outcomes, IdentityCalibrator{})
	if math.Abs(got-0.25) > 1e-9 {
		t.Errorf("Brier(0.5, 1/0) got %v want 0.25", got)
	}
}

func TestComputeBrierBaseline_Constant(t *testing.T) {
	outcomes := []LabeledOutcome{{Y: 1}, {Y: 1}, {Y: 0}, {Y: 0}}
	// Constant=0.85, Y∈{1,1,0,0}: brier = ((0.85-1)^2*2 + (0.85)^2*2)/4 ≈ 0.372
	got := computeBrierBaseline(outcomes, 0.85)
	want := ((0.15*0.15)*2 + (0.85*0.85)*2) / 4
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("BrierBaseline got %v want %v", got, want)
	}
}

func TestComputeReliabilityBuckets_DropsEmpty(t *testing.T) {
	outcomes := []LabeledOutcome{{Raw: 0.05, Y: 0}, {Raw: 0.95, Y: 1}}
	buckets := computeReliabilityBuckets(outcomes, IdentityCalibrator{}, 10)
	if len(buckets) != 2 {
		t.Errorf("expected 2 non-empty buckets, got %d: %+v", len(buckets), buckets)
	}
}

// ── Persistence ────────────────────────────────────────────────────────────

func TestSaveLoadCalibrator_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "calibrator.json")

	// Build a trained calibrator via Refit.
	var entries []ShadowEntry
	for i := 0; i < 20; i++ {
		entries = append(entries, mkLabeled(0.1+float64(i)*0.04, float64(i%2)))
	}
	r, err := Refit(entries)
	if err != nil {
		t.Fatal(err)
	}

	if err := SaveCalibrator(path, r); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadCalibrator(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil calibrator")
	}

	// Loaded calibrator should produce the same Apply() output as the original.
	for _, raw := range []float64{0.1, 0.3, 0.5, 0.7, 0.9} {
		want := r.Calibrator.Apply(raw)
		got := loaded.Apply(raw)
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("Apply(%v): got %v want %v", raw, got, want)
		}
	}
}

func TestLoadCalibrator_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")
	cal, err := LoadCalibrator(path)
	if err != nil {
		t.Fatalf("expected nil error on missing file, got %v", err)
	}
	if cal != nil {
		t.Error("expected nil calibrator for missing file (cold-start signal)")
	}
}

func TestLoadCalibrator_RejectsBadVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"version": 99, "breakpoints": [], "values": []}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCalibrator(path); err == nil {
		t.Fatal("expected error on unsupported version")
	}
}

func TestLoadCalibrator_RejectsMismatchedArrays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"version": 1, "breakpoints": [0.1, 0.5], "values": [0.2]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCalibrator(path); err == nil {
		t.Fatal("expected error on mismatched arrays")
	}
}

func TestSaveCalibrator_AtomicReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cal.json")

	// First save.
	r1, err := Refit([]ShadowEntry{mkLabeled(0.3, 0), mkLabeled(0.7, 1)})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveCalibrator(path, r1); err != nil {
		t.Fatal(err)
	}
	info1, _ := os.Stat(path)

	// Second save should replace, not error. Temp file must not linger.
	r2, err := Refit([]ShadowEntry{mkLabeled(0.2, 0), mkLabeled(0.8, 1)})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveCalibrator(path, r2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file leaked: %v", err)
	}
	info2, _ := os.Stat(path)
	if info2.ModTime().Before(info1.ModTime()) {
		t.Error("second save did not update file")
	}
}
