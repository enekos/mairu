package cmd

import (
	"strings"

	"mairu/internal/config"
	"mairu/internal/llm"
)

// GetAPIKey looks up the API key for the configured provider
func GetAPIKey() string {
	if appConfig == nil {
		return ""
	}

	provider := appConfig.LLM.Provider
	if provider == "" {
		provider = "gemini" // Default
	}

	switch provider {
	case "kimi":
		if appConfig.API.KimiAPIKey != "" {
			return cleanAPIKey(appConfig.API.KimiAPIKey)
		}
	default: // gemini
		if appConfig.API.GeminiAPIKey != "" {
			return cleanAPIKey(appConfig.API.GeminiAPIKey)
		}
	}

	return ""
}

// GetLLMProviderConfig returns the LLM provider configuration
func GetLLMProviderConfig() llm.ProviderConfig {
	if appConfig == nil {
		return llm.ProviderConfig{}
	}

	providerType := llm.ProviderType(appConfig.LLM.Provider)
	if providerType == "" {
		providerType = llm.ProviderGemini
	}

	cfg := llm.ProviderConfig{
		Type:  providerType,
		Model: appConfig.LLM.Model,
	}

	// Set API key based on provider
	switch providerType {
	case llm.ProviderKimi:
		cfg.APIKey = cleanAPIKey(appConfig.API.KimiAPIKey)
	default:
		cfg.APIKey = cleanAPIKey(appConfig.API.GeminiAPIKey)
	}

	return cfg
}

func cleanAPIKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, "\"'")
	return key
}

// ProviderTypeFromString converts a string to ProviderType
func ProviderTypeFromString(s string) llm.ProviderType {
	switch strings.ToLower(s) {
	case "kimi":
		return llm.ProviderKimi
	case "gemini":
		return llm.ProviderGemini
	default:
		return llm.ProviderGemini
	}
}

// LoadConfigForDirectory loads configuration for a specific directory
func LoadConfigForDirectory(dir string) (*config.Config, error) {
	return config.Load(dir)
}
