package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"mairu/internal/ctxclient"
)

type memoryRecord struct {
	ID         string `json:"id"`
	Project    string `json:"project"`
	Content    string `json:"content"`
	Category   string `json:"category"`
	Owner      string `json:"owner"`
	Importance int    `json:"importance"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func (m *model) handleMemoryCommand(arg string) []ChatMessage {
	parts := strings.Fields(arg)
	sub := "list"
	rest := ""
	if len(parts) > 0 {
		sub = strings.ToLower(parts[0])
		rest = strings.TrimSpace(strings.TrimPrefix(arg, parts[0]))
	}

	switch sub {
	case "", "list", "ls":
		return m.memoryList(rest)
	case "show", "read", "get":
		return m.memoryShow(rest)
	case "search", "find":
		return m.memorySearchCmd(rest)
	case "store", "write", "add":
		return m.memoryStore(rest)
	case "delete", "rm", "del":
		return m.memoryDelete(rest)
	case "help", "?":
		return []ChatMessage{{Role: "System", Content: memoryHelpText()}}
	default:
		return []ChatMessage{{Role: "Error", Content: "Unknown /memory subcommand: " + sub + " (try /memory help)"}}
	}
}

func memoryHelpText() string {
	return strings.Join([]string{
		"**/memory** — inspect stored memories",
		"",
		"  /memory                 List recent memories (default 20)",
		"  /memory list [N]        List recent memories (limit N)",
		"  /memory show <id>       Show full content of a memory",
		"  /memory search <query>  Search memories",
		"  /memory store <text>    Store a new memory (importance 5)",
		"  /memory delete <id>     Delete a memory",
		"",
		"Append `-P <project>` to scope to a project (default: all projects).",
	}, "\n")
}

func extractProject(arg string) (string, string) {
	parts := strings.Fields(arg)
	out := make([]string, 0, len(parts))
	project := ""
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if (p == "-P" || p == "--project") && i+1 < len(parts) {
			project = parts[i+1]
			i++
			continue
		}
		if strings.HasPrefix(p, "--project=") {
			project = strings.TrimPrefix(p, "--project=")
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, " "), project
}

func (m *model) memoryFetch(method, path string, params map[string]string, body any) ([]byte, error) {
	base := m.contextAPIBase()
	req, err := ctxclient.Build(method, base+path, params, body)
	if err != nil {
		return nil, err
	}
	return doContextRequest(req)
}

func (m *model) memoryList(arg string) []ChatMessage {
	rest, project := extractProject(arg)
	limit := 20
	if rest != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil && n > 0 {
			limit = n
		}
	}
	params := map[string]string{"limit": strconv.Itoa(limit)}
	if project != "" {
		params["project"] = project
	}
	raw, err := m.memoryFetch("GET", "/api/memories", params, nil)
	if err != nil {
		return []ChatMessage{{Role: "Error", Content: "Failed to load memories: " + err.Error()}}
	}
	var items []memoryRecord
	if err := json.Unmarshal(raw, &items); err != nil {
		return []ChatMessage{{Role: "Error", Content: "Failed to parse memories: " + err.Error()}}
	}
	if len(items) == 0 {
		hint := ""
		if project != "" {
			hint = " for project `" + project + "`"
		}
		return []ChatMessage{{Role: "System", Content: "No memories stored" + hint + "."}}
	}
	return []ChatMessage{{Role: "System", Content: formatMemoryList(items, project)}}
}

func formatMemoryList(items []memoryRecord, project string) string {
	var b strings.Builder
	if project == "" {
		b.WriteString(fmt.Sprintf("**%d memories** (most recent first)\n\n", len(items)))
	} else {
		b.WriteString(fmt.Sprintf("**%d memories** in project `%s`\n\n", len(items), project))
	}
	for _, it := range items {
		short := strings.ReplaceAll(strings.TrimSpace(it.Content), "\n", " ")
		if len(short) > 80 {
			short = short[:77] + "..."
		}
		b.WriteString(fmt.Sprintf("- `%s` [%s] (imp %d) %s\n", shortID(it.ID), emptyDash(it.Category), it.Importance, short))
	}
	b.WriteString("\nUse `/memory show <id>` to view full content.")
	return b.String()
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func (m *model) memoryShow(arg string) []ChatMessage {
	rest, project := extractProject(arg)
	id := strings.TrimSpace(rest)
	if id == "" {
		return []ChatMessage{{Role: "Error", Content: "Usage: /memory show <id>"}}
	}
	params := map[string]string{"limit": "200"}
	if project != "" {
		params["project"] = project
	}
	raw, err := m.memoryFetch("GET", "/api/memories", params, nil)
	if err != nil {
		return []ChatMessage{{Role: "Error", Content: "Failed to load memories: " + err.Error()}}
	}
	var items []memoryRecord
	if err := json.Unmarshal(raw, &items); err != nil {
		return []ChatMessage{{Role: "Error", Content: "Failed to parse memories: " + err.Error()}}
	}
	for _, it := range items {
		if it.ID == id || strings.HasPrefix(it.ID, id) {
			return []ChatMessage{{Role: "System", Content: formatMemoryDetail(it)}}
		}
	}
	return []ChatMessage{{Role: "Error", Content: "Memory not found: " + id}}
}

func formatMemoryDetail(it memoryRecord) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Memory `%s`\n\n", it.ID))
	b.WriteString(fmt.Sprintf("**Project:** `%s` | **Category:** `%s` | **Owner:** `%s` | **Importance:** %d\n",
		emptyDash(it.Project), emptyDash(it.Category), emptyDash(it.Owner), it.Importance))
	if it.CreatedAt != "" {
		b.WriteString(fmt.Sprintf("**Created:** %s\n", it.CreatedAt))
	}
	b.WriteString("\n## Content\n")
	b.WriteString(it.Content)
	b.WriteString("\n")
	return b.String()
}

func (m *model) memorySearchCmd(arg string) []ChatMessage {
	rest, project := extractProject(arg)
	query := strings.TrimSpace(rest)
	if query == "" {
		return []ChatMessage{{Role: "Error", Content: "Usage: /memory search <query>"}}
	}
	params := map[string]string{
		"q":    query,
		"type": "memory",
		"topK": "10",
	}
	if project != "" {
		params["project"] = project
	}
	raw, err := m.memoryFetch("GET", "/api/search", params, nil)
	if err != nil {
		return []ChatMessage{{Role: "Error", Content: "Search failed: " + err.Error()}}
	}
	var resp struct {
		Memories []map[string]any `json:"memories"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return []ChatMessage{{Role: "Error", Content: "Failed to parse search: " + err.Error()}}
	}
	if len(resp.Memories) == 0 {
		return []ChatMessage{{Role: "System", Content: fmt.Sprintf("No memories matched `%s`.", query)}}
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("**%d matches** for `%s`\n\n", len(resp.Memories), query))
	for _, hit := range resp.Memories {
		id, _ := hit["id"].(string)
		cat, _ := hit["category"].(string)
		content, _ := hit["content"].(string)
		short := strings.ReplaceAll(strings.TrimSpace(content), "\n", " ")
		if len(short) > 80 {
			short = short[:77] + "..."
		}
		b.WriteString(fmt.Sprintf("- `%s` [%s] score=%s %s\n",
			shortID(id), emptyDash(cat), formatScore(hit["_rankingScore"]), short))
	}
	b.WriteString("\nUse `/memory show <id>` to view full content.")
	return []ChatMessage{{Role: "System", Content: b.String()}}
}

