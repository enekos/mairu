package agent

import (
	"testing"

	"mairu/internal/llm"
)

// TestMaybeAutoCompact_BelowThresholdIsNoOp verifies the cheap path: when the
// history is short, maybeAutoCompact must not touch the LLM beyond reading
// GetHistory. This guards against accidental thrashing where every tool round
// would trigger a real summarization call.
func TestMaybeAutoCompact_BelowThresholdIsNoOp(t *testing.T) {
	mock := newMockProvider()
	mock.history = []llm.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "yo"},
	}
	a := &Agent{llm: mock}

	a.maybeAutoCompact(nil)

	// Below threshold the function must early-return, leaving history untouched.
	// (Compaction would replace it with a synthetic 2-message summary using
	// providerCfg, which is unset here — so this test would panic if the
	// guard were missing.)
	if len(mock.history) != 2 || mock.history[0].Content != "hi" {
		t.Fatalf("history was mutated below threshold: %+v", mock.history)
	}
}

func TestMaybeAutoCompact_NilLLMIsSafe(t *testing.T) {
	a := &Agent{}
	// Must not panic.
	a.maybeAutoCompact(nil)
}
