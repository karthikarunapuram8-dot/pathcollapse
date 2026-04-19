package subcmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/karthikarunapuram8-dot/pathcollapse/internal/testdata"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/confidence"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/controls"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/scoring"
)

// NewBreakpointsCmd returns the breakpoints subcommand.
func NewBreakpointsCmd() *cobra.Command {
	var top int
	var graphFile string
	var confidenceMode string
	var quiet bool
	var shadowMode bool
	var shadowLog string

	cmd := &cobra.Command{
		Use:   "breakpoints",
		Short: "Compute minimal control changes to collapse the top risk paths",
		Example: `  pathcollapse breakpoints --top 10
  pathcollapse breakpoints --graph /tmp/graph.json --top 5
  pathcollapse breakpoints --confidence off   # skip calibrated scoring
  pathcollapse breakpoints --shadow-mode      # log each rec to shadow.jsonl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var g *graph.Graph
			var err error
			if graphFile != "" {
				g, err = LoadGraphFromFile(graphFile)
				if err != nil {
					return fmt.Errorf("load graph: %w", err)
				}
			} else {
				g = testdata.EnterpriseAD()
				fmt.Fprintln(infoWriter(cmd.ErrOrStderr(), quiet), "INFO: using built-in fixture (pass --graph <snapshot.json> to use ingested data)")
			}

			cfg := scoring.DefaultConfig()
			scored := gatherTopPaths(g, cfg, top*5)

			optCfg := controls.DefaultOptimizerConfig()
			optCfg.MaxRecommendations = top
			// Shadow mode always needs confidence enabled internally so we
			// have a breakdown to log — even if the user passed
			// --confidence off, force it on here. The displayed score is
			// still blanked below.
			effectiveMode := confidenceMode
			if shadowMode && effectiveMode == confidenceModeOff {
				effectiveMode = confidenceModeOn
			}
			optCfg.Confidence, err = ResolveConfidence(cmd, effectiveMode, quiet)
			if err != nil {
				return err
			}
			recs := controls.Optimize(scored, g, optCfg)

			// Shadow mode: append every rec with a breakdown to the JSONL
			// log, then overwrite the displayed confidence with the legacy
			// constant so analyst decisions aren't biased by unvalidated
			// scores during the collection period.
			shadowPath := ""
			if shadowMode {
				shadowPath, err = appendShadowEntries(shadowLog, recs, g)
				if err != nil {
					return fmt.Errorf("shadow-mode log: %w", err)
				}
				for i := range recs {
					recs[i].Confidence = controls.LegacyConfidence
					recs[i].Breakdown = nil
					recs[i].Regime = ""
				}
			}

			if len(recs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No breakpoints found. The graph may not contain tier-0 targets or attack paths.")
				return nil
			}

			if shadowMode {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"SHADOW: wrote %d entries to %s (confidence scores hidden — run 'pathcollapse confidence refit' once labeled)\n",
					len(recs), shadowPath)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Top %d control breakpoints (%d paths analysed, ordered by paths collapsed):\n\n",
				len(recs), len(scored))
			for i, rec := range recs {
				fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n", i+1, rec.Change.Description)
				fmt.Fprintf(cmd.OutOrStdout(),
					"   type=%-20s  paths-collapsed=%-3d  risk-reduction=%.3f  difficulty=%-6s  confidence=%.0f%%\n",
					rec.Change.Type, rec.PathsRemoved, rec.RiskReduction, rec.Difficulty, rec.Confidence*100)
				if rec.Breakdown != nil {
					fmt.Fprintf(cmd.OutOrStdout(),
						"   drivers: E=%.2f R=%.2f S=%.2f T=%.2f K=%.2f  regime=%s\n",
						rec.Breakdown.Evidence, rec.Breakdown.Robustness, rec.Breakdown.Safety,
						rec.Breakdown.TemporalStability, rec.Breakdown.CoverageConcentration, rec.Regime)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&top, "top", 10, "Number of breakpoint recommendations to output")
	cmd.Flags().StringVar(&graphFile, "graph", "", "Graph snapshot file written by 'ingest --output'")
	cmd.Flags().BoolVar(&shadowMode, "shadow-mode", false, "Log per-rec confidence scores to ~/.pathcollapse/shadow.jsonl; display the legacy 0.85 so decisions aren't biased by unvalidated scores")
	cmd.Flags().StringVar(&shadowLog, "shadow-log", "", "Path to shadow-mode JSONL log (default: ~/.pathcollapse/shadow.jsonl)")
	AddConfidenceFlag(cmd, &confidenceMode)
	AddQuietFlag(cmd, &quiet)
	return cmd
}

// appendShadowEntries writes one JSONL line per recommendation that carries
// a breakdown. Returns the resolved log path (useful for the SHADOW: note).
func appendShadowEntries(logPath string, recs []controls.ControlRecommendation, g *graph.Graph) (string, error) {
	if logPath == "" {
		var err error
		logPath, err = confidence.DefaultShadowLogPath()
		if err != nil {
			return "", err
		}
	}
	for _, rec := range recs {
		if rec.Breakdown == nil {
			continue
		}
		edgeID := rec.Change.EdgeID
		var source, target, etype string
		if e := g.GetEdge(edgeID); e != nil {
			source = e.Source
			target = e.Target
			etype = string(e.Type)
		}
		entry := confidence.NewShadowEntry(edgeID, source, target, etype, rec.Breakdown.Raw, *rec.Breakdown, time.Now().UTC())
		if err := confidence.AppendShadowEntry(logPath, entry); err != nil {
			return logPath, err
		}
	}
	return logPath, nil
}
