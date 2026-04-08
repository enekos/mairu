package cmd

import (
	"bytes"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAnalyzeGraphAndImpactE2E(t *testing.T) {
	// Setup test API
	api := newE2EContextAPI()
	ts := httptest.NewServer(api)
	defer ts.Close()

	os.Setenv("MAIRU_CONTEXT_SERVER_URL", ts.URL)
	defer os.Unsetenv("MAIRU_CONTEXT_SERVER_URL")

	// Inject a node with a logic_graph
	api.mu.Lock()
	api.nodes["contextfs://test/flows/A"] = e2eNode{
		URI:     "contextfs://test/flows/A",
		Project: "test",
		Name:    "Node A",
		Metadata: map[string]any{
			"logic_graph": map[string]any{
				"symbols": []any{
					map[string]any{"ID": "func_a", "name": "func_a"},
					map[string]any{"ID": "func_b", "name": "func_b"},
				},
				"edges": []any{
					map[string]any{"From": "func_a", "To": "func_b", "Kind": "call"},
				},
			},
		},
	}
	api.nodes["contextfs://test/flows/B"] = e2eNode{
		URI:     "contextfs://test/flows/B",
		Project: "test",
		Name:    "Node B",
		Metadata: map[string]any{
			"logic_graph": map[string]any{
				"symbols": []any{
					map[string]any{"ID": "func_b", "name": "func_b"},
					map[string]any{"ID": "func_c", "name": "func_c"},
				},
				"edges": []any{
					map[string]any{"From": "func_b", "To": "func_c", "Kind": "call"},
				},
			},
		},
	}
	api.mu.Unlock()

	// Test analyze-graph
	cmd := rootCmd
	cmd.SetArgs([]string{"analyze", "graph", "-P", "test", "--save"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("analyze-graph failed: %v", err)
	}

	// Verify new nodes were created (skills and flows)
	api.mu.Lock()
	nodesCount := len(api.nodes)
	api.mu.Unlock()

	if nodesCount <= 2 {
		t.Errorf("Expected new nodes to be created, got total nodes: %d", nodesCount)
	}

	// Test impact
	cmd.SetArgs([]string{"impact", "func_c", "-P", "test"})
	var outImpact bytes.Buffer
	cmd.SetOut(&outImpact)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("impact failed: %v", err)
	}

	output := outImpact.String()
	if !bytes.Contains(outImpact.Bytes(), []byte("func_c")) {
		t.Errorf("Impact output doesn't contain target: %s", output)
	}
}
