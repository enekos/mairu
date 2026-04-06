package crawler

import (
	"context"
	"mairu/internal/llm"
)

// SearchLinkGraph fetches a page and extracts relevant links based on a prompt
type SearchLinkGraph struct {
	graph *Graph
}

// NewSearchLinkGraph initializes a graph that fetches HTML and filters relevant links
func NewSearchLinkGraph(provider *llm.GeminiProvider) *SearchLinkGraph {
	return &SearchLinkGraph{
		graph: NewGraph(
			&FetchNode{},
			&SearchLinkNode{Provider: provider},
		),
	}
}

// Run executes the graph and returns the relevant URLs
func (s *SearchLinkGraph) Run(ctx context.Context, targetURL string, prompt string) ([]string, error) {
	initialState := State{
		"url":    targetURL,
		"prompt": prompt,
	}

	finalState, err := s.graph.Run(ctx, initialState)
	if err != nil {
		return nil, err
	}

	if links, ok := finalState["relevant_links"].([]string); ok {
		return links, nil
	}

	return nil, nil
}
