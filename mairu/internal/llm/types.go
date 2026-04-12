package llm

import (
	"context"
	"time"
)

// ProviderType represents the supported LLM provider types
type ProviderType string

const (
	ProviderGemini ProviderType = "gemini"
	ProviderKimi   ProviderType = "kimi"
)

// ProviderConfig contains configuration for creating a provider
type ProviderConfig struct {
	Type    ProviderType
	APIKey  string
	Model   string // Default model override
	BaseURL string // Optional: for custom endpoints
}

// Message represents a chat message (provider-agnostic)
type Message struct {
	Role    string // "system", "user", "assistant", "tool"
	Content string
	// ToolCalls is populated when Role is "assistant" and the model wants to call tools
	ToolCalls []ToolCall
	// ToolCallID is populated when Role is "tool" - matches the ID from ToolCalls
	ToolCallID string
}

// ToolCall represents a tool invocation requested by the model
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

// Tool represents a callable tool/function
type Tool struct {
	Name        string
	Description string
	Parameters  *JSONSchema
}

// JSONSchema represents a JSON schema for structured output (provider-agnostic)
type JSONSchema struct {
	Type        JSONSchemaType
	Description string
	Properties  map[string]*JSONSchema
	Required    []string
	Items       *JSONSchema // For array type
	Enum        []string    // For string enum types
}

// JSONSchemaType represents the type of a JSON schema property
type JSONSchemaType string

const (
	TypeObject  JSONSchemaType = "object"
	TypeArray   JSONSchemaType = "array"
	TypeString  JSONSchemaType = "string"
	TypeInteger JSONSchemaType = "integer"
	TypeNumber  JSONSchemaType = "number"
	TypeBoolean JSONSchemaType = "boolean"
)

// ChatResponse represents a non-streaming chat response
type ChatResponse struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string // "stop", "tool_calls", "length", etc.
}

// ChatStreamIterator represents a streaming chat response iterator
type ChatStreamIterator interface {
	// Next returns the next chunk of the response. Empty string means no content yet.
	Next() (ChatStreamChunk, error)
	// Done returns true when the stream is complete
	Done() bool
}

// ChatStreamChunk represents a single chunk from a streaming response
type ChatStreamChunk struct {
	Content      string
	ToolCalls    []ToolCall // Partial tool calls may accumulate
	FinishReason string
}

// FunctionResponsePayload represents a tool/function response to send back to the model
type FunctionResponsePayload struct {
	Name     string
	Response map[string]any
}

// CachingProvider is an optional interface for providers that support prompt caching
type CachingProvider interface {
	CacheContext(ctx context.Context, systemPrompt string, ttl time.Duration) (string, error)
	SetCachedContent(ctx context.Context, name string) error
	DeleteCachedContent(ctx context.Context, name string) error
}

// SchemaConverter converts a genai.Schema to our provider-agnostic JSONSchema
// This is kept in gemini-specific files to avoid importing genai here
