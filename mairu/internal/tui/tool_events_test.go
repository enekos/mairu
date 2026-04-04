package tui

import (
	"strings"
	"testing"
)

func TestSanitizeStatusText_StripsLeadingEmojiAndSpaces(t *testing.T) {
	got := sanitizeStatusText("🖥️   Running bash: ls -la")
	want := "Running bash: ls -la"
	if got != want {
		t.Fatalf("sanitizeStatusText() = %q, want %q", got, want)
	}
}

func TestBuildToolEventCall_IncludesStructuredFields(t *testing.T) {
	ev := buildToolCallEvent("bash", map[string]any{
		"command":    "ls -la",
		"timeout_ms": 60000,
	})

	if ev.Title != "Tool call: bash" {
		t.Fatalf("unexpected title: %q", ev.Title)
	}
	if len(ev.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d (%v)", len(ev.Lines), ev.Lines)
	}
	if ev.Lines[0] != "command: ls -la" {
		t.Fatalf("unexpected first line: %q", ev.Lines[0])
	}
	if ev.Lines[1] != "timeout_ms: 60000" {
		t.Fatalf("unexpected second line: %q", ev.Lines[1])
	}
}

func TestBuildToolEventResult_TruncatesLongValues(t *testing.T) {
	ev := buildToolResultEvent("bash", map[string]any{
		"output": strings.Repeat("x", 200),
	})

	if ev.Title != "Tool result: bash" {
		t.Fatalf("unexpected title: %q", ev.Title)
	}
	if len(ev.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d (%v)", len(ev.Lines), ev.Lines)
	}
	if !strings.HasPrefix(ev.Lines[0], "output: ") {
		t.Fatalf("unexpected line prefix: %q", ev.Lines[0])
	}
	if !strings.HasSuffix(ev.Lines[0], "...") {
		t.Fatalf("expected truncated result line, got %q", ev.Lines[0])
	}
}

func TestBuildToolEventResult_CollapsesLargePayloads(t *testing.T) {
	ev := buildToolResultEvent("read_file", map[string]any{
		"a": 1,
		"b": 2,
		"c": 3,
		"d": 4,
		"e": 5,
		"f": 6,
		"g": 7,
		"h": 8,
		"i": 9,
	})

	if len(ev.Lines) != 9 {
		t.Fatalf("expected 9 lines (8 fields + summary), got %d (%v)", len(ev.Lines), ev.Lines)
	}
	if ev.Lines[8] != "... and 1 more field" {
		t.Fatalf("unexpected collapsed summary line: %q", ev.Lines[8])
	}
}
