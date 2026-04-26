package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/enekos/mairu/pii-redact/pkg/redact"
	"mairu/internal/contextsrv"
	"mairu/internal/crawler"
	"mairu/internal/llm"
)

type SymbolLocator interface {
	FindSymbol(name string) ([]contextsrv.SymbolLocation, error)
}

type HistoryLogger interface {
	InsertBashHistory(ctx context.Context, project string, command string, exitCode int, durationMs int, output string) error
}

type Agent struct {
	llm         llm.Provider
	locator     SymbolLocator
	root        string
	currentDir  string
	providerCfg llm.ProviderConfig

	stuckDetector *StuckDetector
	utcp          *UTCPManager
	utcpProviders []string

	Unattended bool
	council    CouncilConfig

	historyLogger   HistoryLogger
	interceptors    []ToolInterceptor
	AgentSystemData map[string]any

	// redactor, when non-nil, scrubs bash tool output before it is returned
	// to the model. Opt-in via config (`[agent] redact_bash_output = true`),
	// CLI (`--redact`), or env (`MAIRU_REDACT_BASH=1`).
	redactor *redact.Redactor

	fileQueue *fileMutationQueue

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
	// RedactBashOutput enables pii-redact on bash tool stdout+stderr before
	// it is handed back to the model. Defaults to false. CLI/env may
	// override (see ResolveRedactBashOutput).
	RedactBashOutput bool
}

// ResolveRedactBashOutput returns the effective opt-in state combining the
// caller's config value with the MAIRU_REDACT_BASH env var (truthy values:
// "1", "true", "yes", "on" — case-insensitive). The env wins if set.
func ResolveRedactBashOutput(cfgVal bool) bool {
	v := strings.TrimSpace(os.Getenv("MAIRU_REDACT_BASH"))
	if v == "" {
		return cfgVal
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return cfgVal
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

// New creates a new Agent with the specified LLM provider configuration
func New(projectRoot string, providerCfg llm.ProviderConfig, cfg ...Config) (*Agent, error) {
	resolved := normalizeConfig(cfg...)

	llmProvider, err := llm.NewProvider(providerCfg)
	if err != nil {
		return nil, err
	}
	llmProvider.SetTools(builtinToolSchemas())

	utcpManager, err := NewUTCPManager(resolved.UTCPProviders)
	if err != nil {
		return nil, fmt.Errorf("failed to init UTCP manager: %w", err)
	}

	// Fetch dynamic tools
	if len(resolved.UTCPProviders) > 0 {
		utcpTools := utcpManager.Initialize(context.Background())
		llmProvider.RegisterDynamicTools(utcpTools)
	}

	a := &Agent{
		llm:             llmProvider,
		locator:         resolved.SymbolLocator,
		root:            projectRoot,
		currentDir:      projectRoot,
		providerCfg:     providerCfg,
		stuckDetector:   NewStuckDetector(),
		utcp:            utcpManager,
		utcpProviders:   resolved.UTCPProviders,
		Unattended:      resolved.Unattended,
		council:         resolved.Council.withDefaults(),
		historyLogger:   resolved.HistoryLogger,
		interceptors:    resolved.Interceptors,
		AgentSystemData: resolved.AgentSystemData,
		approvalChan:    make(chan bool),
		fileQueue:       newFileMutationQueue(),
	}
	if ResolveRedactBashOutput(resolved.RedactBashOutput) {
		rd, err := redact.New(redact.Options{})
		if err != nil {
			return nil, fmt.Errorf("init pii-redact: %w", err)
		}
		a.redactor = rd
	}
	return a, nil
}

// NewWithProvider creates a new Agent with an existing LLM provider (for testing/advanced use)
func NewWithProvider(projectRoot string, provider llm.Provider, cfg ...Config) (*Agent, error) {
	resolved := normalizeConfig(cfg...)
	provider.SetTools(builtinToolSchemas())

	utcpManager, err := NewUTCPManager(resolved.UTCPProviders)
	if err != nil {
		return nil, fmt.Errorf("failed to init UTCP manager: %w", err)
	}

	// Fetch dynamic tools
	if len(resolved.UTCPProviders) > 0 {
		utcpTools := utcpManager.Initialize(context.Background())
		provider.RegisterDynamicTools(utcpTools)
	}

	a := &Agent{
		llm:             provider,
		locator:         resolved.SymbolLocator,
		root:            projectRoot,
		currentDir:      projectRoot,
		stuckDetector:   NewStuckDetector(),
		utcp:            utcpManager,
		utcpProviders:   resolved.UTCPProviders,
		Unattended:      resolved.Unattended,
		council:         resolved.Council.withDefaults(),
		historyLogger:   resolved.HistoryLogger,
		interceptors:    resolved.Interceptors,
		AgentSystemData: resolved.AgentSystemData,
		approvalChan:    make(chan bool),
		fileQueue:       newFileMutationQueue(),
	}
	if ResolveRedactBashOutput(resolved.RedactBashOutput) {
		rd, err := redact.New(redact.Options{})
		if err != nil {
			return nil, fmt.Errorf("init pii-redact: %w", err)
		}
		a.redactor = rd
	}
	return a, nil
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

// scraper returns a crawler.Scraper backed by the agent's LLM provider.
func (a *Agent) scraper() *crawler.Scraper {
	return crawler.NewScraper(crawler.NewEngine(nil), a.llm)
}

type AgentEvent struct {
	Type       string         `json:"Type"` // "text", "status", "error", "done", "tool_call", "tool_result", "log", "bash_output"
	Content    string         `json:"Content"`
	ToolName   string         `json:"ToolName,omitempty"`
	ToolArgs   map[string]any `json:"ToolArgs,omitempty"`
	ToolResult map[string]any `json:"ToolResult,omitempty"`
}
