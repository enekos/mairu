package crawler

import (
	"context"
	"fmt"
	"mairu/internal/llm"
)

// OmniScraperGraph is a pipeline that merges data from multiple pages into a single cohesive response.
// It fetches multiple URLs, extracts info using SmartScraper for each, and then uses a MergeAnswersNode.
type OmniScraperGraph struct {
	provider    *llm.GeminiProvider
	concurrency int
}

func NewOmniScraperGraph(provider *llm.GeminiProvider, concurrency int) *OmniScraperGraph {
	if concurrency <= 0 {
		concurrency = 3
	}
	return &OmniScraperGraph{
		provider:    provider,
		concurrency: concurrency,
	}
}

// Run executes the scraper across multiple URLs and merges the results into a single JSON response
func (s *OmniScraperGraph) Run(ctx context.Context, targetURLs []string, prompt string) (map[string]any, error) {
	// 1. Run multi-scraper
	multiGraph := NewSmartScraperMultiGraph(s.provider, s.concurrency)
	multiResults, err := multiGraph.Run(ctx, targetURLs, prompt)
	if err != nil {
		return nil, fmt.Errorf("multi-scrape failed: %w", err)
	}

	if len(multiResults) == 0 {
		return nil, nil
	}

	// 2. Prepare state for MergeAnswersNode
	state := State{
		"prompt":  prompt,
		"results": multiResults,
	}

	// 3. Run MergeAnswersNode
	mergeNode := &MergeAnswersNode{Provider: s.provider}
	finalState, err := mergeNode.Execute(ctx, state)
	if err != nil {
		return nil, err
	}

	if data, ok := finalState["merged_data"].(map[string]any); ok {
		return data, nil
	}

	return nil, nil
}
