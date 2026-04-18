// pathcollapse-report generates analysis reports.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/karunapuram/pathcollapse/cmd/pathcollapse/subcmd"
)

func main() {
	root := &cobra.Command{
		Use:   "pathcollapse-report",
		Short: "Generate PathCollapse analysis reports",
	}
	root.AddCommand(subcmd.NewReportCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
