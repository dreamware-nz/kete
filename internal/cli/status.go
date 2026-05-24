package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/dreamware-nz/kete/internal/store"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show captured tasks for the current project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(func(db *store.DB) error {
				return runStatus(cmd.OutOrStdout(), db, all)
			})
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "list tasks across all projects")
	return cmd
}

// projectPath resolves cwd to the canonical form used as a tasks key.
// Symlinks are resolved so tmpdir aliases collapse to one identity.
func projectPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		return real, nil
	}
	return cwd, nil
}

func runStatus(w io.Writer, db *store.DB, all bool) error {
	ctx := context.Background()
	var (
		tasks []*store.Task
		err   error
	)
	if all {
		tasks, err = db.ListAllTasks(ctx)
	} else {
		var project string
		project, err = projectPath()
		if err != nil {
			return err
		}
		tasks, err = db.ListTasks(ctx, project)
	}
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		fmt.Fprintln(w, "no tasks captured")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	fmt.Fprintln(tw, "ID\tCREATED\tSOURCE\tGOAL")
	for _, t := range tasks {
		goal := t.Goal
		if len(goal) > 60 {
			goal = goal[:57] + "..."
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			shortID(t.ID), t.CreatedAt.Format("2006-01-02 15:04"), t.Source, goal)
	}
	return nil
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
