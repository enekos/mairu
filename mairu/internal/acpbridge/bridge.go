// Package acpbridge runs a tailnet-only WebSocket server that proxies ACP
// JSON-RPC frames between remote clients and locally-spawned ACP agents.
package acpbridge

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

// Options configures a Bridge instance.
type Options struct {
	Addr           string               // e.g. "100.64.0.1:7777" or ":7777"
	Authorizer     PeerAuthorizer       // nil = AllowAll (tests only)
	Agents         map[string]AgentSpec // override default agent specs
	RingBufferSize int                  // events per session, default 500
}

// Bridge is a tailnet-only WebSocket ACP proxy.
type Bridge struct {
	opts     Options
	registry *Registry
	srv      *http.Server
	listener net.Listener
	mu       sync.RWMutex
	addr     string
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
	return &Bridge{opts: opts, registry: NewRegistry()}, nil
}

// Start begins serving ACP connections. Blocks until ctx is cancelled.
func (b *Bridge) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", b.opts.Addr)
	if err != nil {
		return err
	}
	b.listener = ln
	b.mu.Lock()
	b.addr = ln.Addr().String()
	b.srv = &http.Server{Handler: b.Mux()}
	b.mu.Unlock()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = b.srv.Shutdown(shutdownCtx)
	}()
	return b.srv.Serve(ln)
}

// ListenAddr returns the actual address the bridge is listening on.
// Returns an empty string if the bridge has not started yet.
func (b *Bridge) ListenAddr() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.addr
}

// Shutdown gracefully stops the bridge.
func (b *Bridge) Shutdown(ctx context.Context) error {
	if b.srv == nil {
		return nil
	}
	return b.srv.Shutdown(ctx)
}

// errors is used by New; keep the import live.
var _ = errors.New
