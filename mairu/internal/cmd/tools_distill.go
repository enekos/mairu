package cmd

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

func NewDistillCmd() *cobra.Command {
	cmd := &cobra.Command{
	Use:   "distill [command...]",
	Short: "AI-optimized error isolator (runs command, captures stack traces/errors)",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		execCmd := exec.Command(args[0], args[1:]...)

		// Capture both stdout and stderr
		output, err := execCmd.CombinedOutput()

		if err == nil && len(output) == 0 {
			fmt.Println("Command completed successfully with no output.")
			return
		}

		outStr := string(output)

		// If success, just print the tail or say success
		if err == nil {
			lines := strings.Split(strings.TrimSpace(outStr), "\n")
			if len(lines) > 20 {
				fmt.Printf("Command completed successfully. Output (last 20 lines):\n...\n%s\n", strings.Join(lines[len(lines)-20:], "\n"))
			} else {
				fmt.Printf("Command completed successfully. Output:\n%s\n", outStr)
			}
			return
		}

		// It failed. Let's distill the errors.
		lines := strings.Split(outStr, "\n")
		var errorLines []string

		// Basic heuristics for errors
		// 1. Lines containing "Error:", "Exception:", "FAIL", "panic:"
		// 2. Lines that look like file paths with line numbers (e.g. foo/bar.go:12:34)
		pathRegex := regexp.MustCompile(`([a-zA-Z0-9_/.-]+\.[a-zA-Z0-9]+:\d+(?::\d+)?)`)
		keywordRegex := regexp.MustCompile(`(?i)(error|exception|fail|panic|traceback)`)

		for i, line := range lines {
			if keywordRegex.MatchString(line) || pathRegex.MatchString(line) {
				// Include some context (previous line and next line) if possible, but keep it tight
				start := i
				end := i
				// Expand up to 1 line of context if it's indented (part of a stack trace)
				for start > 0 && strings.HasPrefix(lines[start-1], " ") {
					start--
				}
				for end < len(lines)-1 && strings.HasPrefix(lines[end+1], " ") {
					end++
				}

				for j := start; j <= end; j++ {
					if strings.TrimSpace(lines[j]) != "" {
						errorLines = append(errorLines, lines[j])
					}
				}
			}
		}

		// Deduplicate consecutive identical lines (from context expansion)
		var distilled []string
		var lastLine string
		for _, line := range errorLines {
			if line != lastLine {
				distilled = append(distilled, line)
				lastLine = line
			}
		}

		if len(distilled) == 0 {
			// Fallback to the last 30 lines if heuristics failed
			if len(lines) > 30 {
				fmt.Printf("Command failed. Returning last 30 lines of output:\n...\n%s\n", strings.Join(lines[len(lines)-30:], "\n"))
			} else {
				fmt.Printf("Command failed. Output:\n%s\n", outStr)
			}
			return
		}

		fmt.Printf("Command failed. Distilled Errors:\n\n%s\n", strings.Join(distilled, "\n"))
	},
}
	return cmd
}
