package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/dreamware-nz/kete/internal/store"
	"github.com/spf13/cobra"
)

// check is one diagnostic row. status is "PASS" or "FAIL".
type check struct {
	name   string
	status string
	detail string
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose kete setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.OutOrStdout())
		},
	}
}

func runDoctor(w io.Writer) error {
	checks := []check{
		checkDotdir(),
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	failed := 0
	for _, c := range checks {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", c.status, c.name, c.detail)
		if c.status == "FAIL" {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}

// checkDotdir asserts ~/.kete exists with mode 0700 (ADR 0004).
func checkDotdir() check {
	dir, err := store.DefaultDir()
	if err != nil {
		return check{"dotdir", "FAIL", err.Error()}
	}
	info, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return check{"dotdir", "FAIL", "missing: " + dir}
	}
	if err != nil {
		return check{"dotdir", "FAIL", err.Error()}
	}
	if !info.IsDir() {
		return check{"dotdir", "FAIL", "not a directory: " + dir}
	}
	if info.Mode().Perm() != 0o700 {
		return check{"dotdir", "FAIL",
			fmt.Sprintf("%s mode %o, want 700", dir, info.Mode().Perm())}
	}
	return check{"dotdir", "PASS", dir}
}
