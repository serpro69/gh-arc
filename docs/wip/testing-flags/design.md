# Testing Flags Feature Design

## Overview

Add command-line flags to `gh-arc` commands to enable comprehensive end-to-end testing without requiring human interaction, network access, or modifying real repositories.

## Motivation

### Current Testing Challenges

1. **Editor Interaction**: Commands like `diff` open `$EDITOR`, requiring manual input
2. **Network Dependencies**: GitHub API calls require authentication and network access
3. **Destructive Operations**: Tests modify real repositories (commits, pushes, branch creation)
4. **CI/CD Integration**: Automated tests can't handle interactive prompts
5. **Limited Testability**: Can't easily verify command behavior without side effects

### Use Cases Beyond Testing

These flags also provide value to end users:

- **`--dry-run`**: Preview what a command would do before executing
- **`--offline`**: Work on planes/trains or with unreliable network
- **`--no-edit`**: Scripting and automation workflows
- **Combined**: Quickly validate commands in scripts without side effects

## Proposed Flags

### 1. `--no-edit` Flag

**Purpose**: Skip interactive editor step, use defaults or provided values.

**Behavior**:
- When set, don't open `$EDITOR`
- For mandatory fields (title, test plan), use:
  - Values from command-line flags (`--title`, `--body`, etc.) if provided
  - Placeholder values if not provided
  - Extract from commits if possible (existing behavior)

**Example Usage**:

```bash
# Use placeholders for mandatory fields
gh arc diff --no-edit

# Provide values via flags
gh arc diff --no-edit --title "My Feature" --body "Description" --test-plan "Manual testing"

# Extract from commits (current behavior, no editor)
gh arc diff --no-edit  # Title from first commit, body from all commits
```

**Placeholder Values**:
```
Test Plan: [Manual testing required]
```

**Implementation**:
- Add `--no-edit` flag to `diff` command
- Add `--title`, `--body`, `--test-plan`, `--reviewers`, flags for explicit values
- Modify `template.OpenEditor()` to skip editor when `--no-edit` flag set
- Fall back to `template.GenerateDefaults()` for missing required fields

### 2. `--dry-run` Flag

**Purpose**: Show what would happen without executing any operations.

**Behavior**:
- Execute **NO** side-effect operations:
  - No git commands (commit, push, checkout, branch creation)
  - No GitHub API calls (PR creation, updates, reviews)
  - No file modifications (config changes, template files)
- Print detailed output showing what **would** be executed
- Validate inputs and configuration
- Exit with appropriate code (0 if would succeed, 1 if would fail)

**Output Format**:

```
[DRY RUN] Checkout branch: feature/auto-from-main-1729180000
[DRY RUN] Push branch: feature/auto-from-main-1729180000 to origin
[DRY RUN] Create PR:
  Title: Add feature
  Base: main
  Head: feature/auto-from-main-1729180000
  Draft: false
[DRY RUN] Assign reviewers: john, jane
[DRY RUN] Checkout branch: feature/auto-from-main-1729180000

✓ Profit
```

**Example Usage**:

```bash
# Preview diff command
gh arc diff --dry-run

# Preview with auto-branch
gh arc diff --dry-run  # On main with commits

# Preview land command
gh arc land --dry-run

# Combine with no-edit
gh arc diff --dry-run --no-edit
```

**Implementation**:
- Add global `--dry-run` flag (available on all commands)
- Pass `DryRun: true` context through command execution
- Wrap all side-effect operations with dry-run checks:
  ```go
  if !ctx.DryRun {
      err := repo.Push(ctx, branchName)
  } else {
      logger.Info().Msgf("[DRY RUN] Would push branch: %s to origin", branchName)
  }
  ```
- Validation still runs (catch errors early)

### 3. `--offline` Flag

**Purpose**: Execute local operations only, skip network-dependent operations.

**Behavior**:
- Execute **local** operations normally:
  - Git operations (commit, checkout, branch creation, diff)
  - Configuration loading
  - Template generation
  - File operations
- **Skip** network operations:
  - GitHub API calls (PR creation, fetching PRs)
  - `git push` to remote
  - `git fetch` from remote
- Print what was skipped and why
- Exit with success if local operations work

**Output Format**:

