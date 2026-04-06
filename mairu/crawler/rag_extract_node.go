package crawler

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"

	"mairu/internal/llm"
)

// RAGExtractNode uses embeddings to find the most relevant chunks of a large document
// before asking the LLM to extract data, reducing token usage and improving accuracy.
type RAGExtractNode struct {
	Provider    *llm.GeminiProvider
	ChunkSize   int
	TopK        int
	Concurrency int
}

func (n *RAGExtractNode) Name() string { return "RAGExtractNode" }

func cosineSimilarity(a, b []float32) float32 {
	var dotProduct, normA, normB float32
	for i := 0; i < len(a) && i < len(b); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

func chunkText(text string, chunkSize int) []string {
	var chunks []string
	runes := []rune(text)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

func (n *RAGExtractNode) Execute(ctx context.Context, state State) (State, error) {
	doc, ok := state["doc"].(string)
	if !ok || doc == "" {
		return state, fmt.Errorf("RAGExtractNode: missing 'doc' in state")
	}
	userPrompt, ok := state["prompt"].(string)
	if !ok || userPrompt == "" {
		return state, fmt.Errorf("RAGExtractNode: missing 'prompt' in state")
	}

	if n.ChunkSize <= 0 {
		n.ChunkSize = 4000
	}
	if n.TopK <= 0 {
		n.TopK = 5
	}
	if n.Concurrency <= 0 {
		n.Concurrency = 5
	}

	chunks := chunkText(doc, n.ChunkSize)

	// If it fits in one chunk (or TopK chunks easily), just bypass RAG to save time
	if len(chunks) <= n.TopK {
		extractNode := &ExtractNode{Provider: n.Provider}
		return extractNode.Execute(ctx, state)
	}

	// 1. Embed Prompt
	promptEmb, err := n.Provider.GetEmbedding(ctx, userPrompt)
	if err != nil {
		return state, fmt.Errorf("RAGExtractNode: failed to embed prompt: %w", err)
	}

	// 2. Embed Chunks concurrently
	type chunkScore struct {
		text  string
		score float32
		index int
	}

	scores := make([]chunkScore, len(chunks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, n.Concurrency)
	var firstErr error
	var errMu sync.Mutex

	for i, text := range chunks {
		wg.Add(1)
		go func(idx int, chunk string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			emb, err := n.Provider.GetEmbedding(ctx, chunk)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
			}

			score := cosineSimilarity(promptEmb, emb)
			scores[idx] = chunkScore{text: chunk, score: score, index: idx}
		}(i, text)
	}

	wg.Wait()
	if firstErr != nil {
		return state, fmt.Errorf("RAGExtractNode: failed embedding chunks: %w", firstErr)
	}

	// 3. Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// 4. Take top K and restore original order to maintain context
	topK := n.TopK
	if topK > len(scores) {
		topK = len(scores)
	}
	selected := scores[:topK]

	sort.Slice(selected, func(i, j int) bool {
		return selected[i].index < selected[j].index
	})

	var combinedContext string
	for _, s := range selected {
		combinedContext += s.text + "\n...\n"
	}

	// 5. Replace doc with relevant chunks and call ExtractNode
	state["doc"] = combinedContext

	extractNode := &ExtractNode{Provider: n.Provider}
	return extractNode.Execute(ctx, state)
}
