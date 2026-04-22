package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"mairu/internal/contextsrv"
	"mairu/internal/llm"
	"mairu/internal/prompts"
)

// ============================================================================
// Core Nodes
// ============================================================================

// FetchNode fetches HTML content from a URL using the Engine.
type FetchNode struct {
	Engine *Engine
}

func (n *FetchNode) Name() string { return "FetchNode" }

func (n *FetchNode) Execute(ctx context.Context, state State) (State, error) {
	targetURL, ok := state["url"].(string)
	if !ok || targetURL == "" {
		return state, fmt.Errorf("FetchNode: missing or invalid 'url' in state")
	}
	if n.Engine == nil {
		return state, fmt.Errorf("FetchNode: missing Engine")
	}

	content, err := n.Engine.Fetch(ctx, targetURL)
	if err != nil {
		return state, fmt.Errorf("FetchNode: %w", err)
	}

	state["html"] = content
	return state, nil
}

// ParseNode extracts the main content from HTML using the Engine.
type ParseNode struct {
	Engine *Engine
}

func (n *ParseNode) Name() string { return "ParseNode" }

func (n *ParseNode) Execute(ctx context.Context, state State) (State, error) {
	htmlContent, ok := state["html"].(string)
	if !ok || htmlContent == "" {
		return state, fmt.Errorf("ParseNode: missing 'html' in state")
	}
	if n.Engine == nil {
		return state, fmt.Errorf("ParseNode: missing Engine")
	}

	targetURL, _ := state["url"].(string)
	doc, err := n.Engine.Parse(ctx, htmlContent, targetURL)
	if err != nil {
		return state, fmt.Errorf("ParseNode: %w", err)
	}

	state["doc"] = doc
	return state, nil
}

// ExtractNode uses an LLM to extract structured data based on a prompt.
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
	if n.Provider == nil {
		return state, fmt.Errorf("ExtractNode: missing Provider")
	}

	systemInstruction, err := prompts.Render("crawler_extract_sys", nil)
	if err != nil {
		return state, fmt.Errorf("ExtractNode: failed to render prompt: %w", err)
	}

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
	err = n.Provider.GenerateJSON(ctx, systemInstruction, fullPrompt, nil, &result)
	if err != nil {
		return state, fmt.Errorf("ExtractNode: LLM extraction failed: %w", err)
	}

	state["extracted_data"] = result
	return state, nil
}

