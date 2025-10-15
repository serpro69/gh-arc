# Testing Guide

This guide provides comprehensive testing guidelines and examples for contributors who may not be familiar with good test design practices.

## Table of Contents

- [Testing Philosophy](#testing-philosophy)
- [Running Tests](#running-tests)
- [Test Organization](#test-organization)
- [Writing Tests](#writing-tests)
- [Table-Driven Tests](#table-driven-tests)
- [Unit Testing](#unit-testing)
- [Integration Testing](#integration-testing)
- [Mocking and Test Doubles](#mocking-and-test-doubles)
- [Test Coverage](#test-coverage)
- [Common Pitfalls](#common-pitfalls)
- [Best Practices](#best-practices)

## Testing Philosophy

### Test-Driven Development (TDD)

We follow TDD principles:

1. **Red**: Write a failing test
2. **Green**: Write minimal code to make it pass
3. **Refactor**: Improve code while keeping tests green

### Testing Pyramid

```
         / \
        /   \
       / E2E \        Few, slow, expensive
      /_______\
     /         \
    /Integration\     More, moderate speed
   /_____________\
  /               \
 /      Unit       \  Many, fast, cheap
/___________________\
```

- **Unit Tests**: Test individual functions/methods in isolation (80%)
- **Integration Tests**: Test components working together (15%)
- **E2E Tests**: Test complete workflows (5%)

### What to Test

- **Happy Paths**: Normal, expected behavior
- **Edge Cases**: Boundary conditions, limits
- **Error Cases**: Invalid input, failure scenarios
- **Integration Points**: Where components interact

### What NOT to Test

- Third-party library internals
- Simple getters/setters
- Generated code
- Trivial code with no logic

## Running Tests

### Basic Commands

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests in a specific package
go test ./internal/github

# Run a specific test
go test -run TestClientNew ./internal/github

# Run tests matching a pattern
go test -run "TestClient.*" ./internal/github
```

###  Coverage

```bash
# Run tests with coverage
go test -cover ./...

# Generate detailed coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Coverage for specific package
go test -cover ./internal/github
```

### Benchmarking

```bash
# Run benchmarks
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkCacheGet ./internal/cache

# With memory allocation stats
go test -bench=. -benchmem ./...
```

### Parallel Testing

```bash
# Run tests in parallel (default)
go test -parallel 4 ./...

# Disable parallelization
go test -parallel 1 ./...
```

## Test Organization

### File Structure

Place tests alongside the code they test:

```
internal/github/
├── client.go
├── client_test.go      # Tests for client.go
├── pullrequest.go
├── pullrequest_test.go # Tests for pullrequest.go
└── errors.go
    └── errors_test.go  # Tests for errors.go
```

### Test Function Naming

```go
// Test function format: Test + FunctionName
func TestNewClient(t *testing.T) { }

// For methods: Test + Type + Method
func TestClientDo(t *testing.T) { }
func TestRepositoryString(t *testing.T) { }
```

### Test Structure (AAA Pattern)

```go
func TestSomething(t *testing.T) {
    // Arrange - Set up test data and conditions
    input := "test input"
    expected := "expected output"

    // Act - Execute the code being tested
    result := FunctionUnderTest(input)

    // Assert - Verify the results
    if result != expected {
        t.Errorf("got %s, expected %s", result, expected)
    }
}
```

## Writing Tests

### Using Standard Library Testing

We use Go's standard library `testing` package for all assertions:

```go
import (
    "testing"
)

func TestExample(t *testing.T) {
    result, err := SomeFunction()

    // Check for errors first (stops execution on fatal errors)
    if err != nil {
        t.Fatalf("SomeFunction() returned error: %v", err)
    }
    if result == nil {
        t.Fatalf("SomeFunction() returned nil result")
    }

    // Check individual properties
    if result.Value != "expected" {
        t.Errorf("result.Value = %s, expected 'expected'", result.Value)
    }
    if !result.IsValid {
        t.Error("result.IsValid = false, expected true")
    }

    found := false
    for _, item := range result.Items {
        if item == "item1" {
            found = true
            break
        }
    }
    if !found {
        t.Errorf("result.Items does not contain 'item1'")
    }
}
```

### When to Use `t.Fatalf()` vs `t.Errorf()`

```go
func TestConfig(t *testing.T) {
    cfg, err := config.Load()

    // Use t.Fatalf when continuing is pointless
    if err != nil {
        t.Fatalf("config.Load() failed: %v", err)  // If Load fails, nothing else matters
    }
    if cfg == nil {
        t.Fatalf("config.Load() returned nil")  // Can't test nil config
    }

    // Use t.Errorf for independent checks (test continues)
    if cfg.GitHub.DefaultBranch != "main" {
        t.Errorf("GitHub.DefaultBranch = %s, expected 'main'", cfg.GitHub.DefaultBranch)
    }
    if !cfg.Diff.EnableStacking {
        t.Error("Diff.EnableStacking = false, expected true")
    }
    if cfg.Diff.CreateAsDraft {
        t.Error("Diff.CreateAsDraft = true, expected false")
    }
}
```

## Table-Driven Tests

Table-driven tests are the Go idiom for testing multiple scenarios.

### Basic Pattern

```go
func TestParseConfigKey(t *testing.T) {
    testCases := []struct {
        name          string
        key           string
        expSection    string
        expSubsection string
        expOption     string
    }{
        {
            name:          "two parts - simple key",
            key:           "user.name",
            expSection:    "user",
            expSubsection: "",
            expOption:     "name",
        },
        {
            name:          "three parts - subsection key",
            key:           "remote.origin.url",
            expSection:    "remote",
            expSubsection: "origin",
            expOption:     "url",
        },
        {
            name:          "invalid key",
            key:           "invalid",
            expSection:    "",
            expSubsection: "",
            expOption:     "",
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            section, subsection, option := parseConfigKey(tc.key)
            if section != tc.expSection {
                t.Errorf("section = %s, expected %s", section, tc.expSection)
            }
            if subsection != tc.expSubsection {
                t.Errorf("subsection = %s, expected %s", subsection, tc.expSubsection)
            }
            if option != tc.expOption {
                t.Errorf("option = %s, expected %s", option, tc.expOption)
            }
        })
    }
}
```

### Why Use Table-Driven Tests?

- **DRY**: Don't repeat test code
- **Easy to Add Cases**: Add a struct, done
- **Clear Intent**: Test cases are data, not code
- **Parallel Execution**: Each case runs independently

### Testing Multiple Inputs/Outputs

```go
func TestWithMaxRetries(t *testing.T) {
    tests := []struct {
        name     string
        input    int
        expected int
    }{
        {"positive value", 5, 5},
        {"zero value", 0, 0},
        {"negative value", -1, 0},  // Negative becomes 0
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            client := &Client{config: DefaultConfig()}
            opt := WithMaxRetries(tt.input)

            if err := opt(client); err != nil {
                t.Fatalf("WithMaxRetries() failed: %v", err)
            }
            if client.config.MaxRetries != tt.expected {
                t.Errorf("MaxRetries = %d, expected %d", client.config.MaxRetries, tt.expected)
            }
        })
    }
}
```

### Testing Error Conditions

```go
func TestValidate(t *testing.T) {
    tests := []struct {
        name    string
        config  Config
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid config",
            config: Config{
                Land: LandConfig{DefaultMergeMethod: "squash"},
            },
            wantErr: false,
        },
        {
            name: "invalid merge method",
            config: Config{
                Land: LandConfig{DefaultMergeMethod: "invalid"},
            },
            wantErr: true,
            errMsg:  "invalid merge method",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()

            if tt.wantErr {
                if err == nil {
                    t.Error("Expected error, got nil")
                } else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
                    t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
                }
            } else {
                if err != nil {
                    t.Errorf("Expected no error, got: %v", err)
                }
            }
        })
    }
}

// Helper function for substring checking
func contains(s, substr string) bool {
    return strings.Contains(s, substr)
}
```

## Unit Testing

### Testing Pure Functions

Pure functions (no side effects) are easiest to test:

```go
func TestParseCommitMessage(t *testing.T) {
    testCases := []struct {
        name     string
        message  string
        expTitle string
        expBody  string
    }{
        {
            name:     "simple commit",
            message:  "Add feature",
            expTitle: "Add feature",
            expBody:  "",
        },
        {
            name:     "commit with body",
            message:  "Add feature\n\nThis adds a new feature",
            expTitle: "Add feature",
            expBody:  "This adds a new feature",
        },
        {
            name:     "empty message",
            message:  "",
            expTitle: "",
            expBody:  "",
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            parsed := ParseCommitMessage(tc.message)
            if parsed.Title != tc.expTitle {
                t.Errorf("Title = %s, expected %s", parsed.Title, tc.expTitle)
            }
            if parsed.Body != tc.expBody {
                t.Errorf("Body = %s, expected %s", parsed.Body, tc.expBody)
            }
        })
    }
}
```

### Testing Functions with State

Functions that modify state need careful setup and cleanup:

```go
func TestLoad(t *testing.T) {
    t.Run("load from JSON config file", func(t *testing.T) {
        // Arrange: Create temp directory for test
        tmpDir := t.TempDir()  // Automatically cleaned up
        os.Chdir(tmpDir)

        // Create test config file
        configContent := `{
            "github": {
                "defaultBranch": "develop"
            }
        }`
        err := os.WriteFile(".arc.json", []byte(configContent), 0644)
        if err != nil {
            t.Fatalf("Failed to write config file: %v", err)
        }

        // Act: Load configuration
        cfg, err := Load()

        // Assert: Verify loaded correctly
        if err != nil {
            t.Fatalf("Expected no error, got: %v", err)
        }
        if cfg.GitHub.DefaultBranch != "develop" {
            t.Errorf("Expected branch 'develop', got '%s'", cfg.GitHub.DefaultBranch)
        }
    })
}
```

### Testing Methods

```go
func TestRepositoryString(t *testing.T) {
    tests := []struct {
        name     string
        repo     *Repository
        expected string
    }{
        {
            name: "valid repository",
            repo: &Repository{Owner: "facebook", Name: "react"},
            expected: "facebook/react",
        },
        {
            name:     "nil repository",
            repo:     nil,
            expected: "",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := tt.repo.String()
            if got != tt.expected {
                t.Errorf("Repository.String() = %s, expected %s", got, tt.expected)
            }
        })
    }
}
```

### Testing Error Handling

```go
func TestOpenRepository(t *testing.T) {
    t.Run("open current repository", func(t *testing.T) {
        repo, err := OpenRepository("../..")
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }
        if repo == nil {
            t.Fatal("OpenRepository() returned nil")
        }
    })

    t.Run("non-existent repository", func(t *testing.T) {
        tmpDir := t.TempDir()
        _, err := OpenRepository(tmpDir)

        // Check for specific error type
        if !errors.Is(err, ErrNotARepository) {
            t.Errorf("Expected ErrNotARepository, got: %v", err)
        }
    })
}
```

## Integration Testing

Integration tests verify multiple components working together.

### Testing with Temporary Git Repositories

```go
func TestGetWorkingDirectoryStatus(t *testing.T) {
    t.Run("dirty repository with untracked file", func(t *testing.T) {
        // Create temporary git repository
        tmpDir := t.TempDir()
        gitRepo, err := git.PlainInit(tmpDir, false)
        if err != nil {
            t.Fatalf("Failed to init repository: %v", err)
        }

        worktree, err := gitRepo.Worktree()
        if err != nil {
            t.Fatalf("Failed to get worktree: %v", err)
        }

        // Create initial commit
        testFile := filepath.Join(tmpDir, "test.txt")
        err = os.WriteFile(testFile, []byte("initial content"), 0644)
        if err != nil {
            t.Fatalf("Failed to write test file: %v", err)
        }

        _, err = worktree.Add("test.txt")
        if err != nil {
            t.Fatalf("Failed to add file: %v", err)
        }

        _, err = worktree.Commit("initial commit", &git.CommitOptions{
            Author: &object.Signature{
                Name:  "Test User",
                Email: "test@example.com",
                When:  time.Now(),
            },
        })
        if err != nil {
            t.Fatalf("Failed to commit: %v", err)
        }

        // Add untracked file
        untrackedFile := filepath.Join(tmpDir, "untracked.txt")
        err = os.WriteFile(untrackedFile, []byte("untracked"), 0644)
        if err != nil {
            t.Fatalf("Failed to write untracked file: %v", err)
        }

        // Test the functionality
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("Failed to open repository: %v", err)
        }

        status, err := repo.GetWorkingDirectoryStatus()
        if err != nil {
            t.Fatalf("Failed to get status: %v", err)
        }

        // Verify results
        if status.IsClean {
            t.Error("Expected status.IsClean to be false")
        }
        found := false
        for _, file := range status.UntrackedFiles {
            if file == "untracked.txt" {
                found = true
                break
            }
        }
        if !found {
            t.Error("Expected status.UntrackedFiles to contain 'untracked.txt'")
        }
    })
}
```

### Testing with Real File System

```go
func TestFindRepositoryRoot(t *testing.T) {
    t.Run("find root from subdirectory", func(t *testing.T) {
        // Get current working directory
        cwd, err := os.Getwd()
        if err != nil {
            t.Fatalf("Failed to get working directory: %v", err)
        }

        // Find repository root
        root, err := FindRepositoryRoot(cwd)
        if err != nil {
            t.Fatalf("Failed to find repository root: %v", err)
        }
        if root == "" {
            t.Fatal("FindRepositoryRoot returned empty string")
        }

        // Verify .git exists in root
        gitDir := filepath.Join(root, ".git")
        info, err := os.Stat(gitDir)
        if err != nil {
            t.Fatalf("Failed to stat .git directory: %v", err)
        }
        if !info.IsDir() {
            t.Error(".git is not a directory")
        }
    })
}
```

### Using Test Fixtures

Create helper functions for common test setups:

```go
// Helper function to create a test repository
func createTestRepo(t *testing.T) (string, *git.Repository) {
    t.Helper()  // Mark as helper function

    tmpDir := t.TempDir()
    gitRepo, err := git.PlainInit(tmpDir, false)
    if err != nil {
        t.Fatalf("Failed to init repository: %v", err)
    }

    // Create initial commit
    worktree, err := gitRepo.Worktree()
    if err != nil {
        t.Fatalf("Failed to get worktree: %v", err)
    }

    testFile := filepath.Join(tmpDir, "README.md")
    err = os.WriteFile(testFile, []byte("# Test"), 0644)
    if err != nil {
        t.Fatalf("Failed to write README: %v", err)
    }

    _, err = worktree.Add("README.md")
    if err != nil {
        t.Fatalf("Failed to add README: %v", err)
    }

    _, err = worktree.Commit("Initial commit", &git.CommitOptions{
        Author: &object.Signature{
            Name:  "Test User",
            Email: "test@example.com",
            When:  time.Now(),
        },
    })
    if err != nil {
        t.Fatalf("Failed to commit: %v", err)
    }

    return tmpDir, gitRepo
}

// Use it in tests
func TestSomething(t *testing.T) {
    tmpDir, gitRepo := createTestRepo(t)
    // Test code here...
}
```

## Mocking and Test Doubles

### When to Mock

Mock external dependencies:
- GitHub API calls
- File system operations (when appropriate)
- Time-dependent code
- Network calls

### Interface-Based Mocking

```go
// Define interface
type GitHubClient interface {
    GetPullRequest(ctx context.Context, number int) (*PullRequest, error)
    ListPullRequests(ctx context.Context) ([]*PullRequest, error)
}

// Real implementation
type Client struct { /* ... */ }

// Mock for testing
type MockGitHubClient struct {
    GetPullRequestFunc    func(ctx context.Context, number int) (*PullRequest, error)
    ListPullRequestsFunc  func(ctx context.Context) ([]*PullRequest, error)
}

func (m *MockGitHubClient) GetPullRequest(ctx context.Context, number int) (*PullRequest, error) {
    if m.GetPullRequestFunc != nil {
        return m.GetPullRequestFunc(ctx, number)
    }
    return nil, errors.New("not implemented")
}

// Use in test
func TestSomethingWithMock(t *testing.T) {
    mock := &MockGitHubClient{
        GetPullRequestFunc: func(ctx context.Context, number int) (*PullRequest, error) {
            return &PullRequest{Number: number, Title: "Test PR"}, nil
        },
    }

    pr, err := mock.GetPullRequest(context.Background(), 123)
    if err != nil {
        t.Fatalf("GetPullRequest() failed: %v", err)
    }
    if pr.Number != 123 {
        t.Errorf("pr.Number = %d, expected 123", pr.Number)
    }
}
```

### Testing with Context

```go
func TestPushWithTimeout(t *testing.T) {
    tmpDir, _ := createTestRepo(t)
    repo, err := OpenRepository(tmpDir)
    if err != nil {
        t.Fatalf("Failed to open repository: %v", err)
    }

    // Create context with very short timeout
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
    defer cancel()

    // Wait for context to timeout
    <-ctx.Done()

    // Push with timed-out context should fail
    err = repo.Push(ctx, "master")
    if err == nil {
        t.Error("Expected Push() to fail with timed out context")
    } else if !strings.Contains(err.Error(), "timed out") {
        t.Errorf("Expected error to contain 'timed out', got: %v", err)
    }
}
```

## Test Coverage

### Measuring Coverage

```bash
# Basic coverage
go test -cover ./...

# Detailed coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Coverage by function
go tool cover -func=coverage.out
```

### Coverage Goals

- **Critical Packages**: 80%+ coverage
  - `internal/github` (GitHub API client)
  - `internal/git` (Git operations)
  - `internal/config` (Configuration)

- **Less Critical**: 60%+ coverage
  - `internal/format` (Formatting)
  - `internal/filter` (Filtering)

- **Commands**: 50%+ coverage
  - `cmd/*` (Command handlers)

### What 100% Coverage Doesn't Mean

- Code is bug-free
- Tests are high quality
- Edge cases are covered

**Coverage shows what code runs, not whether it's tested correctly!**

## Common Pitfalls

### 1. Testing Implementation Instead of Behavior

**❌ Bad: Testing implementation**
```go
func TestBadExample(t *testing.T) {
    // Testing internal state
    cache := NewCache()
    cache.mu.Lock()  // Testing lock behavior
    if cache.items == nil {
        t.Error("items is nil")
    }
    cache.mu.Unlock()
}
```

**✅ Good: Testing behavior**
```go
func TestGoodExample(t *testing.T) {
    // Testing public API
    cache := NewCache()
    cache.Set("key", "value", time.Minute)

    value, found := cache.Get("key")
    if !found {
        t.Error("Expected to find cached value")
    }
    if value != "value" {
        t.Errorf("cache.Get() = %v, expected 'value'", value)
    }
}
```

### 2. Tests That Depend on Execution Order

**❌ Bad: Order-dependent tests**
```go
var sharedState string

func TestA(t *testing.T) {
    sharedState = "value"
}

func TestB(t *testing.T) {
    if sharedState != "value" {  // Breaks if TestB runs first!
        t.Errorf("sharedState = %s, expected 'value'", sharedState)
    }
}
```

**✅ Good: Independent tests**
```go
func TestA(t *testing.T) {
    state := "value"
    // Use local state
}

func TestB(t *testing.T) {
    state := setupTestState()
    // Each test sets up what it needs
}
```

### 3. Not Cleaning Up Resources

**❌ Bad: Leaking resources**
```go
func TestBad(t *testing.T) {
    file, _ := os.Create("test.txt")
    // No cleanup! File remains
}
```

**✅ Good: Proper cleanup**
```go
func TestGood(t *testing.T) {
    tmpDir := t.TempDir()  // Auto-cleaned
    file := filepath.Join(tmpDir, "test.txt")

    // Or use defer
    f, err := os.Create("test.txt")
    if err != nil {
        t.Fatalf("Failed to create file: %v", err)
    }
    defer os.Remove("test.txt")
    defer f.Close()
}
```

### 4. Ignoring Errors in Tests

**❌ Bad: Ignoring errors**
```go
func TestBad(t *testing.T) {
    result, _ := DoSomething()  // Ignores error
    if result == nil {
        t.Error("result is nil")
    }
}
```

**✅ Good: Check errors**
```go
func TestGood(t *testing.T) {
    result, err := DoSomething()
    if err != nil {
        t.Fatalf("DoSomething() failed: %v", err)
    }
    if result == nil {
        t.Fatal("DoSomething() returned nil result")
    }
}
```

### 5. Flaky Tests

**❌ Bad: Time-dependent test**
```go
func TestBad(t *testing.T) {
    start := time.Now()
    DoWork()
    duration := time.Since(start)
    if duration != 100*time.Millisecond {  // Flaky!
        t.Errorf("duration = %v, expected 100ms", duration)
    }
}
```

**✅ Good: Test behavior, not timing**
```go
func TestGood(t *testing.T) {
    result := DoWork()
    if result != expectedResult {
        t.Errorf("result = %v, expected %v", result, expectedResult)
    }

    // Or use ranges for timing tests
    start := time.Now()
    DoWork()
    duration := time.Since(start)
    if duration >= 200*time.Millisecond {
        t.Errorf("DoWork() took too long: %v", duration)
    }
    if duration <= 50*time.Millisecond {
        t.Errorf("DoWork() too fast: %v", duration)
    }
}
```

## Best Practices

### 1. Use Descriptive Test Names

```go
// Good names describe what is being tested
func TestGetCurrentBranch_ReturnsCorrectBranchName(t *testing.T) {}
func TestLoadConfig_WithMissingFile_ReturnsDefaultConfig(t *testing.T) {}
func TestPush_WithEmptyBranchName_ReturnsError(t *testing.T) {}
```

### 2. Test One Thing Per Test

```go
// Bad: Testing multiple things
func TestEverything(t *testing.T) {
    repo, _ := OpenRepository(".")
    branch, _ := repo.GetCurrentBranch()
    commits, _ := repo.GetCommitRange("main", branch)
    diff, _ := repo.GetDiffBetween("main", branch)
    // Too much in one test!
}

// Good: Focused tests
func TestGetCurrentBranch(t *testing.T) {
    repo, err := OpenRepository(".")
    if err != nil {
        t.Fatalf("OpenRepository() failed: %v", err)
    }

    branch, err := repo.GetCurrentBranch()
    if err != nil {
        t.Fatalf("GetCurrentBranch() failed: %v", err)
    }
    if branch == "" {
        t.Error("GetCurrentBranch() returned empty branch name")
    }
}
```

### 3. Use t.Helper() for Test Helpers

```go
func createTestRepo(t *testing.T) string {
    t.Helper()  // Marks this as helper function

    tmpDir := t.TempDir()
    // Setup code...
    return tmpDir
}

// When test fails, error points to caller, not helper
```

### 4. Use t.TempDir() for Temporary Directories

```go
func TestWithTempDir(t *testing.T) {
    tmpDir := t.TempDir()  // Automatically cleaned up after test

    file := filepath.Join(tmpDir, "test.txt")
    err := os.WriteFile(file, []byte("content"), 0644)
    if err != nil {
        t.Fatalf("Failed to write file: %v", err)
    }

    // No manual cleanup needed!
}
```

### 5. Use t.Fatalf() for Fatal Errors, t.Errorf() for Non-Fatal

```go
func TestExample(t *testing.T) {
    repo, err := OpenRepository(".")
    if err != nil {
        t.Fatalf("OpenRepository() failed: %v", err)  // Can't continue if this fails
    }
    if repo == nil {
        t.Fatal("OpenRepository() returned nil")
    }

    branch, err := repo.GetCurrentBranch()
    if err != nil {
        t.Fatalf("GetCurrentBranch() failed: %v", err)
    }

    // These can fail independently (test continues)
    if branch == "" {
        t.Error("GetCurrentBranch() returned empty branch name")
    }
    if strings.Contains(branch, "refs/heads/") {
        t.Errorf("branch contains 'refs/heads/': %s", branch)
    }
}
```

### 6. Write Tests Before Fixing Bugs

When you find a bug:
1. Write a failing test that reproduces the bug
2. Fix the bug
3. Verify the test passes
4. The test prevents regression

### 7. Keep Tests Fast

- Use mocks for slow operations (network, disk)
- Don't sleep unless necessary
- Run slow tests separately: `go test -short` skips them

```go
func TestSlow(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping slow test in short mode")
    }
    // Slow test code...
}
```

### 8. Test Error Messages

```go
func TestErrorMessage(t *testing.T) {
    err := ValidateBranchName("invalid branch")
    if err == nil {
        t.Fatal("ValidateBranchName() should return error for invalid branch")
    }
    if !strings.Contains(err.Error(), "branch name cannot contain spaces") {
        t.Errorf("error message = %q, should contain 'branch name cannot contain spaces'", err.Error())
    }
}
```

---

## Additional Resources

- [Go Testing Documentation](https://pkg.go.dev/testing)
- [Table Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [Go Test Comments](https://github.com/golang/go/wiki/TestComments)

Remember: **Tests are documentation**. Write tests that clearly show how your code should be used and what it should do.
