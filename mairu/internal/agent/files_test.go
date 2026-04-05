package agent

import (
	"mairu/internal/db"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesTools(t *testing.T) {
	// Create a temporary directory to act as the project root
	tempDir, err := os.MkdirTemp("", "mairu_files_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	agent := &Agent{
		db: db.NewTestDB(tempDir),
	}

	t.Run("Write and Read File", func(t *testing.T) {
		content := "line 1\nline 2\nline 3"
		filePath := "test_dir/test_file.txt"

		_, err := agent.WriteFile(filePath, content)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		// Verify using standard os tools
		fullPath := filepath.Join(tempDir, filePath)
		savedContent, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("failed to read saved file directly: %v", err)
		}
		if string(savedContent) != content {
			t.Errorf("content mismatch. expected %q, got %q", content, string(savedContent))
		}

		// Verify using agent.ReadFile
		readContent, err := agent.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read file via agent: %v", err)
		}

		expectedReadContent := "1: line 1\n2: line 2\n3: line 3\n"
		if readContent != expectedReadContent {
			t.Errorf("agent read content mismatch. expected %q, got %q", expectedReadContent, readContent)
		}
	})

	t.Run("Find Files", func(t *testing.T) {
		// Create another file
		_, err := agent.WriteFile("test_dir/another_file.go", "package main")
		if err != nil {
			t.Fatal(err)
		}

		res, err := agent.FindFiles("**/*.txt")
		if err != nil {
			t.Fatalf("FindFiles error: %v", err)
		}
		if !strings.Contains(res, "test_dir/test_file.txt") {
			t.Errorf("expected FindFiles to find the txt file, got: %v", res)
		}

		res, err = agent.FindFiles("**/*.go")
		if err != nil {
			t.Fatalf("FindFiles error: %v", err)
		}
		if !strings.Contains(res, "test_dir/another_file.go") {
			t.Errorf("expected FindFiles to find the go file, got: %v", res)
		}

		res, err = agent.FindFiles("**/*.rs")
		if err != nil {
			t.Fatalf("FindFiles error: %v", err)
		}
		if res != "No files found matching pattern." {
			t.Errorf("expected 'No files found...', got: %v", res)
		}
	})

	t.Run("Search Codebase", func(t *testing.T) {
		// Ensure file content is on disk
		res, err := agent.SearchCodebase("line 2")
		if err != nil {
			t.Fatalf("SearchCodebase error: %v", err)
		}
		if !strings.Contains(res, "test_file.txt") || !strings.Contains(res, "line 2") {
			t.Errorf("expected SearchCodebase to find 'line 2', got: %v", res)
		}

		res, err = agent.SearchCodebase("nonexistenttext")
		if err == nil && !strings.Contains(res, "No matches found") && res != "" {
			t.Errorf("expected no matches message or error, got: %q", res)
		}
	})
}
