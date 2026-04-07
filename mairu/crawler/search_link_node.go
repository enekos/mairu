package crawler

import (
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"mairu/internal/llm"
	"mairu/internal/prompts"
)

// SearchLinkNode extracts all links from a webpage and uses the LLM to filter
// and return only the ones relevant to the user prompt.
type SearchLinkNode struct {
	Provider *llm.GeminiProvider
}

func (n *SearchLinkNode) Name() string { return "SearchLinkNode" }

func (n *SearchLinkNode) Execute(ctx context.Context, state State) (State, error) {
	htmlContent, ok := state["html"].(string)
	if !ok || htmlContent == "" {
		return state, fmt.Errorf("SearchLinkNode: missing 'html' in state")
	}
	userPrompt, ok := state["prompt"].(string)
	if !ok || userPrompt == "" {
		return state, fmt.Errorf("SearchLinkNode: missing 'prompt' in state")
	}

	geminiProvider := n.Provider
	if geminiProvider == nil {
		return state, fmt.Errorf("SearchLinkNode: missing GeminiProvider")
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return state, fmt.Errorf("SearchLinkNode: failed to parse HTML: %w", err)
	}

	var allLinks []string
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && href != "" && !strings.HasPrefix(href, "#") && !strings.HasPrefix(href, "javascript:") {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				allLinks = append(allLinks, fmt.Sprintf("%s (URL: %s)", text, href))
			} else {
				allLinks = append(allLinks, href)
			}
		}
	})

	if len(allLinks) == 0 {
		state["relevant_links"] = []string{}
		return state, nil
	}

	linksText := strings.Join(allLinks, "\n")
	if len(linksText) > 60000 {
		linksText = linksText[:60000] + "\n...[truncated]"
	}

	systemInstruction := prompts.Render("crawler_search_link_sys", nil)

	fullPrompt := prompts.Render("crawler_search_link_user", map[string]any{
		"UserPrompt": userPrompt,
		"LinksText":  linksText,
	})

	var result []string
	err = geminiProvider.GenerateJSON(ctx, systemInstruction, fullPrompt, nil, &result)
	if err != nil {
		return state, fmt.Errorf("SearchLinkNode: LLM failed: %w", err)
	}

	state["relevant_links"] = result
	return state, nil
}
