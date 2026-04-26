package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mairu/internal/agent"
	"mairu/internal/llm"
)

// TestIntegration_SessionLifecycle exercises a full initialize -> new -> prompt
// -> cancel flow through real stdio pipes.
func TestIntegration_SessionLifecycle(t *testing.T) {
	tmpDir := t.TempDir()

	build := func(cwd string) (*agent.Agent, error) {
		// Use a provider that immediately returns a stop chunk so the prompt
		// handler finishes quickly.
		p := newTestProvider()
		p.chatStreamResp = []llm.ChatStreamChunk{{FinishReason: "stop"}}
		return agent.NewWithProvider(tmpDir, p)
	}

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	defer stdinR.Close()
	defer stdoutR.Close()

	srv := New(llm.ProviderConfig{}, build)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.Run(ctx, stdinR, stdoutW)
	}()

	enc := json.NewEncoder(stdinW)
	dec := json.NewDecoder(stdoutR)

	// 1. initialize
	if err := enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{"protocolVersion": 1},
	}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var initResp rpcMessage
	if err := dec.Decode(&initResp); err != nil {
		t.Fatalf("decode init: %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("init error: %v", initResp.Error)
	}

	// 2. session/new
	if err := enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "session/new",
		"params":  map[string]any{"cwd": tmpDir},
	}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var newResp rpcMessage
	if err := dec.Decode(&newResp); err != nil {
		t.Fatalf("decode new: %v", err)
	}
	if newResp.Error != nil {
		t.Fatalf("new error: %v", newResp.Error)
	}
	var nsr newSessionResult
	if err := json.Unmarshal(newResp.Result, &nsr); err != nil {
		t.Fatalf("unmarshal new: %v", err)
	}
	if nsr.SessionID == "" {
		t.Fatal("expected session id")
	}

	// 3. session/prompt (fires a prompt that finishes immediately)
	promptReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "session/prompt",
		"params": map[string]any{
			"sessionId": nsr.SessionID,
			"prompt": []map[string]any{
				{"type": "text", "text": "hello"},
			},
		},
	}
	if err := enc.Encode(promptReq); err != nil {
		t.Fatalf("encode: %v", err)
	}

	// We should get a session/update notification with agent_message_chunk
	// and/or agent_thought_chunk, then finally the prompt result.
	var sawUpdate bool
	var sawResult bool
	for !sawResult {
		var msg rpcMessage
		if err := dec.Decode(&msg); err != nil {
			t.Fatalf("decode during prompt: %v", err)
		}
		if msg.Method == "session/update" {
			sawUpdate = true
			var up sessionUpdateParams
			json.Unmarshal(msg.Params, &up)
			if up.SessionID != nsr.SessionID {
				t.Errorf("update sessionId=%q, want %q", up.SessionID, nsr.SessionID)
			}
		} else if msg.ID != nil && string(msg.ID) == "3" {
			sawResult = true
			if msg.Error != nil {
				t.Fatalf("prompt error: %v", msg.Error)
			}
			var pr promptResult
			json.Unmarshal(msg.Result, &pr)
			if pr.StopReason != "end_turn" {
				t.Errorf("stopReason=%q, want end_turn", pr.StopReason)
			}
		}
	}
	if !sawUpdate {
		t.Error("expected at least one session/update notification")
	}

	// 4. session/cancel notification (no response expected)
	if err := enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"method":  "session/cancel",
		"params":  map[string]any{"sessionId": nsr.SessionID},
	}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	// Give it a moment; no response should appear.
	time.Sleep(100 * time.Millisecond)

	stdinW.Close()
	stdoutW.Close()

	if err := <-done; err != nil && !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Run returned error: %v", err)
	}
}

// TestIntegration_SessionLoad exercises session/load with a persisted session.
func TestIntegration_SessionLoad(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a saved session with some history.
	sessionPath := filepath.Join(tmpDir, ".mairu", "sessions", "test-session.json")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sessionData := `[
  {"role": "user", "parts": [{"type": "text", "text": "previous context"}]},
  {"role": "model", "parts": [{"type": "text", "text": "acknowledged"}]}
]`
	if err := os.WriteFile(sessionPath, []byte(sessionData), 0644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	build := func(cwd string) (*agent.Agent, error) {
		p := newTestProvider()
		p.chatStreamResp = []llm.ChatStreamChunk{{FinishReason: "stop"}}
		return agent.NewWithProvider(tmpDir, p)
	}

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	defer stdinR.Close()
	defer stdoutR.Close()

	srv := New(llm.ProviderConfig{}, build)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.Run(ctx, stdinR, stdoutW)
	}()

	enc := json.NewEncoder(stdinW)
	dec := json.NewDecoder(stdoutR)

	// initialize -> new -> load
	enc.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	var resp rpcMessage
	dec.Decode(&resp)

	enc.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "session/new", "params": map[string]any{"cwd": tmpDir}})
	dec.Decode(&resp)
	var nsr newSessionResult
	json.Unmarshal(resp.Result, &nsr)

	enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "session/load",
		"params": map[string]any{
			"sessionId":   nsr.SessionID,
			"sessionName": "test-session",
		},
	})
	dec.Decode(&resp)
	if resp.Error != nil {
		t.Fatalf("load error: %v", resp.Error)
	}
	var lsr loadSessionResult
	json.Unmarshal(resp.Result, &lsr)
	if !lsr.Success {
		t.Fatal("expected load success")
	}

	stdinW.Close()
	stdoutW.Close()
	<-done
}

// TestIntegration_OutboundRequestRoundTrip verifies that outbound requests
// (session/request_permission) are correctly correlated with responses.
func TestIntegration_OutboundRequestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{out: &buf, enc: json.NewEncoder(&buf)}

	go func() {
		time.Sleep(20 * time.Millisecond)
		s.pending.Range(func(key, value any) bool {
			ch := value.(chan rpcMessage)
			ch <- rpcMessage{
				JSONRPC: "2.0",
				ID:      json.RawMessage(key.(string)),
				Result:  mustMarshal(t, permissionResult{Outcome: map[string]any{"type": "selected", "optionId": "allow"}}),
			}
			return false
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	approved := s.requestPermission(ctx, "sess_1", agent.AgentEvent{Content: "test"}, "tc_1")
	if !approved {
		t.Error("expected approved")
	}
}

// TestIntegration_NDJSONFraming ensures each message is on its own line.
func TestIntegration_NDJSONFraming(t *testing.T) {
	var stdout bytes.Buffer
	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n")

	s := New(llm.ProviderConfig{}, func(cwd string) (*agent.Agent, error) {
		return nil, errors.New("no agent")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.Run(ctx, stdin, &stdout); err != nil {
		t.Fatalf("Run: %v", err)
	}

	scanner := bufio.NewScanner(&stdout)
	lineCount := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lineCount++
		var msg rpcMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Fatalf("line %d is not valid JSON: %v", lineCount, err)
		}
		if strings.Contains(line, "\n") {
			t.Fatalf("line %d contains embedded newline", lineCount)
		}
	}
	if lineCount != 1 {
		t.Fatalf("expected 1 response line, got %d", lineCount)
	}
}
