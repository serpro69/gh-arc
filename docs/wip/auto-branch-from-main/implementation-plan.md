# Implementation Plan: Auto-Branch from Main (Simplified)

## Overview

This document provides a comprehensive implementation plan for the simplified Auto-Branch from Main feature. This approach avoids destructive operations like `git reset --hard` and eliminates the need for stash operations.

## Prerequisites

Before starting implementation:
- Review the feature design document: `docs/wip/auto-branch-from-main/feature-design.md`
- Review architecture documentation: `docs/contributing/ARCHITECTURE.md`
- Review testing guidelines: `docs/contributing/TESTING.md`
- Ensure development environment works: project builds successfully

## Implementation Phases

### Phase 1: Configuration Infrastructure (Tasks 1-2)
### Phase 2: Git Operations (Tasks 3-5)
### Phase 3: Detection and Auto-Branch Logic (Tasks 6-8)
### Phase 4: Integration with diff Command (Tasks 9-11)
### Phase 5: Documentation and Testing (Tasks 12-14)

---

## Phase 1: Configuration Infrastructure

### Task 1: Add Configuration Fields

**Goal**: Extend `DiffConfig` struct with auto-branch configuration fields.

**Files to modify**:
- `internal/config/config.go`

**What to implement**:

1. **Add new fields to DiffConfig struct** (around line 28):
   - `AutoCreateBranchFromMain` (bool): Enable/disable auto-branch creation
   - `AutoBranchNamePattern` (string): Pattern for branch name generation
   - `StaleRemoteThresholdHours` (int): Warn if origin/main is older than this (0 = disable)
   - All fields need `mapstructure` tags for Viper configuration loading

2. **Set default values in `setDefaults()` function** (around line 167):
   - `autoCreateBranchFromMain`: true (enabled by default)
   - `autoBranchNamePattern`: "" (empty string = use default pattern)
   - `staleRemoteThresholdHours`: 24 (warn if older than 24 hours)

**Why these defaults**:
- Enabled by default for seamless workflow
- Empty pattern uses safe default: `feature/auto-from-main-{timestamp}`
- 24 hour threshold catches most stale scenarios without being annoying
- Users can opt-out by setting to false or 0, or customizing values

**Testing approach**:
- Verify config loads with default values
- Verify project still builds after struct changes

---

### Task 2: Add Configuration Validation

**Goal**: Validate `autoBranchNamePattern` to prevent invalid git branch names.

**Files to modify**:
- `internal/config/config.go`

**What to implement**:

