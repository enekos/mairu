//go:build !slim && !headless && !contextsrvonly

package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"mairu/internal/acpbridge"
)

// NewACPBridgeCmd returns the `mairu acp-bridge` subcommand which runs a
// WebSocket bridge daemon proxying ACP JSON-RPC frames between remote clients
// and locally-spawned ACP agents.
func NewACPBridgeCmd() *cobra.Command {
	var (
		addr              string
		noTailscale       bool
		maxSessions       int
		idleTimeout       time.Duration
		ringSize          int
		permissionTimeout time.Duration
		maxFrameBytes     int64
		debug             bool
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
			level := slog.LevelInfo
			if debug {
				level = slog.LevelDebug
			}
			logger := slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: level}))

			opts := acpbridge.Options{
				Addr:              addr,
				MaxSessions:       maxSessions,
				IdleTimeout:       idleTimeout,
				RingBufferSize:    ringSize,
				PermissionTimeout: permissionTimeout,
				MaxFrameBytes:     maxFrameBytes,
				Logger:            logger,
			}
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

			// Print the bound address only after Start binds it. Run Start in
			// a goroutine so we can announce the bound port (matters when the
			// caller passed ":0", e.g. from a test or systemd socket).
			errCh := make(chan error, 1)
			go func() { errCh <- b.Start(ctx) }()

			// Poll for bound addr, but don't loop forever.
			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			tick := time.NewTicker(20 * time.Millisecond)
			defer tick.Stop()
		announceLoop:
			for {
				select {
				case err := <-errCh:
					if err != nil && !errors.Is(err, http.ErrServerClosed) {
						return err
					}
					return nil
				case <-deadline.C:
					return errors.New("acp-bridge: failed to bind within 5s")
				case <-tick.C:
					if a := b.ListenAddr(); a != "" {
						fmt.Fprintf(os.Stderr, "acp-bridge listening on %s (auth: allow-all)\n", a)
						break announceLoop
					}
				}
			}
			err = <-errCh
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7777", "Listen address")
	cmd.Flags().BoolVar(&noTailscale, "no-tailscale", false, "Bypass Tailscale identity gate (development only)")
	cmd.Flags().IntVar(&maxSessions, "max-sessions", 32, "Maximum concurrent active sessions")
	cmd.Flags().DurationVar(&idleTimeout, "idle-timeout", 30*time.Minute, "Idle session reap interval")
	cmd.Flags().IntVar(&ringSize, "ring-size", 500, "Per-session event-replay ring buffer size")
	cmd.Flags().DurationVar(&permissionTimeout, "permission-timeout", 60*time.Second, "Synthetic-denial deadline for session/request_permission")
	cmd.Flags().Int64Var(&maxFrameBytes, "max-frame-bytes", 1<<20, "Maximum WebSocket message size in bytes")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")
	return cmd
}
