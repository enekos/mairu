package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func NewCodeCmd() *cobra.Command {
	var project string
	c := &cobra.Command{
		Use:   "code",
		Short: "Semantic code search (bypasses LLM vibe query)",
	}
	c.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search code files natively via AST daemon context nodes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			out, err := ContextGet("/api/search", SearchParamsFromFlags(cmd, args[0], "context", project))
			if err != nil {
				return err
			}

			if format == "json" {
				PrintJSON(out)
				return nil
			}

			var resp struct {
				ContextNodes []struct {
					URI      string  `json:"uri"`
					Name     string  `json:"name"`
					Abstract string  `json:"abstract"`
					Score    float64 `json:"_score"`
				} `json:"contextNodes"`
			}
			if err := json.Unmarshal(out, &resp); err != nil {
				PrintJSON(out)
				return nil
			}

			var validNodes []struct {
				URI      string  `json:"uri"`
				Name     string  `json:"name"`
				Abstract string  `json:"abstract"`
				Score    float64 `json:"_score"`
			}

			for _, node := range resp.ContextNodes {
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
				path := strings.TrimPrefix(node.URI, "contextfs://")
				if project != "" {
					path = strings.TrimPrefix(path, project+"/")
				} else {
					parts := strings.SplitN(path, "/", 2)
					if len(parts) == 2 {
						path = parts[1]
					}
				}

				if format == "paths" {
					fmt.Println(path)
				} else {
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
	AddCommonSearchFlags(searchCmd)
	searchCmd.Flags().String("format", "default", "Output format: default, paths, or json")

	c.AddCommand(searchCmd)
	return c
}
