package contextsrv

import (
	"context"
	"fmt"
	"mairu/internal/llm"
	"mairu/internal/prompts"
	"strings"
)

// LLMClient defines the interface for LLM operations used by the context service
type LLMClient interface {
	llm.RouterLLMClient
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
		{Store: StoreMemory, Query: prompt},
		{Store: StoreSkill, Query: prompt},
		{Store: StoreNode, Query: prompt},
	}
	reasoning := "Queried memories, skills, and context nodes with the same prompt for broad recall."

	if s.llmClient != nil {
		if sys, err := prompts.Get("vibe_query_planner", struct {
			Project string
		}{
			Project: strings.TrimSpace(project),
		}); err == nil {
			var res vibeQueryPlan
			err := s.llmClient.GenerateJSON(context.Background(), sys, prompt, nil, &res)
			if err == nil {
				plannedReasoning, plannedQueries := validateSearchPlan(res)
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
		var items []map[string]any
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

	var res VibeMutationPlan
	err = s.llmClient.GenerateJSON(context.Background(), systemPrompt, "USER PROMPT: "+mutationPrompt, nil, &res)
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

	err = s.llmClient.GenerateJSON(context.Background(), compactSystemPrompt, "USER PROMPT (possibly truncated): "+truncateForLLM(mutationPrompt, 6000), nil, &res)
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
		Store:   StoreMemories,
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
