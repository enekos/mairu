package crawler

import (
	"context"
	"fmt"
	"strings"

	"mairu/internal/llm"
	"mairu/internal/prompts"
)

// PromptRefinerNode uses the LLM to refine the user's scraping prompt,
// expanding it to be more precise for the extraction phase.
type PromptRefinerNode struct {
	Provider *llm.GeminiProvider
}

func (n *PromptRefinerNode) Name() string { return "PromptRefinerNode" }

func (n *PromptRefinerNode) Execute(ctx context.Context, state State) (State, error) {
	userPrompt, ok := state["prompt"].(string)
	if !ok || userPrompt == "" {
		return state, fmt.Errorf("PromptRefinerNode: missing 'prompt' in state")
	}

	geminiProvider := n.Provider
	if geminiProvider == nil {
		return state, fmt.Errorf("PromptRefinerNode: missing GeminiProvider")
	}

	systemInstruction := prompts.Render("crawler_prompt_refiner_sys", nil)
	userPromptStr := prompts.Render("crawler_prompt_refiner_user", map[string]any{
		"UserPrompt": userPrompt,
	})

	refinedPrompt, err := geminiProvider.GenerateContent(ctx, geminiProvider.GetModelName(), systemInstruction+"\n\n"+userPromptStr)
	if err != nil {
		return state, fmt.Errorf("PromptRefinerNode: LLM failed: %w", err)
	}

	refinedPrompt = strings.TrimSpace(refinedPrompt)

	// Replace original prompt with refined one
	state["prompt"] = refinedPrompt
	state["original_prompt"] = userPrompt

	return state, nil
}
