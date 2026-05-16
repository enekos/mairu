package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mairu/internal/trace"
)

// Ensure KimiProvider implements Provider interface
var _ Provider = (*KimiProvider)(nil)

// KimiProvider implements the Provider interface for Kimi (Moonshot AI)
type KimiProvider struct {
	client  *KimiClient
	model   string
	apiKey  string
	baseURL string

	// Session state
	history      []Message
	isNewSession bool
	systemPrompt string
	tools        []Tool
	dynamicTools []Tool
}

// NewKimiProvider creates a new Kimi provider from configuration
func NewKimiProvider(cfg ProviderConfig) (*KimiProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("kimi API key is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		if strings.HasPrefix(cfg.APIKey, "sk-kimi-") {
			baseURL = kimiCodeBaseURL
		} else {
			baseURL = kimiDefaultBaseURL
		}
	}

	model := cfg.Model
	if model == "" {
		if strings.Contains(strings.ToLower(baseURL), "api.kimi.com") {
			model = "kimi-for-coding"
		} else {
			model = kimiDefaultModel
		}
	}

	provider := &KimiProvider{
		client:       NewKimiClient(cfg.APIKey, baseURL),
		model:        model,
		apiKey:       cfg.APIKey,
		baseURL:      cfg.BaseURL,
		history:      make([]Message, 0),
		isNewSession: true,
		tools:        make([]Tool, 0),
		dynamicTools: make([]Tool, 0),
	}

	return provider, nil
}

// GetModelName returns the current model name
func (k *KimiProvider) GetModelName() string {
	return k.model
}

// SetSystemInstruction sets the system prompt
func (k *KimiProvider) SetSystemInstruction(prompt string) {
	k.systemPrompt = prompt
}

// SetModel changes the model being used
func (k *KimiProvider) SetModel(modelName string) {
	k.model = modelName
}

// GetTools returns the currently configured base tools
func (k *KimiProvider) GetTools() []Tool {
	return append([]Tool(nil), k.tools...)
}

// SetTools replaces the currently configured base tools
func (k *KimiProvider) SetTools(tools []Tool) {
	k.tools = tools
}

// IsNewSession returns true if no messages have been exchanged yet
func (k *KimiProvider) IsNewSession() bool {
	return k.isNewSession
}

// GetHistory returns the chat history
func (k *KimiProvider) GetHistory() []Message {
	return k.history
}

// SetHistory sets the chat history
func (k *KimiProvider) SetHistory(history []Message) {
	k.history = history
	k.isNewSession = false
}

// Chat sends a single message and returns the complete response
func (k *KimiProvider) Chat(ctx context.Context, prompt string) (*ChatResponse, error) {
	k.isNewSession = false

	// Build messages
	messages := k.buildMessages(prompt)

	req := KimiChatRequest{
		Model:    k.model,
		Messages: messages,
	}

	// Add tools if configured
	if len(k.tools) > 0 || len(k.dynamicTools) > 0 {
		req.Tools = k.buildKimiTools()
	}

	resp, err := k.client.ChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from Kimi API")
	}

	choice := resp.Choices[0]

	// Update history
	k.history = append(k.history, Message{Role: "user", Content: prompt})
	k.history = append(k.history, Message{
		Role:             choice.Message.Role,
		Content:          choice.Message.Content,
		ReasoningContent: choice.Message.ReasoningContent,
		ToolCalls:        k.convertKimiToolCalls(choice.Message.ToolCalls),
	})

	return &ChatResponse{
		Content:          choice.Message.Content,
		ReasoningContent: choice.Message.ReasoningContent,
		ToolCalls:        k.convertKimiToolCalls(choice.Message.ToolCalls),
		FinishReason:     choice.FinishReason,
	}, nil
}

// ChatStream initiates a streaming chat response
func (k *KimiProvider) ChatStream(ctx context.Context, prompt string) (ChatStreamIterator, error) {
	k.isNewSession = false

	// Build messages
	messages := k.buildMessages(prompt)

	req := KimiChatRequest{
		Model:    k.model,
		Messages: messages,
	}

	// Add tools if configured
	if len(k.tools) > 0 || len(k.dynamicTools) > 0 {
		req.Tools = k.buildKimiTools()
	}

	inner, err := k.client.ChatCompletionStream(ctx, req)
	if err != nil {
		return nil, err
	}

	return &kimiHistoryTrackingIterator{
		inner:    inner,
		provider: k,
		prompt:   prompt,
	}, nil
}

// SendFunctionResponseStream sends a single tool response back to the model
func (k *KimiProvider) SendFunctionResponseStream(ctx context.Context, name string, result map[string]any) ChatStreamIterator {
	// Convert result to JSON string
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &errorStreamIterator{err: err}
	}

	// Add tool response to history
	k.history = append(k.history, Message{
		Role:       "tool",
		Content:    string(resultJSON),
		ToolCallID: name,
	})

	// Make a new streaming request with updated history
	messages := k.buildMessagesFromHistory()

	req := KimiChatRequest{
		Model:    k.model,
		Messages: messages,
	}

	if len(k.tools) > 0 || len(k.dynamicTools) > 0 {
		req.Tools = k.buildKimiTools()
	}

	iter, err := k.client.ChatCompletionStream(ctx, req)
	if err != nil {
		// Return an error iterator
		return &errorStreamIterator{err: err}
	}

	return &kimiHistoryTrackingIterator{
		inner:          iter,
		provider:       k,
		skipUserCommit: true,
	}
}

