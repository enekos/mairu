package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectIgnorerHonorsGitignore(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.log\nbuild/\n"), 0o644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "build"), 0o755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	ignoredFile := filepath.Join(root, "debug.log")
	if err := os.WriteFile(ignoredFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to write ignored file: %v", err)
	}
	ignoredDirFile := filepath.Join(root, "build", "main.go")
	if err := os.WriteFile(ignoredDirFile, []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed to write ignored dir file: %v", err)
	}
	visibleFile := filepath.Join(root, "README.md")
	if err := os.WriteFile(visibleFile, []byte("ok"), 0o644); err != nil {
		t.Fatalf("failed to write visible file: %v", err)
	}

	ignorer := NewProjectIgnorer(root)
	if !ignorer.IsIgnored(ignoredFile) {
		t.Fatalf("expected %s to be ignored", ignoredFile)
	}
	if !ignorer.IsIgnored(ignoredDirFile) {
		t.Fatalf("expected %s to be ignored", ignoredDirFile)
	}
	if ignorer.IsIgnored(visibleFile) {
		t.Fatalf("did not expect %s to be ignored", visibleFile)
	}
}
