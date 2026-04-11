package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var peekLines string
var peekSymbol string
var peekNumbered bool



type peekResult struct {
	F       string `json:"f"`
	Lines   string `json:"lines,omitempty"`
	Symbol  string `json:"symbol,omitempty"`
	Content string `json:"content"`
}

func extractSymbol(lines []string, file string, symbol string, numbered bool) peekResult {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(symbol) + `\b`)
	foundIdx := -1
	for i, line := range lines {
		if re.MatchString(line) {
			foundIdx = i
			break
		}
	}
	if foundIdx == -1 {
		return peekResult{F: file, Symbol: symbol, Content: "// symbol not found"}
	}

	start := foundIdx + 1 - 2
	if start < 1 {
		start = 1
	}

	ext := strings.ToLower(filepath.Ext(file))
	var end int
	if ext == ".py" {
		end = extractByIndent(lines, foundIdx) + 1
	} else {
		end = extractByBracket(lines, foundIdx) + 1
	}

	if end < start {
		end = start + 10
	}
	if start > len(lines) {
		start = len(lines)
	}
	if end > len(lines) {
		end = len(lines)
	}

	snippet := lines[start-1 : end]
	content := formatSnippet(snippet, start, numbered)

	return peekResult{
		F:       file,
		Symbol:  symbol,
		Lines:   fmt.Sprintf("%d-%d", start, end),
		Content: content,
	}
}

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

func NewPeekCmd() *cobra.Command {
	cmd := &cobra.Command{
	Use:   "peek <file>",
	Short: "AI-optimized file peeker (JSON)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]
		contentBytes, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		if peekLines == "" && peekSymbol == "" {
			fmt.Fprintf(os.Stderr, "error: --lines (-l) or --symbol (-s) flag is required for peek\n")
			os.Exit(1)
		}

		lines := strings.Split(string(contentBytes), "\n")

		if peekLines != "" {
			// Line range extraction (unchanged logic)
			parts := strings.Split(peekLines, "-")
			if len(parts) != 2 {
				fmt.Fprintf(os.Stderr, "error: invalid lines format, expected N-M\n")
				os.Exit(1)
			}
			start, err := strconv.Atoi(parts[0])
			end, _ := strconv.Atoi(parts[1])
			if err != nil || start < 1 || end < start {
				fmt.Fprintf(os.Stderr, "error: invalid line range\n")
				os.Exit(1)
			}
			if start > len(lines) {
				start = len(lines)
			}
			if end > len(lines) {
				end = len(lines)
			}
			snippet := lines[start-1 : end]
			content := formatSnippet(snippet, start, peekNumbered)

			res := peekResult{F: file, Lines: peekLines, Content: content}
			out, _ := json.Marshal(res)
			fmt.Println(string(out))
			return
		}

		// Symbol extraction
		symbols := strings.Split(peekSymbol, ",")
		if len(symbols) == 1 {
			res := extractSymbol(lines, file, strings.TrimSpace(symbols[0]), peekNumbered)
			out, _ := json.Marshal(res)
			fmt.Println(string(out))
		} else {
			var results []peekResult
			for _, s := range symbols {
				results = append(results, extractSymbol(lines, file, strings.TrimSpace(s), peekNumbered))
			}
			out, _ := json.Marshal(results)
			fmt.Println(string(out))
		}
	},
}
	cmd.Flags().StringVarP(&peekLines, "lines", "l", "", "Line range to peek (e.g. 10-20)")
	cmd.Flags().StringVarP(&peekSymbol, "symbol", "s", "", "Symbol name to extract (finds bracket bounds, comma-separated for multiple)")
	cmd.Flags().BoolVarP(&peekNumbered, "numbered", "N", false, "Prefix each line with its line number")
	return cmd
}
