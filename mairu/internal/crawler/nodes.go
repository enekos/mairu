package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	markdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
	"mairu/internal/llm"
	"mairu/internal/prompts"
)

// FetchNode fetches HTML content from a URL
type FetchNode struct{}

func (n *FetchNode) Name() string { return "FetchNode" }

func (n *FetchNode) Execute(ctx context.Context, state State) (State, error) {
	targetURL, ok := state["url"].(string)
	if !ok || targetURL == "" {
		return state, fmt.Errorf("FetchNode: missing or invalid 'url' in state")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return state, fmt.Errorf("FetchNode: failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "mairu-crawler/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return state, fmt.Errorf("FetchNode: fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return state, fmt.Errorf("FetchNode: bad status code %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return state, fmt.Errorf("FetchNode: read failed: %w", err)
	}

	state["html"] = string(bodyBytes)
	return state, nil
}

// ParseNode extracts the main content from HTML using go-readability
type ParseNode struct{}

func (n *ParseNode) Name() string { return "ParseNode" }

func (n *ParseNode) Execute(ctx context.Context, state State) (State, error) {
	htmlContent, ok := state["html"].(string)
	if !ok || htmlContent == "" {
		return state, fmt.Errorf("ParseNode: missing 'html' in state")
	}

	// Quick check if it's likely JSON or XML
	trimmed := strings.TrimSpace(htmlContent)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") || (strings.HasPrefix(trimmed, "<?xml") || strings.HasPrefix(trimmed, "<rss")) {
		// Bypass readability for pure data formats
		state["doc"] = trimmed
		return state, nil
	}

	// Simple heuristic for CSV (if there are commas on the first line and it's not HTML)
	if !strings.HasPrefix(trimmed, "<") && strings.Contains(strings.SplitN(trimmed, "\n", 2)[0], ",") {
		state["doc"] = trimmed
		return state, nil
	}

	targetURL, _ := state["url"].(string)

	parsedURL, _ := url.Parse(targetURL)
	if parsedURL == nil {
		parsedURL, _ = url.Parse("http://localhost")
	}

	article, err := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if err != nil {
		// Fallback to naive text extraction if readability fails
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
		if err == nil {
			state["doc"] = strings.TrimSpace(doc.Text())
			return state, nil
		}
		return state, fmt.Errorf("ParseNode: readability failed: %w", err)
	}

	// Try to convert to Markdown for better LLM context
	md, err := markdown.ConvertString(article.Content)
	if err == nil && md != "" {
		state["doc"] = md
	} else {
		state["doc"] = article.TextContent
	}
	return state, nil
}

// ExtractNode uses an LLM to extract structured data based on a prompt
type ExtractNode struct {
	Provider llm.Provider
}

func (n *ExtractNode) Name() string { return "ExtractNode" }

func (n *ExtractNode) Execute(ctx context.Context, state State) (State, error) {
	doc, ok := state["doc"].(string)
	if !ok || doc == "" {
		return state, fmt.Errorf("ExtractNode: missing 'doc' in state")
	}
	userPrompt, ok := state["prompt"].(string)
	if !ok || userPrompt == "" {
		return state, fmt.Errorf("ExtractNode: missing 'prompt' in state")
	}

	geminiProvider := n.Provider
	if geminiProvider == nil {
		return state, fmt.Errorf("ExtractNode: missing GeminiProvider")
	}

	systemInstruction, err := prompts.Render("crawler_extract_sys", nil)
	if err != nil {
		return state, fmt.Errorf("ExtractNode: failed to render prompt: %w", err)
	}

	// Ensure we don't blow up context size (cap at ~60k chars roughly)
	if len(doc) > 60000 {
		doc = doc[:60000] + "\n...[truncated]"
	}

	fullPrompt, err := prompts.Render("crawler_extract_user", map[string]any{
		"Doc":        doc,
		"UserPrompt": userPrompt,
	})
	if err != nil {
		return state, fmt.Errorf("ExtractNode: failed to render prompt: %w", err)
	}

	var result map[string]any
	err = geminiProvider.GenerateJSON(ctx, systemInstruction, fullPrompt, nil, &result)
	if err != nil {
		return state, fmt.Errorf("ExtractNode: LLM extraction failed: %w", err)
	}

	state["extracted_data"] = result
	return state, nil
}

// SearchNode searches DuckDuckGo for a query and extracts result URLs
type SearchNode struct {
	MaxResults int
	SearchURL  string // Optional: Override for testing
}

func (n *SearchNode) Name() string { return "SearchNode" }

func (n *SearchNode) Execute(ctx context.Context, state State) (State, error) {
	query, ok := state["search_query"].(string)
	if !ok || query == "" {
		return state, fmt.Errorf("SearchNode: missing 'search_query' in state")
	}

	searchURL := n.SearchURL
	if searchURL == "" {
		searchURL = fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return state, fmt.Errorf("SearchNode: failed to create request: %w", err)
	}
	// DDG requires a real-looking user agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return state, fmt.Errorf("SearchNode: fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return state, fmt.Errorf("SearchNode: bad status code %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return state, fmt.Errorf("SearchNode: parse failed: %w", err)
	}

	var urls []string
	doc.Find("a.result__url").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			// DDG redirects look like //duckduckgo.com/l/?rut=...
			if strings.HasPrefix(href, "//duckduckgo.com/l/?") {
				u, err := url.Parse("https:" + href)
				if err == nil {
					target := u.Query().Get("uddg")
					if target != "" {
						urls = append(urls, target)
					}
				}
			} else {
				urls = append(urls, href)
			}
		}
	})

	if len(urls) > n.MaxResults && n.MaxResults > 0 {
		urls = urls[:n.MaxResults]
	}

	state["search_results"] = urls
	return state, nil
}
