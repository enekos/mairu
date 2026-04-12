package llm

// Kimi API request/response types
// Kimi uses an OpenAI-compatible API format

// KimiChatRequest represents a chat completion request
type KimiChatRequest struct {
	Model          string              `json:"model"`
	Messages       []KimiMessage       `json:"messages"`
	Stream         bool                `json:"stream,omitempty"`
	Tools          []KimiTool          `json:"tools,omitempty"`
	ToolChoice     string              `json:"tool_choice,omitempty"`
	ResponseFormat *KimiResponseFormat `json:"response_format,omitempty"`
	Temperature    float64             `json:"temperature,omitempty"`
	MaxTokens      int                 `json:"max_tokens,omitempty"`
}

// KimiMessage represents a message in the chat
type KimiMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []KimiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
}

// KimiTool represents a tool definition
type KimiTool struct {
	Type     string          `json:"type"`
	Function KimiFunctionDef `json:"function"`
}

// KimiFunctionDef represents a function definition
type KimiFunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  *JSONSchema `json:"parameters"`
}

// KimiToolCall represents a tool invocation
type KimiToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function KimiFunctionCall `json:"function"`
}

// KimiFunctionCall represents a function call details
type KimiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// KimiResponseFormat specifies the response format
type KimiResponseFormat struct {
	Type       string      `json:"type"` // "json_object" or "json_schema"
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

// KimiChatResponse represents a chat completion response (non-streaming)
type KimiChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []KimiChoice `json:"choices"`
	Usage   KimiUsage    `json:"usage"`
}

// KimiChoice represents a choice in the response
type KimiChoice struct {
	Index        int          `json:"index"`
	Message      KimiMessage  `json:"message"`
	FinishReason string       `json:"finish_reason"`
	Delta        *KimiMessage `json:"delta,omitempty"` // For streaming
}

// KimiUsage represents token usage
type KimiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// KimiStreamResponse represents a streaming response chunk
type KimiStreamResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []KimiChoice `json:"choices"`
}

// KimiErrorResponse represents an error response
type KimiErrorResponse struct {
	Error KimiError `json:"error"`
}

// KimiError represents error details
type KimiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// KimiEmbeddingRequest represents an embedding request
type KimiEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// KimiEmbeddingResponse represents an embedding response
type KimiEmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []KimiEmbedding `json:"data"`
	Model  string          `json:"model"`
	Usage  KimiUsage       `json:"usage"`
}

// KimiEmbedding represents a single embedding
type KimiEmbedding struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}
