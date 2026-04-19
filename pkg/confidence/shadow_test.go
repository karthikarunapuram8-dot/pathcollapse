package confidence

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func mkBreakdown() Breakdown {
	return Breakdown{
		Evidence:              0.40,
		Robustness:            0.95,
		Safety:                0.50,
		TemporalStability:     0.30,
		CoverageConcentration: 1.00,
		Raw:                   0.78,
		Final:                 0.78,
	}
}

func TestShadowEntry_IsLabeled(t *testing.T) {
	collapsed := true
	e := ShadowEntry{}
	if e.IsLabeled() {
		t.Fatal("unlabeled entry must not report labeled")
	}
	e.ObservedCollapsed = &collapsed
	if !e.IsLabeled() {
		t.Fatal("entry with ObservedCollapsed must report labeled")
	}
}

func TestShadowEntry_Label(t *testing.T) {
	yes, no := true, false

	tests := []struct {
		name       string
		collapsed  *bool
		regression *bool
		wantY      float64
		wantOk     bool
	}{
		{"unlabeled", nil, nil, 0, false},
		{"collapsed no regression (explicit)", &yes, &no, 1, true},
		{"collapsed no regression (missing)", &yes, nil, 1, true},
		{"collapsed with regression", &yes, &yes, 0, true},
		{"not collapsed no regression", &no, &no, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := ShadowEntry{ObservedCollapsed: tc.collapsed, ObservedRegression: tc.regression}
			got, ok := e.Label()
			if ok != tc.wantOk || got != tc.wantY {
				t.Errorf("got (%v, %v); want (%v, %v)", got, ok, tc.wantY, tc.wantOk)
			}
		})
	}
}

func TestAppendAndRead_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shadow.jsonl")

	e1 := NewShadowEntry("e-1", "alice", "admins", "member_of", 0.91, mkBreakdown(), time.Now().UTC())
	e2 := NewShadowEntry("e-2", "bob", "admins", "member_of", 0.72, mkBreakdown(), time.Now().UTC())
	collapsed := true
	e2.ObservedCollapsed = &collapsed

	for _, e := range []ShadowEntry{e1, e2} {
		if err := AppendShadowEntry(path, e); err != nil {
			t.Fatal(err)
		}
	}

	got, stats, err := ReadShadowLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalLines != 2 || stats.Parsed != 2 || stats.Malformed != 0 {
		t.Fatalf("stats = %+v", stats)
	}
	if stats.Labeled != 1 {
		t.Fatalf("expected 1 labeled entry, got %d", stats.Labeled)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].EdgeID != "e-1" || got[1].EdgeID != "e-2" {
		t.Errorf("order preservation broken: %q %q", got[0].EdgeID, got[1].EdgeID)
	}
	if got[1].ObservedCollapsed == nil || *got[1].ObservedCollapsed != true {
		t.Error("label round-trip broken")
	}
}

func TestReadShadowLog_MissingFileIsNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.jsonl")
	entries, stats, err := ReadShadowLog(path)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if len(entries) != 0 || stats.TotalLines != 0 {
		t.Errorf("expected empty result, got entries=%d lines=%d", len(entries), stats.TotalLines)
	}
}

func TestReadShadowLog_SkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shadow.jsonl")

	contents := `{"edge_id":"e-1","raw":0.5,"ts":"2026-04-18T18:00:00Z"}
not-valid-json
{"edge_id":"e-2","raw":0.6,"ts":"2026-04-18T18:00:00Z"}

{"also_broken":
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	entries, stats, err := ReadShadowLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Parsed != 2 {
		t.Errorf("expected 2 parsed, got %d", stats.Parsed)
	}
	if stats.Malformed != 2 {
		t.Errorf("expected 2 malformed, got %d", stats.Malformed)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestDefaultShadowLogPath_ShapeLooksRight(t *testing.T) {
	p, err := DefaultShadowLogPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "shadow.jsonl" {
		t.Errorf("unexpected base: %q", p)
	}
	if filepath.Base(filepath.Dir(p)) != ".pathcollapse" {
		t.Errorf("unexpected parent dir: %q", p)
	}
}