1. **Add validation logic in `Validate()` method** (around line 277):
   - Check for invalid git characters in pattern: `..`, `~`, `^`, `:`, `?`, `*`, `[`, `\`, space
   - Reject patterns starting with `/` (absolute paths are invalid in branch names)
   - Empty string is valid (uses default pattern)
   - "null" as a literal string means prompt user (valid)
   - Return descriptive errors mentioning the invalid character

2. **Import requirements**:
   - Ensure `strings` package is imported for validation

**Testing approach**:
- Create/update `internal/config/config_test.go`
- Write `TestValidate_AutoBranchNamePattern` with table-driven tests:
  - Valid patterns: `feature/{timestamp}`, empty string, "null"
  - Invalid patterns: patterns with `..`, spaces, starting with `/`
  - Verify error messages are helpful

---

## Phase 2: Git Operations

### Task 3: Add CountCommitsAhead Method

**Goal**: Implement method to count commits that a branch has ahead of another branch/ref.

**Files to modify**:
- `internal/git/git.go`

**What to implement**:

Add `CountCommitsAhead(branchName, baseBranch string) (int, error)` method to `Repository` struct:

1. **Location**: After the `HasUnpushedCommits` method (around line 946)

2. **Functionality**:
   - Validate inputs: both branch names must be non-empty
   - Check if `baseBranch` exists using `git rev-parse --verify`
   - If base doesn't exist (e.g., offline, first commit), return 0 with no error
   - Count commits using `git rev-list --count baseBranch..branchName`
   - Parse output to integer
   - Return count or error

3. **Error handling**:
   - Return 0 when base branch doesn't exist (not an error - might be offline)
   - Return error only for actual failures (invalid branch format, git command failure)

**Testing approach**:
- Write `TestCountCommitsAhead` in `internal/git/git_test.go`
- Test cases:
  - Normal case: feature branch with 2 commits ahead of main
  - Non-existent base branch returns 0
  - Equal branches return 0

---

### Task 4: Add Branch Existence Check Helper

**Goal**: Add utility method to check if a branch exists locally or remotely.

**Files to modify**:
- `internal/git/git.go`

**What to implement**:

Add `BranchExists(name string) (bool, error)` method to `Repository` struct:

1. **Functionality**:
   - Check local branch first: `plumbing.NewBranchReferenceName(name)`
   - If not found locally, check remote: `plumbing.NewRemoteReferenceName("origin", name)`
   - Return true if either exists, false if neither exists
   - Return error only for repository errors (not for "branch doesn't exist")

2. **Why check both local and remote**:
   - Branch might exist locally but not pushed yet
   - Branch might exist remotely but not checked out locally
   - Both cases count as "exists" for our purposes (prevents duplicate names)

**Use case**:
- Used in Task 7 to ensure unique branch names

**Testing approach**:
- Write `TestBranchExists`:
  - Test existing branch (main) returns true
  - Test non-existent branch returns false
  - Test newly created branch returns true

---

### Task 5: Add Push and Checkout Operations

**Goal**: Add git push and checkout operations for the auto-branch flow.

**Files to modify**:
- `internal/git/git.go`

**What to implement**:

Add two methods to `Repository` struct:

1. **`PushBranch(ctx context.Context, localRef, remoteBranch string) error`**:
   - Execute `git push origin <localRef>:refs/heads/<remoteBranch>`
   - Use context for cancellation support
   - Log operation at Info level
   - **Parse error output** to detect specific failure types:
     - Check for "remote ref already exists" or "! [rejected]" in stderr
     - If detected, return custom error type `ErrRemoteBranchExists` (define in git package)
     - Otherwise, return generic error with full git output
   - This allows callers to handle race conditions with retry logic

2. **`CheckoutTrackingBranch(branchName, remoteBranch string) error`**:
   - Execute `git checkout -b <branchName> <remoteBranch>`
   - This creates local branch tracking the remote branch
   - Log successful checkout at Debug level
   - Return error if checkout fails

**Error types to define**:
```go
// ErrRemoteBranchExists indicates the remote branch already exists
var ErrRemoteBranchExists = errors.New("remote branch already exists")
```

**Why use git CLI**:
- go-git's push implementation can be complex with remote authentication
- git CLI checkout is battle-tested and handles edge cases
- Matches existing patterns in the codebase

**Testing approach**:
- Write `TestPushBranch`:
  - Mock test (requires network) or mark as integration test
  - Verify correct git command is constructed
- Write `TestCheckoutTrackingBranch`:
  - Create remote branch, checkout tracking branch
  - Verify tracking relationship is established

---

### Task 5.5: Add Remote Tracking Ref Age Check

**Goal**: Add method to check the age of a remote tracking branch reference.

**Files to modify**:
- `internal/git/git.go`

**What to implement**:

Add `GetRemoteRefAge(remoteBranch string) (time.Duration, error)` method to `Repository` struct:

1. **Functionality**:
   - Get the reference for the remote branch: `plumbing.NewRemoteReferenceName("origin", remoteBranch)`
   - If ref doesn't exist, return 0 duration with no error (might be offline, first commit)
   - Get the commit object that the ref points to
   - Extract commit timestamp
   - Calculate duration: `time.Since(commitTime)`
   - Return duration

2. **Error handling**:
   - Return 0 duration if remote ref doesn't exist (not an error - might be offline)
   - Return error only for actual repository errors

3. **Why check commit time, not ref update time**:
   - Commit time represents when code was last changed on remote
   - Ref update time would just be when we last fetched
   - We want to know if remote has stale code, not just if we fetched recently

**Testing approach**:
- Write `TestGetRemoteRefAge`:
  - Create repo with commit from known time
  - Verify age calculation is correct
  - Test non-existent remote returns 0

---

## Phase 3: Detection and Auto-Branch Logic

### Task 6: Create Auto-Branch Module Structure

**Goal**: Set up the auto-branch detection module with basic structure and types.

**Files to create**:
- `internal/diff/auto_branch.go`
- `internal/diff/auto_branch_test.go`

**What to implement**:

1. **In `auto_branch.go`**:

   Create core types:
   - **`AutoBranchDetector` struct**: Main detector and orchestrator
     - Fields: `repo GitRepository`, `config *config.DiffConfig`
     - Constructor: `NewAutoBranchDetector(repo, cfg)`

   - **`DetectionResult` struct**: Result of detection
     - Fields: `OnMainBranch bool`, `CommitsAhead int`, `DefaultBranch string`

   - **`AutoBranchContext` struct**: Holds state for the auto-branch operation
     - Fields: `BranchName string`, `ShouldProceed bool`

   Implement methods:
   - **`DetectCommitsOnMain(ctx context.Context) (*DetectionResult, error)`**:
     - Get current branch from repo
     - Get default branch from repo
     - Compare: are they the same?
     - If on main, count commits ahead of `origin/{defaultBranch}`
     - Return populated DetectionResult
     - Log detection results at Debug level

   - **`ShouldAutoBranch(result *DetectionResult) bool`**:
     - Simple boolean logic: `OnMainBranch && CommitsAhead > 0`
     - Used to determine if flow should activate

2. **In `auto_branch_test.go`**:
   - Write `TestShouldAutoBranch` with table-driven tests
   - Test cases: on main with commits (true), on feature (false), on main no commits (false)

**Package dependencies**:
- Import `internal/config` for configuration
- Import `internal/git` for GitRepository interface
- Import `internal/logger` for structured logging
- Import `context` for context propagation

---

### Task 7: Implement Branch Name Generation

**Goal**: Add branch name generation with support for custom patterns and placeholders.

**Files to modify**:
- `internal/diff/auto_branch.go`

**What to implement**:

Add three related functions:

1. **`GenerateBranchName() (string, bool, error)`**:
   - Returns: (generated name, shouldPrompt, error)
   - Check pattern from config:
     - Pattern == "null" → return ("", true, nil) to trigger prompt
     - Pattern == "" → generate default: `feature/auto-from-main-{timestamp}`
     - Pattern != "" → apply placeholders
   - Placeholders to support:
     - `{timestamp}`: Unix timestamp from `time.Now().Unix()`
     - `{date}`: ISO date format `2006-01-02`
     - `{datetime}`: ISO datetime format `2006-01-02T150405`
     - `{username}`: From git config `user.name`, sanitized
     - `{random}`: 6-character random alphanumeric string
   - Use `strings.ReplaceAll` for each placeholder

2. **`sanitizeBranchName(name string) string`**:
   - Helper function to clean username/input for git branch compatibility
   - Replace spaces with hyphens
   - Replace `..` with `-`
   - Convert to lowercase
   - Remove: `~`, `^`, `:`, `?`, `*`, `[`, `\`
   - Called when applying `{username}` placeholder

3. **`EnsureUniqueBranchName(baseName string) (string, error)`**:
   - Check if branch exists using `repo.BranchExists()`
   - If exists, append `-1`, `-2`, etc. until unique name found
   - Safety limit: stop after 100 attempts to prevent infinite loop
   - Return unique name or error

**Testing approach**:
- Write `TestGenerateBranchName`:
  - Test empty pattern returns default format
  - Test "null" pattern triggers prompt flag
  - Test each placeholder type produces expected format
- Write `TestSanitizeBranchName`:
  - Test cases: "John Doe" → "john-doe", "test..name" → "test--name"
- Write `TestEnsureUniqueBranchName`:
  - Mock repo with existing branches
  - Verify appends counter correctly

---

### Task 8: Implement User Prompts and Decision Logic

**Goal**: Add interactive prompts for user confirmation and input.

**Files to modify**:
- `internal/diff/auto_branch.go`

**What to implement**:

Add prompt utility functions and decision methods:

1. **`promptYesNo(message string, defaultYes bool) (bool, error)`**:
   - Display message with (Y/n) or (y/N) based on default
   - Read user input from stdin using `bufio.Reader`
   - Handle: "y"/"yes" → true, "n"/"no" → false, empty → use default
   - Invalid input: re-prompt with "Please answer 'y' or 'n'"
   - Return (response, error)

2. **`promptBranchName() (string, error)`**:
   - Display: "Enter branch name (or press Enter for default): "
   - Read user input
   - Trim whitespace
   - Empty string means use default (caller generates default)
   - Validate: reject names with spaces, re-prompt if invalid
   - Return (name, error)

3. **`CheckStaleRemote(ctx context.Context, defaultBranch string) (bool, error)`**:
   - Get config threshold: `cfg.StaleRemoteThresholdHours`
   - If threshold is 0, skip check (disabled), return (true, nil)
   - Call `repo.GetRemoteRefAge("origin/" + defaultBranch)`
   - If age == 0 (remote doesn't exist), skip check, return (true, nil)
   - Convert threshold to duration: `time.Duration(threshold) * time.Hour`
   - If age > threshold:
     - Calculate human-readable time (days/hours)
     - Display warning about stale remote
     - Prompt user: "Continue anyway? (y/N)"
     - If user declines, return (false, error "user declined due to stale remote")
     - If user accepts, log warning and return (true, nil)
   - Return (true, nil) if fresh

4. **`PrepareAutoBranch(ctx context.Context, detection *DetectionResult) (*AutoBranchContext, error)`**:
   - Check stale remote first
   - If stale check fails/user declines, return error
   - Check if should proceed (config or prompt)
   - If config.AutoCreateBranchFromMain == false, prompt user
   - If user declines, return error "cancelled by user"
   - Get branch name (from pattern or prompt)
   - Ensure branch name is unique
   - Return AutoBranchContext with branch name

**Imports needed**:
- `bufio` for reading stdin
- `os` for stdin access
- `fmt` for Printf

**Testing approach**:
- Write `TestPrepareAutoBranch`:
  - Test with config enabled (no prompt)
  - Document stdin mocking as "tested manually"
  - Test branch name generation logic
- Manual testing for actual prompts

---

## Phase 4: Integration with diff Command

### Task 9: Integrate Detection into diff Command

**Goal**: Add auto-branch detection at the start of `runDiff` function.

**Files to modify**:
- `cmd/diff.go`

**Location**: In `runDiff` function after git repository is opened and current branch is obtained (around line 154).

**What to implement**:

1. **Import the diff package**:
   - Add import for `internal/diff` package

2. **Create detector instance** (after currentBranch is logged):
   - Initialize `AutoBranchDetector` with gitRepo and cfg.Diff
   - Store in variable for use throughout function

3. **Detect commits on main**:
   - Call `DetectCommitsOnMain(ctx)`
   - Handle errors (detection failure should fail the command)
   - Store result

4. **Prepare auto-branch if detected**:
   - Declare variable for `*AutoBranchContext`
   - Check if `ShouldAutoBranch(detection)` returns true
   - If true:
     - Print warning: "\n⚠️  Warning: You have {count} commits on {branch}\n"
     - Call `PrepareAutoBranch(ctx, detection)`
     - Handle errors (user cancellation, etc.)
     - Store context for later use

5. **Continue normal flow**:
   - Flow continues with template generation, PR metadata, etc.
   - Auto-branch context is carried through for post-PR operations

**Error handling**:
- Detection failure: return error immediately
- Prepare failure: check if user cancelled, provide helpful message
- Other errors: return with context

**Testing approach**:
- Verify project builds
- Manual testing: create commits on main, run `gh arc diff`

---

### Task 10: Add Pre-PR Push and Post-PR Checkout

**Goal**: After template is completed, push branch to remote BEFORE creating PR, then checkout locally after PR creation succeeds.

**Files to modify**:
- `cmd/diff.go`

**Location**: Split into two parts:
- **Part A**: After template parsing/validation, before PR creation (around line 580)
- **Part B**: After PR creation succeeds, before success message (around line 630)

**What to implement**:

**Part A: Push Branch to Remote (BEFORE PR Creation)**

1. **Check if auto-branch was used**:
   - If `autoBranchContext == nil`, skip (normal diff flow)
   - If not nil, proceed with push

2. **Push branch to remote with retry on collision**:
   - Implement retry loop (max 3 attempts):
     - Call `gitRepo.PushBranch(ctx, "HEAD", autoBranchContext.BranchName)`
     - If push succeeds, break out of loop
     - If error is `git.ErrRemoteBranchExists` (race condition):
       - Log: "Branch name collision detected, generating new name"
       - Call `EnsureUniqueBranchName()` again to get new name with incremented counter
       - Update `autoBranchContext.BranchName` with new name
       - Retry push with new name
     - If error is NOT `ErrRemoteBranchExists` (other failure):
       - Log error with full git output
       - Display error message: "✗ Failed to push branch to remote"
       - Explain that user is still on main
       - Provide manual recovery instructions:
         ```bash
         You can create the branch and push manually:
           git checkout -b {branch-name}
           git push origin {branch-name}
           gh arc diff
         ```
       - Return error (abort operation)
   - If max retries exceeded:
     - Return error: "Failed to push after 3 attempts (branch name collision)"

3. **Display push success**:
   - Print: "✓ Pushed branch '{name}' to remote"

**Part B: Create PR and Checkout Locally (AFTER PR Creation)**

4. **Create Pull Request via GitHub API** (existing logic, not in this task):
   - This happens after Part A push succeeds
   - If PR creation fails after push succeeded, handle specially (see below)

5. **Checkout tracking branch**:
   - Call `gitRepo.CheckoutTrackingBranch(autoBranchContext.BranchName, "origin/"+autoBranchContext.BranchName)`
   - If checkout fails:
     - Log error with full git output
     - Display error with recovery instructions:
       ```bash
       ✗ Failed to checkout tracking branch

       The PR was created successfully, but local checkout failed.
       You can checkout the branch manually:
         git checkout -b {branch-name} origin/{branch-name}
       ```
     - Note: PR and remote branch exist, just local tracking failed
     - Still return success (PR was created, this is non-fatal)

6. **Display success**:
   - Print: "✓ Switched to feature branch '{name}'"

7. **Display informational message**:
   - Print note about main branch still being ahead:
     ```
     ℹ️  Note: Your local 'main' branch is still ahead of origin/main.
        You can reset it manually with:
          git checkout main && git reset --hard origin/main
        Or it will sync automatically when you run 'gh arc land'.
     ```

**Special Error Handling: Push Succeeded but PR Creation Failed**

Between Part A and Part B, if PR creation fails after push succeeded:
- Display clear message with recovery path:
  ```bash
  ✗ Failed to create Pull Request: <GitHub API Error>

  ✓ Branch '{branch-name}' was successfully pushed to remote.
    You can create the PR manually by running:
      gh pr create --head {branch-name} --base main

    Or delete the remote branch and start over:
      git push origin --delete {branch-name}
  ```
- User is still on main branch (safe state)
- Return error with recovery instructions

**Error handling summary**:
- Push failure: Critical, abort immediately, user stays on main
- PR creation failure (after push): Provide manual PR creation command
- Checkout failure: Non-fatal, PR exists, provide manual checkout command
- All errors include full output for debugging

**Testing approach**:
- Verify project builds
- Manual testing: complete full flow
- Test push failure scenario (invalid credentials, network issue)
- Test PR creation failure after push (API error simulation)
- Test checkout failure scenario (permission issue)

---

### Task 11: Add User-Friendly Error Messages

**Goal**: Improve error messages when auto-branch flow is cancelled or fails.

**Files to modify**:
- `cmd/diff.go`

**Location**: In the error handling for `PrepareAutoBranch` (Task 9).

**What to implement**:

Enhance the error handling after `PrepareAutoBranch` call:

1. **Check for user cancellation**:
   - Use `strings.Contains(err.Error(), "cancelled by user")`
   - If cancelled, print helpful message:
     - "\n✗ Cannot create PR from main to main."
     - "Please create a feature branch manually:"
     - Step-by-step commands using actual branch names
     - Include: checkout -b, push, gh arc diff
   - Return error "operation cancelled"

2. **For other errors**:
   - Return error with context: "auto-branch preparation failed: {error}"
   - Preserve specific error information for troubleshooting

**Why distinguish cancellation**:
- User cancellation is expected behavior, not a failure
- Other errors (git failures, name conflicts) are unexpected
- Cancellation needs manual instructions
- Other errors need error details

**Testing approach**:
- Verify project builds
- Manual testing: decline auto-branch prompt
- Verify helpful message with correct commands

---

## Phase 5: Documentation and Testing

### Task 12: Write Comprehensive Integration Tests

**Goal**: Add end-to-end integration tests for the complete auto-branch flow.

**Files to create**:
- `internal/diff/auto_branch_integration_test.go`

**What to test**:

Create integration test file with sub-tests:

1. **Full automatic flow**:
   - Set up: Create temp git repo with initial commit
   - Add commits on main
   - Configure: AutoCreateBranchFromMain = true
   - Execute: Run detection and prepare flow
   - Verify:
     - Detection found commits on main
     - Branch name was generated
     - ShouldProceed is true

2. **Custom branch name pattern**:
   - Set up: Config with pattern `feature/{username}-{date}`
   - Execute: Generate branch name
   - Verify: Generated name matches pattern format

3. **Branch name collision**:
   - Set up: Create branch with name that would be generated
   - Execute: EnsureUniqueBranchName
   - Verify: Appends counter correctly

4. **Skip if short mode**:
   - Add `if testing.Short() { t.Skip() }` to all tests

**Test helpers needed**:
- `createTestRepo(t)` - Set up temp git repo
- Use go-git for repository operations
- Use `t.TempDir()` for automatic cleanup

**Running tests**:
- These are integration tests, may be slow
- Run with: `go test ./internal/diff -v`

---

### Task 13: Update Architecture Documentation

**Goal**: Document the auto-branch module in the architecture guide.

**Files to modify**:
- `docs/contributing/ARCHITECTURE.md`

**Location**: In the `internal/diff/` package section, add new subsection.

**What to document**:

1. **Module Overview**:
   - Purpose: handles commits on main scenario
   - Flow diagram: Detection → Prepare → Normal Diff → Push → Checkout
   - When it runs: at start and end of diff command

2. **Key Types**:
   - `AutoBranchDetector`: main orchestrator
   - `DetectionResult`: detection output
   - `AutoBranchContext`: state for post-PR operations
   - Explain fields and purpose of each

3. **Key Methods**:
   - List main methods with one-sentence descriptions
   - Explain two-phase pattern (detect before, execute after)

4. **Configuration**:
   - List config fields
   - Explain config-first pattern
   - Note defaults

5. **Integration Points**:
   - Where called in cmd/diff.go (start and end)
   - Why split into two phases

**Style guidance**:
- Match existing architecture doc formatting
- Use code blocks for type definitions
- Keep technical but readable

---

### Task 14: Create Architecture Decision Record

**Goal**: Document the architectural decision in an ADR.

**Files to create**:
- `docs/adr/0002-auto-branch-from-main.md`

**What to include**:

Structure following ADR template:

1. **Status**: Accepted

2. **Date**: 2025-10-16

3. **Context**:
   - Problem: users commit directly to main by mistake
   - Current state: diff fails with confusing error
   - Alternatives considered: error only, auto-fix with reset, this simplified approach

4. **Decision**:
   - Implement simplified detection and branch creation
   - Push to remote, then checkout tracking branch
   - No destructive local operations (no reset, no stash)
   - Make behavior configurable

5. **Consequences**:
   - Positive: seamless workflow, safe, simple
   - Negative: main branch stays ahead (requires manual sync)
   - Neutral: user must understand local main state

6. **References**:
   - Link to design document
   - Link to implementation plan

**Why create ADR**:
- Documents decision rationale
- Explains why this approach over alternatives
- Records trade-offs
- Standard practice for significant changes

---

## Summary

This simplified implementation plan provides 14 tasks organized into 5 phases:

1. **Phase 1 (Tasks 1-2)**: Configuration infrastructure
2. **Phase 2 (Tasks 3-5)**: Git operations
3. **Phase 3 (Tasks 6-8)**: Detection and auto-branch logic
4. **Phase 4 (Tasks 9-11)**: Integration with diff command
5. **Phase 5 (Tasks 12-14)**: Documentation and testing

## Key Simplifications from Original Design

1. **Removed stash operations** - Not needed since we don't change HEAD until after PR creation
2. **Removed reset operations** - No `git reset --hard`, main stays as-is
3. **Simpler error handling** - Fewer failure modes to handle
4. **Two-phase execution** - Detect/prepare before PR, push/checkout after PR
5. **Safer recovery** - If anything fails, user stays on main

## Implementation Guidelines

**Follow TDD**:
- Write tests before or alongside implementation
- Each task includes testing guidance

**Commit Frequently**:
- Commit after each task (minimum 14 commits)
- Each commit should be atomic and reversible, with passing tests

**DRY Principle**:
- Reuse existing patterns from the codebase
- Extract common functionality to helpers

**YAGNI Principle**:
- Implement only what's specified
- Don't add speculative features

## Testing Strategy

- **Unit tests**: Tasks 2, 3, 4, 7, 8
- **Integration tests**: Task 12
- **Manual testing**: After each phase
- **End-to-end testing**: After Phase 4

## Questions or Issues?

Refer to:
- Feature design: `docs/wip/auto-branch-from-main/feature-design.md`
- Architecture guide: `docs/contributing/ARCHITECTURE.md`
- Testing guide: `docs/contributing/TESTING.md`
