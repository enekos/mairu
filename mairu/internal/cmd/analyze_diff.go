package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func NewAnalyzeDiffCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "analyze-diff",
		Short: "Analyze the current git diff against the codebase graph to determine blast radius",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Get changed files from git
			gitCmd := exec.Command("git", "diff", "--name-only", "HEAD")
			out, err := gitCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to run git diff: %v", err)
			}

			changedFiles := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(changedFiles) == 0 || changedFiles[0] == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No changed files detected by git diff.")
				return nil
			}

			// 2. Load the graph
			graph, err := loadLogicGraph(project)
			if err != nil {
				return fmt.Errorf("failed to load graph: %v", err)
			}

			// 3. Find affected symbols and their reverse dependencies
			reverseDeps := graph.GetReverseDependencies()

			directlyModifiedURIs := map[string]bool{}
			var modifiedSymbolIDs []string

			for _, file := range changedFiles {
				for symID, uri := range graph.Symbols {
					if strings.HasSuffix(uri, file) {
						directlyModifiedURIs[uri] = true
						modifiedSymbolIDs = append(modifiedSymbolIDs, symID)
					}
				}
			}

			if len(directlyModifiedURIs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Changed files do not match any known context nodes in the graph.")
				return nil
			}

			affectedDownstream := map[string]bool{}
			for _, symID := range modifiedSymbolIDs {
				for _, depURI := range reverseDeps[symID] {
					if !directlyModifiedURIs[depURI] {
						affectedDownstream[depURI] = true
					}
				}
			}

			// 4. Output Risk Analysis
			fmt.Fprintf(cmd.OutOrStdout(), "--- Diff Blast Radius Analysis ---\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Directly Modified Nodes: %d\n", len(directlyModifiedURIs))
			for uri := range directlyModifiedURIs {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", uri)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nAffected Downstream Nodes: %d\n", len(affectedDownstream))
			for uri := range affectedDownstream {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", uri)
			}

			riskLevel := "LOW"
			if len(affectedDownstream) > 10 {
				riskLevel = "HIGH"
			} else if len(affectedDownstream) > 3 {
				riskLevel = "MEDIUM"
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nOverall Risk Level: %s\n", riskLevel)

			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project name")
	return cmd
}
