package agent

import (
	"strings"
	"testing"

	"mairu/internal/llm"
)

func TestExtractFileOps_ClassifiesByTool(t *testing.T) {
	history := []llm.Message{
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{
				{Name: "read_file", Arguments: map[string]any{"file_path": "a.go"}},
				{Name: "read_file", Arguments: map[string]any{"file_path": "b.go"}},
				{Name: "multi_edit", Arguments: map[string]any{"file_path": "b.go"}},
				{Name: "write_file", Arguments: map[string]any{"file_path": "c.go"}},
				{Name: "replace_block", Arguments: map[string]any{"file_path": "d.go"}},
				{Name: "delete_file", Arguments: map[string]any{"path": "e.go"}},
				{Name: "bash", Arguments: map[string]any{"command": "ls"}}, // ignored
			},
		},
	}
	ops := extractFileOps(history)
	read, mod := ops.computeLists()

	// b.go was both read and edited → should appear only in modified.
	if containsStr(read, "b.go") {
		t.Errorf("b.go should not be in read-only list once edited")
	}
	if !containsStr(read, "a.go") {
		t.Errorf("a.go should be in read-only list, got %v", read)
	}
	for _, want := range []string{"b.go", "c.go", "d.go", "e.go"} {
		if !containsStr(mod, want) {
			t.Errorf("%s missing from modified list: %v", want, mod)
		}
	}
}

func TestExtractFileOps_FormatProducesXMLBlocks(t *testing.T) {
	history := []llm.Message{
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{
				{Name: "read_file", Arguments: map[string]any{"file_path": "x.go"}},
				{Name: "write_file", Arguments: map[string]any{"file_path": "y.go"}},
			},
		},
	}
	out := extractFileOps(history).format()
	if !strings.Contains(out, "<read-files>\nx.go\n</read-files>") {
		t.Errorf("missing read-files block: %s", out)
	}
	if !strings.Contains(out, "<modified-files>\ny.go\n</modified-files>") {
		t.Errorf("missing modified-files block: %s", out)
	}
}

func TestExtractFileOps_EmptyHistoryFormatsBlank(t *testing.T) {
	if got := extractFileOps(nil).format(); got != "" {
		t.Errorf("expected empty format for empty history, got %q", got)
	}
}

func TestExtractFileOps_IgnoresMissingPaths(t *testing.T) {
	history := []llm.Message{
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{
				{Name: "read_file", Arguments: map[string]any{}},
				{Name: "write_file", Arguments: map[string]any{"file_path": "  "}},
			},
		},
	}
	ops := extractFileOps(history)
	read, mod := ops.computeLists()
	if len(read) != 0 || len(mod) != 0 {
		t.Errorf("blank/missing paths should be skipped, got read=%v mod=%v", read, mod)
	}
}

func containsStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
