package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// errNotImplemented is the placeholder return for stub subcommands.
// Phases that wire real behaviour replace it.
var errNotImplemented = errors.New("not yet implemented")

func newProxyCmd() *cobra.Command {
	var debug, extendedCache bool
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Run the local HTTP proxy",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), errNotImplemented)
			return nil
		},
	}
	cmd.Flags().BoolVar(&debug, "debug", false, "enable verbose request/response logging")
	cmd.Flags().BoolVar(&extendedCache, "extended-cache", false, "extend Anthropic prompt cache via keep-alive injection")
	return cmd
}
