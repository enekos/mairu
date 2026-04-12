package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"mairu/internal/agent"
	"mairu/internal/tui"
)

var sessionName string

func NewTuiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Start the Mairu interactive terminal session",
		Run: func(cmd *cobra.Command, args []string) {
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

			if sessionName != "" {
				if err := a.LoadSession(sessionName); err != nil {
					slog.Warn("Failed to load session", "session", sessionName, "error", err)
				}
			}

			if err := tui.Start(a, sessionName); err != nil {
				slog.Error("TUI Error", "error", err)
				os.Exit(1)
			}

			if sessionName != "" {
				a.SaveSession(sessionName)
			}
		},
	}
	cmd.Flags().StringVarP(&sessionName, "session", "s", "", "Load/Save a named session (e.g. 'bug-123')")
	return cmd
}
