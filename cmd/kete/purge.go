package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dreamware-nz/kete/internal/store"
	"github.com/spf13/cobra"
)

func newPurgeCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Delete the kete dotdir",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPurge(cmd.OutOrStdout(), cmd.InOrStdin(), yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	return cmd
}

func runPurge(w io.Writer, in io.Reader, yes bool) error {
	dir, err := store.DefaultDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Fprintf(w, "nothing to purge: %s does not exist\n", dir)
		return nil
	}
	if !yes {
		fmt.Fprintf(w, "delete %s and all captured tasks? [y/N] ", dir)
		r := bufio.NewReader(in)
		line, _ := r.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(line)) != "y" {
			fmt.Fprintln(w, "aborted")
			return nil
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove %s: %w", dir, err)
	}
	fmt.Fprintf(w, "removed %s\n", dir)
	return nil
}
