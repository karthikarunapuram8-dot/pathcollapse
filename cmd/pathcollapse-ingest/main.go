// pathcollapse-ingest ingests identity data into a graph snapshot.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/karthikarunapuram8-dot/pathcollapse/cmd/pathcollapse/subcmd"
)

func main() {
	root := &cobra.Command{
		Use:   "pathcollapse-ingest",
		Short: "Ingest identity data into a PathCollapse graph snapshot",
	}
	root.AddCommand(subcmd.NewIngestCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
