package crawler

import (
	"context"
	"strings"
	"testing"
)

func TestNodeNames(t *testing.T) {
	nodes := []Node{
		&FetchNode{},
		&ParseNode{},
		&ExtractNode{},
		&SearchNode{},
		&MergeAnswersNode{},
		&PromptRefinerNode{},
		&RAGExtractNode{},
		&MinifyHTMLNode{},
		&GenerateScriptNode{},
		&SearchLinkNode{},
	}

	for _, n := range nodes {
		if n.Name() == "" {
			t.Errorf("Node name should not be empty")
		}
	}
}

func TestNodesStateValidation(t *testing.T) {
	ctx := context.Background()
	emptyState := State{}

	// ExtractNode
	extract := &ExtractNode{}
	_, err := extract.Execute(ctx, emptyState)
	if err == nil || !strings.Contains(err.Error(), "missing 'doc'") {
		t.Errorf("ExtractNode missed doc validation: %v", err)
	}
	_, err = extract.Execute(ctx, State{"doc": "test"})
	if err == nil || !strings.Contains(err.Error(), "missing 'prompt'") {
		t.Errorf("ExtractNode missed prompt validation: %v", err)
	}

	// SearchNode
	search := &SearchNode{}
	_, err = search.Execute(ctx, emptyState)
	if err == nil || !strings.Contains(err.Error(), "missing 'search_query'") {
		t.Errorf("SearchNode missed search_query validation: %v", err)
	}

	// RAGExtractNode
	rag := &RAGExtractNode{}
	_, err = rag.Execute(ctx, emptyState)
	if err == nil || !strings.Contains(err.Error(), "missing 'doc'") {
		t.Errorf("RAGExtractNode missed doc validation: %v", err)
	}
	_, err = rag.Execute(ctx, State{"doc": "test"})
	if err == nil || !strings.Contains(err.Error(), "missing 'prompt'") {
		t.Errorf("RAGExtractNode missed prompt validation: %v", err)
	}

	// GenerateScriptNode
	script := &GenerateScriptNode{}
	_, err = script.Execute(ctx, emptyState)
	if err == nil || !strings.Contains(err.Error(), "missing 'minified_html'") {
		t.Errorf("GenerateScriptNode missed minified_html validation: %v", err)
	}
	_, err = script.Execute(ctx, State{"minified_html": "test"})
	if err == nil || !strings.Contains(err.Error(), "missing 'prompt'") {
		t.Errorf("GenerateScriptNode missed prompt validation: %v", err)
	}
	_, err = script.Execute(ctx, State{"minified_html": "test", "prompt": "test"})
	if err == nil || !strings.Contains(err.Error(), "missing GeminiProvider") {
		t.Errorf("GenerateScriptNode missed provider validation: %v", err)
	}

	// SearchLinkNode
	searchLink := &SearchLinkNode{}
	_, err = searchLink.Execute(ctx, emptyState)
	if err == nil || !strings.Contains(err.Error(), "missing 'html'") {
		t.Errorf("SearchLinkNode missed html validation: %v", err)
	}
	_, err = searchLink.Execute(ctx, State{"html": "test"})
	if err == nil || !strings.Contains(err.Error(), "missing 'prompt'") {
		t.Errorf("SearchLinkNode missed prompt validation: %v", err)
	}
	_, err = searchLink.Execute(ctx, State{"html": "test", "prompt": "test"})
	if err == nil || !strings.Contains(err.Error(), "missing GeminiProvider") {
		t.Errorf("SearchLinkNode missed provider validation: %v", err)
	}
}
