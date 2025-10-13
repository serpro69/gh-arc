package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, DefaultTTL, c.ttl)
	assert.NotEmpty(t, c.baseDir)
}

func TestSetTTL(t *testing.T) {
	c, err := New()
	require.NoError(t, err)

	customTTL := 5 * time.Minute
	c.SetTTL(customTTL)
	assert.Equal(t, customTTL, c.ttl)
}

func TestSetAndGet(t *testing.T) {
	c, err := New()
	require.NoError(t, err)

	// Clean up after test
	defer func() {
		_ = c.Clear()
	}()

	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	original := testData{Name: "test", Value: 42}
	key := "test-key"

	// Set data
	err = c.Set(key, original)
	require.NoError(t, err)

	// Get data
	var retrieved testData
	hit, err := c.Get(key, &retrieved)
	require.NoError(t, err)
	assert.True(t, hit)
	assert.Equal(t, original, retrieved)
}

func TestGetMiss(t *testing.T) {
	c, err := New()
	require.NoError(t, err)

	var data map[string]string
	hit, err := c.Get("non-existent-key", &data)
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestExpiration(t *testing.T) {
	c, err := New()
	require.NoError(t, err)

	// Set short TTL
	c.SetTTL(100 * time.Millisecond)

	// Clean up after test
	defer func() {
		_ = c.Clear()
	}()

	key := "expiring-key"
	data := map[string]string{"test": "value"}

	// Set data
	err = c.Set(key, data)
	require.NoError(t, err)

	// Should be in cache immediately
	var retrieved map[string]string
	hit, err := c.Get(key, &retrieved)
	require.NoError(t, err)
	assert.True(t, hit)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	hit, err = c.Get(key, &retrieved)
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestDelete(t *testing.T) {
	c, err := New()
	require.NoError(t, err)

	// Clean up after test
	defer func() {
		_ = c.Clear()
	}()

	key := "delete-key"
	data := map[string]string{"test": "value"}

	// Set data
	err = c.Set(key, data)
	require.NoError(t, err)

	// Verify it's in cache
	var retrieved map[string]string
	hit, err := c.Get(key, &retrieved)
	require.NoError(t, err)
	assert.True(t, hit)

	// Delete it
	err = c.Delete(key)
	require.NoError(t, err)

	// Should not be in cache anymore
	hit, err = c.Get(key, &retrieved)
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestClear(t *testing.T) {
	c, err := New()
	require.NoError(t, err)

	// Add multiple entries
	for i := 0; i < 5; i++ {
		key := GenerateKey("test", string(rune('a'+i)))
		err = c.Set(key, map[string]int{"value": i})
		require.NoError(t, err)
	}

	// Clear all
	err = c.Clear()
	require.NoError(t, err)

	// Verify all are gone
	for i := 0; i < 5; i++ {
		key := GenerateKey("test", string(rune('a'+i)))
		var data map[string]int
		hit, err := c.Get(key, &data)
		require.NoError(t, err)
		assert.False(t, hit)
	}
}

func TestCleanExpired(t *testing.T) {
	c, err := New()
	require.NoError(t, err)

	// Set short TTL
	c.SetTTL(100 * time.Millisecond)

	// Clean up after test
	defer func() {
		_ = c.Clear()
	}()

	// Add entries
	key1 := "key1"
	key2 := "key2"
	err = c.Set(key1, map[string]string{"test": "value1"})
	require.NoError(t, err)

	// Wait a bit then add another entry
	time.Sleep(150 * time.Millisecond)
	err = c.Set(key2, map[string]string{"test": "value2"})
	require.NoError(t, err)

	// Clean expired entries
	err = c.CleanExpired()
	require.NoError(t, err)

	// key1 should be gone (expired)
	var data map[string]string
	hit, err := c.Get(key1, &data)
	require.NoError(t, err)
	assert.False(t, hit)

	// key2 should still be there
	hit, err = c.Get(key2, &data)
	require.NoError(t, err)
	assert.True(t, hit)
}

func TestGenerateKey(t *testing.T) {
	tests := []struct {
		name       string
		components []string
		expected   string
	}{
		{
			name:       "single component",
			components: []string{"test"},
			expected:   "test",
		},
		{
			name:       "multiple components",
			components: []string{"repo", "owner", "pr"},
			expected:   "repo:owner:pr",
		},
		{
			name:       "with empty component",
			components: []string{"repo", "", "pr"},
			expected:   "repo::pr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateKey(tt.components...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPath(t *testing.T) {
	c, err := New()
	require.NoError(t, err)

	key := "test-key"
	path := c.getPath(key)

	// Should be in cache directory
	assert.Contains(t, path, CacheDir)

	// Should end with .json
	assert.Equal(t, ".json", filepath.Ext(path))

	// Same key should produce same path
	path2 := c.getPath(key)
	assert.Equal(t, path, path2)

	// Different key should produce different path
	path3 := c.getPath("different-key")
	assert.NotEqual(t, path, path3)
}

func TestInvalidCacheEntry(t *testing.T) {
	c, err := New()
	require.NoError(t, err)

	// Clean up after test
	defer func() {
		_ = c.Clear()
	}()

	key := "invalid-key"
	path := c.getPath(key)

	// Write invalid JSON to cache file
	err = os.WriteFile(path, []byte("invalid json"), 0644)
	require.NoError(t, err)

	// Get should treat as cache miss and clean up
	var data map[string]string
	hit, err := c.Get(key, &data)
	require.NoError(t, err)
	assert.False(t, hit)

	// File should be removed
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}
