package crawler

import (
	"context"
	"strings"
	"testing"
)

func TestPromptRefinerNodeValidation(t *testing.T) {
	node := &PromptRefinerNode{}

	// Test missing prompt
	state1 := State{}
	_, err := node.Execute(context.Background(), state1)
	if err == nil || !strings.Contains(err.Error(), "missing 'prompt'") {
		t.Fatalf("Expected error for missing prompt, got: %v", err)
	}

	// Test missing provider
	state2 := State{"prompt": "Get emails"}
	_, err = node.Execute(context.Background(), state2)
	if err == nil || !strings.Contains(err.Error(), "missing GeminiProvider") {
		t.Fatalf("Expected error for missing GeminiProvider, got: %v", err)
	}
}
