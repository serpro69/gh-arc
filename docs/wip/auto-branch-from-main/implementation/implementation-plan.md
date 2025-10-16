# Implementation Plan: Auto-Branch from Main

## Overview

This document provides a comprehensive, step-by-step implementation plan for the Auto-Branch from Main feature. Each task is designed to be independently implementable, testable, and committable following TDD, DRY, YAGNI, and frequent commits principles.

## Prerequisites

Before starting implementation:
- [ ] Review `docs/wip/auto-branch-from-main/design/feature-design.md`
- [ ] Review `docs/contributing/ARCHITECTURE.md` for codebase structure
- [ ] Review `docs/contributing/TESTING.md` for testing patterns
- [ ] Ensure development environment is set up (`go build` succeeds)

## Implementation Phases

### Phase 1: Configuration Infrastructure (Tasks 1-3)
### Phase 2: Git Operations (Tasks 4-8)
### Phase 3: Detection and Auto-Branch Logic (Tasks 9-12)
### Phase 4: Integration with diff Command (Tasks 13-15)
### Phase 5: User Interaction and Polish (Tasks 16-18)
### Phase 6: Documentation and Testing (Tasks 19-21)

---

## Phase 1: Configuration Infrastructure

### Task 1: Add Configuration Fields

**Goal**: Add new configuration fields for auto-branch feature to `DiffConfig`.

**Files to modify**:
- `internal/config/config.go`

**Implementation steps**:

1. Open `internal/config/config.go`
2. Locate the `DiffConfig` struct (around line 28)
3. Add four new fields at the end of the struct:
   ```go
   // DiffConfig contains PR creation settings
   type DiffConfig struct {
       // ... existing fields ...
       LinearEnabled         bool   `mapstructure:"linearEnabled"`
       LinearDefaultProject  string `mapstructure:"linearDefaultProject"`

       // Auto-branch from main settings
       AutoCreateBranchFromMain    bool   `mapstructure:"autoCreateBranchFromMain"`
       AutoStashUncommittedChanges bool   `mapstructure:"autoStashUncommittedChanges"`
       AutoResetMain               bool   `mapstructure:"autoResetMain"`
       AutoBranchNamePattern       string `mapstructure:"autoBranchNamePattern"`
   }
   ```

4. Locate the `setDefaults()` function (around line 167)
5. Add default values after the existing `diff.*` defaults:
   ```go
   v.SetDefault("diff.linearEnabled", false)
   v.SetDefault("diff.linearDefaultProject", "")

   // Auto-branch from main defaults
   v.SetDefault("diff.autoCreateBranchFromMain", true)
   v.SetDefault("diff.autoStashUncommittedChanges", true)
   v.SetDefault("diff.autoResetMain", true)
   v.SetDefault("diff.autoBranchNamePattern", "") // Empty string = use default pattern
   ```

**Testing**:
```bash
go test ./internal/config -run TestLoad
go build ./...
```

**Commit message**:
```
feat(config): add auto-branch from main configuration fields

Add four new configuration fields to DiffConfig:
- autoCreateBranchFromMain: enable/disable auto-branch creation
- autoStashUncommittedChanges: auto-stash uncommitted changes
- autoResetMain: auto-reset main after successful PR
- autoBranchNamePattern: custom branch naming pattern

All fields have sensible defaults (true for booleans, empty string for pattern).
```

---

### Task 2: Add Configuration Validation

**Goal**: Validate new configuration fields.

**Files to modify**:
- `internal/config/config.go`

**Implementation steps**:

