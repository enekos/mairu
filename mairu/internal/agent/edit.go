package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type EditBlock struct {
	StartLine uint32 // 1-indexed
	EndLine   uint32 // 1-indexed
	Content   string
}

// MultiEdit safely applies multiple block replacements to a file.
func (a *Agent) MultiEdit(filePath string, edits []EditBlock) (string, error) {
	fullPath := fmt.Sprintf("%s/%s", a.db.Root(), filePath)
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

	// Create a temp file with original content to compute diff
	tmpFile, err := os.CreateTemp("", "mairu-diff-*")
	if err == nil {
		tmpFile.Write(content)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		tmpFile2, err2 := os.CreateTemp("", "mairu-diff-new-*")
		if err2 == nil {
			tmpFile2.Write([]byte(newContent))
			tmpFile2.Close()
			defer os.Remove(tmpFile2.Name())

			cmd := exec.Command("diff", "-u", tmpFile.Name(), tmpFile2.Name())
			out, _ := cmd.CombinedOutput()
			diffStr := string(out)
			// clean up file names in diff
			diffStr = strings.Replace(diffStr, tmpFile.Name(), filePath+" (old)", 1)
			diffStr = strings.Replace(diffStr, tmpFile2.Name(), filePath+" (new)", 1)

			if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
				return "", err
			}
			return diffStr, nil
		}
	}

	return "", os.WriteFile(fullPath, []byte(newContent), 0644)
}

// ReplaceBlock safely replaces an exact string block in a file.
func (a *Agent) ReplaceBlock(filePath string, oldString, newString string) (string, error) {
	fullPath := filepath.Join(a.db.Root(), filePath)
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

	// Create a temp file with original content to compute diff
	tmpFile, err := os.CreateTemp("", "mairu-diff-*")
	if err == nil {
		tmpFile.Write(content)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		tmpFile2, err2 := os.CreateTemp("", "mairu-diff-new-*")
		if err2 == nil {
			tmpFile2.Write([]byte(newContent))
			tmpFile2.Close()
			defer os.Remove(tmpFile2.Name())

			cmd := exec.Command("diff", "-u", tmpFile.Name(), tmpFile2.Name())
			out, _ := cmd.CombinedOutput()
			diffStr := string(out)
			// clean up file names in diff
			diffStr = strings.Replace(diffStr, tmpFile.Name(), filePath+" (old)", 1)
			diffStr = strings.Replace(diffStr, tmpFile2.Name(), filePath+" (new)", 1)

			if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
				return "", err
			}
			return diffStr, nil
		}
	}

	return "", os.WriteFile(fullPath, []byte(newContent), 0644)
}
