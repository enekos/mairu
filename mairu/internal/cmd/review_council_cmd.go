//go:build !contextsrvonly

package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

func NewReviewCouncilCmd() *cobra.Command {
	var githubPR string
	cmd := &cobra.Command{
		Use:   "review-council",
		Short: "Run a council of expert reviewers on a pull request",
		Long: `Runs the Mairu PR reviewer council against a GitHub pull request.
Spawns expert reviewers and synthesizes their findings through a Product Lead.
Outputs the final synthesized review to stdout.`,
		Run: func(cmd *cobra.Command, args []string) {
			if githubPR == "" {
				slog.Error("--github-pr is required")
				os.Exit(1)
			}

			providerCfg := GetLLMProviderConfig()
			if providerCfg.APIKey == "" {
				providerName := providerCfg.Type
				if providerName == "" {
					providerName = "gemini"
				}
				slog.Error(fmt.Sprintf("%s API key not found. Please run 'mairu setup' or set the appropriate API key environment variable.", providerName))
				os.Exit(1)
			}

			cwd, _ := os.Getwd()
			fmt.Fprintf(os.Stderr, "Launching PR reviewer council for PR #%s\n", githubPR)
			reviewOutput, err := runPRReviewerCouncil(cwd, githubPR)
			if err != nil {
				slog.Error("PR reviewer council failed", "error", err)
				os.Exit(1)
			}
			fmt.Println(reviewOutput)
		},
	}
	cmd.Flags().StringVar(&githubPR, "github-pr", "", "GitHub PR number to review")
	return cmd
}
