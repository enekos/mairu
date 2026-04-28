package acpbridge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// buildFixture compiles testdata/echo_acp/ into a temp file and returns its path.
func buildFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "echo_acp")
	cmd := exec.Command("go", "build", "-tags", "acpbridgefixture", "-o", bin, "./testdata/echo_acp")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	return bin
}

func TestSessionEchoRoundTrip(t *testing.T) {
	bin := buildFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := StartSession(ctx, "test-1", AgentSpec{Command: bin}, NewRing(16))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer sess.Close()

	sub := sess.Subscribe()
	defer sess.Unsubscribe(sub)

	if err := sess.Send([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case sf, ok := <-sub:
		if !ok {
			t.Fatal("subscription closed unexpectedly")
		}
		if sf.ID != 1 {
			t.Fatalf("event_id = %d, want 1", sf.ID)
		}
		if len(sf.Frame) == 0 {
			t.Fatal("empty frame")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no frame received")
	}
}
