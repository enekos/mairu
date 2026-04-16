package llm

import (
	"fmt"
	"log/slog"
	"os"

	"mairu/internal/config"
)

// NewEmbedder creates an embedder based on the configuration provider.
func NewEmbedder(cfg config.EmbeddingConfig) (Embedder, error) {
	switch cfg.Provider {
	case "fastembed":
		emb, err := NewFastEmbedder(cfg.Model, cfg.Dimensions)
		if err != nil {
			return nil, fmt.Errorf("fastembed init failed (ensure ONNX Runtime is installed: https://onnxruntime.ai): %w", err)
		}
		return emb, nil
	case "openai":
		return NewOpenAIEmbedder(cfg.Model, cfg.BaseURL, cfg.APIKey), nil
	case "ollama":
		// Legacy provider: fall back to openai format pointing at the Ollama base URL.
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = os.Getenv("MAIRU_OLLAMA_URL")
		}
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		// Ollama's OpenAI-compatible endpoint is under /v1.
		if baseURL[len(baseURL)-1] != '/' {
			baseURL += "/"
		}
		baseURL += "v1"
		slog.Warn("embedding provider 'ollama' is deprecated; using openai-compatible endpoint", "base_url", baseURL)
		return NewOpenAIEmbedder(cfg.Model, baseURL, cfg.APIKey), nil
	case "":
		// Default to fastembed when not specified.
		emb, err := NewFastEmbedder(cfg.Model, cfg.Dimensions)
		if err != nil {
			return nil, fmt.Errorf("fastembed init failed (ensure ONNX Runtime is installed: https://onnxruntime.ai): %w", err)
		}
		return emb, nil
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", cfg.Provider)
	}
}
