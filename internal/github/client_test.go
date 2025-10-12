package github

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"MaxRetries", config.MaxRetries, DefaultMaxRetries},
		{"BaseDelay", config.BaseDelay, DefaultBaseDelay},
		{"MaxDelay", config.MaxDelay, DefaultMaxDelay},
		{"Timeout", config.Timeout, DefaultTimeout},
		{"CacheTTL", config.CacheTTL, DefaultCacheTTL},
		{"EnableCache", config.EnableCache, true},
		{"HTTPClient", config.HTTPClient, (*http.Client)(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("DefaultConfig().%s = %v, expected %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestWithConfig(t *testing.T) {
	customConfig := &Config{
		MaxRetries:  5,
		BaseDelay:   2 * time.Second,
		MaxDelay:    60 * time.Second,
		Timeout:     45 * time.Second,
		CacheTTL:    10 * time.Minute,
		EnableCache: false,
	}

	client := &Client{config: DefaultConfig()}
	opt := WithConfig(customConfig)

	if err := opt(client); err != nil {
		t.Fatalf("WithConfig returned error: %v", err)
	}

	if client.config != customConfig {
		t.Errorf("WithConfig did not set config correctly")
	}
}

func TestWithConfigNil(t *testing.T) {
	original := DefaultConfig()
	client := &Client{config: original}
	opt := WithConfig(nil)

	if err := opt(client); err != nil {
		t.Fatalf("WithConfig(nil) returned error: %v", err)
	}

	if client.config != original {
		t.Errorf("WithConfig(nil) should not change config")
	}
}

func TestWithTimeout(t *testing.T) {
	client := &Client{config: DefaultConfig()}
	timeout := 45 * time.Second
	opt := WithTimeout(timeout)

	if err := opt(client); err != nil {
		t.Fatalf("WithTimeout returned error: %v", err)
	}

	if client.config.Timeout != timeout {
		t.Errorf("WithTimeout: got %v, expected %v", client.config.Timeout, timeout)
	}
}

func TestWithMaxRetries(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"positive value", 5, 5},
		{"zero value", 0, 0},
		{"negative value", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{config: DefaultConfig()}
			opt := WithMaxRetries(tt.input)

			if err := opt(client); err != nil {
				t.Fatalf("WithMaxRetries returned error: %v", err)
			}

			if client.config.MaxRetries != tt.expected {
				t.Errorf("WithMaxRetries(%d): got %d, expected %d", tt.input, client.config.MaxRetries, tt.expected)
			}
		})
	}
}

func TestWithBaseDelay(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected time.Duration
	}{
		{"positive value", 2 * time.Second, 2 * time.Second},
		{"zero value", 0, 0},
		{"negative value", -1 * time.Second, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{config: DefaultConfig()}
			opt := WithBaseDelay(tt.input)

			if err := opt(client); err != nil {
				t.Fatalf("WithBaseDelay returned error: %v", err)
			}

			if client.config.BaseDelay != tt.expected {
				t.Errorf("WithBaseDelay(%v): got %v, expected %v", tt.input, client.config.BaseDelay, tt.expected)
			}
		})
	}
}

func TestWithMaxDelay(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected time.Duration
	}{
		{"positive value", 60 * time.Second, 60 * time.Second},
		{"zero value", 0, 0},
		{"negative value", -1 * time.Second, DefaultMaxDelay},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{config: DefaultConfig()}
			opt := WithMaxDelay(tt.input)

			if err := opt(client); err != nil {
				t.Fatalf("WithMaxDelay returned error: %v", err)
			}

			if client.config.MaxDelay != tt.expected {
				t.Errorf("WithMaxDelay(%v): got %v, expected %v", tt.input, client.config.MaxDelay, tt.expected)
			}
		})
	}
}

func TestWithCacheTTL(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected time.Duration
	}{
		{"positive value", 10 * time.Minute, 10 * time.Minute},
		{"zero value", 0, 0},
		{"negative value", -1 * time.Minute, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{config: DefaultConfig()}
			opt := WithCacheTTL(tt.input)

			if err := opt(client); err != nil {
				t.Fatalf("WithCacheTTL returned error: %v", err)
			}

			if client.config.CacheTTL != tt.expected {
				t.Errorf("WithCacheTTL(%v): got %v, expected %v", tt.input, client.config.CacheTTL, tt.expected)
			}
		})
	}
}

func TestWithCache(t *testing.T) {
	client := &Client{config: DefaultConfig()}
	cache := &NoOpCache{}
	opt := WithCache(cache)

	if err := opt(client); err != nil {
		t.Fatalf("WithCache returned error: %v", err)
	}

	if client.cache != cache {
		t.Errorf("WithCache did not set cache correctly")
	}

	if !client.config.EnableCache {
		t.Errorf("WithCache should enable caching")
	}
}

