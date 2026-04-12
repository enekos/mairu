package agent

import (
	"context"
	"fmt"
	"sync"

	"mairu/internal/contextsrv"
	"mairu/internal/llm"
)

type SymbolLocator interface {
	FindSymbol(name string) ([]contextsrv.SymbolLocation, error)
}

type HistoryLogger interface {
	InsertBashHistory(ctx context.Context, project string, command string, exitCode int, durationMs int, output string) error
}

type Agent struct {
	llm        *llm.GeminiProvider
	locator    SymbolLocator
	root       string
	currentDir string
	apiKey     string

	stuckDetector *StuckDetector
	utcp          *UTCPManager
	utcpProviders []string

	Unattended bool
	council    CouncilConfig

	historyLogger   HistoryLogger
	interceptors    []ToolInterceptor
	AgentSystemData map[string]any

	mu           sync.Mutex
	cancel       context.CancelFunc
	approvalChan chan bool
}

type Config struct {
	SymbolLocator   SymbolLocator
	Unattended      bool
	Council         CouncilConfig
	HistoryLogger   HistoryLogger
	Interceptors    []ToolInterceptor
	UTCPProviders   []string
	AgentSystemData map[string]any
}

func normalizeConfig(cfg ...Config) Config {
	if len(cfg) == 0 {
		return Config{}
	}
	resolved := cfg[0]
	resolved.Council.Roles = append([]CouncilRole(nil), resolved.Council.Roles...)
	resolved.Interceptors = append([]ToolInterceptor(nil), resolved.Interceptors...)
	resolved.UTCPProviders = append([]string(nil), resolved.UTCPProviders...)
	if resolved.AgentSystemData != nil {
		cloned := make(map[string]any, len(resolved.AgentSystemData))
		for k, v := range resolved.AgentSystemData {
			cloned[k] = v
		}
		resolved.AgentSystemData = cloned
	}
	return resolved
}

func New(projectRoot string, apiKey string, cfg ...Config) (*Agent, error) {
	resolved := normalizeConfig(cfg...)

	llmProvider, err := llm.NewGeminiProvider(context.Background(), apiKey)
	if err != nil {
		return nil, err
	}

	utcpManager, err := NewUTCPManager(resolved.UTCPProviders)
	if err != nil {
		return nil, fmt.Errorf("failed to init UTCP manager: %w", err)
	}

	// Fetch dynamic tools
	if len(resolved.UTCPProviders) > 0 {
		utcpTools := utcpManager.Initialize(context.Background())
		llmProvider.RegisterDynamicTools(utcpTools)
	}

	return &Agent{
		llm:             llmProvider,
		locator:         resolved.SymbolLocator,
		root:            projectRoot,
		currentDir:      projectRoot,
		apiKey:          apiKey,
		stuckDetector:   NewStuckDetector(),
		utcp:            utcpManager,
		utcpProviders:   resolved.UTCPProviders,
		Unattended:      resolved.Unattended,
		council:         resolved.Council.withDefaults(),
		historyLogger:   resolved.HistoryLogger,
		interceptors:    resolved.Interceptors,
		AgentSystemData: resolved.AgentSystemData,
		approvalChan:    make(chan bool),
	}, nil
}

func (a *Agent) childConfig() Config {
	return normalizeConfig(Config{
		SymbolLocator:   a.locator,
		Unattended:      a.Unattended,
		Council:         a.council,
		HistoryLogger:   a.historyLogger,
		Interceptors:    a.interceptors,
		UTCPProviders:   a.utcpProviders,
		AgentSystemData: a.AgentSystemData,
	})
}

func (a *Agent) GetModelName() string {
	return a.llm.GetModelName()
}

func (a *Agent) Interrupt() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
}

func (a *Agent) ApproveAction(approved bool) {
	// Take ownership of the channel under the lock, then send outside it.
	// Sending inside the lock would deadlock if the channel were unbuffered;
	// nil-ing first prevents double-sends from concurrent callers.
	a.mu.Lock()
	ch := a.approvalChan
	a.approvalChan = nil
	a.mu.Unlock()
	if ch != nil {
		ch <- approved
	}
}

func (a *Agent) GetRoot() string {
	return a.root
}

func (a *Agent) SetModel(modelName string) {
	a.llm.SetModel(modelName)
}

func (a *Agent) SetCouncilEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.council.Enabled = enabled
}

func (a *Agent) IsCouncilEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.council.Enabled
}

type AgentEvent struct {
	Type       string         `json:"Type"` // "text", "status", "error", "done", "tool_call", "tool_result", "log", "bash_output"
	Content    string         `json:"Content"`
	ToolName   string         `json:"ToolName,omitempty"`
	ToolArgs   map[string]any `json:"ToolArgs,omitempty"`
	ToolResult map[string]any `json:"ToolResult,omitempty"`
}
