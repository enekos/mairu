package batchwriter

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type BatchOpType string

const (
	MemoryOp BatchOpType = "memory"
	SkillOp  BatchOpType = "skill"
	NodeOp   BatchOpType = "node"
)

type BatchOp struct {
	Type BatchOpType
	Data map[string]any
}

type BatchResult struct {
	Successful int
	Failed     int
	Errors     []map[string]string
}

type Embedder interface {
	GetEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
}

type BulkIndexer interface {
	BulkIndex(ctx context.Context, ops []BulkOp) (BatchResult, error)
}

type BulkOp struct {
	Index string
	ID    string
	Body  map[string]any
}

type Options struct {
	BatchSize       int
	FlushIntervalMs int
}

var indexByType = map[BatchOpType]string{
	MemoryOp: "contextfs_memories",
	SkillOp:  "contextfs_skills",
	NodeOp:   "contextfs_context_nodes",
}

type Writer struct {
	embedder   Embedder
	indexer    BulkIndexer
	batchSize  int
	flushEvery time.Duration

	mu    sync.Mutex
	queue []BatchOp
	timer *time.Timer
}

func New(embedder Embedder, indexer BulkIndexer, opts Options) *Writer {
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}
	flushMs := opts.FlushIntervalMs
	if flushMs <= 0 {
		flushMs = 2000
	}
	return &Writer{
		embedder:   embedder,
		indexer:    indexer,
		batchSize:  batchSize,
		flushEvery: time.Duration(flushMs) * time.Millisecond,
	}
}

func (w *Writer) Enqueue(op BatchOp) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queue = append(w.queue, op)
	if len(w.queue) >= w.batchSize {
		if w.timer != nil {
			w.timer.Stop()
			w.timer = nil
		}
		return
	}
	if w.timer == nil {
		w.timer = time.AfterFunc(w.flushEvery, func() {
			_, _ = w.Flush(context.Background())
		})
	}
}

func embedText(op BatchOp) string {
	switch op.Type {
	case MemoryOp:
		if s, _ := op.Data["content"].(string); s != "" {
			return s
		}
	case SkillOp:
		return fmt.Sprintf("%v: %v", op.Data["name"], op.Data["description"])
	case NodeOp:
		return fmt.Sprintf("%v: %v", op.Data["name"], op.Data["abstract"])
	}
	return ""
}

func opID(op BatchOp) string {
	if op.Type == NodeOp {
		if uri, _ := op.Data["uri"].(string); uri != "" {
			return uri
		}
	}
	if id, _ := op.Data["id"].(string); id != "" {
		return id
	}
	return ""
}

func (w *Writer) Flush(ctx context.Context) (BatchResult, error) {
	w.mu.Lock()
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	if len(w.queue) == 0 {
		w.mu.Unlock()
		return BatchResult{}, nil
	}
	batch := append([]BatchOp(nil), w.queue...)
	w.queue = nil
	w.mu.Unlock()

	texts := make([]string, 0, len(batch))
	for _, op := range batch {
		texts = append(texts, embedText(op))
	}
	embeddings, err := w.embedder.GetEmbeddings(ctx, texts)
	if err != nil {
		return BatchResult{}, err
	}

	ops := make([]BulkOp, 0, len(batch))
	for i, op := range batch {
		body := map[string]any{}
		for k, v := range op.Data {
			body[k] = v
		}
		if i < len(embeddings) {
			body["embedding"] = embeddings[i]
		}
		ops = append(ops, BulkOp{
			Index: indexByType[op.Type],
			ID:    opID(op),
			Body:  body,
		})
	}
	return w.indexer.BulkIndex(ctx, ops)
}
