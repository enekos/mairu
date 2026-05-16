package cmd

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"mairu/internal/contextsrv"
	"mairu/internal/trace"
)

type localServiceFactory struct {
	mu  sync.Mutex
	app *contextsrv.App
}

func (f *localServiceFactory) init() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.app != nil {
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
		GeminiAPIKey:      appCfg.API.GeminiAPIKey,
		KimiAPIKey:        appCfg.API.KimiAPIKey,
		LLMProvider:       appCfg.LLM.Provider,
		LLMModel:          appCfg.LLM.Model,
		LLMBaseURL:        appCfg.LLM.BaseURL,
		AuthToken:         appCfg.Server.AuthToken,
		EnableProjector:   false, // CLI tools generally shouldn't run background loop
		ProjectorBatch:    appCfg.Server.ProjectorBatch,
		ModerationEnabled: appCfg.Server.ModerationEnabled,
		EmbeddingProvider: appCfg.Embedding.Provider,
		EmbeddingBaseURL:  appCfg.Embedding.BaseURL,
		EmbeddingAPIKey:   appCfg.Embedding.APIKey,
		EmbeddingModel:    appCfg.Embedding.Model,
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

	f.app = app

	if traceEnabled() && cfg.MeiliURL != "" {
		trace.SetDefault(trace.NewMeiliRecorder(cfg.MeiliURL, cfg.MeiliAPIKey))
	}
}

func traceEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("MAIRU_TRACE")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (f *localServiceFactory) handler() http.Handler {
	f.init()
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.app.Handler()
}

func (f *localServiceFactory) close() {
	f.mu.Lock()
	app := f.app
	f.app = nil
	f.mu.Unlock()
	if app != nil {
		importCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := app.Flush(importCtx); err != nil {
			slog.Warn("Failed to flush local service outbox", "error", err)
		}

		_ = app.Shutdown(importCtx)
	}
}

func (f *localServiceFactory) getApp() *contextsrv.App {
	f.init()
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.app
}

var localFactory localServiceFactory

func getLocalHandler() http.Handler {
	return localFactory.handler()
}

func closeLocalService() {
	localFactory.close()
}

// GetLocalApp returns the underlying App instance for accessing the SymbolLocator.
func GetLocalApp() *contextsrv.App {
	return localFactory.getApp()
}
