package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func NewSummarizeCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "summarize <query>",
		Short: "Summarize using vibe query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, _ := cmd.Flags().GetInt("k")
			out, err := ContextPost("/api/vibe/query", map[string]any{
				"prompt":  args[0],
				"project": project,
				"topK":    k,
			})
			if err != nil {
				return err
			}
			PrintJSON(out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "", "Project name")
	cmd.Flags().IntP("k", "k", 5, "Top K results")
	return cmd
}
func NewFlushCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "flush [prompt]",
		Short: "Flush transcript into durable facts",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("prompt is required")
			}
			k, _ := cmd.Flags().GetInt("k")
			return runVibeMutation(project, args[0], k)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "", "Project name")
	cmd.Flags().IntP("k", "k", 5, "Top K results")
	return cmd
}
func NewNudgeCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "nudge [prompt]",
		Short: "Suggest mutations from transcript",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("prompt is required")
			}
			k, _ := cmd.Flags().GetInt("k")
			return runVibeMutation(project, args[0], k)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "", "Project name")
	cmd.Flags().IntP("k", "k", 5, "Top K results")
	return cmd
}
func NewIngestCmd() *cobra.Command {
	var project, baseURI, textStr string
	var yes, noRouter bool

	cmd := &cobra.Command{
		Use:   "ingest [file]",
		Short: "Parse an MD file or free text via LLM into context nodes, review, then persist",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && textStr == "" {
				return fmt.Errorf("provide a file path or --text <text>")
			}
			content := textStr
			if len(args) > 0 {
				b, err := os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				content = string(b)
				fmt.Printf("\nRead %d characters from %s\n", len(content), args[0])
			}

			fmt.Println("\nParsing into context nodes via LLM...")
			out, err := ContextPost("/api/vibe/ingest", map[string]any{
				"text":     content,
				"base_uri": baseURI,
			})
			if err != nil {
				return err
			}

			var res struct {
				Nodes []map[string]any `json:"nodes"`
			}
			if err := json.Unmarshal(out, &res); err != nil {
				return err
			}

			fmt.Printf("\nProposed %d context node(s):\n\n", len(res.Nodes))
			for _, n := range res.Nodes {
				fmt.Printf("URI: %v\nName: %v\nAbstract: %v\n---\n", n["URI"], n["Name"], n["Abstract"])
			}

			if !yes {
				fmt.Print("Persist these nodes? [y/N]: ")
				var answer string
				fmt.Scanln(&answer)
				if answer != "y" && answer != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			fmt.Printf("\nPersisting %d node(s)...\n", len(res.Nodes))
			for _, n := range res.Nodes {
				uri, _ := n["URI"].(string)
				name, _ := n["Name"].(string)
				abstract, _ := n["Abstract"].(string)
				contentStr, _ := n["Content"].(string)
				overview, _ := n["Overview"].(string)
				parent, _ := n["ParentURI"].(string)

				if err := RunNodeStore(project, uri, name, abstract, parent, overview, contentStr); err != nil {
					fmt.Printf("Failed to store %s: %v\n", uri, err)
				} else {
					fmt.Printf("Stored %s\n", uri)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().StringVar(&textStr, "text", "", "Free text to ingest")
	cmd.Flags().StringVar(&baseURI, "base-uri", "contextfs://ingested", "Base URI namespace for generated nodes")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip interactive review and persist all proposed nodes")
	cmd.Flags().BoolVar(&noRouter, "no-router", false, "Skip LLM dedup router when persisting nodes")
	return cmd
}
