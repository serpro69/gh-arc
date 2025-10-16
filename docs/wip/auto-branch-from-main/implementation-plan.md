# Implementation Plan: Auto-Branch from Main

## Overview

This document provides a comprehensive implementation plan for the Auto-Branch from Main feature. Each task is designed to be independently implementable and testable, following TDD, DRY, YAGNI principles, with atomic commits.

## Prerequisites

Before starting implementation:
- Review the feature design document: `docs/wip/auto-branch-from-main/feature-design.md`
- Review architecture documentation: `docs/contributing/ARCHITECTURE.md`
- Review testing guidelines: `docs/contributing/TESTING.md`
- Ensure development environment works: project builds successfully

## Implementation Phases

### Phase 1: Configuration Infrastructure (Tasks 1-3)
### Phase 2: Git Operations (Tasks 4-8)
### Phase 3: Detection and Auto-Branch Logic (Tasks 9-12)
### Phase 4: Integration with diff Command (Tasks 13-15)
### Phase 5: User Interaction and Polish (Tasks 16-18)
### Phase 6: Documentation and Testing (Tasks 19-22)

---

## Phase 1: Configuration Infrastructure

### Task 1: Add Configuration Fields

**Goal**: Extend `DiffConfig` struct with auto-branch configuration fields.

**Files to modify**:
- `internal/config/config.go`

**What to implement**:

1. **Add new fields to DiffConfig struct** (around line 28):
   - `AutoCreateBranchFromMain` (bool): Enable/disable auto-branch creation
   - `AutoStashUncommittedChanges` (bool): Auto-stash uncommitted changes during branch creation
   - `AutoResetMain` (bool): Auto-reset main to origin/main after successful PR
   - `AutoBranchNamePattern` (string): Pattern for branch name generation
   - All fields need `mapstructure` tags for Viper configuration loading

2. **Set default values in `setDefaults()` function** (around line 167):
   - `autoCreateBranchFromMain`: true (enabled by default)
   - `autoStashUncommittedChanges`: true (auto-stash by default)
   - `autoResetMain`: true (auto-reset by default)
   - `autoBranchNamePattern`: "" (empty string = use default pattern)

**Why these defaults**:
- All enabled by default for seamless workflow
- Empty pattern uses safe default: `feature/auto-from-main-{timestamp}`
- Users can opt-out by setting to false or customizing pattern

**Testing approach**:
- Verify config loads with default values
- Verify project still builds after struct changes

---

### Task 2: Add Configuration Validation

**Goal**: Validate `autoBranchNamePattern` to prevent invalid git branch names.

**Files to modify**:
- `internal/config/config.go`

**What to implement**:

