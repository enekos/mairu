package tui

import "testing"

func TestComputeSessionStats_CountsAndEstimates(t *testing.T) {
	messages := []ChatMessage{
		{Role: "System", Content: "Loaded session: default"},
		{Role: "You", Content: "Hi"},
		{Role: "Mairu", Content: "Hello there"},
		{Role: "Error", Content: "bad"},
		{Role: "Diff", Content: "```diff\n+ a\n```"},
		{Role: "You", Content: "Please summarize this file."},
	}
	toolEvents := []toolEvent{
		{Kind: "call", Title: "Tool call: read_file"},
		{Kind: "result", Title: "Tool result: read_file"},
		{Kind: "status", Title: "Processing"},
	}

	stats := computeSessionStats(messages, "Partial reply", toolEvents, true, "gemini-3.1-pro-preview")

	if stats.Model != "gemini-3.1-pro-preview" {
		t.Fatalf("unexpected model: %q", stats.Model)
	}
	if stats.StreamState != "streaming" {
		t.Fatalf("expected streaming state, got %q", stats.StreamState)
	}
	if stats.UserMessages != 2 || stats.AssistantMessages != 1 {
		t.Fatalf("unexpected user/assistant counts: %+v", stats)
	}
	if stats.SystemMessages != 1 || stats.ErrorMessages != 1 || stats.DiffMessages != 1 {
		t.Fatalf("unexpected system/error/diff counts: %+v", stats)
	}
	if stats.ToolEvents != 3 || stats.ToolCalls != 1 || stats.ToolResults != 1 {
		t.Fatalf("unexpected tool counts: %+v", stats)
	}
	if stats.EstimatedUserTokens <= 0 {
		t.Fatalf("expected estimated user tokens > 0, got %d", stats.EstimatedUserTokens)
	}
	if stats.EstimatedAgentTokens <= 0 {
		t.Fatalf("expected estimated agent tokens > 0, got %d", stats.EstimatedAgentTokens)
	}
	if stats.EstimatedTotalTokens != stats.EstimatedUserTokens+stats.EstimatedAgentTokens {
		t.Fatalf("expected total tokens to equal user+agent, got %+v", stats)
	}
}

func TestEstimateTokenCount(t *testing.T) {
	tests := []struct {
		chars int
		want  int
	}{
		{chars: 0, want: 0},
		{chars: -10, want: 0},
		{chars: 1, want: 1},
		{chars: 4, want: 1},
		{chars: 5, want: 2},
		{chars: 8, want: 2},
	}

	for _, tc := range tests {
		got := estimateTokenCount(tc.chars)
		if got != tc.want {
			t.Fatalf("estimateTokenCount(%d) = %d, want %d", tc.chars, got, tc.want)
		}
	}
}
