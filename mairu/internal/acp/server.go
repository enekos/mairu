// Package acp implements an Agent Client Protocol (ACP) server that exposes
// mairu's agent loop to ACP-compatible editors (Zed, etc.) over stdio.
//
// Wire format: newline-delimited JSON-RPC 2.0 on stdin/stdout. Each line is a
// complete JSON object. Logs MUST go to stderr; stdout is reserved for
// protocol traffic.
//
// Implemented methods (server side):
//   - initialize
//   - session/new
//   - session/prompt
//   - session/cancel (notification)
//
// Outbound (server-initiated) requests:
//   - session/update (notification)
//   - session/request_permission (request)
//
// Not yet implemented: session/load, fs/read_text_file, fs/write_text_file,
// terminal/*, MCP server pass-through, multiple concurrent sessions.
package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"mairu/internal/agent"
	"mairu/internal/llm"
)

const protocolVersion = 1

// --- JSON-RPC 2.0 framing ----------------------------------------------------

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *rpcError) Error() string { return e.Message }

const (
	errParseError     = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternal       = -32603
)

// --- ACP types ---------------------------------------------------------------

type initializeParams struct {
	ProtocolVersion int             `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"clientCapabilities,omitempty"`
}

type initializeResult struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities agentCapabilities `json:"agentCapabilities"`
	AuthMethods       []any             `json:"authMethods"`
}

type agentCapabilities struct {
	LoadSession        bool               `json:"loadSession"`
	PromptCapabilities promptCapabilities `json:"promptCapabilities"`
}

type promptCapabilities struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type newSessionParams struct {
	CWD        string `json:"cwd"`
	MCPServers []any  `json:"mcpServers,omitempty"`
}

type newSessionResult struct {
	SessionID string `json:"sessionId"`
}

type promptParams struct {
	SessionID string         `json:"sessionId"`
	Prompt    []contentBlock `json:"prompt"`
}

type promptResult struct {
	StopReason string `json:"stopReason"`
}

type cancelParams struct {
	SessionID string `json:"sessionId"`
}

type loadSessionParams struct {
	SessionID   string `json:"sessionId"`
	SessionName string `json:"sessionName"`
}

