package acpbridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// buildMairu compiles cmd/mairu into a temp dir and returns the binary path.
// The binary is cached at the package level via sync.Once so multiple e2e
// tests reuse it.
var (
	mairuBin     string
	mairuBinErr  error
	mairuBinOnce sync.Once
)

func builtMairu(t *testing.T) string {
	t.Helper()
	mairuBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "mairu-e2e-*")
		if err != nil {
			mairuBinErr = err
			return
		}
		bin := filepath.Join(dir, "mairu")
		cmd := exec.Command("go", "build", "-o", bin, "../../cmd/mairu")
		if out, err := cmd.CombinedOutput(); err != nil {
			mairuBinErr = err
			t.Logf("build output: %s", out)
			return
		}
		mairuBin = bin
	})
	if mairuBinErr != nil {
		t.Fatalf("build mairu: %v", mairuBinErr)
	}
	return mairuBin
}

// wsURL turns httptest.Server.URL + a session id into a ws:// URL.
func wsURL(srv *httptest.Server, id string) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/acp?session=" + id
}

// readEnvelope reads one WS text frame and JSON-decodes it.
func readEnvelope(ctx context.Context, t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("parse %q: %v", data, err)
	}
	return env
}

// TestE2E_RealMairuACP_Initialize spawns a real `mairu acp` subprocess
// through the bridge and runs the JSON-RPC initialize handshake. Validates
// (1) the bridge passes frames in both directions, (2) the response is
// stamped with x-mairu-event-id, and (3) the JSON-RPC id round-trips.
func TestE2E_RealMairuACP_Initialize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in short mode")
	}
	bin := builtMairu(t)
	specs := map[string]AgentSpec{"mairu": {Command: bin, Args: []string{"acp"}}}
	b, err := New(Options{Addr: ":0", Agents: specs})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	id, err := b.registry.Create(ctx, "mairu", specs, 64, defaultSessionOpts(), 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer b.registry.Delete(id)

	c, _, err := websocket.Dial(ctx, wsURL(srv, id), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "bye")

	req := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{}}}`)
	if err := c.Write(ctx, websocket.MessageText, req); err != nil {
		t.Fatalf("write: %v", err)
	}

	env := readEnvelope(ctx, t, c)
	if env["x-mairu-event-id"] == nil {
		t.Fatalf("missing event id in %v", env)
	}
	if v, ok := env["id"].(float64); !ok || int(v) != 1 {
		t.Fatalf("not a response to id=1: %v", env)
	}
	res, ok := env["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %v", env)
	}
	if pv, ok := res["protocolVersion"].(float64); !ok || int(pv) != 1 {
		t.Fatalf("bad protocolVersion: %v", res)
	}
}

// TestE2E_HTTPCreateSessionAndAttach exercises the full operator flow:
// POST /sessions, then attach a WS to it (no shortcut via registry.Create).
func TestE2E_HTTPCreateSessionAndAttach(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in short mode")
	}
	bin := builtMairu(t)
	specs := map[string]AgentSpec{"mairu": {Command: bin, Args: []string{"acp"}}}
	b, err := New(Options{Addr: ":0", Agents: specs})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// POST /sessions
	body := strings.NewReader(`{"agent":"mairu"}`)
	req, _ := http.NewRequestWithContext(ctx, "POST", srv.URL+"/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("POST /sessions status %d", resp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if created.ID == "" {
		t.Fatal("no session id")
	}
	defer b.registry.Delete(created.ID)

	// GET /sessions returns it
	gr, _ := http.Get(srv.URL + "/sessions")
	var list []SessionInfo
	_ = json.NewDecoder(gr.Body).Decode(&list)
	gr.Body.Close()
	found := false
	for _, s := range list {
		if s.ID == created.ID && s.Active {
			found = true
		}
	}
	if !found {
		t.Fatalf("session not in list: %v", list)
	}

	// GET /sessions/:id returns single info
	gr, _ = http.Get(srv.URL + "/sessions/" + created.ID)
	if gr.StatusCode != 200 {
		t.Fatalf("GET /sessions/:id status %d", gr.StatusCode)
	}
	var one SessionInfo
	_ = json.NewDecoder(gr.Body).Decode(&one)
	gr.Body.Close()
	if one.ID != created.ID {
		t.Fatalf("got %v want %s", one, created.ID)
	}

	// Attach via WS and ping initialize.
	c, _, err := websocket.Dial(ctx, wsURL(srv, created.ID), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "bye")
	_ = c.Write(ctx, websocket.MessageText,
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{}}}`))
	env := readEnvelope(ctx, t, c)
	if env["x-mairu-event-id"] == nil {
		t.Fatalf("no event id: %v", env)
	}
}

// TestE2E_FullBridgeViaStart starts the bridge through its public Start
// API (bound to :0), exercises a session over the real bound port, then
// gracefully shuts down. Verifies the production lifecycle path.
func TestE2E_FullBridgeViaStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in short mode")
	}
	bin := builtMairu(t)
	specs := map[string]AgentSpec{"mairu": {Command: bin, Args: []string{"acp"}}}
	b, err := New(Options{Addr: "127.0.0.1:0", Agents: specs})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- b.Start(ctx) }()

	deadline := time.After(5 * time.Second)
	for b.ListenAddr() == "" {
		select {
		case <-deadline:
			t.Fatal("bridge never bound")
		case <-time.After(20 * time.Millisecond):
		}
	}

	base := "http://" + b.ListenAddr()
	resp, err := http.Post(base+"/sessions", "application/json", strings.NewReader(`{"agent":"mairu"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("POST /sessions = %d", resp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	ws, _, err := websocket.Dial(context.Background(),
		"ws://"+b.ListenAddr()+"/acp?session="+created.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	wctx, wcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer wcancel()
	_ = ws.Write(wctx, websocket.MessageText,
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{}}}`))
	env := readEnvelope(wctx, t, ws)
	if env["x-mairu-event-id"] == nil {
		t.Fatal("missing event id in real-bridge attach")
	}
	ws.Close(websocket.StatusNormalClosure, "bye")

	// Healthz works.
	hr, err := http.Get(base + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	if hr.StatusCode != 200 {
		t.Fatalf("healthz status %d", hr.StatusCode)
	}
	hr.Body.Close()

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Fatalf("Start err: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("bridge did not stop")
	}
}
