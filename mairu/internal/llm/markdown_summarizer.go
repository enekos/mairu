package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mairu/internal/prompts"
)

type markdownSummary struct {
	Abstract string `json:"abstract"`
	Overview string `json:"overview"`
}

// SummarizeMarkdownDoc calls the LLM to generate a semantic abstract and structured
// overview for a markdown document, optimized for retrieval.
func SummarizeMarkdownDoc(ctx context.Context, gen ContentGenerator, model, filename, content string) (abstract, overview string, err error) {
	if len(content) > MaxInputChars {
		content = content[:MaxInputChars]
	}

	prompt, err := prompts.Render("markdown_summarize", struct {
		Filename string
		Content  string
	}{
		Filename: filename,
		Content:  content,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to render prompt: %w", err)
	}

	raw, err := generateWithRetry(ctx, gen, model, prompt, 1)
	if err != nil {
		return "", "", fmt.Errorf("LLM call failed: %w", err)
	}

	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var result markdownSummary
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return "", "", fmt.Errorf("failed to parse LLM response: %w", err)
	}
	if result.Abstract == "" {
		return "", "", fmt.Errorf("LLM returned empty abstract")
	}
	return result.Abstract, result.Overview, nil
}
