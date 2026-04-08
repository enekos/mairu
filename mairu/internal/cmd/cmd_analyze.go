package cmd

import (
	"github.com/spf13/cobra"
)

func newAnalyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze codebase graphs and diffs",
	}

	// Rename them to drop the "analyze-" prefix
	diffCmd := newAnalyzeDiffCmd()
	diffCmd.Use = "diff"

	graphCmd := newAnalyzeGraphCmd()
	graphCmd.Use = "graph"

	cmd.AddCommand(diffCmd, graphCmd)
	return cmd
}
