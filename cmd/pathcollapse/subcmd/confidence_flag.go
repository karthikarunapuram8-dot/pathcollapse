package subcmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/karunapuram/pathcollapse/pkg/confidence"
	"github.com/karunapuram/pathcollapse/pkg/controls"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
	"github.com/karunapuram/pathcollapse/pkg/snapshot"
)

// confidenceModeOn / confidenceModeOff are the accepted values for the
// --confidence flag. Everything else is rejected to keep the surface tight.
const (
	confidenceModeOn  = "on"
	confidenceModeOff = "off"
)

// AddConfidenceFlag registers --confidence on cmd. The pointer returned
// holds the chosen value after Cobra parses args.
func AddConfidenceFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVar(dest, "confidence", confidenceModeOn,
		"Calibrated breakpoint confidence: on|off. See docs/confidence.md")
}

// ResolveConfidence turns the flag value into *controls.ConfidenceOptions
// ready to assign to OptimizerConfig.Confidence.
//
// Returns (nil, nil) when the flag is "off" — callers fall back to the
// legacy 0.85 value.
//
// When the flag is "on":
//   - Attempts to attach snapshot-backed Presence if ~/.pathcollapse/snapshots.db
//     exists and contains ≥2 snapshots.
//   - Emits a single-line INFO note to stderr when falling back to cold-start
//     (no DB, <2 snapshots, or load failure) so operators know the T(e)
//     factor is using the non-informative prior.
//
// Invalid flag values return a typed error that Cobra surfaces to the user.
func ResolveConfidence(cmd *cobra.Command, mode string) (*controls.ConfidenceOptions, error) {
	switch strings.ToLower(mode) {
	case confidenceModeOff:
		return nil, nil
	case confidenceModeOn:
		return buildConfidenceOptions(cmd.ErrOrStderr()), nil
	default:
		return nil, fmt.Errorf("invalid --confidence value %q: expected %q or %q",
			mode, confidenceModeOn, confidenceModeOff)
	}
}

// buildConfidenceOptions assembles the ConfidenceOptions, attaching a
// snapshot-backed Presence when history is available. Errors opening the
// DB are not propagated — we degrade to cold-start with a stderr note
// rather than block the user's workflow on an optional enrichment path.
func buildConfidenceOptions(stderr io.Writer) *controls.ConfidenceOptions {
	opts := &controls.ConfidenceOptions{
		ScoringCfg: scoring.DefaultConfig(),
		Config:     confidence.DefaultConfig(),
	}

	dbPath, err := snapshot.DefaultDBPath()
	if err != nil {
		noteColdStart(stderr, "cannot locate home directory for snapshot DB")
		return opts
	}
	if _, err := os.Stat(dbPath); err != nil {
		noteColdStart(stderr, "no snapshot history at "+dbPath)
		return opts
	}

	store, err := snapshot.Open(dbPath)
	if err != nil {
		noteColdStart(stderr, "snapshot DB unreadable: "+err.Error())
		return opts
	}
	// NOTE: the store intentionally leaks for the lifetime of the command.
	// The Presence index holds unmarshalled graphs in memory and does not
	// need an open handle after construction; closing immediately is
	// correct but verbose given the single-command CLI lifecycle.
	defer store.Close()

	window := confidence.DefaultConfig().TemporalSnapshotWindow
	presence, err := snapshot.NewPresence(store, window)
	if err != nil {
		noteColdStart(stderr, "failed to build presence index: "+err.Error())
		return opts
	}

	if presence.Window() < 2 {
		noteColdStart(stderr,
			fmt.Sprintf("only %d snapshot(s) indexed, need ≥2 for T(e)", presence.Window()))
		return opts
	}

	opts.Snapshots = presence
	return opts
}

// noteColdStart writes a single stderr line explaining the degradation.
// Kept under 120 chars so it reads cleanly on a terminal without wrapping.
func noteColdStart(w io.Writer, reason string) {
	fmt.Fprintf(w, "INFO: confidence: %s — T(e) using cold-start prior (see docs/confidence.md §4.4)\n", reason)
}
