package ingest

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type e2eRepo struct {
	mu    sync.Mutex
	calls []string
}

func (r *e2eRepo) InsertBashHistory(_ context.Context, _ string, command string, _ int, _ int, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, command)
	return nil
}

func (r *e2eRepo) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

// TestEndToEnd_SocketRedactPersistence is the load-bearing integration
// check for the full Step-3 pipeline: shell -> socket -> redact -> repo.
// It dials the server over a real Unix socket, sends three records
// (benign, secret-bearing, damage-cap-bait), and asserts the resulting
// repo state is exactly {benign, redacted-placeholder} with the
// damage-cap record silently dropped.
func TestEndToEnd_SocketRedactPersistence(t *testing.T) {
	// macOS caps Unix socket paths at ~104 bytes. t.TempDir() returns a
	// path under /var/folders/... that blows past that, so we hand-build
	// a short path in /tmp and clean it up ourselves.
	sockDir, err := os.MkdirTemp("/tmp", "mairu-e2e-")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(sockDir) })
	sockPath := filepath.Join(sockDir, "s")

	repo := &e2eRepo{}
	server := NewServer(sockPath, repo, mustRedactor())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErr := make(chan error, 1)
	go func() { runErr <- server.Run(ctx) }()

	// Wait for the listener to become ready — poll the socket file.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(sockPath); err == nil {
			break
		}
		select {
		case err := <-runErr:
			t.Fatalf("server.Run returned during startup: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not come up within 2s (sock=%s)", sockPath)
		}
		time.Sleep(10 * time.Millisecond)
	}

	conn, dialErr := net.Dial("unix", sockPath)
	if dialErr != nil {
		t.Fatalf("dial: %v", dialErr)
	}

	records := []Record{
		{Command: "echo hello", ExitCode: 0, DurationMs: 3, Project: "demo", Timestamp: time.Now().UTC()},
		{
			Command:    `curl -H "Authorization: Bearer abc123randomtoken" https://api.example.com/v1`,
			ExitCode:   0,
			DurationMs: 12,
			Project:    "demo",
			Timestamp:  time.Now().UTC(),
		},
		// Three Layer-1 hits → damage cap triggers → dropped.
		{
			Command:    `curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhYmMifQ.sig" https://user:hunter2supersecret@api.example.com/path?token=ghp_1234567890abcdefghijklmnopqrstuvwxyz`,
			ExitCode:   0,
			DurationMs: 8,
			Project:    "demo",
			Timestamp:  time.Now().UTC(),
		},
	}
	for _, rec := range records {
		if err := Encode(conn, rec); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	conn.Close()

	// Poll until two inserts land (third is dropped), with a 1s deadline.
	deadline = time.Now().Add(1 * time.Second)
	for {
		if got := repo.snapshot(); len(got) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("repo received %d inserts after 1s; want 2", len(repo.snapshot()))
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Give the server a short additional moment in case the dropped record
	// would have arrived — if it did, we'd see 3 calls.
	time.Sleep(100 * time.Millisecond)

	got := repo.snapshot()
	if len(got) != 2 {
		t.Fatalf("repo got %d inserts; want 2 (dropped one): %v", len(got), got)
	}
	if got[0] != "echo hello" {
		t.Errorf("call 0 = %q; want 'echo hello'", got[0])
	}
	if strings.Contains(got[1], "abc123randomtoken") {
		t.Errorf("bearer token leaked into storage: %q", got[1])
	}

	cancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Errorf("server.Run returned %v; want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server.Run did not shut down within 2s")
	}
}