func TestWithCacheNil(t *testing.T) {
	client := &Client{config: DefaultConfig()}
	original := client.cache
	opt := WithCache(nil)

	if err := opt(client); err != nil {
		t.Fatalf("WithCache(nil) returned error: %v", err)
	}

	if client.cache != original {
		t.Errorf("WithCache(nil) should not change cache")
	}
}

func TestWithoutCache(t *testing.T) {
	client := &Client{config: DefaultConfig()}
	opt := WithoutCache()

	if err := opt(client); err != nil {
		t.Fatalf("WithoutCache returned error: %v", err)
	}

	if client.config.EnableCache {
		t.Errorf("WithoutCache should disable caching")
	}

	if _, ok := client.cache.(*NoOpCache); !ok {
		t.Errorf("WithoutCache should set NoOpCache")
	}
}

func TestWithHTTPClient(t *testing.T) {
	client := &Client{config: DefaultConfig()}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	opt := WithHTTPClient(httpClient)

	if err := opt(client); err != nil {
		t.Fatalf("WithHTTPClient returned error: %v", err)
	}

	if client.config.HTTPClient != httpClient {
		t.Errorf("WithHTTPClient did not set HTTP client correctly")
	}
}

func TestWithRepository(t *testing.T) {
	client := &Client{config: DefaultConfig()}
	owner := "facebook"
	name := "react"
	opt := WithRepository(owner, name)

	if err := opt(client); err != nil {
		t.Fatalf("WithRepository returned error: %v", err)
	}

	if client.repo == nil {
		t.Fatalf("WithRepository did not set repository")
	}

	if client.repo.Owner != owner {
		t.Errorf("WithRepository: got owner %s, expected %s", client.repo.Owner, owner)
	}

	if client.repo.Name != name {
		t.Errorf("WithRepository: got name %s, expected %s", client.repo.Name, name)
	}
}

func TestRepositoryString(t *testing.T) {
	tests := []struct {
		name     string
		repo     *Repository
		expected string
	}{
		{"valid repository", &Repository{Owner: "facebook", Name: "react"}, "facebook/react"},
		{"nil repository", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.repo.String()
			if got != tt.expected {
				t.Errorf("Repository.String(): got %s, expected %s", got, tt.expected)
			}
		})
	}
}

func TestClientImplementsGitHubClient(t *testing.T) {
	var _ GitHubClient = (*Client)(nil)
}

func TestClientAccessorMethods(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	t.Run("REST returns REST client", func(t *testing.T) {
		restClient := client.REST()
		if restClient == nil {
			t.Error("REST() returned nil")
		}
	})

	t.Run("GraphQL returns GraphQL client", func(t *testing.T) {
		graphqlClient := client.GraphQL()
		if graphqlClient == nil {
			t.Error("GraphQL() returned nil")
		}
	})

	t.Run("Repository returns repository context", func(t *testing.T) {
		// May be nil if not in a repo
		_ = client.Repository()
	})
}

func TestClientCacheManagement(t *testing.T) {
	t.Run("CacheStats returns stats", func(t *testing.T) {
		client, err := NewClient()
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Close()

		stats := client.CacheStats()
		if stats.Hits < 0 || stats.Misses < 0 {
			t.Error("CacheStats returned negative values")
		}
	})

	t.Run("ClearCache clears the cache", func(t *testing.T) {
		client, err := NewClient()
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Close()

		// Add something to cache
		client.cache.Set("test", "value", 1*time.Minute)

		// Clear cache
		client.ClearCache()

		// Verify it's cleared
		_, found := client.cache.Get("test")
		if found {
			t.Error("ClearCache did not clear the cache")
		}
	})

	t.Run("InvalidateCacheKey removes specific key", func(t *testing.T) {
		client, err := NewClient()
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Close()

		// Add something to cache
		client.cache.Set("test", "value", 1*time.Minute)

		// Invalidate specific key
		client.InvalidateCacheKey("test")

		// Verify it's removed
		_, found := client.cache.Get("test")
		if found {
			t.Error("InvalidateCacheKey did not remove the key")
		}
	})

	t.Run("Close stops cache cleanup", func(t *testing.T) {
		client, err := NewClient()
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Should not panic
		err = client.Close()
		if err != nil {
			t.Errorf("Close returned error: %v", err)
		}

		// Multiple closes should not panic
		err = client.Close()
		if err != nil {
			t.Errorf("Second Close returned error: %v", err)
		}
	})

	t.Run("Close with NoOpCache does not panic", func(t *testing.T) {
		client, err := NewClient(WithoutCache())
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Should not panic even with NoOpCache
		err = client.Close()
		if err != nil {
			t.Errorf("Close with NoOpCache returned error: %v", err)
		}
	})
}

func TestClientDoGraphQL(t *testing.T) {
	t.Run("Do placeholder returns nil", func(t *testing.T) {
		client, err := NewClient()
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Close()

		// Do is a placeholder that returns nil
		err = client.Do(nil, "GET", "/test", nil, nil)
		if err != nil {
			t.Errorf("Do() returned error: %v", err)
		}
	})
}
