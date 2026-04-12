package llm

import (
	"context"
	"fmt"
	"os"
)

// NewProvider creates a new LLM provider based on the configuration.
// It also supports legacy behavior: if Type is empty but GEMINI_API_KEY is set,
// it defaults to Gemini.
func NewProvider(cfg ProviderConfig) (Provider, error) {
	// Auto-detect provider type if not specified
	if cfg.Type == "" {
		if cfg.APIKey != "" {
			// Try to detect from key prefix or environment
			if os.Getenv("MAIRU_LLM_PROVIDER") != "" {
				cfg.Type = ProviderType(os.Getenv("MAIRU_LLM_PROVIDER"))
			} else if os.Getenv("GEMINI_API_KEY") != "" && cfg.APIKey == os.Getenv("GEMINI_API_KEY") {
				cfg.Type = ProviderGemini
			} else {
				// Default to Gemini for backward compatibility
				cfg.Type = ProviderGemini
			}
		} else {
			return nil, fmt.Errorf("no API key provided")
		}
	}

	switch cfg.Type {
	case ProviderGemini:
		return NewGeminiProviderFromConfig(context.Background(), cfg)
	case ProviderKimi:
		return NewKimiProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
}

// NewProviderFromEnv creates a provider using environment configuration.
// This is a convenience function that reads standard environment variables.
func NewProviderFromEnv() (Provider, error) {
	providerType := ProviderType(os.Getenv("MAIRU_LLM_PROVIDER"))
	if providerType == "" {
		providerType = ProviderGemini // Default
	}

	var apiKey string
	switch providerType {
	case ProviderKimi:
		apiKey = os.Getenv("MAIRU_KIMI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("KIMI_API_KEY")
		}
	default: // Gemini
		apiKey = os.Getenv("MAIRU_GEMINI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GEMINI_API_KEY")
		}
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no API key found for provider %s", providerType)
	}

	cfg := ProviderConfig{
		Type:    providerType,
		APIKey:  apiKey,
		Model:   os.Getenv("MAIRU_LLM_MODEL"),
		BaseURL: os.Getenv("MAIRU_LLM_BASE_URL"),
	}

	return NewProvider(cfg)
}