// SearchNode searches DuckDuckGo for a query and extracts result URLs.
type SearchNode struct {
	Engine     *Engine
	MaxResults int
	SearchURL  string
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

// ============================================================================
// Ingest Nodes
// ============================================================================

// MarkdownExtractNode extracts Markdown from HTML.
type MarkdownExtractNode struct {
	Selector string
}

func (n *MarkdownExtractNode) Name() string { return "MarkdownExtract" }

func (n *MarkdownExtractNode) Execute(_ context.Context, state State) (State, error) {
	html, ok := state["html"].(string)
	if !ok {
		return state, fmt.Errorf("MarkdownExtractNode: missing html in state")
	}
	urlStr, _ := state["url"].(string)

	content := ExtractContent(html, n.Selector, urlStr)
	if content.Markdown == "" {
		state["skipped"] = true
		return state, nil
	}

	state["title"] = content.Title
	state["markdown"] = content.Markdown
	return state, nil
}

// SummarizeNode generates a summary using the configured LLM provider.
type SummarizeNode struct {
	Provider llm.Provider
}

func (n *SummarizeNode) Name() string { return "Summarize" }

func (n *SummarizeNode) Execute(ctx context.Context, state State) (State, error) {
	if skipped, _ := state["skipped"].(bool); skipped {
		return state, nil
	}

	title, _ := state["title"].(string)
	md, _ := state["markdown"].(string)
	urlStr, _ := state["url"].(string)

	summary := SummarizePage(ctx, n.Provider, title, md, urlStr)
	state["abstract"] = summary.Abstract
	state["overview"] = summary.Overview
	return state, nil
}

// NodeStoreFunc is the callback used by StoreContextNode.
type NodeStoreFunc func(ctx context.Context, input contextsrv.ContextCreateInput) error

// StoreContextNode persists results via the provided store function.
type StoreContextNode struct {
	StoreFn NodeStoreFunc
	Project string
	DryRun  bool
}

func (n *StoreContextNode) Name() string { return "StoreContext" }

func (n *StoreContextNode) Execute(ctx context.Context, state State) (State, error) {
	if skipped, _ := state["skipped"].(bool); skipped || n.DryRun || n.StoreFn == nil {
		return state, nil
	}

	urlStr, _ := state["url"].(string)
	title, _ := state["title"].(string)
	md, _ := state["markdown"].(string)
	abstract, _ := state["abstract"].(string)
	overview, _ := state["overview"].(string)

	uri := URLToURI(urlStr)
	parentURI := URLToParentURI(urlStr)

	err := n.StoreFn(ctx, contextsrv.ContextCreateInput{
		URI:       uri,
		Project:   n.Project,
		ParentURI: parentURI,
		Name:      title,
		Abstract:  abstract,
		Overview:  overview,
		Content:   md,
	})
	if err != nil {
		return state, err
	}

	state["stored"] = true
	return state, nil
}

// ============================================================================
// RAG Extraction
// ============================================================================

// RAGExtractNode uses embeddings to find the most relevant chunks of a large
// document before asking the LLM to extract data.
type RAGExtractNode struct {
	Provider    llm.Provider
	Embedder    llm.Embedder
	ChunkSize   int
	TopK        int
	Concurrency int
}

func (n *RAGExtractNode) Name() string { return "RAGExtractNode" }

func (n *RAGExtractNode) Execute(ctx context.Context, state State) (State, error) {
	doc, ok := state["doc"].(string)
	if !ok || doc == "" {
		return state, fmt.Errorf("RAGExtractNode: missing 'doc' in state")
	}
	userPrompt, ok := state["prompt"].(string)
	if !ok || userPrompt == "" {
		return state, fmt.Errorf("RAGExtractNode: missing 'prompt' in state")
	}
	if n.Provider == nil {
		return state, fmt.Errorf("RAGExtractNode: missing Provider")
	}

	if n.ChunkSize <= 0 {
		n.ChunkSize = 4000
	}
	if n.TopK <= 0 {
		n.TopK = 5
	}
	if n.Concurrency <= 0 {
		n.Concurrency = 5
	}

	chunks := chunkText(doc, n.ChunkSize)
	if len(chunks) <= n.TopK {
		extractNode := &ExtractNode{Provider: n.Provider}
		return extractNode.Execute(ctx, state)
	}

	if n.Embedder == nil {
		return state, fmt.Errorf("RAGExtractNode: missing Embedder")
	}
	promptEmb, err := n.Embedder.GetEmbedding(ctx, userPrompt)
	if err != nil {
		return state, fmt.Errorf("RAGExtractNode: failed to embed prompt: %w", err)
	}

	type chunkScore struct {
		text  string
		score float32
		index int
	}

	scores := make([]chunkScore, len(chunks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, n.Concurrency)
	var firstErr error
	var errMu sync.Mutex

	for i, text := range chunks {
		wg.Add(1)
		go func(idx int, chunk string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			emb, err := n.Embedder.GetEmbedding(ctx, chunk)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
			}
			scores[idx] = chunkScore{text: chunk, score: cosineSimilarity(promptEmb, emb), index: idx}
		}(i, text)
	}

	wg.Wait()
	if firstErr != nil {
		return state, fmt.Errorf("RAGExtractNode: failed embedding chunks: %w", firstErr)
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	topK := n.TopK
	if topK > len(scores) {
		topK = len(scores)
	}
	selected := scores[:topK]
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].index < selected[j].index
	})

	var b strings.Builder
	for _, s := range selected {
		b.WriteString(s.text)
		b.WriteString("\n...\n")
	}
	combined := b.String()

	state["doc"] = combined
	extractNode := &ExtractNode{Provider: n.Provider}
	return extractNode.Execute(ctx, state)
}

