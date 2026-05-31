package acpbridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
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
	ExitErr    string    `json:"exit_error,omitempty"`
	Stderr     []string  `json:"stderr_tail,omitempty"`
}

type entry struct {
	session *Session
	info    SessionInfo
}

type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*entry
	logger   *slog.Logger
}

// ErrSessionCap is returned by Create when MaxSessions would be exceeded.
var ErrSessionCap = errors.New("acpbridge: session cap reached")

func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{sessions: map[string]*entry{}, logger: logger}
}

func newSessionID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Create starts a session. maxSessions=0 means no cap.
func (r *Registry) Create(ctx context.Context, agent string, specs map[string]AgentSpec, ringSize int, opts SessionStartOptions, maxSessions int) (string, error) {
	spec, ok := specs[agent]
	if !ok {
		return "", fmt.Errorf("unknown agent %q", agent)
	}
	if maxSessions > 0 {
		r.mu.RLock()
		active := 0
		for _, e := range r.sessions {
			if e.info.Active {
				active++
			}
		}
		r.mu.RUnlock()
		if active >= maxSessions {
			return "", ErrSessionCap
		}
	}
	id := newSessionID()
	ring := NewRing(ringSize)
	opts.Logger = r.logger
	sess, err := StartSession(ctx, id, spec, ring, opts)
	if err != nil {
		return "", err
	}
	pm := NewPermissionMux(opts.PermissionTimeout)
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
			if err := sess.ExitErr(); err != nil {
				e.info.ExitErr = err.Error()
			}
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

// Info returns a snapshot of session metadata, including a tail of agent
// stderr. Returns (zero, false) if the session does not exist.
func (r *Registry) Info(id string) (SessionInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.sessions[id]
	if !ok {
		return SessionInfo{}, false
	}
	info := e.info
	info.Stderr = e.session.StderrTail()
	return info, true
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
// Only active sessions are considered.
func (r *Registry) Newest() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var best *entry
	for _, e := range r.sessions {
		if !e.info.Active {
			continue
		}
		if best == nil || e.info.LastActive.After(best.info.LastActive) {
			best = e
		}
	}
	if best == nil {
		return ""
	}
	return best.info.ID
}

// TouchActivity updates a session's LastActive timestamp.
func (r *Registry) TouchActivity(id string) {
	r.mu.Lock()
	if e, ok := r.sessions[id]; ok {
		e.info.LastActive = time.Now()
	}
	r.mu.Unlock()
}

// ReapIdle closes sessions whose subprocess has exited *and* whose LastActive
// is older than the cutoff. Returns the IDs that were reaped. Active sessions
// are never reaped — operator must DELETE them explicitly.
func (r *Registry) ReapIdle(idleAfter time.Duration) []string {
	cutoff := time.Now().Add(-idleAfter)
	type victim struct {
		id   string
		sess *Session
	}
	var victims []victim
	r.mu.Lock()
	for id, e := range r.sessions {
		exited := !e.info.Active
		if !exited {
			select {
			case <-e.session.Done():
				exited = true
			default:
			}
		}
		if exited && e.info.LastActive.Before(cutoff) {
			victims = append(victims, victim{id: id, sess: e.session})
			delete(r.sessions, id)
		}
	}
	r.mu.Unlock()
	ids := make([]string, 0, len(victims))
	for _, v := range victims {
		_ = v.sess.Close()
		ids = append(ids, v.id)
	}
	return ids
}

// Shutdown closes every session. Used during bridge teardown.
func (r *Registry) Shutdown() {
	r.mu.Lock()
	sessions := make([]*Session, 0, len(r.sessions))
	for _, e := range r.sessions {
		sessions = append(sessions, e.session)
	}
	r.sessions = map[string]*entry{}
	r.mu.Unlock()
	for _, s := range sessions {
		_ = s.Close()
	}
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
