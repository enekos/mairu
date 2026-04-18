package agent

import (
	"context"
	"fmt"

	"mairu/internal/llm"
	"mairu/internal/prompts"
)

type reviewWorkTool struct{}

func (t *reviewWorkTool) Definition() llm.Tool {
	return llm.Tool{
		Name:        "review_work",
		Description: "Before finishing a task, use this tool to review the work done against the requirements, and self-critique it for potential flaws or missed edge cases. This ensures better accuracy and reliability.",
		Parameters: &llm.JSONSchema{
			Type: llm.TypeObject,
			Properties: map[string]*llm.JSONSchema{
				"summary":  {Type: llm.TypeString, Description: "A summary of the changes made and how they resolve the task."},
				"critique": {Type: llm.TypeString, Description: "A self-critique identifying any edge cases, potential failures, or unaddressed requirements."},
			},
			Required: []string{"summary", "critique"},
		},
	}
}

func (t *reviewWorkTool) Execute(ctx context.Context, args map[string]any, a *Agent, outChan chan<- AgentEvent) (map[string]any, error) {
	summary, _ := args["summary"].(string)
	critique, _ := args["critique"].(string)
	outChan <- AgentEvent{Type: "status", Content: "🧠 Reviewing work and self-critiquing..."}
	return map[string]any{
		"status":   "review acknowledged. Proceed to finish or fix issues.",
		"summary":  summary,
		"critique": critique,
	}, nil
}

type delegateTaskTool struct{}

func (t *delegateTaskTool) Definition() llm.Tool {
	return llm.Tool{
		Name:        "delegate_task",
		Description: "Delegate a complex sub-task to another AI agent. Useful for researching or exploring while you focus on the main task.",
		Parameters: &llm.JSONSchema{
			Type: llm.TypeObject,
			Properties: map[string]*llm.JSONSchema{
				"task_description": {Type: llm.TypeString, Description: "A highly detailed prompt describing what the sub-agent should do."},
			},
			Required: []string{"task_description"},
		},
	}
}

func (t *delegateTaskTool) Execute(ctx context.Context, args map[string]any, a *Agent, outChan chan<- AgentEvent) (map[string]any, error) {
	task, _ := args["task_description"].(string)
	outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🤖 Delegating task: %s", task)}

	subAgent, err := New(a.root, a.providerCfg, a.childConfig())
	if err != nil {
		return map[string]any{"error": "failed to spawn sub-agent: " + err.Error()}, nil
	}

	contextStr := a.GetRecentContext()
	fullTask, err := prompts.GetForProject("delegate_task", struct {
		Context string
		Task    string
	}{
		Context: contextStr,
		Task:    task,
	}, a.root)
	if err != nil {
		return map[string]any{"error": "failed to render delegate prompt: " + err.Error()}, nil
	}

	subOut := make(chan AgentEvent, 100)
	go subAgent.RunStream(fullTask, subOut)

	var subResult string
	for ev := range subOut {
		if ev.Type == "text" {
			subResult += ev.Content
		} else if ev.Type == "status" || ev.Type == "tool_call" || ev.Type == "tool_result" {
			outChan <- AgentEvent{Type: "status", Content: "  ↳ " + ev.Content}
		}
	}
	subAgent.Close()
	return map[string]any{"result": subResult}, nil
}

type browserContextTool struct{}

func (t *browserContextTool) Definition() llm.Tool {
	return llm.Tool{
		Name:        "browser_context",
		Description: "Get real-time browser context from the Mairu browser extension.",
		Parameters: &llm.JSONSchema{
			Type: llm.TypeObject,
			Properties: map[string]*llm.JSONSchema{
				"command": {Type: llm.TypeString, Description: "The command to run: current, history, search, or session."},
				"query":   {Type: llm.TypeString, Description: "The search query (only for 'search' command)."},
				"limit":   {Type: llm.TypeInteger, Description: "The limit for search results (only for 'search' command)."},
			},
			Required: []string{"command"},
		},
	}
}

func (t *browserContextTool) Execute(ctx context.Context, args map[string]any, a *Agent, outChan chan<- AgentEvent) (map[string]any, error) {
	command, _ := args["command"].(string)
	query, _ := args["query"].(string)
	limitF, ok := args["limit"].(float64)
	limit := 5
	if ok {
		limit = int(limitF)
	}
	outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🌐 Browser: %s", command)}
	resp, err := queryBrowserContext(command, query, limit)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	return map[string]any{"response": resp}, nil
}