func chunkText(text string, chunkSize int) []string {
	var chunks []string
	runes := []rune(text)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

func cosineSimilarity(a, b []float32) float32 {
	var dotProduct, normA, normB float32
	for i := 0; i < len(a) && i < len(b); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// ============================================================================
// Merge & Refine Nodes
// ============================================================================

// MergeAnswersNode merges extracted data from multiple sources into a single
// coherent JSON object.
type MergeAnswersNode struct {
	Provider llm.Provider
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
	if n.Provider == nil {
		return state, fmt.Errorf("MergeAnswersNode: missing Provider")
	}

	resultsJSON, _ := json.MarshalIndent(results, "", "  ")
	resultsStr := string(resultsJSON)
	if len(resultsStr) > 80000 {
		resultsStr = resultsStr[:80000] + "\n...[truncated]"
	}

	systemInstruction, err := prompts.Render("crawler_merge_answers_sys", nil)
	if err != nil {
		return state, fmt.Errorf("MergeAnswersNode: failed to render prompt: %w", err)
	}

	fullPrompt, err := prompts.Render("crawler_merge_answers_user", map[string]any{
		"UserPrompt": userPrompt,
		"Results":    resultsStr,
	})
	if err != nil {
		return state, fmt.Errorf("MergeAnswersNode: failed to render prompt: %w", err)
	}

	var mergedResult map[string]any
	err = n.Provider.GenerateJSON(ctx, systemInstruction, fullPrompt, nil, &mergedResult)
	if err != nil {
		return state, fmt.Errorf("MergeAnswersNode: LLM failed: %w", err)
	}

	state["merged_data"] = mergedResult
	return state, nil
}

// PromptRefinerNode uses the LLM to refine the user's scraping prompt.
type PromptRefinerNode struct {
	Provider llm.Provider
}

func (n *PromptRefinerNode) Name() string { return "PromptRefinerNode" }

func (n *PromptRefinerNode) Execute(ctx context.Context, state State) (State, error) {
	userPrompt, ok := state["prompt"].(string)
	if !ok || userPrompt == "" {
		return state, fmt.Errorf("PromptRefinerNode: missing 'prompt' in state")
	}
	if n.Provider == nil {
		return state, fmt.Errorf("PromptRefinerNode: missing Provider")
	}

	systemInstruction, err := prompts.Render("crawler_prompt_refiner_sys", nil)
	if err != nil {
		return state, fmt.Errorf("PromptRefinerNode: failed to render prompt: %w", err)
	}
	userPromptStr, err := prompts.Render("crawler_prompt_refiner_user", map[string]any{
		"UserPrompt": userPrompt,
	})
	if err != nil {
		return state, fmt.Errorf("PromptRefinerNode: failed to render prompt: %w", err)
	}

	refinedPrompt, err := n.Provider.GenerateContent(ctx, n.Provider.GetModelName(), systemInstruction+"\n\n"+userPromptStr)
	if err != nil {
		return state, fmt.Errorf("PromptRefinerNode: LLM failed: %w", err)
	}

	refinedPrompt = strings.TrimSpace(refinedPrompt)
	state["prompt"] = refinedPrompt
	state["original_prompt"] = userPrompt
	return state, nil
}

// ============================================================================
// Link Search Node
// ============================================================================

// SearchLinkNode extracts all links from a webpage and uses the LLM to filter
// and return only the ones relevant to the user prompt.
type SearchLinkNode struct {
	Provider llm.Provider
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
	if n.Provider == nil {
		return state, fmt.Errorf("SearchLinkNode: missing Provider")
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

	systemInstruction, err := prompts.Render("crawler_search_link_sys", nil)
	if err != nil {
		return state, fmt.Errorf("SearchLinkNode: failed to render prompt: %w", err)
	}

	fullPrompt, err := prompts.Render("crawler_search_link_user", map[string]any{
		"UserPrompt": userPrompt,
		"LinksText":  linksText,
	})
	if err != nil {
		return state, fmt.Errorf("SearchLinkNode: failed to render prompt: %w", err)
	}

	var result []string
	err = n.Provider.GenerateJSON(ctx, systemInstruction, fullPrompt, nil, &result)
	if err != nil {
		return state, fmt.Errorf("SearchLinkNode: LLM failed: %w", err)
	}

	state["relevant_links"] = result
	return state, nil
}

// ============================================================================
// Script Generation Nodes
// ============================================================================

// MinifyHTMLNode strips out irrelevant tags to compress HTML for script generation.
type MinifyHTMLNode struct{}

func (n *MinifyHTMLNode) Name() string { return "MinifyHTMLNode" }

func (n *MinifyHTMLNode) Execute(_ context.Context, state State) (State, error) {
	htmlContent, ok := state["html"].(string)
	if !ok || htmlContent == "" {
		return state, fmt.Errorf("MinifyHTMLNode: missing 'html' in state")
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return state, fmt.Errorf("MinifyHTMLNode: failed to parse HTML: %w", err)
	}

	doc.Find("script, style, svg, path, noscript, meta, link, iframe").Remove()

	minifiedHTML, err := doc.Html()
	if err != nil {
		return state, fmt.Errorf("MinifyHTMLNode: failed to render HTML: %w", err)
	}

	reSpace := regexp.MustCompile(`\s+`)
	minifiedHTML = reSpace.ReplaceAllString(minifiedHTML, " ")
	minifiedHTML = strings.ReplaceAll(minifiedHTML, "> <", "><")

	state["minified_html"] = minifiedHTML
	return state, nil
}

// GenerateScriptNode asks the LLM to generate a Go script using goquery.
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
	if n.Provider == nil {
		return state, fmt.Errorf("GenerateScriptNode: missing Provider")
	}

	if len(htmlContent) > 80000 {
		htmlContent = htmlContent[:80000] + "...[truncated]"
	}

	systemInstruction, err := prompts.Render("crawler_script_generator_sys", nil)
	if err != nil {
		return state, fmt.Errorf("GenerateScriptNode: failed to render prompt: %w", err)
	}

	fullPrompt, err := prompts.Render("crawler_script_generator_user", map[string]any{
		"TargetURL":   targetURL,
		"UserPrompt":  userPrompt,
		"HTMLContent": htmlContent,
	})
	if err != nil {
		return state, fmt.Errorf("GenerateScriptNode: failed to render prompt: %w", err)
	}

	script, err := n.Provider.GenerateContent(ctx, n.Provider.GetModelName(), systemInstruction+"\n\n"+fullPrompt)
	if err != nil {
		return state, fmt.Errorf("GenerateScriptNode: LLM failed: %w", err)
	}

	script = strings.TrimPrefix(script, "```go\n")
	script = strings.TrimPrefix(script, "```go")
	script = strings.TrimSuffix(script, "```\n")
	script = strings.TrimSuffix(script, "```")
	script = strings.TrimSpace(script)

	state["generated_script"] = script
	return state, nil
}
