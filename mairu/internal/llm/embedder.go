package llm

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/google/generative-ai-go/genai"
)

func (g *GeminiProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	if g == nil || g.client == nil {
		return nil, fmt.Errorf("gemini provider is not initialized")
	}

	modelName := g.EmbeddingModel
	if modelName == "" {
		modelName = os.Getenv("EMBEDDING_MODEL")
	}
	if modelName == "" {
		modelName = "text-embedding-004" // gemini-embedding-001 is older, text-embedding-004 is recommended, but let's default to text-embedding-004 for newer projects or use what they have.
	}
	em := g.client.EmbeddingModel(modelName)
	res, err := em.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %w", err)
	}
	if res.Embedding == nil || len(res.Embedding.Values) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return res.Embedding.Values, nil
}

func (g *GeminiProvider) GetEmbeddingsBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if g == nil || g.client == nil {
		return nil, fmt.Errorf("gemini provider is not initialized")
	}

	modelName := g.EmbeddingModel
	if modelName == "" {
		modelName = os.Getenv("EMBEDDING_MODEL")
	}
	if modelName == "" {
		modelName = "text-embedding-004"
	}
	em := g.client.EmbeddingModel(modelName)
	batch := em.NewBatch()
	for _, t := range texts {
		batch.AddContent(genai.Text(t))
	}

	res, err := em.BatchEmbedContents(ctx, batch)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch embeddings: %w", err)
	}

	if len(res.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(res.Embeddings))
	}

	var out [][]float32
	for _, e := range res.Embeddings {
		out = append(out, e.Values)
	}
	return out, nil
}

func (g *GeminiProvider) GetEmbeddingDimension() int {
	if g == nil {
		return 3072
	}

	if g.EmbeddingDim > 0 {
		return g.EmbeddingDim
	}
	dimStr := os.Getenv("EMBEDDING_DIM")
	if dimStr != "" {
		dim, err := strconv.Atoi(dimStr)
		if err == nil {
			return dim
		}
	}
	model := g.EmbeddingModel
	if model == "" {
		model = os.Getenv("EMBEDDING_MODEL")
	}
	if model == "text-embedding-004" {
		return 768
	}
	return 3072 // gemini-embedding-001
}