1. **Add validation logic in `Validate()` method** (around line 277, after template path validation):
   - Check for invalid git characters in pattern: `..`, `~`, `^`, `:`, `?`, `*`, `[`, `\`, space
   - Reject patterns starting with `/` (absolute paths are invalid in branch names)
   - Empty string is valid (uses default pattern)
   - Return descriptive errors mentioning the invalid character and the full pattern

2. **Import requirements**:
   - Ensure `strings` package is imported for validation

**Why this validation**:
- Git has specific rules for valid branch names (see: `git check-ref-format`)
- Prevents user errors and git failures at runtime
- Pattern validation happens at config load time, not during auto-branch flow

**Testing approach**:
- Create `internal/config/config_test.go` if it doesn't exist
- Write `TestValidate_AutoBranchNamePattern` with table-driven tests:
  - Valid patterns: `feature/{timestamp}`, empty string
  - Invalid patterns: patterns with `..`, spaces, starting with `/`
  - Verify error messages contain helpful information
- Test that other valid config fields still pass validation (e.g., `DefaultMergeMethod`)

---

### Task 3: Add Configuration Documentation

**Goal**: Document the new configuration fields with inline comments for developers.

**Files to modify**:
- `internal/config/config.go`

**What to implement**:

Add godoc comments above each new field in the `DiffConfig` struct explaining:
- **Purpose**: What the field controls
- **Behavior**: What happens when true/false (for booleans)
- **Format/Options**: For string fields like pattern
- **Default value**: What users get without explicit configuration

**Comment structure guidance**:
- Start with field name and one-sentence summary
- Explain behavior for different values
- Mention default value
- For pattern field, list available placeholders and give examples

**Why inline documentation**:
- Helps future developers understand config options without searching documentation
- godoc will extract these comments for API documentation
- Provides context when reading the code

---

## Phase 2: Git Operations

### Task 4: Add CountCommitsAhead Method

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

**Why this approach**:
- Uses standard git revision range syntax: `base..head`
- Returns 0 for missing base (graceful degradation for offline work)
- Matches existing repository method patterns

**Testing approach**:
- Write `TestCountCommitsAhead` in `internal/git/git_test.go`
- Test cases:
  - Normal case: feature branch with 2 commits ahead of main
  - Non-existent base branch returns 0
  - Equal branches return 0
- Use `createTestRepo(t)` helper to set up git repository
- Create test commits using go-git's `worktree.Commit()`

---

### Task 5: Add Stash and StashPop Methods

**Goal**: Implement git stash operations to preserve uncommitted changes.

**Files to modify**:
- `internal/git/git.go`

**What to implement**:

Add three methods to `Repository` struct (after `CountCommitsAhead`):

1. **`Stash(message string) (string, error)`**:
   - Execute `git stash push` with optional message
   - Return stash reference/output string
   - Log stash operation to debug logger
   - Handle empty message (git stash works without message)

2. **`StashPop() error`**:
   - Execute `git stash pop`
   - Return error if pop fails (e.g., conflicts, empty stash)
   - Log pop operation to debug logger

3. **`HasStash() (bool, error)`**:
   - Execute `git stash list`
   - Return true if output is non-empty
   - Used to check if stash exists before operations

**Why separate methods**:
- Stash and pop are distinct operations with different error conditions
- HasStash provides safety check before popping
- Follows Repository pattern with one method per operation

**Error handling**:
- Stash can fail if there's nothing to stash (not critical, but return error)
- StashPop can fail on conflicts (critical - return descriptive error)
- Include git command output in error messages for debugging

**Testing approach**:
- Write `TestStash` with sub-tests:
  - Stash uncommitted file, verify file disappears from working directory
  - HasStash returns true after stash
  - StashPop restores file to working directory
  - HasStash returns false after pop
  - Stash with no changes (should succeed but might report "No local changes")

---

### Task 6: Add ResetHard Method

**Goal**: Implement `git reset --hard` operation for resetting branches.

**Files to modify**:
- `internal/git/git.go`

**What to implement**:

Add `ResetHard(ref string) error` method to `Repository` struct (after stash methods):

1. **Validation**:
   - Ref cannot be empty
   - Verify ref exists using `git rev-parse --verify ref` before resetting
   - Return descriptive error if ref doesn't exist

2. **Functionality**:
   - Execute `git reset --hard ref`
   - Log operation at Info level (this is a destructive operation)
   - Include ref name in log message

3. **Safety considerations**:
   - This method discards uncommitted changes
   - Document in godoc comment: ⚠️  WARNING: Discards uncommitted changes. Caller must stash first if preservation needed.
   - Verification before reset prevents resetting to non-existent ref

**Why verify before reset**:
- Prevents cryptic git errors
- Allows returning user-friendly error message
- Follows defensive programming pattern

**Testing approach**:
- Write `TestResetHard`:
  - Create commit, add another commit on top, reset to first commit
  - Verify HEAD points to first commit after reset
  - Verify new file from second commit is gone
  - Test error case: reset to non-existent ref fails with clear message

---

### Task 7: Add CheckoutBranch Method

**Goal**: Implement branch checkout operation.

**Files to modify**:
- `internal/git/git.go`

**What to implement**:

Add `CheckoutBranch(name string) error` method to `Repository` struct (after `CreateBranch`):

1. **Validation**:
   - Branch name cannot be empty
   - Verify branch exists using go-git's `Reference()` method
   - Return error if branch doesn't exist

2. **Implementation approach**:
   - Use git CLI (`git checkout name`) instead of go-git's worktree operations
   - Reason: go-git's checkout is complex with worktree state; CLI is reliable
   - Log successful checkout at Debug level with branch name and commit hash

**Why use git CLI here**:
- go-git's worktree checkout can have issues with file permissions and modifications
- git CLI checkout is battle-tested and handles edge cases
- Follows pattern from other git operations in this codebase

**Testing approach**:
- Write `TestCheckoutBranch`:
  - Create feature branch from main
  - Checkout feature, verify current branch changes
  - Checkout back to main, verify current branch changes back
  - Test error: checkout non-existent branch fails

---

### Task 8: Add Branch Existence Check Helper

**Goal**: Add utility method to check if a branch exists locally or remotely.

**Files to modify**:
- `internal/git/git.go`

**What to implement**:

Add `BranchExists(name string) (bool, error)` method to `Repository` struct (after `CheckoutBranch`):

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
- Used in Task 10 to ensure unique branch names
- Prevents creating branches that collide with existing branches

**Testing approach**:
- Write `TestBranchExists`:
  - Test existing branch (main) returns true
  - Test non-existent branch returns false
  - Test newly created branch returns true

---

## Phase 3: Detection and Auto-Branch Logic

### Task 9: Create Auto-Branch Module Structure

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
   - For `DetectCommitsOnMain`, skip tests initially (mark with `t.Skip("Requires git fixtures")`)
   - Full integration tests will be added in Task 20

**Package dependencies**:
- Import `internal/config` for configuration
- Import `internal/git` for GitRepository interface
- Import `internal/logger` for structured logging
- Import `context` for context propagation

**Why this structure**:
- Separates detection from execution (follows Single Responsibility Principle)
- DetectionResult is reusable across the workflow
- ShouldAutoBranch provides clear decision point

---

### Task 10: Implement Branch Name Generation

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

**Random string generation**:
- Use `crypto/rand.Read()` for cryptographically secure random
- Encode as hex, take first N characters
- Fallback to timestamp-based if crypto/rand fails

**Why placeholders**:
- Flexibility for different workflows and teams
- {username} useful for multi-developer teams
- {datetime} provides human-readable uniqueness
- {timestamp} provides numeric uniqueness
- {random} for truly random branch names

**Testing approach**:
- Write `TestGenerateBranchName`:
  - Test empty pattern returns default format
  - Test "null" pattern triggers prompt flag
  - Test each placeholder type produces expected format
  - Use mock GitRepository to provide test `user.name`
- Write `TestSanitizeBranchName`:
  - Test cases: "John Doe" → "john-doe", "test..name" → "test--name", etc.
- Write `TestEnsureUniqueBranchName`:
  - Mock repo with existing branches: "feature/test", "feature/test-1"
  - Verify returns "feature/test-2"

**Mock implementation**:
- Create simple mock struct implementing GitRepository interface
- Store test data like `gitConfig map[string]string` and `existingBranches map[string]bool`
- Implement only methods needed for tests

---

### Task 11: Implement User Prompts

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

3. **`ShouldCreateBranch() (bool, error)`** (method on AutoBranchDetector):
   - If config.AutoCreateBranchFromMain == true, return true immediately
   - Otherwise, call promptYesNo("Create feature branch automatically?", true)
   - Return result

4. **`ShouldStashChanges() (bool, error)`** (method on AutoBranchDetector):
   - If config.AutoStashUncommittedChanges == true, return true immediately
   - Otherwise, call promptYesNo("You have uncommitted changes. Stash them?", true)

5. **`ShouldResetMain() (bool, error)`** (method on AutoBranchDetector):
   - If config.AutoResetMain == true, return true immediately
   - Otherwise, call promptYesNo("Reset main to origin/main?", true)

6. **`GetBranchName() (string, error)`** (method on AutoBranchDetector):
   - Call GenerateBranchName() to get (name, shouldPrompt, error)
   - If shouldPrompt, call promptBranchName()
   - If user input empty, generate default: `feature/auto-from-main-{timestamp}`
   - Call EnsureUniqueBranchName() on final name
   - Return unique name

**Why this pattern**:
- Config-first: respects user configuration
- Prompt fallback: gives control when config is false
- Consistent interface: all Should* methods return (bool, error)
- GetBranchName orchestrates: generation → prompt (if needed) → uniqueness check

**Testing approach**:
- Write `TestShouldCreateBranch`:
  - Test config true returns true without prompt
  - Prompt testing requires stdin mocking - document as "tested manually"
- Write `TestGetBranchName`:
  - Test with pattern configs using mock repo
  - Test that uniqueness check is called
- For actual prompt testing, rely on manual testing and integration tests

**Imports needed**:
- `bufio` for reading stdin
- `os` for stdin access
- `fmt` for Printf

---

### Task 12: Implement Core Auto-Branch Flow

**Goal**: Orchestrate the complete auto-branch workflow from detection to branch creation.

**Files to modify**:
- `internal/diff/auto_branch.go`

**What to implement**:

Add two key orchestration methods:

1. **`ExecuteAutoBranch(ctx context.Context, detection *DetectionResult) (*AutoBranchResult, error)`**:

   Define `AutoBranchResult` struct:
   - Fields: `BranchCreated string`, `CommitsMoved int`, `StashCreated bool`, `ResetPending bool`

   Workflow (each step can fail and abort):
   - **Step 1**: Call `ShouldCreateBranch()`, if false return error "cancelled by user"
   - **Step 2**: Check working directory status via `repo.GetWorkingDirectoryStatus()`
   - **Step 3**: If not clean, call `ShouldStashChanges()`, if yes call `repo.Stash()`, set `result.StashCreated = true`
   - **Step 4**: Call `GetBranchName()` to get unique branch name
   - **Step 5**: Create branch from current HEAD via `repo.CreateBranch(name, "")`
   - **Step 6**: Checkout new branch via `repo.CheckoutBranch(name)`
   - **Step 7**: If stashed, call `repo.StashPop()`, warn on error but don't fail
   - **Step 8**: Set `result.ResetPending = true`, return result

   Output messages:
   - "✓ Creating feature branch: {name}"
   - "✓ Stashing uncommitted changes..."
   - "✓ Created feature branch '{name}' with your {count} commits"
   - On stash pop failure: "⚠️  Warning: Failed to restore stashed changes: {error}"

   Logging:
   - Info log at start with detection details
   - Info log at end with result summary
   - Debug logs for each step

2. **`ResetMainBranch(ctx context.Context, defaultBranch, currentBranch string) error`**:

   This runs AFTER successful PR creation (called from cmd/diff.go):
   - **Step 1**: Call `ShouldResetMain()`, if false return nil (skip reset)
   - **Step 2**: Checkout defaultBranch via `repo.CheckoutBranch()`
   - **Step 3**: Reset to origin via `repo.ResetHard("origin/" + defaultBranch)`
   - **Step 4**: Checkout back to currentBranch

   Output messages:
   - "\n✓ Resetting main to origin/main..."
   - "✓ Reset main to origin/main"

   Error handling:
   - If any step fails, return error with context
   - Caller in cmd/diff.go will handle error gracefully (already have successful PR)

**Why two methods**:
- ExecuteAutoBranch runs BEFORE PR creation (prepares branch)
- ResetMainBranch runs AFTER PR creation (cleans up main)
- Split allows PR creation to happen in between (normal diff flow)
- ResetPending flag signals to caller that reset should happen later

**Error handling philosophy**:
- ExecuteAutoBranch: Any failure aborts before modifying repo
- Stash pop failure: Warn but don't fail (branch is created, user can manually pop)
- ResetMainBranch failure: Return error but let caller decide (PR already exists)

**Testing approach**:
- Write `TestExecuteAutoBranch`:
  - Mark as integration test: `if testing.Short() { t.Skip() }`
  - Full integration test will be in Task 20
  - Unit tests for individual methods already covered in previous tasks

---

## Phase 4: Integration with diff Command

*This phase connects auto-branch detection to the actual diff command.*

### Task 13: Integrate Detection into diff Command

**Goal**: Add auto-branch detection and execution at the start of `runDiff` function.

**Files to modify**:
- `cmd/diff.go`

**Location**: In `runDiff` function after git repository is opened and current branch is obtained (around line 154).

**What to implement**:

1. **Import the diff package**:
   - Add import for `internal/diff` package (use package alias if needed)

2. **Create detector instance** (after line 157 where currentBranch is logged):
   - Initialize `AutoBranchDetector` with gitRepo and cfg.Diff
   - Store in variable for use throughout function

3. **Detect commits on main**:
   - Call `DetectCommitsOnMain(ctx)`
   - Handle errors (detection failure should fail the command)
   - Store result for later use

4. **Execute auto-branch if detected**:
   - Declare variable for `*AutoBranchResult` (will store result if auto-branch runs)
   - Check if `ShouldAutoBranch(detection)` returns true
   - If true:
     - Print warning: "\n⚠️  Warning: You have {count} commits on {branch}\n"
     - Call `ExecuteAutoBranch(ctx, detection)`
     - Handle errors (user cancellation, stash failure, etc.)
     - Update `currentBranch` variable to `autoBranchResult.BranchCreated`
     - Log success at Info level

5. **Continue normal flow**:
   - After auto-branch block, continue to GitHub client creation
   - Rest of runDiff uses updated `currentBranch` variable
   - PR will be created from new feature branch

**Why this location**:
- After git repo is opened (need repo operations)
- Before GitHub client creation (don't need GitHub API for detection)
- Before any PR operations (must be on feature branch first)
- currentBranch variable is used throughout remaining function

**Error handling**:
- Detection failure: return error immediately
- Auto-branch failure: return error with context
- User cancellation: handled in Task 15 with friendly message

**Testing approach**:
- Verify project builds
- Manual testing: create commits on main, run `gh arc diff`
- Full e2e testing in Task 20

---

### Task 14: Add Reset After Successful PR Creation

**Goal**: Reset main branch to origin/main after PR is successfully created.

**Files to modify**:
- `cmd/diff.go`

**Location**: At the end of `runDiff` function, after the success message (around line 644).

**What to implement**:

1. **Add reset logic before final return**:
   - Check if `autoBranchResult` is not nil (auto-branch was used)
   - Check if `autoBranchResult.ResetPending` is true
   - If both true:
     - Call `autoBranchDetector.ResetMainBranch(ctx, detection.DefaultBranch, currentBranch)`
     - Handle error gracefully (warn but don't fail command)

2. **Error handling for reset**:
   - If reset fails, log warning (logger.Warn)
   - Print warning to user: "\n⚠️  Warning: Failed to reset main: {error}"
   - Print manual instructions:
     - "You can manually reset with:"
     - "  git checkout {defaultBranch}"
     - "  git reset --hard origin/{defaultBranch}"
     - "  git checkout {currentBranch}"
   - Do NOT return error (PR creation already succeeded, reset is cleanup)

**Why handle reset errors gracefully**:
- PR creation is the primary operation - already succeeded
- Reset is cleanup/convenience
- User can manually reset if auto-reset fails
- Failing the entire command would be confusing (PR exists but command "failed")

**Testing approach**:
- Verify project builds
- Manual testing: complete full flow, verify main gets reset
- Check that PR creation success message appears before reset

---

### Task 15: Add User-Friendly Error Messages

**Goal**: Improve error messages when auto-branch flow is cancelled or fails.

**Files to modify**:
- `cmd/diff.go`

**Location**: In the auto-branch execution block (Task 13), enhance error handling.

**What to implement**:

Enhance the error handling after `ExecuteAutoBranch` call:

1. **Check for user cancellation**:
   - Use `strings.Contains(err.Error(), "cancelled by user")`
   - If cancelled, print helpful message:
     - "\n✗ Cannot create PR from main to main."
     - "Please create a feature branch manually:"
     - Step-by-step commands using actual branch names from detection
     - Include all commands: checkout -b, checkout main, reset, checkout feature, gh arc diff
   - Return error "operation cancelled"

2. **For other errors**:
   - Return error with context: "auto-branch flow failed: {error}"
   - Don't wrap or modify the error (preserves specific error information)

**Why distinguish cancellation**:
- User cancellation is expected behavior, not a failure
- Other errors (git failures, stash conflicts) are unexpected
- Cancellation needs manual instructions
- Other errors need error details for troubleshooting

**Testing approach**:
- Verify project builds
- Manual testing: decline auto-branch prompt, verify helpful message
- Manual testing: cause stash failure, verify error propagation

---

## Phase 5: User Interaction and Polish

*This phase improves the user experience with better output and logging.*

### Task 16: Add Progress Indicators

**Goal**: Improve visual feedback during auto-branch operations.

**Files to modify**:
- `internal/diff/auto_branch.go`

**What to polish**:

Review and enhance all user-facing messages in `ExecuteAutoBranch`:

1. **Visual separation**:
   - Add blank line (`fmt.Println()`) after user confirms branch creation
   - Separates prompts from progress messages
   - Makes output easier to scan

2. **Consistent formatting**:
   - All success messages start with "✓"
   - All warnings start with "⚠️"
   - All errors start with "✗"
   - Indent sub-messages with 2 spaces

3. **Progress clarity**:
   - Each step that takes time should have output
   - Match formatting style from existing diff command
   - Keep messages concise but informative

**Why polish matters**:
- Auto-branch involves multiple git operations
- User needs to see progress to know tool isn't hung
- Consistent formatting matches rest of gh-arc
- Visual separation helps parse output quickly

**Testing approach**:
- Run through full flow manually
- Verify output is clear and well-formatted
- Check that messages appear at appropriate times

---

### Task 17: Add Comprehensive Debug Logging

**Goal**: Add structured logging throughout auto-branch flow for troubleshooting.

**Files to modify**:
- `internal/diff/auto_branch.go`

**What to log**:

Add structured logging at key points in `ExecuteAutoBranch`:

1. **At start**:
   - Info level: defaultBranch, commitsAhead
   - Message: "Starting auto-branch flow"

2. **Working directory status**:
   - Debug level: number of staged/unstaged/untracked files
   - Message: "Working directory has uncommitted changes" or "Working directory is clean"

3. **Stash operations**:
   - Debug level before: "Stashing uncommitted changes"
   - Debug level after: stashCreated=true, "Successfully stashed changes"

4. **Branch operations**:
   - Debug level: branchName
   - Message: "Creating feature branch" / "Checked out feature branch"

5. **At end**:
   - Info level: branchName, commitsMoved, stashCreated
   - Message: "Auto-branch flow completed successfully"

**Why structured logging**:
- Debug flag (`-v`) enables verbose output
- Structured fields (Str, Int, Bool) are easier to parse than plain text
- Helps troubleshoot issues in production use
- Follows existing logging patterns in gh-arc

**Testing approach**:
- Run with verbose flag: `./gh-arc diff -v`
- Verify debug messages appear
- Check that structured fields are populated correctly

---

### Task 18: Create Configuration Examples

**Goal**: Provide example configuration files for different use cases.

**Files to create**:
- `docs/wip/auto-branch-from-main/examples/config-auto.yaml`
- `docs/wip/auto-branch-from-main/examples/config-manual.yaml`
- `docs/wip/auto-branch-from-main/examples/config-custom-pattern.yaml`

**What to create**:

1. **config-auto.yaml** (Fully automatic, no prompts):
   - All auto-branch booleans set to true
   - Empty pattern (uses default)
   - Include comments explaining "no prompts" behavior
   - Show other common diff settings for context

2. **config-manual.yaml** (All prompts, maximum control):
   - All auto-branch booleans set to false
   - Pattern set to "null" (prompt for name)
   - Include comments explaining each prompt that will appear
   - Note which prompts happen when

3. **config-custom-pattern.yaml** (Custom naming patterns):
   - Show 4-5 different pattern examples with their outputs
   - Comment out alternatives (only one active)
   - Include explanation of each placeholder
   - Show how patterns generate names

**Format**:
- Use YAML for readability (JSON also supported but YAML has comments)
- Add descriptive header comment in each file
- Explain use case for each configuration
- Include example outputs in comments

**Why provide examples**:
- Users can copy-paste and modify rather than writing from scratch
- Shows common configurations (automatic vs manual)
- Demonstrates placeholder usage
- Reduces configuration errors

---

## Phase 6: Documentation and Testing

*This phase completes the feature with documentation and comprehensive tests.*

### Task 19: Write User-Facing Documentation

**Goal**: Document the auto-branch feature for end users.

**Files to modify**:
- `README.md`

**Location**: In the configuration section, add new subsection after existing diff configuration.

**What to document**:

1. **Feature Overview**:
   - One-paragraph explanation of what auto-branch does
   - Show example output (terminal session)
   - Explain when it activates

2. **Configuration Section**:
   - Table or list of configuration fields
   - Explain each field's purpose
   - Show default values
   - Provide JSON example

3. **Branch Naming Section**:
   - Explain pattern system
   - List all placeholders with descriptions
   - Show 3-4 example patterns with generated outputs
   - Explain empty string vs "null" behavior

4. **Disabling/Customizing Section**:
   - How to make it prompt instead of automatic
   - How to completely disable
   - What happens when you decline prompts

**Style guidance**:
- Use code blocks for configuration examples
- Use bash syntax highlighting for terminal examples
- Keep explanations concise
- Match existing README formatting

**Testing approach**:
- Preview README rendering locally or on GitHub
- Verify all code blocks have proper syntax highlighting
- Check that examples are copy-paste ready

---

### Task 20: Write Comprehensive Integration Tests

**Goal**: Add end-to-end integration tests for the complete auto-branch flow.

**Files to create**:
- `internal/diff/auto_branch_integration_test.go`

**What to test**:

Create integration test file with build tag `// +build integration` at top.

