package cmd

import (
	"strings"

	"mairu/internal/llm"
)

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
		Type:    providerType,
		Model:   appConfig.LLM.Model,
		BaseURL: appConfig.LLM.BaseURL,
	}

	// Set API key based on provider
	switch providerType {
	case llm.ProviderKimi:
		cfg.APIKey = cleanAPIKey(appConfig.API.KimiAPIKey)
		if cfg.APIKey == "" {
			cfg.APIKey = cleanAPIKey(appConfig.LLM.KimiAPIKey)
		}
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
