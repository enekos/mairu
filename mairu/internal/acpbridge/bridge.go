// Package acpbridge runs a tailnet-only WebSocket server that proxies ACP
// JSON-RPC frames between remote clients and locally-spawned ACP agents.
package acpbridge

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// Options configures a Bridge instance.
type Options struct {
	Addr              string               // e.g. "100.64.0.1:7777" or ":7777"
	Authorizer        PeerAuthorizer       // nil = AllowAll (tests only)
	Agents            map[string]AgentSpec // override default agent specs
	RingBufferSize    int                  // events per session, default 500
	MaxSessions       int                  // hard cap, default 32; 0 = unlimited
	IdleTimeout       time.Duration        // sessions with no activity for this long are reaped (default 30m, 0 = disabled)
	PermissionTimeout time.Duration        // session/request_permission synth-denial deadline (default 60s)
	MaxFrameBytes     int64                // per-message size cap, default 1 MiB; 0 = use default
	Logger            *slog.Logger         // optional; nil → discard
}

// Bridge is a tailnet-only WebSocket ACP proxy.
type Bridge struct {
	opts     Options
	registry *Registry
	srv      *http.Server
	mu       sync.RWMutex
	addr     string
	logger   *slog.Logger
}

// New validates opts and returns a ready-to-start Bridge.
func New(opts Options) (*Bridge, error) {
	if opts.Addr == "" {
		return nil, errors.New("acpbridge: Addr required")
	}
	if opts.RingBufferSize == 0 {
		opts.RingBufferSize = 500
	}
	if opts.Authorizer == nil {
		opts.Authorizer = AllowAll{}
	}
	if opts.Agents == nil {
		opts.Agents = DefaultAgentSpecs()
	}
	if opts.MaxSessions == 0 {
		opts.MaxSessions = 32
	}
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = 30 * time.Minute
	}
	if opts.PermissionTimeout == 0 {
		opts.PermissionTimeout = 60 * time.Second
	}
	if opts.MaxFrameBytes <= 0 {
		opts.MaxFrameBytes = 1 << 20 // 1 MiB
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Bridge{
		opts:     opts,
		registry: NewRegistry(logger),
		logger:   logger,
	}, nil
}

// Start begins serving ACP connections. Blocks until ctx is cancelled or the
// server stops. Returns http.ErrServerClosed on clean shutdown.
func (b *Bridge) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", b.opts.Addr)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.addr = ln.Addr().String()
	b.srv = &http.Server{
		Handler:           b.Mux(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	b.mu.Unlock()

	// Idle reaper.
	var reaperWG sync.WaitGroup
	reaperCtx, cancelReaper := context.WithCancel(context.Background())
	if b.opts.IdleTimeout > 0 {
		reaperWG.Add(1)
		go func() {
			defer reaperWG.Done()
			b.runReaper(reaperCtx)
		}()
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = b.srv.Shutdown(shutdownCtx)
		cancelReaper()
		b.registry.Shutdown()
	}()
	err = b.srv.Serve(ln)
	cancelReaper()
	reaperWG.Wait()
	return err
}

// ListenAddr returns the actual address the bridge is listening on.
// Returns an empty string if the bridge has not started yet.
func (b *Bridge) ListenAddr() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.addr
}

// Shutdown gracefully stops the bridge and tears down all sessions.
func (b *Bridge) Shutdown(ctx context.Context) error {
	b.mu.RLock()
	srv := b.srv
	b.mu.RUnlock()
	if srv == nil {
		return nil
	}
	err := srv.Shutdown(ctx)
	b.registry.Shutdown()
	return err
}

// runReaper periodically closes sessions whose subprocess has exited or whose
// LastActive time is older than IdleTimeout.
func (b *Bridge) runReaper(ctx context.Context) {
	interval := b.opts.IdleTimeout / 4
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			reaped := b.registry.ReapIdle(b.opts.IdleTimeout)
			for _, id := range reaped {
				b.logger.Info("acpbridge: reaped idle session", "session", id)
			}
		}
	}
}
