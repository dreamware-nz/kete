package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/dreamware-nz/kete/internal/drift"
	"github.com/dreamware-nz/kete/internal/extract"
	"github.com/spf13/cobra"
)

func newDriftTestCmd() *cobra.Command {
	var goal, fixture string
	cmd := &cobra.Command{
		Use:   "drift-test [<prompt>]",
		Short: "Score drift on a prompt or fixture set",
		Long: `Score drift on a single prompt (with --goal) or run an entire
fixture file with --fixture <path>. Fixture mode prints a per-row
table comparing expected vs actual level so you can eyeball
calibration against the hand-labelled set in testdata/drift/.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := extract.NewClient()
			if err != nil {
				return fmt.Errorf("drift-test: %w", err)
			}
			ctx := context.Background()
			out := cmd.OutOrStdout()

			if fixture != "" {
				return runFixtures(ctx, out, c, fixture)
			}

			if len(args) != 1 {
				return errors.New("drift-test: provide a prompt argument or --fixture <path>")
			}
			if goal == "" {
				return errors.New("--goal is required (or use --fixture)")
			}
			return runOne(ctx, out, c, goal, args[0])
		},
	}
	cmd.Flags().StringVar(&goal, "goal", "", "stated goal of the session")
	cmd.Flags().StringVar(&fixture, "fixture", "", "path to a fixture JSON file")
	return cmd
}

func runOne(ctx context.Context, out io.Writer, c *extract.Client, goal, action string) error {
	score, level, err := drift.ScoreAction(ctx, c, goal, action)
	if err != nil {
		return err
	}
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
	correction, err := drift.BuildCorrection(ctx, c, goal, action, level)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "correction: %s\n", correction)
	return nil
}

func runFixtures(ctx context.Context, out io.Writer, c *extract.Client, path string) error {
	fx, err := drift.LoadFixtures(path)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tEXPECTED\tACTUAL\tSCORE\tMATCH")
	matches := 0
	for _, f := range fx {
		score, level, err := drift.ScoreAction(ctx, c, f.Goal, f.Action)
		if err != nil {
			fmt.Fprintf(tw, "%s\t%s\t-\t-\tERR: %s\n", f.ID, f.ExpectedLevel, err)
			continue
		}
		want := drift.LevelOf(f)
		mark := "-"
		if level == want {
			mark = "yes"
			matches++
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n",
			f.ID, want, level, score.Score, mark)
	}
	tw.Flush()
	fmt.Fprintf(out, "\n%d/%d matches\n", matches, len(fx))
	return nil
}
