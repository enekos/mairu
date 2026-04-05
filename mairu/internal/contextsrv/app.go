package contextsrv

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"mairu/internal/llm"
)

// Config defines configuration parameters for starting the Context Server.
// It includes server settings, database connections, API keys, and projector configuration.
type Config struct {
	Port              int
	PostgresDSN       string
	MeiliURL          string
	MeiliAPIKey       string
	GeminiAPIKey      string
	AuthToken         string
	EnableProjector   bool
	ProjectorEvery    time.Duration
	ProjectorBatch    int
	ModerationEnabled bool
}

// LoadConfig creates a Config with defaults and environment variables overrides.
func LoadConfig() Config {
	cfg := Config{
		Port:              8788,
		PostgresDSN:       os.Getenv("CONTEXT_SERVER_POSTGRES_DSN"),
		MeiliURL:          os.Getenv("MEILI_URL"),
		MeiliAPIKey:       os.Getenv("MEILI_API_KEY"),
		GeminiAPIKey:      os.Getenv("GEMINI_API_KEY"),
		AuthToken:         os.Getenv("CONTEXT_AUTH_TOKEN"),
		ProjectorEvery:    3 * time.Second,
		ProjectorBatch:    50,
		ModerationEnabled: os.Getenv("CONTEXT_ENABLE_MODERATION") == "true",
	}
	if cfg.MeiliURL == "" {
		cfg.MeiliURL = "http://localhost:7700"
	}
	if cfg.MeiliAPIKey == "" {
		cfg.MeiliAPIKey = "contextfs-dev-key"
	}
	return cfg
}

// App represents the Context Server application instance.
// It bundles the configuration, repository, projector, and HTTP server to manage the application lifecycle.
type App struct {
	cfg       Config
	repo      *PostgresRepository
	projector *Projector
	server    *http.Server
}

// NewApp initializes and returns a new App instance using the provided Config.
// It sets up the repository, LLM provider, indexing service, handler, and an HTTP server.
func NewApp(cfg Config) (*App, error) {
	var repo *PostgresRepository
	var err error
	if cfg.PostgresDSN != "" {
		repo, err = NewPostgresRepository(cfg.PostgresDSN)
		if err != nil {
			return nil, err
		}
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

	var repoIntf Repository
	if repo != nil {
		repoIntf = repo
	}

	svc := NewServiceWithSearch(repoIntf, indexer, llmClient, cfg.ModerationEnabled)
	handler := NewHandler(svc, cfg.AuthToken)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	var projector *Projector
	if repo != nil {
		projector = NewProjector(repo, indexer, geminiClient)
	}

	return &App{
		cfg:       cfg,
		repo:      repo,
		projector: projector,
		server:    srv,
	}, nil
}

// Start begins the execution of the application.
// It starts the background projector (if enabled) and listens on the configured HTTP port.
func (a *App) Start(ctx context.Context) error {
	if a.cfg.EnableProjector && a.projector != nil {
		go a.runProjector(ctx)
	}
	return a.server.ListenAndServe()
}

// runProjector runs an infinite loop, triggering the projector periodically
// according to the configuration. It exits when the provided context is done.
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

// Shutdown gracefully stops the server and closes the underlying repository.
func (a *App) Shutdown(ctx context.Context) error {
	if a.repo != nil {
		_ = a.repo.Close()
	}
	return a.server.Shutdown(ctx)
}
