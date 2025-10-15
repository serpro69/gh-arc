# Development Workflows

This guide provides step-by-step instructions for common development tasks in the `gh-arc` project.

## Table of Contents

- [Setting Up Your Environment](#setting-up-your-environment)
- [Adding a New Feature](#adding-a-new-feature)
- [Fixing a Bug](#fixing-a-bug)
- [Adding a New Command](#adding-a-new-command)
- [Adding GitHub API Operations](#adding-github-api-operations)
- [Adding Configuration Options](#adding-configuration-options)
- [Updating Dependencies](#updating-dependencies)
- [Working with Git](#working-with-git)
- [Code Review Process](#code-review-process)

## Setting Up Your Environment

### Initial Setup

```bash
# 1. Fork the repository on GitHub
# 2. Clone your fork
git clone https://github.com/YOUR_USERNAME/gh-arc.git
cd gh-arc

# 3. Add upstream remote
git remote add upstream https://github.com/serpro69/gh-arc.git

# 4. Install dependencies
go mod download

# 5. Verify everything works
go build -o gh-arc
./gh-arc version

# 6. Run tests
go test ./...

# 7. Install locally for testing
gh extension install .
```

### IDE Setup (VS Code)

```bash
# Install Go extension
code --install-extension golang.go

# Configure settings.json
{
    "go.lintTool": "golangci-lint",
    "go.testFlags": ["-v"],
    "go.coverOnSave": true
}
```

### Pre-commit Checks

Create `.git/hooks/pre-commit`:

```bash
#!/bin/bash
set -e

echo "Running pre-commit checks..."

# Format code
go fmt ./...

# Run linter
go vet ./...

# Run tests
go test ./...

echo "✓ All checks passed"
```

Make it executable:
```bash
chmod +x .git/hooks/pre-commit
```

## Adding a New Feature

### Step-by-Step Process

#### 1. Create a Feature Branch

```bash
# Update master
git checkout master
git pull upstream master

# Create feature branch
git checkout -b feature/my-feature
```

#### 2. Write Tests First (TDD)

**Example: Adding a new function to parse reviewer list**

```go
// internal/github/reviewers_test.go
package github

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestParseReviewerList(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected []string
    }{
        {
            name:     "single reviewer",
            input:    "user1",
            expected: []string{"user1"},
        },
        {
            name:     "multiple reviewers",
            input:    "user1, user2, user3",
            expected: []string{"user1", "user2", "user3"},
        },
        {
            name:     "with whitespace",
            input:    " user1 , user2 ",
            expected: []string{"user1", "user2"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := ParseReviewerList(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

#### 3. Run Tests (They Should Fail)

```bash
go test ./internal/github
# FAIL: undefined: ParseReviewerList
```

#### 4. Implement the Feature

```go
// internal/github/reviewers.go
package github

import "strings"

// ParseReviewerList parses a comma-separated list of reviewers
func ParseReviewerList(input string) []string {
    if input == "" {
        return []string{}
    }

    parts := strings.Split(input, ",")
    result := make([]string, 0, len(parts))

    for _, part := range parts {
        trimmed := strings.TrimSpace(part)
        if trimmed != "" {
            result = append(result, trimmed)
        }
    }

    return result
}
```

#### 5. Run Tests (They Should Pass)

```bash
go test ./internal/github
# PASS
```

#### 6. Refactor if Needed

```go
// Improved version with better performance
func ParseReviewerList(input string) []string {
    if input == "" {
        return nil  // Return nil for empty input
    }

    parts := strings.Split(input, ",")
    result := make([]string, 0, len(parts))

    for _, part := range parts {
        if trimmed := strings.TrimSpace(part); trimmed != "" {
            result = append(result, trimmed)
        }
    }

    return result
}
```

#### 7. Add Documentation

```go
// ParseReviewerList parses a comma-separated list of GitHub usernames
// into individual reviewers. Whitespace around names is trimmed.
//
// Example:
//   ParseReviewerList("user1, user2") // => []string{"user1", "user2"}
//   ParseReviewerList("") // => nil
func ParseReviewerList(input string) []string {
    // ...
}
```

#### 8. Commit Your Changes

```bash
git add .
git commit -m "feat(github): add ParseReviewerList function

- Add function to parse comma-separated reviewer lists
- Include tests for various input formats
- Handle whitespace and empty inputs correctly"
```

#### 9. Push and Create PR

```bash
git push origin feature/my-feature
gh pr create --title "feat: add reviewer list parsing" --body "Adds ParseReviewerList to parse comma-separated reviewer names."
```

## Fixing a Bug

### Step-by-Step Process

#### 1. Reproduce the Bug

```bash
# Try to reproduce manually
gh arc diff

# Or write a test that demonstrates the bug
```

#### 2. Write a Failing Test

```go
func TestBugReproduction(t *testing.T) {
    // This test reproduces the bug
    input := "problematic input"
    result := FunctionWithBug(input)

    // This assertion currently fails
    assert.Equal(t, "expected", result)
}
```

#### 3. Fix the Bug

```go
func FunctionWithBug(input string) string {
    // Add fix here
    if input == "" {
        return "default"  // Handle edge case
    }
    // ... rest of function
}
```

#### 4. Verify Fix

```bash
# Run the specific test
go test -run TestBugReproduction ./internal/package

# Run all tests to ensure no regression
go test ./...
```

#### 5. Commit with Bug Reference

```bash
git commit -m "fix(diff): handle empty input in template parsing

- Add check for empty input
- Add test to prevent regression
- Fixes #123"
```

## Adding a New Command

### Example: Adding `gh arc status` Command

#### 1. Create Command File

```go
// cmd/status.go
package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show status of current branch and PRs",
    Long: `Display comprehensive status information including:
  - Current branch
  - Working directory status
  - Associated PR (if any)
  - CI status`,
    RunE: runStatus,
}

func init() {
    rootCmd.AddCommand(statusCmd)

    // Add flags
    statusCmd.Flags().BoolP("verbose", "v", false, "Show detailed status")
}

func runStatus(cmd *cobra.Command, args []string) error {
    verbose, _ := cmd.Flags().GetBool("verbose")

    fmt.Println("Status command not yet implemented")
    fmt.Printf("Verbose: %v\n", verbose)

    return nil
}
```

#### 2. Write Tests

```go
// cmd/status_test.go
package cmd

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestStatusCommand(t *testing.T) {
    t.Run("command exists", func(t *testing.T) {
        cmd := rootCmd
        statusCmd, _, err := cmd.Find([]string{"status"})

        assert.NoError(t, err)
        assert.Equal(t, "status", statusCmd.Name())
    })

    t.Run("has verbose flag", func(t *testing.T) {
        flag := statusCmd.Flags().Lookup("verbose")
        assert.NotNil(t, flag)
        assert.Equal(t, "bool", flag.Value.Type())
    })
}
```

#### 3. Implement Functionality

```go
func runStatus(cmd *cobra.Command, args []string) error {
    ctx := context.Background()
    verbose, _ := cmd.Flags().GetBool("verbose")

    // Get current repository
    repo, err := git.OpenRepository(".")
    if err != nil {
        return fmt.Errorf("not a git repository: %w", err)
    }

    // Get current branch
    branch, err := repo.GetCurrentBranch()
    if err != nil {
        return err
    }

    fmt.Printf("Branch: %s\n", branch)

    // Get working directory status
    status, err := repo.GetWorkingDirectoryStatus()
    if err != nil {
        return err
    }

    if status.IsClean {
        fmt.Println("Working directory: clean")
    } else {
        fmt.Println("Working directory: has changes")
        if verbose {
            // Show detailed status
            fmt.Printf("  Staged: %d\n", len(status.StagedFiles))
            fmt.Printf("  Unstaged: %d\n", len(status.UnstagedFiles))
            fmt.Printf("  Untracked: %d\n", len(status.UntrackedFiles))
        }
    }

    // Check for associated PR
    client, err := github.NewClient()
    if err != nil {
        return err
    }
    defer client.Close()

    pr, err := client.FindExistingPRForCurrentBranch(ctx, branch)
    if err != nil {
        // Don't fail, just note PR not found
        fmt.Println("Pull Request: none")
    } else {
        fmt.Printf("Pull Request: #%d - %s\n", pr.Number, pr.Title)
        fmt.Printf("  Status: %s\n", pr.State)
        fmt.Printf("  URL: %s\n", pr.HTMLURL)
    }

    return nil
}
```

#### 4. Update Documentation

Add to README.md:

```markdown
### `gh arc status`

Display status information for current branch:

\```bash
# Show basic status
gh arc status

# Show detailed status
gh arc status --verbose
\```
```

#### 5. Test Manually

```bash
# Rebuild
go build -o gh-arc

# Test
./gh-arc status
./gh-arc status --verbose
```

## Adding GitHub API Operations

### Example: Adding a Method to Get PR Comments

#### 1. Define the Data Structure

```go
// internal/github/pullrequest.go

// Comment represents a PR comment
type Comment struct {
    ID        int    `json:"id"`
    Body      string `json:"body"`
    User      User   `json:"user"`
    CreatedAt string `json:"created_at"`
}

// User represents a GitHub user
type User struct {
    Login string `json:"login"`
    Name  string `json:"name"`
}
```

#### 2. Write Tests

```go
// internal/github/pullrequest_test.go

func TestGetPRComments(t *testing.T) {
    t.Run("successful fetch", func(t *testing.T) {
        // This test requires mocking or integration testing
        // For now, we'll test the method signature exists
        var client *Client
        _, err := client.GetPRComments(context.Background(), 123)
        // Will fail with nil pointer, but method exists
        assert.Error(t, err)
    })
}
```

#### 3. Implement the Method

```go
// internal/github/pullrequest.go

// GetPRComments retrieves all comments for a pull request
func (c *Client) GetPRComments(ctx context.Context, number int) ([]Comment, error) {
    if c.repo == nil {
        return nil, fmt.Errorf("repository context not set")
    }

    path := fmt.Sprintf("repos/%s/%s/issues/%d/comments",
        c.repo.Owner, c.repo.Name, number)

    var comments []Comment
    err := c.Do(ctx, "GET", path, nil, &comments)
    if err != nil {
        return nil, fmt.Errorf("failed to get PR comments: %w", err)
    }

    return comments, nil
}
```

#### 4. Use in Command

```go
// cmd/diff.go or other command

comments, err := client.GetPRComments(ctx, pr.Number)
if err != nil {
    logger.Warn().Err(err).Msg("Failed to fetch comments")
} else {
    fmt.Printf("Comments: %d\n", len(comments))
}
```

## Adding Configuration Options

### Example: Adding a New Diff Option

#### 1. Add to Config Struct

```go
// internal/config/config.go

type DiffConfig struct {
    CreateAsDraft           bool     `mapstructure:"createAsDraft"`
    AutoUpdatePR            bool     `mapstructure:"autoUpdatePR"`
    IncludeCommitMessages   bool     `mapstructure:"includeCommitMessages"`
    EnableStacking          bool     `mapstructure:"enableStacking"`
    DefaultBase             string   `mapstructure:"defaultBase"`
    ShowStackingWarnings    bool     `mapstructure:"showStackingWarnings"`
    TemplatePath            string   `mapstructure:"templatePath"`
    RequireTestPlan         bool     `mapstructure:"requireTestPlan"`
    LinearEnabled           bool     `mapstructure:"linearEnabled"`
    LinearDefaultProject    string   `mapstructure:"linearDefaultProject"`

    // NEW: Add your option here
    AutoMergeable           bool     `mapstructure:"autoMergeable"`
}
```

#### 2. Set Default Value

```go
// internal/config/config.go

func setDefaults() {
    // ... existing defaults ...

    // Add default for new option
    viper.SetDefault("diff.autoMergeable", false)
}
```

#### 3. Add Validation (if needed)

```go
// internal/config/config.go

func (c *Config) Validate() error {
    // ... existing validation ...

    // Add validation for new option
    // (example: no validation needed for boolean)

    return nil
}
```

#### 4. Update Documentation

Add to README.md configuration section:

```markdown
- **`diff.autoMergeable`** (bool, default: `false`): Automatically set PR as mergeable when creating
```

#### 5. Use in Code

```go
// cmd/diff.go

autoMergeable := cfg.Diff.AutoMergeable
if autoMergeable {
    // Set PR as auto-mergeable
}
```

#### 6. Write Tests

```go
// internal/config/config_test.go

func TestDiffAutoMergeable(t *testing.T) {
    tmpDir := t.TempDir()
    os.Chdir(tmpDir)

    configContent := `{
        "diff": {
            "autoMergeable": true
        }
    }`

    err := os.WriteFile(".arc.json", []byte(configContent), 0644)
    require.NoError(t, err)

    cfg, err := Load()
    require.NoError(t, err)
    assert.True(t, cfg.Diff.AutoMergeable)
}
```

## Updating Dependencies

### Check for Updates

```bash
# List available updates
go list -u -m all

# Check specific module
go list -m -versions github.com/spf13/cobra
```

### Update a Dependency

```bash
# Update to latest minor/patch version
go get -u github.com/spf13/cobra

# Update to specific version
go get github.com/spf13/cobra@v1.10.0

# Update all dependencies (careful!)
go get -u ./...
```

### After Updating

```bash
# Tidy up go.mod and go.sum
go mod tidy

# Run tests
go test ./...

# Check if everything still builds
go build -o gh-arc

# Test manually
./gh-arc version
./gh-arc diff
```

### Commit

```bash
git add go.mod go.sum
git commit -m "chore: update cobra to v1.10.0"
```

## Working with Git

### Keep Your Branch Up to Date

```bash
# Update master
git checkout master
git pull upstream master

# Rebase your feature branch
git checkout feature/my-feature
git rebase master

# If there are conflicts
# 1. Fix conflicts in files
# 2. Stage resolved files
git add .
git rebase --continue

# Force push (rebase rewrites history)
git push --force-with-lease origin feature/my-feature
```

### Squash Commits Before Merging

```bash
# Interactive rebase last 3 commits
git rebase -i HEAD~3

# In the editor, change 'pick' to 'squash' for commits to squash
# Save and close
# Edit commit message in next editor

# Force push
git push --force-with-lease origin feature/my-feature
```

### Amend Last Commit

```bash
# Make changes
git add .
git commit --amend

# Or just fix message
git commit --amend -m "fix(diff): correct typo in error message"

# Force push
git push --force-with-lease origin feature/my-feature
```

## Code Review Process

### Before Requesting Review

1. **Self-review your changes**
   ```bash
   git diff master...feature/my-feature
   ```

2. **Run all checks**
   ```bash
   go test ./...
   go vet ./...
   go fmt ./...
   ```

3. **Test manually**
   ```bash
   go build -o gh-arc
   ./gh-arc diff
   ./gh-arc list
   ```

4. **Update documentation** if needed

5. **Write clear PR description**
   - What changed
   - Why it changed
   - How to test

### During Review

#### Address Feedback

```bash
# Make changes based on feedback
git add .
git commit -m "address review comments"

# Or amend if you prefer clean history
git add .
git commit --amend
git push --force-with-lease origin feature/my-feature
```

#### Respond to Comments

- Be respectful and open to feedback
- Ask clarifying questions if unsure
- Explain your reasoning
- Thank reviewers for their time

### After Approval

1. **Ensure all checks pass**
2. **Rebase on latest master** if needed
3. **Squash commits** if required
4. **Wait for maintainer to merge**

---

## Quick Reference

### Common Tasks

```bash
# Create feature branch
git checkout -b feature/my-feature

# Run tests
go test ./...

# Run specific test
go test -run TestMyFunction ./internal/package

# Build
go build -o gh-arc

# Install locally
gh extension install .

# Format code
go fmt ./...

# Lint
go vet ./...

# Commit
git commit -m "type(scope): description"

# Push
git push origin feature/my-feature

# Create PR
gh pr create
```

### Commit Message Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding/updating tests
- `refactor`: Code refactoring
- `chore`: Maintenance tasks
- `style`: Code style changes (formatting)
- `perf`: Performance improvements

---

These workflows should cover most common development tasks. For more complex scenarios, refer to the [Architecture Guide](ARCHITECTURE.md) or ask in PR discussions.
