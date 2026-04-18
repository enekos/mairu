package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"mairu/internal/llm"
)

// mockStreamIterator is a programmable streaming iterator.
type mockStreamIterator struct {
	chunks []llm.ChatStreamChunk
	idx    int
	done   bool
}

func (m *mockStreamIterator) Next() (llm.ChatStreamChunk, error) {
	if m.idx >= len(m.chunks) {
		m.done = true
		return llm.ChatStreamChunk{}, errors.New("EOF")
	}
	chunk := m.chunks[m.idx]
	m.idx++
	if chunk.FinishReason == "stop" || chunk.FinishReason == "length" {
		m.done = true
	}
	return chunk, nil
}

func (m *mockStreamIterator) Done() bool {
	return m.done
}

// mockProvider implements llm.Provider for end-to-end testing.
type mockProvider struct {
	history      []llm.Message
	isNew        bool
	systemPrompt string
	model        string
	tools        []llm.Tool
	dynamicTools []llm.Tool

	// Programmable responses
	chatStreamResp      []llm.ChatStreamChunk
	chatStreamErr       error
	funcResponseResp    []llm.ChatStreamChunk
	generateContentResp string
	generateContentErr  error

	// Hooks for assertions
	setToolsCalls [][]llm.Tool
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		isNew:   true,
		model:   "mock-model",
		history: []llm.Message{},
		tools: []llm.Tool{
			{Name: "bash", Description: "run shell commands"},
			{Name: "read_file", Description: "read files"},
			{Name: "review_work", Description: "review work"},
		},
	}
}

func (m *mockProvider) Chat(ctx context.Context, prompt string) (*llm.ChatResponse, error) {
	m.isNew = false
	m.history = append(m.history, llm.Message{Role: "user", Content: prompt})
	m.history = append(m.history, llm.Message{Role: "assistant", Content: "mock response"})
	return &llm.ChatResponse{Content: "mock response", FinishReason: "stop"}, nil
}

func (m *mockProvider) ChatStream(ctx context.Context, prompt string) (llm.ChatStreamIterator, error) {
	m.isNew = false
	if m.chatStreamErr != nil {
		return nil, m.chatStreamErr
	}
	m.history = append(m.history, llm.Message{Role: "user", Content: prompt})
	var toolCalls []llm.ToolCall
	var content string
	for _, c := range m.chatStreamResp {
		if len(c.ToolCalls) > 0 {
			toolCalls = append(toolCalls, c.ToolCalls...)
		}
		content += c.Content
	}
	m.history = append(m.history, llm.Message{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	})
	return &mockStreamIterator{chunks: m.chatStreamResp}, nil
}

func (m *mockProvider) SendFunctionResponseStream(ctx context.Context, name string, result map[string]any) llm.ChatStreamIterator {
	m.history = append(m.history, llm.Message{Role: "tool", ToolCallID: name, Content: fmt.Sprintf("%v", result)})
	var content string
	var toolCalls []llm.ToolCall
	for _, c := range m.funcResponseResp {
		content += c.Content
		if len(c.ToolCalls) > 0 {
			toolCalls = append(toolCalls, c.ToolCalls...)
		}
	}
	if len(m.funcResponseResp) > 0 {
		m.history = append(m.history, llm.Message{Role: "assistant", Content: content, ToolCalls: toolCalls})
	}
	return &mockStreamIterator{chunks: m.funcResponseResp}
}

func (m *mockProvider) SendFunctionResponsesStream(ctx context.Context, responses []llm.FunctionResponsePayload) llm.ChatStreamIterator {
	for _, r := range responses {
		toolCallID := r.ToolCallID
		if toolCallID == "" {
			toolCallID = r.Name
		}
		m.history = append(m.history, llm.Message{Role: "tool", ToolCallID: toolCallID, Content: fmt.Sprintf("%v", r.Response)})
	}
	var content string
	var toolCalls []llm.ToolCall
	for _, c := range m.funcResponseResp {
		content += c.Content
		if len(c.ToolCalls) > 0 {
			toolCalls = append(toolCalls, c.ToolCalls...)
		}
	}
	if len(m.funcResponseResp) > 0 {
		m.history = append(m.history, llm.Message{Role: "assistant", Content: content, ToolCalls: toolCalls})
	}
	return &mockStreamIterator{chunks: m.funcResponseResp}
}

func (m *mockProvider) GenerateJSON(ctx context.Context, system, user string, schema *llm.JSONSchema, out any) error {
	return errors.New("not implemented")
}

func (m *mockProvider) GenerateContent(ctx context.Context, model, prompt string) (string, error) {
	if m.generateContentErr != nil {
		return "", m.generateContentErr
	}
	return m.generateContentResp, nil
}

func (m *mockProvider) SetSystemInstruction(prompt string) { m.systemPrompt = prompt }
func (m *mockProvider) SetModel(modelName string)          { m.model = modelName }
func (m *mockProvider) GetModelName() string               { return m.model }
func (m *mockProvider) GetHistory() []llm.Message          { return append([]llm.Message(nil), m.history...) }
func (m *mockProvider) SetHistory(history []llm.Message) {
	m.history = append([]llm.Message(nil), history...)
	m.isNew = false
}
func (m *mockProvider) IsNewSession() bool   { return m.isNew }
func (m *mockProvider) SetupTools()          {}
func (m *mockProvider) GetTools() []llm.Tool { return append([]llm.Tool(nil), m.tools...) }
func (m *mockProvider) SetTools(tools []llm.Tool) {
	m.setToolsCalls = append(m.setToolsCalls, append([]llm.Tool(nil), tools...))
}
func (m *mockProvider) RegisterDynamicTools(tools []llm.Tool) {
	m.dynamicTools = append(m.dynamicTools, tools...)
}
func (m *mockProvider) Close() error { return nil }

