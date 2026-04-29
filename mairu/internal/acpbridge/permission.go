package acpbridge

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PermissionMux tracks outstanding session/request_permission JSON-RPC
// requests so the bridge can fan them out to multiple WS clients and
// synthesize a denial response if no client answers within the timeout.
//
// Tracking is keyed by the JSON-RPC id (preserved verbatim as bytes so
// numeric and string ids round-trip identically).
type PermissionMux struct {
	timeout   time.Duration
	mu        sync.Mutex
	pending   map[string]chan struct{}
	OnTimeout func(synthetic []byte) // called when no client responds; fed to agent stdin
}

func NewPermissionMux(timeout time.Duration) *PermissionMux {
	return &PermissionMux{timeout: timeout, pending: map[string]chan struct{}{}}
}

// Track starts a timeout for an outstanding permission request. id is the
// raw JSON-RPC id (preserved exactly so we can echo it in synthetic
// denials). frame is the full server→client request, kept for diagnostics
// only and currently unused.
func (p *PermissionMux) Track(ctx context.Context, id []byte, _frame string) {
	key := string(id)
	done := make(chan struct{})
	p.mu.Lock()
	p.pending[key] = done
	p.mu.Unlock()

	go func() {
		select {
		case <-done: // resolved
			return
		case <-time.After(p.timeout):
			p.mu.Lock()
			if _, still := p.pending[key]; !still {
				p.mu.Unlock()
				return
			}
			delete(p.pending, key)
			p.mu.Unlock()
			if p.OnTimeout != nil {
				p.OnTimeout([]byte(fmt.Sprintf(
					`{"jsonrpc":"2.0","id":%s,"result":{"outcome":{"outcome":"cancelled"}}}`, key)))
			}
		case <-ctx.Done():
			return
		}
	}()
}

// Resolve marks a tracked permission request as answered; the timeout will
// not fire.
func (p *PermissionMux) Resolve(id []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := string(id)
	if ch, ok := p.pending[key]; ok {
		close(ch)
		delete(p.pending, key)
	}
}
