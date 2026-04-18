package subcmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/karunapuram/pathcollapse/pkg/drift"
)

// NewDiffCmd returns the diff subcommand.
func NewDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <old-snapshot.json> <new-snapshot.json>",
		Short: "Compare two graph snapshots and report drift",
		Args:  cobra.ExactArgs(2),
		Example: `  pathcollapse diff snapshot-jan.json snapshot-feb.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			oldG, err := LoadGraphFromFile(args[0])
			if err != nil {
				return fmt.Errorf("load old snapshot: %w", err)
			}
			newG, err := LoadGraphFromFile(args[1])
			if err != nil {
				return fmt.Errorf("load new snapshot: %w", err)
			}

			rep := drift.CompareSnapshots(oldG, newG, time.Time{}, time.Time{})

			fmt.Fprintf(cmd.OutOrStdout(), "# Drift Report\n\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Nodes added: %d, removed: %d\n", rep.NodesAdded, rep.NodesRemoved)
			fmt.Fprintf(cmd.OutOrStdout(), "Edges added: %d, removed: %d\n\n", rep.EdgesAdded, rep.EdgesRemoved)

			if len(rep.Items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No security-relevant drift detected.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Security-relevant changes (%d):\n\n", len(rep.Items))
			for i, item := range rep.Items {
				fmt.Fprintf(cmd.OutOrStdout(), "%d. [%s] %s — %s\n",
					i+1, item.Severity, item.Category, item.Description)
			}
			return nil
		},
	}
	return cmd
}
