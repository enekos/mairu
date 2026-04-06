package crawler

import (
	"strings"
	"testing"
)

func TestChunkText(t *testing.T) {
	text := strings.Repeat("A", 10) // "AAAAAAAAAA"
	chunks := chunkText(text, 3)
	if len(chunks) != 4 {
		t.Errorf("Expected 4 chunks, got %d", len(chunks))
	}
	if chunks[0] != "AAA" {
		t.Errorf("Expected 'AAA', got '%s'", chunks[0])
	}
	if chunks[3] != "A" {
		t.Errorf("Expected 'A', got '%s'", chunks[3])
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1.0, 0.0, 0.0}
	b := []float32{1.0, 0.0, 0.0}
	c := []float32{0.0, 1.0, 0.0}

	simAB := cosineSimilarity(a, b)
	if simAB < 0.99 {
		t.Errorf("Expected ~1.0, got %f", simAB)
	}

	simAC := cosineSimilarity(a, c)
	if simAC > 0.01 {
		t.Errorf("Expected ~0.0, got %f", simAC)
	}
}
