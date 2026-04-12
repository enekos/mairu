package cmd

import (
	"fmt"
	"log/slog"
	"mairu/internal/agent"
	"mairu/internal/prompts"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var (
	minionMaxRetries int
	minionCouncil    bool
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
	cmd.Flags().BoolVar(&minionCouncil, "council", false, "Enable council mode with expert reviewers before execution")
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
	cfg := GetAgentConfig()
	cfg.Unattended = true
	cfg.Council.Enabled = minionCouncil
	a, err := agent.New(cwd, providerCfg, cfg)
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

	if !minionCouncil {
		return
	}

	discoveredPR := discoverCurrentPRNumber()
	reviewPR := resolvePRReviewTarget(minionGithubPR, discoveredPR)
	if reviewPR == "" {
		return
	}

	fmt.Printf("\n🔎 Launching PR reviewer council for PR #%s\n", reviewPR)
	reviewOutput, err := runPRReviewerCouncil(cwd, reviewPR)
	if err != nil {
		fmt.Printf("⚠️ PR reviewer council failed: %v\n", err)
		return
	}
	fmt.Println("\n📌 PR reviewer council suggestions:")
	fmt.Println(reviewOutput)
}

type prReviewerRole struct {
	Name  string
	Focus string
}

type prReviewerResult struct {
	role    string
	content string
	err     error
}

func runPRReviewerCouncil(cwd, prNumber string) (string, error) {
	prData, err := fetchGitHubContext("pr", prNumber)
	if err != nil {
		return "", fmt.Errorf("failed to fetch PR context: %w", err)
	}

	roles := []prReviewerRole{
		{
			Name:  "App Developer",
			Focus: "Validate feature behavior and practical product heuristics against surrounding code paths.",
		},
		{
			Name:  "Developer Evangelist",
			Focus: "Review style, architecture consistency, pitfalls, and performance concerns.",
		},
		{
			Name:  "Tests Evangelist",
			Focus: "Check that relevant tests are added or updated, and identify testing gaps.",
		},
	}

	resultsCh := make(chan prReviewerResult, len(roles))
	var wg sync.WaitGroup
	for _, role := range roles {
		wg.Add(1)
		go func(r prReviewerRole) {
			defer wg.Done()
			out, reviewErr := runSinglePRReviewer(cwd, prNumber, prData, r)
			resultsCh <- prReviewerResult{
				role:    r.Name,
				content: out,
				err:     reviewErr,
			}
		}(role)
	}
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	findings := map[string]string{}
	for res := range resultsCh {
		if res.err != nil {
			findings[res.role] = fmt.Sprintf("Reviewer failed: %v", res.err)
			continue
		}
		findings[res.role] = strings.TrimSpace(res.content)
	}

	return runProductLeadSynthesis(cwd, prNumber, findings)
}

func runSinglePRReviewer(cwd string, prNumber, prContext string, role prReviewerRole) (string, error) {
	providerCfg := GetLLMProviderConfig()
	if providerCfg.APIKey == "" {
		return "", fmt.Errorf("API key not found")
	}

	cfg := GetAgentConfig()
	cfg.Unattended = true
	cfg.Council.Enabled = false

	a, err := agent.New(cwd, providerCfg, cfg)
	if err != nil {
		return "", err
	}
	defer a.Close()

	prompt := prompts.Render("council_pr_reviewer_expert", struct {
		PRNumber string
		PRData   string
		Role     string
		Focus    string
	}{
		PRNumber: prNumber,
		PRData:   prContext,
		Role:     role.Name,
		Focus:    role.Focus,
	})

	outChan := make(chan agent.AgentEvent, 100)
	go a.RunStream(prompt, outChan)

	var b strings.Builder
	var eventErr error
	for ev := range outChan {
		if ev.Type == "text" {
			b.WriteString(ev.Content)
		}
		if ev.Type == "error" {
			eventErr = fmt.Errorf("%s", ev.Content)
		}
	}
	if eventErr != nil {
		return "", eventErr
	}
	return strings.TrimSpace(b.String()), nil
}

func runProductLeadSynthesis(cwd string, prNumber string, findings map[string]string) (string, error) {
	providerCfg := GetLLMProviderConfig()
	if providerCfg.APIKey == "" {
		return "", fmt.Errorf("API key not found")
	}

	cfg := GetAgentConfig()
	cfg.Unattended = true
	cfg.Council.Enabled = false

	a, err := agent.New(cwd, providerCfg, cfg)
	if err != nil {
		return "", err
	}
	defer a.Close()

	prompt := prompts.Render("council_pr_reviewer_product_lead", struct {
		PRNumber string
		Findings string
	}{
		PRNumber: prNumber,
		Findings: formatReviewerFindings(findings),
	})

	outChan := make(chan agent.AgentEvent, 100)
	go a.RunStream(prompt, outChan)

	var b strings.Builder
	var eventErr error
	for ev := range outChan {
		if ev.Type == "text" {
			b.WriteString(ev.Content)
		}
		if ev.Type == "error" {
			eventErr = fmt.Errorf("%s", ev.Content)
		}
	}
	if eventErr != nil {
		return "", eventErr
	}
	return strings.TrimSpace(b.String()), nil
}

func discoverCurrentPRNumber() string {
	cmd := exec.Command("gh", "pr", "view", "--json", "number", "--jq", ".number")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func resolvePRReviewTarget(explicitPR, discoveredPR string) string {
	if strings.TrimSpace(explicitPR) != "" {
		return strings.TrimSpace(explicitPR)
	}
	return strings.TrimSpace(discoveredPR)
}

func formatReviewerFindings(findings map[string]string) string {
	roles := make([]string, 0, len(findings))
	for role := range findings {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	var b strings.Builder
	for _, role := range roles {
		b.WriteString("## ")
		b.WriteString(role)
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(findings[role]))
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}
