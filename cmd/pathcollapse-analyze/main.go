// pathcollapse-analyze runs DSL queries against an identity graph.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/karthikarunapuram8-dot/pathcollapse/cmd/pathcollapse/subcmd"
)

func main() {
	root := &cobra.Command{
		Use:   "pathcollapse-analyze",
		Short: "Run PathCollapse DSL queries",
	}
	root.AddCommand(subcmd.NewAnalyzeCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
