package contextsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"mairu/internal/prompts"
	"math"
	"strings"
)

type LLMClient interface {
	GenerateJSON(ctx context.Context, system, user string) (map[string]any, error)
	GenerateContent(ctx context.Context, model, prompt string) (string, error)
}

const (
	maxSearchPromptChars   = 8000
	maxMutationPromptChars = 16000
	maxContextItems        = 20
	maxContextChars        = 24000
)

func (s *AppService) VibeQuery(prompt, project string, topK int) (VibeQueryResult, error) {
	if strings.TrimSpace(prompt) == "" {
		return VibeQueryResult{}, fmt.Errorf("prompt is required")
	}
	if topK <= 0 {
		topK = 5
	}

	queries := []vibeQueryPlanItem{
		{Store: "memory", Query: prompt},
		{Store: "skill", Query: prompt},
		{Store: "node", Query: prompt},
	}
	reasoning := "Queried memories, skills, and context nodes with the same prompt for broad recall."

	if s.llmClient != nil {
		if sys, err := prompts.Get("vibe_query_planner", struct {
			Project string
		}{
			Project: strings.TrimSpace(project),
		}); err == nil {
			res, err := s.llmClient.GenerateJSON(context.Background(), sys, prompt)
			if err == nil {
				plannedReasoning, plannedQueries := parseSearchPlan(res)
				if len(plannedQueries) > 0 {
					reasoning = plannedReasoning
					queries = plannedQueries
				}
			}
		}
	}

	results := make([]VibeSearchGroup, 0, len(queries))
	for _, q := range queries {
		search, err := s.Search(SearchOptions{
			Query:   q.Query,
			Project: project,
			Store:   q.Store,
			TopK:    topK,
		})
		if err != nil {
			continue
		}
		items := []map[string]any{}
		switch q.Store {
		case "memory":
			items = toAnyMapSlice(search["memories"])
		case "skill":
			items = toAnyMapSlice(search["skills"])
		default:
			items = toAnyMapSlice(search["contextNodes"])
		}
		results = append(results, VibeSearchGroup{Store: q.Store, Query: q.Query, Items: items})
	}
	return VibeQueryResult{Reasoning: reasoning, Results: results}, nil
}

func (s *AppService) PlanVibeMutation(prompt, project string, topK int) (VibeMutationPlan, error) {
	if strings.TrimSpace(prompt) == "" {
		return VibeMutationPlan{}, fmt.Errorf("prompt is required")
	}
	if topK <= 0 {
		topK = 5
	}

	normalizedPrompt := strings.TrimSpace(prompt)
	searchPrompt := truncateForLLM(normalizedPrompt, maxSearchPromptChars)
	mutationPrompt := truncateForLLM(normalizedPrompt, maxMutationPromptChars)

	// Keep a conservative non-LLM fallback for environments without API keys.
	fallback := s.fallbackVibeMutationPlan(prompt, project, topK)

	if s.llmClient == nil {
		return fallback, nil
	}

	contextStr, err := s.gatherBoundedContext(searchPrompt, project, topK)
	if err != nil {
		return fallback, nil
	}

	systemPrompt, err := prompts.Get("vibe_mutation_planner", struct {
		Project    string
		ContextStr string
	}{
		Project:    strings.TrimSpace(project),
		ContextStr: contextStr,
	})
	if err != nil {
		return fallback, nil
	}

	res, err := s.llmClient.GenerateJSON(context.Background(), systemPrompt, "USER PROMPT: "+mutationPrompt)
	if err == nil {
		if parsed, ok := parseMutationPlan(res); ok {
			return parsed, nil
		}
	}

	compactSystemPrompt, err := prompts.Get("vibe_mutation_planner_compact", struct {
		Project                string
		ExistingEntriesSummary string
	}{
		Project:                strings.TrimSpace(project),
		ExistingEntriesSummary: truncateForLLM(contextStr, 8000),
	})
	if err != nil {
		return fallback, nil
	}

	res, err = s.llmClient.GenerateJSON(context.Background(), compactSystemPrompt, "USER PROMPT (possibly truncated): "+truncateForLLM(mutationPrompt, 6000))
	if err == nil {
		if parsed, ok := parseMutationPlan(res); ok {
			return parsed, nil
		}
	}

	return fallback, nil
}