Write `TestAutoBranch_Integration` with sub-tests:

1. **Full automatic flow**:
   - Set up: Create temp git repo with initial commit
   - Simulate: Add 2 commits on main
   - Configure: All automatic (all booleans true)
   - Execute: Run detection and auto-branch flow
   - Verify:
     - Detection found 2 commits on main
     - Branch was created with expected name pattern
     - Current branch is new feature branch
     - Main still has all commits (branch was created, not moved)
     - New branch has same commits as main

2. **With uncommitted changes**:
   - Set up: Repo with commits on main + uncommitted file
   - Execute: Auto-branch with auto-stash enabled
   - Verify:
     - File was stashed (disappeared during branch creation)
     - File was restored (reappeared after checkout)
     - Working directory state preserved

3. **Custom branch name pattern**:
   - Set up: Config with pattern `feature/{username}-{date}`
   - Execute: Auto-branch flow
   - Verify: Generated name matches pattern format

4. **Skip if short mode**:
   - Add `if testing.Short() { t.Skip() }` to all tests
   - These tests are slow due to git operations

**Test helpers needed**:
- `createTestRepo(t)` - Set up temp git repo with initial commit
- Use go-git for repository operations in tests
- Use `t.TempDir()` for automatic cleanup

**Why integration tests**:
- Unit tests cover individual methods
- Integration tests verify complete workflow
- Ensure git operations work correctly together
- Catch issues with git state management

