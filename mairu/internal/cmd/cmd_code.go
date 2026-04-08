package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCodeCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "code",
		Short: "Semantic code search (bypasses LLM vibe query)",
	}
	cmd.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search code files natively via AST daemon context nodes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			out, err := contextGet("/api/search", searchParamsFromFlags(cmd, args[0], "context", project))
			if err != nil {
				return err
			}

			if format == "json" {
				printJSON(out)
				return nil
			}

			// Parse response to extract paths
			var resp struct {
				ContextNodes []struct {
					URI      string  `json:"uri"`
					Name     string  `json:"name"`
					Abstract string  `json:"abstract"`
					Score    float64 `json:"_score"` // Meilisearch score if available
				} `json:"contextNodes"`
			}
			if err := json.Unmarshal(out, &resp); err != nil {
				// Fallback to raw if parsing fails
				printJSON(out)
				return nil
			}

			var validNodes []struct {
				URI      string  `json:"uri"`
				Name     string  `json:"name"`
				Abstract string  `json:"abstract"`
				Score    float64 `json:"_score"`
			}

			for _, node := range resp.ContextNodes {
				// strictly scope to daemon-generated files (or project files)
				// file URIs look like: contextfs://<project>/path/to/file.ext
				prefix := "contextfs://"
				if project != "" {
					prefix += project + "/"
				}
				if project != "" && !strings.HasPrefix(node.URI, prefix) {
					continue
				}
				validNodes = append(validNodes, node)
			}

			if len(validNodes) == 0 {
				fmt.Println("No matching code files found.")
				return nil
			}

			for _, node := range validNodes {
				// Convert contextfs://my-project/src/file.ts -> src/file.ts
				path := strings.TrimPrefix(node.URI, "contextfs://")
				if project != "" {
					path = strings.TrimPrefix(path, project+"/")
				} else {
					// If no project specified, extract it from URI
					parts := strings.SplitN(path, "/", 2)
					if len(parts) == 2 {
						path = parts[1]
					}
				}

				if format == "paths" {
					fmt.Println(path)
				} else {
					// Default format: path + abstract
					fmt.Printf("%s\n", path)
					if node.Abstract != "" {
						fmt.Printf("  %s\n", strings.TrimSpace(node.Abstract))
					}
					fmt.Println()
				}
			}

			return nil
		},
	}
	addCommonSearchFlags(searchCmd)
	searchCmd.Flags().String("format", "default", "Output format: default, paths, or json")

	cmd.AddCommand(searchCmd)
	return cmd
}
