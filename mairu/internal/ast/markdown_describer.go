package ast

import (
	"fmt"
	"regexp"
	"strings"
)

var reMarkdownHeading = regexp.MustCompile(`(?m)^(#{1,6}) (.+)$`)

// MarkdownDescriber implements LanguageDescriber for Markdown and MDX files.
// It extracts headings as symbols and returns the raw document text as content.
type MarkdownDescriber struct{}

func (MarkdownDescriber) LanguageID() string { return "markdown" }

func (MarkdownDescriber) Extensions() []string { return []string{".md", ".mdx"} }

func (MarkdownDescriber) ExtractFileGraph(filePath, source string) FileGraph {
	symbols := extractMarkdownHeadings(source)
	return FileGraph{
		FileSummary:        extractMarkdownSummary(source),
		RawContent:         source,
		Symbols:            symbols,
		Edges:              []LogicEdge{},
		Imports:            []string{},
		SymbolDescriptions: map[string]string{},
	}
}

func extractMarkdownHeadings(source string) []LogicSymbol {
	matches := reMarkdownHeading.FindAllStringSubmatch(source, -1)
	seen := map[string]int{}
	var symbols []LogicSymbol
	for _, m := range matches {
		level := len(m[1])
		text := strings.TrimSpace(m[2])
		kind := fmt.Sprintf("h%d", level)
		baseID := kind + ":" + text
		id := baseID
		if n := seen[baseID]; n > 0 {
			id = fmt.Sprintf("%s:%d", baseID, n+1)
		}
		seen[baseID]++
		symbols = append(symbols, LogicSymbol{ID: id, Name: text, Kind: kind})
	}
	return symbols
}

// extractMarkdownSummary returns a short description for the file.
// It looks for the first H1 heading and the first text paragraph beneath it.
func extractMarkdownSummary(source string) string {
	lines := strings.Split(source, "\n")

	// Skip YAML frontmatter (--- ... ---)
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}

	var title string
	var descLines []string
	pastTitle := false

	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)

		if !pastTitle {
			if strings.HasPrefix(trimmed, "# ") {
				title = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
				pastTitle = true
			}
			continue
		}

		// Collect first non-empty text paragraph after the H1
		if trimmed == "" {
			if len(descLines) > 0 {
				break
			}
			continue
		}
		// Stop at next heading or HTML/image lines
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "![") || strings.HasPrefix(trimmed, "<") {
			if len(descLines) > 0 {
				break
			}
			continue
		}
		descLines = append(descLines, trimmed)
	}

	desc := strings.Join(descLines, " ")
	var summary string
	if title != "" && desc != "" {
		summary = title + ": " + desc
	} else if title != "" {
		summary = title
	} else {
		summary = desc
	}

	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}
	if summary == "" {
		summary = "Markdown document"
	}
	return summary
}