**Running tests**:
- Document that these require `--tags=integration` flag
- Excluded from normal `go test ./...` (too slow)
- Run explicitly during development and before release

---

### Task 21: Update Architecture Documentation

**Goal**: Document the auto-branch module in the architecture guide.

**Files to modify**:
- `docs/contributing/ARCHITECTURE.md`

**Location**: In the `internal/diff/` package section, add new subsection for auto-branch.

**What to document**:

1. **Module Overview**:
   - Purpose: handles commits on main scenario
   - Flow diagram: Detection → Confirm → Stash → Create → Checkout → Unstash
   - When it runs: at start of diff command

2. **Key Types**:
   - `AutoBranchDetector`: main orchestrator
   - `DetectionResult`: detection output
   - `AutoBranchResult`: execution output
   - Explain fields and purpose of each

3. **Key Methods**:
   - List main methods with one-sentence descriptions
   - Explain orchestration pattern (detection separate from execution)
   - Note which methods prompt vs which are automatic

4. **Configuration**:
   - List config fields
   - Explain config-first pattern (check config before prompting)
   - Note defaults (all enabled)

5. **Integration Point**:
   - Where it's called in cmd/diff.go
   - Two-phase execution: before PR creation, after PR creation
   - Why it's split (reset happens after PR)

6. **Thread Safety**:
   - Note: not thread-safe, designed for single-user CLI
   - No concurrent access expected

