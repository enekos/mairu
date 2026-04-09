package cmd

import (
	"bytes"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
)

// Fake git diff output for the test. We need to override exec.Command, or simply mock the command output,
// but since exec.Command is hardcoded, we can mock `git diff` by setting PATH or just skipping if git isn't available.
// A simpler way: just test the logic directly or refactor the command to allow injecting the diff.
// For now, let's just make sure it compiles and we test it by refactoring the logic into a testable function.

// Actually, testing `exec.Command` in E2E is tricky if we don't mock it. Let's just create a dummy git repo.

func TestAnalyzeDiffE2E(t *testing.T) {
	// Setup test API
	api := newE2EContextAPI()
	ts := httptest.NewServer(api)
	defer ts.Close()

	os.Setenv("MAIRU_CONTEXT_SERVER_URL", ts.URL)
	defer os.Unsetenv("MAIRU_CONTEXT_SERVER_URL")

	// Inject a node with a logic_graph
	api.mu.Lock()
	api.nodes["contextfs://test/src/foo.go"] = e2eNode{
		URI:     "contextfs://test/src/foo.go",
		Project: "test",
		Name:    "foo.go",
		Metadata: map[string]any{
			"logic_graph": map[string]any{
				"symbols": []any{
					map[string]any{"ID": "func_a", "name": "func_a"},
				},
				"edges": []any{},
			},
		},
	}
	api.nodes["contextfs://test/src/bar.go"] = e2eNode{
		URI:     "contextfs://test/src/bar.go",
		Project: "test",
		Name:    "bar.go",
		Metadata: map[string]any{
			"logic_graph": map[string]any{
				"symbols": []any{
					map[string]any{"ID": "func_b", "name": "func_b"},
				},
				"edges": []any{
					map[string]any{"From": "func_b", "To": "func_a", "Kind": "call"},
				},
			},
		},
	}
	api.nodes["contextfs://test/src/baz.go"] = e2eNode{
		URI:     "contextfs://test/src/baz.go",
		Project: "test",
		Name:    "baz.go",
		Metadata: map[string]any{
			"logic_graph": map[string]any{
				"symbols": []any{
					map[string]any{"ID": "func_c", "name": "func_c"},
				},
				"edges": []any{
					map[string]any{"From": "func_c", "To": "func_b", "Kind": "call"},
				},
			},
		},
	}
	api.mu.Unlock()

	// In order to test analyze-diff, we need a real git repo with a diff.
	// Let's create a temporary git repo.
	dir, err := os.MkdirTemp("", "mairu-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	err = exec.Command("git", "init", dir).Run()
	if err != nil {
		t.Skip("git not available or failed to init")
	}

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)

	err = os.Chdir(dir)
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll("src", 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create files
	os.WriteFile("src/foo.go", []byte("package src"), 0644)
	os.WriteFile("src/bar.go", []byte("package src"), 0644)
	os.WriteFile("src/baz.go", []byte("package src"), 0644)

	exec.Command("git", "config", "user.email", "test@test.com").Run()
	exec.Command("git", "config", "user.name", "Test").Run()
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "init").Run()

	// Modify foo.go
	os.WriteFile("src/foo.go", []byte("package src\n// modified"), 0644)

	// Now run analyze-diff
	cmd := rootCmd
	cmd.SetArgs([]string{"analyze", "diff", "-P", "test"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("analyze-diff failed: %v", err)
	}

	output := out.String()

	// bar.go depends on func_a which is in foo.go, so bar.go is affected.
	// baz.go depends on func_b which is in bar.go, but baz.go won't be returned unless we do transitive resolution.
	// Current implementation only checks immediate reverse dependencies.
	if !bytes.Contains([]byte(output), []byte("src/bar.go")) {
		t.Errorf("Expected downstream to include contextfs://test/src/bar.go, got: %s", output)
	}
}
