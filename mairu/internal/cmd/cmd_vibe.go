package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func runVibeMutation(project, prompt string, k int) error {
	planOut, err := ContextPost("/api/vibe/mutation/plan", map[string]any{
		"prompt":  prompt,
		"project": project,
		"topK":    k,
	})
	if err != nil {
		return err
	}
	var plan struct {
		Operations []map[string]any `json:"operations"`
	}
	if err := json.Unmarshal(planOut, &plan); err != nil {
		return err
	}
	execOut, err := ContextPost("/api/vibe/mutation/execute", map[string]any{
		"project":    project,
		"operations": plan.Operations,
	})
	if err != nil {
		return err
	}
	PrintJSON(execOut)
	return nil
}
func NewVibeCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "vibe",
		Short: "ContextFS vibe operations",
	}
	cmd.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	queryCmd := &cobra.Command{
		Use:   "query <prompt>",
		Short: "Run vibe query",
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
	queryCmd.Flags().IntP("k", "k", 5, "Top K results")
	queryCmd.Aliases = []string{"summarize"}

	mutationCmd := &cobra.Command{
		Use:   "mutation [prompt]",
		Short: "Plan and execute vibe mutation",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("prompt is required")
			}
			k, _ := cmd.Flags().GetInt("k")
			return runVibeMutation(project, args[0], k)
		},
	}
	mutationCmd.Flags().IntP("k", "k", 5, "Top K results")
	mutationCmd.Aliases = []string{"flush", "nudge"}

	cmd.AddCommand(queryCmd, mutationCmd)
	return cmd
}
func NewVibeQueryAliasCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "vibe-query <prompt>",
		Short: "Alias for vibe query",
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
func NewVibeMutationAliasCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "vibe-mutation [prompt]",
		Short: "Alias for vibe mutation",
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
