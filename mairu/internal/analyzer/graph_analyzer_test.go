package analyzer

import "testing"

func TestGetReverseDependencies(t *testing.T) {
	g := &LogicGraph{
		Symbols: map[string]string{
			"a": "contextfs://proj/a",
			"b": "contextfs://proj/b",
		},
		Edges: []LogicEdge{
			{From: "a", To: "b", SourceURI: "contextfs://proj/a"},
			{From: "a", To: "b", SourceURI: "contextfs://proj/a#duplicate"},
		},
	}

	reverse := g.GetReverseDependencies()
	got := reverse["b"]
	if len(got) != 2 {
		t.Fatalf("expected 2 reverse dependencies for b, got %d", len(got))
	}
}

func TestAnalyzeFlows(t *testing.T) {
	g := &LogicGraph{
		Symbols: map[string]string{
			"start": "uri:start",
			"mid":   "uri:mid",
			"end":   "uri:end",
		},
		Edges: []LogicEdge{
			{From: "start", To: "mid"},
			{From: "mid", To: "end"},
		},
	}

	flows := AnalyzeFlows(g)
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
	if flows[0].StartSymbol != "start" {
		t.Fatalf("expected start symbol to be start, got %s", flows[0].StartSymbol)
	}
	if len(flows[0].Trace) != 3 {
		t.Fatalf("expected trace length 3, got %d", len(flows[0].Trace))
	}
}

func TestAnalyzeClusters(t *testing.T) {
	g := &LogicGraph{
		Symbols: map[string]string{
			"a": "uri:a",
			"b": "uri:b",
			"c": "uri:c",
			"d": "uri:d",
		},
		Edges: []LogicEdge{
			{From: "a", To: "b"},
			{From: "c", To: "d"},
		},
	}

	clusters := AnalyzeClusters(g)
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}
	for _, c := range clusters {
		if len(c.Symbols) != 2 {
			t.Fatalf("expected each cluster to have 2 symbols, got %d", len(c.Symbols))
		}
	}
}
