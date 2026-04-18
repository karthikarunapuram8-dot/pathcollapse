package subcmd

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/karunapuram/pathcollapse/internal/testdata"
	"github.com/karunapuram/pathcollapse/pkg/graph"
	"github.com/karunapuram/pathcollapse/pkg/snapshot"
)

// NewSnapshotCmd returns the snapshot command group.
func NewSnapshotCmd() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage graph snapshots in local SQLite storage",
	}

	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to snapshot database (default: ~/.pathcollapse/snapshots.db)")

	openStore := func() (*snapshot.Store, error) {
		p := dbPath
		if p == "" {
			var err error
			p, err = snapshot.DefaultDBPath()
			if err != nil {
				return nil, err
			}
		}
		return snapshot.Open(p)
	}

	cmd.AddCommand(
		newSnapshotSaveCmd(openStore),
		newSnapshotListCmd(openStore),
		newSnapshotDiffCmd(openStore),
		newSnapshotPruneCmd(openStore),
	)
	return cmd
}

func newSnapshotSaveCmd(openStore func() (*snapshot.Store, error)) *cobra.Command {
	var name string
	var graphFile string

	cmd := &cobra.Command{
		Use:   "save",
		Short: "Save the current graph as a named snapshot",
		Example: `  pathcollapse snapshot save --name weekly-jan
  pathcollapse snapshot save --graph /tmp/graph.json --name post-migration`,
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
				fmt.Fprintln(cmd.ErrOrStderr(), "INFO: using built-in fixture (pass --graph to use ingested data)")
			}

			if name == "" {
				name = "snapshot-" + time.Now().UTC().Format("2006-01-02T15-04-05")
			}

			store, err := openStore()
			if err != nil {
				return err
			}
			defer store.Close()

			id, err := store.Save(name, g)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved snapshot %q (id=%d, nodes=%d, edges=%d)\n",
				name, id, g.NodeCount(), g.EdgeCount())
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Snapshot name (default: timestamp)")
	cmd.Flags().StringVarP(&graphFile, "graph", "g", "", "Graph snapshot JSON to save (default: built-in fixture)")
	return cmd
}

func newSnapshotListCmd(openStore func() (*snapshot.Store, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all stored snapshots",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openStore()
			if err != nil {
				return err
			}
			defer store.Close()

			snaps, err := store.List()
			if err != nil {
				return err
			}
			if len(snaps) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No snapshots stored.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tSAVED AT\tNODES\tEDGES")
			for _, s := range snaps {
				fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%d\n",
					s.ID, s.Name, s.SavedAt.Format(time.RFC3339), s.NodeCount, s.EdgeCount)
			}
			return w.Flush()
		},
	}
}

func newSnapshotDiffCmd(openStore func() (*snapshot.Store, error)) *cobra.Command {
	return &cobra.Command{
		Use:     "diff <old-id> <new-id>",
		Short:   "Compare two stored snapshots and report drift",
		Args:    cobra.ExactArgs(2),
		Example: `  pathcollapse snapshot diff 1 3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			oldID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid old-id: %w", err)
			}
			newID, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid new-id: %w", err)
			}

			store, err := openStore()
			if err != nil {
				return err
			}
			defer store.Close()

			rep, oldSnap, newSnap, err := store.Diff(oldID, newID)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "# Drift: %q (id=%d) → %q (id=%d)\n\n",
				oldSnap.Name, oldSnap.ID, newSnap.Name, newSnap.ID)
			fmt.Fprintf(out, "Nodes added: %d, removed: %d\n", rep.NodesAdded, rep.NodesRemoved)
			fmt.Fprintf(out, "Edges added: %d, removed: %d\n\n", rep.EdgesAdded, rep.EdgesRemoved)

			if len(rep.Items) == 0 {
				fmt.Fprintln(out, "No security-relevant drift detected.")
				return nil
			}
			fmt.Fprintf(out, "Security-relevant changes (%d):\n\n", len(rep.Items))
			for i, item := range rep.Items {
				fmt.Fprintf(out, "%d. [%s] %s — %s\n",
					i+1, item.Severity, item.Category, item.Description)
			}
			return nil
		},
	}
}

func newSnapshotPruneCmd(openStore func() (*snapshot.Store, error)) *cobra.Command {
	var maxAgeDays int
	var keepMin int

	cmd := &cobra.Command{
		Use:     "prune",
		Short:   "Delete old snapshots from the database",
		Example: `  pathcollapse snapshot prune --older-than 30 --keep-min 5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openStore()
			if err != nil {
				return err
			}
			defer store.Close()

			maxAge := time.Duration(maxAgeDays) * 24 * time.Hour
			deleted, err := store.Prune(maxAge, keepMin)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Pruned %d snapshot(s).\n", deleted)
			return nil
		},
	}

	cmd.Flags().IntVar(&maxAgeDays, "older-than", 90, "Delete snapshots older than N days")
	cmd.Flags().IntVar(&keepMin, "keep-min", 5, "Always keep at least this many recent snapshots")
	return cmd
}
