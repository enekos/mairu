//go:build !slim && !contextsrvonly

package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"mairu/internal/acp"
	"mairu/internal/agent"
)

// NewACPCmd returns the `mairu acp` subcommand which speaks the Agent Client
// Protocol on stdio. Use it to run mairu as a first-class agent inside ACP
// hosts (Zed, Neovim CodeCompanion, etc.).
func NewACPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "acp",
		Short: "Run mairu as an Agent Client Protocol server on stdio",
		Long: `Start an Agent Client Protocol (ACP) server speaking newline-delimited
JSON-RPC 2.0 on stdin/stdout. Logs go to stderr.

Wire it up in Zed's settings.json:

  {
    "agent_servers": {
      "mairu": {
        "command": "mairu",
        "args": ["acp"]
      }
    }
  }
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			providerCfg := GetLLMProviderConfig()
			// Defer GetAgentConfig() until session/new — it touches the local
			// context service (embedder, Meilisearch) which we don't want to
			// require at process startup.
			build := func(cwd string) (*agent.Agent, error) {
				return agent.New(cwd, providerCfg, GetAgentConfig())
			}
			srv := acp.New(providerCfg, build)
			return srv.Run(ctx, os.Stdin, os.Stdout)
		},
	}
}
