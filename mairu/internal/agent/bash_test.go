package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enekos/mairu/pii-redact/pkg/redact"
)

func TestRunBash(t *testing.T) {
	agent := &Agent{
		root: ".",
	}

	t.Run("basic command", func(t *testing.T) {
		out, err := agent.RunBash(context.Background(), "echo hello", 1000, nil)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(out, "hello") {
			t.Errorf("expected output to contain 'hello', got: %s", out)
		}
	})

	t.Run("timeout command", func(t *testing.T) {
		_, err := agent.RunBash(context.Background(), "sleep 2", 100, nil) // 100ms timeout
		if err == nil {
			t.Fatal("expected an error due to timeout")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Errorf("expected timeout error message, got: %v", err)
		}
	})

	t.Run("failing command", func(t *testing.T) {
		out, err := agent.RunBash(context.Background(), "ls /non/existent/path/123", 1000, nil)
		if err != nil {
			t.Fatalf("expected no error from RunBash itself for non-zero exit code, got: %v", err)
		}
		if !strings.Contains(out, "STDERR") || !strings.Contains(out, "No such file or directory") {
			t.Errorf("expected stderr in output, got: %s", out)
		}
	})

	t.Run("bash tool redacts output when redactor is set", func(t *testing.T) {
		rd, err := redact.New(redact.Options{})
		if err != nil {
			t.Fatalf("redact.New: %v", err)
		}
		a := &Agent{root: ".", currentDir: ".", redactor: rd, approvalChan: make(chan bool)}
		tool := &bashTool{}
		// Echo a realistic-looking GitHub PAT. The raw shell output
		// contains the secret; the value the model sees must not.
		const pat = "ghp_1234567890abcdefghijklmnopqrstuvwxyz"
		out := make(chan AgentEvent, 128)
		go func() {
			for range out {
			}
		}()
		result, err := tool.Execute(context.Background(), map[string]any{
			"command": "echo token=" + pat,
		}, a, out)
		close(out)
		if err != nil {
			t.Fatalf("tool.Execute: %v", err)
		}
		body, _ := result["output"].(string)
		if strings.Contains(body, pat) {
			t.Errorf("PAT leaked to model-visible output: %q", body)
		}
		if !strings.Contains(body, "REDACTED") {
			t.Errorf("expected [REDACTED:...] marker in output: %q", body)
		}
	})

	t.Run("retries once on hangup", func(t *testing.T) {
		flagPath := filepath.Join(t.TempDir(), "hup-flag")
		cmd := "if [ -f '" + flagPath + "' ]; then echo recovered; else touch '" + flagPath + "'; kill -HUP $$; fi"

		out, err := agent.RunBash(context.Background(), cmd, 2000, nil)
		if err != nil {
			t.Fatalf("expected no error after hangup retry, got: %v", err)
		}
		if !strings.Contains(out, "recovered") {
			t.Fatalf("expected retry to recover and print output, got: %s", out)
		}
	})
}
