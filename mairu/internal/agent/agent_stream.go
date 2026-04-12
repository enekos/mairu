package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mairu/internal/llm"
	"mairu/internal/prompts"
)

func (a *Agent) emitLog(outChan chan<- AgentEvent, format string, args ...any) {
	outChan <- AgentEvent{Type: "log", Content: fmt.Sprintf(format, args...)}
}

func (a *Agent) loadContextFiles() string {
	var contextFiles string
	filesToTry := []string{"SYSTEM.md", "AGENTS.md", "CLAUDE.md"}

	contextFiles += "\n\n# Project Context\n\nProject-specific instructions and guidelines:\n\n"

	foundAny := false
	for _, file := range filesToTry {
		content, err := os.ReadFile(filepath.Join(a.GetRoot(), file))
		if err == nil {
			contextFiles += "## " + file + "\n\n" + string(content) + "\n\n"
			foundAny = true
		} else {
			cwd, _ := os.Getwd()
			if cwd != a.GetRoot() {
				content, err := os.ReadFile(filepath.Join(cwd, file))
				if err == nil {
					contextFiles += "## " + file + " (local)\n\n" + string(content) + "\n\n"
					foundAny = true
				}
			}
		}
	}

	if !foundAny {
		return ""
	}
	return contextFiles
}

func (a *Agent) RunStream(prompt string, outChan chan<- AgentEvent) {
	defer close(outChan)

	a.emitLog(outChan, "Agent RunStream started")
	if err := a.CompactContext(); err != nil {
		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("⚠️ Warning: Failed to compact context: %v", err)}
		a.emitLog(outChan, "Failed to compact context: %v", err)
	}
	a.stuckDetector.Reset()

	fullPrompt := prompt
	if a.IsCouncilEnabled() {
		synthesized, err := a.runCouncil(outChan, prompt)
		if err != nil {
			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("⚠️ Council fallback: %v", err)}
		} else {
			fullPrompt = synthesized
		}
	}
	if a.llm.IsNewSession() {
		systemPrompt, err := prompts.GetForProject("agent_system", a.AgentSystemData, a.root)
		if err != nil {
			outChan <- AgentEvent{Type: "error", Content: fmt.Sprintf("failed to render agent_system prompt: %v", err)}
			return
		}

		contextFiles := a.loadContextFiles()
		if contextFiles != "" {
			systemPrompt += contextFiles
		}

		systemPrompt += fmt.Sprintf("\n\nCurrent working directory: %s", a.currentDir)

		a.llm.SetSystemInstruction(systemPrompt)
		a.emitLog(outChan, "Initialized new session with context length: %d chars", len(systemPrompt))
	}

	a.emitLog(outChan, "Sending prompt to LLM (length: %d)", len(fullPrompt))
	a.processLoopStream(fullPrompt, outChan)
}

func (a *Agent) processLoopStream(input string, outChan chan<- AgentEvent) {
	const maxStreamAttempts = 2
	for attempt := 1; attempt <= maxStreamAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)

		a.mu.Lock()
		a.cancel = cancel
		a.mu.Unlock()

		iter, err := a.llm.ChatStream(ctx, input)
		if err != nil {
			a.mu.Lock()
			a.cancel = nil
			a.mu.Unlock()
			cancel()
			outChan <- AgentEvent{Type: "error", Content: fmt.Sprintf("Failed to start chat stream: %v", err)}
			return
		}
		a.emitLog(outChan, "LLM ChatStream established (attempt %d/%d)", attempt, maxStreamAttempts)
		err = a.handleIterator(ctx, iter, outChan)

		a.mu.Lock()
		a.cancel = nil
		a.mu.Unlock()
		cancel()

		if err == nil || errors.Is(err, context.Canceled) {
			if errors.Is(err, context.Canceled) {
				outChan <- AgentEvent{Type: "status", Content: "Interrupted by user"}
				outChan <- AgentEvent{Type: "error", Content: "Interrupted"}
			}
			return
		}

		if isRetryableStreamErr(err) && attempt < maxStreamAttempts {
			outChan <- AgentEvent{Type: "status", Content: "⚠️ Stream interrupted, retrying once..."}
			a.emitLog(outChan, "Retrying stream after transient error: %v", err)
			continue
		}

		outChan <- AgentEvent{Type: "error", Content: err.Error()}
		return
	}
}

