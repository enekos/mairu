package llm

import (
	lru "github.com/hashicorp/golang-lru/v2"
)

// EmbeddingCache is a bounded, concurrency-safe in-memory cache for query
// embeddings.
//
// Typical embedding calls cost ~10 ms and a network round-trip to the
// Gemini API. Caching the last N query vectors eliminates redundant calls
// for repeated or near-identical queries within a session.
type EmbeddingCache struct {
	cache *lru.Cache[string, []float32]
}

// NewEmbeddingCache creates a cache with the given capacity.
// capacity ≤ 0 disables caching (Get always misses, Put is a no-op).
func NewEmbeddingCache(capacity int) *EmbeddingCache {
	if capacity <= 0 {
		return &EmbeddingCache{cache: nil}
	}
	c, _ := lru.New[string, []float32](capacity)
	return &EmbeddingCache{cache: c}
}

// Get returns the cached embedding for key and true, or nil, false if absent.
func (c *EmbeddingCache) Get(key string) ([]float32, bool) {
	if c.cache == nil {
		return nil, false
	}
	return c.cache.Get(key)
}

// Put stores the embedding for key, evicting the least-recently-used entry
// if the cache is at capacity.
func (c *EmbeddingCache) Put(key string, value []float32) {
	if c.cache == nil {
		return
	}
	c.cache.Add(key, value)
}
