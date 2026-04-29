package acpbridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

type SessionInfo struct {
	ID         string    `json:"id"`
	Agent      string    `json:"agent"`
	StartedAt  time.Time `json:"started_at"`
	LastActive time.Time `json:"last_activity_at"`
	Active     bool      `json:"active"`
}

type entry struct {
	session *Session
	info    SessionInfo
}

type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*entry
}

func NewRegistry() *Registry {
	return &Registry{sessions: map[string]*entry{}}
}

func newSessionID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (r *Registry) Create(ctx context.Context, agent string, specs map[string]AgentSpec, ringSize int) (string, error) {
	spec, ok := specs[agent]
	if !ok {
		return "", fmt.Errorf("unknown agent %q", agent)
	}
	id := newSessionID()
	ring := NewRing(ringSize)
	sess, err := StartSession(ctx, id, spec, ring)
	if err != nil {
		return "", err
	}
	pm := NewPermissionMux(60 * time.Second)
	pm.OnTimeout = func(synthetic []byte) {
		// Send adds its own newline; pass the raw JSON-RPC frame.
		_ = sess.Send(synthetic)
	}
	sess.PermissionMux = pm
	now := time.Now()
	r.mu.Lock()
	r.sessions[id] = &entry{
		session: sess,
		info:    SessionInfo{ID: id, Agent: agent, StartedAt: now, LastActive: now, Active: true},
	}
	r.mu.Unlock()
	go func() {
		<-sess.Done()
		r.mu.Lock()
		if e, ok := r.sessions[id]; ok {
			e.info.Active = false
		}
		r.mu.Unlock()
	}()
	return id, nil
}

func (r *Registry) Get(id string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.sessions[id]
	if !ok {
		return nil, false
	}
	return e.session, true
}

func (r *Registry) List() []SessionInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]SessionInfo, 0, len(r.sessions))
	for _, e := range r.sessions {
		out = append(out, e.info)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastActive.After(out[j].LastActive)
	})
	return out
}

func (r *Registry) Delete(id string) error {
	r.mu.Lock()
	e, ok := r.sessions[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("no such session %s", id)
	}
	delete(r.sessions, id)
	r.mu.Unlock()
	return e.session.Close()
}

// Newest returns the most-recently-active session id, or "" if none.
func (r *Registry) Newest() string {
	list := r.List()
	if len(list) == 0 {
		return ""
	}
	return list[0].ID
}

// TouchActivity updates a session's LastActive timestamp.
func (r *Registry) TouchActivity(id string) {
	r.mu.Lock()
	if e, ok := r.sessions[id]; ok {
		e.info.LastActive = time.Now()
	}
	r.mu.Unlock()
}

// replay returns the buffered frames for the given session whose ID is > after.
// Returns nil if the session does not exist.
func (r *Registry) replay(id string, after uint64) []StampedFrame {
	r.mu.RLock()
	e, ok := r.sessions[id]
	r.mu.RUnlock()
	if !ok {
		return nil
	}
	return e.session.ring.Since(after)
}
