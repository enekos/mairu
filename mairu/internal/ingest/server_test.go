package ingest

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// nilRepo satisfies BashRepo without doing anything.
type nilRepo struct{}

func (nilRepo) InsertBashHistory(_ context.Context, _, _ string, _, _ int, _ string) error {
	return nil
}

// captureRepo records InsertBashHistory calls for test verification.
type captureRepo struct {
	mu    sync.Mutex
	calls []captureCall
}

type captureCall struct {
	project string
	command string
}

func (c *captureRepo) InsertBashHistory(_ context.Context, project, command string, _, _ int, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, captureCall{project: project, command: command})
	return nil
}

// newTestServer creates a Server wired to a temp socket path with a nil repo.
func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s")
	srv := NewServer(path, nilRepo{}, nil)
	return srv, path
}

// startServer runs srv.Run in a goroutine and returns a cancel func and the
// error channel so callers can wait for Run to return.
func startServer(ctx context.Context, srv *Server) (cancel context.CancelFunc, errCh chan error) {
	ctx, cancel = context.WithCancel(ctx)
	errCh = make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()
	return cancel, errCh
}

// waitListening polls until the socket file exists (up to 500 ms).
func waitListening(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server never started listening at %s", path)
}

// TestServerRun_ReceivesRecords verifies that encoded records arrive and are persisted.
func TestServerRun_ReceivesRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s")
	repo := &captureRepo{}
	srv := NewServer(path, repo, mustRedactor())

	cancel, errCh := startServer(context.Background(), srv)
	defer cancel()

	waitListening(t, path)

	// Dial and send two records.
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	r1 := Record{Command: "echo hello", Project: "proj"}
	r2 := Record{Command: "ls -la", Project: "proj"}
	if err := Encode(conn, r1); err != nil {
		t.Fatalf("encode r1: %v", err)
	}
	if err := Encode(conn, r2); err != nil {
		t.Fatalf("encode r2: %v", err)
	}
	conn.Close()

	// Wait up to 500 ms for both records to arrive.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		repo.mu.Lock()
		n := len(repo.calls)
		repo.mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.calls) != 2 {
		t.Fatalf("expected 2 records, got %d", len(repo.calls))
	}
	if repo.calls[0].command != r1.Command {
		t.Errorf("record[0].Command = %q, want %q", repo.calls[0].command, r1.Command)
	}
	if repo.calls[1].command != r2.Command {
		t.Errorf("record[1].Command = %q, want %q", repo.calls[1].command, r2.Command)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("Run returned error: %v", err)
	}
}

// TestServerRun_CleansUpSocketFile asserts that the socket file is removed after shutdown.
func TestServerRun_CleansUpSocketFile(t *testing.T) {
	srv, path := newTestServer(t)

	cancel, errCh := startServer(context.Background(), srv)

	waitListening(t, path)
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("socket file still exists after shutdown")
	}
}

// TestServerRun_ReturnsCleanlyOnCancel asserts that Run returns nil on context cancellation.
func TestServerRun_ReturnsCleanlyOnCancel(t *testing.T) {
	srv, path := newTestServer(t)

	cancel, errCh := startServer(context.Background(), srv)

	waitListening(t, path)
	cancel()

	if err := <-errCh; err != nil {
		t.Errorf("Run returned non-nil error on cancel: %v", err)
	}
}

// TestServerRun_HandlesMalformedPayload verifies a bad payload doesn't crash the server
// and subsequent valid records from a new connection are still processed.
func TestServerRun_HandlesMalformedPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s")
	repo := &captureRepo{}
	srv := NewServer(path, repo, mustRedactor())

	cancel, errCh := startServer(context.Background(), srv)
	defer cancel()

	waitListening(t, path)

	// Send malformed payload on first connection.
	bad, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial bad: %v", err)
	}
	bad.Write([]byte("not a record\n")) //nolint:errcheck
	bad.Close()

	// Give the server a moment to process the bad connection.
	time.Sleep(20 * time.Millisecond)

	// Send a good record on a second connection.
	good, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial good: %v", err)
	}
	rec := Record{Command: "good command", Project: "p"}
	if err := Encode(good, rec); err != nil {
		t.Fatalf("encode: %v", err)
	}
	good.Close()

	// Wait for the good record.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		repo.mu.Lock()
		n := len(repo.calls)
		repo.mu.Unlock()
		if n >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 good record, got %d", len(repo.calls))
	}
	if repo.calls[0].command != "good command" {
		t.Errorf("got command %q, want %q", repo.calls[0].command, "good command")
	}
}

// TestServerRun_RemovesStaleSocketFile pre-creates a file at the socket path and
// verifies the server starts successfully (removing the stale file).
func TestServerRun_RemovesStaleSocketFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s")

	// Pre-create a stale file at the socket path.
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	srv := NewServer(path, nilRepo{}, nil)
	cancel, errCh := startServer(context.Background(), srv)

	// If Run errors immediately, we fail here.
	waitListening(t, path)

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("Run returned error after stale socket removal: %v", err)
	}
}