**Style guidance**:
- Match existing architecture doc formatting
- Use code blocks for type definitions
- Use diagrams (ASCII art) if helpful
- Keep technical but readable

---

### Task 22: Create Architecture Decision Record

**Goal**: Document the architectural decision in an ADR.

**Files to create**:
- `docs/adr/0002-auto-branch-from-main.md`

**What to include**:

Structure following ADR template (see existing ADR-0001):

1. **Status**: Accepted

2. **Date**: 2025-10-16

3. **Context**:
   - Problem: users commit directly to main by mistake
   - Current state: diff fails with confusing error
   - Impact: interrupts workflow, manual fix is error-prone
   - Alternatives considered: reject and error, auto-fix silently, prompt-based fix

4. **Decision**:
   - Implement automatic detection and branch creation
   - Make behavior configurable (automatic vs prompt)
   - Support custom branch naming
   - Reset main after successful PR

5. **Consequences**:
   - Positive: seamless workflow, prevents mistakes, configurable
   - Negative: adds complexity, more git operations, requires testing
   - Neutral: changes default behavior (but only when commits on main)

6. **References**:
   - Link to design document
   - Link to implementation plan
   - Link to any related issues/discussions

**Why create ADR**:
- Documents decision rationale for future maintainers
- Explains why this approach over alternatives
- Records trade-offs and considerations
- Standard practice for significant architectural changes

