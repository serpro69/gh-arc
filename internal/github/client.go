package github

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
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

	// Initialize client with defaults
	client := &Client{
		restClient:    restClient,
		graphqlClient: graphqlClient,
		config:        DefaultConfig(),
		cache:         &NoOpCache{}, // Default to no cache, will be replaced if caching is enabled
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

// VerifyAuthentication checks if the client is properly authenticated
// by attempting to retrieve the current user's information
func (c *Client) VerifyAuthentication(ctx context.Context) error {
	_, err := c.CurrentUser(ctx)
	if err != nil {
		return err
	}
	return nil
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
