package subcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/karunapuram/pathcollapse/internal/testdata"
	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/model"
	"github.com/karunapuram/pathcollapse/pkg/query"
	"github.com/karunapuram/pathcollapse/pkg/scoring"
)

// NewAnalyzeCmd returns the analyze subcommand.
func NewAnalyzeCmd() *cobra.Command {
	var queryStr string
	var graphFile string
	var baselineFile string

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Run a PathCollapse DSL query against the graph",
		Example: `  pathcollapse analyze --query "FIND PATHS FROM user:alice TO privilege:tier0 LIMIT 10"
  pathcollapse analyze --query "FIND BREAKPOINTS FOR top_paths LIMIT 5"
  pathcollapse analyze --graph /tmp/graph.json --query "FIND PATHS FROM user:alice TO privilege:tier0"
  pathcollapse analyze --graph /tmp/new.json --baseline /tmp/old.json --query "SHOW DRIFT"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if queryStr == "" {
				return fmt.Errorf("--query is required")
			}

			stmt, err := query.ParseQuery(queryStr)
			if err != nil {
				return fmt.Errorf("parse query: %w", err)
			}

			var g *graph.Graph
			if graphFile != "" {
				g, err = LoadGraphFromFile(graphFile)
				if err != nil {
					return fmt.Errorf("load graph: %w", err)
				}
			} else {
				g = testdata.EnterpriseAD()
				fmt.Fprintln(cmd.ErrOrStderr(), "INFO: using built-in fixture (pass --graph <snapshot.json> to use ingested data)")
			}

			ex := query.NewExecutor(g, scoring.DefaultConfig())

			if baselineFile != "" {
				baseline, err := LoadGraphFromFile(baselineFile)
				if err != nil {
					return fmt.Errorf("load baseline: %w", err)
				}
				ex.SetBaseline(baseline)
			}

			result, err := ex.Execute(stmt)
			if err != nil {
				return fmt.Errorf("execute: %w", err)
			}

			if result.Message != "" {
				fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			}

			if len(result.ScoredPaths) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d path(s) found:\n\n", len(result.ScoredPaths))
				for i, sp := range result.ScoredPaths {
					src := nodeName(g, sp.Path.Source())
					tgt := nodeName(g, sp.Path.Target())
					fmt.Fprintf(cmd.OutOrStdout(), "  %d. [risk=%.3f] %s → %s (%d hops)\n",
						i+1, sp.Score, src, tgt, sp.Path.Len())
					for _, e := range sp.Path.Edges {
						fmt.Fprintf(cmd.OutOrStdout(), "       %s -[%s]-> %s\n",
							nodeLookup(g, e.Source), e.Type, nodeLookup(g, e.Target))
					}
					fmt.Fprintln(cmd.OutOrStdout())
				}
			}

			if len(result.Recommendations) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Control Recommendations (%d):\n\n", len(result.Recommendations))
				for i, rec := range result.Recommendations {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, rec.Change.Description)
					fmt.Fprintf(cmd.OutOrStdout(),
						"     type=%-20s  paths-collapsed=%-3d  risk-reduction=%.3f  difficulty=%s  confidence=%.0f%%\n\n",
						rec.Change.Type, rec.PathsRemoved, rec.RiskReduction, rec.Difficulty, rec.Confidence*100)
				}
			}

			if len(result.ScoredPaths) == 0 && len(result.Recommendations) == 0 && result.Message == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No results.")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&queryStr, "query", "q", "", "DSL query string (required)")
	cmd.Flags().StringVar(&graphFile, "graph", "", "Graph snapshot file written by 'ingest --output'")
	cmd.Flags().StringVar(&baselineFile, "baseline", "", "Baseline snapshot for SHOW DRIFT comparison")

	return cmd
}

func nodeName(g *graph.Graph, n *model.Node) string {
	if n == nil {
		return "?"
	}
	return n.Name
}

func nodeLookup(g *graph.Graph, id string) string {
	n := g.GetNode(id)
	if n == nil {
		return id
	}
	return n.Name
}
