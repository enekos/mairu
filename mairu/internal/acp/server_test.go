package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"mairu/internal/agent"
	"mairu/internal/llm"
)

// --- minimal llm.Provider mock ------------------------------------------------

type testIterator struct {
	chunks []llm.ChatStreamChunk
	idx    int
	done   bool
}

func (t *testIterator) Next() (llm.ChatStreamChunk, error) {
	if t.idx >= len(t.chunks) {
		return llm.ChatStreamChunk{}, errors.New("EOF")
	}
	c := t.chunks[t.idx]
	t.idx++
	if c.FinishReason == "stop" || c.FinishReason == "length" {
		t.done = true
	}
	return c, nil
}

func (t *testIterator) Done() bool { return t.done }

type testProvider struct {
	mu              sync.Mutex
	systemPrompt    string
	model           string
	history         []llm.Message
	tools           []llm.Tool
	chatStreamResp  []llm.ChatStreamChunk
	chatStreamErr   error
	generateJSONErr error
}

func newTestProvider() *testProvider {
	return &testProvider{
		model:   "test-model",
		history: []llm.Message{},
	}
}

func (p *testProvider) Chat(ctx context.Context, prompt string) (*llm.ChatResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.history = append(p.history, llm.Message{Role: "user", Content: prompt})
	return &llm.ChatResponse{Content: "test response", FinishReason: "stop"}, nil
}

func (p *testProvider) ChatStream(ctx context.Context, prompt string) (llm.ChatStreamIterator, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.chatStreamErr != nil {
		return nil, p.chatStreamErr
	}
	p.history = append(p.history, llm.Message{Role: "user", Content: prompt})
	return &testIterator{chunks: p.chatStreamResp}, nil
}

func (p *testProvider) SendFunctionResponseStream(ctx context.Context, name string, result map[string]any) llm.ChatStreamIterator {
	return &testIterator{chunks: []llm.ChatStreamChunk{{FinishReason: "stop"}}}
}

func (p *testProvider) SendFunctionResponsesStream(ctx context.Context, responses []llm.FunctionResponsePayload) llm.ChatStreamIterator {
	return &testIterator{chunks: []llm.ChatStreamChunk{{FinishReason: "stop"}}}
}

func (p *testProvider) GenerateJSON(ctx context.Context, system, user string, schema *llm.JSONSchema, out any) error {
	return p.generateJSONErr
}

func (p *testProvider) GenerateContent(ctx context.Context, model, prompt string) (string, error) {
	return "generated", nil
}

func (p *testProvider) SetSystemInstruction(prompt string) { p.systemPrompt = prompt }
func (p *testProvider) SetModel(modelName string)          { p.model = modelName }
func (p *testProvider) GetModelName() string               { return p.model }
func (p *testProvider) GetHistory() []llm.Message          { return p.history }
func (p *testProvider) SetHistory(history []llm.Message)   { p.history = history }
func (p *testProvider) IsNewSession() bool                 { return len(p.history) == 0 }
func (p *testProvider) GetTools() []llm.Tool               { return p.tools }
func (p *testProvider) SetTools(tools []llm.Tool)          { p.tools = tools }
func (p *testProvider) RegisterDynamicTools(tools []llm.Tool) {
	p.tools = append(p.tools, tools...)
}
func (p *testProvider) Close() error { return nil }

// --- test helpers -------------------------------------------------------------

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// newTestServer creates a server that writes to a bytes.Buffer.
// Safe for synchronous handler tests.
func newTestServer(t *testing.T) (*Server, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	s := New(llm.ProviderConfig{}, func(cwd string) (*agent.Agent, error) {
		return nil, errors.New("no agent in this test")
	})
	s.out = &buf
	s.enc = json.NewEncoder(&buf)
	return s, &buf
}

