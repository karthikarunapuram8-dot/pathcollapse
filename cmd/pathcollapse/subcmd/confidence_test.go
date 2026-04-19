package subcmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/confidence"
)

func boolp(v bool) *bool { return &v }

func TestConfidenceStatus_NoDataYet(t *testing.T) {
	cmd := newConfidenceStatusCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)

	logPath := filepath.Join(t.TempDir(), "shadow.jsonl")
	calPath := filepath.Join(t.TempDir(), "calibrator.json")
	cmd.SetArgs([]string{"--log", logPath, "--calibrator", calPath})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"Shadow-mode collection",
		"labeled entries:     0",
		"to partial (50):     0/50 (50 remaining)",
		"status:              none saved yet",
		"cold-start / identity calibrator remains active",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q:\n%s", want, got)
		}
	}
}

func TestConfidenceStatus_WithSavedCalibrator(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "shadow.jsonl")
	calPath := filepath.Join(t.TempDir(), "calibrator.json")

	// Three parsed entries, two labeled.
	if err := confidence.AppendShadowEntry(logPath, confidence.ShadowEntry{
		EdgeID:            "e1",
		Raw:               0.2,
		ObservedCollapsed: boolp(false),
	}); err != nil {
		t.Fatal(err)
	}
	if err := confidence.AppendShadowEntry(logPath, confidence.ShadowEntry{
		EdgeID:            "e2",
		Raw:               0.8,
		ObservedCollapsed: boolp(true),
	}); err != nil {
		t.Fatal(err)
	}
	if err := confidence.AppendShadowEntry(logPath, confidence.ShadowEntry{
		EdgeID: "e3",
		Raw:    0.5,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := confidence.Refit([]confidence.ShadowEntry{
		{EdgeID: "e1", Raw: 0.2, ObservedCollapsed: boolp(false)},
		{EdgeID: "e2", Raw: 0.8, ObservedCollapsed: boolp(true)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := confidence.SaveCalibrator(calPath, result); err != nil {
		t.Fatal(err)
	}

	cmd := newConfidenceStatusCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"--log", logPath, "--calibrator", calPath})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"parsed entries:      3",
		"labeled entries:     2",
		"unlabeled entries:   1",
		"status:              present",
		"training labels:     2",
		"regime:              cold_start",
		"auto-load:           will be used by 'breakpoints/report --confidence on'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q:\n%s", want, got)
		}
	}
}
