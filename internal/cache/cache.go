package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultTTL is the default time-to-live for cache entries
	DefaultTTL = 60 * time.Second

	// CacheDir is the directory where cache files are stored
	CacheDir = ".cache/gh-arc"
)

// Cache represents a filesystem-based cache
type Cache struct {
	baseDir string
	ttl     time.Duration
}

// CacheEntry represents a cached item with metadata
type CacheEntry struct {
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt time.Time       `json:"expires_at"`
}

// New creates a new cache instance
func New() (*Cache, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, CacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Cache{
		baseDir: cacheDir,
		ttl:     DefaultTTL,
	}, nil
}

// SetTTL sets the time-to-live for cache entries
func (c *Cache) SetTTL(ttl time.Duration) {
	c.ttl = ttl
}

// Get retrieves a cached item by key
func (c *Cache) Get(key string, v interface{}) (bool, error) {
	path := c.getPath(key)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // Cache miss
		}
		return false, fmt.Errorf("failed to read cache file: %w", err)
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		// Invalid cache entry, treat as miss
		_ = os.Remove(path)
		return false, nil
	}

	// Check if entry has expired
	if time.Now().After(entry.ExpiresAt) {
		_ = os.Remove(path)
		return false, nil // Cache expired
	}

	// Unmarshal the cached data
	if err := json.Unmarshal(entry.Data, v); err != nil {
		return false, fmt.Errorf("failed to unmarshal cached data: %w", err)
	}

	return true, nil
}

// Set stores an item in the cache
func (c *Cache) Set(key string, v interface{}) error {
	path := c.getPath(key)

	// Marshal the data
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Create cache entry with metadata
	now := time.Now()
	entry := CacheEntry{
		Data:      data,
		CreatedAt: now,
		ExpiresAt: now.Add(c.ttl),
	}

	entryData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, entryData, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Delete removes a cache entry
func (c *Cache) Delete(key string) error {
	path := c.getPath(key)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file: %w", err)
	}
	return nil
}

// Clear removes all cache entries
func (c *Cache) Clear() error {
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(c.baseDir, entry.Name())
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove cache file %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// CleanExpired removes expired cache entries
func (c *Cache) CleanExpired() error {
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(c.baseDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var cacheEntry CacheEntry
		if err := json.Unmarshal(data, &cacheEntry); err != nil {
			// Invalid entry, remove it
			_ = os.Remove(path)
			continue
		}

		if now.After(cacheEntry.ExpiresAt) {
			_ = os.Remove(path)
		}
	}

	return nil
}

// getPath generates a filesystem path for a cache key
func (c *Cache) getPath(key string) string {
	hash := sha256.Sum256([]byte(key))
	filename := hex.EncodeToString(hash[:]) + ".json"
	return filepath.Join(c.baseDir, filename)
}

// GenerateKey creates a cache key from components
func GenerateKey(components ...string) string {
	key := ""
	for i, comp := range components {
		if i > 0 {
			key += ":"
		}
		key += comp
	}
	return key
}