func (s *AppService) ExecuteVibeMutation(ops []VibeMutationOp, project string) ([]map[string]any, error) {
	results := make([]map[string]any, 0, len(ops))
	for _, op := range ops {
		switch op.Op {
		case "create_memory":
			content, _ := op.Data["content"].(string)
			if strings.TrimSpace(content) == "" {
				results = append(results, map[string]any{"op": op.Op, "error": "missing content"})
				continue
			}
			importance := intFromAny(op.Data["importance"], 5)
			mem, err := s.CreateMemory(MemoryCreateInput{
				Project:    firstString(op.Data["project"], project),
				Content:    content,
				Category:   firstString(op.Data["category"], "observation"),
				Owner:      firstString(op.Data["owner"], "agent"),
				Importance: importance,
			})
			if err != nil {
				results = append(results, map[string]any{"op": op.Op, "error": err.Error()})
				continue
			}
			results = append(results, map[string]any{"op": op.Op, "result": "created memory " + mem.ID})
		case "update_memory":
			id := firstString(op.Data["id"], op.Target)
			if id == "" {
				results = append(results, map[string]any{"op": op.Op, "error": "missing id"})
				continue
			}
			updated, err := s.UpdateMemory(MemoryUpdateInput{
				ID:         id,
				Content:    firstString(op.Data["content"], ""),
				Category:   firstString(op.Data["category"], ""),
				Owner:      firstString(op.Data["owner"], ""),
				Importance: intFromAny(op.Data["importance"], 0),
			})
			if err != nil {
				results = append(results, map[string]any{"op": op.Op, "error": err.Error()})
				continue
			}
			results = append(results, map[string]any{"op": op.Op, "result": "updated memory " + updated.ID})
		case "delete_memory":
			id := firstString(op.Data["id"], op.Target)
			if id == "" {
				results = append(results, map[string]any{"op": op.Op, "error": "missing id"})
				continue
			}
			if err := s.DeleteMemory(id); err != nil {
				results = append(results, map[string]any{"op": op.Op, "error": err.Error()})
				continue
			}
			results = append(results, map[string]any{"op": op.Op, "result": "deleted memory " + id})
		case "create_skill":
			skill, err := s.CreateSkill(SkillCreateInput{
				Project:     firstString(op.Data["project"], project),
				Name:        firstString(op.Data["name"], "Derived Skill"),
				Description: firstString(op.Data["description"], ""),
			})
			if err != nil {
				results = append(results, map[string]any{"op": op.Op, "error": err.Error()})
				continue
			}
			results = append(results, map[string]any{"op": op.Op, "result": "created skill " + skill.ID})
		case "update_skill":
			id := firstString(op.Data["id"], op.Target)
			if id == "" {
				results = append(results, map[string]any{"op": op.Op, "error": "missing id"})
				continue
			}
			skill, err := s.UpdateSkill(SkillUpdateInput{
				ID:          id,
				Name:        firstString(op.Data["name"], ""),
				Description: firstString(op.Data["description"], ""),
			})
			if err != nil {
				results = append(results, map[string]any{"op": op.Op, "error": err.Error()})
				continue
			}
			results = append(results, map[string]any{"op": op.Op, "result": "updated skill " + skill.ID})
		case "delete_skill":
			id := firstString(op.Data["id"], op.Target)
			if id == "" {
				results = append(results, map[string]any{"op": op.Op, "error": "missing id"})
				continue
			}
			if err := s.DeleteSkill(id); err != nil {
				results = append(results, map[string]any{"op": op.Op, "error": err.Error()})
				continue
			}
			results = append(results, map[string]any{"op": op.Op, "result": "deleted skill " + id})
		case "create_node":
			uri := firstString(op.Data["uri"], "")
			if uri == "" {
				results = append(results, map[string]any{"op": op.Op, "error": "missing uri"})
				continue
			}
			var parentURI *string
			if parent := firstString(op.Data["parent_uri"], ""); parent != "" {
				parentURI = &parent
			}
			node, err := s.CreateContextNode(ContextCreateInput{
				URI:       uri,
				Project:   firstString(op.Data["project"], project),
				ParentURI: parentURI,
				Name:      firstString(op.Data["name"], ""),
				Abstract:  firstString(op.Data["abstract"], ""),
				Overview:  firstString(op.Data["overview"], ""),
				Content:   firstString(op.Data["content"], ""),
			})
			if err != nil {
				results = append(results, map[string]any{"op": op.Op, "error": err.Error()})
				continue
			}
			results = append(results, map[string]any{"op": op.Op, "result": "created node " + node.URI})
		case "update_node":
			uri := firstString(op.Data["uri"], op.Target)
			if uri == "" {
				results = append(results, map[string]any{"op": op.Op, "error": "missing uri"})
				continue
			}
			node, err := s.UpdateContextNode(ContextUpdateInput{
				URI:      uri,
				Name:     firstString(op.Data["name"], ""),
				Abstract: firstString(op.Data["abstract"], ""),
				Overview: firstString(op.Data["overview"], ""),
				Content:  firstString(op.Data["content"], ""),
			})
			if err != nil {
				results = append(results, map[string]any{"op": op.Op, "error": err.Error()})
				continue
			}
			results = append(results, map[string]any{"op": op.Op, "result": "updated node " + node.URI})
		case "delete_node":
			uri := firstString(op.Data["uri"], op.Target)
			if uri == "" {
				results = append(results, map[string]any{"op": op.Op, "error": "missing uri"})
				continue
			}
			if err := s.DeleteContextNode(uri); err != nil {
				results = append(results, map[string]any{"op": op.Op, "error": err.Error()})
				continue
			}
			results = append(results, map[string]any{"op": op.Op, "result": "deleted node " + uri})
		default:
			results = append(results, map[string]any{"op": op.Op, "error": "unsupported op"})
		}
	}
	return results, nil
}