// SendFunctionResponsesStream sends multiple tool responses back to the model
func (k *KimiProvider) SendFunctionResponsesStream(ctx context.Context, responses []FunctionResponsePayload) ChatStreamIterator {
	// Add all tool responses to history
	for _, resp := range responses {
		resultJSON, err := json.Marshal(resp.Response)
		if err != nil {
			return &errorStreamIterator{err: err}
		}
		toolCallID := resp.ToolCallID
		if toolCallID == "" {
			toolCallID = resp.Name
		}
		k.history = append(k.history, Message{
			Role:       "tool",
			Content:    string(resultJSON),
			ToolCallID: toolCallID,
		})
	}

	// Make a new streaming request with updated history
	messages := k.buildMessagesFromHistory()

	req := KimiChatRequest{
		Model:    k.model,
		Messages: messages,
	}

	if len(k.tools) > 0 || len(k.dynamicTools) > 0 {
		req.Tools = k.buildKimiTools()
	}

	iter, err := k.client.ChatCompletionStream(ctx, req)
	if err != nil {
		return &errorStreamIterator{err: err}
	}

	return &kimiHistoryTrackingIterator{
		inner:          iter,
		provider:       k,
		skipUserCommit: true,
	}
}

// errorStreamIterator is a stream iterator that returns an error
type errorStreamIterator struct {
	err  error
	done bool
}

func (e *errorStreamIterator) Next() (ChatStreamChunk, error) {
	if e.done {
		return ChatStreamChunk{}, e.err
	}
	e.done = true
	return ChatStreamChunk{}, e.err
}

func (e *errorStreamIterator) Done() bool {
	return e.done
}

// GenerateJSON generates structured JSON output
func (k *KimiProvider) GenerateJSON(ctx context.Context, system, user string, schema *JSONSchema, out any) error {
	messages := []KimiMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}

	t := trace.LLMTrace{
		Model:  k.model,
		System: system,
		Prompt: user,
	}

	// Add schema instructions to user prompt if provided
	if schema != nil {
		schemaJSON, err := json.Marshal(schema)
		if err != nil {
			return fmt.Errorf("failed to marshal schema: %w", err)
		}
		t.Schema = string(schemaJSON)
		messages[1].Content += "\n\nRespond with JSON conforming to this schema:\n" + string(schemaJSON)
	}

	req := KimiChatRequest{
		Model:    k.model,
		Messages: messages,
		ResponseFormat: &KimiResponseFormat{
			Type: "json_object",
		},
	}

	start := time.Now()
	resp, err := k.client.ChatCompletion(ctx, req)
	t.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		t.Error = err.Error()
		trace.Emit(ctx, t)
		return err
	}
	t.TokensIn = resp.Usage.PromptTokens
	t.TokensOut = resp.Usage.CompletionTokens

	if len(resp.Choices) == 0 {
		t.Error = "empty response"
		trace.Emit(ctx, t)
		return fmt.Errorf("empty response")
	}

	content := resp.Choices[0].Message.Content
	t.Response = content
	if err := json.Unmarshal([]byte(content), out); err != nil {
		t.Error = fmt.Sprintf("failed to parse JSON: %v", err)
		trace.Emit(ctx, t)
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	trace.Emit(ctx, t)
	return nil
}

// GenerateContent generates plain text content
func (k *KimiProvider) GenerateContent(ctx context.Context, model, prompt string) (string, error) {
	req := KimiChatRequest{
		Model: model,
		Messages: []KimiMessage{
			{Role: "user", Content: prompt},
		},
	}

	t := trace.LLMTrace{
		Model:  model,
		Prompt: prompt,
	}

	start := time.Now()
	resp, err := k.client.ChatCompletion(ctx, req)
	t.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		t.Error = err.Error()
		trace.Emit(ctx, t)
		return "", err
	}
	t.TokensIn = resp.Usage.PromptTokens
	t.TokensOut = resp.Usage.CompletionTokens

	if len(resp.Choices) == 0 {
		t.Error = "no content generated"
		trace.Emit(ctx, t)
		return "", fmt.Errorf("no content generated")
	}

	out := resp.Choices[0].Message.Content
	t.Response = out
	trace.Emit(ctx, t)
	return out, nil
}

// Close cleans up resources.
func (k *KimiProvider) Close() error {
	return nil
}

// RegisterDynamicTools registers additional tools at runtime.
func (k *KimiProvider) RegisterDynamicTools(tools []Tool) {
	k.dynamicTools = append(k.dynamicTools, tools...)
}
