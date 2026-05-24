package cli

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/dreamware-nz/kete/internal/proxy"
	"github.com/dreamware-nz/kete/internal/store"
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
			cfg, err := proxy.LoadConfig()
			if err != nil {
				return err
			}
			cfg.ExtendedCache = extendedCache
			return withStore(func(db *store.DB) error {
				srv := proxy.NewServer(cfg, db)
				ctx, stop := signal.NotifyContext(context.Background(),
					syscall.SIGINT, syscall.SIGTERM)
				defer stop()
				fmt.Fprintf(cmd.ErrOrStderr(),
					"kete proxy listening on %s (debug=%t, extended-cache=%t)\n",
					srv.Addr(), debug, extendedCache)
				return srv.Run(ctx)
			})
		},
	}
	cmd.Flags().BoolVar(&debug, "debug", false, "enable verbose request/response logging")
	cmd.Flags().BoolVar(&extendedCache, "extended-cache", false, "extend Anthropic prompt cache via keep-alive injection")
	return cmd
}
