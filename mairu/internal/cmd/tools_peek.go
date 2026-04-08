package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var peekLines string
var peekSymbol string

func init() {
	peekCmd.Flags().StringVarP(&peekLines, "lines", "l", "", "Line range to extract (e.g., 50-100)")
	peekCmd.Flags().StringVarP(&peekSymbol, "symbol", "s", "", "Symbol name to extract (e.g., myFunc)")
}

type peekResult struct {
	F       string `json:"f"`
	Lines   string `json:"lines,omitempty"`
	Symbol  string `json:"symbol,omitempty"`
	Content string `json:"content"`
}

var peekCmd = &cobra.Command{
	Use:   "peek <file>",
	Short: "AI-optimized exact line extraction (JSON)",
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
		var start, end int

		if peekLines != "" {
			parts := strings.Split(peekLines, "-")
			if len(parts) != 2 {
				fmt.Fprintf(os.Stderr, "error: invalid lines format, expected N-M\n")
				os.Exit(1)
			}

			start, err = strconv.Atoi(parts[0])
			end, _ = strconv.Atoi(parts[1])
			if err != nil || start < 1 || end < start {
				fmt.Fprintf(os.Stderr, "error: invalid line range\n")
				os.Exit(1)
			}
		} else if peekSymbol != "" {
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(peekSymbol) + `\b`)
			foundIdx := -1
			for i, line := range lines {
				if re.MatchString(line) {
					foundIdx = i
					break
				}
			}
			if foundIdx == -1 {
				fmt.Fprintf(os.Stderr, "error: symbol not found\n")
				os.Exit(1)
			}

			start = foundIdx + 1 - 2
			if start < 1 {
				start = 1
			}

			// Bracket matching heuristic
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

				// Safety breaker: don't grab more than 100 lines if no brackets
				if !startedBrackets && i-foundIdx > 10 {
					break
				}
				if i-foundIdx > 500 { // Max 500 lines for a symbol
					break
				}
			}

			end = endIdx + 1
			if end < start {
				end = start + 10 // fallback
			}
		}

		if start > len(lines) {
			start = len(lines)
		}
		if end > len(lines) {
			end = len(lines)
		}

		snippet := lines[start-1 : end]

		res := peekResult{
			F:       file,
			Lines:   peekLines,
			Symbol:  peekSymbol,
			Content: strings.Join(snippet, "\n"),
		}

		out, _ := json.Marshal(res)
		fmt.Println(string(out))
	},
}
