package cmd

import (
	"fmt"
	"mairu/internal/analyzer"
	"strings"

	"github.com/spf13/cobra"
)

func NewAnalyzeGraphCmd() *cobra.Command {
	var project string
	var save bool
	cmd := &cobra.Command{
		Use:   "analyze-graph",
		Short: "Analyze the AST graph to generate execution flows and functional clusters (skills)",
		RunE: func(cmd *cobra.Command, args []string) error {
			graph, err := loadLogicGraph(project)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Graph loaded: %d symbols, %d edges\n", len(graph.Symbols), len(graph.Edges))

			flows := analyzer.AnalyzeFlows(graph)
			fmt.Fprintf(cmd.OutOrStdout(), "\n--- Execution Flows ---\n")
			for _, flow := range flows {
				fmt.Fprintf(cmd.OutOrStdout(), "Flow starting at %s: %d steps\n", flow.StartSymbol, len(flow.Trace))
				if save {
					flowURI := fmt.Sprintf("contextfs://%s/flows/%s", project, flow.StartSymbol)
					abstract := fmt.Sprintf("Execution flow starting at %s", flow.StartSymbol)
					overview := strings.Join(flow.Trace, " -> ")
					_, err := StoreNodeRaw(project, flowURI, "Flow: "+flow.StartSymbol, abstract, "", overview, overview)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Failed to save flow %s: %v\n", flow.StartSymbol, err)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "  Saved flow to %s\n", flowURI)
					}
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Generated %d execution flows.\n", len(flows))

			clusters := analyzer.AnalyzeClusters(graph)
			fmt.Fprintf(cmd.OutOrStdout(), "\n--- Functional Clusters ---\n")
			for i, cluster := range clusters {
				fmt.Fprintf(cmd.OutOrStdout(), "Cluster %d: %d symbols\n", i+1, len(cluster.Symbols))
				if save {
					skillURI := fmt.Sprintf("contextfs://%s/skills/cluster_%d", project, i+1)
					abstract := fmt.Sprintf("Functional Cluster %d containing %d symbols", i+1, len(cluster.Symbols))
					overview := strings.Join(cluster.Symbols, ", ")
					_, err := StoreNodeRaw(project, skillURI, fmt.Sprintf("Cluster %d", i+1), abstract, "", overview, overview)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Failed to save cluster %d: %v\n", i+1, err)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "  Saved cluster to %s\n", skillURI)
					}
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project name")
	cmd.Flags().BoolVar(&save, "save", false, "Save the generated flows and clusters as context nodes")
	return cmd
}
