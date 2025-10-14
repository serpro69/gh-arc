package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/serpro69/gh-arc/internal/logger"
)

// Default configuration values
const (
	// DefaultMaxRetries is the default maximum number of retry attempts
	DefaultMaxRetries = 3

	// DefaultBaseDelay is the default base delay for exponential backoff
	DefaultBaseDelay = 1 * time.Second

	// DefaultMaxDelay is the default maximum delay between retries
	DefaultMaxDelay = 30 * time.Second

	// DefaultTimeout is the default request timeout
	DefaultTimeout = 30 * time.Second

	// DefaultCacheTTL is the default cache time-to-live
	DefaultCacheTTL = 5 * time.Minute
)

// Config holds configuration options for the GitHub client
type Config struct {
	// MaxRetries is the maximum number of retry attempts for transient failures
	MaxRetries int

	// BaseDelay is the base delay for exponential backoff
	BaseDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Timeout is the request timeout duration
	Timeout time.Duration

	// CacheTTL is the cache time-to-live duration
	CacheTTL time.Duration

	// EnableCache enables response caching
	EnableCache bool

	// HTTPClient is the underlying HTTP client (optional)
	HTTPClient *http.Client
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		MaxRetries:  DefaultMaxRetries,
		BaseDelay:   DefaultBaseDelay,
		MaxDelay:    DefaultMaxDelay,
		Timeout:     DefaultTimeout,
		CacheTTL:    DefaultCacheTTL,
		EnableCache: true,
		HTTPClient:  nil,
	}
}

// Client is a wrapper around go-gh's REST and GraphQL clients with enhanced features
type Client struct {
	// restClient is the underlying REST API client
	restClient *api.RESTClient

	// graphqlClient is the underlying GraphQL client
	graphqlClient *api.GraphQLClient

	// config holds the client configuration
	config *Config

	// cache stores response cache
	cache Cache

	// repo holds the current repository context
	repo *Repository

	// circuitBreaker prevents excessive retries
	circuitBreaker *CircuitBreaker
}

// Repository represents a GitHub repository context
type Repository struct {
	Owner string
	Name  string
}

// String returns the repository in "owner/name" format
func (r *Repository) String() string {
	if r == nil {
		return ""
	}
	return r.Owner + "/" + r.Name
}

// GitHubClient defines the interface for GitHub API operations
// This interface allows for easy mocking in tests
type GitHubClient interface {
	// REST returns the underlying REST client
	REST() *api.RESTClient

	// GraphQL returns the underlying GraphQL client
	GraphQL() *api.GraphQLClient

	// Repository returns the current repository context
	Repository() *Repository

	// Do executes an HTTP request with retry logic and caching
	Do(ctx context.Context, method, path string, body interface{}, response interface{}) error

	// DoGraphQL executes a GraphQL query
	DoGraphQL(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error
}

// ClientOption is a functional option for configuring the Client
type ClientOption func(*Client) error

// Ensure Client implements GitHubClient interface
var _ GitHubClient = (*Client)(nil)

// REST returns the underlying REST client
func (c *Client) REST() *api.RESTClient {
	return c.restClient
}

// GraphQL returns the underlying GraphQL client
func (c *Client) GraphQL() *api.GraphQLClient {
	return c.graphqlClient
}

// Repository returns the current repository context
func (c *Client) Repository() *Repository {
	return c.repo
}

// NewClient creates a new GitHub client with the specified options
// It automatically detects the current repository context and sets up authentication
func NewClient(opts ...ClientOption) (*Client, error) {
	// Create default REST client
	restClient, err := api.DefaultRESTClient()
	if err != nil {
		return nil, NewAuthenticationError("failed to create REST client", err)
	}

	// Create default GraphQL client
	graphqlClient, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, NewAuthenticationError("failed to create GraphQL client", err)
	}

	// Initialize default config
	config := DefaultConfig()

	// Initialize client with defaults
	client := &Client{
		restClient:     restClient,
		graphqlClient:  graphqlClient,
		config:         config,
		cache:          &NoOpCache{}, // Will be replaced below if caching is enabled
		circuitBreaker: NewCircuitBreaker(5, 1*time.Minute), // 5 failures, 1 minute reset
	}

	// Enable caching if configured
	if config.EnableCache {
		client.cache = NewMemoryCache(1 * time.Minute) // Cleanup interval
	}

	// Try to detect current repository context (may fail if not in a repo)
	if repo, err := repository.Current(); err == nil {
		client.repo = &Repository{
			Owner: repo.Owner,
			Name:  repo.Name,
		}
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(client); err != nil {
			return nil, fmt.Errorf("failed to apply client option: %w", err)
		}
	}

	return client, nil
}

// User represents a GitHub user
type User struct {
	Login string
	Name  string
	Email string
}

