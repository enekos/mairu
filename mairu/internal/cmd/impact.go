package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func NewImpactCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "impact <uri>",
		Short: "Analyze blast radius / reverse dependencies for a given node or symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uri := args[0]

			graph, err := loadLogicGraph(project)
			if err != nil {
				return err
			}

			reverseDeps := graph.GetReverseDependencies()
			affectedFiles := map[string]bool{}

			for toSymbol, dependentFiles := range reverseDeps {
				if strings.Contains(toSymbol, uri) {
					for _, fileURI := range dependentFiles {
						affectedFiles[fileURI] = true
					}
				}
			}

			if len(affectedFiles) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No upstream dependencies found for '%s'.\n", uri)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "TARGET: %s\n\n", uri)
			fmt.Fprintln(cmd.OutOrStdout(), "UPSTREAM (what depends on this):")
			for f := range affectedFiles {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", f)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "", "Project name")
	return cmd
}
