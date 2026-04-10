package agent

import (
	"context"
)

type ErrRequiresApproval struct {
	Reason string
}

func (e *ErrRequiresApproval) Error() string {
	return e.Reason
}

type ToolContext struct {
	Context context.Context
	Agent   *Agent
	OutChan chan<- AgentEvent
}

type ToolInterceptor interface {
	Name() string
	PreExecute(ctx ToolContext, toolName string, args map[string]any) (map[string]any, error)
}
