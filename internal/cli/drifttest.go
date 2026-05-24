package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/dreamware-nz/kete/internal/drift"
	"github.com/dreamware-nz/kete/internal/extract"
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
			c, err := extract.NewClient()
			if err != nil {
				return fmt.Errorf("drift-test: %w", err)
			}
			ctx := context.Background()
			score, level, err := drift.ScoreAction(ctx, c, goal, args[0])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "score: %d\nlevel: %s\nreasoning: %s\n",
				score.Score, level, score.Reasoning)
			if len(score.ScopeViolations) > 0 {
				fmt.Fprintln(out, "scope violations:")
				for _, v := range score.ScopeViolations {
					fmt.Fprintf(out, "  - %s\n", v)
				}
			}
			if level == drift.LevelNone {
				return nil
			}
			correction, err := drift.BuildCorrection(ctx, c, goal, args[0], level)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "correction: %s\n", correction)
			return nil
		},
	}
	cmd.Flags().StringVar(&goal, "goal", "", "stated goal of the session")
	return cmd
}
