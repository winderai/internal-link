package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DocumentCache represents cached document analysis results
type DocumentCache struct {
	WordFreq    map[string]int `json:"word_freq"`
	LastUpdated time.Time      `json:"last_updated"`
}

// Cache manages document analysis caching
type Cache struct {
	cacheDir string
}

// NewCache creates a new cache instance
func NewCache(cacheDir string) (*Cache, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	return &Cache{cacheDir: cacheDir}, nil
}

// Get retrieves cached document analysis if available and fresh
func (c *Cache) Get(docPath string) (*DocumentCache, error) {
	cachePath := c.getCachePath(docPath)

	// Check if cache file exists
	info, err := os.Stat(cachePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat cache file: %w", err)
	}

	// Check if source file is newer than cache
	sourceInfo, err := os.Stat(docPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat source file: %w", err)
	}

	if sourceInfo.ModTime().After(info.ModTime()) {
		return nil, nil
	}

	// Read and parse cache file
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache DocumentCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse cache file: %w", err)
	}

	return &cache, nil
}

// Set stores document analysis in cache
func (c *Cache) Set(docPath string, wordFreq map[string]int) error {
	cache := DocumentCache{
		WordFreq:    wordFreq,
		LastUpdated: time.Now(),
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	cachePath := c.getCachePath(docPath)
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Clear removes all cached data
func (c *Cache) Clear() error {
	if err := os.RemoveAll(c.cacheDir); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}
	return os.MkdirAll(c.cacheDir, 0755)
}

func (c *Cache) getCachePath(docPath string) string {
	// Create a cache file name based on the document path
	hashedName := fmt.Sprintf("%x", docPath)
	return filepath.Join(c.cacheDir, hashedName+".cache")
}
