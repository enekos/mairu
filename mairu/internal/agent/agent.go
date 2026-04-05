package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"mairu/internal/db"
	"mairu/internal/llm"
	"mairu/internal/prompts"
)

type Agent struct {
	llm    *llm.GeminiProvider
	db     *db.DB
	apiKey string
}

type Config struct {
	MeiliURL    string
	MeiliAPIKey string
}

func New(projectRoot string, apiKey string, cfg ...Config) (*Agent, error) {
	var dbCfg db.Config
	if len(cfg) > 0 {
		dbCfg = db.Config{
			MeiliURL:    cfg[0].MeiliURL,
			MeiliAPIKey: cfg[0].MeiliAPIKey,
		}
	}

	database, err := db.InitDB(projectRoot, dbCfg)
	if err != nil {
		return nil, err
	}

	llmProvider, err := llm.NewGeminiProvider(context.Background(), apiKey)
	if err != nil {
		return nil, err
	}

	return &Agent{
		llm:    llmProvider,
		db:     database,
		apiKey: apiKey,
	}, nil
}

func (a *Agent) GetModelName() string {
	return a.llm.GetModelName()
}

func (a *Agent) GetRoot() string {
	return a.db.Root()
}

func (a *Agent) SetModel(modelName string) {
	a.llm.SetModel(modelName)
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

	fullPrompt := prompt
	if a.llm.IsNewSession() {
		systemPrompt := prompts.Render("agent_system", nil)

		contextFiles := a.loadContextFiles()
		if contextFiles != "" {
			systemPrompt += contextFiles
		}

		cwd, _ := os.Getwd()
		systemPrompt += fmt.Sprintf("\n\nCurrent working directory: %s", cwd)

		fullPrompt = systemPrompt + "\n\nUser Request: " + prompt
		a.emitLog(outChan, "Initialized new session with context length: %d chars", len(systemPrompt))
	}

	a.emitLog(outChan, "Sending prompt to LLM (length: %d)", len(fullPrompt))
	a.processLoopStream(fullPrompt, outChan)
}

func (a *Agent) processLoopStream(input string, outChan chan<- AgentEvent) {
	iter := a.llm.ChatStream(context.Background(), input)
	a.emitLog(outChan, "LLM ChatStream established")
	a.handleIterator(iter, outChan)
}

func (a *Agent) handleIterator(iter *genai.GenerateContentResponseIterator, outChan chan<- AgentEvent) {
	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			outChan <- AgentEvent{Type: "error", Content: err.Error()}
			return
		}

		if len(resp.Candidates) == 0 {
			continue
		}

		if resp.UsageMetadata != nil {
			a.emitLog(outChan, "Token Usage: prompt=%d, candidates=%d, total=%d", resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount, resp.UsageMetadata.TotalTokenCount)
		}

		var functionCalls []genai.FunctionCall
		for _, part := range resp.Candidates[0].Content.Parts {
			if funcCall, ok := part.(genai.FunctionCall); ok {
				functionCalls = append(functionCalls, funcCall)
			}
			if text, ok := part.(genai.Text); ok {
				outChan <- AgentEvent{Type: "text", Content: string(text)}
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
					res := a.executeToolCall(fc, outChan)
					results[idx] = llm.FunctionResponsePayload{
						Name:     fc.Name,
						Response: res,
					}
					a.emitLog(outChan, "Tool call %s completed", fc.Name)
				}(i, funcCall)
			}
			wg.Wait()

			nextIter := a.llm.SendFunctionResponsesStream(context.Background(), results)
			a.handleIterator(nextIter, outChan)
			return // we recurse via handleIterator, then exit this frame
		}
	}
	outChan <- AgentEvent{Type: "done"}
}

