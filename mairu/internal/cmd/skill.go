package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewSkillCmd() *cobra.Command {
	var project string
	c := &cobra.Command{
		Use:   "skill",
		Short: "ContextFS skill operations",
	}
	c.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	addCmd := &cobra.Command{
		Use:   "add <name> <description>",
		Short: "Store a skill",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := ContextPost("/api/skills", map[string]any{
				"project":     project,
				"name":        args[0],
				"description": args[1],
			})
			if err != nil {
				return err
			}
			PrintJSON(out)
			return nil
		},
	}

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search skills",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := ContextGet("/api/search", SearchParamsFromFlags(cmd, args[0], "skill", project))
			if err != nil {
				return err
			}

			printSearchOrListResults(out, []string{"score", "name", "description"}, func(item map[string]any) map[string]string {
				return map[string]string{
					"score":       fmt.Sprintf("%.2f", item["_rankingScore"]),
					"name":        fmt.Sprintf("%v", item["name"]),
					"description": Truncate(fmt.Sprintf("%v", item["description"]), 80),
				}
			})
			return nil
		},
	}
	AddCommonSearchFlags(searchCmd)

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			out, err := ContextGet("/api/skills", map[string]string{
				"project": project,
				"limit":   fmt.Sprintf("%d", limit),
			})
			if err != nil {
				return err
			}

			printSearchOrListResults(out, []string{"id", "name", "description"}, func(item map[string]any) map[string]string {
				return map[string]string{
					"id":          fmt.Sprintf("%v", item["id"]),
					"name":        fmt.Sprintf("%v", item["name"]),
					"description": Truncate(fmt.Sprintf("%v", item["description"]), 80),
				}
			})
			return nil
		},
	}
	listCmd.Flags().Int("limit", 200, "Maximum items")

	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := ContextDelete("/api/skills", map[string]string{"id": args[0]})
			if err != nil {
				return err
			}
			PrintJSON(out)
			return nil
		},
	}

	c.AddCommand(addCmd, searchCmd, listCmd, deleteCmd)
	return c
}
