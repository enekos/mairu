// Package acpbridge runs a tailnet-only WebSocket server that proxies ACP
// JSON-RPC frames between remote clients and locally-spawned ACP agents.
package acpbridge

import (
	"context"
	"errors"
	"net/http"
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
func (b *Bridge) Start(ctx context.Context) error { return errors.New("not implemented") }

// Shutdown gracefully stops the bridge.
func (b *Bridge) Shutdown(ctx context.Context) error { return nil }
