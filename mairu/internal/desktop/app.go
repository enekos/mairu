package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mairu/internal/agent"
	"mairu/internal/config"
	"mairu/internal/contextsrv"
	"mairu/internal/logger"

	"github.com/gen2brain/beeep"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type WindowState struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (a *App) windowStatePath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".mairu", "window-state.json")
}

func (a *App) LoadWindowState() *WindowState {
	data, err := os.ReadFile(a.windowStatePath())
	if err != nil {
		return nil
	}
	var state WindowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	return &state
}

func (a *App) SaveWindowState() {
	x, y := wailsRuntime.WindowGetPosition(a.ctx)
	w, h := wailsRuntime.WindowGetSize(a.ctx)
	state := WindowState{X: x, Y: y, Width: w, Height: h}
	data, _ := json.Marshal(state)
	_ = os.MkdirAll(filepath.Dir(a.windowStatePath()), 0o755)
	_ = os.WriteFile(a.windowStatePath(), data, 0o644)
}

// ShowNotification sends an OS notification.
func (a *App) ShowNotification(title, body string) {
	_ = beeep.Notify(title, body, "")
}

type WailsLogHandler struct {
	ctx context.Context
}

func (h *WailsLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h *WailsLogHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.ctx != nil {
		attrs := make(map[string]any)
		r.Attrs(func(a slog.Attr) bool {
			attrs[a.Key] = a.Value.Any()
			return true
		})

		logEntry := map[string]any{
			"time":    r.Time.Format(time.RFC3339),
			"level":   r.Level.String(),
			"message": r.Message,
			"attrs":   attrs,
		}
		wailsRuntime.EventsEmit(h.ctx, "sys:log", logEntry)
	}
	return nil
}

func (h *WailsLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h // Not fully implemented for simplicity
}

func (h *WailsLogHandler) WithGroup(name string) slog.Handler {
	return h // Not fully implemented for simplicity
}

// App is the Wails-bound application struct.
// All exported methods become callable from the frontend via window.go.desktop.App.
type App struct {
	ctx        context.Context
	svc        contextsrv.Service
	meili      *MeiliManager
	cfg        *config.Config
	agentsMu   sync.Mutex
	agents     map[string]*agent.Agent // session name → active agent
	meiliReady chan struct{}
}

// NewApp creates an uninitialized App. Call startup() to wire services.
func NewApp() *App {
	return &App{
		agents:     make(map[string]*agent.Agent),
		meiliReady: make(chan struct{}),
	}
}

// Startup is called by Wails when the app window is ready.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// Setup Wails logger
	wailsHandler := &WailsLogHandler{ctx: ctx}
	logger.GlobalHandlers = append(logger.GlobalHandlers, wailsHandler)
	logger.Setup(true) // Enable debug by default for desktop

	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("failed to get home dir", "error", err)
		return
	}

	cfg, err := config.Load(".")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		return
	}
	a.cfg = cfg

	// Start managed Meilisearch
	a.meili = NewMeiliManager(filepath.Join(homeDir, ".mairu", "meilisearch"))

	if !a.meili.IsInstalled() {
		wailsRuntime.EventsEmit(ctx, "app:status", "Downloading Meilisearch...")
		if err := a.meili.Download(ctx, func(pct int) {
			wailsRuntime.EventsEmit(ctx, "app:download-progress", pct)
		}); err != nil {
			wailsRuntime.EventsEmit(ctx, "app:error", fmt.Sprintf("Failed to download Meilisearch: %v", err))
			return
		}
	}

	wailsRuntime.EventsEmit(ctx, "app:status", "Starting Meilisearch...")
	if err := a.meili.Start(ctx); err != nil {
		wailsRuntime.EventsEmit(ctx, "app:error", fmt.Sprintf("Failed to start Meilisearch: %v", err))
		return
	}

	// Wire the contextsrv.Service using the managed Meilisearch
	svcCfg := contextsrv.Config{
		Port:              0, // not used — no HTTP server in desktop mode
		SQLiteDSN:         cfg.Server.SQLiteDSN,
		MeiliURL:          a.meili.URL(),
		MeiliAPIKey:       a.meili.APIKey(),
		GeminiAPIKey:      cfg.API.GeminiAPIKey,
		ModerationEnabled: cfg.Server.ModerationEnabled,
		OllamaURL:         cfg.Embedding.OllamaURL,
		EmbeddingModel:    cfg.Embedding.Model,
	}

	ctxApp, err := contextsrv.NewApp(svcCfg)
	if err != nil {
		wailsRuntime.EventsEmit(ctx, "app:error", fmt.Sprintf("Failed to init service: %v", err))
		return
	}
	a.svc = ctxApp.Service()

	close(a.meiliReady)

	a.ShowNotification("Mairu", "Meilisearch is ready")
	wailsRuntime.EventsEmit(ctx, "app:ready", true)
	a.SetupTray()

	if state := a.LoadWindowState(); state != nil {
		wailsRuntime.WindowSetPosition(a.ctx, state.X, state.Y)
		wailsRuntime.WindowSetSize(a.ctx, state.Width, state.Height)
	}
}

