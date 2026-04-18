package subcmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/karthikarunapuram8-dot/pathcollapse/internal/testdata"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/controls"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/drift"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/reporting"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/scoring"
)

// NewReportCmd returns the report subcommand.
func NewReportCmd() *cobra.Command {
	var format string
	var top int
	var outputFile string
	var graphFile string
	var baselineFile string
	var confidenceMode string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate an analysis report",
		Example: `  pathcollapse report --format markdown
  pathcollapse report --graph /tmp/graph.json --format html --output report.html
  pathcollapse report --graph snapshot.json --baseline baseline.json --format html --output drift-report.html
  pathcollapse report --confidence off   # skip calibrated scoring in rendered output`,
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
			scored := gatherTopPaths(g, cfg, top)

			optCfg := controls.DefaultOptimizerConfig()
			optCfg.Confidence, err = ResolveConfidence(cmd, confidenceMode)
			if err != nil {
				return err
			}
			recs := controls.Optimize(scored, g, optCfg)

			rep := reporting.BuildReport(g, scored, recs)

			if baselineFile != "" {
				baseline, err := LoadGraphFromFile(baselineFile)
				if err != nil {
					return fmt.Errorf("load baseline: %w", err)
				}
				rep.Drift = drift.CompareSnapshots(baseline, g, time.Time{}, time.Time{})
			}

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

	cmd.Flags().StringVarP(&format, "format", "f", "markdown", "Output format: markdown, json, html")
	cmd.Flags().IntVar(&top, "top", 10, "Number of top paths to include")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().StringVar(&graphFile, "graph", "", "Graph snapshot file written by 'ingest --output'")
	cmd.Flags().StringVar(&baselineFile, "baseline", "", "Baseline snapshot for drift analysis (HTML only)")
	AddConfidenceFlag(cmd, &confidenceMode)

	return cmd
}
