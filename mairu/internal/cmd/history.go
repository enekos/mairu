package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"mairu/internal/contextsrv"
)

func newHistoryCmd() *cobra.Command {
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

	cmd.AddCommand(searchCmd, statsCmd)
	return cmd
}
