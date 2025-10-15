# Architecture Guide

This guide provides a detailed explanation of the `gh-arc` codebase architecture, design patterns, and component interactions.

## Table of Contents

- [Overview](#overview)
- [Architecture Principles](#architecture-principles)
- [Package Structure](#package-structure)
- [Core Components](#core-components)
- [Data Flow](#data-flow)
- [Design Patterns](#design-patterns)
- [Key Subsystems](#key-subsystems)
- [Extension Points](#extension-points)

## Overview

`gh-arc` is a GitHub CLI extension built with Go that implements trunk-based development workflows. The architecture follows standard Go project layout with clear separation between command definitions (`cmd/`) and internal implementation (`internal/`).

### High-Level Architecture

```
┌───────────────────────────────────────────────────────┐
│                   CLI Layer (cmd/)                    │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐   │
│  │  root   │  │  diff   │  │  list   │  │  auth   │   │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘   │
└───────┼────────────┼────────────┼────────────┼────────┘
        │            │            │            │
        └────────────┴─────┬──────┴────────────┘
                           │
┌──────────────────────────┴────────────────────────────┐
│           Business Logic Layer (internal/)            │
│       ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│       │  github  │  │   git    │  │  config  │        │
│       └────┬─────┘  └────┬─────┘  └────┬─────┘        │
│            │             │             │              │
│       ┌────┴─────┐  ┌────┴─────┐  ┌────┴─────┐        │
│       │  cache   │  │ template │  │  logger  │        │
│       └──────────┘  └──────────┘  └──────────┘        │
└──────────────────────────┬────────────────────────────┘
                           │
┌──────────────────────────┴────────────────────────────┐
│              External Dependencies                    │
│       ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│       │  go-gh   │  │  go-git  │  │  viper   │        │
│       └──────────┘  └──────────┘  └──────────┘        │
└───────────────────────────────────────────────────────┘
```

## Architecture Principles

### 1. Separation of Concerns

- **Commands** (`cmd/`) handle CLI interface and user interaction
- **Business Logic** (`internal/`) implements core functionality
- **External Integrations** are abstracted behind interfaces

### 2. Dependency Injection

- Components receive dependencies through constructors
- Enables easy testing with mocks
- Example: `NewClient(opts ...ClientOption)`

### 3. Error Handling

- Use `fmt.Errorf` with `%w` for error wrapping
- Define custom error types for specific scenarios
- Return errors up the call stack, handle at command level

### 4. Configuration Over Convention

- Use Viper for flexible configuration
- Support multiple config sources (files, env vars, flags)
- Provide sensible defaults

### 5. Testability

- Write testable code with clear interfaces
- Use table-driven tests for multiple scenarios
- Mock external dependencies (GitHub API, Git operations)

## Package Structure

### `cmd/` - Command Definitions

Contains Cobra command definitions that handle:
- CLI argument parsing
- Flag handling
- User output formatting
- Error display
- Orchestrating business logic

**Key Files:**
- `root.go` - Root command, global flags, initialization
- `diff.go` - PR creation/update workflow
- `list.go` - PR listing and filtering
- `auth.go` - Authentication verification
- `version.go` - Version information display

### `internal/` - Business Logic

Contains the core implementation organized by domain:

#### `internal/github/` - GitHub API Integration

**Purpose:** Wraps `go-gh` library with enhanced features.

**Key Components:**
- `client.go` - Base client with auth, retry, caching
- `pullrequest.go` - PR CRUD operations
- `ratelimit.go` - Rate limit detection and handling
- `retry.go` - Exponential backoff retry logic
- `cache.go` - Response caching with ETags
- `errors.go` - Custom error types

**Responsibilities:**
- GitHub API authentication via `go-gh`
- REST and GraphQL API calls
- Rate limiting and backoff
- Response caching
- Error handling and recovery

#### `internal/git/` - Git Repository Operations

**Purpose:** Wraps `go-git` library for repository operations.

**Key Components:**
- `git.go` - Repository operations

**Responsibilities:**
- Repository detection and validation
- Branch operations (list, create, get current)
- Commit analysis and parsing
- Diff generation
- Working directory status
- Git config reading

#### `internal/config/` - Configuration Management

**Purpose:** Loads and manages application configuration.

**Key Components:**
- `config.go` - Configuration types and loading

**Responsibilities:**
- Load config from files (.arc.json, .arc.yaml)
- Environment variable support
- Configuration validation
- Default values
- Config file search paths

#### `internal/logger/` - Structured Logging

**Purpose:** Centralized logging with zerolog.

**Key Components:**
- `logger.go` - Logger initialization and access

**Responsibilities:**
- Configure log levels based on verbosity flags
- Structured logging with fields
- JSON and console output formats
- Context-aware logging

#### `internal/template/` - PR Template Management

**Purpose:** Generate, parse, and validate PR templates.

**Key Components:**
- `template.go` - Template generation and parsing

**Responsibilities:**
- Generate PR templates with prefilled data
- Parse user-edited templates
- Validate required fields
- Handle template persistence for --continue
- Support stacking context in templates

#### `internal/cache/` - Generic Caching

**Purpose:** Provides in-memory caching with TTL.

**Key Components:**
- `cache.go` - Thread-safe cache implementation

**Responsibilities:**
- Store key-value pairs with expiration
- ETag support for HTTP caching
- Cache statistics
- Automatic cleanup of expired entries

#### `internal/codeowners/` - CODEOWNERS Parsing

**Purpose:** Parse CODEOWNERS file for reviewer suggestions.

**Key Components:**
- `codeowners.go` - Parser and reviewer suggestion

**Responsibilities:**
- Parse CODEOWNERS file format
- Match file patterns to owners
- Provide reviewer suggestions based on changed files
- Stack-aware reviewer detection

#### `internal/format/` - Output Formatting

**Purpose:** Format data for display to users.

**Key Components:**
- `pr_formatter.go` - PR list table formatting

**Responsibilities:**
- Format PR lists as tables
- Color-coded status indicators
- Responsive column sizing
- JSON output support

#### `internal/filter/` - Data Filtering

**Purpose:** Filter data based on user criteria.

**Key Components:**
- `filter.go` - PR filtering logic

**Responsibilities:**
- Filter PRs by status, author, labels, etc.
- Support complex filter expressions
- Reusable filtering logic

#### `internal/diff/` - Diff Workflow

**Purpose:** Implements diff command workflow logic.

**Key Components:**
- Files for base branch detection, stacking, commit analysis

**Responsibilities:**
- Detect base branch for PRs
- Implement stacking logic
- Analyze commits for PR metadata
- Detect dependent PRs

#### `internal/version/` - Version Management

**Purpose:** Version information and build metadata.

**Key Components:**
- `version.go` - Version constants and build info

**Responsibilities:**
- Store version number
- Build information (commit, date, etc.)
- Version display formatting

## Core Components

### 1. GitHub Client (`internal/github/client.go`)

The GitHub client is the primary interface to the GitHub API.

#### Features

- **Authentication**: Uses `go-gh` for OAuth authentication
- **Rate Limiting**: Detects and respects GitHub rate limits
- **Retry Logic**: Exponential backoff for transient failures
- **Caching**: In-memory cache with ETag support
- **Circuit Breaker**: Prevents excessive retry attempts

#### Usage Pattern

```go
// Create client with default options
client, err := github.NewClient()
if err != nil {
    return fmt.Errorf("failed to create client: %w", err)
}
defer client.Close()

// Create client with custom options
client, err := github.NewClient(
    github.WithTimeout(60 * time.Second),
    github.WithMaxRetries(5),
    github.WithCacheTTL(10 * time.Minute),
)

// Make API calls
ctx := context.Background()
prs, err := client.ListPullRequestsForCurrentRepo(ctx, filters)
```

#### Configuration Options

Functional options pattern for flexibility:
- `WithTimeout(duration)` - Request timeout
- `WithMaxRetries(n)` - Maximum retry attempts
- `WithBaseDelay(duration)` - Base delay for backoff
- `WithMaxDelay(duration)` - Maximum backoff delay
- `WithCacheTTL(duration)` - Cache expiration time
- `WithoutCache()` - Disable caching
- `WithRepository(owner, name)` - Set repo context

#### Thread Safety

The client is **thread-safe** and can be shared across goroutines. The underlying cache uses mutexes for concurrent access.

### 2. Git Repository (`internal/git/git.go`)

Wraps `go-git` for Git operations.

#### Features

- **Repository Detection**: Find and validate Git repositories
- **Branch Operations**: List, create, get current branch
- **Commit Analysis**: Parse commits, get ranges, extract messages
- **Diff Generation**: Generate diffs between branches/commits
- **Working Directory**: Check for uncommitted changes

#### Usage Pattern

```go
// Open repository
repo, err := git.OpenRepository(".")
if err != nil {
    return fmt.Errorf("not a git repository: %w", err)
}

// Get current branch
branch, err := repo.GetCurrentBranch()

// Get commits between branches
commits, err := repo.GetCommitsBetween("main", "feature-branch")

// Get diff
diff, err := repo.GetDiffBetween("main", "feature-branch")
```

#### Thread Safety

The Repository struct is **NOT thread-safe**. The underlying `go-git` repository should not be accessed concurrently. Create separate Repository instances for concurrent operations if needed.

### 3. Configuration System (`internal/config/config.go`)

Uses Viper for flexible configuration management.

#### Configuration Hierarchy (highest to lowest priority)

1. **Command-line flags** (highest priority)
2. **Environment variables** (`GHARC_*` prefix)
3. **Config files** in current directory
4. **Config files** in user config directory (`~/.config/gh-arc/`)
5. **Config files** in system directory (`/etc/gh-arc/`)
6. **Default values** (lowest priority)

#### Configuration Structure

```go
type Config struct {
    GitHub GitHubConfig
    Diff   DiffConfig
    Land   LandConfig
    Output OutputConfig
    Test   TestConfig
    Lint   LintConfig
}
```

#### Usage Pattern

```go
// Load configuration
cfg, err := config.Load()
if err != nil {
    return err
}

// Access configuration values
createAsDraft := cfg.Diff.CreateAsDraft
defaultBranch := cfg.GitHub.DefaultBranch

// Or use Viper directly for dynamic access
enableStacking := viper.GetBool("diff.enableStacking")
```

### 4. Logger (`internal/logger/logger.go`)

Centralized structured logging with zerolog.

#### Features

- **Structured Logging**: Log with contextual fields
- **Multiple Levels**: Debug, Info, Warn, Error
- **Configurable Output**: Console (pretty) or JSON
- **Verbosity Control**: Control via command flags

#### Usage Pattern

```go
// Get logger instance
logger := logger.Get()

// Simple logging
logger.Info().Msg("Processing PR")

// Logging with context
logger.Info().
    Str("repo", "owner/repo").
    Int("pr_number", 123).
    Msg("Creating PR")

// Error logging
logger.Error().
    Err(err).
    Str("branch", "feature").
    Msg("Failed to push branch")

// Debug logging (only shown with -v flag)
logger.Debug().
    Interface("config", cfg).
    Msg("Loaded configuration")
```

## Data Flow

### Example: `gh arc diff` Command Flow

```
1. User runs: gh arc diff --draft

2. cmd/diff.go:runDiff()
   ├─> Load configuration (config.Load())
   ├─> Open Git repository (git.OpenRepository())
   ├─> Create GitHub client (github.NewClient())
   ├─> Get current branch (repo.GetCurrentBranch())
   │
   ├─> Detect base branch (diff.DetectBaseBranch())
   │   ├─> Check for existing PR on current branch
   │   ├─> Determine if stacking (feature → feature vs feature → main)
   │   └─> Return base branch and stacking info
   │
   ├─> Detect dependent PRs (diff.DetectDependentPRs())
   │   └─> Find PRs that target current branch
   │
   ├─> Analyze commits (diff.AnalyzeCommitsForTemplate())
   │   ├─> Get commit range (repo.GetCommitsBetween())
   │   ├─> Parse commit messages (git.ParseCommitMessage())
   │   └─> Generate summary and title suggestions
   │
   ├─> Check for existing PR (client.FindExistingPRForCurrentBranch())
   │
   ├─> IF existing PR and no --edit:
   │   ├─> Update draft status if flags provided
   │   ├─> Push new commits if unpushed
   │   └─> Exit (fast path)
   │
   ├─> Generate PR template (template.Generate())
   │   ├─> Pre-fill title, summary, test plan
   │   ├─> Add reviewer suggestions from CODEOWNERS
   │   ├─> Add stacking context
   │   └─> Return template string
   │
   ├─> Open editor (template.OpenEditor())
   │   ├─> Write template to temp file
   │   ├─> Open $EDITOR
   │   ├─> Wait for user to save and close
   │   └─> Read edited template
   │
   ├─> Parse template (template.ParseTemplate())
   │   ├─> Extract fields (title, summary, reviewers, etc.)
   │   └─> Return structured data
   │
   ├─> Validate template (template.ValidateFields())
   │   ├─> Check required fields (title, test plan)
   │   ├─> If invalid, save for --continue and return error
   │   └─> If valid, continue
   │
   ├─> Create or update PR (client.CreatePullRequest() or client.UpdatePullRequest())
   │   ├─> Push branch to remote if needed
   │   ├─> Make GitHub API call
   │   └─> Handle draft status transitions
   │
   ├─> Assign reviewers (client.AssignReviewers())
   │   └─> Parse and assign users/teams
   │
   └─> Display success message

3. Return to user's shell
```

### Example: `gh arc list` Command Flow

```
1. User runs: gh arc list --state open

2. cmd/list.go:runList()
   ├─> Parse flags (state, author, labels, etc.)
   ├─> Create GitHub client (github.NewClient())
   │
   ├─> Fetch PRs (client.ListPullRequestsForCurrentRepo())
   │   ├─> Build GraphQL query with filters
   │   ├─> Check cache for recent results
   │   ├─> If cache miss, make API call with retry
   │   ├─> Parse response
   │   ├─> Store in cache for future requests
   │   └─> Return PR list
   │
   ├─> For each PR, fetch additional metadata:
   │   ├─> Get review states (client.GetReviewsForPR())
   │   ├─> Get CI/check statuses (client.GetCheckRunsForPR())
   │   └─> Aggregate data
   │
   ├─> Apply filters (filter.FilterPRs())
   │   ├─> Filter by state (open, closed, merged)
   │   ├─> Filter by author
   │   ├─> Filter by labels
   │   └─> Return filtered list
   │
   ├─> Format output (format.FormatPRTable())
   │   ├─> IF --json flag:
   │   │   └─> Return JSON output
   │   ├─> ELSE:
   │   │   ├─> Build table with columns (PR#, Title, Status, etc.)
   │   │   ├─> Add color coding
   │   │   └─> Return formatted table
   │
   └─> Print to stdout

3. Return to user's shell
```

## Design Patterns

### 1. Functional Options Pattern

Used for configurable constructors, especially in GitHub client.

**Why:** Provides flexible, extensible configuration without breaking changes.

```go
// Definition
type ClientOption func(*Client) error

func WithTimeout(timeout time.Duration) ClientOption {
    return func(c *Client) error {
        c.config.Timeout = timeout
        return nil
    }
}

// Usage
client, err := NewClient(
    WithTimeout(30*time.Second),
    WithMaxRetries(3),
)
```

### 2. Repository Pattern

Used for data access abstraction (GitHub API, Git operations).

**Why:** Separates data access logic from business logic.

```go
// Client acts as a repository for GitHub data
type Client struct {
    // ...
}

func (c *Client) ListPullRequests(ctx context.Context) ([]*PullRequest, error)
func (c *Client) GetPullRequest(ctx context.Context, number int) (*PullRequest, error)
func (c *Client) CreatePullRequest(ctx context.Context, opts *PROptions) (*PullRequest, error)
```

### 3. Builder Pattern

Used for constructing complex objects (templates, queries).

**Why:** Provides fluent API for complex object construction.

```go
// Template generation
gen := template.NewTemplateGenerator(
    stackingContext,
    commitAnalysis,
    reviewers,
    linearEnabled,
    draftDefault,
)
templateContent := gen.Generate()
```

### 4. Strategy Pattern

Used for different behaviors (caching, retry policies).

**Why:** Allows runtime selection of algorithms.

```go
// Cache interface allows different implementations
type Cache interface {
    Get(key string) (interface{}, bool)
    Set(key string, value interface{}, ttl time.Duration)
    // ...
}

// Multiple implementations
type MemoryCache struct { /* ... */ }
type NoOpCache struct { /* ... */ }
```

### 5. Decorator Pattern

Used for enhancing client behavior (retry, caching, rate limiting).

**Why:** Adds responsibilities without modifying core logic.

```go
// Client.Do() decorates REST calls with:
// - Cache checking
// - Circuit breaker
// - Retry logic
// - Rate limiting
func (c *Client) Do(ctx context.Context, method, path string, body, response interface{}) error {
    // Check circuit breaker
    // Check cache
    // Make request with retry
    // Store in cache
}
```

## Key Subsystems

### Retry and Rate Limiting

**File:** `internal/github/retry.go`, `internal/github/ratelimit.go`

**Purpose:** Handle transient failures and respect GitHub rate limits.

**Key Components:**
- `RetryPolicy` - Defines retry behavior
- `IsRetryableError()` - Determines if error should be retried
- `calculateBackoff()` - Exponential backoff calculation
- `CircuitBreaker` - Prevents excessive retries

**Algorithm:**
1. Attempt request
2. If error, check if retryable (rate limit, network, 5xx)
3. If retryable and attempts remain, wait with exponential backoff
4. Retry request
5. If max retries exceeded, return error

**Exponential Backoff Formula:**
```
delay = min(baseDelay * 2^attempt, maxDelay)
```

### Response Caching

**File:** `internal/github/cache.go`

**Purpose:** Reduce API calls by caching responses with ETags.

**Key Features:**
- In-memory cache with TTL
- ETag support for conditional requests
- Thread-safe with mutex protection
- Automatic cleanup of expired entries

**Flow:**
1. Before request, check cache for key
2. If cached and ETag exists, add `If-None-Match` header
3. Make request
4. If 304 Not Modified, return cached response
5. If 200 OK, store response and ETag in cache
6. Background goroutine cleans expired entries

### PR Stacking

**File:** `internal/diff/` (multiple files)

**Purpose:** Enable trunk-based development with stacked PRs.

**Key Concepts:**
- **Stacking**: Creating PRs that target feature branches instead of main
- **Parent PR**: The PR that the current branch's PR targets
- **Dependent PRs**: PRs that target the current branch

**Detection Logic:**
1. Get current branch
2. Check if PR exists for current branch
3. Determine base branch:
   - If `--base` flag provided, use that
   - Otherwise, check for existing PR on potential parent branches
   - If found, current PR should stack (feature → feature)
   - If not found, target trunk (feature → main)
4. Warn about dependent PRs if updating parent

**Example Scenario:**
```
main
 └─> feature/auth (PR #1 → main)
      └─> feature/auth-tests (PR #2 → feature/auth)  [STACKED]

If you update feature/auth, you'll see:
⚠️  Warning: 1 dependent PR targets this branch
```

### Template System

**File:** `internal/template/template.go`

**Purpose:** Generate, edit, and parse PR templates.

**Components:**
1. **Template Generator**: Creates pre-filled templates
2. **Editor Integration**: Opens $EDITOR for user input
3. **Parser**: Extracts structured data from templates
4. **Validator**: Ensures required fields are present
5. **Persistence**: Saves templates for `--continue` flow

**Template Format:**
```
PR Title: <title>

Summary: <summary>

Test Plan: <test plan>

Reviewers: <reviewer1>, <reviewer2>

Draft: <true|false>

Ref: <Linear-123>
```

**Validation Rules:**
- Title is required
- Test Plan is required (unless disabled in config)
- Reviewers are optional
- Draft defaults to config value
- Ref is optional (Linear integration)

## Extension Points

### Adding New Commands

1. **Create command file** in `cmd/`
   ```go
   // cmd/mycommand.go
   package cmd

   import "github.com/spf13/cobra"

   var myCmd = &cobra.Command{
       Use:   "mycommand",
       Short: "Description",
       RunE:  runMyCommand,
   }

   func init() {
       rootCmd.AddCommand(myCmd)
       // Add flags
   }

   func runMyCommand(cmd *cobra.Command, args []string) error {
       // Implementation
       return nil
   }
   ```

2. **Add tests** in `cmd/mycommand_test.go`

3. **Update documentation** in README.md

### Adding New GitHub API Operations

1. **Add method to Client** in `internal/github/client.go` or new file
   ```go
   // internal/github/myoperation.go
   func (c *Client) MyOperation(ctx context.Context, params) (*Result, error) {
       // Implementation using c.Do() or c.DoGraphQL()
   }
   ```

2. **Add tests** in `internal/github/myoperation_test.go`

3. **Document in this guide**

### Adding New Configuration Options

1. **Add to Config struct** in `internal/config/config.go`
   ```go
   type DiffConfig struct {
       // Existing fields...
       MyNewOption bool `mapstructure:"myNewOption"`
   }
   ```

2. **Set default** in `setDefaults()`
   ```go
   viper.SetDefault("diff.myNewOption", false)
   ```

3. **Document** in README.md configuration section

4. **Add tests** in `internal/config/config_test.go`

### Adding New Error Types

1. **Define error type** in `internal/github/errors.go` or relevant package
   ```go
   type MyError struct {
       Message string
       Cause   error
   }

   func (e *MyError) Error() string {
       return fmt.Sprintf("my error: %s", e.Message)
   }

   func (e *MyError) Unwrap() error {
       return e.Cause
   }
   ```

2. **Use `errors.Is()` and `errors.As()` for type checking**
   ```go
   var myErr *MyError
   if errors.As(err, &myErr) {
       // Handle specific error
   }
   ```

---

This architecture guide should be updated as the codebase evolves. When making significant architectural changes, document them here for future contributors.