func formatScore(v any) string {
	switch x := v.(type) {
	case float64:
		return fmt.Sprintf("%.2f", x)
	case nil:
		return "?"
	default:
		return fmt.Sprintf("%v", x)
	}
}

func (m *model) memoryStore(arg string) []ChatMessage {
	rest, project := extractProject(arg)
	content := strings.TrimSpace(rest)
	if content == "" {
		return []ChatMessage{{Role: "Error", Content: "Usage: /memory store <content>"}}
	}
	body := map[string]any{
		"content":    content,
		"category":   "observation",
		"owner":      "user",
		"importance": 5,
	}
	if project != "" {
		body["project"] = project
	}
	raw, err := m.memoryFetch("POST", "/api/memories", nil, body)
	if err != nil {
		return []ChatMessage{{Role: "Error", Content: "Failed to store memory: " + err.Error()}}
	}
	var created memoryRecord
	_ = json.Unmarshal(raw, &created)
	if created.ID == "" {
		return []ChatMessage{{Role: "System", Content: "Stored memory."}}
	}
	return []ChatMessage{{Role: "System", Content: fmt.Sprintf("Stored memory `%s`.", shortID(created.ID))}}
}

func (m *model) memoryDelete(arg string) []ChatMessage {
	rest, _ := extractProject(arg)
	id := strings.TrimSpace(rest)
	if id == "" {
		return []ChatMessage{{Role: "Error", Content: "Usage: /memory delete <id>"}}
	}
	if _, err := m.memoryFetch("DELETE", "/api/memories", map[string]string{"id": id}, nil); err != nil {
		return []ChatMessage{{Role: "Error", Content: "Delete failed: " + err.Error()}}
	}
	return []ChatMessage{{Role: "System", Content: "Deleted memory `" + shortID(id) + "`."}}
}
