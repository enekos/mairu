package enricher

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initGitRepo creates a git repo at dir with a tracked file and N commits.
func initGitRepo(t *testing.T, dir, filename string, commitCount int) string {
	t.Helper()
	filePath := filepath.Join(dir, filename)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")

	for i := 0; i < commitCount; i++ {
		content := []byte("line " + string(rune('A'+i)) + "\n")
		if err := os.WriteFile(filePath, content, 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", filename)
		run("commit", "-m", "commit "+string(rune('A'+i)))
	}
	return filePath
}

func TestChangeVelocityEnricher(t *testing.T) {
	dir := t.TempDir()
	filePath := initGitRepo(t, dir, "main.go", 5)

	e := &ChangeVelocityEnricher{LookbackDays: 180}
	fc := &FileContext{
		FilePath: filePath,
		RelPath:  "main.go",
		WatchDir: dir,
		Metadata: map[string]any{},
	}

	if err := e.Enrich(context.Background(), fc); err != nil {
		t.Fatalf("enrich failed: %v", err)
	}

	score, ok := fc.Metadata["enrichment_churn_score"].(float64)
	if !ok {
		t.Fatalf("expected enrichment_churn_score in metadata, got: %v", fc.Metadata)
	}
	if score <= 0 {
		t.Fatalf("expected positive churn score for 5-commit file, got %f", score)
	}

	label, ok := fc.Metadata["enrichment_churn_label"].(string)
	if !ok || label == "" {
		t.Fatalf("expected enrichment_churn_label, got: %v", fc.Metadata)
	}

	total, ok := fc.Metadata["enrichment_total_commits"].(int)
	if !ok || total != 5 {
		t.Fatalf("expected 5 total commits, got: %v", fc.Metadata["enrichment_total_commits"])
	}
}

func TestChangeVelocityNoGitRepo(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "orphan.go")
	os.WriteFile(filePath, []byte("package main"), 0o644)

	e := &ChangeVelocityEnricher{LookbackDays: 180}
	fc := &FileContext{
		FilePath: filePath,
		RelPath:  "orphan.go",
		WatchDir: dir,
		Metadata: map[string]any{},
	}

	// Should not error — just skip enrichment gracefully
	if err := e.Enrich(context.Background(), fc); err != nil {
		t.Fatalf("should not error on non-git directory: %v", err)
	}
	if _, ok := fc.Metadata["enrichment_churn_score"]; ok {
		t.Fatal("should not set churn score when git is unavailable")
	}
}
