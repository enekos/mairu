package scraper

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
)

type Cache struct {
	mu       sync.RWMutex
	data     map[string]CacheEntry
	filePath string
}

func NewCache(filePath string) *Cache {
	c := &Cache{
		data:     make(map[string]CacheEntry),
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
	_ = json.Unmarshal(b, &c.data)
}

func (c *Cache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.filePath == "" {
		return nil
	}
	b, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.filePath, b, 0644)
}

func (c *Cache) Get(url string) (CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.data[url]
	return entry, ok
}

func (c *Cache) Set(url string, entry CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[url] = entry
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
