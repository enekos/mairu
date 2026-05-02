package acpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestWSAttachAndEcho(t *testing.T) {
	bin := buildFixture(t)
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{"echo": {Command: bin}}})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, err := b.registry.Create(ctx, "echo", b.opts.Agents, 16)
	if err != nil {
		t.Fatal(err)
	}

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/acp?session=" + id
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "bye")

	if err := c.Write(ctx, websocket.MessageText, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := env["x-mairu-event-id"]; !ok {
		t.Fatal("missing event id")
	}
	if _, ok := env["echo"]; !ok {
		t.Fatalf("frame did not echo: %s", data)
	}
}

func TestWSAttachUnknownSession(t *testing.T) {
	b, _ := New(Options{Addr: ":0"})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/acp?session=nope"
	_, _, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected dial failure on unknown session")
	}
}

// Task 9: Last-Event-ID replay
func TestWSLastEventIDReplay(t *testing.T) {
	bin := buildFixture(t)
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{"echo": {Command: bin}}, RingBufferSize: 16})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	id, _ := b.registry.Create(ctx, "echo", b.opts.Agents, 16)

	// Round 1: send 3 frames, receive them.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/acp?session=" + id
	c1, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_ = c1.Write(ctx, websocket.MessageText, []byte(`{"jsonrpc":"2.0","method":"ping"}`))
	}
	for i := 0; i < 3; i++ {
		_, _, _ = c1.Read(ctx)
	}
	_ = c1.Close(websocket.StatusNormalClosure, "")

	// Round 2: reconnect with Last-Event-ID=1 → expect frames with id=2,3.
	hdr := http.Header{}
	hdr.Set("Last-Event-ID", "1")
	c2, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close(websocket.StatusNormalClosure, "")
	got := []uint64{}
	for i := 0; i < 2; i++ {
		_, data, err := c2.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var env map[string]any
		_ = json.Unmarshal(data, &env)
		got = append(got, uint64(env["x-mairu-event-id"].(float64)))
	}
	if got[0] != 2 || got[1] != 3 {
		t.Fatalf("replayed ids = %v, want [2 3]", got)
	}
}

// Task 10: auth gate rejection
func TestWSRejectsUnauthorizedPeer(t *testing.T) {
	bin := buildFixture(t)
	deny := TailscaleAuth{WhoIs: func(string) (string, error) { return "", errors.New("nope") }}
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{"echo": {Command: bin}}, Authorizer: deny})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	id, _ := b.registry.Create(ctx, "echo", b.opts.Agents, 16)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/acp?session=" + id
	_, _, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected rejection")
	}
}