func (s *AppService) gatherBoundedContext(searchPrompt, project string, topK int) (string, error) {
	queryResult, err := s.VibeQuery(searchPrompt, project, topK)
	if err != nil {
		return "", err
	}
	combined := make([]map[string]any, 0, 16)
	seen := map[string]struct{}{}
	for _, group := range queryResult.Results {
		for _, item := range group.Items {
			key := firstString(item["id"], firstString(item["uri"], ""))
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			merged := make(map[string]any, len(item)+1)
			merged["store"] = group.Store
			for k, v := range item {
				merged[k] = v
			}
			combined = append(combined, merged)
		}
	}
	return buildBoundedContext(combined), nil
}

func (s *AppService) fallbackVibeMutationPlan(prompt, project string, topK int) VibeMutationPlan {
	plan := VibeMutationPlan{
		Reasoning: "Generated a conservative mutation plan from plain-English intent.",
	}
	search, err := s.Search(SearchOptions{
		Query:   prompt,
		Project: project,
		Store:   "memories",
		TopK:    topK,
	})
	if err == nil {
		if existing, ok := bestMemoryMatch(search["memories"], prompt); ok {
			plan.Reasoning = "Existing memory is highly similar, so route to update instead of duplicate create."
			plan.Operations = append(plan.Operations, VibeMutationOp{
				Op:          "update_memory",
				Target:      existing.ID,
				Description: "Update the closest matching memory with refined content.",
				Data: map[string]any{
					"id":       existing.ID,
					"content":  prompt,
					"category": "observation",
					"owner":    "agent",
				},
			})
			return plan
		}
	}

	lower := strings.ToLower(prompt)
	switch {
	case strings.Contains(lower, "remember"):
		plan.Operations = append(plan.Operations, VibeMutationOp{
			Op:          "create_memory",
			Description: "Store the statement as a durable memory.",
			Data: map[string]any{
				"content":    prompt,
				"category":   "observation",
				"owner":      "agent",
				"importance": 5,
				"project":    project,
			},
		})
	case strings.Contains(lower, "skill"):
		plan.Operations = append(plan.Operations, VibeMutationOp{
			Op:          "create_skill",
			Description: "Create a skill from the prompt.",
			Data: map[string]any{
				"name":        "Derived Skill",
				"description": prompt,
				"project":     project,
			},
		})
	default:
		plan.Operations = append(plan.Operations, VibeMutationOp{
			Op:          "create_memory",
			Description: "Default to storing prompt as an observation memory.",
			Data: map[string]any{
				"content":    prompt,
				"category":   "observation",
				"owner":      "agent",
				"importance": 4,
				"project":    project,
			},
		})
	}
	return plan
}

type vibeQueryPlan struct {
	Reasoning string              `json:"reasoning"`
	Queries   []vibeQueryPlanItem `json:"queries"`
}

type vibeQueryPlanItem struct {
	Store string `json:"store"`
	Query string `json:"query"`
}

func parseSearchPlan(raw map[string]any) (string, []vibeQueryPlanItem) {
	b, err := json.Marshal(raw)
	if err != nil {
		return "", nil
	}
	var parsed vibeQueryPlan
	if err := json.Unmarshal(b, &parsed); err != nil {
		return "", nil
	}
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

func parseMutationPlan(raw map[string]any) (VibeMutationPlan, bool) {
	b, err := json.Marshal(raw)
	if err != nil {
		return VibeMutationPlan{}, false
	}
	var parsed VibeMutationPlan
	if err := json.Unmarshal(b, &parsed); err != nil {
		return VibeMutationPlan{}, false
	}
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
