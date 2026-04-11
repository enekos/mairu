package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func NewMemoryCmd() *cobra.Command {
	var project string
	c := &cobra.Command{
		Use:   "memory",
		Short: "ContextFS memory operations",
	}
	c.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search memories",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := ContextGet("/api/search", SearchParamsFromFlags(cmd, args[0], "memory", project))
			if err != nil {
				return err
			}

			printSearchOrListResults(out, []string{"score", "category", "content"}, func(item map[string]any) map[string]string {
				return map[string]string{
					"score":    fmt.Sprintf("%.2f", item["_rankingScore"]),
					"category": fmt.Sprintf("%v", item["category"]),
					"content":  Truncate(fmt.Sprintf("%v", item["content"]), 80),
				}
			})
			return nil
		},
	}
	AddCommonSearchFlags(searchCmd)

	storeCmd := &cobra.Command{
		Use:   "store <content>",
		Short: "Store a memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			category, _ := cmd.Flags().GetString("category")
			owner, _ := cmd.Flags().GetString("owner")
			importance, _ := cmd.Flags().GetInt("importance")
			return RunMemoryStore(project, args[0], category, owner, importance)
		},
	}
	storeCmd.Flags().StringP("category", "c", "observation", "Memory category")
	storeCmd.Flags().StringP("owner", "", "agent", "Memory owner")
	storeCmd.Flags().IntP("importance", "i", 5, "Importance (1-10)")

	addCmd := &cobra.Command{
		Use:    "add <content>",
		Short:  "Alias for memory store",
		Args:   cobra.ExactArgs(1),
		Hidden: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			category, _ := cmd.Flags().GetString("category")
			owner, _ := cmd.Flags().GetString("owner")
			importance, _ := cmd.Flags().GetInt("importance")
			return RunMemoryStore(project, args[0], category, owner, importance)
		},
	}
	addCmd.Flags().StringP("category", "c", "observation", "Memory category")
	addCmd.Flags().StringP("owner", "", "agent", "Memory owner")
	addCmd.Flags().IntP("importance", "i", 5, "Importance (1-10)")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List memories",
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			out, err := ContextGet("/api/memories", map[string]string{
				"project": project,
				"limit":   fmt.Sprintf("%d", limit),
			})
			if err != nil {
				return err
			}

			printSearchOrListResults(out, []string{"id", "category", "content"}, func(item map[string]any) map[string]string {
				return map[string]string{
					"id":       fmt.Sprintf("%v", item["id"]),
					"category": fmt.Sprintf("%v", item["category"]),
					"content":  Truncate(fmt.Sprintf("%v", item["content"]), 80),
				}
			})
			return nil
		},
	}
	listCmd.Flags().Int("limit", 200, "Maximum items")

	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, _ := cmd.Flags().GetString("content")
			category, _ := cmd.Flags().GetString("category")
			owner, _ := cmd.Flags().GetString("owner")
			importance, _ := cmd.Flags().GetInt("importance")
			out, err := ContextPut("/api/memories", map[string]any{
				"id":         args[0],
				"content":    content,
				"category":   category,
				"owner":      owner,
				"importance": importance,
			})
			if err != nil {
				return err
			}
			PrintJSON(out)
			return nil
		},
	}
	updateCmd.Flags().String("content", "", "New content")
	updateCmd.Flags().String("category", "", "New category")
	updateCmd.Flags().String("owner", "", "New owner")
	updateCmd.Flags().Int("importance", 0, "New importance")

	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := ContextDelete("/api/memories", map[string]string{"id": args[0]})
			if err != nil {
				return err
			}
			PrintJSON(out)
			return nil
		},
	}

	feedbackCmd := &cobra.Command{
		Use:   "feedback <id> <reward>",
		Short: "Apply reinforcement learning feedback to a memory (reward 1-10)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			reward, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("reward must be an integer between 1 and 10")
			}
			out, err := ContextPost("/api/memories/feedback", map[string]any{
				"id":     args[0],
				"reward": reward,
			})
			if err != nil {
				return err
			}
			PrintJSON(out)
			return nil
		},
	}

	c.AddCommand(searchCmd, storeCmd, addCmd, listCmd, updateCmd, deleteCmd, feedbackCmd)
	return c
}
