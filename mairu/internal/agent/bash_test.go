package agent

import (
	"mairu/internal/db"
	"strings"
	"testing"
)

func TestRunBash(t *testing.T) {
	agent := &Agent{
		db: db.NewTestDB("."),
	}

	t.Run("basic command", func(t *testing.T) {
		out, err := agent.RunBash("echo hello", 1000, nil)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(out, "hello") {
			t.Errorf("expected output to contain 'hello', got: %s", out)
		}
	})

	t.Run("timeout command", func(t *testing.T) {
		_, err := agent.RunBash("sleep 2", 100, nil) // 100ms timeout
		if err == nil {
			t.Fatal("expected an error due to timeout")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Errorf("expected timeout error message, got: %v", err)
		}
	})

	t.Run("failing command", func(t *testing.T) {
		out, err := agent.RunBash("ls /non/existent/path/123", 1000, nil)
		if err != nil {
			t.Fatalf("expected no error from RunBash itself for non-zero exit code, got: %v", err)
		}
		if !strings.Contains(out, "STDERR") || !strings.Contains(out, "No such file or directory") {
			t.Errorf("expected stderr in output, got: %s", out)
		}
	})
}
