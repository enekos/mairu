package llm

import "testing"

func TestEnsureEmbeddingDimension(t *testing.T) {
	if !EnsureEmbeddingDimension(make([]float32, 3072), 3072) {
		t.Fatal("expected true for matching dimensions")
	}
	if EnsureEmbeddingDimension(make([]float32, 10), 3072) {
		t.Fatal("expected false for mismatching dimensions")
	}
}

func TestGetEmbedding_NilProviderDoesNotPanic(t *testing.T) {
	var provider *GeminiProvider

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected nil provider to return error, got panic: %v", r)
		}
	}()

	_, err := provider.GetEmbedding(t.Context(), "hello")
	if err == nil {
		t.Fatal("expected error when provider is nil")
	}
}
