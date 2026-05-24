// Command kete is the local memory and reasoning layer for Crush sessions.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overridden at build time:
//
//	go build -ldflags "-X main.version=0.1.0" ./cmd/kete
var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "kete",
		Short:         "Local memory and reasoning layer for AI coding sessions",
		Version:       version,
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
	return root
}

func run() error {
	return newRootCmd().Execute()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
