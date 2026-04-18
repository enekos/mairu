package agent

import "testing"

func TestBuiltinToolRegistry_AllPresent(t *testing.T) {
	want := []string{
		"bash",
		"read_file", "write_file", "delete_file",
		"replace_block", "multi_edit",
		"find_files", "search_codebase",
		"fetch_url", "scrape_url", "search_web",
		"delegate_task", "review_work", "browser_context",
	}
	if got := len(builtinTools); got != len(want) {
		t.Errorf("registry has %d tools, want %d", got, len(want))
	}
	for _, name := range want {
		if findBuiltinTool(name) == nil {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestBuiltinToolRegistry_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, bt := range builtinTools {
		name := bt.Definition().Name
		if seen[name] {
			t.Errorf("duplicate tool name: %s", name)
		}
		seen[name] = true
	}
}
