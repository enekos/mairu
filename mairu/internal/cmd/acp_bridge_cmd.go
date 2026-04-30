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
		addr        string
		noTailscale bool
	)

	cmd := &cobra.Command{
		Use:   "acp-bridge",
		Short: "Run the ACP-over-WebSocket bridge daemon",
		Long: `Starts a WebSocket server that proxies ACP JSON-RPC frames between
remote clients (e.g. mairu-mobile) and locally-spawned ACP agents.

By default the bridge binds 127.0.0.1:7777. Without --no-tailscale the
bridge expects to run behind a Tailscale identity gate (tsnet wiring is
deferred; until then --no-tailscale is required for the daemon to start).
Pass --no-tailscale to bypass the gate for development, CI, and the
mobile e2e harness.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := acpbridge.Options{Addr: addr}
			if noTailscale {
				opts.Authorizer = acpbridge.AllowAll{}
			} else {
				return errors.New("acp-bridge: Tailscale identity gate is not yet wired; pass --no-tailscale to run without auth (development only)")
			}
			b, err := acpbridge.New(opts)
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			fmt.Fprintf(cmd.ErrOrStderr(), "acp-bridge listening on %s (auth: allow-all)\n", addr)
			if err := b.Start(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7777", "Listen address")
	cmd.Flags().BoolVar(&noTailscale, "no-tailscale", false, "Bypass Tailscale identity gate (development only)")
	return cmd
}
