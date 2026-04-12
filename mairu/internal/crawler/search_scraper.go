package crawler

import (
	"context"
	"fmt"
	"mairu/internal/llm"
)

// SearchScraperGraph wraps a pipeline that searches DDG, fetches the top results, and extracts data
type SearchScraperGraph struct {
	provider llm.Provider
}

// NewSearchScraperGraph initializes a graph for search-based scraping
func NewSearchScraperGraph(provider llm.Provider) *SearchScraperGraph {
	return &SearchScraperGraph{
		provider: provider,
	}
}

// Run executes the search scraper and returns the extracted JSON data from the best matching page
func (s *SearchScraperGraph) Run(ctx context.Context, query string, prompt string, maxResults int) ([]map[string]any, error) {
	if maxResults <= 0 {
		maxResults = 3
	}

	// 1. Run the search node
	searchState := State{"search_query": query}
	searchNode := &SearchNode{MaxResults: maxResults}
	searchState, err := searchNode.Execute(ctx, searchState)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	urls, ok := searchState["search_results"].([]string)
	if !ok || len(urls) == 0 {
		return nil, fmt.Errorf("no search results found")
	}

	var results []map[string]any

	// 2. Iterate through results and run SmartScraper on each
	for _, url := range urls {
		smartGraph := NewSmartScraperGraph(s.provider)
		data, err := smartGraph.Run(ctx, url, prompt)
		if err == nil && len(data) > 0 {
			// Attach source URL
			data["_source_url"] = url
			results = append(results, data)
		}
	}

	return results, nil
}
