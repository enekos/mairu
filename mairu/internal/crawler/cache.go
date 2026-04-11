package crawler

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
)

type Cache struct {
	mu       sync.RWMutex
	data     *lru.Cache[string, CacheEntry]
	filePath string
}

func NewCache(filePath string) *Cache {
	// Set an upper bound to prevent massive memory usage on huge crawls
	lruCache, _ := lru.New[string, CacheEntry](5000)
	c := &Cache{
		data:     lruCache,
		filePath: filePath,
	}
	c.load()
	return c
}

func (c *Cache) load() {
	if c.filePath == "" {
		return
	}
	b, err := os.ReadFile(c.filePath)
	if err != nil {
		return
	}

	// Temporarily load all into a map, then push to LRU
	var temp map[string]CacheEntry
	if err := json.Unmarshal(b, &temp); err == nil {
		for k, v := range temp {
			c.data.Add(k, v)
		}
	}
}

func (c *Cache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.filePath == "" {
		return nil
	}

	// Dump LRU keys to map for persistence
	temp := make(map[string]CacheEntry)
	for _, key := range c.data.Keys() {
		if val, ok := c.data.Get(key); ok {
			temp[key] = val
		}
	}

	b, err := json.MarshalIndent(temp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.filePath, b, 0644)
}

func (c *Cache) Get(url string) (CacheEntry, bool) {
	return c.data.Get(url)
}

func (c *Cache) Set(url string, entry CacheEntry) {
	c.data.Add(url, entry)
}

func (c *Cache) ContentHash(content string) string {
	h := sha1.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Cache) IsUnchanged(url, content string) bool {
	entry, ok := c.Get(url)
	if !ok {
		return false
	}
	hash := c.ContentHash(content)
	return entry.ContentHash == hash
}
