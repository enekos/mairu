//go:build !contextsrvonly

package cmd

import (
	"fmt"
	"log/slog"
	"mairu/internal/agent"
	"os"

	"github.com/spf13/cobra"
)

var agentSystemData map[string]any

func init() {
	oldPreRun := rootCmd.PersistentPreRun
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		oldPreRun(cmd, args)
		agentSystemData = map[string]any{
			"CliHelp": GenerateAgentCLIRef(cmd.Root()),
		}
	}
}

func runHeadless(prompt string) {
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
	a, err := agent.New(cwd, providerCfg, GetAgentConfig())
	if err != nil {
		slog.Error("Failed to initialize agent", "error", err)
		os.Exit(1)
	}
	defer a.Close()

	response, err := a.Run(prompt)
	if err != nil {
		slog.Error("Agent error", "error", err)
		os.Exit(1)
	}

	fmt.Println("\n" + response)
}

// GetAgentConfig returns a populated agent.Config using the global config and contextsrv.App
func GetAgentConfig() agent.Config {
	cfg := GetConfig()
	var interceptors []agent.ToolInterceptor
	if cfg != nil {
		interceptors = append(interceptors, &agent.SecurityFilter{
			BlockedCommands: cfg.Security.BlockedCommands,
			BlockedPaths:    cfg.Security.BlockedPaths,
		})
	}

	app := GetLocalApp()
	var repo agent.HistoryLogger
	if app != nil {
		repo = app.Repo()
	}

	// CLI --redact takes precedence over config file; env var (handled in
	// agent.ResolveRedactBashOutput) is evaluated last inside agent.New.
	redactBash := cfg.Agent.RedactBashOutput
	if redactBashFlag {
		redactBash = true
	}

	return agent.Config{
		SymbolLocator:    GetLocalApp().SymbolLocator(),
		HistoryLogger:    repo,
		Interceptors:     interceptors,
		UTCPProviders:    cfg.Tools.UTCPProviders,
		AgentSystemData:  agentSystemData,
		RedactBashOutput: redactBash,
	}
}
