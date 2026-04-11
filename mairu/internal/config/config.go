package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config is the top-level resolved configuration.
type Config struct {
	API       APIConfig       `mapstructure:"api"`
	Search    SearchConfig    `mapstructure:"search"`
	Daemon    DaemonConfig    `mapstructure:"daemon"`
	Server    ServerConfig    `mapstructure:"server"`
	Embedding EmbeddingConfig `mapstructure:"embedding"`
	Output    OutputConfig    `mapstructure:"output"`
	Enricher  EnricherConfig  `mapstructure:"enricher"`
	Security  SecurityConfig  `mapstructure:"security"`
	Tools     ToolsConfig     `mapstructure:"tools"`
}

type ToolsConfig struct {
	UTCPProviders []string `mapstructure:"utcp_providers"`
}

type SecurityConfig struct {
	BlockedCommands []string `mapstructure:"blocked_commands"`
	BlockedPaths    []string `mapstructure:"blocked_paths"`
}

type EnricherConfig struct {
	GitIntent      GitIntentConfig      `mapstructure:"git_intent"`
	ChangeVelocity ChangeVelocityConfig `mapstructure:"change_velocity"`
}

type GitIntentConfig struct {
	Enabled    bool `mapstructure:"enabled"`
	MaxCommits int  `mapstructure:"max_commits"`
}

type ChangeVelocityConfig struct {
	Enabled      bool `mapstructure:"enabled"`
	LookbackDays int  `mapstructure:"lookback_days"`
}

type APIConfig struct {
	GeminiAPIKey string `mapstructure:"gemini_api_key"`
	MeiliURL     string `mapstructure:"meili_url"`
	MeiliAPIKey  string `mapstructure:"meili_api_key"`
}

type SearchConfig struct {
	Memories     WeightsConfig `mapstructure:"memories"`
	Skills       WeightsConfig `mapstructure:"skills"`
	Context      WeightsConfig `mapstructure:"context"`
	RecencyScale string        `mapstructure:"recency_scale"`
	RecencyDecay float64       `mapstructure:"recency_decay"`
	Synonyms     []string      `mapstructure:"synonyms"`
}

type WeightsConfig struct {
	Vector     float64 `mapstructure:"vector"`
	Keyword    float64 `mapstructure:"keyword"`
	Recency    float64 `mapstructure:"recency"`
	Importance float64 `mapstructure:"importance"`
	Churn      float64 `mapstructure:"churn"`
}

type DaemonConfig struct {
	Concurrency     int    `mapstructure:"concurrency"`
	MaxFileSize     string `mapstructure:"max_file_size"`
	Debounce        string `mapstructure:"debounce"`
	MaxContentChars int    `mapstructure:"max_content_chars"`
}

// MaxFileSizeBytes parses the MaxFileSize string (e.g. "512KB") into bytes.
func (d DaemonConfig) MaxFileSizeBytes() int64 {
	n, err := ParseByteSize(d.MaxFileSize)
	if err != nil {
		return 512 * 1024 // fallback
	}
	return n
}

type ServerConfig struct {
	Port              int    `mapstructure:"port"`
	ProjectorInterval string `mapstructure:"projector_interval"`
	ProjectorBatch    int    `mapstructure:"projector_batch"`
	ReadTimeout       string `mapstructure:"read_timeout"`
	SQLiteDSN         string `mapstructure:"sqlite_dsn"`
	AuthToken         string `mapstructure:"auth_token"`
	ModerationEnabled bool   `mapstructure:"moderation_enabled"`
}

type EmbeddingConfig struct {
	Model      string `mapstructure:"model"`
	Dimensions int    `mapstructure:"dimensions"`
	CacheSize  int    `mapstructure:"cache_size"`
}

type OutputConfig struct {
	Format  string `mapstructure:"format"`
	Color   bool   `mapstructure:"color"`
	Verbose bool   `mapstructure:"verbose"`
}

// Load reads configuration using five-tier cascade:
//  1. Hardcoded defaults
//  2. User config (~/.config/mairu/config.toml)
//  3. Project config (.mairu.toml, discovered by walking up from workDir)
//  4. Environment variables (MAIRU_ prefix + legacy aliases)
//  5. CLI flags (bound externally via BindPFlag)
func Load(workDir string) (*Config, error) {
	v := NewViper(workDir)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &cfg, nil
}

