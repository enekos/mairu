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

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"mairu/internal/contextsrv"
	"mairu/internal/crawler"
	"mairu/internal/llm"
	"mairu/internal/prompts"
)

type SymbolLocator interface {
	FindSymbol(name string) ([]contextsrv.SymbolLocation, error)
}

type HistoryLogger interface {
	InsertBashHistory(ctx context.Context, project string, command string, exitCode int, durationMs int, output string) error
}

type Agent struct {
	llm        *llm.GeminiProvider
	locator    SymbolLocator
	root       string
	currentDir string
	apiKey     string

	stuckDetector *StuckDetector
	utcp          *UTCPManager
	utcpProviders []string

	Unattended bool
	council    CouncilConfig

	historyLogger   HistoryLogger
	interceptors    []ToolInterceptor
	AgentSystemData map[string]any

	mu           sync.Mutex
	cancel       context.CancelFunc
	approvalChan chan bool
}

type Config struct {
	SymbolLocator   SymbolLocator
	Unattended      bool
	Council         CouncilConfig
	HistoryLogger   HistoryLogger
	Interceptors    []ToolInterceptor
	UTCPProviders   []string
	AgentSystemData map[string]any
}

func normalizeConfig(cfg ...Config) Config {
	if len(cfg) == 0 {
		return Config{}
	}
	resolved := cfg[0]
	resolved.Council.Roles = append([]CouncilRole(nil), resolved.Council.Roles...)
	resolved.Interceptors = append([]ToolInterceptor(nil), resolved.Interceptors...)
	resolved.UTCPProviders = append([]string(nil), resolved.UTCPProviders...)
	if resolved.AgentSystemData != nil {
		cloned := make(map[string]any, len(resolved.AgentSystemData))
		for k, v := range resolved.AgentSystemData {
			cloned[k] = v
		}
		resolved.AgentSystemData = cloned
	}
	return resolved
}

func New(projectRoot string, apiKey string, cfg ...Config) (*Agent, error) {
	resolved := normalizeConfig(cfg...)

	llmProvider, err := llm.NewGeminiProvider(context.Background(), apiKey)
	if err != nil {
		return nil, err
	}

	utcpManager, err := NewUTCPManager(resolved.UTCPProviders)
	if err != nil {
		return nil, fmt.Errorf("failed to init UTCP manager: %w", err)
	}

	// Fetch dynamic tools
	if len(resolved.UTCPProviders) > 0 {
		utcpTools := utcpManager.Initialize(context.Background())
		llmProvider.RegisterDynamicTools(utcpTools)
	}

	return &Agent{
		llm:             llmProvider,
		locator:         resolved.SymbolLocator,
		root:            projectRoot,
		currentDir:      projectRoot,
		apiKey:          apiKey,
		stuckDetector:   NewStuckDetector(),
		utcp:            utcpManager,
		utcpProviders:   resolved.UTCPProviders,
		Unattended:      resolved.Unattended,
		council:         resolved.Council.withDefaults(),
		historyLogger:   resolved.HistoryLogger,
		interceptors:    resolved.Interceptors,
		AgentSystemData: resolved.AgentSystemData,
		approvalChan:    make(chan bool),
	}, nil
}

func (a *Agent) childConfig() Config {
	return normalizeConfig(Config{
		SymbolLocator:   a.locator,
		Unattended:      a.Unattended,
		Council:         a.council,
		HistoryLogger:   a.historyLogger,
		Interceptors:    a.interceptors,
		UTCPProviders:   a.utcpProviders,
		AgentSystemData: a.AgentSystemData,
	})
}

func (a *Agent) GetModelName() string {
	return a.llm.GetModelName()
}

func (a *Agent) Interrupt() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
}

func (a *Agent) ApproveAction(approved bool) {
	// Take ownership of the channel under the lock, then send outside it.
	// Sending inside the lock would deadlock if the channel were unbuffered;
	// nil-ing first prevents double-sends from concurrent callers.
	a.mu.Lock()
	ch := a.approvalChan
	a.approvalChan = nil
	a.mu.Unlock()
	if ch != nil {
		ch <- approved
	}
}

func (a *Agent) GetRoot() string {
	return a.root
}

func (a *Agent) SetModel(modelName string) {
	a.llm.SetModel(modelName)
}

func (a *Agent) SetCouncilEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.council.Enabled = enabled
}