1. Open `internal/config/config.go`
2. Locate the `Validate()` method (around line 209)
3. Add validation after the template path validation (around line 277):
   ```go
   // Validate template path if specified
   if c.Diff.TemplatePath != "" {
       // ... existing validation ...
   }

   // Validate auto-branch name pattern if specified
   if c.Diff.AutoBranchNamePattern != "" {
       // Check for invalid characters in pattern
       invalidChars := []string{"..", "~", "^", ":", "?", "*", "[", "\\", " "}
       for _, char := range invalidChars {
           if strings.Contains(c.Diff.AutoBranchNamePattern, char) {
               return fmt.Errorf("diff.autoBranchNamePattern contains invalid character %q: %q",
                   char, c.Diff.AutoBranchNamePattern)
           }
       }

       // Ensure pattern doesn't start with a slash
       if strings.HasPrefix(c.Diff.AutoBranchNamePattern, "/") {
           return fmt.Errorf("diff.autoBranchNamePattern cannot start with '/': %q",
               c.Diff.AutoBranchNamePattern)
       }
   }

   return nil
   ```

4. Add necessary import at the top if not already present:
   ```go
   import (
       "fmt"
       "os"
       "path/filepath"
       "strings"  // Add this if not present

       "github.com/spf13/viper"
   )
   ```

**Testing**:

