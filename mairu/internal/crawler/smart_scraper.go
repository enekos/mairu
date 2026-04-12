package crawler

import (
	"context"
	"mairu/internal/llm"
)

// SmartScraperGraph wraps a pipeline for extracting structured data from a single URL
type SmartScraperGraph struct {
	graph *Graph
}

// NewSmartScraperGraph initializes a graph that fetches, parses, and extracts data
func NewSmartScraperGraph(provider llm.Provider) *SmartScraperGraph {
	return &SmartScraperGraph{
		graph: NewGraph(
			&FetchNode{},
			&ParseNode{},
			&ExtractNode{Provider: provider},
		),
	}
}

// NewRAGSmartScraperGraph initializes a graph that uses RAG for very large documents
func NewRAGSmartScraperGraph(provider llm.Provider, embedder llm.Embedder, chunkSize, topK int) *SmartScraperGraph {
	return &SmartScraperGraph{
		graph: NewGraph(
			&FetchNode{},
			&ParseNode{},
			&RAGExtractNode{
				Provider:  provider,
				Embedder:  embedder,
				ChunkSize: chunkSize,
				TopK:      topK,
			},
		),
	}
}

// NewRefinedSmartScraperGraph initializes a graph that fetches, parses, refines the prompt, and extracts data
func NewRefinedSmartScraperGraph(provider llm.Provider) *SmartScraperGraph {
	return &SmartScraperGraph{
		graph: NewGraph(
			&FetchNode{},
			&ParseNode{},
			&PromptRefinerNode{Provider: provider},
			&ExtractNode{Provider: provider},
		),
	}
}

// Run executes the scraper and returns the extracted JSON data
func (s *SmartScraperGraph) Run(ctx context.Context, targetURL string, prompt string) (map[string]any, error) {
	initialState := State{
		"url":    targetURL,
		"prompt": prompt,
	}

	finalState, err := s.graph.Run(ctx, initialState)
	if err != nil {
		return nil, err
	}

	if data, ok := finalState["extracted_data"].(map[string]any); ok {
		return data, nil
	}

	return nil, nil
}