func mustReadOne(t *testing.T, buf *bytes.Buffer) rpcMessage {
	t.Helper()
	var msg rpcMessage
	if err := json.NewDecoder(buf).Decode(&msg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return msg
}

// --- handler tests ------------------------------------------------------------

func TestHandleInitialize(t *testing.T) {
	s, buf := newTestServer(t)

	req := rpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "initialize",
		Params:  mustMarshal(t, initializeParams{ProtocolVersion: 1}),
	}
	s.handleInitialize(req)

	msg := mustReadOne(t, buf)
	if msg.Error != nil {
		t.Fatalf("unexpected error: %v", msg.Error)
	}
	var res initializeResult
	if err := json.Unmarshal(msg.Result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.ProtocolVersion != protocolVersion {
		t.Errorf("protocolVersion=%d, want %d", res.ProtocolVersion, protocolVersion)
	}
	if res.AgentCapabilities.LoadSession {
		t.Error("expected LoadSession=false")
	}
}

func TestHandleUnknownMethod(t *testing.T) {
	s, buf := newTestServer(t)

	req := rpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"2"`),
		Method:  "foo/bar",
	}
	s.handle(context.Background(), req)

	msg := mustReadOne(t, buf)
	if msg.Error == nil {
		t.Fatal("expected error response")
	}
	if msg.Error.Code != errMethodNotFound {
		t.Errorf("code=%d, want %d", msg.Error.Code, errMethodNotFound)
	}
}

func TestHandleUnknownMethod_Notification(t *testing.T) {
	s, buf := newTestServer(t)

	// Notifications (no id) must not produce a response.
	req := rpcMessage{
		JSONRPC: "2.0",
		Method:  "foo/bar",
	}
	s.handle(context.Background(), req)

	if buf.Len() != 0 {
		t.Fatalf("expected no response for notification, got: %s", buf.String())
	}
}

func TestHandleNewSession(t *testing.T) {
	called := false
	build := func(cwd string) (*agent.Agent, error) {
		called = true
		return agent.NewWithProvider("/tmp", newTestProvider())
	}
	var buf bytes.Buffer
	s := New(llm.ProviderConfig{}, build)
	s.out = &buf
	s.enc = json.NewEncoder(&buf)

	req := rpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"3"`),
		Method:  "session/new",
		Params:  mustMarshal(t, newSessionParams{CWD: "/tmp"}),
	}
	s.handleNewSession(req)

	msg := mustReadOne(t, &buf)
	if msg.Error != nil {
		t.Fatalf("unexpected error: %v", msg.Error)
	}
	var res newSessionResult
	if err := json.Unmarshal(msg.Result, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.SessionID == "" {
		t.Error("expected non-empty sessionId")
	}
	if !called {
		t.Error("expected buildAgent to be called")
	}
}

