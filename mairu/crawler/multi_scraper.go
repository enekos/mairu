package crawler

import (
	"context"
	"fmt"
	"mairu/internal/llm"
	"sync"
)

// SmartScraperMultiGraph wraps a pipeline for extracting structured data from multiple URLs concurrently
type SmartScraperMultiGraph struct {
	provider    *llm.GeminiProvider
	concurrency int
}

// NewSmartScraperMultiGraph initializes a graph for multi-URL scraping
func NewSmartScraperMultiGraph(provider *llm.GeminiProvider, concurrency int) *SmartScraperMultiGraph {
	if concurrency <= 0 {
		concurrency = 3 // default concurrency
	}
	return &SmartScraperMultiGraph{
		provider:    provider,
		concurrency: concurrency,
	}
}

// Run executes the scraper across multiple URLs concurrently and returns a map of URL -> extracted JSON data
func (s *SmartScraperMultiGraph) Run(ctx context.Context, targetURLs []string, prompt string) (map[string]map[string]any, error) {
	results := make(map[string]map[string]any)
	var mu sync.Mutex

	sem := make(chan struct{}, s.concurrency)
	var wg sync.WaitGroup

	var firstErr error
	var errMu sync.Mutex

	for _, u := range targetURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Create a single scraper instance for this URL
			graph := NewSmartScraperGraph(s.provider)
			data, err := graph.Run(ctx, url, prompt)

			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed on url %s: %w", url, err)
				}
				errMu.Unlock()
				return
			}

			if data != nil {
				mu.Lock()
				results[url] = data
				mu.Unlock()
			}
		}(u)
	}

	wg.Wait()

	if firstErr != nil && len(results) == 0 {
		// Only return error if we got NOTHING. Partial results are better than none.
		return nil, firstErr
	}

	return results, nil
}