```
✓ Detected 2 commits on main
✓ Generated branch name: feature/auto-from-main-1729180000
✓ Created local branch: feature/auto-from-main-1729180000
[OFFLINE] Skipped push to origin (would push feature/auto-from-main-1729180000)
[OFFLINE] Skipped PR creation (would create PR: "Add feature" from feature/auto-from-main-1729180000 to main)
✓ Checked out branch: feature/auto-from-main-1729180000

⚠ Completed in offline mode. Manual steps required:
  1. Push branch: git push origin feature/auto-from-main-1729180000
  2. Create PR: gh pr create --head feature/auto-from-main-1729180000 --base main
```

**Example Usage**:

```bash
# Work offline (airplane, train, etc.)
gh arc diff --offline

# Prepare work for later push
gh arc diff --offline --no-edit

# Test local git operations without touching remote
gh arc diff --offline  # Useful for development/testing
```

**Implementation**:
- Add global `--offline` flag
- Pass `Offline: true` context through command execution
- Wrap network operations:
  ```go
  if !ctx.Offline {
      err := repo.Push(ctx, branchName)
  } else {
      logger.Warn().Msgf("[OFFLINE] Skipped push to origin (would push %s)", branchName)
  }
  ```
- At end, print summary of skipped operations with manual commands

## Flag Combinations

### Valid Combinations

| Flags | Behavior | Use Case |
|-------|----------|----------|
| `--dry-run` | Nothing executed, show preview | Preview command safely |
| `--offline` | Local ops only, skip network | Work without internet |
| `--no-edit` | Skip editor, use defaults | Automation/scripting |
| `--dry-run --no-edit` | Preview without editor | Quick validation |
| `--offline --no-edit` | Local ops only, no editor | Fast offline workflow |
| `--dry-run --offline` | Preview, assume offline | Test command logic |

### Flag Precedence

When multiple flags conflict:
1. `--dry-run` takes precedence (nothing executed)
2. `--offline` prevents network operations
3. `--no-edit` prevents editor invocation

Example:
```bash
gh arc diff --dry-run --offline --no-edit
# Result: Pure dry-run (nothing executed), shows what would happen offline without editor
```

## Implementation Plan

### Phase 1: Infrastructure

1. **Add flag definitions**:
   ```go
   // cmd/root.go - Global flags
   rootCmd.PersistentFlags().Bool("dry-run", false, "Show what would be executed without doing it")
   rootCmd.PersistentFlags().Bool("offline", false, "Execute local operations only, skip network calls")

   // cmd/diff.go - Command-specific flags
   diffCmd.Flags().Bool("no-edit", false, "Skip interactive editor, use defaults")
   diffCmd.Flags().String("title", "", "PR title (used with --no-edit)")
   diffCmd.Flags().String("body", "", "PR body/summary (used with --no-edit)")
   diffCmd.Flags().String("test-plan", "", "PR test plan (used with --no-edit)")
   ```

2. **Create execution context**:
   ```go
   // internal/context/context.go
   type ExecutionContext struct {
       Context  context.Context
       DryRun   bool
       Offline  bool
       NoEdit   bool
       // ... other flags
   }
   ```

3. **Pass context through commands**:
   ```go
   func runDiff(cmd *cobra.Command, args []string) error {
       execCtx := &context.ExecutionContext{
           Context: cmd.Context(),
           DryRun:  viper.GetBool("dry-run"),
           Offline: viper.GetBool("offline"),
           NoEdit:  viper.GetBool("no-edit"),
       }

       return executeDiff(execCtx, cfg)
   }
   ```

### Phase 2: Wrap Side Effects

1. **Git operations** (`internal/git/git.go`):
   ```go
   func (r *Repository) Push(ctx *context.ExecutionContext, branchName string) error {
       if ctx.DryRun {
           logger.Info().Msgf("[DRY RUN] Would push branch: %s to origin", branchName)
           return nil
       }

       if ctx.Offline {
           logger.Warn().Msgf("[OFFLINE] Skipped push to origin (would push %s)", branchName)
           return ErrSkippedOffline
       }

       // Actual push logic
       return r.pushImpl(ctx.Context, branchName)
   }
   ```

2. **GitHub operations** (`internal/github/client.go`):
   ```go
   func (c *Client) CreatePullRequest(ctx *context.ExecutionContext, opts *PROptions) (*PullRequest, error) {
       if ctx.DryRun {
           logger.Info().Msg("[DRY RUN] Would create PR:")
           logger.Info().Msgf("  Title: %s", opts.Title)
           logger.Info().Msgf("  Base: %s", opts.Base)
           logger.Info().Msgf("  Head: %s", opts.Head)
           return &PullRequest{Number: 0, URL: "https://github.com/..."}, nil
       }

       if ctx.Offline {
           logger.Warn().Msg("[OFFLINE] Skipped PR creation")
           return nil, ErrSkippedOffline
       }

       // Actual PR creation
       return c.createPRImpl(ctx.Context, opts)
   }
   ```

