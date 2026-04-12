package contextsrv

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type vibeQueryPlan struct {
	Reasoning string              `json:"reasoning"`
	Queries   []vibeQueryPlanItem `json:"queries"`
}

type vibeQueryPlanItem struct {
	Store string `json:"store"`
	Query string `json:"query"`
}

func validateSearchPlan(parsed vibeQueryPlan) (string, []vibeQueryPlanItem) {
	valid := make([]vibeQueryPlanItem, 0, len(parsed.Queries))
	for _, q := range parsed.Queries {
		if strings.TrimSpace(q.Query) == "" {
			continue
		}
		store := strings.ToLower(strings.TrimSpace(q.Store))
		if store != "memory" && store != "skill" && store != "node" {
			continue
		}
		valid = append(valid, vibeQueryPlanItem{
			Store: store,
			Query: q.Query,
		})
	}
	return parsed.Reasoning, valid
}

func parseMutationPlan(parsed VibeMutationPlan) (VibeMutationPlan, bool) {
	if len(parsed.Operations) == 0 {
		return VibeMutationPlan{Reasoning: parsed.Reasoning, Operations: []VibeMutationOp{}}, true
	}
	validOps := map[string]struct{}{
		"create_memory": {}, "update_memory": {}, "delete_memory": {},
		"create_skill": {}, "update_skill": {}, "delete_skill": {},
		"create_node": {}, "update_node": {}, "delete_node": {},
	}
	filtered := make([]VibeMutationOp, 0, len(parsed.Operations))
	for _, op := range parsed.Operations {
		if _, ok := validOps[op.Op]; !ok {
			continue
		}
		if strings.TrimSpace(op.Description) == "" {
			continue
		}
		if op.Data == nil {
			op.Data = map[string]any{}
		}
		filtered = append(filtered, op)
	}
	return VibeMutationPlan{
		Reasoning:  parsed.Reasoning,
		Operations: filtered,
	}, true
}

func truncateForLLM(text string, maxChars int) string {
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	marker := fmt.Sprintf("\n...[truncated %d chars]...\n", len(text)-maxChars)
	headLen := int(math.Max(0, math.Floor(float64(maxChars-len(marker))*0.75)))
	tailLen := int(math.Max(0, float64(maxChars-len(marker)-headLen)))
	return text[:headLen] + marker + text[len(text)-tailLen:]
}

func buildBoundedContext(entries []map[string]any) string {
	if len(entries) == 0 {
		return "(no existing entries found)"
	}
	compact := make([]map[string]any, 0, min(maxContextItems, len(entries)))
	for _, item := range entries {
		if len(compact) >= maxContextItems {
			break
		}
		trimmed := map[string]any{}
		for k, v := range item {
			if k == "_score" || k == "embedding" || k == "ancestors" {
				continue
			}
			trimmed[k] = v
		}
		compact = append(compact, trimmed)
	}
	serializedBytes, _ := json.MarshalIndent(compact, "", "  ")
	serialized := string(serializedBytes)
	if len(serialized) <= maxContextChars {
		omitted := len(entries) - len(compact)
		if omitted > 0 {
			return serialized + fmt.Sprintf("\n\n(Note: %d additional entries omitted due to context size limits.)", omitted)
		}
		return serialized
	}
	bounded := make([]map[string]any, 0, len(compact))
	for _, item := range compact {
		candidate := append(append([]map[string]any{}, bounded...), item)
		nextBytes, _ := json.MarshalIndent(candidate, "", "  ")
		next := string(nextBytes)
		if len(next) > maxContextChars {
			break
		}
		bounded = append(bounded, item)
		serialized = next
	}
	omitted := len(entries) - len(bounded)
	if omitted > 0 {
		return serialized + fmt.Sprintf("\n\n(Note: %d additional entries omitted due to context size limits.)", omitted)
	}
	return serialized
}

type memorySearchHit struct {
	ID      string
	Content string
}

func bestMemoryMatch(raw any, prompt string) (memorySearchHit, bool) {
	items := toAnyMapSlice(raw)
	best := memorySearchHit{}
	bestScore := 0.0
	for _, item := range items {
		id, _ := item["id"].(string)
		content, _ := item["content"].(string)
		if id == "" || strings.TrimSpace(content) == "" {
			continue
		}
		score := textSimilarity(content, prompt)
		if score > bestScore {
			bestScore = score
			best = memorySearchHit{ID: id, Content: content}
		}
	}
	return best, bestScore >= 0.85
}

func textSimilarity(a, b string) float64 {
	setA := tokenSet(a)
	setB := tokenSet(b)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	inter := 0
	for token := range setA {
		if _, ok := setB[token]; ok {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / math.Max(float64(union), 1)
}

func tokenSet(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range strings.Fields(strings.ToLower(s)) {
		token = strings.Trim(token, ".,:;!?()[]{}\"'`")
		if token == "" {
			continue
		}
		out[token] = struct{}{}
	}
	return out
}

func toAnyMapSlice(v any) []map[string]any {
	if v == nil {
		return []map[string]any{}
	}
	if direct, ok := v.([]map[string]any); ok {
		return direct
	}
	out := []map[string]any{}
	switch arr := v.(type) {
	case []any:
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

func firstString(v any, fallback string) string {
	if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return fallback
}

func intFromAny(v any, fallback int) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return fallback
	}
}