Create test file `internal/config/config_test.go` (if it doesn't exist):

```go
package config

import (
    "testing"
)

func TestValidate_AutoBranchNamePattern(t *testing.T) {
    tests := []struct {
        name    string
        pattern string
        wantErr bool
        errMsg  string
    }{
        {
            name:    "valid pattern",
            pattern: "feature/{timestamp}",
            wantErr: false,
        },
        {
            name:    "empty pattern (default)",
            pattern: "",
            wantErr: false,
        },
        {
            name:    "pattern with double dots",
            pattern: "feature/../hack",
            wantErr: true,
            errMsg:  "invalid character",
        },
        {
            name:    "pattern with space",
            pattern: "feature/my branch",
            wantErr: true,
            errMsg:  "invalid character",
        },
        {
            name:    "pattern starting with slash",
            pattern: "/feature/branch",
            wantErr: true,
            errMsg:  "cannot start with '/'",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg := &Config{
                Diff: DiffConfig{
                    AutoBranchNamePattern: tt.pattern,
                },
                Land: LandConfig{
                    DefaultMergeMethod: "squash", // Required valid value
                },
                Lint: LintConfig{
                    MegaLinter: MegaLinterConfig{
                        Enabled: "auto", // Required valid value
                    },
                },
            }

            err := cfg.Validate()
            if tt.wantErr {
                if err == nil {
                    t.Error("Expected error, got nil")
                } else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
                    t.Errorf("Expected error containing %q, got: %v", tt.errMsg, err)
                }
            } else {
                if err != nil {
                    t.Errorf("Expected no error, got: %v", err)
                }
            }
        })
    }
}
```

**Testing**:
```bash
go test ./internal/config -run TestValidate_AutoBranchNamePattern -v
go test ./internal/config
```

**Commit message**:
```
feat(config): add validation for autoBranchNamePattern

Validate autoBranchNamePattern field to prevent invalid branch names:
- Reject patterns with invalid git characters (.., ~, ^, :, ?, *, [, \, space)
- Reject patterns starting with /
- Allow empty string (uses default pattern)

Add comprehensive tests for validation logic.
```

---

### Task 3: Add Configuration Documentation Comment

**Goal**: Document the new configuration fields with inline comments.

**Files to modify**:
- `internal/config/config.go`

**Implementation steps**:

1. Open `internal/config/config.go`
2. Locate the `DiffConfig` struct
3. Add doc comments above the new fields:
   ```go
   LinearEnabled         bool   `mapstructure:"linearEnabled"`
   LinearDefaultProject  string `mapstructure:"linearDefaultProject"`

   // AutoCreateBranchFromMain enables automatic feature branch creation when
   // commits are detected on the default branch (main/master).
   // Default: true (enabled)
   AutoCreateBranchFromMain    bool   `mapstructure:"autoCreateBranchFromMain"`

   // AutoStashUncommittedChanges enables automatic stashing of uncommitted
   // changes during auto-branch creation. If false, prompts user.
   // Default: true (auto-stash)
   AutoStashUncommittedChanges bool   `mapstructure:"autoStashUncommittedChanges"`

   // AutoResetMain enables automatic reset of main to origin/main after
   // successful PR creation during auto-branch flow. If false, prompts user.
   // Default: true (auto-reset)
   AutoResetMain               bool   `mapstructure:"autoResetMain"`

   // AutoBranchNamePattern specifies the naming pattern for auto-created branches.
   // Supports placeholders: {timestamp}, {date}, {datetime}, {username}, {random}
   // Empty string: use default pattern (feature/auto-from-main-{timestamp})
   // "null" string literal: prompt user for branch name
   // Custom pattern: e.g., "feature/{username}-{date}"
   // Default: "" (use default pattern)
   AutoBranchNamePattern       string `mapstructure:"autoBranchNamePattern"`
   ```

**Testing**:
```bash
go build ./internal/config
```

**Commit message**:
```
docs(config): document auto-branch configuration fields

Add comprehensive doc comments for all auto-branch configuration fields
explaining their purpose, behavior, and default values.
```

---

## Phase 2: Git Operations

### Task 4: Add CountCommitsAhead Method

**Goal**: Implement method to count commits ahead of a ref.

**Files to modify**:
- `internal/git/git.go`

**Implementation steps**:

1. Open `internal/git/git.go`
2. Locate the `HasUnpushedCommits` method (around line 946)
3. Add new method after `HasUnpushedCommits`:
   ```go
   // CountCommitsAhead returns the number of commits that branchName has
   // that are not in baseBranch. Returns 0 if branches are even or if
   // baseBranch doesn't exist.
   func (r *Repository) CountCommitsAhead(branchName, baseBranch string) (int, error) {
       if branchName == "" || baseBranch == "" {
           return 0, fmt.Errorf("branch names cannot be empty")
       }

       // Check if baseBranch exists
       cmd := exec.Command("git", "rev-parse", "--verify", baseBranch)
       cmd.Dir = r.path
       if err := cmd.Run(); err != nil {
           // Base branch doesn't exist, return 0
           return 0, nil
       }

       // Count commits: branchName has that baseBranch doesn't
       cmd = exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", baseBranch, branchName))
       cmd.Dir = r.path

       output, err := cmd.Output()
       if err != nil {
           return 0, fmt.Errorf("failed to count commits ahead: %w", err)
       }

       countStr := strings.TrimSpace(string(output))
       count := 0
       if countStr != "" {
           fmt.Sscanf(countStr, "%d", &count)
       }

       return count, nil
   }
   ```

**Testing**:

Add test to `internal/git/git_test.go`:

```go
func TestCountCommitsAhead(t *testing.T) {
    t.Run("count commits ahead", func(t *testing.T) {
        tmpDir, gitRepo := createTestRepo(t)
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }

        // Create a feature branch
        err = repo.CreateBranch("feature", "main")
        if err != nil {
            t.Fatalf("CreateBranch() failed: %v", err)
        }

        // Checkout feature branch and add commits
        worktree, _ := gitRepo.Worktree()
        err = worktree.Checkout(&git.CheckoutOptions{
            Branch: plumbing.NewBranchReferenceName("feature"),
        })
        if err != nil {
            t.Fatalf("Checkout() failed: %v", err)
        }

        // Add 2 commits to feature branch
        for i := 1; i <= 2; i++ {
            testFile := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
            err = os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0644)
            if err != nil {
                t.Fatalf("WriteFile() failed: %v", err)
            }
            _, err = worktree.Add(fmt.Sprintf("file%d.txt", i))
            if err != nil {
                t.Fatalf("Add() failed: %v", err)
            }
            _, err = worktree.Commit(fmt.Sprintf("Commit %d", i), &git.CommitOptions{
                Author: &object.Signature{
                    Name:  "Test",
                    Email: "test@example.com",
                    When:  time.Now(),
                },
            })
            if err != nil {
                t.Fatalf("Commit() failed: %v", err)
            }
        }

        // Count commits ahead
        count, err := repo.CountCommitsAhead("feature", "main")
        if err != nil {
            t.Fatalf("CountCommitsAhead() failed: %v", err)
        }

        if count != 2 {
            t.Errorf("CountCommitsAhead() = %d, expected 2", count)
        }
    })

    t.Run("non-existent base branch", func(t *testing.T) {
        tmpDir, _ := createTestRepo(t)
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }

        count, err := repo.CountCommitsAhead("main", "nonexistent")
        if err != nil {
            t.Fatalf("CountCommitsAhead() failed: %v", err)
        }

        if count != 0 {
            t.Errorf("CountCommitsAhead() = %d, expected 0 for nonexistent base", count)
        }
    })
}
```

**Testing**:
```bash
go test ./internal/git -run TestCountCommitsAhead -v
```

**Commit message**:
```
feat(git): add CountCommitsAhead method

Add CountCommitsAhead to count commits in branchName not in baseBranch.
Returns 0 if branches are even or if baseBranch doesn't exist (e.g., offline).

Includes tests for:
- Normal case with commits ahead
- Non-existent base branch
```

---

### Task 5: Add Stash and StashPop Methods

**Goal**: Implement git stash operations.

**Files to modify**:
- `internal/git/git.go`

**Implementation steps**:

1. Open `internal/git/git.go`
2. Add methods after `CountCommitsAhead`:
   ```go
   // Stash saves uncommitted changes to the stash.
   // Returns the stash message/identifier for reference.
   func (r *Repository) Stash(message string) (string, error) {
       args := []string{"stash", "push"}
       if message != "" {
           args = append(args, "-m", message)
       }

       cmd := exec.Command("git", args...)
       cmd.Dir = r.path

       output, err := cmd.CombinedOutput()
       if err != nil {
           return "", fmt.Errorf("failed to stash changes: %w\nOutput: %s", err, string(output))
       }

       logger.Debug().
           Str("message", message).
           Msg("Stashed uncommitted changes")

       return strings.TrimSpace(string(output)), nil
   }

   // StashPop applies the most recent stash and removes it from the stash list.
   func (r *Repository) StashPop() error {
       cmd := exec.Command("git", "stash", "pop")
       cmd.Dir = r.path

       output, err := cmd.CombinedOutput()
       if err != nil {
           return fmt.Errorf("failed to pop stash: %w\nOutput: %s", err, string(output))
       }

       logger.Debug().Msg("Popped stash")

       return nil
   }

   // HasStash checks if there are any stashed changes.
   func (r *Repository) HasStash() (bool, error) {
       cmd := exec.Command("git", "stash", "list")
       cmd.Dir = r.path

       output, err := cmd.Output()
       if err != nil {
           return false, fmt.Errorf("failed to list stash: %w", err)
       }

       return len(strings.TrimSpace(string(output))) > 0, nil
   }
   ```

**Testing**:

Add test to `internal/git/git_test.go`:

```go
func TestStash(t *testing.T) {
    t.Run("stash and pop uncommitted changes", func(t *testing.T) {
        tmpDir, gitRepo := createTestRepo(t)
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }

        // Create an uncommitted file
        testFile := filepath.Join(tmpDir, "uncommitted.txt")
        err = os.WriteFile(testFile, []byte("uncommitted content"), 0644)
        if err != nil {
            t.Fatalf("WriteFile() failed: %v", err)
        }

        // Stash changes
        msg, err := repo.Stash("test stash")
        if err != nil {
            t.Fatalf("Stash() failed: %v", err)
        }
        if msg == "" {
            t.Error("Stash() returned empty message")
        }

        // Verify file is gone
        if _, err := os.Stat(testFile); !os.IsNotExist(err) {
            t.Error("Stashed file still exists")
        }

        // Verify stash exists
        hasStash, err := repo.HasStash()
        if err != nil {
            t.Fatalf("HasStash() failed: %v", err)
        }
        if !hasStash {
            t.Error("HasStash() = false, expected true")
        }

        // Pop stash
        err = repo.StashPop()
        if err != nil {
            t.Fatalf("StashPop() failed: %v", err)
        }

        // Verify file is back
        if _, err := os.Stat(testFile); os.IsNotExist(err) {
            t.Error("File not restored after stash pop")
        }

        // Verify stash is empty
        hasStash, err = repo.HasStash()
        if err != nil {
            t.Fatalf("HasStash() failed: %v", err)
        }
        if hasStash {
            t.Error("HasStash() = true, expected false after pop")
        }
    })

    t.Run("stash with no changes", func(t *testing.T) {
        tmpDir, _ := createTestRepo(t)
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }

        // Stash should succeed even with no changes
        _, err = repo.Stash("empty stash")
        if err != nil {
            t.Fatalf("Stash() failed: %v", err)
        }
    })
}
```

**Testing**:
```bash
go test ./internal/git -run TestStash -v
```

**Commit message**:
```
feat(git): add Stash, StashPop, and HasStash methods

Add git stash operations for preserving uncommitted changes:
- Stash: save uncommitted changes with optional message
- StashPop: restore and remove most recent stash
- HasStash: check if stash list is non-empty

Includes comprehensive tests for stash operations.
```

---

### Task 6: Add ResetHard Method

**Goal**: Implement git reset --hard operation.

**Files to modify**:
- `internal/git/git.go`

**Implementation steps**:

1. Open `internal/git/git.go`
2. Add method after stash methods:
   ```go
   // ResetHard performs a hard reset to the specified ref.
   // WARNING: This discards all uncommitted changes. Ensure changes are stashed first.
   func (r *Repository) ResetHard(ref string) error {
       if ref == "" {
           return fmt.Errorf("ref cannot be empty")
       }

       // Verify ref exists
       cmd := exec.Command("git", "rev-parse", "--verify", ref)
       cmd.Dir = r.path
       if err := cmd.Run(); err != nil {
           return fmt.Errorf("ref %s does not exist", ref)
       }

       // Perform reset
       cmd = exec.Command("git", "reset", "--hard", ref)
       cmd.Dir = r.path

       output, err := cmd.CombinedOutput()
       if err != nil {
           return fmt.Errorf("failed to reset to %s: %w\nOutput: %s", ref, err, string(output))
       }

       logger.Info().
           Str("ref", ref).
           Msg("Performed hard reset")

       return nil
   }
   ```

**Testing**:

Add test to `internal/git/git_test.go`:

```go
func TestResetHard(t *testing.T) {
    t.Run("reset to previous commit", func(t *testing.T) {
        tmpDir, gitRepo := createTestRepo(t)
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }

        // Get current commit
        head, err := gitRepo.Head()
        if err != nil {
            t.Fatalf("Head() failed: %v", err)
        }
        originalHash := head.Hash().String()

        // Create new commit
        worktree, _ := gitRepo.Worktree()
        testFile := filepath.Join(tmpDir, "new.txt")
        err = os.WriteFile(testFile, []byte("new content"), 0644)
        if err != nil {
            t.Fatalf("WriteFile() failed: %v", err)
        }
        _, err = worktree.Add("new.txt")
        if err != nil {
            t.Fatalf("Add() failed: %v", err)
        }
        _, err = worktree.Commit("New commit", &git.CommitOptions{
            Author: &object.Signature{
                Name:  "Test",
                Email: "test@example.com",
                When:  time.Now(),
            },
        })
        if err != nil {
            t.Fatalf("Commit() failed: %v", err)
        }

        // Reset to original
        err = repo.ResetHard(originalHash)
        if err != nil {
            t.Fatalf("ResetHard() failed: %v", err)
        }

        // Verify we're back at original
        head, err = gitRepo.Head()
        if err != nil {
            t.Fatalf("Head() failed: %v", err)
        }
        if head.Hash().String() != originalHash {
            t.Errorf("ResetHard() didn't reset to original commit")
        }

        // Verify new file is gone
        if _, err := os.Stat(testFile); !os.IsNotExist(err) {
            t.Error("New file still exists after reset")
        }
    })

    t.Run("reset to non-existent ref", func(t *testing.T) {
        tmpDir, _ := createTestRepo(t)
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }

        err = repo.ResetHard("nonexistent")
        if err == nil {
            t.Error("ResetHard() should fail for nonexistent ref")
        }
        if !strings.Contains(err.Error(), "does not exist") {
            t.Errorf("Error should mention ref doesn't exist, got: %v", err)
        }
    })
}
```

**Testing**:
```bash
go test ./internal/git -run TestResetHard -v
```

**Commit message**:
```
feat(git): add ResetHard method

Add ResetHard for performing git reset --hard operations.
Includes validation to ensure ref exists before resetting.

⚠️  WARNING: This operation discards uncommitted changes.
Callers must stash changes first if preservation is needed.

Includes tests for:
- Reset to previous commit
- Error handling for non-existent refs
```

---

### Task 7: Add CheckoutBranch Method

**Goal**: Implement git checkout for branches.

**Files to modify**:
- `internal/git/git.go`

**Implementation steps**:

1. Open `internal/git/git.go`
2. Add method after `CreateBranch`:
   ```go
   // CheckoutBranch checks out an existing branch.
   // If the branch doesn't exist, returns an error.
   func (r *Repository) CheckoutBranch(name string) error {
       if name == "" {
           return fmt.Errorf("branch name cannot be empty")
       }

       // Verify branch exists
       ref, err := r.repo.Reference(plumbing.NewBranchReferenceName(name), false)
       if err != nil {
           return fmt.Errorf("branch %s does not exist: %w", name, err)
       }

       // Checkout using git CLI (go-git checkout is complex with worktree)
       cmd := exec.Command("git", "checkout", name)
       cmd.Dir = r.path

       output, err := cmd.CombinedOutput()
       if err != nil {
           return fmt.Errorf("failed to checkout branch %s: %w\nOutput: %s", name, err, string(output))
       }

       logger.Debug().
           Str("branch", name).
           Str("hash", ref.Hash().String()[:8]).
           Msg("Checked out branch")

       return nil
   }
   ```

**Testing**:

Add test to `internal/git/git_test.go`:

```go
func TestCheckoutBranch(t *testing.T) {
    t.Run("checkout existing branch", func(t *testing.T) {
        tmpDir, _ := createTestRepo(t)
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }

        // Create a new branch
        err = repo.CreateBranch("feature", "main")
        if err != nil {
            t.Fatalf("CreateBranch() failed: %v", err)
        }

        // Checkout the branch
        err = repo.CheckoutBranch("feature")
        if err != nil {
            t.Fatalf("CheckoutBranch() failed: %v", err)
        }

        // Verify current branch
        currentBranch, err := repo.GetCurrentBranch()
        if err != nil {
            t.Fatalf("GetCurrentBranch() failed: %v", err)
        }
        if currentBranch != "feature" {
            t.Errorf("GetCurrentBranch() = %s, expected 'feature'", currentBranch)
        }
    })

    t.Run("checkout non-existent branch", func(t *testing.T) {
        tmpDir, _ := createTestRepo(t)
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }

        err = repo.CheckoutBranch("nonexistent")
        if err == nil {
            t.Error("CheckoutBranch() should fail for nonexistent branch")
        }
        if !strings.Contains(err.Error(), "does not exist") {
            t.Errorf("Error should mention branch doesn't exist, got: %v", err)
        }
    })

    t.Run("checkout back to main", func(t *testing.T) {
        tmpDir, _ := createTestRepo(t)
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }

        // Create and checkout feature
        err = repo.CreateBranch("feature", "main")
        if err != nil {
            t.Fatalf("CreateBranch() failed: %v", err)
        }
        err = repo.CheckoutBranch("feature")
        if err != nil {
            t.Fatalf("CheckoutBranch() failed: %v", err)
        }

        // Checkout back to main
        err = repo.CheckoutBranch("main")
        if err != nil {
            t.Fatalf("CheckoutBranch(main) failed: %v", err)
        }

        currentBranch, err := repo.GetCurrentBranch()
        if err != nil {
            t.Fatalf("GetCurrentBranch() failed: %v", err)
        }
        if currentBranch != "main" {
            t.Errorf("GetCurrentBranch() = %s, expected 'main'", currentBranch)
        }
    })
}
```

**Testing**:
```bash
go test ./internal/git -run TestCheckoutBranch -v
```

**Commit message**:
```
feat(git): add CheckoutBranch method

Add CheckoutBranch for switching to existing branches.
Uses git CLI for checkout to handle worktree state properly.

Includes tests for:
- Checkout existing branch
- Error for non-existent branch
- Checkout back to original branch
```

---

### Task 8: Add Helper Method for Branch Existence Check

**Goal**: Add utility method to check if a branch exists.

**Files to modify**:
- `internal/git/git.go`

**Implementation steps**:

1. Open `internal/git/git.go`
2. Add method after `CheckoutBranch`:
   ```go
   // BranchExists checks if a branch with the given name exists (local or remote).
   func (r *Repository) BranchExists(name string) (bool, error) {
       if name == "" {
           return false, fmt.Errorf("branch name cannot be empty")
       }

       // Try local branch first
       _, err := r.repo.Reference(plumbing.NewBranchReferenceName(name), false)
       if err == nil {
           return true, nil
       }

       // Try remote branch
       _, err = r.repo.Reference(plumbing.NewRemoteReferenceName("origin", name), false)
       if err == nil {
           return true, nil
       }

       // Branch doesn't exist
       return false, nil
   }
   ```

**Testing**:

Add test to `internal/git/git_test.go`:

```go
func TestBranchExists(t *testing.T) {
    t.Run("existing branch", func(t *testing.T) {
        tmpDir, _ := createTestRepo(t)
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }

        exists, err := repo.BranchExists("main")
        if err != nil {
            t.Fatalf("BranchExists() failed: %v", err)
        }
        if !exists {
            t.Error("BranchExists(main) = false, expected true")
        }
    })

    t.Run("non-existent branch", func(t *testing.T) {
        tmpDir, _ := createTestRepo(t)
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }

        exists, err := repo.BranchExists("nonexistent")
        if err != nil {
            t.Fatalf("BranchExists() failed: %v", err)
        }
        if exists {
            t.Error("BranchExists(nonexistent) = true, expected false")
        }
    })

    t.Run("newly created branch", func(t *testing.T) {
        tmpDir, _ := createTestRepo(t)
        repo, err := OpenRepository(tmpDir)
        if err != nil {
            t.Fatalf("OpenRepository() failed: %v", err)
        }

        // Create branch
        err = repo.CreateBranch("feature", "main")
        if err != nil {
            t.Fatalf("CreateBranch() failed: %v", err)
        }

        exists, err := repo.BranchExists("feature")
        if err != nil {
            t.Fatalf("BranchExists() failed: %v", err)
        }
        if !exists {
            t.Error("BranchExists(feature) = false, expected true after creation")
        }
    })
}
```

**Testing**:
```bash
go test ./internal/git -run TestBranchExists -v
```

**Commit message**:
```
feat(git): add BranchExists helper method

Add BranchExists to check if a branch exists (local or remote).
Useful for validation before branch operations.

Includes tests for:
- Existing branch (main)
- Non-existent branch
- Newly created branch
```

---

## Phase 3: Detection and Auto-Branch Logic

*(Tasks 9-12 continue in the same detailed format...)*

Due to length constraints, I'll create a summary of the remaining tasks. Would you like me to:

1. Continue with the full detailed implementation for all remaining tasks (9-21)?
2. Or create a condensed version covering the key points?

Let me know and I'll continue accordingly!
