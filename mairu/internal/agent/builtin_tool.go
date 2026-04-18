package agent

import (
	"context"

	"mairu/internal/llm"
)

// BuiltinTool is a built-in agent capability: a schema the LLM sees and a handler
// the agent calls when the LLM invokes it.
type BuiltinTool interface {
	Definition() llm.Tool
	Execute(ctx context.Context, args map[string]any, a *Agent, outChan chan<- AgentEvent) (map[string]any, error)
}
