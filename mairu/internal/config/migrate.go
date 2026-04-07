package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MigrateLegacyJSON checks for a config.json in configDir and migrates it to
// config.toml. The old file is renamed to config.json.bak. Returns true if
// migration occurred.
func MigrateLegacyJSON(configDir string) (bool, error) {
	jsonPath := filepath.Join(configDir, "config.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	// Don't migrate if TOML already exists.
	tomlPath := filepath.Join(configDir, "config.toml")
	if _, err := os.Stat(tomlPath); err == nil {
		return false, nil
	}

	var legacy struct {
		GeminiAPIKey string `json:"gemini_api_key"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return false, fmt.Errorf("parse legacy config.json: %w", err)
	}

	var b strings.Builder
	b.WriteString("# Migrated from config.json\n\n")
	b.WriteString("[api]\n")
	if legacy.GeminiAPIKey != "" {
		b.WriteString(fmt.Sprintf("gemini_api_key = %q\n", legacy.GeminiAPIKey))
	}

	if err := os.WriteFile(tomlPath, []byte(b.String()), 0600); err != nil {
		return false, fmt.Errorf("write config.toml: %w", err)
	}

	if err := os.Rename(jsonPath, jsonPath+".bak"); err != nil {
		return false, fmt.Errorf("rename config.json to .bak: %w", err)
	}

	return true, nil
}
