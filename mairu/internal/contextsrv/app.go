package contextsrv

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"mairu/internal/llm"
)

type Config struct {
	Port            int
	PostgresDSN     string
	MeiliURL        string
	MeiliAPIKey     string
	GeminiAPIKey    string
	AuthToken       string
	EnableProjector bool
	ProjectorEvery  time.Duration
	ProjectorBatch  int
}

// LoadConfig creates a Config with defaults and environment variables overrides.
func LoadConfig() Config {
	cfg := Config{
		Port:           8788,
		PostgresDSN:    os.Getenv("CONTEXT_SERVER_POSTGRES_DSN"),
		MeiliURL:       os.Getenv("MEILI_URL"),
		MeiliAPIKey:    os.Getenv("MEILI_API_KEY"),
		GeminiAPIKey:   os.Getenv("GEMINI_API_KEY"),
		AuthToken:      os.Getenv("CONTEXT_AUTH_TOKEN"),
		ProjectorEvery: 3 * time.Second,
		ProjectorBatch: 50,
	}
	if cfg.MeiliURL == "" {
		cfg.MeiliURL = "http://localhost:7700"
	}
	return cfg
}

type App struct {
	cfg       Config
	repo      *PostgresRepository
	projector *Projector
	server    *http.Server
}

func NewApp(cfg Config) (*App, error) {
	if cfg.PostgresDSN == "" {
		return nil, fmt.Errorf("PostgresDSN is required")
	}

	repo, err := NewPostgresRepository(cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}

	var geminiClient *llm.GeminiProvider
	if cfg.GeminiAPIKey != "" {
		client, err := llm.NewGeminiProvider(context.Background(), cfg.GeminiAPIKey)
		if err == nil {
			geminiClient = client
		}
	}

	indexer := NewMeiliIndexer(cfg.MeiliURL, cfg.MeiliAPIKey, geminiClient)
	_ = indexer.EnsureIndexes()

	var llmClient LLMClient
	if geminiClient != nil {
		llmClient = geminiClient
	}

	svc := NewServiceWithSearch(repo, indexer, llmClient)
	handler := NewHandler(svc, cfg.AuthToken)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &App{
		cfg:       cfg,
		repo:      repo,
		projector: NewProjector(repo, indexer, geminiClient),
		server:    srv,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	if a.cfg.EnableProjector {
		go a.runProjector(ctx)
	}
	return a.server.ListenAndServe()
}

func (a *App) runProjector(ctx context.Context) {
	t := time.NewTicker(a.cfg.ProjectorEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = a.projector.RunOnce(ctx, a.cfg.ProjectorBatch)
		}
	}
}

func (a *App) Shutdown(ctx context.Context) error {
	_ = a.repo.Close()
	return a.server.Shutdown(ctx)
}