func (a *Agent) IsCouncilEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.council.Enabled
}

type AgentEvent struct {
	Type       string         `json:"Type"` // "text", "status", "error", "done", "tool_call", "tool_result", "log", "bash_output"
	Content    string         `json:"Content"`
	ToolName   string         `json:"ToolName,omitempty"`
	ToolArgs   map[string]any `json:"ToolArgs,omitempty"`
	ToolResult map[string]any `json:"ToolResult,omitempty"`
}

func (a *Agent) emitLog(outChan chan<- AgentEvent, format string, args ...any) {
	outChan <- AgentEvent{Type: "log", Content: fmt.Sprintf(format, args...)}
}

func (a *Agent) loadContextFiles() string {
	var contextFiles string
	filesToTry := []string{"SYSTEM.md", "AGENTS.md", "CLAUDE.md"}

	contextFiles += "\n\n# Project Context\n\nProject-specific instructions and guidelines:\n\n"

	foundAny := false
	for _, file := range filesToTry {
		// Try root
		content, err := os.ReadFile(filepath.Join(a.GetRoot(), file))
		if err == nil {
			contextFiles += "## " + file + "\n\n" + string(content) + "\n\n"
			foundAny = true
		} else {
			// Try cwd
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

		iter := a.llm.ChatStream(ctx, input)
		a.emitLog(outChan, "LLM ChatStream established (attempt %d/%d)", attempt, maxStreamAttempts)
		err := a.handleIterator(ctx, iter, outChan)

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

func (a *Agent) handleIterator(ctx context.Context, iter *genai.GenerateContentResponseIterator, outChan chan<- AgentEvent) error {
	var functionCalls []genai.FunctionCall
	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		if len(resp.Candidates) == 0 {
			continue
		}

		if resp.UsageMetadata != nil {
			a.emitLog(outChan, "Token Usage: prompt=%d, candidates=%d, total=%d", resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount, resp.UsageMetadata.TotalTokenCount)
		}

		for _, part := range resp.Candidates[0].Content.Parts {
			if funcCall, ok := part.(genai.FunctionCall); ok {
				functionCalls = append(functionCalls, funcCall)
			}
			if text, ok := part.(genai.Text); ok {
				outChan <- AgentEvent{Type: "text", Content: string(text)}
			}
			if execCode, ok := part.(genai.ExecutableCode); ok {
				langStr := ""
				if execCode.Language == genai.ExecutableCodePython {
					langStr = "python"
				}
				outChan <- AgentEvent{
					Type:    "text",
					Content: fmt.Sprintf("\n\n```%s\n%s\n```\n", langStr, execCode.Code),
				}
			}
			if execResult, ok := part.(genai.CodeExecutionResult); ok {
				outcomeStr := "OK"
				if execResult.Outcome != genai.CodeExecutionResultOutcomeOK {
					outcomeStr = execResult.Outcome.String()
				}
				outChan <- AgentEvent{
					Type:    "text",
					Content: fmt.Sprintf("\n> Execution Outcome: %s\n> Output:\n```\n%s\n```\n", outcomeStr, execResult.Output),
				}
			}
		}
	}

	if len(functionCalls) > 0 {
		var wg sync.WaitGroup
		results := make([]llm.FunctionResponsePayload, len(functionCalls))

		for i, funcCall := range functionCalls {
			wg.Add(1)
			go func(idx int, fc genai.FunctionCall) {
				defer wg.Done()
				a.emitLog(outChan, "Executing tool call: %s", fc.Name)
				res := a.executeToolCall(ctx, fc, outChan)
				results[idx] = llm.FunctionResponsePayload{
					Name:     fc.Name,
					Response: res,
				}
				a.emitLog(outChan, "Tool call %s completed", fc.Name)
			}(i, funcCall)
		}
		wg.Wait()

		// --- Stuck detection ---
		for _, fc := range functionCalls {
			a.stuckDetector.Record(NewToolSignature(fc.Name, fc.Args))
		}

		switch verdict := a.stuckDetector.Check(); verdict {
		case VerdictStop:
			a.emitLog(outChan, "StuckDetector: stop verdict — terminating loop")
			outChan <- AgentEvent{Type: "error", Content: StopMessage()}
			outChan <- AgentEvent{Type: "done"}
			return nil
		case VerdictNudge:
			a.emitLog(outChan, "StuckDetector: nudge verdict — injecting warning")
			outChan <- AgentEvent{Type: "status", Content: "⚠️ Loop detected — nudging agent to try a different approach"}
			// Inject warning into the last tool result so the LLM sees it
			// alongside the function responses.
			lastIdx := len(results) - 1
			if results[lastIdx].Response == nil {
				results[lastIdx].Response = make(map[string]any)
			}
			results[lastIdx].Response["_loop_warning"] = NudgeMessage()
		}
		// --- End stuck detection ---

		nextIter := a.llm.SendFunctionResponsesStream(ctx, results)
		return a.handleIterator(ctx, nextIter, outChan)
	}

	outChan <- AgentEvent{Type: "done"}
	return nil
}

func (a *Agent) searchBySymbolName(symName string, outChan chan<- AgentEvent) map[string]any {
	if a.locator == nil {
		return map[string]any{"error": "SymbolLocator is not configured for this agent"}
	}
	locations, err := a.locator.FindSymbol(symName)
	if err != nil || len(locations) == 0 {
		return map[string]any{"error": fmt.Sprintf("symbol '%s' not found", symName)}
	}

	content, err := a.SurgicalRead(locations[0])
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	outChan <- AgentEvent{
		Type:    "status",
		Content: fmt.Sprintf("📄 Read %d lines from %s", locations[0].EndRow-locations[0].StartRow+1, locations[0].FilePath),
	}
	return map[string]any{"content": content}
}

func (a *Agent) executeToolCall(ctx context.Context, funcCall genai.FunctionCall, outChan chan<- AgentEvent) map[string]any {
	a.emitLog(outChan, "Tool args for %s: %v", funcCall.Name, funcCall.Args)
	outChan <- AgentEvent{
		Type:     "tool_call",
		Content:  fmt.Sprintf("🔧 Tool Call: %s", funcCall.Name),
		ToolName: funcCall.Name,
		ToolArgs: funcCall.Args,
	}
	outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🔧 Tool Call: %s", funcCall.Name)}

	// Pre-execute hook evaluation
	toolCtx := ToolContext{
		Context: ctx,
		Agent:   a,
		OutChan: outChan,
	}
	for _, interceptor := range a.interceptors {
		var err error
		funcCall.Args, err = interceptor.PreExecute(toolCtx, funcCall.Name, funcCall.Args)
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
		res, err := a.utcp.ExecuteTool(ctx, funcCall.Name, funcCall.Args)
		if err != nil {
			result = map[string]any{"error": err.Error()}
		} else {
			result = map[string]any{"result": res}
		}
	} else {
		switch funcCall.Name {
		case "review_work":
			summary, _ := funcCall.Args["summary"].(string)
			critique, _ := funcCall.Args["critique"].(string)
			outChan <- AgentEvent{Type: "status", Content: "🧠 Reviewing work and self-critiquing..."}
			result = map[string]any{"status": "review acknowledged. Proceed to finish or fix issues.", "summary": summary, "critique": critique}

		case "replace_block":
			filePath, _ := funcCall.Args["file_path"].(string)
			oldCode, _ := funcCall.Args["old_code"].(string)
			newCode, _ := funcCall.Args["new_code"].(string)

			diffStr, err := a.ReplaceBlock(filePath, oldCode, newCode)

			if err != nil {
				result = map[string]any{"error": err.Error()}
			} else {
				outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("✏️ Edited %s", filePath)}
				if diffStr != "" {
					outChan <- AgentEvent{Type: "diff", Content: fmt.Sprintf("```diff\n%s\n```", diffStr)}
				}

				// Auto-Verification loop hook
				verifOut, verifErr := a.runAutoVerification(ctx, filePath, outChan)
				if verifErr != nil {
					result = map[string]any{
						"status":  "edit applied but auto-verification failed",
						"error":   verifErr.Error(),
						"output":  verifOut,
						"message": "The edit was applied, but the project failed to build/lint. Please review the output and fix the errors immediately.",
					}
					outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("⚠️ Auto-verification failed for %s", filePath)}
				} else {
					result = map[string]any{"status": "success", "verification": "passed"}
				}
			}

		case "multi_edit":
			filePath, _ := funcCall.Args["file_path"].(string)
			startLineFloat, _ := funcCall.Args["start_line"].(float64)
			endLineFloat, _ := funcCall.Args["end_line"].(float64)
			content, _ := funcCall.Args["content"].(string)

			diffStr, err := a.MultiEdit(filePath, []EditBlock{{
				StartLine: uint32(startLineFloat),
				EndLine:   uint32(endLineFloat),
				Content:   content,
			}})

			if err != nil {
				result = map[string]any{"error": err.Error()}
			} else {
				outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("✏️ Edited %s (%d-%d)", filePath, uint32(startLineFloat), uint32(endLineFloat))}
				if diffStr != "" {
					outChan <- AgentEvent{Type: "diff", Content: fmt.Sprintf("```diff\n%s\n```", diffStr)}
				}
				result = map[string]any{"status": "success"}
			}

		case "bash":
			command, _ := funcCall.Args["command"].(string)
			timeoutMsFloat, ok := funcCall.Args["timeout_ms"].(float64)
			var timeout int
			if ok {
				timeout = int(timeoutMsFloat)
			}

			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🖥️ Running bash: %s", command)}

			start := time.Now()
			out, err := a.RunBash(ctx, command, timeout, outChan)
			duration := int(time.Since(start).Milliseconds())

			exitCode := 0
			if err != nil {
				result = map[string]any{"error": err.Error(), "output": out}
				exitCode = 1 // Simplified for now
			} else {
				result = map[string]any{"output": out}
				if strings.HasPrefix(out, "STDOUT:\ndiff ") || strings.Contains(out, "\n--- ") && strings.Contains(out, "\n+++ ") {
					outChan <- AgentEvent{Type: "diff", Content: fmt.Sprintf("```diff\n%s\n```", out)}
				}
			}

			if a.historyLogger != nil {
				go func() {
					// Don't block the agent loop
					_ = a.historyLogger.InsertBashHistory(context.Background(), a.root, command, exitCode, duration, out)
				}()
			}

		case "read_file":
			filePath, _ := funcCall.Args["file_path"].(string)
			offsetFloat, _ := funcCall.Args["offset"].(float64)
			limitFloat, _ := funcCall.Args["limit"].(float64)

			offset := int(offsetFloat)
			if offset <= 0 {
				offset = 1
			}
			limit := int(limitFloat)
			if limit <= 0 {
				limit = 2000
			}

			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("📄 Reading file: %s (offset: %d, limit: %d)", filePath, offset, limit)}
			content, err := a.ReadFile(filePath, offset, limit)
			if err != nil {
				result = map[string]any{"error": err.Error()}
			} else {
				result = map[string]any{"content": content}
			}

		case "write_file":
			filePath, _ := funcCall.Args["file_path"].(string)
			content, _ := funcCall.Args["content"].(string)

			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("💾 Writing file: %s", filePath)}
			diffStr, err := a.WriteFile(filePath, content)
			if err != nil {
				result = map[string]any{"error": err.Error()}
			} else {
				if diffStr != "" {
					outChan <- AgentEvent{Type: "diff", Content: fmt.Sprintf("```diff\n%s\n```", diffStr)}
				}

				// Auto-Verification loop hook
				verifOut, verifErr := a.runAutoVerification(ctx, filePath, outChan)
				if verifErr != nil {
					result = map[string]any{
						"status":  "file written but auto-verification failed",
						"error":   verifErr.Error(),
						"output":  verifOut,
						"message": "The file was written, but the project failed to build/lint. Please review the output and fix the errors immediately.",
					}
					outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("⚠️ Auto-verification failed for %s", filePath)}
				} else {
					result = map[string]any{"status": "success", "verification": "passed"}
				}
			}

		case "find_files":
			pattern, _ := funcCall.Args["pattern"].(string)

			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🔍 Finding files: %s", pattern)}
			files, err := a.FindFiles(pattern)
			if err != nil {
				result = map[string]any{"error": err.Error()}
			} else {
				result = map[string]any{"files": files}
			}

		case "search_codebase":
			query, _ := funcCall.Args["query"].(string)
			symName, _ := funcCall.Args["symbol_name"].(string)

			if strings.TrimSpace(symName) != "" {
				outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🔎 Searching symbol: %s", symName)}
				result = a.searchBySymbolName(symName, outChan)
				break
			}

			if strings.TrimSpace(query) == "" {
				result = map[string]any{"error": "missing query: provide either query or symbol_name"}
				break
			}

			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🔎 Searching for: %s", query)}
			matches, err := a.SearchCodebase(query)
			if err != nil {
				result = map[string]any{"error": err.Error()}
			} else {
				result = map[string]any{"matches": matches}
			}

		case "delegate_task":
			task, _ := funcCall.Args["task_description"].(string)
			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🤖 Delegating task: %s", task)}

			subAgent, err := New(a.root, a.apiKey, a.childConfig())
			if err != nil {
				result = map[string]any{"error": "failed to spawn sub-agent: " + err.Error()}
			} else {
				// Prepend main conversation context
				contextStr := a.GetRecentContext()

				fullTask, err := prompts.GetForProject("delegate_task", struct {
					Context string
					Task    string
				}{
					Context: contextStr,
					Task:    task,
				}, a.root)
				if err != nil {
					result = map[string]any{"error": "failed to render delegate prompt: " + err.Error()}
					break
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

				result = map[string]any{"result": subResult}
				subAgent.Close()
			}

		case "fetch_url":
			urlToFetch, _ := funcCall.Args["url"].(string)
			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🌐 Fetching URL: %s", urlToFetch)}

			content, err := a.FetchURL(urlToFetch)
			if err != nil {
				result = map[string]any{"error": err.Error()}
			} else {
				result = map[string]any{"content": content}
			}

		case "scrape_url":
			urlToScrape, _ := funcCall.Args["url"].(string)
			prompt, _ := funcCall.Args["prompt"].(string)
			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🕸️ Scraping URL: %s", urlToScrape)}

			graph := crawler.NewSmartScraperGraph(a.llm)
			data, err := graph.Run(ctx, urlToScrape, prompt)
			if err != nil {
				result = map[string]any{"error": err.Error()}
			} else {
				result = map[string]any{"data": data}
			}

		case "search_web":
			query, _ := funcCall.Args["query"].(string)
			prompt, _ := funcCall.Args["prompt"].(string)
			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🔍 Searching Web: %s", query)}

			graph := crawler.NewSearchScraperGraph(a.llm)
			data, err := graph.Run(ctx, query, prompt, 3)
			if err != nil {
				result = map[string]any{"error": err.Error()}
			} else {
				result = map[string]any{"data": data}
			}

		case "delete_file":
			pathToDelete, _ := funcCall.Args["path"].(string)
			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🗑️ Deleting: %s", pathToDelete)}
			err := os.RemoveAll(pathToDelete)
			if err != nil {
				result = map[string]any{"error": err.Error()}
			} else {
				result = map[string]any{"status": "success"}
			}

		case "browser_context":
			command, _ := funcCall.Args["command"].(string)
			query, _ := funcCall.Args["query"].(string)
			limitF, ok := funcCall.Args["limit"].(float64)
			limit := 5
			if ok {
				limit = int(limitF)
			}
			outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🌐 Browser: %s", command)}
			resp, err := queryBrowserContext(command, query, limit)
			if err != nil {
				result = map[string]any{"error": err.Error()}
			} else {
				result = map[string]any{"response": resp}
			}

		default:
			result = map[string]any{"error": "unknown function"}
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

// runAutoVerification intelligently checks the project based on the file extension
// or presence of typical configuration files (go.mod, tsconfig.json, etc.).
func (a *Agent) runAutoVerification(ctx context.Context, editedFilePath string, outChan chan<- AgentEvent) (string, error) {
	root := a.GetRoot()

	// Default: if it's a Go file, try running `go build ./...`
	if strings.HasSuffix(editedFilePath, ".go") {
		// Verify there's a go.mod
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return a.RunBash(ctx, "go build ./...", 30000, outChan)
		}
	}

	// For TypeScript/JS, we could do bun run typecheck or npx tsc --noEmit
	if strings.HasSuffix(editedFilePath, ".ts") || strings.HasSuffix(editedFilePath, ".tsx") {
		if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); err == nil {
			// check if bun is available or package.json has a typecheck script
			content, err := os.ReadFile(filepath.Join(root, "package.json"))
			if err == nil && strings.Contains(string(content), "\"typecheck\"") {
				return a.RunBash(ctx, "npm run typecheck", 45000, outChan) // Fallback for general
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
		return false // Never retry explicit cancellations
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "hangup") ||
		strings.Contains(lower, "sighup") ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "broken pipe") ||
		strings.Contains(lower, "stream removed") ||
		strings.Contains(lower, "eof")
}
