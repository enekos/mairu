package cmd

import (
	_ "embed"
	"fmt"
	"log/slog"
	"mairu/internal/agent"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed minion_prompt.md
var minionPromptTemplate string

var (
	minionMaxRetries int
)

var minionCmd = &cobra.Command{
	Use:   "minion [prompt]",
	Short: "Run Mairu in unattended, one-shot Minion Mode",
	Long: `Minion Mode executes tasks completely unattended. It will automatically approve shell commands, 
run verification checks, attempt to fix issues (up to --max-retries), and open a Pull Request.
Ideal for executing from background jobs or automation pipelines.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		prompt := strings.Join(args, " ")
		runMinion(prompt)
	},
}

func init() {
	minionCmd.Flags().IntVar(&minionMaxRetries, "max-retries", 2, "Maximum attempts to fix failing tests/linters")
}

func runMinion(userPrompt string) {
	apiKey := GetAPIKey()
	if apiKey == "" {
		slog.Error("Gemini API key not found. Please run 'mairu setup' or set GEMINI_API_KEY environment variable.")
		os.Exit(1)
	}

	cwd, _ := os.Getwd()
	a, err := agent.New(cwd, apiKey, agent.Config{
		Unattended: true,
	})
	if err != nil {
		slog.Error("Failed to initialize agent", "error", err)
		os.Exit(1)
	}
	defer a.Close()

	// Minion specific instructions wrapping the user prompt
	minionPrompt := fmt.Sprintf(minionPromptTemplate, userPrompt, minionMaxRetries)

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
