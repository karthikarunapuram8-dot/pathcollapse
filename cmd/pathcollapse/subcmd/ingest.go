package subcmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/graph"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/ingest"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/model"
	"github.com/karthikarunapuram8-dot/pathcollapse/pkg/normalize"
)

// NewIngestCmd returns the ingest subcommand.
func NewIngestCmd() *cobra.Command {
	var inputFile string
	var adapterType string
	var outputFile string

	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Ingest identity data into a graph snapshot",
		Example: `  pathcollapse ingest --input data.json --type json --output snapshot.json
  pathcollapse ingest --input users.csv --type csv_users`,
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := os.Open(inputFile)
			if err != nil {
				return fmt.Errorf("open %q: %w", inputFile, err)
			}
			defer f.Close()

			adapter, err := ingest.Get(ingest.AdapterType(adapterType))
			if err != nil {
				return err
			}

			result, err := adapter.Ingest(f)
			if err != nil {
				return fmt.Errorf("ingest: %w", err)
			}

			for _, w := range result.Warns {
				fmt.Fprintf(cmd.ErrOrStderr(), "WARN: %s\n", w)
			}

			normalized := normalize.Normalize(result)

			// Load into graph.
			g := graph.New()
			for _, n := range normalized.Nodes {
				if err := g.AddNode(n); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "WARN: %v\n", err)
				}
			}
			for _, e := range normalized.Edges {
				if err := g.AddEdge(e); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "WARN: %v\n", err)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Ingested %d nodes, %d edges\n",
				g.NodeCount(), g.EdgeCount())

			if outputFile != "" {
				return writeGraphJSON(g, outputFile)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input file path (required)")
	cmd.Flags().StringVarP(&adapterType, "type", "t", "json", "Adapter type: json, csv_users, csv_groups, csv_local_admin, csv_gpo, bloodhound, yaml")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output graph snapshot file (JSON)")
	cmd.MarkFlagRequired("input")

	return cmd
}

type graphSnapshot struct {
	Nodes []*model.Node `json:"nodes"`
	Edges []*model.Edge `json:"edges"`
}

func writeGraphJSON(g *graph.Graph, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %q: %w", path, err)
	}
	defer f.Close()

	snap := graphSnapshot{
		Nodes: g.Nodes(),
		Edges: g.Edges(),
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}
