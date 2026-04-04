package contextsrv

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMeiliReadContextEndToEnd(t *testing.T) {
	host := strings.TrimSpace(os.Getenv("MEILI_URL"))
	if host == "" {
		host = "http://localhost:7700"
	}
	apiKey := strings.TrimSpace(os.Getenv("MEILI_API_KEY"))

	indexer := NewMeiliIndexer(host, apiKey, nil)
	if _, err := indexer.client.GetStats(); err != nil {
		t.Skipf("skipping Meili integration test (Meilisearch not reachable at %s): %v", host, err)
	}
	if err := indexer.EnsureIndexes(); err != nil {
		t.Fatalf("failed to ensure indexes: %v", err)
	}

	project := fmt.Sprintf("meili-e2e-%d", time.Now().UnixNano())
	uri := fmt.Sprintf("contextfs://%s/root/auth", project)
	doc := map[string]any{
		"id":         uri,
		"uri":        uri,
		"project":    project,
		"name":       "Auth context node",
		"abstract":   "Authentication flow context",
		"content":    "Token rotation policy for service accounts",
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"importance": 8,
	}
	if err := indexer.Upsert("context_node", doc); err != nil {
		t.Fatalf("failed to upsert context node into Meili: %v", err)
	}
	t.Cleanup(func() {
		_ = indexer.Delete("context_node", uri)
	})

	deadline := time.Now().Add(30 * time.Second)
	for {
		out, err := indexer.Search(SearchOptions{
			Query:   "token rotation policy",
			Project: project,
			Store:   "context",
			TopK:    5,
		})
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		nodes, _ := out["contextNodes"].([]map[string]any)
		if len(nodes) > 0 {
			gotURI, _ := nodes[0]["uri"].(string)
			if gotURI != uri {
				t.Fatalf("expected top context node uri %q, got %q", uri, gotURI)
			}
			return
		}

		if time.Now().After(deadline) {
			// Add debug info
			stats := indexer.ClusterStats()
			t.Fatalf("context node was not readable from Meili within timeout. Cluster stats: %v", stats)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
