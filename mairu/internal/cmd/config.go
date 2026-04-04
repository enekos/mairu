package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config represents the local CLI configuration settings.
type Config struct {
	GeminiAPIKey string `json:"gemini_api_key"`
}

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, ".config", "mairu")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

// SaveConfig persists the given configuration to the local filesystem.
func SaveConfig(cfg Config) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadConfig retrieves the configuration from the local filesystem.
func LoadConfig() (Config, error) {
	var cfg Config
	path, err := getConfigPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

// GetAPIKey looks up the Gemini API key, preferring environment variables over config.
func GetAPIKey() string {
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return cleanAPIKey(key)
	}
	if cfg, err := LoadConfig(); err == nil && cfg.GeminiAPIKey != "" {
		return cleanAPIKey(cfg.GeminiAPIKey)
	}
	return ""
}

func cleanAPIKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, "\"'")
	return key
}