3. **Template operations** (`internal/template/template.go`):
   ```go
   func OpenEditor(ctx *context.ExecutionContext, content string) (string, error) {
       if ctx.NoEdit {
           logger.Info().Msg("Skipping editor (--no-edit flag set)")
           return content, nil  // Return generated content as-is
       }

       if ctx.DryRun {
           logger.Info().Msg("[DRY RUN] Would open editor with content:")
           logger.Info().Msg(content)
           return content, nil
       }

       // Actual editor logic
       return openEditorImpl(content)
   }
   ```

### Phase 3: Testing Integration

1. **E2E test script**:
   ```bash
   test_auto_branch_full_workflow() {
       cd "$TEST_REPO"
       git checkout main
       create_test_commits 2
       create_test_config true

       # Execute actual binary in dry-run mode
       output=$("$GH_ARC_BIN" diff --dry-run --no-edit 2>&1)

       # Verify output
       assert_contains "$output" "Would push branch: feature/auto-from-main"
       assert_contains "$output" "Would create PR"

       # Verify no actual changes
       assert_on_branch "main"
       assert_no_remote_branches "feature/auto-from-main"
   }

   test_auto_branch_offline() {
       cd "$TEST_REPO"
       git checkout main
       create_test_commits 2
       create_test_config true

       # Execute in offline mode
       output=$("$GH_ARC_BIN" diff --offline --no-edit 2>&1)

       # Verify local operations executed
       assert_not_on_branch "main"
       assert_branch_exists "feature/auto-from-main-*"

       # Verify remote operations skipped
       assert_no_remote_branches "feature/auto-from-main"
       assert_contains "$output" "Skipped push to origin"
   }
   ```

2. **Go integration tests**:
   ```go
   func TestDiffCommand_DryRun(t *testing.T) {
       tmpDir, gitRepo := createTestRepo(t)
       createCommit(t, gitRepo, tmpDir, "Test commit")

       // Execute command in dry-run mode
       cmd := exec.Command(ghArcBinary, "diff", "--dry-run", "--no-edit")
       cmd.Dir = tmpDir
       output, err := cmd.CombinedOutput()

       // Verify no errors
       if err != nil {
           t.Fatalf("Command failed: %v\nOutput: %s", err, output)
       }

       // Verify output shows dry-run actions
       if !strings.Contains(string(output), "[DRY RUN]") {
           t.Errorf("Expected dry-run output, got: %s", output)
       }

       // Verify no actual changes
       currentBranch := getCurrentBranch(t, gitRepo)
       if currentBranch != "main" {
           t.Errorf("Branch should still be main, got: %s", currentBranch)
       }
   }
   ```

### Phase 4: Documentation

1. **Update command help text**
2. **Add examples to README**
3. **Update testing guide with E2E test instructions**
4. **Document flag combinations and use cases**

## Error Handling

### Sentinel Errors

```go
// internal/context/errors.go
var (
    ErrSkippedOffline = errors.New("operation skipped in offline mode")
    ErrSkippedDryRun  = errors.New("operation skipped in dry-run mode")
)
```

### Error Messages

```go
// When validation fails in dry-run
if ctx.DryRun && err != nil {
    logger.Error().Err(err).Msg("[DRY RUN] Command would fail:")
    return fmt.Errorf("dry-run validation failed: %w", err)
}

// When offline mode can't proceed
if ctx.Offline && requiresNetwork {
    logger.Error().Msg("[OFFLINE] Cannot proceed without network access")
    return ErrSkippedOffline
}
```

## User Experience

### Success Output (Dry Run)

```
$ gh arc diff --dry-run --no-edit

Auto-Branch Detection:
✓ Detected 2 commits on main
✓ Would create branch: feature/auto-from-main-1729180000

[DRY RUN] Would execute the following operations:

Git Operations:
  [DRY RUN] git push origin HEAD:feature/auto-from-main-1729180000
  [DRY RUN] git checkout -b feature/auto-from-main-1729180000 origin/feature/auto-from-main-1729180000

GitHub Operations:
  [DRY RUN] Create Pull Request:
    Title: Add new feature
    Base: main
    Head: feature/auto-from-main-1729180000
    Draft: false
    Reviewers: john, jane

✓ Command would succeed
```

### Success Output (Offline)

