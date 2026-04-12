package crawler

import (
	"context"
	"fmt"
	"net/url"

	"mairu/internal/llm"
)

// DepthSearchScraperGraph fetches a seed URL, extracts relevant links up to maxDepth,
// and then extracts data from all relevant discovered pages concurrently.
type DepthSearchScraperGraph struct {
	provider    llm.Provider
	maxDepth    int
	concurrency int
}

func NewDepthSearchScraperGraph(provider llm.Provider, maxDepth, concurrency int) *DepthSearchScraperGraph {
	if maxDepth < 0 {
		maxDepth = 0
	}
	if concurrency <= 0 {
		concurrency = 3
	}
	return &DepthSearchScraperGraph{
		provider:    provider,
		maxDepth:    maxDepth,
		concurrency: concurrency,
	}
}

// Run executes the depth search and returns combined results
func (s *DepthSearchScraperGraph) Run(ctx context.Context, seedURL string, prompt string) (map[string]map[string]any, error) {
	visited := make(map[string]bool)
	var queue []string
	queue = append(queue, seedURL)

	var relevantURLs []string

	// Breadth-first relevant link discovery
	for depth := 0; depth <= s.maxDepth; depth++ {
		var nextQueue []string

		for _, currentURL := range queue {
			if visited[currentURL] {
				continue
			}
			visited[currentURL] = true
			relevantURLs = append(relevantURLs, currentURL)

			// If not at max depth, find more links
			if depth < s.maxDepth {
				searchLinkGraph := NewSearchLinkGraph(s.provider)
				links, err := searchLinkGraph.Run(ctx, currentURL, prompt)
				if err == nil {
					// Normalize and enqueue
					base, err := url.Parse(currentURL)
					if err == nil {
						for _, l := range links {
							parsed, err := url.Parse(l)
							if err == nil {
								abs := base.ResolveReference(parsed)
								abs.Fragment = ""
								normalized := abs.String()
								if !visited[normalized] {
									nextQueue = append(nextQueue, normalized)
								}
							}
						}
					}
				}
			}
		}
		queue = nextQueue
		if len(queue) == 0 {
			break
		}
	}

	if len(relevantURLs) == 0 {
		return nil, fmt.Errorf("no relevant URLs found")
	}

	fmt.Printf("DepthSearch found %d relevant URLs. Scraping...\n", len(relevantURLs))

	// Scrape all found URLs concurrently
	multiGraph := NewSmartScraperMultiGraph(s.provider, s.concurrency)
	return multiGraph.Run(ctx, relevantURLs, prompt)
}
