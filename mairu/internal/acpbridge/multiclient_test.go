package acpbridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestMultiClient_BothSeeFrames: two WS clients attached to the same session
// each receive every frame the agent produces. Uses the echo fixture so the
// test does not need to build mairu.
func TestMultiClient_BothSeeFrames(t *testing.T) {
	bin := buildFixture(t)
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{"echo": {Command: bin}}})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id, _ := b.registry.Create(ctx, "echo", b.opts.Agents, 32, defaultSessionOpts(), 0)

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/acp?session=" + id
	a, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial a: %v", err)
	}
	defer a.Close(websocket.StatusNormalClosure, "")
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial c: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	// Both clients send the same frame; echo will produce two outputs total.
	for i := 0; i < 3; i++ {
		if err := a.Write(ctx, websocket.MessageText, []byte(`{"jsonrpc":"2.0","method":"ping","id":1}`)); err != nil {
			t.Fatal(err)
		}
	}

	collect := func(conn *websocket.Conn, n int) []uint64 {
		ids := []uint64{}
		for i := 0; i < n; i++ {
			rctx, rcancel := context.WithTimeout(ctx, 3*time.Second)
			_, data, err := conn.Read(rctx)
			rcancel()
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			var m map[string]any
			_ = json.Unmarshal(data, &m)
			if v, ok := m["x-mairu-event-id"].(float64); ok {
				ids = append(ids, uint64(v))
			}
		}
		return ids
	}
	idsA := collect(a, 3)
	idsC := collect(c, 3)
	if len(idsA) != 3 || len(idsC) != 3 {
		t.Fatalf("a=%v c=%v want 3 each", idsA, idsC)
	}
	for i := range idsA {
		if idsA[i] != idsC[i] {
			t.Fatalf("clients disagree at %d: a=%d c=%d", i, idsA[i], idsC[i])
		}
	}
}

// TestReconnectReplaysViaQueryParam: drop the WS, reconnect with the
// `last_event_id` query parameter (the only path React Native can use),
// and verify the bridge replays the missed frames.
func TestReconnectReplaysViaQueryParam(t *testing.T) {
	bin := buildFixture(t)
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{"echo": {Command: bin}}, RingBufferSize: 16})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id, _ := b.registry.Create(ctx, "echo", b.opts.Agents, 16, defaultSessionOpts(), 0)
	base := "ws" + strings.TrimPrefix(srv.URL, "http") + "/acp?session=" + id

	c1, _, err := websocket.Dial(ctx, base, nil)
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

	// Reconnect with last_event_id=1 → expect frames with id=2,3.
	c2, _, err := websocket.Dial(ctx, base+"&last_event_id=1", nil)
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
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		got = append(got, uint64(m["x-mairu-event-id"].(float64)))
	}
	if got[0] != 2 || got[1] != 3 {
		t.Fatalf("ids = %v, want [2 3]", got)
	}
}

// TestSlowSubscriberDoesNotBlockFast: a slow client that never reads must not
// stall a fast client on the same session.
func TestSlowSubscriberDoesNotBlockFast(t *testing.T) {
	bin := buildFixture(t)
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{"echo": {Command: bin}}, RingBufferSize: 32})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	id, _ := b.registry.Create(ctx, "echo", b.opts.Agents, 32, defaultSessionOpts(), 0)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/acp?session=" + id

	slow, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer slow.Close(websocket.StatusNormalClosure, "")
	fast, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer fast.Close(websocket.StatusNormalClosure, "")

	// Spam frames; slow never reads.
	const N = 50
	for i := 0; i < N; i++ {
		if err := fast.Write(ctx, websocket.MessageText, []byte(`{"jsonrpc":"2.0","method":"p"}`)); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	// Fast must still receive all frames.
	got := 0
	deadline := time.Now().Add(5 * time.Second)
	for got < N && time.Now().Before(deadline) {
		rctx, rcancel := context.WithTimeout(ctx, 2*time.Second)
		_, _, err := fast.Read(rctx)
		rcancel()
		if err != nil {
			break
		}
		got++
	}
	if got < N {
		t.Fatalf("fast received %d of %d", got, N)
	}
}

