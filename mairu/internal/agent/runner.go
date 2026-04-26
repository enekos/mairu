package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"mairu/internal/llm"
)

// ErrStuckLoop is returned when the agent is stopped by the stuck detector.
var ErrStuckLoop = errors.New("agent stuck in a loop")

// Runner encapsulates the ReAct loop execution for a single agent run.
type Runner struct {
	agent *Agent
}

// RunResult summarizes the outcome of a runner execution.
type RunResult struct {
	Success bool
	Error   error
}

// NewRunner creates a new runner bound to an agent.
func NewRunner(agent *Agent) *Runner {
	return &Runner{agent: agent}
}

// Run executes the streaming ReAct loop for the given prompt.
func (r *Runner) Run(ctx context.Context, prompt string, outChan chan<- AgentEvent) RunResult {
	r.agent.emitLog(outChan, "Runner started")

	const maxStreamAttempts = 2
	for attempt := 1; attempt <= maxStreamAttempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 8*time.Minute)

		r.agent.mu.Lock()
		r.agent.cancel = cancel
		r.agent.mu.Unlock()

		// Just-in-time compaction. If a long mid-turn tool chain has pushed
		// history past threshold since RunStream's initial check, fold it now
		// to keep the next request inside the cache window.
		r.agent.maybeAutoCompact(outChan)

		iter, err := r.agent.llm.ChatStream(attemptCtx, prompt)
		if err != nil {
			r.agent.mu.Lock()
			r.agent.cancel = nil
			r.agent.mu.Unlock()
			cancel()
			outChan <- AgentEvent{Type: "error", Content: fmt.Sprintf("Failed to start chat stream: %v", err)}
			return RunResult{Success: false, Error: err}
		}
		r.agent.emitLog(outChan, "LLM ChatStream established (attempt %d/%d)", attempt, maxStreamAttempts)
		err = r.handleIterator(attemptCtx, iter, outChan)

		r.agent.mu.Lock()
		r.agent.cancel = nil
		r.agent.mu.Unlock()
		cancel()

		if err == nil || errors.Is(err, context.Canceled) {
			if errors.Is(err, context.Canceled) {
				outChan <- AgentEvent{Type: "status", Content: "Interrupted by user"}
				outChan <- AgentEvent{Type: "error", Content: "Interrupted"}
				return RunResult{Success: false, Error: err}
			}
			return RunResult{Success: true}
		}

		if isRetryableStreamErr(err) && attempt < maxStreamAttempts {
			outChan <- AgentEvent{Type: "status", Content: "⚠️ Stream interrupted, retrying once..."}
			r.agent.emitLog(outChan, "Retrying stream after transient error: %v", err)
			continue
		}

		outChan <- AgentEvent{Type: "error", Content: err.Error()}
		return RunResult{Success: false, Error: err}
	}

	return RunResult{Success: true}
}

func (r *Runner) handleIterator(ctx context.Context, iter llm.ChatStreamIterator, outChan chan<- AgentEvent) error {
	var toolCalls []llm.ToolCall

	for {
		chunk, err := iter.Next()
		if err != nil {
			if err.Error() == "EOF" || err.Error() == "stream done" {
				break
			}
			return err
		}

		if chunk.Content != "" {
			outChan <- AgentEvent{Type: "text", Content: chunk.Content}
		}

		if len(chunk.ToolCalls) > 0 {
			toolCalls = append(toolCalls, chunk.ToolCalls...)
		}

		if chunk.FinishReason == "stop" || chunk.FinishReason == "length" {
			break
		}
	}

	if len(toolCalls) > 0 {
		var wg sync.WaitGroup
		results := make([]llm.FunctionResponsePayload, len(toolCalls))

		for i, tc := range toolCalls {
			wg.Add(1)
			go func(idx int, call llm.ToolCall) {
				defer wg.Done()
				r.agent.emitLog(outChan, "Executing tool call: %s", call.Name)
				res := r.agent.executeToolCall(ctx, call, outChan)
				results[idx] = llm.FunctionResponsePayload{
					Name:       call.Name,
					ToolCallID: call.ID,
					Response:   res,
				}
				r.agent.emitLog(outChan, "Tool call %s completed", call.Name)
			}(i, tc)
		}
		wg.Wait()

		for _, tc := range toolCalls {
			r.agent.stuckDetector.Record(NewToolSignature(tc.Name, tc.Arguments))
		}

		switch verdict := r.agent.stuckDetector.Check(); verdict {
		case VerdictStop:
			r.agent.emitLog(outChan, "StuckDetector: stop verdict - terminating loop")
			outChan <- AgentEvent{Type: "error", Content: StopMessage()}
			outChan <- AgentEvent{Type: "done"}
			return ErrStuckLoop
		case VerdictNudge:
			r.agent.emitLog(outChan, "StuckDetector: nudge verdict - injecting warning")
			outChan <- AgentEvent{Type: "status", Content: "⚠️ Loop detected - nudging agent to try a different approach"}
			lastIdx := len(results) - 1
			if results[lastIdx].Response == nil {
				results[lastIdx].Response = make(map[string]any)
			}
			results[lastIdx].Response["_loop_warning"] = NudgeMessage()
		}

		// Same JIT compaction guard between tool-result rounds.
		r.agent.maybeAutoCompact(outChan)

		nextIter := r.agent.llm.SendFunctionResponsesStream(ctx, results)
		return r.handleIterator(ctx, nextIter, outChan)
	}

	outChan <- AgentEvent{Type: "done"}
	return nil
}

func isRetryableStreamErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "hangup") ||
		strings.Contains(lower, "sighup") ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "broken pipe") ||
		strings.Contains(lower, "stream removed") ||
		strings.Contains(lower, "eof")
}
