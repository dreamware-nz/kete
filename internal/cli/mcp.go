package cli

import (
	"context"
	"os"
	"path/filepath"

	"github.com/dreamware-nz/kete/internal/mcp"
	"github.com/dreamware-nz/kete/internal/store"
	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run the stdio MCP server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withStore(func(db *store.DB) error {
				logFile, err := openMCPLog()
				if err != nil {
					return err
				}
				defer logFile.Close()
				srv := mcp.NewServer(db, Version, logFile)
				return srv.Serve(context.Background(), cmd.InOrStdin(), cmd.OutOrStdout())
			})
		},
	}
}

// openMCPLog opens ~/.kete/kete-mcp.log (append, 0600).
func openMCPLog() (*os.File, error) {
	dir, err := store.DefaultDir()
	if err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, "kete-mcp.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}
