package subcmd

import (
	"fmt"
	"io"
	"os"
	"time"

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

  status   Show shadow-log progress and saved calibrator metadata
  refit    Fit an isotonic calibrator from a shadow-mode JSONL log and
           persist it so subsequent 'breakpoints --confidence on' runs
           produce calibrated (rather than cold-start) scores.

See docs/confidence.md for the algorithm.`,
	}
	cmd.AddCommand(
		newConfidenceStatusCmd(),
		newConfidenceRefitCmd(),
	)
	return cmd
}

func newConfidenceStatusCmd() *cobra.Command {
	var (
		logPath        string
		calibratorPath string
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show shadow-mode collection progress and calibrator status",
		Long: `Displays the current shadow-mode JSONL statistics and any saved
calibrator metadata so operators can see how close they are to the
'partial' (50 labels) and 'calibrated' (500 labels) regimes.`,
		Example: `  pathcollapse confidence status
  pathcollapse confidence status --log ./shadow.jsonl --calibrator ./cal.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if logPath == "" {
				var err error
				logPath, err = confidence.DefaultShadowLogPath()
				if err != nil {
					return err
				}
			}
			if calibratorPath == "" {
				var err error
				calibratorPath, err = confidence.DefaultCalibratorPath()
				if err != nil {
					return err
				}
			}

			_, stats, err := confidence.ReadShadowLog(logPath)
			if err != nil {
				return fmt.Errorf("read shadow log: %w", err)
			}
			meta, err := confidence.LoadCalibratorMetadata(calibratorPath)
			if err != nil {
				return fmt.Errorf("read calibrator: %w", err)
			}

			printConfidenceStatus(cmd.OutOrStdout(), logPath, calibratorPath, stats, meta)
			return nil
		},
	}

	cmd.Flags().StringVar(&logPath, "log", "", "Path to the shadow JSONL log (default: ~/.pathcollapse/shadow.jsonl)")
	cmd.Flags().StringVar(&calibratorPath, "calibrator", "", "Path to the saved calibrator (default: ~/.pathcollapse/calibrator.json)")
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

func printConfidenceStatus(
	w io.Writer,
	logPath string,
	calibratorPath string,
	stats confidence.ReadStats,
	meta *confidence.CalibratorMetadata,
) {
	labeled := stats.Labeled
	unlabeled := stats.Parsed - stats.Labeled

	fmt.Fprintln(w, "Shadow-mode collection")
	fmt.Fprintf(w, "  log path:            %s\n", logPath)
	fmt.Fprintf(w, "  total lines:         %d\n", stats.TotalLines)
	fmt.Fprintf(w, "  parsed entries:      %d\n", stats.Parsed)
	fmt.Fprintf(w, "  malformed lines:     %d\n", stats.Malformed)
	fmt.Fprintf(w, "  labeled entries:     %d\n", labeled)
	fmt.Fprintf(w, "  unlabeled entries:   %d\n", unlabeled)
	fmt.Fprintf(w, "  to partial (50):     %s\n", progressString(labeled, 50))
	fmt.Fprintf(w, "  to calibrated (500): %s\n", progressString(labeled, 500))

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Saved calibrator")
	fmt.Fprintf(w, "  path:                %s\n", calibratorPath)
	if meta == nil {
		fmt.Fprintln(w, "  status:              none saved yet")
		fmt.Fprintln(w, "  auto-load:           cold-start / identity calibrator remains active")
		return
	}

	fmt.Fprintln(w, "  status:              present")
	fmt.Fprintf(w, "  fit time (UTC):      %s\n", meta.FitTime.Format(time.RFC3339))
	fmt.Fprintf(w, "  training labels:     %d\n", meta.TrainingLabels)
	fmt.Fprintf(w, "  regime:              %s\n", meta.Regime)
	fmt.Fprintf(w, "  brier:               %.4f\n", meta.Brier)
	fmt.Fprintf(w, "  brier baseline:      %.4f\n", meta.BrierBaseline)
	fmt.Fprintf(w, "  brier improvement:   %s\n", brierImprovementString(meta.Brier, meta.BrierBaseline))
	fmt.Fprintf(w, "  ece:                 %.4f\n", meta.ECE)
	fmt.Fprintln(w, "  auto-load:           will be used by 'breakpoints/report --confidence on'")
}

func progressString(current, target int) string {
	if current >= target {
		return fmt.Sprintf("%d/%d (ready)", current, target)
	}
	return fmt.Sprintf("%d/%d (%d remaining)", current, target, target-current)
}

func brierImprovementString(brier, baseline float64) string {
	if baseline == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%+.1f%% vs baseline", 100*(baseline-brier)/baseline)
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
