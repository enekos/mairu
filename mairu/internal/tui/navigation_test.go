package tui

import "testing"

func TestClampMessageIndex(t *testing.T) {
	if got := clampMessageIndex(-1, 1, 0); got != -1 {
		t.Fatalf("expected -1 for empty list, got %d", got)
	}
	if got := clampMessageIndex(-1, 1, 3); got != 0 {
		t.Fatalf("expected first selection to be 0, got %d", got)
	}
	if got := clampMessageIndex(1, 1, 3); got != 2 {
		t.Fatalf("expected next selection to be 2, got %d", got)
	}
	if got := clampMessageIndex(2, 1, 3); got != 2 {
		t.Fatalf("expected selection to stay at upper bound, got %d", got)
	}
	if got := clampMessageIndex(0, -1, 3); got != 0 {
		t.Fatalf("expected selection to stay at lower bound, got %d", got)
	}
}

func TestPreviewText(t *testing.T) {
	if got := previewText("hello\nworld", 20); got != "hello world" {
		t.Fatalf("unexpected newline normalization: %q", got)
	}
	if got := previewText("123456789", 5); got != "12..." {
		t.Fatalf("unexpected truncation: %q", got)
	}
}
