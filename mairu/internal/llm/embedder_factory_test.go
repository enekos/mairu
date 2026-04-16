package llm

import (
	"testing"

	"mairu/internal/config"
)

func TestNewEmbedder_OpenAI(t *testing.T) {
	emb, err := NewEmbedder(config.EmbeddingConfig{
		Provider: "openai",
		Model:    "nomic-embed-text",
		BaseURL:  "http://localhost:11434/v1",
	})
	if err != nil {
		t.Fatalf("NewEmbedder(openai) error: %v", err)
	}
	if emb == nil {
		t.Fatal("expected embedder, got nil")
	}
	oe, ok := emb.(*OpenAIEmbedder)
	if !ok {
		t.Fatalf("expected *OpenAIEmbedder, got %T", emb)
	}
	if oe.GetEmbeddingDimension() != 768 {
		t.Errorf("dimension = %d, want 768", oe.GetEmbeddingDimension())
	}
}

func TestNewEmbedder_EmptyProviderDefaultsToFastEmbed(t *testing.T) {
	emb, err := NewEmbedder(config.EmbeddingConfig{
		Model: "fast-all-MiniLM-L6-v2",
	})
	if err != nil {
		// fastembed requires ONNX Runtime to be installed; skip if it's missing.
		t.Skipf("NewEmbedder(empty) skipped: %v", err)
	}
	if _, ok := emb.(*FastEmbedder); !ok {
		t.Fatalf("expected *FastEmbedder for empty provider, got %T", emb)
	}
}

func TestNewEmbedder_LegacyOllama(t *testing.T) {
	emb, err := NewEmbedder(config.EmbeddingConfig{
		Provider: "ollama",
		Model:    "nomic-embed-text",
		BaseURL:  "http://localhost:11434",
	})
	if err != nil {
		t.Fatalf("NewEmbedder(ollama) error: %v", err)
	}
	if _, ok := emb.(*OpenAIEmbedder); !ok {
		t.Fatalf("expected *OpenAIEmbedder for legacy ollama provider, got %T", emb)
	}
}

func TestNewEmbedder_UnknownProvider(t *testing.T) {
	_, err := NewEmbedder(config.EmbeddingConfig{
		Provider: "unknown",
	})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
