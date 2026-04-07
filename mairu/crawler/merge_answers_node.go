package crawler

import (
	"context"
	"encoding/json"
	"fmt"

	"mairu/internal/llm"
	"mairu/internal/prompts"
)

// MergeAnswersNode merges extracted data from multiple sources into a single coherent JSON object.
type MergeAnswersNode struct {
	Provider *llm.GeminiProvider
}

func (n *MergeAnswersNode) Name() string { return "MergeAnswersNode" }

func (n *MergeAnswersNode) Execute(ctx context.Context, state State) (State, error) {
	results, ok := state["results"].(map[string]map[string]any)
	if !ok || len(results) == 0 {
		return state, fmt.Errorf("MergeAnswersNode: missing or empty 'results' in state")
	}
	userPrompt, ok := state["prompt"].(string)
	if !ok || userPrompt == "" {
		return state, fmt.Errorf("MergeAnswersNode: missing 'prompt' in state")
	}

	geminiProvider := n.Provider
	if geminiProvider == nil {
		return state, fmt.Errorf("MergeAnswersNode: missing GeminiProvider")
	}

	resultsJSON, _ := json.MarshalIndent(results, "", "  ")
	resultsStr := string(resultsJSON)

	// Cap to fit in prompt if extremely large
	if len(resultsStr) > 80000 {
		resultsStr = resultsStr[:80000] + "\n...[truncated]"
	}

	systemInstruction := prompts.Render("crawler_merge_answers_sys", nil)

	fullPrompt := prompts.Render("crawler_merge_answers_user", map[string]any{
		"UserPrompt": userPrompt,
		"Results":    resultsStr,
	})

	var mergedResult map[string]any
	err := geminiProvider.GenerateJSON(ctx, systemInstruction, fullPrompt, nil, &mergedResult)
	if err != nil {
		return state, fmt.Errorf("MergeAnswersNode: LLM failed: %w", err)
	}

	state["merged_data"] = mergedResult
	return state, nil
}
