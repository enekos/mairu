package crawler

import (
	"context"
	"strings"
	"testing"
)

func TestExtractNodeWithMock(t *testing.T) {
	// We only test validation in the absence of a real LLM.
	node := &ExtractNode{}

	state := State{"doc": "test", "prompt": "test"}
	_, err := node.Execute(context.Background(), state)
	if err == nil || !strings.Contains(err.Error(), "missing GeminiProvider") {
		t.Errorf("ExtractNode missed provider validation: %v", err)
	}

	// Too long doc truncates
	longDoc := strings.Repeat("A", 70000)
	state["doc"] = longDoc
	_, err = node.Execute(context.Background(), state)
	if err == nil || !strings.Contains(err.Error(), "missing GeminiProvider") {
		t.Errorf("ExtractNode missed provider validation on long doc: %v", err)
	}
}

func TestSearchLinkNodeWithMock(t *testing.T) {
	node := &SearchLinkNode{}

	// Missing links vs empty links
	state := State{"html": "<html><body></body></html>", "prompt": "test"}

	// This should fail because of provider missing
	_, err := node.Execute(context.Background(), state)
	if err == nil || !strings.Contains(err.Error(), "missing GeminiProvider") {
		t.Errorf("SearchLinkNode missed provider validation on empty links: %v", err)
	}

	// With links it should hit the provider validation
	state["html"] = "<html><body><a href='/test'>Link</a></body></html>"
	_, err = node.Execute(context.Background(), state)
	if err == nil || !strings.Contains(err.Error(), "missing GeminiProvider") {
		t.Errorf("SearchLinkNode missed provider validation on links: %v", err)
	}
}

func TestMergeAnswersNodeWithMock(t *testing.T) {
	node := &MergeAnswersNode{}

	// Too long results
	results := map[string]map[string]any{
		"url1": {"key": strings.Repeat("A", 90000)},
	}
	state := State{"results": results, "prompt": "test"}
	_, err := node.Execute(context.Background(), state)
	if err == nil || !strings.Contains(err.Error(), "missing GeminiProvider") {
		t.Errorf("MergeAnswers missed provider validation on long results: %v", err)
	}
}
