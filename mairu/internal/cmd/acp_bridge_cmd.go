//go:build !slim && !headless && !contextsrvonly

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"mairu/internal/acpbridge"
)

// NewACPBridgeCmd returns the `mairu acp-bridge` subcommand which runs a
// WebSocket bridge daemon proxying ACP JSON-RPC frames between remote clients
// and locally-spawned ACP agents.
func NewACPBridgeCmd() *cobra.Command {
	var (
		addr   string
		noAuth bool
	)

	cmd := &cobra.Command{
		Use:   "acp-bridge",
		Short: "Run the ACP-over-WebSocket bridge daemon",
		Long: `Starts a WebSocket server that proxies ACP JSON-RPC frames between
remote clients (e.g. mairu-mobile) and locally-spawned ACP agents.

By default the bridge binds 127.0.0.1:7777 and accepts any peer (use only
behind a tailnet). Pass --no-auth=false to enable the pluggable peer
authorizer (currently no real Tailscale identity wiring; see plan).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := acpbridge.Options{Addr: addr}
			if noAuth {
				opts.Authorizer = acpbridge.AllowAll{}
			}
			b, err := acpbridge.New(opts)
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			fmt.Fprintf(cmd.ErrOrStderr(), "acp-bridge listening on %s\n", addr)
			if err := b.Start(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7777", "Listen address")
	cmd.Flags().BoolVar(&noAuth, "no-auth", true, "Disable peer auth (development; tailnet recommended)")
	return cmd
}
