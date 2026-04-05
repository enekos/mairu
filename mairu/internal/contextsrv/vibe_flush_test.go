package contextsrv

import (
	"testing"
)

func TestFlushVibeOps(t *testing.T) {
	ops := []VibeMutationOp{
		{Op: "create_memory"},
		{Op: "create_skill"},
		{Op: "create_context"},
	}
	got := FlushVibeOps(ops, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(got))
	}
}
