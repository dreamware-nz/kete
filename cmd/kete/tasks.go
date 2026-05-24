package main

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/dreamware-nz/kete/internal/store"
	"github.com/spf13/cobra"
)

func newTasksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tasks <query>",
		Short: "Search captured tasks by goal/keywords",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(func(db *store.DB) error {
				return runTasks(cmd.OutOrStdout(), db, args[0])
			})
		},
	}
}

func runTasks(w io.Writer, db *store.DB, query string) error {
	tasks, err := db.SearchTasks(context.Background(), query)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		fmt.Fprintln(w, "no matches")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	fmt.Fprintln(tw, "ID\tCREATED\tPROJECT\tGOAL")
	for _, t := range tasks {
		goal := t.Goal
		if len(goal) > 50 {
			goal = goal[:47] + "..."
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			shortID(t.ID), t.CreatedAt.Format("2006-01-02"), t.ProjectPath, goal)
	}
	return nil
}
