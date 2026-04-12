package llm

import (
	"github.com/google/generative-ai-go/genai"
)

// geminiStreamIterator wraps genai.GenerateContentResponseIterator to implement ChatStreamIterator
type geminiStreamIterator struct {
	iter   *genai.GenerateContentResponseIterator
	done   bool
	buffer string
}

func newGeminiStreamIterator(iter *genai.GenerateContentResponseIterator) ChatStreamIterator {
	return &geminiStreamIterator{iter: iter}
}

func (g *geminiStreamIterator) Next() (ChatStreamChunk, error) {
	resp, err := g.iter.Next()
	if err != nil {
		g.done = true
		return ChatStreamChunk{}, err
	}

	if resp == nil {
		g.done = true
		return ChatStreamChunk{FinishReason: "stop"}, nil
	}

	chunk := ChatStreamChunk{}

	// Extract content from response
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, part := range resp.Candidates[0].Content.Parts {
			if text, ok := part.(genai.Text); ok {
				chunk.Content += string(text)
			}
			// Handle function calls
			if fc, ok := part.(genai.FunctionCall); ok {
				chunk.ToolCalls = append(chunk.ToolCalls, ToolCall{
					ID:        fc.Name, // Gemini doesn't have call IDs, use name
					Name:      fc.Name,
					Arguments: fc.Args,
				})
			}
		}

		// Check finish reason
		if resp.Candidates[0].FinishReason != 0 {
			switch resp.Candidates[0].FinishReason {
			case genai.FinishReasonStop:
				chunk.FinishReason = "stop"
			case genai.FinishReasonMaxTokens:
				chunk.FinishReason = "length"
			case genai.FinishReasonSafety:
				chunk.FinishReason = "safety"
			case genai.FinishReasonRecitation:
				chunk.FinishReason = "recitation"
			default:
				chunk.FinishReason = "stop"
			}
			g.done = true
		}
	}

	return chunk, nil
}

func (g *geminiStreamIterator) Done() bool {
	return g.done
}