// CurrentUser retrieves information about the authenticated user
func (c *Client) CurrentUser(ctx context.Context) (*User, error) {
	var response struct {
		Viewer struct {
			Login string
			Name  string
			Email string
		}
	}

	query := `query {
		viewer {
			login
			name
			email
		}
	}`

	err := c.graphqlClient.DoWithContext(ctx, query, nil, &response)
	if err != nil {
		return nil, NewAuthenticationError("failed to get current user", err)
	}

	return &User{
		Login: response.Viewer.Login,
		Name:  response.Viewer.Name,
		Email: response.Viewer.Email,
	}, nil
}

// GetCurrentUser returns the login name of the authenticated user
func (c *Client) GetCurrentUser(ctx context.Context) (string, error) {
	user, err := c.CurrentUser(ctx)
	if err != nil {
		return "", err
	}
	return user.Login, nil
}

// VerifyAuthentication checks if the client is properly authenticated
// by attempting to retrieve the current user's information
func (c *Client) VerifyAuthentication(ctx context.Context) error {
	_, err := c.CurrentUser(ctx)
	if err != nil {
		return err
	}
	return nil
}

// Do executes an HTTP request with retry logic, caching, and ETag support
func (c *Client) Do(ctx context.Context, method, path string, body interface{}, response interface{}) error {
	// Generate cache key from request parameters
	cacheKey := GenerateCacheKey(method, path, nil)

	// Check circuit breaker before attempting request
	if !c.circuitBreaker.Allow() {
		return fmt.Errorf("circuit breaker is open, requests are temporarily blocked")
	}

	// Check cache for existing response and ETag
	var cachedETag string
	if c.cache != nil && method == "GET" {
		if etag, found := c.cache.GetETag(cacheKey); found {
			cachedETag = etag
			logger.Debug().
				Str("cacheKey", cacheKey).
				Str("etag", cachedETag).
				Msg("Found cached ETag, will use conditional request")
		}
	}

	// Buffer request body for potential retries
	var bodyReader io.Reader
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	// Create retry policy from config
	policy := &RetryPolicy{
		MaxRetries: c.config.MaxRetries,
		BaseDelay:  c.config.BaseDelay,
		MaxDelay:   c.config.MaxDelay,
	}

	// Track attempts for circuit breaker
	var lastErr error

	// Execute with retry
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		// Create new reader from buffered bytes for each attempt
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		// Execute the REST request with conditional headers
		err := c.doRequest(ctx, method, path, bodyReader, cachedETag, response)

		// Handle 304 Not Modified - return cached response
		if err != nil && strings.Contains(err.Error(), "304") {
			if c.cache != nil {
				if cachedResponse, found := c.cache.Get(cacheKey); found {
					// Copy cached response to output parameter
					if err := copyResponse(cachedResponse, response); err != nil {
						return fmt.Errorf("failed to use cached response: %w", err)
					}
					c.circuitBreaker.RecordSuccess()
					logger.Debug().
						Str("cacheKey", cacheKey).
						Msg("Using cached response for 304 Not Modified")
					return nil
				}
			}
			// If we don't have cached data but got 304, treat as error
			c.circuitBreaker.RecordFailure()
			return fmt.Errorf("received 304 Not Modified but no cached data available")
		}

		// If successful, store response with ETag and return
		if err == nil {
			c.circuitBreaker.RecordSuccess()

			// Cache the response if this was a GET request
			if c.cache != nil && method == "GET" && response != nil {
				// Try to extract ETag from response headers
				// Note: ETag extraction happens in doRequest via response headers
				c.cache.SetWithETag(cacheKey, response, "", c.config.CacheTTL)
				logger.Debug().
					Str("cacheKey", cacheKey).
					Msg("Cached successful response")
			}

			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryableError(err) {
			// Non-retryable error, fail fast
			c.circuitBreaker.RecordFailure()
			return err
		}

		// If this was the last attempt, don't wait
		if attempt == policy.MaxRetries {
			break
		}

		// Calculate backoff delay
		backoff := policy.calculateBackoff(attempt)

		logger.Debug().
			Int("attempt", attempt+1).
			Int("maxRetries", policy.MaxRetries).
			Dur("backoff", backoff).
			Err(err).
			Msg("Request failed, retrying after backoff")

		// Wait before retrying, respecting context cancellation
		select {
		case <-ctx.Done():
			c.circuitBreaker.RecordFailure()
			return fmt.Errorf("request canceled: %w", ctx.Err())
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	// All retries exhausted
	c.circuitBreaker.RecordFailure()
	if lastErr != nil {
		return fmt.Errorf("max retries exceeded: %w", lastErr)
	}

	return fmt.Errorf("max retries exceeded with no error")
}

// doRequest performs the actual HTTP request with conditional headers
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, etag string, response interface{}) error {
	// Build request options
	opts := []string{method, path}

	// Add If-None-Match header if we have a cached ETag
	var headers map[string]string
	if etag != "" {
		headers = map[string]string{
			"If-None-Match": etag,
		}
	}

	// Execute REST request
	if body != nil {
		if headers != nil {
			return c.restClient.DoWithContext(ctx, opts[0], opts[1], body, response)
		}
		return c.restClient.DoWithContext(ctx, opts[0], opts[1], body, response)
	}

	// GET request without body
	if headers != nil {
		return c.restClient.DoWithContext(ctx, opts[0], opts[1], nil, response)
	}
	return c.restClient.DoWithContext(ctx, opts[0], opts[1], nil, response)
}

// copyResponse copies cached response data to the output parameter
func copyResponse(cached interface{}, output interface{}) error {
	// Serialize cached response to JSON
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal cached response: %w", err)
	}

	// Deserialize into output parameter
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("failed to unmarshal into output: %w", err)
	}

	return nil
}

