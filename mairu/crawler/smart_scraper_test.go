package crawler

import (
	"testing"
)

func TestSmartScraperInitialization(t *testing.T) {
	// Provider can be nil for initialization test
	graph := NewSmartScraperGraph(nil)
	if graph == nil {
		t.Fatal("NewSmartScraperGraph returned nil")
	}
	if len(graph.graph.Nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(graph.graph.Nodes))
	}

	ragGraph := NewRAGSmartScraperGraph(nil, 1000, 3)
	if ragGraph == nil {
		t.Fatal("NewRAGSmartScraperGraph returned nil")
	}
	if len(ragGraph.graph.Nodes) != 3 {
		t.Errorf("Expected 3 nodes for RAG, got %d", len(ragGraph.graph.Nodes))
	}

	refinedGraph := NewRefinedSmartScraperGraph(nil)
	if refinedGraph == nil {
		t.Fatal("NewRefinedSmartScraperGraph returned nil")
	}
	if len(refinedGraph.graph.Nodes) != 4 {
		t.Errorf("Expected 4 nodes for Refined, got %d", len(refinedGraph.graph.Nodes))
	}
}

func TestSmartScraperMultiInitialization(t *testing.T) {
	graph := NewSmartScraperMultiGraph(nil, 5)
	if graph == nil {
		t.Fatal("NewSmartScraperMultiGraph returned nil")
	}
	if graph.concurrency != 5 {
		t.Errorf("Expected concurrency 5, got %d", graph.concurrency)
	}

	graphDefault := NewSmartScraperMultiGraph(nil, 0)
	if graphDefault.concurrency != 3 {
		t.Errorf("Expected default concurrency 3, got %d", graphDefault.concurrency)
	}
}

func TestOmniScraperInitialization(t *testing.T) {
	graph := NewOmniScraperGraph(nil, 5)
	if graph == nil {
		t.Fatal("NewOmniScraperGraph returned nil")
	}
	if graph.concurrency != 5 {
		t.Errorf("Expected concurrency 5, got %d", graph.concurrency)
	}
}

func TestDepthSearchScraperInitialization(t *testing.T) {
	graph := NewDepthSearchScraperGraph(nil, 2, 5)
	if graph == nil {
		t.Fatal("NewDepthSearchScraperGraph returned nil")
	}
	if graph.maxDepth != 2 {
		t.Errorf("Expected maxDepth 2, got %d", graph.maxDepth)
	}
	if graph.concurrency != 5 {
		t.Errorf("Expected concurrency 5, got %d", graph.concurrency)
	}
}
