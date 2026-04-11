package cmd

import (
	"fmt"
	"log/slog"
	"mairu/internal/agent"
	"mairu/internal/prompts"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	minionMaxRetries int
)

func NewMinionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "minion [prompt]",
		Short: "Run Mairu in unattended, one-shot Minion Mode",
		Long: `Minion Mode executes tasks completely unattended. It will automatically approve shell commands, 
run verification checks, attempt to fix issues (up to --max-retries), and open a Pull Request.
Ideal for executing from background jobs or automation pipelines.`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var prompt string
			if len(args) > 0 {
				prompt = strings.Join(args, " ")
			}

			if minionGithubIssue != "" {
				issueData, err := fetchGitHubContext("issue", minionGithubIssue)
				if err != nil {
					slog.Error("Failed to fetch github issue", "error", err)
					os.Exit(1)
				}
				prompt = fmt.Sprintf("Resolve the following GitHub Issue:\n\n%s\n\nAdditional user prompt: %s", issueData, prompt)
			} else if minionGithubPR != "" {
				prData, err := fetchGitHubContext("pr", minionGithubPR)
				if err != nil {
					slog.Error("Failed to fetch github PR", "error", err)
					os.Exit(1)
				}
				prompt = fmt.Sprintf("Address the following PR feedback:\n\n%s\n\nAdditional user prompt: %s", prData, prompt)
			} else if prompt == "" {
				slog.Error("Either a prompt, --github-issue, or --github-pr is required")
				cmd.Usage()
				os.Exit(1)
			}

			runMinion(prompt)
		},
	}
	cmd.Flags().IntVar(&minionMaxRetries, "max-retries", 2, "Maximum attempts to fix failing tests/linters")
	cmd.Flags().StringVar(&minionGithubIssue, "github-issue", "", "GitHub Issue number to resolve")
	cmd.Flags().StringVar(&minionGithubPR, "github-pr", "", "GitHub PR number to review and fix")
	return cmd
}

var (
	minionGithubIssue string
	minionGithubPR    string
)

func fetchGitHubContext(entityType, number string) (string, error) {
	var cmd *exec.Cmd
	var out []byte
	var err error

	if entityType == "issue" {
		cmd = exec.Command("gh", "issue", "view", number, "--comments")
		out, err = cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("gh issue view failed: %s, output: %s", err, string(out))
		}
		return string(out), nil
	} else {
		cmd = exec.Command("gh", "pr", "view", number, "--comments")
		viewOut, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("gh pr view failed: %s, output: %s", err, string(viewOut))
		}

		diffCmd := exec.Command("gh", "pr", "diff", number)
		diffOut, diffErr := diffCmd.CombinedOutput()
		if diffErr != nil {
			// Don't fail the whole command if diff fails, just return what we have
			slog.Warn("Failed to fetch PR diff", "error", diffErr)
			return string(viewOut), nil
		}

		return fmt.Sprintf("%s\n\n=== PR DIFF ===\n%s", string(viewOut), string(diffOut)), nil
	}
}

func runMinion(userPrompt string) {
	apiKey := GetAPIKey()
	if apiKey == "" {
		slog.Error("Gemini API key not found. Please run 'mairu setup' or set GEMINI_API_KEY environment variable.")
		os.Exit(1)
	}

	cwd, _ := os.Getwd()
	cfg := GetAgentConfig()
	cfg.Unattended = true
	a, err := agent.New(cwd, apiKey, cfg)
	if err != nil {
		slog.Error("Failed to initialize agent", "error", err)
		os.Exit(1)
	}
	defer a.Close()

	// Minion specific instructions wrapping the user prompt
	minionPrompt := prompts.Render("minion_prompt", struct {
		Task       string
		MaxRetries int
	}{
		Task:       userPrompt,
		MaxRetries: minionMaxRetries,
	})

	outChan := make(chan agent.AgentEvent)
	go a.RunStream(minionPrompt, outChan)

	var hasError bool

	for ev := range outChan {
		switch ev.Type {
		case "status":
			fmt.Printf("ℹ️  %s\n", ev.Content)
		case "tool_call":
			fmt.Printf("🔧 Executing: %s\n", ev.ToolName)
		case "tool_result":
			// Output tool results if in verbose mode, or keep it clean
			if verbose {
				fmt.Printf("✅ Tool %s finished\n", ev.ToolName)
			}
		case "text":
			// We can stream text, or just accumulate it. In unattended mode,
			// printing it out directly can provide visibility into the agent's thought process.
			fmt.Print(ev.Content)
		case "diff":
			// Show diffs for context
			fmt.Printf("\n%s\n\n", ev.Content)
		case "error":
			fmt.Printf("\n❌ Error: %s\n", ev.Content)
			hasError = true
		case "done":
			fmt.Println("\n🏁 Minion finished.")
		}
	}

	if hasError {
		os.Exit(1)
	}
}
