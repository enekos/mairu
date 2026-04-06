package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchNode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>
			<a class="result__url" href="//duckduckgo.com/l/?uddg=https://example.com/1">Example 1</a>
			<a class="result__url" href="https://example.com/2">Example 2</a>
		</body></html>`))
	}))
	defer ts.Close()

	node := &SearchNode{MaxResults: 2, SearchURL: ts.URL}
	state := State{"search_query": "test query"}

	newState, err := node.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("SearchNode failed: %v", err)
	}

	results, ok := newState["search_results"].([]string)
	if !ok {
		t.Fatalf("SearchNode did not return string array")
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if results[0] != "https://example.com/1" {
		t.Errorf("Expected https://example.com/1, got %s", results[0])
	}

	if results[1] != "https://example.com/2" {
		t.Errorf("Expected https://example.com/2, got %s", results[1])
	}
}
