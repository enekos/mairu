package crawler

import (
	"context"
	"mairu/internal/llm"
)

// ScriptCreatorGraph fetches a page and generates a custom scraper script (Go/goquery) for it
type ScriptCreatorGraph struct {
	graph *Graph
}

// NewScriptCreatorGraph initializes a graph that fetches HTML and generates a scraper script
func NewScriptCreatorGraph(provider *llm.GeminiProvider) *ScriptCreatorGraph {
	return &ScriptCreatorGraph{
		graph: NewGraph(
			&FetchNode{},
			&MinifyHTMLNode{},
			&GenerateScriptNode{Provider: provider},
		),
	}
}

// Run executes the graph and returns the generated script content
func (s *ScriptCreatorGraph) Run(ctx context.Context, targetURL string, prompt string) (string, error) {
	initialState := State{
		"url":    targetURL,
		"prompt": prompt,
	}

	finalState, err := s.graph.Run(ctx, initialState)
	if err != nil {
		return "", err
	}

	if script, ok := finalState["generated_script"].(string); ok {
		return script, nil
	}

	return "", nil
}
