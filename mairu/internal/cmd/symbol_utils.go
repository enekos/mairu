package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func extractByBracket(lines []string, foundIdx int) int {
	openBrackets := 0
	startedBrackets := false
	endIdx := foundIdx

	for i := foundIdx; i < len(lines); i++ {
		endIdx = i
		openBrackets += strings.Count(lines[i], "{")
		openBrackets -= strings.Count(lines[i], "}")

		if strings.Contains(lines[i], "{") {
			startedBrackets = true
		}

		if startedBrackets && openBrackets <= 0 {
			break
		}
		if !startedBrackets && i-foundIdx > 10 {
			break
		}
		if i-foundIdx > 500 {
			break
		}
	}
	return endIdx
}

func extractByIndent(lines []string, startIdx int) int {
	baseIndent := indentLevel(lines[startIdx])
	endIdx := startIdx
	for i := startIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			endIdx = i
			continue
		}
		if indentLevel(line) > baseIndent {
			endIdx = i
		} else {
			break
		}
		if i-startIdx > 500 {
			break
		}
	}
	return endIdx
}

func indentLevel(line string) int {
	n := 0
	for _, ch := range line {
		if ch == ' ' {
			n++
		} else if ch == '\t' {
			n += 4
		} else {
			break
		}
	}
	return n
}

func formatSnippet(lines []string, startLine int, numbered bool) string {
	if !numbered {
		return strings.Join(lines, "\n")
	}
	var out []string
	for i, line := range lines {
		out = append(out, fmt.Sprintf("%d: %s", startLine+i, line))
	}
	return strings.Join(out, "\n")
}

func getSymbolBounds(lines []string, file string, symbol string) (int, int, error) {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(symbol) + `\b`)
	foundIdx := -1
	for i, line := range lines {
		if re.MatchString(line) {
			foundIdx = i
			break
		}
	}
	if foundIdx == -1 {
		return -1, -1, fmt.Errorf("symbol '%s' not found", symbol)
	}

	ext := strings.ToLower(filepath.Ext(file))
	var endIdx int
	if ext == ".py" || ext == ".yaml" || ext == ".yml" {
		endIdx = extractByIndent(lines, foundIdx)
	} else {
		endIdx = extractByBracket(lines, foundIdx)
	}

	return foundIdx, endIdx, nil
}
