package agent

import (
	"context"
	"errors"
	"fmt"

	"mairu/internal/llm"
)

func (a *Agent) searchBySymbolName(symName string, outChan chan<- AgentEvent) map[string]any {
	if a.locator == nil {
		return map[string]any{"error": "SymbolLocator is not configured for this agent"}
	}
	locations, err := a.locator.FindSymbol(symName)
	if err != nil || len(locations) == 0 {
		return map[string]any{"error": fmt.Sprintf("symbol '%s' not found", symName)}
	}

	outChan <- AgentEvent{
		Type:    "status",
		Content: fmt.Sprintf("📄 Reading symbol from %s", locations[0].FilePath),
	}

	content, err := a.SurgicalRead(locations[0])
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return map[string]any{"content": content}
}

func (a *Agent) executeToolCall(ctx context.Context, funcCall llm.ToolCall, outChan chan<- AgentEvent) map[string]any {
	a.emitLog(outChan, "Tool args for %s: %v", funcCall.Name, funcCall.Arguments)
	outChan <- AgentEvent{
		Type:     "tool_call",
		Content:  fmt.Sprintf("🔧 Tool Call: %s", funcCall.Name),
		ToolName: funcCall.Name,
		ToolArgs: funcCall.Arguments,
	}
	toolCtx := ToolContext{
		Context: ctx,
		Agent:   a,
		OutChan: outChan,
	}
	for _, interceptor := range a.interceptors {
		var err error
		funcCall.Arguments, err = interceptor.PreExecute(toolCtx, funcCall.Name, funcCall.Arguments)
		if err != nil {
			var approvalErr *ErrRequiresApproval
			if errors.As(err, &approvalErr) {
				if a.Unattended {
					outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("✅ Auto-approved (Minion Mode): %s", approvalErr.Reason)}
				} else {
					approvalCh := make(chan bool, 1)
					a.mu.Lock()
					a.approvalChan = approvalCh
					a.mu.Unlock()

					outChan <- AgentEvent{
						Type:    "approval_request",
						Content: fmt.Sprintf("⚠️ Action requires approval:\n\n%s\n\nApprove or deny this action by typing `/approve` or `/deny`.", approvalErr.Reason),
					}

					approved := false
					select {
					case <-ctx.Done():
						return map[string]any{"error": "tool execution cancelled"}
					case approved = <-approvalCh:
					}

					if !approved {
						outChan <- AgentEvent{Type: "status", Content: "❌ Action denied by user."}
						return map[string]any{"error": "User denied action execution."}
					}
					outChan <- AgentEvent{Type: "status", Content: "✅ Action approved."}
				}
			} else {
				outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("❌ Blocked by %s", interceptor.Name())}
				return map[string]any{"error": fmt.Sprintf("Tool execution blocked by %s: %v", interceptor.Name(), err)}
			}
		}
	}

	var result map[string]any

	if a.utcp != nil && a.utcp.IsUTCPTool(funcCall.Name) {
		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🔌 Executing UTCP Tool: %s", funcCall.Name)}
		res, err := a.utcp.ExecuteTool(ctx, funcCall.Name, funcCall.Arguments)
		if err != nil {
			result = map[string]any{"error": err.Error()}
		} else {
			result = map[string]any{"result": res}
		}
	} else {
		bt := findBuiltinTool(funcCall.Name)
		if bt == nil {
			result = map[string]any{"error": "unknown function"}
		} else {
			var err error
			result, err = bt.Execute(ctx, funcCall.Arguments, a, outChan)
			if err != nil {
				result = map[string]any{"error": err.Error()}
			}
		}
	}

	outChan <- AgentEvent{
		Type:       "tool_result",
		Content:    fmt.Sprintf("✅ Tool %s finished", funcCall.Name),
		ToolName:   funcCall.Name,
		ToolResult: result,
	}

	a.emitLog(outChan, "Tool %s result: %v", funcCall.Name, result)

	return result
}
