package batchwriter

import (
	"context"
	"testing"
)

type fakeEmbedder struct{}

func (f fakeEmbedder) GetEmbeddings(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for range texts {
		out = append(out, make([]float32, 768))
	}
	return out, nil
}

type fakeIndexer struct {
	calls int
	ops   []BulkOp
}

func (f *fakeIndexer) BulkIndex(_ context.Context, ops []BulkOp) (BatchResult, error) {
	f.calls++
	f.ops = append(f.ops, ops...)
	return BatchResult{Successful: len(ops)}, nil
}

func TestEnqueueDoesNotImmediatelyWrite(t *testing.T) {
	idx := &fakeIndexer{}
	w := New(fakeEmbedder{}, idx, Options{BatchSize: 3, FlushIntervalMs: 10000})
	w.Enqueue(BatchOp{Type: MemoryOp, Data: map[string]any{"id": "m1", "content": "hello"}})
	if idx.calls != 0 {
		t.Fatalf("expected no bulk index calls, got %d", idx.calls)
	}
}

func TestFlushWritesQueuedOps(t *testing.T) {
	idx := &fakeIndexer{}
	w := New(fakeEmbedder{}, idx, Options{BatchSize: 3, FlushIntervalMs: 10000})
	w.Enqueue(BatchOp{Type: MemoryOp, Data: map[string]any{"id": "m1", "content": "hello"}})
	w.Enqueue(BatchOp{Type: SkillOp, Data: map[string]any{"id": "s1", "name": "code", "description": "write code"}})
	res, err := w.Flush(context.Background())
	if err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if idx.calls != 1 {
		t.Fatalf("expected 1 bulk call, got %d", idx.calls)
	}
	if res.Successful != 2 {
		t.Fatalf("expected 2 successful, got %d", res.Successful)
	}
}
