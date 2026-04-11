package cmd

import (
	"github.com/spf13/cobra"
)

func NewAnalyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze codebase graphs and diffs",
	}

	// Rename them to drop the "analyze-" prefix
	diffCmd := NewAnalyzeDiffCmd()
	diffCmd.Use = "diff"

	graphCmd := NewAnalyzeGraphCmd()
	graphCmd.Use = "graph"

	cmd.AddCommand(diffCmd, graphCmd)
	return cmd
}
