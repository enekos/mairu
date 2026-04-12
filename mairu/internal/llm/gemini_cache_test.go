package llm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGeminiCacheSmartLogic(t *testing.T) {
	ctx := context.Background()

	// Use a mock provider with just the basic structure (we only test the heuristic here)
	provider := &GeminiProvider{}

	// Short prompt -> should return empty
	shortPrompt := "This is a short prompt."
	name, err := provider.CacheContext(ctx, shortPrompt, 15*time.Minute)
	assert.NoError(t, err)
	assert.Empty(t, name)

	// SetCachedContent with empty name should do nothing and return nil
	err = provider.SetCachedContent(ctx, "")
	assert.NoError(t, err)

	// DeleteCachedContent with empty name should do nothing and return nil
	err = provider.DeleteCachedContent(ctx, "")
	assert.NoError(t, err)
}

func TestGeminiCacheIntegration(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping Gemini cache integration test: GEMINI_API_KEY is not set")
	}

	ctx := context.Background()
	provider, err := NewGeminiProvider(ctx, apiKey)
	assert.NoError(t, err)
	defer provider.Close()

	// Generate a huge prompt to bypass the 100k smart limit
	var sb strings.Builder
	for i := 0; i < 10000; i++ {
		sb.WriteString("This is a dummy context line to inflate the token count for caching. ")
	}
	hugePrompt := sb.String()

	cacheName, err := provider.CacheContext(ctx, hugePrompt, 15*time.Minute)
	if status.Code(err) == codes.PermissionDenied {
		t.Skip("Skipping Gemini cache integration test: API key is invalid or leaked")
	}
	assert.NoError(t, err)
	assert.NotEmpty(t, cacheName)

	// Ensure cleanup
	defer func() {
		err := provider.DeleteCachedContent(ctx, cacheName)
		assert.NoError(t, err, "failed to cleanup cache")
	}()

	// 2. Load the cache into the provider
	err = provider.SetCachedContent(ctx, cacheName)
	assert.NoError(t, err)

	// We can check if the model in the provider was successfully swapped out
	assert.NotNil(t, provider.model)
}