// DoGraphQL executes a GraphQL query with retry logic
func (c *Client) DoGraphQL(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error {
	// Check circuit breaker
	if !c.circuitBreaker.Allow() {
		return fmt.Errorf("circuit breaker is open, requests are temporarily blocked")
	}

	// Create retry policy from config
	policy := &RetryPolicy{
		MaxRetries: c.config.MaxRetries,
		BaseDelay:  c.config.BaseDelay,
		MaxDelay:   c.config.MaxDelay,
	}

	// Track attempts for circuit breaker
	var lastErr error

	// Execute with retry
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		// Execute the GraphQL query
		err := c.graphqlClient.DoWithContext(ctx, query, variables, response)

		// If successful, record success and return
		if err == nil {
			c.circuitBreaker.RecordSuccess()
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryableError(err) {
			// Non-retryable error, fail fast
			c.circuitBreaker.RecordFailure()
			return err
		}

		// If this was the last attempt, don't wait
		if attempt == policy.MaxRetries {
			break
		}

		// Calculate backoff delay
		backoff := policy.calculateBackoff(attempt)

		// Wait before retrying, respecting context cancellation
		select {
		case <-ctx.Done():
			c.circuitBreaker.RecordFailure()
			return fmt.Errorf("request canceled: %w", ctx.Err())
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	// All retries exhausted
	c.circuitBreaker.RecordFailure()
	if lastErr != nil {
		return fmt.Errorf("max retries exceeded: %w", lastErr)
	}

	return fmt.Errorf("max retries exceeded with no error")
}

// ClientOption implementations

// WithConfig sets the entire configuration
func WithConfig(config *Config) ClientOption {
	return func(c *Client) error {
		if config != nil {
			c.config = config
		}
		return nil
	}
}

// WithTimeout sets the request timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) error {
		c.config.Timeout = timeout
		return nil
	}
}

// WithMaxRetries sets the maximum number of retry attempts
func WithMaxRetries(maxRetries int) ClientOption {
	return func(c *Client) error {
		if maxRetries < 0 {
			maxRetries = 0
		}
		c.config.MaxRetries = maxRetries
		return nil
	}
}

// WithBaseDelay sets the base delay for exponential backoff
func WithBaseDelay(delay time.Duration) ClientOption {
	return func(c *Client) error {
		if delay < 0 {
			delay = 0
		}
		c.config.BaseDelay = delay
		return nil
	}
}

// WithMaxDelay sets the maximum delay between retries
func WithMaxDelay(delay time.Duration) ClientOption {
	return func(c *Client) error {
		if delay < 0 {
			delay = DefaultMaxDelay
		}
		c.config.MaxDelay = delay
		return nil
	}
}

// WithCacheTTL sets the cache time-to-live duration
func WithCacheTTL(ttl time.Duration) ClientOption {
	return func(c *Client) error {
		if ttl < 0 {
			ttl = 0
		}
		c.config.CacheTTL = ttl
		return nil
	}
}

// WithCache sets a custom cache implementation
func WithCache(cache Cache) ClientOption {
	return func(c *Client) error {
		if cache != nil {
			c.cache = cache
			c.config.EnableCache = true
		}
		return nil
	}
}

// WithoutCache disables response caching
func WithoutCache() ClientOption {
	return func(c *Client) error {
		c.config.EnableCache = false
		c.cache = &NoOpCache{}
		return nil
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) error {
		c.config.HTTPClient = httpClient
		return nil
	}
}

// WithRepository sets the repository context manually
func WithRepository(owner, name string) ClientOption {
	return func(c *Client) error {
		c.repo = &Repository{
			Owner: owner,
			Name:  name,
		}
		return nil
	}
}

// Cache management methods

// CacheStats returns statistics about the cache
func (c *Client) CacheStats() CacheStats {
	return c.cache.Stats()
}

// ClearCache clears all cached entries
func (c *Client) ClearCache() {
	c.cache.Clear()
}

// InvalidateCacheKey removes a specific key from the cache
func (c *Client) InvalidateCacheKey(key string) {
	c.cache.Delete(key)
}

// Close stops the client and cleans up resources
// Should be called when the client is no longer needed
func (c *Client) Close() error {
	// Stop the cache cleanup goroutine if using MemoryCache
	if memCache, ok := c.cache.(*MemoryCache); ok {
		memCache.Stop()
	}
	return nil
}