func (a *Agent) handleIterator(ctx context.Context, iter llm.ChatStreamIterator, outChan chan<- AgentEvent) error {
	var toolCalls []llm.ToolCall

	for {
		chunk, err := iter.Next()
		if err != nil {
			if err.Error() == "EOF" || err.Error() == "stream done" {
				break
			}
			return err
		}

		// Handle content
		if chunk.Content != "" {
			outChan <- AgentEvent{Type: "text", Content: chunk.Content}
		}

		// Accumulate tool calls
		if len(chunk.ToolCalls) > 0 {
			toolCalls = append(toolCalls, chunk.ToolCalls...)
		}

		// Check if stream is done
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
				a.emitLog(outChan, "Executing tool call: %s", call.Name)
				res := a.executeToolCall(ctx, call, outChan)
				results[idx] = llm.FunctionResponsePayload{
					Name:     call.Name,
					Response: res,
				}
				a.emitLog(outChan, "Tool call %s completed", call.Name)
			}(i, tc)
		}
		wg.Wait()

		for _, tc := range toolCalls {
			a.stuckDetector.Record(NewToolSignature(tc.Name, tc.Arguments))
		}

		switch verdict := a.stuckDetector.Check(); verdict {
		case VerdictStop:
			a.emitLog(outChan, "StuckDetector: stop verdict - terminating loop")
			outChan <- AgentEvent{Type: "error", Content: StopMessage()}
			outChan <- AgentEvent{Type: "done"}
			return nil
		case VerdictNudge:
			a.emitLog(outChan, "StuckDetector: nudge verdict - injecting warning")
			outChan <- AgentEvent{Type: "status", Content: "⚠️ Loop detected - nudging agent to try a different approach"}
			lastIdx := len(results) - 1
			if results[lastIdx].Response == nil {
				results[lastIdx].Response = make(map[string]any)
			}
			results[lastIdx].Response["_loop_warning"] = NudgeMessage()
		}

		nextIter := a.llm.SendFunctionResponsesStream(ctx, results)
		return a.handleIterator(ctx, nextIter, outChan)
	}

	outChan <- AgentEvent{Type: "done"}
	return nil
}

func (a *Agent) Run(prompt string) (string, error) {
	outChan := make(chan AgentEvent, 100)
	go a.RunStream(prompt, outChan)

	var result string
	var err error
	for ev := range outChan {
		if ev.Type == "text" {
			result += ev.Content
		} else if ev.Type == "error" {
			err = fmt.Errorf("%s", ev.Content)
		} else if ev.Type == "status" {
			fmt.Println(ev.Content)
		}
	}
	return result, err
}

func (a *Agent) Close() {
	a.llm.Close()
}

func (a *Agent) runAutoVerification(ctx context.Context, editedFilePath string, outChan chan<- AgentEvent) (string, error) {
	root := a.GetRoot()

	if strings.HasSuffix(editedFilePath, ".go") {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return a.RunBash(ctx, "go build ./...", 30000, outChan)
		}
	}

	if strings.HasSuffix(editedFilePath, ".ts") || strings.HasSuffix(editedFilePath, ".tsx") {
		if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); err == nil {
			content, err := os.ReadFile(filepath.Join(root, "package.json"))
			if err == nil && strings.Contains(string(content), "\"typecheck\"") {
				return a.RunBash(ctx, "npm run typecheck", 45000, outChan)
			}
			return a.RunBash(ctx, "npx tsc --noEmit", 45000, outChan)
		}
	}

	return "", nil
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
