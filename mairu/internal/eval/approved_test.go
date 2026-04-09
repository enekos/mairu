//go:build integration

package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mairu/internal/approved"
	"mairu/internal/contextsrv"
)

// Thresholds for the Feedback Loop pattern.
const (
	minMRR    = 0.8
	minRecall = 0.75
)

// approvedFixture is a raw Meilisearch document with an explicit type and payload.
type approvedFixture struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

// approvedCase is a single eval query with known expected result IDs.
type approvedCase struct {
	ID       string   `json:"id"`
	Domain   string   `json:"domain"`
	Query    string   `json:"query"`
	TopK     int      `json:"top_k"`
	Expected []string `json:"expected"`
}

// approvedDataset is the self-contained dataset format for approved-log eval tests.
type approvedDataset struct {
	Project  string            `json:"project"`
	Fixtures []approvedFixture `json:"fixtures"`
	Cases    []approvedCase    `json:"cases"`
}

// TestApprovedRetrieval is an integration test that:
//  1. Seeds known fixtures directly into Meilisearch.
//  2. Runs each case query and captures the full ranked result trace.
//  3. Compares the trace (ranking order, not scores) against testdata/retrieval.approved.md.
//  4. Asserts MRR >= 0.8 and Recall >= 0.75 as a Feedback Loop gate.
//
// Run with:
//
//	go test -tags=integration ./internal/eval/... -run TestApprovedRetrieval
//
// Regenerate the approved file:
//
//	UPDATE_APPROVED=1 go test -tags=integration ./internal/eval/... -run TestApprovedRetrieval
func TestApprovedRetrieval(t *testing.T) {
	host := strings.TrimSpace(os.Getenv("MEILI_URL"))
	if host == "" {
		host = "http://localhost:7700"
	}
	apiKey := strings.TrimSpace(os.Getenv("MEILI_API_KEY"))

	indexer := contextsrv.NewMeiliIndexer(host, apiKey, nil)

	if err := indexer.EnsureIndexes(); err != nil {
		t.Fatalf("EnsureIndexes failed: %v", err)
	}

	// Skip gracefully if Meilisearch is not reachable.
	if stats := indexer.ClusterStats(); stats == nil {
		t.Skip("Meilisearch not reachable — set MEILI_URL or start Meilisearch to run integration tests")
	}

	raw, err := os.ReadFile(filepath.Join("testdata", "retrieval.dataset.json"))
	if err != nil {
		t.Fatalf("reading dataset: %v", err)
	}
	var dataset approvedDataset
	if err := json.Unmarshal(raw, &dataset); err != nil {
		t.Fatalf("parsing dataset: %v", err)
	}

	// Seed fixtures directly into Meilisearch with known IDs.
	for _, f := range dataset.Fixtures {
		if err := indexer.Upsert(f.Type, copyMap(f.Payload)); err != nil {
			t.Fatalf("upserting fixture %v: %v", f.Payload["id"], err)
		}
	}
	t.Cleanup(func() {
		for _, f := range dataset.Fixtures {
			entityType := f.Type
			id, _ := f.Payload["id"].(string)
			if id == "" {
				id, _ = f.Payload["uri"].(string)
			}
			_ = indexer.Delete(entityType, id)
		}
	})

	// Wait until all fixtures are searchable (Meilisearch indexes asynchronously).
	waitForIndexed(t, indexer, dataset.Project, 30*time.Second)

	// Run each case and collect results.
	var lines []string
	lines = append(lines, "# Retrieval Evaluation", "")

	var evalCases []Case
	for _, c := range dataset.Cases {
		topK := c.TopK
		if topK <= 0 {
			topK = 5
		}
		store := domainToStore(c.Domain)
		res, err := indexer.Search(contextsrv.SearchOptions{
			Query:   c.Query,
			Project: dataset.Project,
			Store:   store,
			TopK:    topK,
		})
		if err != nil {
			t.Fatalf("case %s search failed: %v", c.ID, err)
		}

		gotItems := extractItems(res, c.Domain)

		// Build section for this case.
		lines = append(lines,
			fmt.Sprintf("## Case: %s", c.ID),
			fmt.Sprintf("Query: %q", c.Query),
			fmt.Sprintf("Domain: %s | TopK: %d", c.Domain, topK),
			"",
			"### Results",
		)

		expectedSet := toStringSet(c.Expected)
		var got []RetrievalResult
		for i, item := range gotItems {
			id, _ := item["id"].(string)
			if c.Domain == "context" {
				if uri, ok := item["uri"].(string); ok && uri != "" {
					id = uri
				}
			}
			marker := ""
			if expectedSet[id] {
				marker = " [EXPECTED]"
			}
			lines = append(lines, fmt.Sprintf("%d. %s%s", i+1, id, marker))
			score, _ := item["_score"].(float64)
			got = append(got, RetrievalResult{ID: id, Score: score})
		}

		mrr := MeanReciprocalRank(c.Expected, got)
		recall := RecallAtK(c.Expected, got, topK)
		lines = append(lines,
			"",
			fmt.Sprintf("MRR: %.3f | Recall@%d: %.3f", mrr, topK, recall),
			"",
		)

		evalCases = append(evalCases, Case{
			ID:       c.ID,
			Domain:   c.Domain,
			Query:    c.Query,
			Expected: c.Expected,
			Got:      got,
		})
	}

	// Compute aggregate metrics.
	metrics := EvaluateCases(evalCases, 5)
	lines = append(lines,
		"---",
		"## Summary",
		fmt.Sprintf("MRR: %.3f | Recall@5: %.3f | Precision@5: %.3f | NDCG@5: %.3f | MAP: %.3f",
			metrics.MRR, metrics.Recall, metrics.Precision, metrics.NDCG, metrics.MAP),
		"",
	)

	actual := strings.Join(lines, "\n")
	approvedFile := filepath.Join("testdata", "retrieval.approved.md")

	approved.AssertString(t, approvedFile, actual)

	// Feedback Loop: assert quality thresholds.
	if metrics.MRR < minMRR {
		t.Errorf("MRR %.3f is below minimum %.3f — retrieval quality degraded", metrics.MRR, minMRR)
	}
	if metrics.Recall < minRecall {
		t.Errorf("Recall@5 %.3f is below minimum %.3f — retrieval quality degraded", metrics.Recall, minRecall)
	}
}

