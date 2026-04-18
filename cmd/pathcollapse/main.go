// pathcollapse is the unified CLI for the PathCollapse identity graph analytics platform.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/karunapuram/pathcollapse/cmd/pathcollapse/subcmd"
)

func main() {
	root := &cobra.Command{
		Use:   "pathcollapse",
		Short: "Graph-based identity exposure analysis",
		Long: `PathCollapse ingests identity/policy metadata from enterprise environments,
constructs a typed privilege graph, reasons over escalation paths, and computes
the minimal control changes that collapse the highest-risk paths.`,
	}

	root.AddCommand(
		subcmd.NewIngestCmd(),
		subcmd.NewAnalyzeCmd(),
		subcmd.NewReportCmd(),
		subcmd.NewDiffCmd(),
		subcmd.NewBreakpointsCmd(),
		subcmd.NewSnapshotCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