---

## Summary

This implementation plan provides 22 tasks organized into 6 phases:

1. **Phase 1 (Tasks 1-3)**: Configuration infrastructure
2. **Phase 2 (Tasks 4-8)**: Git operations
3. **Phase 3 (Tasks 9-12)**: Detection and auto-branch logic
4. **Phase 4 (Tasks 13-15)**: Integration with diff command
5. **Phase 5 (Tasks 16-18)**: User interaction and polish
6. **Phase 6 (Tasks 19-22)**: Documentation and testing

Each task specifies:
- **Goal**: What to achieve
- **Files**: Which files to modify/create
- **What to implement**: Detailed description of changes needed
- **Why**: Rationale for the approach
- **Testing approach**: What to test and how

## Implementation Guidelines

**Follow TDD**:
- Write tests before or alongside implementation
- Each task includes testing guidance
- Mark slow tests with `testing.Short()` check

**Commit Frequently**:
- Commit after each task (minimum 22 commits)
- Each commit should be atomic and reversible
- Build should pass after every commit

**DRY Principle**:
- Reuse existing patterns from the codebase
- Extract common functionality to helpers
- Follow existing code style

**YAGNI Principle**:
- Implement only what's specified in this plan
- Don't add speculative features
- Keep it simple

## Testing Strategy

- **Unit tests**: Tasks 2, 4, 5, 6, 7, 8, 10, 11
- **Integration tests**: Task 20
- **Manual testing**: After each phase
- **End-to-end testing**: After Phase 4

## Questions or Issues?

Refer to:
- Feature design: `docs/wip/auto-branch-from-main/feature-design.md`
- Architecture guide: `docs/contributing/ARCHITECTURE.md`
- Testing guide: `docs/contributing/TESTING.md`
- Workflow guide: `docs/contributing/WORKFLOWS.md`
