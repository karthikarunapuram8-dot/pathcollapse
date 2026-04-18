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

	cmd := &cobra.Command{
		Use:   "breakpoints",
		Short: "Compute minimal control changes to collapse the top risk paths",
		Example: `  pathcollapse breakpoints --top 10
  pathcollapse breakpoints --graph /tmp/graph.json --top 5`,
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

			paths := gatherTopPaths(g, top*5)
			cfg := scoring.DefaultConfig()
			scored := scoring.RankPaths(paths, g, cfg)

			optCfg := controls.DefaultOptimizerConfig()
			optCfg.MaxRecommendations = top
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
					"   type=%-20s  paths-collapsed=%-3d  risk-reduction=%.3f  difficulty=%-6s  confidence=%.0f%%\n\n",
					rec.Change.Type, rec.PathsRemoved, rec.RiskReduction, rec.Difficulty, rec.Confidence*100)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&top, "top", 10, "Number of breakpoint recommendations to output")
	cmd.Flags().StringVar(&graphFile, "graph", "", "Graph snapshot file written by 'ingest --output'")
	return cmd
}
