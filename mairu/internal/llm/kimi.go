package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
		return nil, fmt.Errorf("Kimi API key is required")
	}

	model := cfg.Model
	if model == "" {
		model = kimiDefaultModel
	}

	provider := &KimiProvider{
		client:       NewKimiClient(cfg.APIKey, cfg.BaseURL),
		model:        model,
		apiKey:       cfg.APIKey,
		baseURL:      cfg.BaseURL,
		history:      make([]Message, 0),
		isNewSession: true,
		tools:        make([]Tool, 0),
		dynamicTools: make([]Tool, 0),
	}

	provider.SetupTools()
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
		Role:      choice.Message.Role,
		Content:   choice.Message.Content,
		ToolCalls: k.convertKimiToolCalls(choice.Message.ToolCalls),
	})

	return &ChatResponse{
		Content:      choice.Message.Content,
		ToolCalls:    k.convertKimiToolCalls(choice.Message.ToolCalls),
		FinishReason: choice.FinishReason,
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

	return k.client.ChatCompletionStream(ctx, req)
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
		ToolCallID: name, // Use function name as ID since Gemini doesn't use IDs
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

	return iter
}

// SendFunctionResponsesStream sends multiple tool responses back to the model
func (k *KimiProvider) SendFunctionResponsesStream(ctx context.Context, responses []FunctionResponsePayload) ChatStreamIterator {
	// Add all tool responses to history
	for _, resp := range responses {
		resultJSON, err := json.Marshal(resp.Response)
		if err != nil {
			return &errorStreamIterator{err: err}
		}
		k.history = append(k.history, Message{
			Role:       "tool",
			Content:    string(resultJSON),
			ToolCallID: resp.Name,
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

	return iter
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

	// Add schema instructions to user prompt if provided
	if schema != nil {
		schemaJSON, err := json.Marshal(schema)
		if err != nil {
			return fmt.Errorf("failed to marshal schema: %w", err)
		}
		messages[1].Content += "\n\nRespond with JSON conforming to this schema:\n" + string(schemaJSON)
	}

	req := KimiChatRequest{
		Model:    k.model,
		Messages: messages,
		ResponseFormat: &KimiResponseFormat{
			Type: "json_object",
		},
	}

	resp, err := k.client.ChatCompletion(ctx, req)
	if err != nil {
		return err
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("empty response")
	}

	content := resp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(content), out); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

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

	resp, err := k.client.ChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	return resp.Choices[0].Message.Content, nil
}

// SetupTools configures the available tools
func (k *KimiProvider) SetupTools() {
	// Define built-in tools for Kimi
	k.tools = []Tool{
		{
			Name:        "replace_block",
			Description: "Safely apply a Search-and-Replace block edit to a file. You must provide the EXACT existing code block you want to replace, including all whitespace. This is much safer and more reliable than multi_edit.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"file_path": {Type: TypeString, Description: "The relative path to the file."},
					"old_code":  {Type: TypeString, Description: "The exact existing code block to be replaced. Must match exactly, including indentation."},
					"new_code":  {Type: TypeString, Description: "The new code block to insert in its place."},
				},
				Required: []string{"file_path", "old_code", "new_code"},
			},
		},
		{
			Name:        "multi_edit",
			Description: "Apply a block replacement to a specific file.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"file_path":  {Type: TypeString, Description: "The relative path to the file."},
					"start_line": {Type: TypeInteger, Description: "The 1-indexed starting line to replace."},
					"end_line":   {Type: TypeInteger, Description: "The 1-indexed ending line to replace."},
					"content":    {Type: TypeString, Description: "The new content to insert in place of those lines."},
				},
				Required: []string{"file_path", "start_line", "end_line", "content"},
			},
		},
		{
			Name:        "bash",
			Description: "Execute a bash command in the project root directory. Use this to run tests, linters, or explore the file system.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"command":    {Type: TypeString, Description: "The bash command to execute."},
					"timeout_ms": {Type: TypeInteger, Description: "Optional timeout in milliseconds (default 30000)."},
				},
				Required: []string{"command"},
			},
		},
		{
			Name:        "read_file",
			Description: "Read the contents of a file. Supports reading specific sections using offset and limit. Output is truncated to 2000 lines by default. Use offset/limit for large files.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"file_path": {Type: TypeString, Description: "The relative path to the file."},
					"offset":    {Type: TypeInteger, Description: "The line number to start reading from (1-indexed). Defaults to 1."},
					"limit":     {Type: TypeInteger, Description: "Maximum number of lines to read. Defaults to 2000."},
				},
				Required: []string{"file_path"},
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file, overwriting it completely. If editing an existing file, prefer multi_edit.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"file_path": {Type: TypeString, Description: "The relative path to the file."},
					"content":   {Type: TypeString, Description: "The entire new content of the file."},
				},
				Required: []string{"file_path", "content"},
			},
		},
		{
			Name:        "find_files",
			Description: "Find files by glob pattern.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"pattern": {Type: TypeString, Description: "The glob pattern (e.g., src/**/*.ts)."},
				},
				Required: []string{"pattern"},
			},
		},
		{
			Name:        "search_codebase",
			Description: "Search the codebase by text/regex query or by symbol name (surgical read).",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"query":       {Type: TypeString, Description: "Text or regex to search in files."},
					"symbol_name": {Type: TypeString, Description: "Exact symbol name to look up (function, method, class)."},
				},
			},
		},
		{
			Name:        "review_work",
			Description: "Before finishing a task, use this tool to review the work done against the requirements, and self-critique it for potential flaws or missed edge cases. This ensures better accuracy and reliability.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"summary":  {Type: TypeString, Description: "A summary of the changes made and how they resolve the task."},
					"critique": {Type: TypeString, Description: "A self-critique identifying any edge cases, potential failures, or unaddressed requirements."},
				},
				Required: []string{"summary", "critique"},
			},
		},
		{
			Name:        "delegate_task",
			Description: "Delegate a complex sub-task to another AI agent. Useful for researching or exploring while you focus on the main task.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"task_description": {Type: TypeString, Description: "A highly detailed prompt describing what the sub-agent should do."},
				},
				Required: []string{"task_description"},
			},
		},
		{
			Name:        "scrape_url",
			Description: "Scrape a web page and extract structured information based on a prompt. Use this when you need specific data extracted intelligently from a website.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"url":    {Type: TypeString, Description: "The full URL to scrape (e.g., https://example.com)."},
					"prompt": {Type: TypeString, Description: "The instructions on what information to extract from the page."},
				},
				Required: []string{"url", "prompt"},
			},
		},
		{
			Name:        "search_web",
			Description: "Search the web for a query and extract structured information from the top results based on a prompt.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"query":  {Type: TypeString, Description: "The search query to look up on the web."},
					"prompt": {Type: TypeString, Description: "The instructions on what information to extract from the search results."},
				},
				Required: []string{"query", "prompt"},
			},
		},
		{
			Name:        "fetch_url",
			Description: "Fetch the text content of a web page by URL. Useful for reading documentation or external resources.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"url": {Type: TypeString, Description: "The full URL to fetch (e.g., https://example.com)."},
				},
				Required: []string{"url"},
			},
		},
		{
			Name:        "delete_file",
			Description: "Delete a file or directory.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"path": {Type: TypeString, Description: "The relative path to the file or directory."},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "browser_context",
			Description: "Get real-time browser context from the Mairu browser extension.",
			Parameters: &JSONSchema{
				Type: TypeObject,
				Properties: map[string]*JSONSchema{
					"command": {Type: TypeString, Description: "The command to run: current, history, search, or session."},
					"query":   {Type: TypeString, Description: "The search query (only for 'search' command)."},
					"limit":   {Type: TypeInteger, Description: "The limit for search results (only for 'search' command)."},
				},
				Required: []string{"command"},
			},
		},
	}
}

