package cmd

import (
	"encoding/json"
	"sort"
)

func RunNodeStore(project, uri, name, abstract, parent, overview, content string) error {
	out, err := StoreNodeRaw(project, uri, name, abstract, parent, overview, content)
	if err != nil {
		return err
	}
	PrintJSON(out)
	return nil
}

func StoreNodeRaw(project, uri, name, abstract, parent, overview, content string) ([]byte, error) {
	payload := map[string]any{
		"uri":      uri,
		"project":  project,
		"name":     name,
		"abstract": abstract,
		"overview": overview,
		"content":  content,
	}
	if parent != "" {
		payload["parent_uri"] = parent
	}
	return ContextPost("/api/context", payload)
}

func FetchAllNodes(project string) ([]map[string]any, error) {
	out, err := ContextGet("/api/context", map[string]string{
		"project": project,
		"limit":   "5000",
	})
	if err != nil {
		return nil, err
	}
	var nodes []map[string]any
	if err := json.Unmarshal(out, &nodes); err != nil {
		return nil, err
	}
	sort.Slice(nodes, func(i, j int) bool {
		left, _ := nodes[i]["uri"].(string)
		right, _ := nodes[j]["uri"].(string)
		return left < right
	})
	return nodes, nil
}

func RunMemoryStore(project, content, category, owner string, importance int) error {
	out, err := ContextPost("/api/memories", map[string]any{
		"project":    project,
		"content":    content,
		"category":   category,
		"owner":      owner,
		"importance": importance,
	})
	if err != nil {
		return err
	}
	PrintJSON(out)
	return nil
}
