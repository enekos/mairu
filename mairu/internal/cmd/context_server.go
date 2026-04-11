package cmd

import (
	"context"
	"log/slog"
	"mairu/internal/contextsrv"
	"mairu/internal/logger"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

func NewContextServerCmd() *cobra.Command {
	cmd := &cobra.Command{
	Use:   "context-server",
	Short: "Start centralized context management server",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		sqliteDSN, _ := cmd.Flags().GetString("sqlite-dsn")
		authToken, _ := cmd.Flags().GetString("auth-token")
		enableProjector, _ := cmd.Flags().GetBool("projector")
		meiliURL, _ := cmd.Flags().GetString("meili-url")
		meiliAPIKey, _ := cmd.Flags().GetString("meili-api-key")

		appCfg := GetConfig()

		cfg := contextsrv.Config{
			Port:              appCfg.Server.Port,
			SQLiteDSN:         appCfg.Server.SQLiteDSN,
			MeiliURL:          appCfg.API.MeiliURL,
			MeiliAPIKey:       appCfg.API.MeiliAPIKey,
			GeminiAPIKey:      appCfg.API.GeminiAPIKey,
			AuthToken:         appCfg.Server.AuthToken,
			EnableProjector:   enableProjector,
			ProjectorBatch:    appCfg.Server.ProjectorBatch,
			ModerationEnabled: appCfg.Server.ModerationEnabled,
			EmbeddingModel:    appCfg.Embedding.Model,
			EmbeddingDim:      appCfg.Embedding.Dimensions,
		}

		// Parse projector interval from config
		if d, err := time.ParseDuration(appCfg.Server.ProjectorInterval); err == nil {
			cfg.ProjectorEvery = d
		} else {
			cfg.ProjectorEvery = 3 * time.Second
		}
		// Parse read timeout from config
		if d, err := time.ParseDuration(appCfg.Server.ReadTimeout); err == nil {
			cfg.ReadTimeout = d
		} else {
			cfg.ReadTimeout = 10 * time.Second
		}

		// CLI flag overrides
		if cmd.Flags().Changed("port") {
			cfg.Port = port
		}
		if sqliteDSN != "" {
			cfg.SQLiteDSN = sqliteDSN
		}
		if meiliURL != "" {
			cfg.MeiliURL = meiliURL
		}
		if meiliAPIKey != "" {
			cfg.MeiliAPIKey = meiliAPIKey
		}
		if authToken != "" {
			cfg.AuthToken = authToken
		}

		logger.Init(logger.Config{
			Level:      "info",
			Structured: true,
		})

		app, err := contextsrv.NewApp(cfg)
		if err != nil {
			slog.Error("failed to initialize context server", "error", err)
			os.Exit(1)
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		go func() {
			<-ctx.Done()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			_ = app.Shutdown(shutdownCtx)
		}()

		slog.Info("Starting context server", "port", cfg.Port)
		if err := app.Start(ctx); err != nil && err.Error() != "http: Server closed" {
			slog.Error("context server exited with error", "error", err)
			os.Exit(1)
		}
	},
}
	cmd.Flags().IntP("port", "p", 8788, "Port to run context server on")
	cmd.Flags().String("sqlite-dsn", os.Getenv("CONTEXT_SERVER_SQLITE_DSN"), "SQLite DSN")
	cmd.Flags().String("auth-token", os.Getenv("CONTEXT_SERVER_AUTH_TOKEN"), "Shared token for internal auth (X-Context-Token)")
	cmd.Flags().Bool("projector", true, "Enable SQLite->Meilisearch outbox projector")
	cmd.Flags().String("meili-url", os.Getenv("MEILI_URL"), "Meilisearch URL")
	cmd.Flags().String("meili-api-key", os.Getenv("MEILI_API_KEY"), "Meilisearch API key")
	return cmd
}


