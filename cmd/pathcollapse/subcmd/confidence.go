package subcmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/confidence"
)

// NewConfidenceCmd returns the `confidence` subcommand group. Currently hosts
// `refit`; more subcommands (e.g. `status`, `export`) may land here.
func NewConfidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "confidence",
		Short: "Manage the calibrated recommendation confidence system",
		Long: `Commands for working with the calibrated confidence system:

  refit    Fit an isotonic calibrator from a shadow-mode JSONL log and
           persist it so subsequent 'breakpoints --confidence on' runs
           produce calibrated (rather than cold-start) scores.

See docs/confidence.md for the algorithm.`,
	}
	cmd.AddCommand(newConfidenceRefitCmd())
	return cmd
}

func newConfidenceRefitCmd() *cobra.Command {
	var (
		logPath        string
		outputPath     string
		quiet          bool
		requireMinimum int
	)

	cmd := &cobra.Command{
		Use:   "refit",
		Short: "Fit an isotonic calibrator from the shadow-mode JSONL log",
		Long: `Reads the shadow-mode log (default: ~/.pathcollapse/shadow.jsonl),
extracts entries where 'observed_collapsed' has been annotated, fits an
isotonic regression calibrator via Pool-Adjacent-Violators, and writes the
result to disk (default: ~/.pathcollapse/calibrator.json).

Subsequent 'pathcollapse breakpoints --confidence on' runs will pick up the
saved calibrator and produce calibrated final scores instead of the raw
log-odds aggregation.`,
		Example: `  pathcollapse confidence refit
  pathcollapse confidence refit --log ./shadow.jsonl --output ./cal.json
  pathcollapse confidence refit --require-minimum 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if logPath == "" {
				var err error
				logPath, err = confidence.DefaultShadowLogPath()
				if err != nil {
					return err
				}
			}
			if outputPath == "" {
				var err error
				outputPath, err = confidence.DefaultCalibratorPath()
				if err != nil {
					return err
				}
			}

			entries, stats, err := confidence.ReadShadowLog(logPath)
			if err != nil {
				return fmt.Errorf("read shadow log: %w", err)
			}

			stderr := cmd.ErrOrStderr()
			stdout := cmd.OutOrStdout()
			info := infoWriter(stderr, quiet)

			fmt.Fprintf(info, "Read %d lines from %s (%d parsed, %d malformed, %d labeled)\n",
				stats.TotalLines, logPath, stats.Parsed, stats.Malformed, stats.Labeled)

			if stats.Labeled == 0 {
				return fmt.Errorf("no labeled entries in %s — annotate 'observed_collapsed' on entries before refitting", logPath)
			}
			if stats.Labeled < requireMinimum {
				return fmt.Errorf("only %d labeled entries (minimum requested: %d)", stats.Labeled, requireMinimum)
			}

			result, err := confidence.Refit(entries)
			if err != nil {
				return fmt.Errorf("refit: %w", err)
			}

			if err := confidence.SaveCalibrator(outputPath, result); err != nil {
				return fmt.Errorf("save calibrator: %w", err)
			}

			printRefitSummary(stdout, result, outputPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&logPath, "log", "", "Path to the shadow JSONL log (default: ~/.pathcollapse/shadow.jsonl)")
	cmd.Flags().StringVar(&outputPath, "output", "", "Path to write the fitted calibrator (default: ~/.pathcollapse/calibrator.json)")
	cmd.Flags().IntVar(&requireMinimum, "require-minimum", 1, "Refuse to fit unless at least N labeled entries are available")
	AddQuietFlag(cmd, &quiet)
	return cmd
}

func printRefitSummary(w io.Writer, r *confidence.RefitResult, outputPath string) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Fitted isotonic calibrator on %d labeled outcomes.\n\n", r.LabeledCount)

	fmt.Fprintf(w, "%-20s %.4f\n", "Brier score:", r.Brier)
	fmt.Fprintf(w, "%-20s %.4f  (constant-0.85 baseline)\n", "Brier baseline:", r.BrierBaseline)
	improvement := 100 * (r.BrierBaseline - r.Brier) / r.BrierBaseline
	fmt.Fprintf(w, "%-20s %+.1f%% vs baseline\n", "Brier improvement:", improvement)
	fmt.Fprintf(w, "%-20s %.4f\n", "ECE (expected):", r.ECE)
	fmt.Fprintf(w, "%-20s %s\n\n", "Regime:", r.Regime)

	if len(r.Buckets) > 0 {
		fmt.Fprintln(w, "Reliability (predicted vs observed, by decile):")
		fmt.Fprintln(w, "   bucket         n     predicted   observed   |delta|")
		fmt.Fprintln(w, "   ──────────────────────────────────────────────────────")
		for _, b := range r.Buckets {
			delta := b.MeanPredicted - b.MeanObserved
			if delta < 0 {
				delta = -delta
			}
			fmt.Fprintf(w, "   [%.2f, %.2f]   %4d   %.3f       %.3f       %.3f\n",
				b.Min, b.Max, b.Count, b.MeanPredicted, b.MeanObserved, delta)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "Calibrator written to %s\n", outputPath)
	switch r.Regime {
	case confidence.RegimeColdStart:
		fmt.Fprintln(w, "Regime is cold_start — ≥50 labeled outcomes required for 'partial', ≥500 for 'calibrated'.")
	case confidence.RegimePartial:
		fmt.Fprintln(w, "Regime is partial — results are usable but ECE may still be loose. Accumulate more labels to reach 'calibrated'.")
	case confidence.RegimeCalibrated:
		if r.ECE > 0.05 {
			fmt.Fprintf(os.Stderr, "WARNING: regime is calibrated but ECE (%.4f) exceeds the 0.05 target. Consider investigating outliers.\n", r.ECE)
		}
	}
}