// Shutdown is called by Wails when the window is closing.
func (a *App) Shutdown(ctx context.Context) {
	a.SaveWindowState()
	if a.meili != nil {
		if err := a.meili.Stop(); err != nil {
			slog.Error("failed to stop meilisearch", "error", err)
		}
	}
}

// Ping is a simple health check binding.
func (a *App) Ping() string {
	return "pong"
}

// ── Memory bindings ─────────────────────────────────────────────

func (a *App) ListMemories(project string, limit int) ([]contextsrv.Memory, error) {
	return a.svc.ListMemories(project, limit)
}

func (a *App) CreateMemory(input contextsrv.MemoryCreateInput) (contextsrv.Memory, error) {
	return a.svc.CreateMemory(input)
}

func (a *App) UpdateMemory(input contextsrv.MemoryUpdateInput) (contextsrv.Memory, error) {
	return a.svc.UpdateMemory(input)
}

func (a *App) DeleteMemory(id string) error {
	return a.svc.DeleteMemory(id)
}

func (a *App) ApplyMemoryFeedback(id string, reward int) (contextsrv.Memory, error) {
	return a.svc.ApplyMemoryFeedback(id, reward)
}

// ── Skill bindings ──────────────────────────────────────────────

func (a *App) ListSkills(project string, limit int) ([]contextsrv.Skill, error) {
	return a.svc.ListSkills(project, limit)
}

func (a *App) CreateSkill(input contextsrv.SkillCreateInput) (contextsrv.Skill, error) {
	return a.svc.CreateSkill(input)
}

func (a *App) UpdateSkill(input contextsrv.SkillUpdateInput) (contextsrv.Skill, error) {
	return a.svc.UpdateSkill(input)
}

func (a *App) DeleteSkill(id string) error {
	return a.svc.DeleteSkill(id)
}

// ── Context Node bindings ───────────────────────────────────────

func (a *App) ListContextNodes(project string, parentURI *string, limit int) ([]contextsrv.ContextNode, error) {
	return a.svc.ListContextNodes(project, parentURI, limit)
}

func (a *App) CreateContextNode(input contextsrv.ContextCreateInput) (contextsrv.ContextNode, error) {
	return a.svc.CreateContextNode(input)
}

func (a *App) UpdateContextNode(input contextsrv.ContextUpdateInput) (contextsrv.ContextNode, error) {
	return a.svc.UpdateContextNode(input)
}

func (a *App) DeleteContextNode(uri string) error {
	return a.svc.DeleteContextNode(uri)
}

// ── Search & Dashboard ──────────────────────────────────────────

func (a *App) Search(opts contextsrv.SearchOptions) (map[string]any, error) {
	return a.svc.Search(opts)
}

func (a *App) Dashboard(limit int, project string) (map[string]any, error) {
	return a.svc.Dashboard(limit, project)
}

func (a *App) Health() map[string]any {
	return a.svc.Health()
}

func (a *App) ClusterStats() map[string]any {
	return a.svc.ClusterStats()
}

// ── Vibe ────────────────────────────────────────────────────────

func (a *App) VibeQuery(prompt, project string, topK int) (contextsrv.VibeQueryResult, error) {
	return a.svc.VibeQuery(prompt, project, topK)
}

func (a *App) VibeMutationPlan(prompt, project string, topK int) (contextsrv.VibeMutationPlan, error) {
	return a.svc.PlanVibeMutation(prompt, project, topK)
}

func (a *App) VibeMutationExecute(ops []contextsrv.VibeMutationOp, project string) ([]map[string]any, error) {
	res, err := a.svc.ExecuteVibeMutation(ops, project)
	if err == nil {
		a.ShowNotification("Mairu", fmt.Sprintf("Executed %d mutations", len(ops)))
	}
	return res, err
}

// ── Moderation ──────────────────────────────────────────────────

func (a *App) ListModerationQueue(limit int) ([]contextsrv.ModerationEvent, error) {
	return a.svc.ListModerationQueue(limit)
}

func (a *App) ReviewModeration(input contextsrv.ModerationReviewInput) error {
	return a.svc.ReviewModeration(input)
}