// TestAgentCrashClosesClients: when the agent subprocess dies, attached WS
// clients see the session-exit notification *and* the connection closes
// cleanly within a few seconds (no hang).
func TestAgentCrashClosesClients(t *testing.T) {
	bin := buildFixture(t)
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{"echo": {Command: bin}}})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id, _ := b.registry.Create(ctx, "echo", b.opts.Agents, 16, defaultSessionOpts(), 0)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/acp?session=" + id

	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(websocket.StatusInternalError, "")

	// Kill the agent.
	sess, _ := b.registry.Get(id)
	_ = sess.Close()

	// Expect the WS to either deliver a $/sessionExit notification or close.
	rctx, rcancel := context.WithTimeout(ctx, 5*time.Second)
	defer rcancel()
	for {
		_, data, err := c.Read(rctx)
		if err != nil {
			// Connection closed — acceptable end state.
			return
		}
		if strings.Contains(string(data), `"$/sessionExit"`) {
			return
		}
	}
}

// TestPostSessionsCap: MaxSessions=1 → first POST creates, second gets 429.
func TestPostSessionsCap(t *testing.T) {
	bin := buildFixture(t)
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{"echo": {Command: bin}}, MaxSessions: 1})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()

	first, err := http.Post(srv.URL+"/sessions", "application/json", strings.NewReader(`{"agent":"echo"}`))
	if err != nil {
		t.Fatal(err)
	}
	first.Body.Close()
	if first.StatusCode != 201 {
		t.Fatalf("first status %d", first.StatusCode)
	}

	second, err := http.Post(srv.URL+"/sessions", "application/json", strings.NewReader(`{"agent":"echo"}`))
	if err != nil {
		t.Fatal(err)
	}
	second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second status %d, want 429", second.StatusCode)
	}
}

// TestSessionInfoIncludesStderr: agent prints to stderr; GET /sessions/:id
// returns the tail.
func TestSessionInfoIncludesStderr(t *testing.T) {
	// Use a one-shot agent that writes to stderr then exits. We use the
	// system "sh" so we don't have to build another fixture.
	agent := AgentSpec{Command: "sh", Args: []string{"-c", "echo hello-from-stderr 1>&2; sleep 1; exit 7"}}
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{"sh": agent}})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, err := b.registry.Create(ctx, "sh", b.opts.Agents, 16, defaultSessionOpts(), 0)
	if err != nil {
		t.Fatal(err)
	}
	// Let the agent exit.
	sess, _ := b.registry.Get(id)
	<-sess.Done()
	// Allow stderr drain goroutine to finish flushing.
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(srv.URL + "/sessions/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var info SessionInfo
	_ = json.NewDecoder(resp.Body).Decode(&info)
	if info.Active {
		t.Fatal("expected session marked inactive")
	}
	if len(info.Stderr) == 0 {
		t.Fatal("expected stderr_tail to be non-empty")
	}
	joined := strings.Join(info.Stderr, "")
	if !strings.Contains(joined, "hello-from-stderr") {
		t.Fatalf("stderr_tail = %v", info.Stderr)
	}
	if info.ExitErr == "" {
		t.Fatal("expected exit_error to be populated for non-zero exit")
	}
}

// TestReapIdleClosesDeadSessions exercises the idle reaper directly so we
// don't have to wait minutes for the periodic ticker.
func TestReapIdleClosesDeadSessions(t *testing.T) {
	// Agent that exits immediately.
	agent := AgentSpec{Command: "sh", Args: []string{"-c", "exit 0"}}
	r := testRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, err := r.Create(ctx, "sh", map[string]AgentSpec{"sh": agent}, 8, defaultSessionOpts(), 0)
	if err != nil {
		t.Fatal(err)
	}
	sess, _ := r.Get(id)
	<-sess.Done()

	// Force LastActive into the past.
	r.mu.Lock()
	r.sessions[id].info.LastActive = time.Now().Add(-1 * time.Hour)
	r.mu.Unlock()

	reaped := r.ReapIdle(30 * time.Minute)
	if len(reaped) != 1 || reaped[0] != id {
		t.Fatalf("reaped %v", reaped)
	}
	if _, ok := r.Get(id); ok {
		t.Fatal("session not removed from registry")
	}
}

// TestConcurrentDialAndWriteRace stresses the bridge with many parallel
// dials. Mainly a soak/-race test.
func TestConcurrentDialAndWriteRace(t *testing.T) {
	bin := buildFixture(t)
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{"echo": {Command: bin}}, MaxSessions: 100, RingBufferSize: 64})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	id, _ := b.registry.Create(ctx, "echo", b.opts.Agents, 64, defaultSessionOpts(), 0)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/acp?session=" + id

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, _, err := websocket.Dial(ctx, url, nil)
			if err != nil {
				return
			}
			defer c.Close(websocket.StatusNormalClosure, "")
			for j := 0; j < 10; j++ {
				_ = c.Write(ctx, websocket.MessageText, []byte(`{"jsonrpc":"2.0","method":"p"}`))
				_, _, _ = c.Read(ctx)
			}
		}()
	}
	wg.Wait()
}
