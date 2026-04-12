package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Embedder is the minimal interface for generating text embeddings.
type Embedder interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
	GetEmbeddingsBatch(ctx context.Context, texts []string) ([][]float32, error)
	GetEmbeddingDimension() int
}

// OllamaEmbedder generates embeddings via the Ollama HTTP API.
type OllamaEmbedder struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

// NewOllamaEmbedder creates an embedder that talks to Ollama.
// It reads MAIRU_OLLAMA_URL from the environment, defaulting to http://localhost:11434.
func NewOllamaEmbedder(model string) *OllamaEmbedder {
	baseURL := os.Getenv("MAIRU_OLLAMA_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	return &OllamaEmbedder{
		BaseURL: baseURL,
		Model:   model,
		Client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// GetEmbedding returns a single embedding vector.
func (o *OllamaEmbedder) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	batch, err := o.GetEmbeddingsBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(batch) == 0 {
		return nil, fmt.Errorf("ollama returned empty embedding batch")
	}
	return batch[0], nil
}

// GetEmbeddingDimension returns the dimension of the model's embeddings.
func (o *OllamaEmbedder) GetEmbeddingDimension() int {
	if o.Model == "nomic-embed-text" {
		return 768
	}
	// conservative fallback
	return 768
}

// GetEmbeddingsBatch returns embedding vectors for multiple texts.
func (o *OllamaEmbedder) GetEmbeddingsBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := map[string]any{
		"model": o.Model,
		"input": texts,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ollama embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/api/embed", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed returned status %d", resp.StatusCode)
	}

	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode ollama embed response: %w", err)
	}

	if len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(result.Embeddings))
	}
	return result.Embeddings, nil
}
