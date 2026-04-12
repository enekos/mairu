package cmd

import (
	"fmt"
	"log/slog"
	"mairu/internal/logger"
	"mairu/internal/web"
	"os"

	"github.com/spf13/cobra"
)

func NewWebCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the Mairu web interface",
		Run: func(cmd *cobra.Command, args []string) {
			logger.Init(logger.Config{
				Level:      "info",
				Structured: true,
			})

			appCfg := GetConfig()
			port := appCfg.Server.Port
			if cmd.Flags().Changed("port") {
				port, _ = cmd.Flags().GetInt("port")
			}

			providerCfg := GetLLMProviderConfig()
			if providerCfg.APIKey == "" {
				fmt.Fprintln(os.Stderr, NewCLIError(nil, "Run 'mairu setup' or set api key in config", "API key not found"))
				os.Exit(1)
			}

			slog.Info("Starting Mairu web interface", "port", port)
			if err := web.StartServer(port, providerCfg, getLocalHandler(), GetLocalApp().SymbolLocator()); err != nil {
				slog.Error("Error starting web server", "error", err)
			}
		},
	}
	cmd.Flags().IntP("port", "p", 8080, "Port to run the web server on")
	return cmd
}
