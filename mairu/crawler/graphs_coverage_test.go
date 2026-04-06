package crawler

import (
	"context"
	"testing"
)

func TestGraphRunValidation(t *testing.T) {
	ctx := context.Background()

	// Multi scraper
	multi := NewSmartScraperMultiGraph(nil, 3)
	// Passing empty URLs should return empty map
	results, err := multi.Run(ctx, []string{}, "prompt")
	if err != nil {
		t.Errorf("Empty multi run should not fail: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}

	// Omni scraper
	omni := NewOmniScraperGraph(nil, 3)
	omniRes, err := omni.Run(ctx, []string{}, "prompt")
	if err != nil {
		t.Errorf("Empty omni run should not fail: %v", err)
	}
	if len(omniRes) != 0 {
		t.Errorf("Expected 0 omni results, got %d", len(omniRes))
	}

	// Depth search
	depth := NewDepthSearchScraperGraph(nil, 1, 3)
	// It will fail because FetchNode will fail with invalid URL
	_, err = depth.Run(ctx, "invalid-url", "prompt")
	if err == nil {
		t.Errorf("Depth search should fail with invalid URL")
	}
}

func TestSearchScraperGraphValidation(t *testing.T) {
	graph := NewSearchScraperGraph(nil)
	if graph == nil {
		t.Fatalf("SearchScraperGraph init failed")
	}
	
	// Try a run which will fail internally
	_, err := graph.Run(context.Background(), "", "prompt", 0)
	if err == nil {
		t.Errorf("Expected error on empty query")
	}
}

func TestScriptCreatorGraphValidation(t *testing.T) {
	graph := NewScriptCreatorGraph(nil)
	if graph == nil {
		t.Fatalf("ScriptCreatorGraph init failed")
	}
	
	// Should fail fetch
	_, err := graph.Run(context.Background(), "invalid", "prompt")
	if err == nil {
		t.Errorf("Expected error on invalid URL")
	}
}

func TestSearchLinkGraphValidation(t *testing.T) {
	graph := NewSearchLinkGraph(nil)
	if graph == nil {
		t.Fatalf("SearchLinkGraph init failed")
	}
	
	// Should fail fetch
	_, err := graph.Run(context.Background(), "invalid", "prompt")
	if err == nil {
		t.Errorf("Expected error on invalid URL")
	}
}