func NewViper(workDir string) *viper.Viper {
	v := viper.New()
	setDefaults(v)

	// Layer 2: user config
	home, err := os.UserHomeDir()
	if err == nil {
		v.AddConfigPath(filepath.Join(home, ".config", "mairu"))
	}
	v.SetConfigName("config")
	v.SetConfigType("toml")
	_ = v.MergeInConfig() // no error if file missing

	// Layer 3: project config
	if projectPath := FindProjectConfig(workDir); projectPath != "" {
		v.SetConfigFile(projectPath)
		_ = v.MergeInConfig()
	}

	// Layer 4: environment variables
	v.SetEnvPrefix("MAIRU")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Legacy env var aliases (old name -> config key)
	bindLegacyEnv(v)
	return v
}

// UserConfigPath returns the path to the user-level config file.
func UserConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "mairu", "config.toml")
}

func setDefaults(v *viper.Viper) {
	// API
	v.SetDefault("api.meili_url", "http://localhost:7700")
	v.SetDefault("api.meili_api_key", "contextfs-dev-key")
	v.SetDefault("api.gemini_api_key", "")

	// Search weights
	v.SetDefault("search.memories.vector", 0.6)
	v.SetDefault("search.memories.keyword", 0.2)
	v.SetDefault("search.memories.recency", 0.05)
	v.SetDefault("search.memories.importance", 0.15)
	v.SetDefault("search.memories.churn", 0.0)

	v.SetDefault("search.skills.vector", 0.7)
	v.SetDefault("search.skills.keyword", 0.3)
	v.SetDefault("search.skills.recency", 0.0)
	v.SetDefault("search.skills.importance", 0.0)
	v.SetDefault("search.skills.churn", 0.0)

	v.SetDefault("search.context.vector", 0.65)
	v.SetDefault("search.context.keyword", 0.3)
	v.SetDefault("search.context.recency", 0.05)
	v.SetDefault("search.context.importance", 0.0)
	v.SetDefault("search.context.churn", 0.05)

	v.SetDefault("search.recency_scale", "30d")
	v.SetDefault("search.recency_decay", 0.5)

	// Daemon
	v.SetDefault("daemon.concurrency", 8)
	v.SetDefault("daemon.max_file_size", "512KB")
	v.SetDefault("daemon.debounce", "200ms")
	v.SetDefault("daemon.max_content_chars", 16000)

	// Server
	v.SetDefault("server.port", 8788)
	v.SetDefault("server.projector_interval", "3s")
	v.SetDefault("server.projector_batch", 50)
	v.SetDefault("server.read_timeout", "10s")
	v.SetDefault("server.sqlite_dsn", "file:mairu.db?cache=shared&mode=rwc")
	v.SetDefault("server.auth_token", "")
	v.SetDefault("server.moderation_enabled", false)

	// Embedding
	v.SetDefault("embedding.model", "gemini-embedding-001")
	v.SetDefault("embedding.dimensions", 3072)
	v.SetDefault("embedding.cache_size", 256)

	// Output
	v.SetDefault("output.format", "table")
	v.SetDefault("output.color", true)
	v.SetDefault("output.verbose", false)

	// Enricher
	v.SetDefault("enricher.git_intent.enabled", true)
	v.SetDefault("enricher.git_intent.max_commits", 20)
	v.SetDefault("enricher.change_velocity.enabled", true)
	v.SetDefault("enricher.change_velocity.lookback_days", 180)

	// Security
	v.SetDefault("security.blocked_commands", []string{
		"rm -rf /", "mkfs", "dd",
	})
	v.SetDefault("security.blocked_paths", []string{
		".git/", ".mairu/", ".env",
	})
}

func bindLegacyEnv(v *viper.Viper) {
	// These map old env var names to their new config keys so users
	// don't have to change their .env files or CI pipelines.
	legacy := map[string]string{
		"GEMINI_API_KEY":            "api.gemini_api_key",
		"MEILI_URL":                 "api.meili_url",
		"MEILI_API_KEY":             "api.meili_api_key",
		"EMBEDDING_MODEL":           "embedding.model",
		"EMBEDDING_DIM":             "embedding.dimensions",
		"CONTEXT_SERVER_SQLITE_DSN": "server.sqlite_dsn",
		"CONTEXT_AUTH_TOKEN":        "server.auth_token",
		"CONTEXT_ENABLE_MODERATION": "server.moderation_enabled",
		"RECENCY_SCALE":             "search.recency_scale",
		"RECENCY_DECAY":             "search.recency_decay",
		"DASHBOARD_API_PORT":        "server.port",
	}
	for envKey, configKey := range legacy {
		if val := os.Getenv(envKey); val != "" {
			v.Set(configKey, val)
		}
	}
}