```
$ gh arc diff --offline --no-edit

Auto-Branch Detection:
✓ Detected 2 commits on main
✓ Generated branch: feature/auto-from-main-1729180000

Executing local operations:
✓ Created branch: feature/auto-from-main-1729180000
✓ Checked out branch: feature/auto-from-main-1729180000

Offline operations skipped:
  [OFFLINE] Push to origin (branch: feature/auto-from-main-1729180000)
  [OFFLINE] Create Pull Request

⚠ Manual steps required to complete workflow:

  1. Push branch to remote:
     git push origin feature/auto-from-main-1729180000

  2. Create pull request:
     gh pr create --head feature/auto-from-main-1729180000 --base main \
       --title "Add new feature" --body "..." --draft

✓ Local operations completed successfully
```

### Error Output (Dry Run)

```
$ gh arc diff --dry-run --no-edit

Auto-Branch Detection:
✓ Detected 2 commits on main
✗ Branch name pattern invalid: "feature//{invalid}"

[DRY RUN] Validation failed. Command would not succeed.

Error: invalid branch name pattern in configuration
  Pattern: "feature//{invalid}"
  Issue: consecutive slashes not allowed

Fix: Update .arc.json with valid pattern, e.g.:
  "autoBranchNamePattern": "feature/auto-from-main-{timestamp}"

✗ Command would fail
```

## Testing Strategy

### Unit Tests
- Test flag parsing and context creation
- Test operation wrappers (dry-run/offline checks)
- Test placeholder value generation

### Integration Tests
- Test commands with each flag combination
- Test that side effects don't occur in dry-run
- Test that network operations skipped in offline
- Test error handling in each mode

### E2E Tests
- Execute actual binary with flags
- Verify repository state after execution
- Verify output messages
- Test against both local and real GitHub repos

### Test Matrix

| Scenario | Flags | Expected Outcome |
|----------|-------|------------------|
| Preview changes | `--dry-run` | No side effects, detailed output |
| Work offline | `--offline` | Local changes only |
| Automation | `--no-edit` | Non-interactive execution |
| Fast preview | `--dry-run --no-edit` | Quick validation |
| Prepare work | `--offline --no-edit` | Local branch ready to push |
| Development | `--dry-run --offline` | Test logic without any effects |

## Benefits

### For Testing

1. **Comprehensive E2E Tests**: Run actual binary without side effects
2. **CI/CD Integration**: Automated tests don't need GitHub credentials
3. **Fast Feedback**: Dry-run tests are instant
4. **Real Behavior**: Test actual command code paths, not mocks

### For Users

1. **Safety**: Preview destructive operations
2. **Offline Work**: Prepare PRs on planes/trains
3. **Automation**: Script workflows without interactive prompts
4. **Learning**: See what commands do before executing

### For Development

1. **Faster Development**: Test changes without network calls
2. **Easier Debugging**: See what would execute step-by-step
3. **Better Documentation**: Dry-run output serves as examples

## Future Enhancements

1. **`--dry-run --json`**: Combine dry-run with json-formatted output for machine-readable dry-run output
2. **`-vvv --dry-run`**: Show even more detail (API payloads, git commands)
    - Examples:
        - Execute git commands with `--dry-run` flag, e.g. `git add file --dry-run` or `git push --dry-run`
    - Extra Notes:
        - Make sure all output is machine-readable if combined with `--json` flag; easiest is probably to skip the output from `git <cmd> --dry-run` if `--json` is enabled?
4. **`--record`**: Record operations for later replay
5. **`--replay`**: Execute recorded operations

## Related Work

Similar patterns in other tools:

- **Terraform**: `terraform plan` (dry-run) vs `terraform apply`
- **Ansible**: `--check` flag for dry-run
- **Git**: `--dry-run` on many commands
- **Docker**: `--dry-run` for compose operations
- **kubectl**: `--dry-run=client|server` flags

## Questions for Consideration

1. Should `--dry-run` validate GitHub credentials? (I think yes, to catch auth errors)
    A: yes
2. Should `--offline` cache PR data for display? (Nice to have)
    A: yes
3. Should `--no-edit` support reading from stdin? (Useful for piping)
    A: yes
4. Should flags be global or per-command? (Global seems better)
    A: Global, apart from `--no-edit` since we probably don't open $EDITOR with other commands
5. Should we add `--yes` flag to skip confirmations? (Yes, for full automation)
    A: yes

## Summary

These flags transform `gh-arc` from a difficult-to-test interactive tool into a fully testable, automatable CLI that can be validated comprehensively with real E2E tests while also providing significant value to end users for previewing, offline work, and automation.
