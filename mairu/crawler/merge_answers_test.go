package crawler

import (
	"context"
	"strings"
	"testing"
)

func TestMergeAnswersNodeValidation(t *testing.T) {
	node := &MergeAnswersNode{}

	// Test missing results
	state1 := State{"prompt": "Combine these"}
	_, err := node.Execute(context.Background(), state1)
	if err == nil || !strings.Contains(err.Error(), "missing or empty 'results'") {
		t.Fatalf("Expected error for missing results, got: %v", err)
	}

	// Test missing prompt
	results := map[string]map[string]any{
		"url1": {"key": "value"},
	}
	state2 := State{"results": results}
	_, err = node.Execute(context.Background(), state2)
	if err == nil || !strings.Contains(err.Error(), "missing 'prompt'") {
		t.Fatalf("Expected error for missing prompt, got: %v", err)
	}

	// Test missing provider
	state3 := State{"results": results, "prompt": "prompt"}
	_, err = node.Execute(context.Background(), state3)
	if err == nil || !strings.Contains(err.Error(), "missing GeminiProvider") {
		t.Fatalf("Expected error for missing GeminiProvider, got: %v", err)
	}
}
