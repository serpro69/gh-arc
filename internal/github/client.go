package github

import (
	"context"
	"net/http"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
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
	restClient api.RESTClient

	// graphqlClient is the underlying GraphQL client
	graphqlClient *api.GraphQLClient

	// config holds the client configuration
	config *Config

	// cache stores response cache
	cache Cache

	// repo holds the current repository context
	repo *Repository
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
	REST() api.RESTClient

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
func (c *Client) REST() api.RESTClient {
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

// Do executes an HTTP request with retry logic and caching
// This is a placeholder that will be implemented in later subtasks
func (c *Client) Do(ctx context.Context, method, path string, body interface{}, response interface{}) error {
	// TODO: Implement in subtask 2.3 (retry logic) and 2.4 (caching)
	// For now, just pass through to the REST client
	// Note: body will need to be converted to io.Reader in actual implementation
	return nil
}

// DoGraphQL executes a GraphQL query
// This is a placeholder that will be implemented in later subtasks
func (c *Client) DoGraphQL(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error {
	// TODO: Implement in subtask 2.3 (retry logic)
	return c.graphqlClient.Do(query, variables, response)
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
