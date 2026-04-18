package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"mairu/internal/llm"
)

type EditBlock struct {
	StartLine uint32 // 1-indexed
	EndLine   uint32 // 1-indexed
	Content   string
}

// MultiEdit safely applies multiple block replacements to a file.
func (a *Agent) MultiEdit(filePath string, edits []EditBlock) (string, error) {
	fullPath := fmt.Sprintf("%s/%s", a.root, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")

	// Sort edits in reverse order so replacing lines doesn't offset subsequent edits
	for i := 0; i < len(edits); i++ {
		for j := i + 1; j < len(edits); j++ {
			if edits[i].StartLine < edits[j].StartLine {
				edits[i], edits[j] = edits[j], edits[i]
			}
		}
	}

	for _, edit := range edits {
		startIdx := int(edit.StartLine) - 1
		endIdx := int(edit.EndLine) // EndLine is inclusive

		if startIdx < 0 || endIdx > len(lines) || startIdx >= endIdx {
			return "", fmt.Errorf("invalid edit block: %d-%d", edit.StartLine, edit.EndLine)
		}

		newLines := strings.Split(edit.Content, "\n")

		// Replace the slice
		before := lines[:startIdx]
		after := lines[endIdx:]

		var updated []string
		updated = append(updated, before...)
		updated = append(updated, newLines...)
		updated = append(updated, after...)

		lines = updated
	}

	newContent := strings.Join(lines, "\n")

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(content)),
		B:        difflib.SplitLines(newContent),
		FromFile: filePath + " (old)",
		ToFile:   filePath + " (new)",
		Context:  3,
	}
	diffStr, _ := difflib.GetUnifiedDiffString(diff)

	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return "", err
	}
	return diffStr, nil
}

// ReplaceBlock safely replaces an exact string block in a file.
func (a *Agent) ReplaceBlock(filePath string, oldString, newString string) (string, error) {
	fullPath := filepath.Join(a.root, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	contentStr := string(content)

	// Aider-style precise match
	if !strings.Contains(contentStr, oldString) {
		return "", fmt.Errorf("could not find exact old_code block in %s; please read the file again and ensure the old_code matches perfectly including whitespace", filePath)
	}

	// Check for multiple matches
	if strings.Count(contentStr, oldString) > 1 {
		return "", fmt.Errorf("found multiple matches for old_code in %s; please include more context lines in old_code to make it uniquely identifiable", filePath)
	}

	newContent := strings.Replace(contentStr, oldString, newString, 1)

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(content)),
		B:        difflib.SplitLines(newContent),
		FromFile: filePath + " (old)",
		ToFile:   filePath + " (new)",
		Context:  3,
	}
	diffStr, _ := difflib.GetUnifiedDiffString(diff)

	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return "", err
	}
	return diffStr, nil
}

type replaceBlockTool struct{}

func (t *replaceBlockTool) Definition() llm.Tool {
	return llm.Tool{
		Name:        "replace_block",
		Description: "Safely apply a Search-and-Replace block edit to a file. You must provide the EXACT existing code block you want to replace, including all whitespace. This is much safer and more reliable than multi_edit.",
		Parameters: &llm.JSONSchema{
			Type: llm.TypeObject,
			Properties: map[string]*llm.JSONSchema{
				"file_path": {Type: llm.TypeString, Description: "The relative path to the file."},
				"old_code":  {Type: llm.TypeString, Description: "The exact existing code block to be replaced. Must match exactly, including indentation."},
				"new_code":  {Type: llm.TypeString, Description: "The new code block to insert in its place."},
			},
			Required: []string{"file_path", "old_code", "new_code"},
		},
	}
}

func (t *replaceBlockTool) Execute(ctx context.Context, args map[string]any, a *Agent, outChan chan<- AgentEvent) (map[string]any, error) {
	filePath, _ := args["file_path"].(string)
	oldCode, _ := args["old_code"].(string)
	newCode, _ := args["new_code"].(string)

	diffStr, err := a.ReplaceBlock(filePath, oldCode, newCode)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("✏️ Edited %s", filePath)}
	if diffStr != "" {
		outChan <- AgentEvent{Type: "diff", Content: fmt.Sprintf("```diff\n%s\n```", diffStr)}
	}

	verifOut, verifErr := a.runAutoVerification(ctx, filePath, outChan)
	if verifErr != nil {
		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("⚠️ Auto-verification failed for %s", filePath)}
		return map[string]any{
			"status":  "edit applied but auto-verification failed",
			"error":   verifErr.Error(),
			"output":  verifOut,
			"message": "The edit was applied, but the project failed to build/lint. Please review the output and fix the errors immediately.",
		}, nil
	}
	return map[string]any{"status": "success", "verification": "passed"}, nil
}

type multiEditTool struct{}

func (t *multiEditTool) Definition() llm.Tool {
	return llm.Tool{
		Name:        "multi_edit",
		Description: "Apply a block replacement to a specific file.",
		Parameters: &llm.JSONSchema{
			Type: llm.TypeObject,
			Properties: map[string]*llm.JSONSchema{
				"file_path":  {Type: llm.TypeString, Description: "The relative path to the file."},
				"start_line": {Type: llm.TypeInteger, Description: "The 1-indexed starting line to replace."},
				"end_line":   {Type: llm.TypeInteger, Description: "The 1-indexed ending line to replace."},
				"content":    {Type: llm.TypeString, Description: "The new content to insert in place of those lines."},
			},
			Required: []string{"file_path", "start_line", "end_line", "content"},
		},
	}
}

func (t *multiEditTool) Execute(ctx context.Context, args map[string]any, a *Agent, outChan chan<- AgentEvent) (map[string]any, error) {
	filePath, _ := args["file_path"].(string)
	startLineFloat, _ := args["start_line"].(float64)
	endLineFloat, _ := args["end_line"].(float64)
	content, _ := args["content"].(string)

	diffStr, err := a.MultiEdit(filePath, []EditBlock{{
		StartLine: uint32(startLineFloat),
		EndLine:   uint32(endLineFloat),
		Content:   content,
	}})
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("✏️ Edited %s (%d-%d)", filePath, uint32(startLineFloat), uint32(endLineFloat))}
	if diffStr != "" {
		outChan <- AgentEvent{Type: "diff", Content: fmt.Sprintf("```diff\n%s\n```", diffStr)}
	}

	verifOut, verifErr := a.runAutoVerification(ctx, filePath, outChan)
	if verifErr != nil {
		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("⚠️ Auto-verification failed for %s", filePath)}
		return map[string]any{
			"status":  "edit applied but auto-verification failed",
			"error":   verifErr.Error(),
			"output":  verifOut,
			"message": "The edit was applied, but the project failed to build/lint. Please review the output and fix the errors immediately.",
		}, nil
	}
	return map[string]any{"status": "success", "verification": "passed"}, nil
}
