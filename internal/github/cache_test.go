package github

import (
	"testing"
	"time"
)

func TestNoOpCache(t *testing.T) {
	cache := &NoOpCache{}

	t.Run("Get always returns not found", func(t *testing.T) {
		value, found := cache.Get("test-key")
		if found {
			t.Errorf("NoOpCache.Get should always return found=false, got true")
		}
		if value != nil {
			t.Errorf("NoOpCache.Get should return nil value, got %v", value)
		}
	})

	t.Run("Set does nothing", func(t *testing.T) {
		// Should not panic
		cache.Set("test-key", "test-value", 1*time.Minute)

		// Verify it's still not found
		_, found := cache.Get("test-key")
		if found {
			t.Errorf("NoOpCache.Set should not store values")
		}
	})

	t.Run("Delete does nothing", func(t *testing.T) {
		// Should not panic
		cache.Delete("test-key")
	})

	t.Run("Clear does nothing", func(t *testing.T) {
		// Should not panic
		cache.Clear()
	})
}

func TestNoOpCacheImplementsCache(t *testing.T) {
	var _ Cache = (*NoOpCache)(nil)
}