func TestHandleLoadSession(t *testing.T) {
	tmpDir := t.TempDir()
	if err := agent.CreateEmptySession(tmpDir, "my-session"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	build := func(cwd string) (*agent.Agent, error) {
		return agent.NewWithProvider(tmpDir, newTestProvider())
	}
	var buf bytes.Buffer
	s := New(llm.ProviderConfig{}, build)
	s.out = &buf
	s.enc = json.NewEncoder(&buf)

	// Create a session first
	s.handleNewSession(rpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "session/new",
		Params:  mustMarshal(t, newSessionParams{CWD: tmpDir}),
	})
	msg := mustReadOne(t, &buf)
	var nsr newSessionResult
	json.Unmarshal(msg.Result, &nsr)

	// Load into it
	s.handleLoadSession(rpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"2"`),
		Method:  "session/load",
		Params: mustMarshal(t, loadSessionParams{
			SessionID:   nsr.SessionID,
			SessionName: "my-session",
		}),
	})
	msg = mustReadOne(t, &buf)
	if msg.Error != nil {
		t.Fatalf("unexpected error: %v", msg.Error)
	}
	var lsr loadSessionResult
	if err := json.Unmarshal(msg.Result, &lsr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !lsr.Success {
		t.Error("expected success=true")
	}
}

func TestHandleLoadSession_UnknownSession(t *testing.T) {
	s, buf := newTestServer(t)

	s.handleLoadSession(rpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "session/load",
		Params: mustMarshal(t, loadSessionParams{
			SessionID:   "nosuch",
			SessionName: "foo",
		}),
	})
	msg := mustReadOne(t, buf)
	if msg.Error == nil {
		t.Fatal("expected error")
	}
	if msg.Error.Code != errInvalidParams {
		t.Errorf("code=%d, want %d", msg.Error.Code, errInvalidParams)
	}
}

func TestHandleCancel(t *testing.T) {
	// Cancel is a notification; it must not produce a response.
	build := func(cwd string) (*agent.Agent, error) {
		return agent.NewWithProvider("/tmp", newTestProvider())
	}
	var buf bytes.Buffer
	s := New(llm.ProviderConfig{}, build)
	s.out = &buf
	s.enc = json.NewEncoder(&buf)

	// Create session
	s.handleNewSession(rpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "session/new",
		Params:  mustMarshal(t, newSessionParams{CWD: "/tmp"}),
	})
	mustReadOne(t, &buf)

	// Cancel it
	s.handleCancel(rpcMessage{
		JSONRPC: "2.0",
		Method:  "session/cancel",
		Params: mustMarshal(t, cancelParams{
			SessionID: s.currentID,
		}),
	})

	if buf.Len() != 0 {
		t.Fatalf("expected no response for cancel notification, got: %s", buf.String())
	}
}

// --- bridge tests -------------------------------------------------------------

func TestBridge_EventMapping(t *testing.T) {
	tests := []struct {
		name     string
		events   []agent.AgentEvent
		want     []map[string]any
		wantStop string
	}{
		{
			name: "text events become agent_message_chunk",
			events: []agent.AgentEvent{
				{Type: "text", Content: "hello"},
				{Type: "done"},
			},
			want: []map[string]any{
				{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "hello"}},
			},
			wantStop: "end_turn",
		},
		{
			name: "error becomes refusal",
			events: []agent.AgentEvent{
				{Type: "text", Content: "oops"},
				{Type: "error", Content: "something broke"},
			},
			want: []map[string]any{
				{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "oops"}},
				{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "Error: something broke"}},
			},
			wantStop: "refusal",
		},
		{
			name: "status/log/diff/bash_output become agent_thought_chunk",
			events: []agent.AgentEvent{
				{Type: "status", Content: "working"},
				{Type: "log", Content: "log line"},
				{Type: "diff", Content: "diff text"},
				{Type: "bash_output", Content: "output"},
				{Type: "done"},
			},
			want: []map[string]any{
				{"sessionUpdate": "agent_thought_chunk", "content": map[string]any{"type": "text", "text": "working"}},
				{"sessionUpdate": "agent_thought_chunk", "content": map[string]any{"type": "text", "text": "log line"}},
				{"sessionUpdate": "agent_thought_chunk", "content": map[string]any{"type": "text", "text": "diff text"}},
				{"sessionUpdate": "agent_thought_chunk", "content": map[string]any{"type": "text", "text": "output"}},
			},
			wantStop: "end_turn",
		},
		{
			name: "tool_call and tool_result pair via stack",
			events: []agent.AgentEvent{
				{Type: "tool_call", ToolName: "bash", ToolArgs: map[string]any{"command": "echo hi"}},
				{Type: "tool_result", ToolName: "bash", ToolResult: map[string]any{"output": "hi"}},
				{Type: "done"},
			},
			want: []map[string]any{
				{"sessionUpdate": "tool_call", "title": "bash: echo hi", "kind": "execute", "status": "in_progress"},
				{"sessionUpdate": "tool_call_update", "status": "completed", "rawOutput": map[string]any{"output": "hi"}},
			},
			wantStop: "end_turn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			s := &Server{out: &buf, enc: json.NewEncoder(&buf)}

			events := make(chan agent.AgentEvent, len(tt.events))
			for _, ev := range tt.events {
				events <- ev
			}
			close(events)

			stop := s.bridge(context.Background(), "sess_1", nil, events)
			if stop != tt.wantStop {
				t.Errorf("stop=%q, want %q", stop, tt.wantStop)
			}

			var got []rpcMessage
			dec := json.NewDecoder(&buf)
			for {
				var msg rpcMessage
				if err := dec.Decode(&msg); err != nil {
					break
				}
				got = append(got, msg)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d updates, want %d", len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if got[i].Method != "session/update" {
					t.Errorf("msg[%d].method=%q, want session/update", i, got[i].Method)
					continue
				}
				var params sessionUpdateParams
				if err := json.Unmarshal(got[i].Params, &params); err != nil {
					t.Fatalf("unmarshal params[%d]: %v", i, err)
				}
				for k, v := range want {
					gotJSON, _ := json.Marshal(params.Update[k])
					wantJSON, _ := json.Marshal(v)
					if !bytes.Equal(gotJSON, wantJSON) {
						t.Errorf("update[%d][%q] = %s, want %s", i, k, gotJSON, wantJSON)
					}
				}
			}
		})
	}
}

func TestBridge_ContextCancel(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{out: &buf, enc: json.NewEncoder(&buf)}

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan agent.AgentEvent, 2)
	events <- agent.AgentEvent{Type: "text", Content: "hello"}
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
		events <- agent.AgentEvent{Type: "text", Content: "world"}
	}()

	stop := s.bridge(ctx, "sess_1", nil, events)
	if stop != "cancelled" {
		t.Errorf("stop=%q, want cancelled", stop)
	}
}

func TestRequestPermission_Allowed(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{out: &buf, enc: json.NewEncoder(&buf)}

	// Inject the client response asynchronously.
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

	approved := s.requestPermission(ctx, "sess_1", agent.AgentEvent{Content: "allow me"}, "tc_42")
	if !approved {
		t.Error("expected approved=true")
	}
}

func TestRequestPermission_Denied(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{out: &buf, enc: json.NewEncoder(&buf)}

	go func() {
		time.Sleep(20 * time.Millisecond)
		s.pending.Range(func(key, value any) bool {
			ch := value.(chan rpcMessage)
			ch <- rpcMessage{
				JSONRPC: "2.0",
				ID:      json.RawMessage(key.(string)),
				Result:  mustMarshal(t, permissionResult{Outcome: map[string]any{"type": "selected", "optionId": "deny"}}),
			}
			return false
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	approved := s.requestPermission(ctx, "sess_1", agent.AgentEvent{Content: "deny me"}, "")
	if approved {
		t.Error("expected approved=false")
	}
}

func TestRequestPermission_IncludesToolCallID(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{out: &buf, enc: json.NewEncoder(&buf)}

	var capturedID string
	go func() {
		time.Sleep(20 * time.Millisecond)
		s.pending.Range(func(key, value any) bool {
			capturedID = key.(string)
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

	s.requestPermission(ctx, "sess_1", agent.AgentEvent{Content: "test"}, "tc_99")
	if capturedID == "" {
		t.Fatal("expected outbound request to be registered")
	}

	// Verify the request was written to the buffer.
	var written rpcMessage
	if err := json.NewDecoder(&buf).Decode(&written); err != nil {
		t.Fatalf("decode written request: %v", err)
	}
	if written.Method != "session/request_permission" {
		t.Errorf("method=%q, want session/request_permission", written.Method)
	}
	var params permissionParams
	if err := json.Unmarshal(written.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params.SessionID != "sess_1" {
		t.Errorf("sessionID=%q, want sess_1", params.SessionID)
	}
	if params.ToolCall["toolCallId"] != "tc_99" {
		t.Errorf("toolCallId=%v, want tc_99", params.ToolCall["toolCallId"])
	}
	if params.ToolCall["title"] != "Action requires approval" {
		t.Errorf("title=%v, want 'Action requires approval'", params.ToolCall["title"])
	}
}

// --- Server.Run integration test ----------------------------------------------

func TestServerRun_InitializeRoundTrip(t *testing.T) {
	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}

	s := New(llm.ProviderConfig{}, func(cwd string) (*agent.Agent, error) {
		return nil, errors.New("no agent")
	})

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{"protocolVersion": 1},
	}
	enc := json.NewEncoder(stdin)
	enc.Encode(req)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run blocks until stdin is closed.
	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx, stdin, stdout)
	}()

	// Give Run a moment to process.
	time.Sleep(100 * time.Millisecond)

	var resp rpcMessage
	if err := json.NewDecoder(stdout).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID == nil {
		t.Fatal("expected id in response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	cancel()
	<-done
}

func TestServerRun_UnknownMethod(t *testing.T) {
	pr, pw := io.Pipe()
	var stdout bytes.Buffer

	s := New(llm.ProviderConfig{}, func(cwd string) (*agent.Agent, error) {
		return nil, errors.New("no agent")
	})

	go func() {
		enc := json.NewEncoder(pw)
		enc.Encode(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "nope"})
		pw.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.Run(ctx, pr, &stdout); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var resp rpcMessage
	if err := json.NewDecoder(&stdout).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != errMethodNotFound {
		t.Fatalf("expected -32601, got %+v", resp.Error)
	}
}

func TestServerRun_NotificationProducesNoResponse(t *testing.T) {
	pr, pw := io.Pipe()
	var stdout bytes.Buffer

	s := New(llm.ProviderConfig{}, func(cwd string) (*agent.Agent, error) {
		return nil, errors.New("no agent")
	})

	go func() {
		enc := json.NewEncoder(pw)
		enc.Encode(map[string]any{"jsonrpc": "2.0", "method": "session/cancel", "params": map[string]any{"sessionId": "x"}})
		pw.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.Run(ctx, pr, &stdout); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("expected empty stdout for notification, got: %s", stdout.String())
	}
}

func TestContentBlocksToText(t *testing.T) {
	blocks := []contentBlock{
		{Type: "text", Text: "hello "},
		{Type: "image"},
		{Type: "text", Text: "world"},
	}
	if got := contentBlocksToText(blocks); got != "hello world" {
		t.Errorf("got %q, want 'hello world'", got)
	}
}

func TestToolTitle(t *testing.T) {
	if got := toolTitle("bash", map[string]any{"command": "echo hello world this is a very long command that definitely exceeds the eighty character limit for truncation"}); !strings.HasSuffix(got, "…") {
		t.Errorf("expected truncation, got %q", got)
	}
	if got := toolTitle("read_file", map[string]any{"path": "/foo"}); got != "read_file: /foo" {
		t.Errorf("got %q, want 'read_file: /foo'", got)
	}
	if got := toolTitle("", nil); got != "Tool call" {
		t.Errorf("got %q, want 'Tool call'", got)
	}
}

func TestToolKind(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"bash", "execute"},
		{"shell", "execute"},
		{"edit", "edit"},
		{"read_file", "read"},
		{"search", "search"},
		{"fetch", "fetch"},
		{"think", "think"},
		{"unknown", "other"},
	}
	for _, c := range cases {
		if got := toolKind(c.name); got != c.want {
			t.Errorf("toolKind(%q)=%q, want %q", c.name, got, c.want)
		}
	}
}

func TestStringifyToolResult(t *testing.T) {
	if got := stringifyToolResult(map[string]any{"output": "hi"}); got != "hi" {
		t.Errorf("got %q, want 'hi'", got)
	}
	if got := stringifyToolResult(map[string]any{"error": "oops"}); got != "error: oops" {
		t.Errorf("got %q, want 'error: oops'", got)
	}
	if got := stringifyToolResult(nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
