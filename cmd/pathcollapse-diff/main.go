// pathcollapse-diff compares two graph snapshots and reports drift.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/karthikarunapuram8-dot/pathcollapse/cmd/pathcollapse/subcmd"
)

func main() {
	root := &cobra.Command{
		Use:   "pathcollapse-diff <old.json> <new.json>",
		Short: "Compare two PathCollapse graph snapshots",
	}
	root.AddCommand(subcmd.NewDiffCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
