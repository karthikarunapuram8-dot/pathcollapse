package subcmd

import (
	"bytes"
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

func TestResolveConfidence_OffReturnsNil(t *testing.T) {
	cmd, stderr, _ := makeCmd(t)
	opts, err := ResolveConfidence(cmd, "off")
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
	opts, err := ResolveConfidence(cmd, "on")
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
	if _, err := ResolveConfidence(cmd, "maybe"); err == nil {
		t.Fatal("expected error on invalid value")
	}
}

func TestResolveConfidence_CaseInsensitive(t *testing.T) {
	cmd, _, _ := makeCmd(t)
	if _, err := ResolveConfidence(cmd, "OFF"); err != nil {
		t.Errorf("uppercase OFF should parse: %v", err)
	}
	if _, err := ResolveConfidence(cmd, "On"); err != nil {
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
