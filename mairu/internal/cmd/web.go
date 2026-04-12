package cmd

import (
	"fmt"
	"log/slog"
	"mairu/internal/logger"
	"mairu/internal/web"

	"github.com/spf13/cobra"
)

func NewWebCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the Mairu web interface",
		RunE: func(cmd *cobra.Command, args []string) error {
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
				return fmt.Errorf("API key not found. Run 'mairu setup' or set api key in config")
			}

			slog.Info("Starting Mairu web interface", "port", port)
			if err := web.StartServer(port, providerCfg, getLocalHandler(), GetLocalApp().SymbolLocator()); err != nil {
				return fmt.Errorf("error starting web server: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().IntP("port", "p", 8080, "Port to run the web server on")
	return cmd
}
