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

func NewPeekCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peek <file>",
		Short: "AI-optimized file peeker (JSON)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file := args[0]
			contentBytes, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("error: %w", err)
			}

			if peekLines == "" && peekSymbol == "" {
				return fmt.Errorf("error: --lines (-l) or --symbol (-s) flag is required for peek")
			}

			lines := strings.Split(string(contentBytes), "\n")

			if peekLines != "" {
				// Line range extraction (unchanged logic)
				parts := strings.Split(peekLines, "-")
				if len(parts) != 2 {
					return fmt.Errorf("error: invalid lines format, expected N-M")
				}
				start, err := strconv.Atoi(parts[0])
				if err != nil {
					return fmt.Errorf("error: invalid line range")
				}
				end, err := strconv.Atoi(parts[1])
				if err != nil || start < 1 || end < start {
					return fmt.Errorf("error: invalid line range")
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
				return nil
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
			return nil
		},
	}
	cmd.Flags().StringVarP(&peekLines, "lines", "l", "", "Line range to peek (e.g. 10-20)")
	cmd.Flags().StringVarP(&peekSymbol, "symbol", "s", "", "Symbol name to extract (finds bracket bounds, comma-separated for multiple)")
	cmd.Flags().BoolVarP(&peekNumbered, "numbered", "N", false, "Prefix each line with its line number")
	return cmd
}
