package llm

import "testing"

func TestEnsureEmbeddingDimension(t *testing.T) {
	if !EnsureEmbeddingDimension(make([]float32, 768), 768) {
		t.Fatal("expected true for matching dimensions")
	}
	if EnsureEmbeddingDimension(make([]float32, 10), 768) {
		t.Fatal("expected false for mismatching dimensions")
	}
}
