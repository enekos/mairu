package agent

import (
	"sort"
	"strings"

	"mairu/internal/llm"
)

// fileOps tracks files seen during a session by category. Inspired by
// pi-mono's compaction utils — the goal is to surface "what files matter
// right now" in the post-compaction summary so the model can pick up state
// without re-reading everything.
type fileOps struct {
	read    map[string]struct{}
	written map[string]struct{}
	edited  map[string]struct{}
}

func newFileOps() *fileOps {
	return &fileOps{
		read:    map[string]struct{}{},
		written: map[string]struct{}{},
		edited:  map[string]struct{}{},
	}
}

func (f *fileOps) ingest(toolName string, args map[string]any) {
	if args == nil {
		return
	}
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := args[k].(string); ok && strings.TrimSpace(v) != "" {
				return v
			}
		}
		return ""
	}
	switch toolName {
	case "read_file":
		if p := pick("file_path", "path"); p != "" {
			f.read[p] = struct{}{}
		}
	case "write_file":
		if p := pick("file_path", "path"); p != "" {
			f.written[p] = struct{}{}
		}
	case "multi_edit", "replace_block":
		if p := pick("file_path", "path"); p != "" {
			f.edited[p] = struct{}{}
		}
	case "delete_file":
		if p := pick("path", "file_path"); p != "" {
			f.edited[p] = struct{}{}
		}
	}
}

// computeLists returns (readOnly, modified) sorted. A file is read-only iff it
// was never written or edited.
func (f *fileOps) computeLists() (readFiles, modifiedFiles []string) {
	modified := map[string]struct{}{}
	for p := range f.edited {
		modified[p] = struct{}{}
	}
	for p := range f.written {
		modified[p] = struct{}{}
	}
	for p := range f.read {
		if _, isMod := modified[p]; !isMod {
			readFiles = append(readFiles, p)
		}
	}
	for p := range modified {
		modifiedFiles = append(modifiedFiles, p)
	}
	sort.Strings(readFiles)
	sort.Strings(modifiedFiles)
	return
}

func (f *fileOps) format() string {
	read, mod := f.computeLists()
	var parts []string
	if len(read) > 0 {
		parts = append(parts, "<read-files>\n"+strings.Join(read, "\n")+"\n</read-files>")
	}
	if len(mod) > 0 {
		parts = append(parts, "<modified-files>\n"+strings.Join(mod, "\n")+"\n</modified-files>")
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(parts, "\n\n")
}

// extractFileOps walks an LLM history and tallies file accesses by tool name.
func extractFileOps(history []llm.Message) *fileOps {
	ops := newFileOps()
	for _, m := range history {
		for _, tc := range m.ToolCalls {
			ops.ingest(tc.Name, tc.Arguments)
		}
	}
	return ops
}
