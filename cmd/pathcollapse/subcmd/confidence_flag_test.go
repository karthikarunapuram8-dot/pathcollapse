package subcmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// makeCmd builds a throwaway Cobra command with a captured stderr for
// assertion. Returns the command, a buffer collecting stderr, and a flag
// pointer already registered via AddConfidenceFlag.
func makeCmd(t *testing.T) (*cobra.Command, *bytes.Buffer, *string) {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	stderr := &bytes.Buffer{}
	cmd.SetErr(stderr)
	var mode string
	AddConfidenceFlag(cmd, &mode)
	return cmd, stderr, &mode
}

func withTTYDetector(t *testing.T, fn func(io.Writer) bool) {
	t.Helper()
	prev := writerTTY
	writerTTY = fn
	t.Cleanup(func() { writerTTY = prev })
}

func TestResolveConfidence_OffReturnsNil(t *testing.T) {
	cmd, stderr, _ := makeCmd(t)
	opts, err := ResolveConfidence(cmd, "off", false)
	if err != nil {
		t.Fatal(err)
	}
	if opts != nil {
		t.Errorf("off mode should return nil, got %+v", opts)
	}
	if stderr.Len() != 0 {
		t.Errorf("off mode should not emit stderr, got %q", stderr.String())
	}
}

func TestResolveConfidence_OnReturnsConfigured(t *testing.T) {
	cmd, _, _ := makeCmd(t)
	opts, err := ResolveConfidence(cmd, "on", false)
	if err != nil {
		t.Fatal(err)
	}
	if opts == nil {
		t.Fatal("on mode must return non-nil options")
	}
	// Defaults threaded through.
	if opts.ScoringCfg.TargetCriticalityWeight == 0 {
		t.Error("expected default scoring config populated")
	}
	if opts.Config.BetaR == 0 {
		t.Error("expected default confidence config populated")
	}
	// Snapshots may or may not be set depending on whether the developer
	// running the tests has a local DB. Both outcomes are valid.
}

func TestResolveConfidence_InvalidValueReturnsError(t *testing.T) {
	cmd, _, _ := makeCmd(t)
	if _, err := ResolveConfidence(cmd, "maybe", false); err == nil {
		t.Fatal("expected error on invalid value")
	}
}

func TestResolveConfidence_CaseInsensitive(t *testing.T) {
	cmd, _, _ := makeCmd(t)
	if _, err := ResolveConfidence(cmd, "OFF", false); err != nil {
		t.Errorf("uppercase OFF should parse: %v", err)
	}
	if _, err := ResolveConfidence(cmd, "On", false); err != nil {
		t.Errorf("mixed-case On should parse: %v", err)
	}
}

func TestNoteColdStart_FormatIsStable(t *testing.T) {
	var buf bytes.Buffer
	noteColdStart(&buf, "no snapshot history")
	out := buf.String()
	for _, want := range []string{
		"INFO: confidence:",
		"no snapshot history",
		"T(e) using cold-start prior",
		"docs/confidence.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("cold-start note missing %q, got:\n%s", want, out)
		}
	}
	// Must be a single line so log collectors don't split it.
	if strings.Count(out, "\n") != 1 {
		t.Errorf("expected single newline, got %d in %q", strings.Count(out, "\n"), out)
	}
}

func TestAddConfidenceFlag_Default(t *testing.T) {
	cmd, _, mode := makeCmd(t)
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	if *mode != "on" {
		t.Errorf("expected default 'on', got %q", *mode)
	}
}

func TestAddConfidenceFlag_AcceptsOverride(t *testing.T) {
	cmd, _, mode := makeCmd(t)
	if err := cmd.ParseFlags([]string{"--confidence", "off"}); err != nil {
		t.Fatal(err)
	}
	if *mode != "off" {
		t.Errorf("expected 'off', got %q", *mode)
	}
}

func TestResolveConfidence_OnQuietSuppressesInfo(t *testing.T) {
	withTTYDetector(t, func(io.Writer) bool { return true })
	cmd, stderr, _ := makeCmd(t)
	if _, err := ResolveConfidence(cmd, "on", true); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected quiet mode to suppress info, got %q", stderr.String())
	}
}

func TestAddQuietFlag_DefaultFalse(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	var quiet bool
	AddQuietFlag(cmd, &quiet)
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	if quiet {
		t.Fatal("expected default quiet=false")
	}
}

func TestAddQuietFlag_AcceptsOverride(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	var quiet bool
	AddQuietFlag(cmd, &quiet)
	if err := cmd.ParseFlags([]string{"--quiet"}); err != nil {
		t.Fatal(err)
	}
	if !quiet {
		t.Fatal("expected --quiet to set quiet=true")
	}
}

func TestWriterTTY_NonTTYWriters(t *testing.T) {
	if writerTTY(&bytes.Buffer{}) {
		t.Fatal("bytes.Buffer must not be treated as a TTY")
	}
	f, err := os.CreateTemp(t.TempDir(), "stderr-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if writerTTY(f) {
		t.Fatal("regular file must not be treated as a TTY")
	}
}

func TestInfoWriter_SuppressionMatrix(t *testing.T) {
	stderr := &bytes.Buffer{}

	withTTYDetector(t, func(io.Writer) bool { return true })
	if got := infoWriter(stderr, true); got != io.Discard {
		t.Fatal("quiet mode should discard informational output")
	}

	withTTYDetector(t, func(io.Writer) bool { return false })
	if got := infoWriter(stderr, false); got != io.Discard {
		t.Fatal("non-TTY stderr should discard informational output")
	}

	withTTYDetector(t, func(io.Writer) bool { return true })
	if got := infoWriter(stderr, false); got != stderr {
		t.Fatal("TTY stderr without quiet should preserve informational output")
	}
}
