package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newDriftTestCmd() *cobra.Command {
	var goal string
	cmd := &cobra.Command{
		Use:   "drift-test <prompt>",
		Short: "Score drift on a fixture prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if goal == "" {
				return errors.New("--goal is required")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "not yet wired")
			return nil
		},
	}
	cmd.Flags().StringVar(&goal, "goal", "", "stated goal of the session")
	return cmd
}
