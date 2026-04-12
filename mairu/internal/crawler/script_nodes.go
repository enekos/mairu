package crawler

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"mairu/internal/llm"
	"mairu/internal/prompts"
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
	Provider llm.Provider
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

	systemInstruction := prompts.Render("crawler_script_generator_sys", nil)

	fullPrompt := prompts.Render("crawler_script_generator_user", map[string]any{
		"TargetURL":   targetURL,
		"UserPrompt":  userPrompt,
		"HTMLContent": htmlContent,
	})

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
