package crawler

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
	"mairu/internal/prompts"
)

const maxInputChars = 8000 * 4
const shortPageThreshold = 5

func truncateMarkdown(markdown string) string {
	if len(markdown) <= maxInputChars {
		return markdown
	}
	return markdown[:maxInputChars] + "\n\n[content truncated]"
}

func buildPrompt(title, markdown, url string) (string, error) {
	return prompts.Render("scraper_page_summarize", struct {
		URL      string
		Title    string
		Markdown string
	}{
		URL:      url,
		Title:    title,
		Markdown: truncateMarkdown(markdown),
	})
}

func fallbackSummary(title, markdown, url string) PageSummary {
	firstLine := title
	lines := strings.Split(markdown, "\n")
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			firstLine = l
			break
		}
	}
	abstract := title + " (" + url + "): " + firstLine
	if len(abstract) > 200 {
		abstract = abstract[:200]
	}
	overview := markdown
	if len(overview) > 500 {
		overview = overview[:500]
	}
	return PageSummary{
		Abstract:       abstract,
		Overview:       overview,
		AIIntent:       nil,
		AITopics:       []string{},
		AIQualityScore: 5,
	}
}

// SummarizePage summarizes the given markdown content using Gemini LLM.
func SummarizePage(ctx context.Context, apiKey, title, markdown, url string) PageSummary {
	words := strings.Fields(markdown)
	if len(words) < shortPageThreshold || apiKey == "" {
		return fallbackSummary(title, markdown, url)
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return fallbackSummary(title, markdown, url)
	}
	defer client.Close()

	prompt, err := buildPrompt(title, markdown, url)
	if err != nil {
		return fallbackSummary(title, markdown, url)
	}
	model := client.GenerativeModel("gemini-2.5-flash") // Default model

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil || resp == nil || len(resp.Candidates) == 0 {
		return fallbackSummary(title, markdown, url)
	}

	var text string
	for _, part := range resp.Candidates[0].Content.Parts {
		if t, ok := part.(genai.Text); ok {
			text += string(t)
		}
	}

	text = strings.TrimSpace(text)
	reFences := regexp.MustCompile(`(?s)^${1,3}(?:json)?\n?(.*?)\n?${1,3}$`)
	if match := reFences.FindStringSubmatch(text); len(match) > 1 {
		text = match[1]
	} else {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
	}

	var parsed PageSummary
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return fallbackSummary(title, markdown, url)
	}

	if parsed.Abstract == "" {
		parsed.Abstract = fallbackSummary(title, markdown, url).Abstract
	}
	if parsed.Overview == "" {
		parsed.Overview = fallbackSummary(title, markdown, url).Overview
	}

	return parsed
}
