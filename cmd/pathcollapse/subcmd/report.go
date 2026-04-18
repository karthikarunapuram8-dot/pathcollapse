package subcmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/karunapuram/pathcollapse/internal/testdata"
	"github.com/karunapuram/pathcollapse/pkg/controls"
	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/reporting"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
)

// NewReportCmd returns the report subcommand.
func NewReportCmd() *cobra.Command {
	var format string
	var top int
	var outputFile string
	var graphFile string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate an analysis report",
		Example: `  pathcollapse report --format markdown
  pathcollapse report --graph /tmp/graph.json --format json --output report.json --top 20`,
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

			allPaths := gatherTopPaths(g, top*3)
			cfg := scoring.DefaultConfig()
			scored := scoring.RankPaths(allPaths, g, cfg)
			if len(scored) > top {
				scored = scored[:top]
			}

			recs := controls.Optimize(scored, g, controls.DefaultOptimizerConfig())

			rep := reporting.BuildReport(g, scored, recs)

			var w = cmd.OutOrStdout()
			if outputFile != "" {
				f, err := os.Create(outputFile)
				if err != nil {
					return fmt.Errorf("create output: %w", err)
				}
				defer f.Close()
				w = f
			}

			r := reporting.New(reporting.Format(format))
			return r.Render(w, rep)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "markdown", "Output format: markdown, json")
	cmd.Flags().IntVar(&top, "top", 10, "Number of top paths to include")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().StringVar(&graphFile, "graph", "", "Graph snapshot file written by 'ingest --output'")

	return cmd
}