type loadSessionResult struct {
	Success bool `json:"success"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func contentBlocksToText(blocks []contentBlock) string {
	var out string
	for _, b := range blocks {
		if b.Type == "text" {
			out += b.Text
		}
	}
	return out
}

// session/update notification payload
type sessionUpdateParams struct {
	SessionID string         `json:"sessionId"`
	Update    map[string]any `json:"update"`
}

type permissionParams struct {
	SessionID string             `json:"sessionId"`
	ToolCall  map[string]any     `json:"toolCall"`
	Options   []permissionOption `json:"options"`
}

type permissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // "allow_once" | "reject_once"
}

type permissionResult struct {
	Outcome map[string]any `json:"outcome"`
}

// --- Server ------------------------------------------------------------------

// Server is a single-process ACP server. One agent session at a time.
type Server struct {
	in  io.Reader
	out io.Writer

	writeMu sync.Mutex
	enc     *json.Encoder
	wg      sync.WaitGroup

	providerCfg llm.ProviderConfig
	buildAgent  func(cwd string) (*agent.Agent, error)

	// session state
	sessMu      sync.Mutex
	currentID   string
	currentAg   *agent.Agent
	currentDone chan struct{} // closed when current prompt finishes

	// outbound request correlation
	nextOutID atomic.Int64
	pending   sync.Map // map[string]chan rpcMessage
}

// New constructs a server. The buildAgent factory is invoked lazily on
// session/new so that backing services (Meilisearch, embedder) only need to
// be reachable when the client actually starts a session — `initialize`
// always succeeds.
func New(providerCfg llm.ProviderConfig, buildAgent func(cwd string) (*agent.Agent, error)) *Server {
	return &Server{
		providerCfg: providerCfg,
		buildAgent:  buildAgent,
	}
}

// Run reads JSON-RPC messages from r and writes responses to w. Blocks until
// r is closed.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	s.in = r
	s.out = w
	s.enc = json.NewEncoder(w)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			s.sendError(nil, errParseError, fmt.Sprintf("parse error: %v", err))
			continue
		}
		// If this is a response to one of our outbound requests, route it.
		if msg.Method == "" && len(msg.ID) > 0 {
			if ch, ok := s.pending.LoadAndDelete(string(msg.ID)); ok {
				ch.(chan rpcMessage) <- msg
				continue
			}
		}
		// Otherwise it's an inbound request or notification.
		s.wg.Add(1)
		go func(m rpcMessage) {
			defer s.wg.Done()
			s.handle(ctx, m)
		}(msg)
	}
	err := scanner.Err()
	s.wg.Wait()
	return err
}

func (s *Server) handle(ctx context.Context, msg rpcMessage) {
	switch msg.Method {
	case "initialize":
		s.handleInitialize(msg)
	case "session/new":
		s.handleNewSession(msg)
	case "session/prompt":
		s.handlePrompt(ctx, msg)
	case "session/cancel":
		s.handleCancel(msg)
	case "session/load":
		s.handleLoadSession(msg)
	case "":
		// orphan response (already routed above) — ignore
	default:
		if len(msg.ID) > 0 {
			s.sendError(msg.ID, errMethodNotFound, "method not found: "+msg.Method)
		}
	}
}

// --- handlers ----------------------------------------------------------------

func (s *Server) handleInitialize(msg rpcMessage) {
	var p initializeParams
	if len(msg.Params) > 0 {
		_ = json.Unmarshal(msg.Params, &p)
	}
	res := initializeResult{
		ProtocolVersion: protocolVersion,
		AgentCapabilities: agentCapabilities{
			LoadSession: false,
			PromptCapabilities: promptCapabilities{
				Image:           false,
				Audio:           false,
				EmbeddedContext: false,
			},
		},
		AuthMethods: []any{},
	}
	s.sendResult(msg.ID, res)
}

func (s *Server) handleNewSession(msg rpcMessage) {
	var p newSessionParams
	if len(msg.Params) > 0 {
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			s.sendError(msg.ID, errInvalidParams, err.Error())
			return
		}
	}
	if p.CWD == "" {
		if cwd, err := os.Getwd(); err == nil {
			p.CWD = cwd
		}
	}
	ag, err := s.buildAgent(p.CWD)
	if err != nil {
		s.sendError(msg.ID, errInternal, fmt.Sprintf("agent init failed: %v", err))
		return
	}

	s.sessMu.Lock()
	if s.currentAg != nil {
		s.currentAg.Close()
	}
	id := newSessionID()
	s.currentID = id
	s.currentAg = ag
	s.sessMu.Unlock()

	s.sendResult(msg.ID, newSessionResult{SessionID: id})
}

func (s *Server) handlePrompt(ctx context.Context, msg rpcMessage) {
	var p promptParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		s.sendError(msg.ID, errInvalidParams, err.Error())
		return
	}

	s.sessMu.Lock()
	if s.currentAg == nil || p.SessionID != s.currentID {
		s.sessMu.Unlock()
		s.sendError(msg.ID, errInvalidParams, "unknown sessionId")
		return
	}
	ag := s.currentAg
	done := make(chan struct{})
	s.currentDone = done
	s.sessMu.Unlock()
	defer func() {
		s.sessMu.Lock()
		if s.currentDone == done {
			s.currentDone = nil
		}
		s.sessMu.Unlock()
		close(done)
	}()

	prompt := contentBlocksToText(p.Prompt)
	events := make(chan agent.AgentEvent, 64)
	go ag.RunStream(prompt, events)

	stop := s.bridge(ctx, p.SessionID, ag, events)
	s.sendResult(msg.ID, promptResult{StopReason: stop})
}

func (s *Server) handleCancel(msg rpcMessage) {
	var p cancelParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return
	}
	s.sessMu.Lock()
	ag := s.currentAg
	match := s.currentID == p.SessionID
	s.sessMu.Unlock()
	if ag != nil && match {
		ag.Interrupt()
	}
}

func (s *Server) handleLoadSession(msg rpcMessage) {
	var p loadSessionParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		s.sendError(msg.ID, errInvalidParams, err.Error())
		return
	}
	s.sessMu.Lock()
	ag := s.currentAg
	match := s.currentID == p.SessionID
	s.sessMu.Unlock()
	if ag == nil || !match {
		s.sendError(msg.ID, errInvalidParams, "unknown sessionId")
		return
	}
	if err := ag.LoadSession(p.SessionName); err != nil {
		s.sendError(msg.ID, errInternal, fmt.Sprintf("load session failed: %v", err))
		return
	}
	s.sendResult(msg.ID, loadSessionResult{Success: true})
}

// --- bridge: AgentEvent -> session/update ------------------------------------

// bridge consumes events until the agent emits "done" or "error", forwards
// each as an ACP session/update notification, and returns the stop reason.
func (s *Server) bridge(ctx context.Context, sessionID string, ag *agent.Agent, events <-chan agent.AgentEvent) string {
	var toolCallStack []string // parallel to mairu's serial tool execution

	for ev := range events {
		switch ev.Type {
		case "text":
			s.sendUpdate(sessionID, map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       map[string]any{"type": "text", "text": ev.Content},
			})
		case "tool_call":
			id := newToolCallID()
			toolCallStack = append(toolCallStack, id)
			s.sendUpdate(sessionID, map[string]any{
				"sessionUpdate": "tool_call",
				"toolCallId":    id,
				"title":         toolTitle(ev.ToolName, ev.ToolArgs),
				"kind":          toolKind(ev.ToolName),
				"status":        "in_progress",
				"rawInput":      ev.ToolArgs,
			})
		case "tool_result":
			var id string
			if n := len(toolCallStack); n > 0 {
				id = toolCallStack[n-1]
				toolCallStack = toolCallStack[:n-1]
			} else {
				id = newToolCallID()
			}
			s.sendUpdate(sessionID, map[string]any{
				"sessionUpdate": "tool_call_update",
				"toolCallId":    id,
				"status":        "completed",
				"rawOutput":     ev.ToolResult,
				"content":       []any{map[string]any{"type": "content", "content": map[string]any{"type": "text", "text": stringifyToolResult(ev.ToolResult)}}},
			})
		case "approval_request":
			var id string
			if n := len(toolCallStack); n > 0 {
				id = toolCallStack[n-1]
			}
			approved := s.requestPermission(ctx, sessionID, ev, id)
			ag.ApproveAction(approved)
		case "status", "log", "diff", "bash_output":
			// surface as agent thoughts so the UI can show them
			s.sendUpdate(sessionID, map[string]any{
				"sessionUpdate": "agent_thought_chunk",
				"content":       map[string]any{"type": "text", "text": ev.Content},
			})
		case "error":
			s.sendUpdate(sessionID, map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       map[string]any{"type": "text", "text": "Error: " + ev.Content},
			})
			return "refusal"
		case "done":
			return "end_turn"
		}
		select {
		case <-ctx.Done():
			if ag != nil {
				ag.Interrupt()
			}
			return "cancelled"
		default:
		}
	}
	return "end_turn"
}

// requestPermission fires session/request_permission and returns whether the
// client allowed the action. Falls back to deny on error/timeout.
func (s *Server) requestPermission(ctx context.Context, sessionID string, ev agent.AgentEvent, toolCallID string) bool {
	title := "Pending action"
	if toolCallID != "" {
		title = "Action requires approval"
	}
	params := permissionParams{
		SessionID: sessionID,
		ToolCall: map[string]any{
			"toolCallId": toolCallID,
			"title":      title,
			"kind":       "other",
			"rawInput":   map[string]any{"reason": ev.Content},
		},
		Options: []permissionOption{
			{OptionID: "allow", Name: "Allow", Kind: "allow_once"},
			{OptionID: "deny", Name: "Deny", Kind: "reject_once"},
		},
	}
	res, err := s.callClient(ctx, "session/request_permission", params)
	if err != nil {
		slog.Warn("acp: request_permission failed", "error", err)
		return false
	}
	var pr permissionResult
	if err := json.Unmarshal(res, &pr); err != nil {
		return false
	}
	if t, _ := pr.Outcome["type"].(string); t == "selected" {
		if id, _ := pr.Outcome["optionId"].(string); id == "allow" {
			return true
		}
	}
	return false
}

// --- outbound RPC ------------------------------------------------------------

func (s *Server) sendResult(id json.RawMessage, result any) {
	raw, err := json.Marshal(result)
	if err != nil {
		s.sendError(id, errInternal, err.Error())
		return
	}
	s.write(rpcMessage{JSONRPC: "2.0", ID: id, Result: raw})
}

func (s *Server) sendError(id json.RawMessage, code int, message string) {
	s.write(rpcMessage{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}})
}

func (s *Server) sendUpdate(sessionID string, update map[string]any) {
	params := sessionUpdateParams{SessionID: sessionID, Update: update}
	raw, _ := json.Marshal(params)
	s.write(rpcMessage{JSONRPC: "2.0", Method: "session/update", Params: raw})
}

func (s *Server) callClient(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := strconv.FormatInt(s.nextOutID.Add(1), 10)
	rawID := json.RawMessage(id)
	ch := make(chan rpcMessage, 1)
	s.pending.Store(string(rawID), ch)
	defer s.pending.Delete(string(rawID))

	rawParams, _ := json.Marshal(params)
	s.write(rpcMessage{JSONRPC: "2.0", ID: rawID, Method: method, Params: rawParams})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, errors.New(resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (s *Server) write(msg rpcMessage) {
	if msg.JSONRPC == "" {
		msg.JSONRPC = "2.0"
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.enc.Encode(&msg); err != nil {
		slog.Error("acp: write failed", "error", err)
	}
}

// --- helpers -----------------------------------------------------------------

var sessionCounter atomic.Int64

func newSessionID() string {
	return fmt.Sprintf("sess_%d", sessionCounter.Add(1))
}

var toolCallCounter atomic.Int64

func newToolCallID() string {
	return fmt.Sprintf("tc_%d", toolCallCounter.Add(1))
}

func toolTitle(name string, args map[string]any) string {
	if name == "" {
		return "Tool call"
	}
	if cmd, ok := args["command"].(string); ok && cmd != "" {
		return fmt.Sprintf("%s: %s", name, truncate(cmd, 80))
	}
	if path, ok := args["path"].(string); ok && path != "" {
		return fmt.Sprintf("%s: %s", name, path)
	}
	return name
}

func toolKind(name string) string {
	switch name {
	case "bash", "shell":
		return "execute"
	case "edit", "write_file", "create_file":
		return "edit"
	case "read_file", "files":
		return "read"
	case "search", "grep":
		return "search"
	case "fetch", "browser":
		return "fetch"
	case "think", "plan":
		return "think"
	}
	return "other"
}

func stringifyToolResult(m map[string]any) string {
	if m == nil {
		return ""
	}
	if s, ok := m["output"].(string); ok {
		return s
	}
	if s, ok := m["error"].(string); ok {
		return "error: " + s
	}
	raw, _ := json.Marshal(m)
	return string(raw)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
