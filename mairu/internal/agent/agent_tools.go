package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"mairu/internal/crawler"
	"mairu/internal/prompts"
)

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
				exitCode = 1
			} else {
				result = map[string]any{"output": out}
				if strings.HasPrefix(out, "STDOUT:\ndiff ") || strings.Contains(out, "\n--- ") && strings.Contains(out, "\n+++ ") {
					outChan <- AgentEvent{Type: "diff", Content: fmt.Sprintf("```diff\n%s\n```", out)}
				}
			}

			if a.historyLogger != nil {
				go func() {
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
