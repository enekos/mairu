package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newSkillCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "ContextFS skill operations",
	}
	cmd.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	addCmd := &cobra.Command{
		Use:   "add <name> <description>",
		Short: "Store a skill",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := contextPost("/api/skills", map[string]any{
				"project":     project,
				"name":        args[0],
				"description": args[1],
			})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search skills",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := contextGet("/api/search", searchParamsFromFlags(cmd, args[0], "skill", project))
			if err != nil {
				return err
			}

			if outputFormat == "json" || outputFormat == "" {
				printJSON(out)
			} else {
				var results []map[string]any
				if err := json.Unmarshal(out, &results); err != nil {
					printJSON(out) // fallback
					return nil
				}
				f := GetFormatter()
				f.PrintItems(
					[]string{"score", "name", "description"},
					results,
					func(item map[string]any) map[string]string {
						return map[string]string{
							"score":       fmt.Sprintf("%.2f", item["_rankingScore"]),
							"name":        fmt.Sprintf("%v", item["name"]),
							"description": truncate(fmt.Sprintf("%v", item["description"]), 80),
						}
					},
				)
			}
			return nil
		},
	}
	addCommonSearchFlags(searchCmd)

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			out, err := contextGet("/api/skills", map[string]string{
				"project": project,
				"limit":   fmt.Sprintf("%d", limit),
			})
			if err != nil {
				return err
			}

			if outputFormat == "json" || outputFormat == "" {
				printJSON(out)
			} else {
				var results []map[string]any
				if err := json.Unmarshal(out, &results); err != nil {
					printJSON(out) // fallback
					return nil
				}
				f := GetFormatter()
				f.PrintItems(
					[]string{"id", "name", "description"},
					results,
					func(item map[string]any) map[string]string {
						return map[string]string{
							"id":          fmt.Sprintf("%v", item["id"]),
							"name":        fmt.Sprintf("%v", item["name"]),
							"description": truncate(fmt.Sprintf("%v", item["description"]), 80),
						}
					},
				)
			}
			return nil
		},
	}
	listCmd.Flags().Int("limit", 200, "Maximum items")

	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := contextDelete("/api/skills", map[string]string{"id": args[0]})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}

	cmd.AddCommand(addCmd, searchCmd, listCmd, deleteCmd)
	return cmd
}
