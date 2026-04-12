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
		RunE: func(cmd *cobra.Command, args []string) error {
			providerCfg := GetLLMProviderConfig()
			if providerCfg.APIKey == "" {
				providerName := providerCfg.Type
				if providerName == "" {
					providerName = "gemini"
				}
				return fmt.Errorf("%s API key not found. Please run 'mairu setup' or set the appropriate API key environment variable", providerName)
			}

			cwd, _ := os.Getwd()
			a, err := agent.New(cwd, providerCfg, GetAgentConfig())
			if err != nil {
				return fmt.Errorf("failed to initialize agent: %w", err)
			}
			defer a.Close()

			if sessionName != "" {
				if err := a.LoadSession(sessionName); err != nil {
					slog.Warn("Failed to load session", "session", sessionName, "error", err)
				}
			}

			if err := tui.Start(a, sessionName); err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}

			if sessionName != "" {
				a.SaveSession(sessionName)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&sessionName, "session", "s", "", "Load/Save a named session (e.g. 'bug-123')")
	return cmd
}
