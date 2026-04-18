package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"mairu/internal/contextsrv"
	"mairu/internal/history"
	"mairu/internal/redact"
)

func NewHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Manage and search your bash command history",
	}

	searchCmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Semantically search past bash commands and outputs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			if project == "" {
				project, _ = os.Getwd()
			}
			topK, _ := cmd.Flags().GetInt("top")

			app := GetLocalApp()
			svc := app.Service()

			opts := contextsrv.SearchOptions{
				Query:   args[0],
				Project: project,
				Store:   "bash_history",
				TopK:    topK,
			}

			res, err := svc.Search(opts)
			if err != nil {
				return err
			}

			historyRaw, ok := res["bashHistory"].([]map[string]any)
			if !ok || len(historyRaw) == 0 {
				fmt.Println("No matching commands found.")
				return nil
			}

			formatter := GetFormatter()
			formatter.PrintJSON(historyRaw)
			return nil
		},
	}
	searchCmd.Flags().IntP("top", "n", 5, "Number of results")
	searchCmd.Flags().StringP("project", "P", "", "Filter by project")

	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show the most frequently run bash commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			if project == "" {
				project, _ = os.Getwd()
			}
			limit, _ := cmd.Flags().GetInt("limit")

			app := GetLocalApp()
			repo := app.Repo()
			if repo == nil {
				return fmt.Errorf("repository is not initialized")
			}

			stats, err := repo.GetBashStats(context.Background(), project, limit)
			if err != nil {
				return err
			}

			formatter := GetFormatter()
			formatter.PrintJSON(stats)
			return nil
		},
	}
	statsCmd.Flags().IntP("limit", "n", 10, "Number of stats to show")
	statsCmd.Flags().StringP("project", "P", "", "Filter by project")

	feedbackCmd := &cobra.Command{
		Use:   "feedback <id> <reward>",
		Short: "Apply reinforcement learning feedback to a bash history item (reward 1-10)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			reward, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("reward must be an integer between 1 and 10")
			}
			out, err := ContextPost("/api/bash/feedback", map[string]any{
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

	cmd.AddCommand(searchCmd, statsCmd, feedbackCmd, newHistoryImportCmd())
	return cmd
}

func newHistoryImportCmd() *cobra.Command {
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Backfill bash history from a shell history file (~/.zsh_history, ~/.bash_history)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			from, _ := cmd.Flags().GetString("from")
			project, _ := cmd.Flags().GetString("project")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			formatFlag, _ := cmd.Flags().GetString("format")

			if from == "" {
				return fmt.Errorf("--from is required")
			}
			if project == "" {
				return fmt.Errorf("--project is required")
			}

			var format history.Format
			switch formatFlag {
			case "":
				format = history.DetectFormat(from)
			case "zsh":
				format = history.FormatZsh
			case "bash":
				format = history.FormatBash
			default:
				return fmt.Errorf("unknown --format %q (want zsh or bash)", formatFlag)
			}

			f, err := os.Open(from)
			if err != nil {
				return fmt.Errorf("open history file: %w", err)
			}
			defer f.Close()

			app := GetLocalApp()
			repo := app.Repo()
			if repo == nil {
				return fmt.Errorf("repository is not initialized")
			}

			res, err := history.Import(context.Background(), f, format, repo, redact.New(), project, dryRun)
			if err != nil {
				return err
			}

			summary := fmt.Sprintf("parsed=%d redacted=%d dropped=%d dup=%d stored=%d",
				res.Parsed, res.Redacted, res.Dropped, res.DuplicateSkipped, res.Stored)
			if dryRun {
				summary += " (dry-run)"
			}
			fmt.Println(summary)
			return nil
		},
	}
	importCmd.Flags().String("from", "", "Path to shell history file (required)")
	importCmd.Flags().StringP("project", "P", "", "Project name to associate entries with (required)")
	importCmd.Flags().Bool("dry-run", false, "Parse and redact but do not write to the repository")
	importCmd.Flags().String("format", "", "Force format: zsh or bash (default: detect from filename)")
	return importCmd
}
