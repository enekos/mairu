package scrapegraph

import (
	"context"
	"fmt"
	"strings"

	"mairu/internal/llm"
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

	systemInstruction := `You are an expert prompt engineer for web scraping tasks.
The user has provided a basic prompt describing what data they want to extract from a website.
Your job is to rewrite and refine their prompt to be extremely precise, clear, and unambiguous for a data extraction LLM.
- If they ask for general things, specify that it should be comprehensive.
- Ensure the extraction agent is instructed to format things cleanly.
- Output ONLY the refined prompt text, no pleasantries or explanation.`

	refinedPrompt, err := geminiProvider.GenerateContent(ctx, geminiProvider.GetModelName(), systemInstruction + "\n\nUSER PROMPT: " + userPrompt)
	if err != nil {
		return state, fmt.Errorf("PromptRefinerNode: LLM failed: %w", err)
	}

	refinedPrompt = strings.TrimSpace(refinedPrompt)
	
	// Replace original prompt with refined one
	state["prompt"] = refinedPrompt
	state["original_prompt"] = userPrompt
	
	return state, nil
}
