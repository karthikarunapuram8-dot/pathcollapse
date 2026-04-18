package subcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/karunapuram/pathcollapse/internal/testdata"
	"github.com/karunapuram/pathcollapse/pkg/controls"
	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
)

// NewBreakpointsCmd returns the breakpoints subcommand.
func NewBreakpointsCmd() *cobra.Command {
	var top int
	var graphFile string
	var confidenceMode string

	cmd := &cobra.Command{
		Use:   "breakpoints",
		Short: "Compute minimal control changes to collapse the top risk paths",
		Example: `  pathcollapse breakpoints --top 10
  pathcollapse breakpoints --graph /tmp/graph.json --top 5
  pathcollapse breakpoints --confidence off   # skip calibrated scoring`,
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
				fmt.Fprintln(cmd.ErrOrStderr(), "INFO: using built-in fixture (pass --graph <snapshot.json> to use ingested data)")
			}

			cfg := scoring.DefaultConfig()
			scored := gatherTopPaths(g, cfg, top*5)

			optCfg := controls.DefaultOptimizerConfig()
			optCfg.MaxRecommendations = top
			optCfg.Confidence, err = ResolveConfidence(cmd, confidenceMode)
			if err != nil {
				return err
			}
			recs := controls.Optimize(scored, g, optCfg)

			if len(recs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No breakpoints found. The graph may not contain tier-0 targets or attack paths.")
				return nil
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
	AddConfidenceFlag(cmd, &confidenceMode)
	return cmd
}
