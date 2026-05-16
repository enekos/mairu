// Package trace records LLM call traces for observability and replay-based
// evaluation. Inspired by LangSmith/Langfuse but intentionally minimal:
// one Recorder interface, one storage backend, and a shape that maps cleanly
// onto Meilisearch so traces can be searched alongside the other contextfs
// indexes.
package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// IndexName is the Meilisearch index for stored traces. Kept here (not in
// contextsrv) so the trace package has no upstream dependency on the rest of
// mairu — the Meili recorder can be built without importing contextsrv.
const IndexName = "contextfs_llm_traces"

// Status values used in LLMTrace.Status.
const (
	StatusSuccess = "success"
	StatusError   = "error"
)

// LLMTrace is a single LLM call event. All fields are optional except
// Operation and CreatedAt — callers should fill in what they have.
type LLMTrace struct {
	ID        string            `json:"id"`
	Project   string            `json:"project,omitempty"`
	Operation string            `json:"operation"`
	Model     string            `json:"model,omitempty"`
	System    string            `json:"system,omitempty"`
	Prompt    string            `json:"prompt,omitempty"`
	Response  string            `json:"response,omitempty"`
	Schema    string            `json:"schema,omitempty"`
	Status    string            `json:"status,omitempty"`
	Error     string            `json:"error,omitempty"`
	LatencyMs int64             `json:"latency_ms"`
	TokensIn  int               `json:"tokens_in,omitempty"`
	TokensOut int               `json:"tokens_out,omitempty"`
	ParentID  string            `json:"parent_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// Recorder persists an LLMTrace. Implementations must be safe for concurrent
// use and MUST NOT block the caller — a failure to record is never fatal to
// the LLM call itself.
type Recorder interface {
	Record(ctx context.Context, t LLMTrace)
}

// NoopRecorder discards traces. Used when tracing is disabled.
type NoopRecorder struct{}

// Record implements Recorder.
func (NoopRecorder) Record(_ context.Context, _ LLMTrace) {}

// MemoryRecorder keeps traces in-memory. Used by tests and as a fallback when
// Meili is unavailable.
type MemoryRecorder struct {
	mu     sync.Mutex
	traces []LLMTrace
}

// Record implements Recorder.
func (m *MemoryRecorder) Record(_ context.Context, t LLMTrace) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.traces = append(m.traces, t)
}

// Snapshot returns a copy of all recorded traces. Safe to call concurrently
// with Record.
func (m *MemoryRecorder) Snapshot() []LLMTrace {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]LLMTrace, len(m.traces))
	copy(out, m.traces)
	return out
}

// --- context plumbing -----------------------------------------------------

type ctxKey int

const (
	keyRecorder ctxKey = iota
	keyOperation
	keyProject
	keyParent
)

// WithRecorder returns a context that carries r. Downstream code should call
// RecorderFromContext to fetch it.
func WithRecorder(ctx context.Context, r Recorder) context.Context {
	if r == nil {
		return ctx
	}
	return context.WithValue(ctx, keyRecorder, r)
}

// RecorderFromContext returns the recorder attached to ctx, falling back to
// the process-global default, then to a NoopRecorder.
func RecorderFromContext(ctx context.Context) Recorder {
	if r, ok := ctx.Value(keyRecorder).(Recorder); ok && r != nil {
		return r
	}
	if r := Default(); r != nil {
		return r
	}
	return NoopRecorder{}
}

var (
	defaultMu sync.RWMutex
	defaultRec Recorder
)

// SetDefault registers a process-wide recorder. Pass nil to clear it.
func SetDefault(r Recorder) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultRec = r
}

// Default returns the process-wide recorder previously set with SetDefault,
// or nil if none is registered.
func Default() Recorder {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultRec
}

// WithOperation tags the context with the operation name (e.g.
// "router.memory_action") so nested calls can pick it up without an explicit
// arg.
func WithOperation(ctx context.Context, op string) context.Context {
	return context.WithValue(ctx, keyOperation, op)
}

// OperationFromContext returns the operation tag, or "" if unset.
func OperationFromContext(ctx context.Context) string {
	if s, ok := ctx.Value(keyOperation).(string); ok {
		return s
	}
	return ""
}

// WithProject tags the context with the project so the recorder can write it
// onto each trace without it being threaded through every signature.
func WithProject(ctx context.Context, project string) context.Context {
	return context.WithValue(ctx, keyProject, project)
}

// ProjectFromContext returns the project tag, or "" if unset.
func ProjectFromContext(ctx context.Context) string {
	if s, ok := ctx.Value(keyProject).(string); ok {
		return s
	}
	return ""
}

// WithParent attaches a parent trace ID so multi-step LLM workflows can be
// reconstructed.
func WithParent(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyParent, id)
}

// ParentFromContext returns the parent trace ID, or "" if unset.
func ParentFromContext(ctx context.Context) string {
	if s, ok := ctx.Value(keyParent).(string); ok {
		return s
	}
	return ""
}

// NewID generates a short random ID for a trace. Not cryptographically scoped —
// just collision-resistant enough for Meili document IDs.
func NewID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