func (a *Agent) executeToolCall(funcCall genai.FunctionCall, outChan chan<- AgentEvent) map[string]any {
	a.emitLog(outChan, "Tool args for %s: %v", funcCall.Name, funcCall.Args)
	outChan <- AgentEvent{
		Type:     "tool_call",
		Content:  fmt.Sprintf("🔧 Tool Call: %s", funcCall.Name),
		ToolName: funcCall.Name,
		ToolArgs: funcCall.Args,
	}
	outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🔧 Tool Call: %s", funcCall.Name)}

	var result map[string]any

	if funcCall.Name == "read_symbol" {
		symName, _ := funcCall.Args["symbol_name"].(string)
		locations, err := a.db.FindSymbol(symName)

		if err != nil || len(locations) == 0 {
			result = map[string]any{"error": fmt.Sprintf("symbol '%s' not found", symName)}
		} else {
			content, err := a.SurgicalRead(locations[0])
			if err != nil {
				result = map[string]any{"error": err.Error()}
			} else {
				outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("📄 Read %d lines from %s", locations[0].EndRow-locations[0].StartRow+1, locations[0].FilePath)}
				result = map[string]any{"content": content}
			}
		}
	} else if funcCall.Name == "replace_block" {
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
			verifOut, verifErr := a.runAutoVerification(filePath, outChan)
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
	} else if funcCall.Name == "multi_edit" {
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
	} else if funcCall.Name == "bash" {
		command, _ := funcCall.Args["command"].(string)
		timeoutMsFloat, ok := funcCall.Args["timeout_ms"].(float64)
		var timeout int
		if ok {
			timeout = int(timeoutMsFloat)
		}

		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🖥️ Running bash: %s", command)}
		out, err := a.RunBash(command, timeout, outChan)
		if err != nil {
			result = map[string]any{"error": err.Error(), "output": out}
		} else {
			result = map[string]any{"output": out}
			// If bash output looks like a diff, show it!
			if strings.HasPrefix(out, "STDOUT:\ndiff ") || strings.Contains(out, "\n--- ") && strings.Contains(out, "\n+++ ") {
				outChan <- AgentEvent{Type: "diff", Content: fmt.Sprintf("```diff\n%s\n```", out)}
			}
		}
	} else if funcCall.Name == "read_file" {
		filePath, _ := funcCall.Args["file_path"].(string)

		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("📄 Reading file: %s", filePath)}
		content, err := a.ReadFile(filePath)
		if err != nil {
			result = map[string]any{"error": err.Error()}
		} else {
			result = map[string]any{"content": content}
		}
	} else if funcCall.Name == "write_file" {
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
			verifOut, verifErr := a.runAutoVerification(filePath, outChan)
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
	} else if funcCall.Name == "find_files" {
		pattern, _ := funcCall.Args["pattern"].(string)

		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🔍 Finding files: %s", pattern)}
		files, err := a.FindFiles(pattern)
		if err != nil {
			result = map[string]any{"error": err.Error()}
		} else {
			result = map[string]any{"files": files}
		}
	} else if funcCall.Name == "search_codebase" {
		query, _ := funcCall.Args["query"].(string)

		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🔎 Searching for: %s", query)}
		matches, err := a.SearchCodebase(query)
		if err != nil {
			result = map[string]any{"error": err.Error()}
		} else {
			result = map[string]any{"matches": matches}
		}
	} else if funcCall.Name == "delegate_task" {
		task, _ := funcCall.Args["task_description"].(string)
		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🤖 Delegating task: %s", task)}

		subAgent, err := New(a.db.Root(), a.apiKey)
		if err != nil {
			result = map[string]any{"error": "failed to spawn sub-agent: " + err.Error()}
		} else {
			// Prepend main conversation context
			contextStr := a.GetRecentContext()

			fullTask := prompts.Render("delegate_task", struct {
				Context string
				Task    string
			}{
				Context: contextStr,
				Task:    task,
			})

			subOut := make(chan AgentEvent)
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
	} else if funcCall.Name == "fetch_url" {
		urlToFetch, _ := funcCall.Args["url"].(string)
		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🌐 Fetching URL: %s", urlToFetch)}

		content, err := a.FetchURL(urlToFetch)
		if err != nil {
			result = map[string]any{"error": err.Error()}
		} else {
			result = map[string]any{"content": content}
		}
	} else if funcCall.Name == "delete_file" {
		pathToDelete, _ := funcCall.Args["path"].(string)
		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🗑️ Deleting: %s", pathToDelete)}
		err := os.RemoveAll(pathToDelete)
		if err != nil {
			result = map[string]any{"error": err.Error()}
		} else {
			result = map[string]any{"status": "success"}
		}
	} else {
		result = map[string]any{"error": "unknown function"}
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
	outChan := make(chan AgentEvent)
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
	a.db.Close()
	a.llm.Close()
}

// runAutoVerification intelligently checks the project based on the file extension
// or presence of typical configuration files (go.mod, tsconfig.json, etc.).
func (a *Agent) runAutoVerification(editedFilePath string, outChan chan<- AgentEvent) (string, error) {
	root := a.GetRoot()

	// Default: if it's a Go file, try running `go build ./...`
	if strings.HasSuffix(editedFilePath, ".go") {
		// Verify there's a go.mod
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return a.RunBash("go build ./...", 30000, outChan)
		}
	}

	// For TypeScript/JS, we could do bun run typecheck or npx tsc --noEmit
	if strings.HasSuffix(editedFilePath, ".ts") || strings.HasSuffix(editedFilePath, ".tsx") {
		if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); err == nil {
			// check if bun is available or package.json has a typecheck script
			content, err := os.ReadFile(filepath.Join(root, "package.json"))
			if err == nil && strings.Contains(string(content), "\"typecheck\"") {
				return a.RunBash("npm run typecheck", 45000, outChan) // Fallback for general
			}
			return a.RunBash("npx tsc --noEmit", 45000, outChan)
		}
	}

	return "", nil
}
