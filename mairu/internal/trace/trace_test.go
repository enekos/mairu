package trace

import (
	"context"
	"errors"
	"testing"
)

func TestRecord_SuccessPath(t *testing.T) {
	rec := &MemoryRecorder{}
	ctx := WithRecorder(context.Background(), rec)
	ctx = WithProject(ctx, "demo")
	resp, err := Record(ctx, "router.test", "gpt-4o", "sys", "hello", func() (string, error) {
		return "world", nil
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp != "world" {
		t.Fatalf("want world, got %q", resp)
	}
	snap := rec.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("want 1 trace, got %d", len(snap))
	}
	tr := snap[0]
	if tr.Operation != "router.test" || tr.Project != "demo" || tr.Status != StatusSuccess {
		t.Fatalf("trace fields wrong: %+v", tr)
	}
	if tr.Response != "world" || tr.Prompt != "hello" {
		t.Fatalf("payload mismatch: %+v", tr)
	}
}

func TestRecord_ErrorPath(t *testing.T) {
	rec := &MemoryRecorder{}
	ctx := WithRecorder(context.Background(), rec)
	_, err := Record(ctx, "router.fail", "", "", "in", func() (string, error) {
		return "", errors.New("boom")
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("want boom, got %v", err)
	}
	snap := rec.Snapshot()
	if len(snap) != 1 || snap[0].Status != StatusError || snap[0].Error != "boom" {
		t.Fatalf("error trace not captured: %+v", snap)
	}
}

func TestDefaultRecorderFallback(t *testing.T) {
	rec := &MemoryRecorder{}
	SetDefault(rec)
	t.Cleanup(func() { SetDefault(nil) })

	// No recorder in ctx — should fall back to default.
	_, _ = Record(context.Background(), "op", "", "", "p", func() (string, error) { return "r", nil })
	if got := len(rec.Snapshot()); got != 1 {
		t.Fatalf("expected default recorder to capture 1 trace, got %d", got)
	}
}

func TestRecord_OperationFromContext(t *testing.T) {
	rec := &MemoryRecorder{}
	ctx := WithRecorder(context.Background(), rec)
	ctx = WithOperation(ctx, "ctx.op")
	_, _ = Record(ctx, "", "", "", "", func() (string, error) { return "", nil })
	snap := rec.Snapshot()
	if snap[0].Operation != "ctx.op" {
		t.Fatalf("expected operation from ctx, got %q", snap[0].Operation)
	}
}
