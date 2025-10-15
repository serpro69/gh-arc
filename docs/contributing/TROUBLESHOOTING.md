# Troubleshooting and Debugging Guide

This guide helps you diagnose and fix common issues when developing `gh-arc`.

## Table of Contents

- [Development Environment Issues](#development-environment-issues)
- [Build and Compilation Issues](#build-and-compilation-issues)
- [Test Failures](#test-failures)
- [GitHub API Issues](#github-api-issues)
- [Git Operations Issues](#git-operations-issues)
- [Configuration Issues](#configuration-issues)
- [Debugging Techniques](#debugging-techniques)
- [Performance Issues](#performance-issues)
- [Getting Help](#getting-help)

## Development Environment Issues

### Go Version Mismatch

**Problem:** Build fails with version-related errors

```
go: go.mod requires go >= 1.23.4
```

**Solution:**

```bash
# Check your Go version
go version

# Install correct version
# On Linux/Mac with homebrew:
brew install go@1.23

# Or download from: https://go.dev/dl/

# Verify
go version  # Should show 1.23.4 or higher
```

### Module Download Failures

**Problem:** Can't download dependencies

```
go: github.com/spf13/cobra@v1.10.1: Get https://proxy.golang.org/...: dial tcp: lookup proxy.golang.org: no such host
```

**Solution:**

```bash
# Check internet connection
ping google.com

# Try with direct module downloads (bypasses proxy)
go env -w GOPROXY=direct

# Or use a different proxy
go env -w GOPROXY=https://goproxy.io,direct

# Clear module cache and retry
go clean -modcache
go mod download
```

### IDE Not Recognizing Code

**Problem:** VS Code shows red squiggles for valid code

**Solution:**

```bash
# Restart Go language server
# In VS Code: Cmd/Ctrl + Shift + P -> "Go: Restart Language Server"

# Or regenerate gopls cache
go clean -cache
go clean -modcache
go mod download
```

## Build and Compilation Issues

### Undefined References

**Problem:**

```
./cmd/diff.go:45:2: undefined: github.NewClient
```

**Solution:**

```bash
# Ensure all imports are present
# Missing import for internal package

# Fix: Add import
import (
    "github.com/serpro69/gh-arc/internal/github"
)

# Run go mod tidy to clean up
go mod tidy
```

### Circular Import

**Problem:**

```
import cycle not allowed
package github.com/serpro69/gh-arc/internal/github
    imports github.com/serpro69/gh-arc/internal/config
    imports github.com/serpro69/gh-arc/internal/github
```

**Solution:**

Refactor to break the cycle. Common approaches:

1. **Extract shared types to a new package**
2. **Use dependency injection**
3. **Reverse the dependency**

```go
// Before: config imports github
// After: github imports config (one direction only)
```

### Missing go.sum Entries

**Problem:**

```
go: module github.com/spf13/cobra: missing go.sum entry
```

**Solution:**

```bash
# Regenerate go.sum
go mod tidy

# If still fails, clean and retry
go clean -modcache
go mod download
```

## Test Failures

### Test Failing Inconsistently (Flaky Tests)

**Problem:** Test passes sometimes, fails other times

**Common Causes:**

1. **Time-dependent code**
2. **Concurrent access to shared state**
3. **External dependencies**
4. **Random values**

**Solution:**

```go
// Bad: Flaky due to timing
func TestBad(t *testing.T) {
    start := time.Now()
    DoWork()
    duration := time.Since(start)
    assert.Equal(t, 100*time.Millisecond, duration)  // Too exact!
}

// Good: Use ranges
func TestGood(t *testing.T) {
    start := time.Now()
    DoWork()
    duration := time.Since(start)
    assert.Less(t, duration, 200*time.Millisecond)
    assert.Greater(t, duration, 50*time.Millisecond)
}

// Or mock time
func TestWithMockTime(t *testing.T) {
    mockTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
    // Use mockTime instead of time.Now()
}
```

### Tests Pass Locally but Fail in CI

**Problem:** Tests work on your machine but fail in GitHub Actions

**Common Causes:**

1. **Different OS (Linux vs macOS/Windows)**
2. **Missing dependencies**
3. **File path issues**
4. **Timing differences**

**Solution:**

```bash
# Test with same Go version as CI
go version  # Check .github/workflows/*.yml for CI version

# Run tests with race detector (CI does this)
go test -race ./...

# Check for OS-specific code
go test -v ./... | grep SKIP
```

### Test Timeout

**Problem:**

```
panic: test timed out after 10m0s
```

**Solution:**

```bash
# Increase timeout
go test -timeout 30m ./...

# Or fix slow test
func TestSlow(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping slow test")
    }
    // ...
}

# Run without slow tests
go test -short ./...
```

### Import Cycle in Tests

**Problem:**

```
import cycle not allowed in test
```

**Solution:**

Use the `_test` package for tests that need to import the package:

```go
// client_test.go
package github_test  // Note: github_test, not github

import (
    "testing"
    "github.com/serpro69/gh-arc/internal/github"
)

func TestClient(t *testing.T) {
    client := github.NewClient()
    // ...
}
```

## GitHub API Issues

### Authentication Failures

**Problem:**

```
error: failed to create GitHub client: authentication required
```

**Solution:**

```bash
# Check authentication
gh auth status

# Refresh with required scopes
gh auth refresh --scopes "repo,read:user,user:email"

# Verify scopes
gh auth status

# Test API access
gh api user
```

### Rate Limiting

**Problem:**

```
error: rate limit exceeded, reset at 2024-01-15 10:30:00
```

**Solution:**

```go
// The client automatically handles rate limiting
// But you can check status:

// In code:
if err := client.Do(ctx, "GET", path, nil, &response); err != nil {
    if github.IsRateLimitError(err) {
        // Handle rate limit
        logger.Warn().Msg("Rate limited, waiting...")
        time.Sleep(time.Minute)
    }
}

// Check rate limit status:
gh api rate_limit
```

### 404 Not Found

**Problem:**

```
error: GET https://api.github.com/repos/owner/repo/pulls/123: 404 Not Found
```

**Debugging:**

```bash
# Check PR exists
gh pr view 123

# Check repository context
git remote -v

# Verify authentication has repo access
gh auth status

# Test API call directly
gh api repos/OWNER/REPO/pulls/123
```

### Request Timeout

**Problem:**

```
context deadline exceeded
```

**Solution:**

```go
// Increase timeout
client, err := github.NewClient(
    github.WithTimeout(60 * time.Second),
)

// Or check network
ping api.github.com
```

## Git Operations Issues

### Not a Git Repository

**Problem:**

```
error: failed to open git repository: repository does not exist
```

**Solution:**

```bash
# Verify you're in a git repository
git status

# If not, initialize
git init

# Check .git exists
ls -la | grep .git
```

### Detached HEAD State

**Problem:**

```
error: cannot perform operation in detached HEAD state
```

**Solution:**

```bash
# Check current state
git status

# Create a branch from current commit
git checkout -b temp-branch

# Or checkout an existing branch
git checkout master
```

### Dirty Working Directory

**Problem:**

```
error: working directory has uncommitted changes
```

**Solution:**

```bash
# Check what changed
git status

# Stash changes
git stash

# Or commit them
git add .
git commit -m "WIP: save changes"

# Or discard them (careful!)
git reset --hard HEAD
```

### Can't Find Remote Branch

**Problem:**

```
error: remote branch 'origin/feature' not found
```

**Solution:**

```bash
# Fetch from remote
git fetch origin

# List remote branches
git branch -r

# If branch doesn't exist, check remote
git remote -v
```

## Configuration Issues

### Config File Not Found

**Problem:** Settings not being applied

**Debugging:**

```bash
# Check config file location
ls -la .arc.json
ls -la ~/.config/gh-arc/.arc.json

# Check which config is loaded
# Add debug logging to config.Load()
```

**Solution:**

```bash
# Create config in current directory
cat > .arc.json << EOF
{
  "github": {
    "defaultBranch": "main"
  }
}
EOF

# Verify it loads
./gh-arc version  # Should show no errors
```

### Invalid Config Format

**Problem:**

```
error: failed to load configuration: invalid JSON
```

**Solution:**

```bash
# Validate JSON
cat .arc.json | jq .

# Or use online validator: https://jsonlint.com

# Common issues:
# - Trailing commas (not allowed in JSON)
# - Missing quotes around strings
# - Unescaped characters
```

### Environment Variables Not Working

**Problem:** `GHARC_*` environment variables not being applied

**Solution:**

```bash
# Check variable is set
echo $GHARC_DIFF_CREATEASDRAFT

# Viper is case-insensitive but use uppercase
export GHARC_DIFF_CREATEASDRAFT=true

# Verify in code
viper.GetBool("diff.createAsDraft")  // Should be true
```

## Debugging Techniques

### Adding Debug Logging

```go
// Use logger for debug output
logger := logger.Get()

logger.Debug().
    Str("key", value).
    Int("count", count).
    Msg("Debug information")

// Run with verbose flag to see debug logs
./gh-arc -v diff
```

### Using Delve Debugger

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug a test
dlv test ./internal/github -- -test.run TestClientNew

# Debug the binary
dlv exec ./gh-arc -- diff --draft

# Common delve commands:
# b main.go:10          - Set breakpoint
# c                     - Continue
# n                     - Next line
# s                     - Step into
# p variable            - Print variable
# bt                    - Backtrace
# q                     - Quit
```

### Print Debugging

```go
// Quick and dirty debugging
import "fmt"

func problematicFunction() {
    fmt.Printf("DEBUG: variable = %+v\n", variable)
    fmt.Printf("DEBUG: type = %T\n", variable)

    // For JSON output
    data, _ := json.MarshalIndent(variable, "", "  ")
    fmt.Printf("DEBUG: %s\n", data)
}
```

### Using pprof for Profiling

```go
// Add to main.go
import (
    "net/http"
    _ "net/http/pprof"
)

func main() {
    go func() {
        http.ListenAndServe("localhost:6060", nil)
    }()
    // ... rest of code
}
```

```bash
# Run your program
./gh-arc diff

# In another terminal:
# CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Memory profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutines
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### Tracing Execution

```go
import "runtime/trace"

func main() {
    f, _ := os.Create("trace.out")
    defer f.Close()

    trace.Start(f)
    defer trace.Stop()

    // Your code here
}
```

```bash
# Generate trace
./gh-arc diff

# View trace
go tool trace trace.out
```

### Inspecting HTTP Requests

```go
// Log HTTP requests in GitHub client
import "net/http/httputil"

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) error {
    // Log request
    if logger.Debug().Enabled() {
        dump, _ := httputil.DumpRequest(req, true)
        logger.Debug().Str("request", string(dump)).Msg("HTTP Request")
    }

    // ... make request ...

    // Log response
    if logger.Debug().Enabled() {
        dump, _ := httputil.DumpResponse(resp, true)
        logger.Debug().Str("response", string(dump)).Msg("HTTP Response")
    }
}
```

Run with verbose flag:
```bash
./gh-arc -v diff
```

## Performance Issues

### Slow Test Execution

**Problem:** Tests take too long

**Solution:**

```bash
# Profile tests
go test -cpuprofile=cpu.prof ./internal/github
go tool pprof cpu.prof

# Run tests in parallel
go test -parallel 8 ./...

# Identify slow tests
go test -v ./... | grep -E "PASS|FAIL" | sort -k3 -n
```

### Memory Leaks

**Problem:** Memory usage grows over time

**Debugging:**

```bash
# Run with memory profiling
go test -memprofile=mem.prof ./internal/cache
go tool pprof mem.prof

# In pprof:
(pprof) top
(pprof) list FunctionName
(pprof) web
```

**Common causes:**
- Not closing resources (files, connections)
- Goroutine leaks
- Growing slices/maps

```go
// Fix: Close resources
defer file.Close()
defer client.Close()

// Fix: Use context to cancel goroutines
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
            // Work
        }
    }
}(ctx)
```

### High CPU Usage

**Problem:** Process using too much CPU

**Debugging:**

```bash
# CPU profile
go test -cpuprofile=cpu.prof -bench=. ./...
go tool pprof cpu.prof

# Look for hot paths
(pprof) top
(pprof) list FunctionName
```

**Common causes:**
- Inefficient algorithms
- Too many allocations
- Unnecessary work in loops

## Getting Help

### Before Asking for Help

1. **Search existing issues**
   ```bash
   # Search GitHub issues
   gh issue list --search "your error message"
   ```

2. **Check documentation**
   - README.md
   - CONTRIBUTING.md
   - Architecture.md
   - This file!

3. **Enable debug logging**
   ```bash
   ./gh-arc -v -v diff  # Double verbose
   ```

4. **Create minimal reproduction**
   - Simplify to smallest failing example
   - Include all relevant code

### Asking for Help

When opening an issue, include:

1. **What you tried to do**
   ```
   I tried to create a PR with: gh arc diff --draft
   ```

2. **What happened**
   ```
   Error: failed to create PR: 404 Not Found
   ```

3. **What you expected**
   ```
   Expected PR to be created as draft
   ```

4. **Environment**
   ```bash
   ./gh-arc version
   go version
   gh version
   git --version
   uname -a  # OS info
   ```

5. **Logs with verbose output**
   ```bash
   ./gh-arc -v -v diff 2>&1 | tee debug.log
   ```

6. **Minimal reproduction**
   - Steps to reproduce
   - Sample code if relevant
   - Configuration files

### Useful Debugging Commands

```bash
# System info
go version
go env

# Git info
git status
git remote -v
git log -1

# GitHub auth
gh auth status

# Config
cat .arc.json

# Binary info
file ./gh-arc
./gh-arc version

# Network
ping api.github.com
curl -I https://api.github.com

# Verbose execution
./gh-arc -v -v command 2>&1 | tee log.txt
```

## Common Error Messages

### "command not found: gh-arc"

**Solution:**
```bash
# Build first
go build -o gh-arc

# Or install as extension
gh extension install .

# Then use
gh arc command
```

### "no required module provides package"

**Solution:**
```bash
go mod tidy
go mod download
```

### "use of internal package not allowed"

**Solution:**
Only code within `gh-arc` can import `internal/*` packages. This is by design.

### "cannot use X (type Y) as type Z"

**Solution:**
Type mismatch. Check:
- Function signature
- Return types
- Interface implementation

```go
// Example fix:
// Expected: func() (string, error)
// Got:      func() string

// Fix: Add error return
func MyFunc() (string, error) {
    return "result", nil
}
```

---

## Quick Troubleshooting Checklist

When something isn't working:

- [ ] Is Go the correct version? (`go version`)
- [ ] Are dependencies up to date? (`go mod download`)
- [ ] Did you rebuild? (`go build -o gh-arc`)
- [ ] Are tests passing? (`go test ./...`)
- [ ] Is GitHub auth working? (`gh auth status`)
- [ ] Are you in a git repository? (`git status`)
- [ ] Is the config file valid? (`cat .arc.json | jq .`)
- [ ] Did you enable verbose logging? (`./gh-arc -v -v`)
- [ ] Did you check existing issues? (`gh issue list`)

If all else fails, open an issue with detailed information!
