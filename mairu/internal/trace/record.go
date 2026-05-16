package trace

import (
	"context"
	"time"
)

// Record wraps an LLM call, capturing timing, status, and the supplied
// request/response strings into a trace event. The recorder is taken from
// ctx; if none is attached, Record is effectively a free wrapper around fn.
//
// The fn closure should return (responseText, err). responseText may be empty
// when the actual model output is structured (e.g. JSON unmarshalled into a
// struct) — in that case callers can stash it on the trace via the populate
// callback.
func Record(ctx context.Context, op, model, system, prompt string, fn func() (string, error)) (string, error) {
	rec := RecorderFromContext(ctx)
	start := time.Now()
	resp, err := fn()
	t := LLMTrace{
		ID:        NewID(),
		Project:   ProjectFromContext(ctx),
		Operation: chooseOp(op, OperationFromContext(ctx)),
		Model:     model,
		System:    system,
		Prompt:    prompt,
		Response:  resp,
		LatencyMs: time.Since(start).Milliseconds(),
		ParentID:  ParentFromContext(ctx),
		CreatedAt: time.Now().UTC(),
	}
	if err != nil {
		t.Status = StatusError
		t.Error = err.Error()
	} else {
		t.Status = StatusSuccess
	}
	rec.Record(ctx, t)
	return resp, err
}

func chooseOp(explicit, ctxOp string) string {
	if explicit != "" {
		return explicit
	}
	return ctxOp
}

// Emit records a fully-formed event. Use this from inside an LLM provider when
// you already have token counts, model name, latency, etc. — the higher-level
// Record helper is only convenient for the simple "wrap a closure" case.
//
// Fields the caller doesn't set are auto-populated where possible: ID,
// CreatedAt, Project (from ctx), Operation (from ctx), ParentID (from ctx),
// and Status (from Error).
func Emit(ctx context.Context, t LLMTrace) {
	if t.ID == "" {
		t.ID = NewID()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	if t.Project == "" {
		t.Project = ProjectFromContext(ctx)
	}
	if t.Operation == "" {
		t.Operation = OperationFromContext(ctx)
	}
	if t.ParentID == "" {
		t.ParentID = ParentFromContext(ctx)
	}
	if t.Status == "" {
		if t.Error != "" {
			t.Status = StatusError
		} else {
			t.Status = StatusSuccess
		}
	}
	RecorderFromContext(ctx).Record(ctx, t)
}
