package ingest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"mairu/internal/redact"
)

// fakeRepo records InsertBashHistory calls for inspection.
type fakeRepo struct {
	mu    sync.Mutex
	calls []insertCall
}

type insertCall struct {
	project string
	command string
	exit    int
	dur     int
	output  string
}

func (f *fakeRepo) InsertBashHistory(_ context.Context, project, command string, exit, dur int, output string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, insertCall{project, command, exit, dur, output})
	return nil
}

// fakeRepoErr always returns an error from InsertBashHistory.
type fakeRepoErr struct{}

func (fakeRepoErr) InsertBashHistory(_ context.Context, _, _ string, _, _ int, _ string) error {
	return errors.New("simulated insert error")
}

// newConsumerServer builds a Server for direct processRecord testing (no socket).
func newConsumerServer(repo BashRepo) *Server {
	return NewServer("", repo, redact.New())
}

// TestProcessRecord_StoresBenignCommand verifies that a benign command is stored as-is.
func TestProcessRecord_StoresBenignCommand(t *testing.T) {
	repo := &fakeRepo{}
	srv := newConsumerServer(repo)

	srv.processRecord(context.Background(), Record{
		Command:    "echo hi",
		ExitCode:   0,
		DurationMs: 5,
		Project:    "demo",
	})

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 repo call, got %d", len(repo.calls))
	}
	got := repo.calls[0]
	if got.project != "demo" {
		t.Errorf("project = %q, want %q", got.project, "demo")
	}
	if got.command != "echo hi" {
		t.Errorf("command = %q, want %q", got.command, "echo hi")
	}
	if got.exit != 0 {
		t.Errorf("exit = %d, want 0", got.exit)
	}
	if got.dur != 5 {
		t.Errorf("dur = %d, want 5", got.dur)
	}
}

// TestProcessRecord_RedactsSecretInCommand verifies secrets are scrubbed before storage.
func TestProcessRecord_RedactsSecretInCommand(t *testing.T) {
	repo := &fakeRepo{}
	srv := newConsumerServer(repo)

	srv.processRecord(context.Background(), Record{
		Command:    `curl -H "Authorization: Bearer abc123randomtoken" https://api.example.com/x`,
		ExitCode:   0,
		DurationMs: 10,
		Project:    "demo",
	})

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 repo call, got %d", len(repo.calls))
	}
	if strings.Contains(repo.calls[0].command, "abc123randomtoken") {
		t.Errorf("token leaked into storage: %q", repo.calls[0].command)
	}
}

// TestProcessRecord_SkipsDroppedRecord verifies that commands tripping the damage cap are dropped.
func TestProcessRecord_SkipsDroppedRecord(t *testing.T) {
	repo := &fakeRepo{}
	srv := newConsumerServer(repo)

	// Three Layer-1 hits: JWT, basic-auth secret, and GitHub token — damage cap fires.
	srv.processRecord(context.Background(), Record{
		Command:    `curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhYmMifQ.sig" https://user:hunter2supersecret@api.example.com/path?token=ghp_1234567890abcdefghijklmnopqrstuvwxyz`,
		ExitCode:   0,
		DurationMs: 10,
		Project:    "demo",
	})

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.calls) != 0 {
		t.Errorf("expected 0 repo calls for dropped record, got %d: %+v", len(repo.calls), repo.calls)
	}
}

// TestProcessRecord_InfersProjectFromCwd verifies that an empty Project field is resolved from Cwd.
func TestProcessRecord_InfersProjectFromCwd(t *testing.T) {
	tmp := t.TempDir()
	cwdDir := filepath.Join(tmp, "inferred")
	if err := os.MkdirAll(filepath.Join(cwdDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	repo := &fakeRepo{}
	srv := newConsumerServer(repo)

	srv.processRecord(context.Background(), Record{
		Command:    "ls",
		ExitCode:   0,
		DurationMs: 1,
		Cwd:        cwdDir,
		// Project intentionally empty
	})

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 repo call, got %d", len(repo.calls))
	}
	if repo.calls[0].project != "inferred" {
		t.Errorf("project = %q, want %q", repo.calls[0].project, "inferred")
	}
}

// TestProcessRecord_StoresRedactedOutput verifies that Output, when present,
// is redacted before persistence while keeping enough benign context that
// the damage cap does not trigger.
func TestProcessRecord_StoresRedactedOutput(t *testing.T) {
	repo := &fakeRepo{}
	srv := newConsumerServer(repo)

	out := `Deploying api-service to production cluster us-east-1
Running pre-flight checks against staging environment
Error: authentication failed talking to the registry service
Tried token: ghp_1234567890abcdefghijklmnopqrstuvwxyz
Retrying after 5 seconds with backoff policy exponential
Retry count: 3; giving up after max attempts reached
`
	srv.processRecord(context.Background(), Record{
		Command:    "make deploy",
		ExitCode:   1,
		DurationMs: 42,
		Project:    "demo",
		Output:     out,
	})

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 repo call, got %d", len(repo.calls))
	}
	got := repo.calls[0].output
	if strings.Contains(got, "ghp_1234567890") {
		t.Errorf("PAT leaked into persisted output: %q", got)
	}
	if !strings.Contains(got, "authentication failed") {
		t.Errorf("benign context was stripped; got %q", got)
	}
}

// TestProcessRecord_OutputDamageCapReplacesBody verifies that when output
// is >50% secrets the whole output is replaced with the damage-cap
// placeholder — the command is still stored, just not the hollow payload.
func TestProcessRecord_OutputDamageCapReplacesBody(t *testing.T) {
	repo := &fakeRepo{}
	srv := newConsumerServer(repo)

	// Short output where the PAT dominates — Layer 5 fires.
	srv.processRecord(context.Background(), Record{
		Command: "make deploy",
		Project: "demo",
		Output:  "tok=ghp_1234567890abcdefghijklmnopqrstuvwxyz\n",
	})

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 repo call, got %d", len(repo.calls))
	}
	if repo.calls[0].output != "[REDACTED:damage_cap]" {
		t.Errorf("output = %q; want damage-cap placeholder", repo.calls[0].output)
	}
	if repo.calls[0].command != "make deploy" {
		t.Errorf("command unexpectedly modified: %q", repo.calls[0].command)
	}
}

// TestProcessRecord_EmptyOutputPassesThrough ensures Records with no Output
// field are not incorrectly annotated.
func TestProcessRecord_EmptyOutputPassesThrough(t *testing.T) {
	repo := &fakeRepo{}
	srv := newConsumerServer(repo)

	srv.processRecord(context.Background(), Record{
		Command: "echo hi",
		Project: "demo",
	})

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 repo call, got %d", len(repo.calls))
	}
	if repo.calls[0].output != "" {
		t.Errorf("empty Output should stay empty; got %q", repo.calls[0].output)
	}
}

// TestProcessRecord_RepoErrorDoesNotPanic verifies that a repo error is logged but does not panic or block.
func TestProcessRecord_RepoErrorDoesNotPanic(t *testing.T) {
	srv := NewServer("", fakeRepoErr{}, redact.New())

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.processRecord(context.Background(), Record{
			Command:    "echo safe",
			ExitCode:   0,
			DurationMs: 1,
			Project:    "demo",
		})
	}()

	select {
	case <-done:
		// processRecord returned normally — no panic or block.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("processRecord blocked or panicked; did not return within 500ms")
	}
}