func collectEvents(ag *Agent, prompt string) []AgentEvent {
	outChan := make(chan AgentEvent, 100)
	go ag.RunStream(prompt, outChan)

	var events []AgentEvent
	for ev := range outChan {
		events = append(events, ev)
	}
	return events
}

func hasEventType(events []AgentEvent, typ string) bool {
	for _, ev := range events {
		if ev.Type == typ {
			return true
		}
	}
	return false
}

func TestEndToEnd_SuccessfulRun_PersistsHistory(t *testing.T) {
	mock := newMockProvider()
	mock.isNew = true
	mock.chatStreamResp = []llm.ChatStreamChunk{
		{Content: "Hello!"},
		{FinishReason: "stop"},
	}

	ag, err := NewWithProvider(t.TempDir(), mock, Config{
		AgentSystemData: map[string]any{"CliHelp": ""},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	mock.history = []llm.Message{{Role: "user", Content: "previous"}}
	ag.SetCouncilEnabled(false)

	events := collectEvents(ag, "say hello")

	if !hasEventType(events, "text") || !hasEventType(events, "done") {
		t.Fatalf("expected text and done events, got %+v", events)
	}

	if len(mock.history) != 3 {
		t.Fatalf("expected 3 history messages, got %d: %+v", len(mock.history), mock.history)
	}
}

func TestEndToEnd_FailedRun_RestoresHistory(t *testing.T) {
	mock := newMockProvider()
	mock.isNew = true
	mock.chatStreamErr = errors.New("network error")

	ag, err := NewWithProvider(t.TempDir(), mock, Config{
		AgentSystemData: map[string]any{"CliHelp": ""},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	mock.history = []llm.Message{{Role: "user", Content: "previous"}}
	ag.SetCouncilEnabled(false)

	events := collectEvents(ag, "say hello")

	if !hasEventType(events, "error") {
		t.Fatalf("expected error event, got %+v", events)
	}

	if len(mock.history) != 1 {
		t.Fatalf("expected history restored to 1 message, got %d: %+v", len(mock.history), mock.history)
	}
}

func TestEndToEnd_ToolCallAndResponse(t *testing.T) {
	mock := newMockProvider()
	mock.isNew = true
	mock.chatStreamResp = []llm.ChatStreamChunk{
		{ToolCalls: []llm.ToolCall{{ID: "tc1", Name: "review_work", Arguments: map[string]any{"summary": "ok", "critique": "none"}}}},
		{FinishReason: "stop"},
	}
	mock.funcResponseResp = []llm.ChatStreamChunk{
		{Content: "Review complete."},
		{FinishReason: "stop"},
	}

	ag, err := NewWithProvider(t.TempDir(), mock, Config{
		AgentSystemData: map[string]any{"CliHelp": ""},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	mock.history = []llm.Message{}
	ag.SetCouncilEnabled(false)

	events := collectEvents(ag, "please review the work")

	if !hasEventType(events, "tool_call") {
		t.Fatalf("expected tool_call event, got %+v", events)
	}
	if !hasEventType(events, "done") {
		t.Fatalf("expected done event, got %+v", events)
	}

	// History should be: user, assistant(toolcall), tool, assistant(text)
	if len(mock.history) != 4 {
		t.Fatalf("expected 4 history messages, got %d: %+v", len(mock.history), mock.history)
	}
	if mock.history[1].Role != "assistant" || len(mock.history[1].ToolCalls) != 1 {
		t.Fatalf("expected assistant message with tool call, got %+v", mock.history[1])
	}
	if mock.history[2].Role != "tool" {
		t.Fatalf("expected tool message, got %+v", mock.history[2])
	}
	if mock.history[3].Role != "assistant" {
		t.Fatalf("expected final assistant message, got %+v", mock.history[3])
	}
}

func TestEndToEnd_PlannerSubsetsToolsForComplexPrompt(t *testing.T) {
	mock := newMockProvider()
	mock.isNew = true
	mock.generateContentResp = `{"tools": ["review_work", "read_file"]}`
	mock.chatStreamResp = []llm.ChatStreamChunk{
		{Content: "Will do!"},
		{FinishReason: "stop"},
	}

	ag, err := NewWithProvider(t.TempDir(), mock, Config{
		AgentSystemData: map[string]any{"CliHelp": ""},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ag.SetCouncilEnabled(false)

	// Complex prompt (>50 words) to trigger planner
	prompt := "first review the current codebase for issues and then read the main file to understand the structure and finally provide a summary of what you found"
	events := collectEvents(ag, prompt)

	if !hasEventType(events, "text") || !hasEventType(events, "done") {
		t.Fatalf("expected text and done events, got %+v", events)
	}

	// SetTools is called once during construction (builtin injection) + subset + restore = at least 3.
	if len(mock.setToolsCalls) < 3 {
		t.Fatalf("expected at least 3 SetTools calls (construction + subset + restore), got %d", len(mock.setToolsCalls))
	}

	// The planner subset is the second-to-last call; the last is the restore.
	subset := mock.setToolsCalls[len(mock.setToolsCalls)-2]
	if len(subset) != 2 {
		t.Fatalf("expected planner to subset to 2 tools, got %d: %+v", len(subset), subset)
	}
	names := map[string]bool{subset[0].Name: true, subset[1].Name: true}
	if !names["review_work"] || !names["read_file"] {
		t.Fatalf("expected review_work and read_file in subset, got %+v", subset)
	}

	// Final restore should contain all tools
	restored := mock.setToolsCalls[len(mock.setToolsCalls)-1]
	if len(restored) != 3 {
		t.Fatalf("expected restored tool count 3, got %d", len(restored))
	}
}