// waitForIndexed polls Meilisearch until at least one fixture per domain is
// searchable, or the deadline is reached.
func waitForIndexed(t *testing.T, indexer *contextsrv.MeiliIndexer, project string, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	domains := []string{"memory", "skill", "context"}
	queries := map[string]string{
		"memory":  "authentication",
		"skill":   "auth",
		"context": "backend",
	}

	for _, domain := range domains {
		store := domainToStore(domain)
		query := queries[domain]
		for {
			res, err := indexer.Search(contextsrv.SearchOptions{
				Query:   query,
				Project: project,
				Store:   store,
				TopK:    1,
			})
			if err == nil && len(extractItems(res, domain)) > 0 {
				break
			}
			if err != nil {
				t.Logf("wait search error (%s): %v", domain, err)
			}
			select {
			case <-ctx.Done():
				t.Fatalf("timed out waiting for %s fixtures to be indexed in Meilisearch", domain)
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}

// extractItems returns the result slice for the given domain from a Search response.
func extractItems(res map[string]any, domain string) []map[string]any {
	var key string
	switch strings.ToLower(domain) {
	case "memory":
		key = "memories"
	case "skill":
		key = "skills"
	case "context":
		key = "contextNodes"
	default:
		key = "memories"
	}
	items, _ := res[key].([]map[string]any)
	return items
}

// domainToStore converts an eval domain name to a contextsrv store name.
func domainToStore(domain string) string {
	switch strings.ToLower(domain) {
	case "memory":
		return contextsrv.StoreMemory
	case "skill":
		return contextsrv.StoreSkill
	case "context":
		return contextsrv.StoreNode
	default:
		return contextsrv.StoreMemory
	}
}

// copyMap returns a shallow copy of a map so Upsert can mutate it freely.
func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// toStringSet converts a slice to a set for O(1) lookup.
func toStringSet(ids []string) map[string]bool {
	s := make(map[string]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}
