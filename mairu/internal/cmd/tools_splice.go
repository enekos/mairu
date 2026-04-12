package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var spliceTarget string
var spliceReplaceWith string
var spliceAddImport string

func NewSpliceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "splice <file>",
		Short: "AI-optimized AST-aware symbol replacer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file := args[0]

			contentBytes, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("error reading file: %w", err)
			}

			replaceBytes, err := os.ReadFile(spliceReplaceWith)
			if err != nil {
				return fmt.Errorf("error reading replacement file: %w", err)
			}

			lines := strings.Split(string(contentBytes), "\n")
			replaceLines := strings.Split(string(replaceBytes), "\n")

			startIdx, endIdx, err := getSymbolBounds(lines, file, spliceTarget)
			if err != nil {
				return fmt.Errorf("error: %w", err)
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
				return fmt.Errorf("error writing file: %w", err)
			}

			fmt.Printf("Successfully spliced '%s' in %s (lines %d-%d replaced)\n", spliceTarget, file, startIdx+1, endIdx+1)
			return nil
		},
	}
	cmd.Flags().StringVarP(&spliceTarget, "target", "t", "", "Symbol name to replace (e.g. calculateTotal)")
	cmd.Flags().StringVarP(&spliceReplaceWith, "replace-with", "r", "", "File containing the new code")
	cmd.Flags().StringVarP(&spliceAddImport, "add-import", "i", "", "Import statement to inject at the top of the file")
	cmd.MarkFlagRequired("target")
	cmd.MarkFlagRequired("replace-with")
	return cmd
}