// RegisterDynamicTools registers additional tools at runtime
func (k *KimiProvider) RegisterDynamicTools(tools []Tool) {
	k.dynamicTools = append(k.dynamicTools, tools...)
}

// Close cleans up resources
func (k *KimiProvider) Close() error {
	// Nothing to close for HTTP client
	return nil
}

// Helper methods

func (k *KimiProvider) buildMessages(userPrompt string) []KimiMessage {
	var messages []KimiMessage

	// Add system prompt if set
	if k.systemPrompt != "" {
		messages = append(messages, KimiMessage{
			Role:    "system",
			Content: k.systemPrompt,
		})
	}

	// Add history
	for _, msg := range k.history {
		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		}

		kMsg := KimiMessage{
			Role:    role,
			Content: msg.Content,
		}

		// Handle tool calls
		if len(msg.ToolCalls) > 0 {
			kMsg.ToolCalls = k.convertToolCallsToKimi(msg.ToolCalls)
		}

		// Handle tool responses
		if msg.ToolCallID != "" {
			kMsg.ToolCallID = msg.ToolCallID
		}

		messages = append(messages, kMsg)
	}

	// Add user message
	messages = append(messages, KimiMessage{
		Role:    "user",
		Content: userPrompt,
	})

	return messages
}

func (k *KimiProvider) buildMessagesFromHistory() []KimiMessage {
	var messages []KimiMessage

	// Add system prompt if set
	if k.systemPrompt != "" {
		messages = append(messages, KimiMessage{
			Role:    "system",
			Content: k.systemPrompt,
		})
	}

	// Add all history
	for _, msg := range k.history {
		role := msg.Role
		if role == "model" || role == "assistant" {
			role = "assistant"
		}

		kMsg := KimiMessage{
			Role:    role,
			Content: msg.Content,
		}

		if len(msg.ToolCalls) > 0 {
			kMsg.ToolCalls = k.convertToolCallsToKimi(msg.ToolCalls)
		}

		if msg.ToolCallID != "" {
			kMsg.ToolCallID = msg.ToolCallID
		}

		messages = append(messages, kMsg)
	}

	return messages
}

func (k *KimiProvider) buildKimiTools() []KimiTool {
	allTools := append(k.tools, k.dynamicTools...)
	kimiTools := make([]KimiTool, 0, len(allTools))

	for _, tool := range allTools {
		kimiTools = append(kimiTools, KimiTool{
			Type: "function",
			Function: KimiFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	return kimiTools
}

func (k *KimiProvider) convertKimiToolCalls(calls []KimiToolCall) []ToolCall {
	toolCalls := make([]ToolCall, 0, len(calls))
	for _, call := range calls {
		var args map[string]any
		if call.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				slog.Error("failed to unmarshal tool call arguments", "error", err)
			}
		}
		toolCalls = append(toolCalls, ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: args,
		})
	}
	return toolCalls
}

func (k *KimiProvider) convertToolCallsToKimi(calls []ToolCall) []KimiToolCall {
	kimiCalls := make([]KimiToolCall, 0, len(calls))
	for _, call := range calls {
		argsJSON, err := json.Marshal(call.Arguments)
		if err != nil {
			slog.Error("failed to marshal tool call arguments", "error", err)
			argsJSON = []byte("{}")
		}
		kimiCalls = append(kimiCalls, KimiToolCall{
			ID:   call.ID,
			Type: "function",
			Function: KimiFunctionCall{
				Name:      call.Name,
				Arguments: string(argsJSON),
			},
		})
	}
	return kimiCalls
}
