package llm

import "context"

// Provider is the unified interface for LLM providers (Gemini, Kimi, etc.)
type Provider interface {
	// Chat sends a single message and returns the complete response
	Chat(ctx context.Context, prompt string) (*ChatResponse, error)

	// ChatStream initiates a streaming chat response
	ChatStream(ctx context.Context, prompt string) (ChatStreamIterator, error)

	// SendFunctionResponse sends a single tool response back to the model (streaming)
	SendFunctionResponseStream(ctx context.Context, name string, result map[string]any) ChatStreamIterator

	// SendFunctionResponses sends multiple tool responses back to the model (streaming)
	SendFunctionResponsesStream(ctx context.Context, responses []FunctionResponsePayload) ChatStreamIterator

	// GenerateJSON generates structured JSON output according to a schema
	GenerateJSON(ctx context.Context, system, user string, schema *JSONSchema, out any) error

	// GenerateContent generates plain text content (non-chat, for one-off generation)
	GenerateContent(ctx context.Context, model, prompt string) (string, error)

	// SetSystemInstruction sets the system prompt for the chat session
	SetSystemInstruction(prompt string)

	// SetModel changes the model being used
	SetModel(modelName string)

	// GetModelName returns the current model name
	GetModelName() string

	// GetHistory returns the chat history
	GetHistory() []Message

	// SetHistory sets the chat history
	SetHistory(history []Message)

	// IsNewSession returns true if no messages have been exchanged yet
	IsNewSession() bool

	// SetupTools configures the available tools for the model
	SetupTools()

	// RegisterDynamicTools registers additional tools at runtime
	RegisterDynamicTools(tools []Tool)

	// Close cleans up any resources
	Close() error
}
