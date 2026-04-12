package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
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

	// Byte truncation just in case the lines are absurdly long (e.g. minified JS)
	maxBytes := 50 * 1024 // 50KB
	if len(res) > maxBytes {
		res = res[:maxBytes] + "\n...[Output truncated, exceeded 50KB limit. Use offset/limit or grep to find what you need]"
	}

	if endIdx < totalLines {
		res += fmt.Sprintf("\n...[File truncated. Showing lines %d to %d of %d. Use offset=%d to read more]", startIdx+1, endIdx, totalLines, endIdx+1)
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
	if _, err := os.Stat(fullPath); err == nil {
		oldContent, err = os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("failed to read existing file %s: %w", filePath, err)
		}
	}

	err := os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
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
