package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"mairu/internal/llm"
)

// ReadFile reads the content of a file, adding line numbers and supporting offset/limit.
func (a *Agent) ReadFile(filePath string, offset, limit int) (string, error) {
	fullPath := filepath.Join(a.root, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	if offset < 1 {
		offset = 1
	}
	if limit <= 0 {
		limit = 2000
	}

	startIdx := offset - 1
	if startIdx >= totalLines {
		return fmt.Sprintf("File only has %d lines. Offset %d is out of bounds.", totalLines, offset), nil
	}

	endIdx := startIdx + limit
	if endIdx > totalLines {
		endIdx = totalLines
	}

	var sb strings.Builder
	for i := startIdx; i < endIdx; i++ {
		sb.WriteString(fmt.Sprintf("%d: %s\n", i+1, lines[i]))
	}

	res := sb.String()

	// Dual-axis head truncation guards against absurdly long single lines
	// (minified JS) AND absurdly long total output.
	if tr := TruncateHead(res, limit, DefaultMaxBytes); tr.Truncated {
		res = tr.Content + FormatTruncationNote(tr, "head")
	}

	if endIdx < totalLines {
		res += fmt.Sprintf("\n...[File window. Showing lines %d to %d of %d. Use offset=%d to read more]", startIdx+1, endIdx, totalLines, endIdx+1)
	}

	return res, nil
}

// WriteFile overwrites a file completely.
func (a *Agent) WriteFile(filePath string, content string) (string, error) {
	fullPath := filepath.Join(a.root, filePath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", err
	}

	var oldContent []byte
	if a.fileQueue == nil {
		a.fileQueue = newFileMutationQueue()
	}
	if err := a.fileQueue.With(fullPath, func() error {
		if _, statErr := os.Stat(fullPath); statErr == nil {
			b, readErr := os.ReadFile(fullPath)
			if readErr != nil {
				return fmt.Errorf("failed to read existing file %s: %w", filePath, readErr)
			}
			oldContent = b
		}
		return os.WriteFile(fullPath, []byte(content), 0644)
	}); err != nil {
		return "", err
	}

	// Compute diff using two temp files, cleaning up regardless of errors.
	tmpFile, err := os.CreateTemp("", "mairu-diff-*")
	if err != nil {
		return "", nil
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Write(oldContent) //nolint:errcheck // best-effort diff
	tmpFile.Close()

	tmpFile2, err := os.CreateTemp("", "mairu-diff-new-*")
	if err != nil {
		return "", nil
	}
	defer os.Remove(tmpFile2.Name())
	tmpFile2.Write([]byte(content)) //nolint:errcheck // best-effort diff
	tmpFile2.Close()

	cmd := exec.Command("diff", "-u", tmpFile.Name(), tmpFile2.Name())
	out, _ := cmd.CombinedOutput()
	diffStr := string(out)
	diffStr = strings.Replace(diffStr, tmpFile.Name(), filePath+" (old)", 1)
	diffStr = strings.Replace(diffStr, tmpFile2.Name(), filePath+" (new)", 1)
	return diffStr, nil
}

// FindFiles uses glob pattern to find files.
func (a *Agent) FindFiles(pattern string) (string, error) {
	fs := os.DirFS(a.root)
	matches, err := doublestar.Glob(fs, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to search pattern %s: %w", pattern, err)
	}

	if len(matches) == 0 {
		return "No files found matching pattern.", nil
	}

	return strings.Join(matches, "\n"), nil
}

// SearchCodebase runs a fast concurrent semantic search in Go.
func (a *Agent) SearchCodebase(query string) (string, error) {
	return a.fallbackSearch(query)
}

type readFileTool struct{}

func (t *readFileTool) Definition() llm.Tool {
	return llm.Tool{
		Name:        "read_file",
		Description: "Read the contents of a file. Supports reading specific sections using offset and limit. Output is truncated to 2000 lines by default. Use offset/limit for large files.",
		Parameters: &llm.JSONSchema{
			Type: llm.TypeObject,
			Properties: map[string]*llm.JSONSchema{
				"file_path": {Type: llm.TypeString, Description: "The relative path to the file."},
				"offset":    {Type: llm.TypeInteger, Description: "The line number to start reading from (1-indexed). Defaults to 1."},
				"limit":     {Type: llm.TypeInteger, Description: "Maximum number of lines to read. Defaults to 2000."},
			},
			Required: []string{"file_path"},
		},
	}
}

func (t *readFileTool) Execute(ctx context.Context, args map[string]any, a *Agent, outChan chan<- AgentEvent) (map[string]any, error) {
	filePath, _ := args["file_path"].(string)
	offsetFloat, _ := args["offset"].(float64)
	limitFloat, _ := args["limit"].(float64)

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
		return map[string]any{"error": err.Error()}, nil
	}
	return map[string]any{"content": content}, nil
}

type writeFileTool struct{}

func (t *writeFileTool) Definition() llm.Tool {
	return llm.Tool{
		Name:        "write_file",
		Description: "Write content to a file, overwriting it completely. If editing an existing file, prefer multi_edit.",
		Parameters: &llm.JSONSchema{
			Type: llm.TypeObject,
			Properties: map[string]*llm.JSONSchema{
				"file_path": {Type: llm.TypeString, Description: "The relative path to the file."},
				"content":   {Type: llm.TypeString, Description: "The entire new content of the file."},
			},
			Required: []string{"file_path", "content"},
		},
	}
}

func (t *writeFileTool) Execute(ctx context.Context, args map[string]any, a *Agent, outChan chan<- AgentEvent) (map[string]any, error) {
	filePath, _ := args["file_path"].(string)
	content, _ := args["content"].(string)

	outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("💾 Writing file: %s", filePath)}
	diffStr, err := a.WriteFile(filePath, content)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	if diffStr != "" {
		outChan <- AgentEvent{Type: "diff", Content: fmt.Sprintf("```diff\n%s\n```", diffStr)}
	}

	verifOut, verifErr := a.runAutoVerification(ctx, filePath, outChan)
	if verifErr != nil {
		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("⚠️ Auto-verification failed for %s", filePath)}
		return map[string]any{
			"status":  "file written but auto-verification failed",
			"error":   verifErr.Error(),
			"output":  verifOut,
			"message": "The file was written, but the project failed to build/lint. Please review the output and fix the errors immediately.",
		}, nil
	}
	return map[string]any{"status": "success", "verification": "passed"}, nil
}

type deleteFileTool struct{}

func (t *deleteFileTool) Definition() llm.Tool {
	return llm.Tool{
		Name:        "delete_file",
		Description: "Delete a file or directory.",
		Parameters: &llm.JSONSchema{
			Type: llm.TypeObject,
			Properties: map[string]*llm.JSONSchema{
				"path": {Type: llm.TypeString, Description: "The relative path to the file or directory."},
			},
			Required: []string{"path"},
		},
	}
}

func (t *deleteFileTool) Execute(ctx context.Context, args map[string]any, a *Agent, outChan chan<- AgentEvent) (map[string]any, error) {
	pathToDelete, _ := args["path"].(string)
	outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🗑️ Deleting: %s", pathToDelete)}
	fullPath := filepath.Join(a.root, pathToDelete)
	if err := os.RemoveAll(fullPath); err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	return map[string]any{"status": "success"}, nil
}
