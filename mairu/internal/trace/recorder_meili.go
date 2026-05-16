package trace

import (
	"context"
	"log"

	"github.com/meilisearch/meilisearch-go"
)

// MeiliRecorder upserts traces into the contextfs_llm_traces index. Records
// asynchronously through a buffered channel so LLM-call hot paths never block
// on Meili.
type MeiliRecorder struct {
	client meilisearch.ServiceManager
	ch     chan LLMTrace
	done   chan struct{}
}

// NewMeiliRecorder builds a recorder that writes to host with apiKey. It
// spawns a single background worker; call Close when shutting down.
func NewMeiliRecorder(host, apiKey string) *MeiliRecorder {
	r := &MeiliRecorder{
		client: meilisearch.New(host, meilisearch.WithAPIKey(apiKey)),
		ch:     make(chan LLMTrace, 256),
		done:   make(chan struct{}),
	}
	go r.run()
	return r
}

// Record implements Recorder. Drops the trace if the buffer is full rather
// than blocking the caller.
func (r *MeiliRecorder) Record(_ context.Context, t LLMTrace) {
	select {
	case r.ch <- t:
	default:
		// Buffer full; drop. Tracing is best-effort.
	}
}

// Close flushes any queued traces and stops the background worker.
func (r *MeiliRecorder) Close() {
	close(r.ch)
	<-r.done
}

func (r *MeiliRecorder) run() {
	defer close(r.done)
	for t := range r.ch {
		doc := map[string]any{
			"id":         t.ID,
			"project":    t.Project,
			"operation":  t.Operation,
			"model":      t.Model,
			"system":     t.System,
			"prompt":     t.Prompt,
			"response":   t.Response,
			"schema":     t.Schema,
			"status":     t.Status,
			"error":      t.Error,
			"latency_ms": t.LatencyMs,
			"tokens_in":  t.TokensIn,
			"tokens_out": t.TokensOut,
			"parent_id":  t.ParentID,
			"metadata":   t.Metadata,
			"created_at": t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
		if _, err := r.client.Index(IndexName).AddDocuments([]map[string]any{doc}, nil); err != nil {
			log.Printf("trace: meili upsert failed: %v", err)
		}
	}
}
