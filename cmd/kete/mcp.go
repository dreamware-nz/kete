package main

import (
	"bufio"
	"fmt"

	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run the stdio MCP server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Drain stdin so a piping client doesn't see EPIPE; the
			// real JSON-RPC loop replaces this in plan 003.
			r := bufio.NewReader(cmd.InOrStdin())
			for {
				if _, err := r.ReadString('\n'); err != nil {
					break
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), errNotImplemented)
			return nil
		},
	}
}
