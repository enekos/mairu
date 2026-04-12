package cmd

import "github.com/spf13/cobra"

func NewAnalyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze codebase graphs and diffs",
	}
	diff := NewAnalyzeDiffCmd()
	diff.Use = "diff"
	graph := NewAnalyzeGraphCmd()
	graph.Use = "graph"
	cmd.AddCommand(diff, graph)
	return cmd
}
