package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"mairu/internal/httptracer"
)

// OpenAIEmbedder generates embeddings via the OpenAI-compatible HTTP API.
type OpenAIEmbedder struct {
	BaseURL string
	Model   string
	APIKey  string
	Client  *http.Client
}

// NewOpenAIEmbedder creates an embedder that talks to any OpenAI-compatible endpoint.
func NewOpenAIEmbedder(model, baseURL, apiKey string) *OpenAIEmbedder {
	if model == "" {
		model = "nomic-embed-text"
	}
	return &OpenAIEmbedder{
		BaseURL: baseURL,
		Model:   model,
		APIKey:  apiKey,
		Client:  httptracer.Wrap(&http.Client{Timeout: 120 * time.Second}, httptracer.Options{Component: "embedder"}),
	}
}

// GetEmbedding returns a single embedding vector.
func (o *OpenAIEmbedder) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	batch, err := o.GetEmbeddingsBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(batch) == 0 {
		return nil, fmt.Errorf("embedder returned empty embedding batch")
	}
	return batch[0], nil
}

// GetEmbeddingDimension returns the dimension of the model's embeddings.
func (o *OpenAIEmbedder) GetEmbeddingDimension() int {
	// Most local embedding models default to 768; keep fallback for backward compat.
	return 768
}

// GetEmbeddingsBatch returns embedding vectors for multiple texts.
func (o *OpenAIEmbedder) GetEmbeddingsBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := map[string]any{
		"model": o.Model,
		"input": texts,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed returned status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode embed response: %w", err)
	}

	embeddings := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index < 0 || d.Index >= len(texts) {
			continue
		}
		embeddings[d.Index] = d.Embedding
	}

	for i, emb := range embeddings {
		if emb == nil {
			return nil, fmt.Errorf("missing embedding for index %d", i)
		}
	}

	return embeddings, nil
}
