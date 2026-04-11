package cmd

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"mairu/internal/contextsrv"
)

var localApp *contextsrv.App

func initLocalApp() {
	if localApp != nil {
		return
	}

	appCfg := GetConfig()
	if appCfg == nil {
		slog.Error("Config not initialized")
		os.Exit(1)
	}

	cfg := contextsrv.Config{
		Port:              appCfg.Server.Port,
		SQLiteDSN:         appCfg.Server.SQLiteDSN,
		MeiliURL:          appCfg.API.MeiliURL,
		MeiliAPIKey:       appCfg.API.MeiliAPIKey,
		GeminiAPIKey:      GetAPIKey(), // use shared api key resolution
		AuthToken:         appCfg.Server.AuthToken,
		EnableProjector:   false, // CLI tools generally shouldn't run background loop
		ProjectorBatch:    appCfg.Server.ProjectorBatch,
		ModerationEnabled: appCfg.Server.ModerationEnabled,
		EmbeddingModel:    appCfg.Embedding.Model,
		EmbeddingDim:      appCfg.Embedding.Dimensions,
	}

	if d, err := time.ParseDuration(appCfg.Server.ReadTimeout); err == nil {
		cfg.ReadTimeout = d
	} else {
		cfg.ReadTimeout = 10 * time.Second
	}

	app, err := contextsrv.NewApp(cfg)
	if err != nil {
		slog.Error("failed to initialize context service", "error", err)
		os.Exit(1)
	}

	localApp = app
}

func getLocalService() contextsrv.Service {
	initLocalApp()
	return localApp.Service()
}

func getLocalHandler() http.Handler {
	initLocalApp()
	return localApp.Handler()
}

func closeLocalService() {
	if localApp != nil {
		// Increase timeout to 30s to allow more items to be embedded
		importCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := localApp.Flush(importCtx); err != nil {
			slog.Warn("Failed to flush local service outbox", "error", err)
		}

		_ = localApp.Shutdown(importCtx)
		localApp = nil
	}
}

// GetLocalApp returns the underlying App instance for accessing the SymbolLocator.
func GetLocalApp() *contextsrv.App {
	initLocalApp()
	return localApp
}
