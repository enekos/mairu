package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var spliceTarget string
var spliceReplaceWith string
var spliceAddImport string

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

func NewSpliceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "splice <file>",
		Short: "AI-optimized AST-aware symbol replacer",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			file := args[0]

			contentBytes, err := os.ReadFile(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
				os.Exit(1)
			}

			replaceBytes, err := os.ReadFile(spliceReplaceWith)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error reading replacement file: %v\n", err)
				os.Exit(1)
			}

			lines := strings.Split(string(contentBytes), "\n")
			replaceLines := strings.Split(string(replaceBytes), "\n")

			startIdx, endIdx, err := getSymbolBounds(lines, file, spliceTarget)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}

			var newLines []string

			// Splice: everything before startIdx, then replacement, then everything after endIdx
			newLines = append(newLines, lines[:startIdx]...)
			newLines = append(newLines, replaceLines...)
			if endIdx+1 < len(lines) {
				newLines = append(newLines, lines[endIdx+1:]...)
			}

			// Add import at the very top of the file
			if spliceAddImport != "" {
				newLines = append([]string{spliceAddImport}, newLines...)
			}

			newContent := strings.Join(newLines, "\n")

			if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "error writing file: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Successfully spliced '%s' in %s (lines %d-%d replaced)\n", spliceTarget, file, startIdx+1, endIdx+1)
		},
	}
	cmd.Flags().StringVarP(&spliceTarget, "target", "t", "", "Symbol name to replace (e.g. calculateTotal)")
	cmd.Flags().StringVarP(&spliceReplaceWith, "replace-with", "r", "", "File containing the new code")
	cmd.Flags().StringVarP(&spliceAddImport, "add-import", "i", "", "Import statement to inject at the top of the file")
	cmd.MarkFlagRequired("target")
	cmd.MarkFlagRequired("replace-with")
	return cmd
}
