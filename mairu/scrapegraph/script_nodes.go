package scrapegraph

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"mairu/internal/llm"
)

// MinifyHTMLNode strips out irrelevant tags (scripts, styles, svg, paths, comments)
// to compress the HTML while keeping classes/IDs/structure for script generation.
type MinifyHTMLNode struct{}

func (n *MinifyHTMLNode) Name() string { return "MinifyHTMLNode" }

func (n *MinifyHTMLNode) Execute(ctx context.Context, state State) (State, error) {
	htmlContent, ok := state["html"].(string)
	if !ok || htmlContent == "" {
		return state, fmt.Errorf("MinifyHTMLNode: missing 'html' in state")
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return state, fmt.Errorf("MinifyHTMLNode: failed to parse HTML: %w", err)
	}

	// Remove noise
	doc.Find("script, style, svg, path, noscript, meta, link, iframe").Remove()

	minifiedHTML, err := doc.Html()
	if err != nil {
		return state, fmt.Errorf("MinifyHTMLNode: failed to render HTML: %w", err)
	}

	// Collapse spaces/newlines to save context
	reSpace := regexp.MustCompile(`\s+`)
	minifiedHTML = reSpace.ReplaceAllString(minifiedHTML, " ")
	minifiedHTML = strings.ReplaceAll(minifiedHTML, "> <", "><")

	state["minified_html"] = minifiedHTML
	return state, nil
}

// GenerateScriptNode asks the LLM to generate a Go script using goquery
type GenerateScriptNode struct {
	Provider *llm.GeminiProvider
}

func (n *GenerateScriptNode) Name() string { return "GenerateScriptNode" }

func (n *GenerateScriptNode) Execute(ctx context.Context, state State) (State, error) {
	htmlContent, ok := state["minified_html"].(string)
	if !ok || htmlContent == "" {
		return state, fmt.Errorf("GenerateScriptNode: missing 'minified_html' in state")
	}
	userPrompt, ok := state["prompt"].(string)
	if !ok || userPrompt == "" {
		return state, fmt.Errorf("GenerateScriptNode: missing 'prompt' in state")
	}
	targetURL, _ := state["url"].(string)

	geminiProvider := n.Provider
	if geminiProvider == nil {
		return state, fmt.Errorf("GenerateScriptNode: missing GeminiProvider")
	}

	// Cap HTML to fit in prompt (gemini can handle quite a bit, but 80k chars is safe)
	if len(htmlContent) > 80000 {
		htmlContent = htmlContent[:80000] + "...[truncated]"
	}

	systemInstruction := "You are an expert Go developer. Generate a self-contained Go script that extracts the requested data from the provided HTML structure.\n" +
		"The script MUST:\n" +
		"1. Be a valid, self-contained main.go file (package main).\n" +
		"2. Use \"github.com/PuerkitoBio/goquery\" and standard libraries.\n" +
		"3. Be robust to missing elements.\n" +
		"4. Output the extracted data as pretty-printed JSON to stdout.\n" +
		"5. Print ONLY the valid Go code, without markdown formatting like \"```go\" or any conversational text."

	fullPrompt := fmt.Sprintf("TARGET URL: %s\n\nPROMPT: %s\n\nHTML STRUCTURE:\n%s", targetURL, userPrompt, htmlContent)

	script, err := geminiProvider.GenerateContent(ctx, geminiProvider.GetModelName(), systemInstruction+"\n\n"+fullPrompt)
	if err != nil {
		return state, fmt.Errorf("GenerateScriptNode: LLM failed: %w", err)
	}

	// Clean up markdown blocks if the LLM adds them anyway
	script = strings.TrimPrefix(script, "```go\n")
	script = strings.TrimPrefix(script, "```go")
	script = strings.TrimSuffix(script, "```\n")
	script = strings.TrimSuffix(script, "```")
	script = strings.TrimSpace(script)

	state["generated_script"] = script
	return state, nil
}
