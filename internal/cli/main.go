// Package cli wires the kete cobra command tree.
//
// The construction lives outside cmd/kete so other binaries (ketedoc)
// can render help against the same tree without depending on main.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set by cmd/kete via ldflags. Default "dev" lets `go run`
// just work.
var Version = "dev"

// NewRoot returns the kete cobra root with all subcommands attached.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "kete",
		Short:         "Local memory and reasoning layer for AI coding sessions",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.AddCommand(newProxyCmd())
	root.AddCommand(newMCPCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newTasksCmd())
	root.AddCommand(newDriftTestCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newPurgeCmd())
	return root
}

// Main is the cmd/kete entry point. Lives here so the cmd binary stays
// a one-line `cli.Main()` shim.
func Main() {
	if err := NewRoot().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
