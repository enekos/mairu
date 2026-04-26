package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// --- Planner ---
	planner := NewPlanner(a.llm)
	plan, err := planner.Plan(context.Background(), fullPrompt)
	if err != nil {
		a.emitLog(outChan, "Planner error: %v", err)
	}

	var originalTools []llm.Tool
	if plan != nil && len(plan.ToolNames) > 0 {
		originalTools = a.llm.GetTools()
		a.llm.SetTools(filterTools(originalTools, plan.ToolNames))
		a.emitLog(outChan, "Planner selected %d tools for this run", len(plan.ToolNames))
	}

	// --- Council (only for complex prompts) ---
	isComplex := plan != nil
	if a.IsCouncilEnabled() && isComplex {
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

		// Volatile fields (date, cwd) go LAST so the cacheable prefix above
		// stays stable across runs. Pi-mono does the same — see system-prompt.ts.
		systemPrompt += fmt.Sprintf("\n\nToday's date: %s\nCurrent working directory: %s",
			time.Now().UTC().Format("2006-01-02"), a.currentDir)

		a.llm.SetSystemInstruction(systemPrompt)
		a.emitLog(outChan, "Initialized new session with context length: %d chars", len(systemPrompt))
	}

	// --- Ephemeral snapshot ---
	historySnapshot := a.llm.GetHistory()

	a.emitLog(outChan, "Sending prompt to LLM (length: %d)", len(fullPrompt))

	// --- Run ---
	runner := NewRunner(a)
	result := runner.Run(context.Background(), fullPrompt, outChan)

	// --- Restore on failure ---
	if !result.Success {
		a.llm.SetHistory(historySnapshot)
		a.emitLog(outChan, "Ephemeral run failed, restored session history")
	}

	// --- Restore full tool set ---
	if originalTools != nil {
		a.llm.SetTools(originalTools)
	}
}

func filterTools(all []llm.Tool, names []string) []llm.Tool {
	allowed := make(map[string]bool, len(names))
	for _, n := range names {
		allowed[n] = true
	}
	var filtered []llm.Tool
	for _, t := range all {
		if allowed[t.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func (a *Agent) Run(prompt string) (string, error) {
	outChan := make(chan AgentEvent, 100)
	go a.RunStream(prompt, outChan)

	var result string
	var err error
	var sawContent bool
	for ev := range outChan {
		if ev.Type == "text" {
			result += ev.Content
			sawContent = true
		} else if ev.Type == "error" {
			err = fmt.Errorf("%s", ev.Content)
		} else if ev.Type == "status" {
			fmt.Println(ev.Content)
		}
	}
	if err == nil && !sawContent {
		err = fmt.Errorf("LLM returned an empty response")
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
