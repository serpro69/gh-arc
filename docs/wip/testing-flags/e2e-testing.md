# End-to-End Testing with Testing Flags

This document describes how to write comprehensive E2E tests that execute the actual `gh-arc` binary using the testing flags (`--dry-run`, `--offline`, `--no-edit`).

## Prerequisites

1. Testing flags implemented (see `design.md`)
2. Built `gh-arc` binary
3. Test repository setup (local git repo with simulated remote)

## Test Categories

### 1. Dry-Run Tests (Pure Validation)

**Purpose**: Verify command logic without any side effects.

**Characteristics**:
- No git operations executed
- No GitHub API calls
- No file modifications
- Fast execution
- Safe to run anywhere

**Example Test**:

```bash
test_auto_branch_dry_run_happy_path() {
    start_test "Auto-branch dry-run happy path"
    reset_test_repo

    # Setup: Commits on main
    git checkout main
    create_test_commits 2
    create_test_config true "feature/test-{timestamp}"

    # Execute: Dry-run mode
    output=$("$GH_ARC_BIN" diff --dry-run --no-edit 2>&1)
    exit_code=$?

    # Verify: Exit code
    assert_equals 0 $exit_code "Should succeed"

    # Verify: Output shows planned operations
    assert_contains "$output" "[DRY RUN]" "Should show dry-run prefix"
    assert_contains "$output" "Would push branch: feature/test-" "Should show push"
    assert_contains "$output" "Would create PR" "Should show PR creation"
    assert_contains "$output" "Would checkout branch" "Should show checkout"

    # Verify: No actual changes
    current_branch=$(git branch --show-current)
    assert_equals "main" "$current_branch" "Should still be on main"

    # Verify: No branches created
    branch_count=$(git branch | grep -c "feature/test" || echo 0)
    assert_equals 0 $branch_count "No feature branches should exist"

    # Verify: No remote branches
    remote_branches=$(git ls-remote origin | grep -c "feature/test" || echo 0)
    assert_equals 0 $remote_branches "No remote branches should exist"

    pass_test "Auto-branch dry-run happy path"
}
```

**Test Matrix for Dry-Run**:

```bash
# Detection scenarios
test_dry_run_on_main_with_commits()          # Should show auto-branch flow
test_dry_run_on_feature_branch()             # Should show normal diff
test_dry_run_no_commits()                    # Should show "no changes"

# Pattern generation
test_dry_run_timestamp_pattern()             # Verify pattern shows timestamp
test_dry_run_date_pattern()                  # Verify pattern shows date
test_dry_run_username_pattern()              # Verify pattern shows username
test_dry_run_random_pattern()                # Verify pattern shows random

# Error cases
test_dry_run_invalid_config()                # Should show validation error
test_dry_run_stale_remote()                  # Should show stale warning
test_dry_run_collision()                     # Should show collision handling

# Edge cases
test_dry_run_master_branch()                 # Verify master detection
test_dry_run_multiple_commits()              # Show all commits
```

### 2. Offline Tests (Local Operations Only)

**Purpose**: Verify local git operations work correctly, skip network calls.

**Characteristics**:
- Git operations executed (commit, checkout, branch)
- No network operations (push, PR creation)
- Verifies local state changes
- Tests git integration

**Example Test**:

```bash
test_auto_branch_offline_creates_local_branch() {
    start_test "Auto-branch offline creates local branch"
    reset_test_repo

    # Setup: Commits on main
    git checkout main
    create_test_commits 2
    create_test_config true "feature/offline-{timestamp}"

    # Execute: Offline mode
    output=$("$GH_ARC_BIN" diff --offline --no-edit 2>&1)
    exit_code=$?

    # Verify: Success
    assert_equals 0 $exit_code "Should succeed"

    # Verify: Output shows offline operations
    assert_contains "$output" "[OFFLINE]" "Should show offline prefix"
    assert_contains "$output" "Skipped push to origin" "Should skip push"
    assert_contains "$output" "Skipped PR creation" "Should skip PR"
    assert_contains "$output" "Manual steps required" "Should show next steps"

    # Verify: Branch created locally
    current_branch=$(git branch --show-current)
    assert_contains "$current_branch" "feature/offline-" "Should be on feature branch"

    # Verify: Branch exists
    branch_exists=$(git branch | grep -c "feature/offline-" || echo 0)
    assert_equals 1 $branch_exists "Feature branch should exist locally"

    # Verify: Not pushed to remote
    remote_branches=$(git ls-remote origin | grep -c "feature/offline-" || echo 0)
    assert_equals 0 $remote_branches "Should not be pushed to remote"

    # Verify: Commits moved to new branch
    commits_on_branch=$(git log --oneline HEAD ^origin/main | wc -l)
    assert_equals 2 $commits_on_branch "Should have 2 commits on feature branch"

    # Verify: Main branch clean
    git checkout main
    commits_on_main=$(git log --oneline HEAD ^origin/main | wc -l)
    assert_equals 0 $commits_on_main "Main should be clean"

    pass_test "Auto-branch offline creates local branch"
}
```

**Test Matrix for Offline**:

```bash
# Happy paths
test_offline_creates_local_branch()          # Branch created, not pushed
test_offline_moves_commits()                 # Commits on new branch
test_offline_generates_correct_name()        # Pattern applied correctly

# Multiple commits
test_offline_with_many_commits()             # All commits moved

# Collision handling (local only)
test_offline_collision_detection()           # Detects local branch collision
test_offline_collision_retry()               # Appends counter locally

# Error cases
test_offline_uncommitted_changes()           # Fails to checkout
test_offline_invalid_pattern()               # Validation error

# Manual verification
test_offline_shows_manual_steps()            # Outputs push/PR commands
```

### 3. No-Edit Tests (Non-Interactive)

**Purpose**: Test automation workflows without editor interaction.

**Characteristics**:
- No editor opened
- Uses defaults or provided values
- Fast execution
- Suitable for CI/CD

**Example Test**:

```bash
test_auto_branch_no_edit_uses_defaults() {
    start_test "Auto-branch no-edit uses defaults"
    reset_test_repo

    # Setup: Commits with good messages
    git checkout main
    echo "feature 1" >> test.txt
    git add test.txt
    git commit -m "Add feature 1

This adds the first feature with tests."

    echo "feature 2" >> test.txt
    git add test.txt
    git commit -m "Add feature 2"

    create_test_config true "feature/auto-{timestamp}"

    # Execute: No-edit with dry-run (to verify without side effects)
    output=$("$GH_ARC_BIN" diff --no-edit --dry-run 2>&1)
    exit_code=$?

    # Verify: Success
    assert_equals 0 $exit_code "Should succeed"

    # Verify: Title extracted from commits
    assert_contains "$output" "Title: Add feature 1" "Should use first commit title"

    # Verify: Body combines commits
    assert_contains "$output" "Add feature 1" "Should include first commit"
    assert_contains "$output" "Add feature 2" "Should include second commit"

    # Verify: Test plan has default
    assert_contains "$output" "Test Plan:" "Should have test plan"

    # Verify: No editor mentioned
    assert_not_contains "$output" "Opening editor" "Should not open editor"
    assert_not_contains "$output" "\$EDITOR" "Should not mention editor"

    pass_test "Auto-branch no-edit uses defaults"
}

test_no_edit_with_explicit_values() {
    start_test "No-edit with explicit values"
    reset_test_repo

    git checkout main
    create_test_commits 1
    create_test_config true

    # Execute: Provide explicit values
    output=$("$GH_ARC_BIN" diff --no-edit --dry-run \
        --title "Custom Title" \
        --body "Custom description of changes" \
        --test-plan "Unit tests and manual verification" \
        --reviewers "alice,bob" \
        --draft 2>&1)

    # Verify: Uses provided values
    assert_contains "$output" "Title: Custom Title" "Should use provided title"
    assert_contains "$output" "Custom description" "Should use provided body"
    assert_contains "$output" "Unit tests and manual verification" "Should use provided test plan"
    assert_contains "$output" "Reviewers: alice, bob" "Should use provided reviewers"
    assert_contains "$output" "Draft: true" "Should respect draft flag"

    pass_test "No-edit with explicit values"
}
```

**Test Matrix for No-Edit**:

```bash
# Default behavior
test_no_edit_extracts_from_commits()         # Title/body from git log
test_no_edit_uses_placeholders()             # When commits not descriptive
test_no_edit_suggests_reviewers()            # From CODEOWNERS

# Explicit values
test_no_edit_with_title()                    # --title flag
test_no_edit_with_body()                     # --body flag
test_no_edit_with_test_plan()                # --test-plan flag
test_no_edit_with_reviewers()                # --reviewers flag
test_no_edit_with_draft()                    # --draft flag
test_no_edit_with_all_flags()                # All flags combined

# Validation
test_no_edit_missing_required_uses_defaults() # Fills required fields
test_no_edit_invalid_reviewers()             # Validates reviewer format
```

### 4. Combined Flag Tests

**Purpose**: Test flag interactions and realistic workflows.

**Example Test**:

```bash
test_dry_run_offline_no_edit() {
    start_test "Combined: dry-run + offline + no-edit"
    reset_test_repo

    git checkout main
    create_test_commits 2
    create_test_config true

    # Execute: All flags
    output=$("$GH_ARC_BIN" diff --dry-run --offline --no-edit 2>&1)
    exit_code=$?

    # Verify: Success
    assert_equals 0 $exit_code "Should succeed"

    # Verify: Dry-run takes precedence
    assert_contains "$output" "[DRY RUN]" "Should show dry-run"

    # Verify: No actual operations
    current_branch=$(git branch --show-current)
    assert_equals "main" "$current_branch" "Should still be on main (dry-run)"

    # Verify: Shows offline context
    assert_contains "$output" "offline" "Should mention offline mode"

    # Verify: No editor
    assert_not_contains "$output" "editor" "Should not mention editor"

    pass_test "Combined flags work correctly"
}
```

### 5. Full Workflow Tests (With Manual Verification)

**Purpose**: Test complete workflow against local repository (requires manual PR check).

**Note**: These tests execute real operations against the local test repository but skip GitHub API calls.

**Example Test**:

```bash
test_full_workflow_local_only() {
    start_test "Full workflow (local operations only)"
    reset_test_repo

    # Setup
    git checkout main
    echo "Feature implementation" >> feature.txt
    git add feature.txt
    git commit -m "Implement feature X

Added comprehensive implementation with tests."

    echo "Documentation" >> docs.md
    git add docs.md
    git commit -m "Add documentation"

    create_test_config true "feature/full-workflow-{timestamp}"

    # Execute: Offline mode (full local workflow)
    output=$("$GH_ARC_BIN" diff --offline --no-edit \
        --title "Feature X Implementation" \
        --body "This implements feature X with tests and docs" \
        --test-plan "Unit tests pass, manual testing completed" 2>&1)
    exit_code=$?

    # Verify: Success
    assert_equals 0 $exit_code "Command should succeed"

    # Verify: Output
    log_info "Command output:"
    echo "$output"

    # Verify: Branch created and checked out
    current_branch=$(git branch --show-current)
    assert_contains "$current_branch" "feature/full-workflow-" "Should be on feature branch"

    # Verify: Commits on new branch
    commits=$(git log --oneline origin/main..HEAD)
    assert_contains "$commits" "Implement feature X" "Should have feature commit"
    assert_contains "$commits" "Add documentation" "Should have docs commit"

    # Verify: Main is clean
    git checkout main
    commits_ahead=$(git log --oneline origin/main..HEAD | wc -l)
    assert_equals 0 $commits_ahead "Main should be clean"

    # Manual verification instructions
    log_warning "Manual verification required:"
    log_warning "  1. Check that branch exists: git branch | grep full-workflow"
    log_warning "  2. Check commits are on branch: git log full-workflow-*"
    log_warning "  3. Verify main is clean: git log main"

    pass_test "Full workflow completed successfully"
}
```

### 6. Real GitHub Tests (Optional, Requires Auth)

**Purpose**: Test against real GitHub repository (for comprehensive validation).

**Prerequisites**:
- GitHub personal access token
- Test repository with write access
- Network connection

**Example Test**:

```bash
test_full_workflow_with_real_github() {
    # Skip if GitHub token not available
    if [ -z "$GITHUB_TOKEN" ]; then
        skip_test "Real GitHub test" "GITHUB_TOKEN not set"
        return
    fi

    start_test "Full workflow with real GitHub"
    reset_test_repo

    # Setup: Use real remote URL
    git remote remove origin
    git remote add origin "https://github.com/test-user/test-repo.git"
    git fetch origin
    git checkout main
    git reset --hard origin/main

    # Create commits
    create_test_commits 2
    create_test_config true

    # Execute: Full flow (no dry-run, no offline)
    output=$("$GH_ARC_BIN" diff --no-edit \
        --title "Test PR from E2E suite" \
        --body "Automated test PR, safe to close" \
        --test-plan "Automated E2E test" \
        --draft 2>&1)
    exit_code=$?

    # Store PR number for cleanup
    pr_number=$(echo "$output" | grep -oP 'Pull request #\K\d+' | head -1)

    # Verify: Success
    assert_equals 0 $exit_code "Should create PR successfully"
    assert_not_empty "$pr_number" "Should extract PR number"

    # Verify: PR exists
    gh pr view "$pr_number" >/dev/null 2>&1
    assert_equals 0 $? "PR should exist on GitHub"

    # Verify: Branch pushed
    git ls-remote origin | grep "feature/auto-from-main-"
    assert_equals 0 $? "Branch should be pushed"

    # Cleanup: Close PR and delete branch
    gh pr close "$pr_number" --delete-branch >/dev/null 2>&1

    pass_test "Full workflow with real GitHub"
}
```

## Test Execution

### Run All Tests

```bash
# All tests (uses built binary)
./test-auto-branch-e2e.sh

# Specific category
./test-auto-branch-e2e.sh dry-run
./test-auto-branch-e2e.sh offline
./test-auto-branch-e2e.sh no-edit

# Specific test
./test-auto-branch-e2e.sh test_auto_branch_dry_run_happy_path

# With real GitHub (requires GITHUB_TOKEN)
GITHUB_TOKEN=$GITHUB_TOKEN ./test-auto-branch-e2e.sh github
```

### CI/CD Integration

```yaml
# .github/workflows/e2e-tests.yml
name: E2E Tests

on: [push, pull_request]

jobs:
  e2e-dry-run:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Build binary
        run: go build -o gh-arc

      - name: Run dry-run tests
        run: ./docs/wip/testing-flags/test-auto-branch-e2e.sh dry-run

  e2e-offline:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Build binary
        run: go build -o gh-arc

      - name: Run offline tests
        run: ./docs/wip/testing-flags/test-auto-branch-e2e.sh offline

  e2e-github:
    runs-on: ubuntu-latest
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v3

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Build binary
        run: go build -o gh-arc

      - name: Run GitHub integration tests
        env:
          GITHUB_TOKEN: ${{ secrets.E2E_TEST_TOKEN }}
        run: ./docs/wip/testing-flags/test-auto-branch-e2e.sh github
```

## Benefits of This Approach

### 1. Tests Real Code Paths
- Executes actual binary, not mocked functions
- Tests real git operations
- Tests real command parsing and flag handling
- Catches integration issues

### 2. Fast and Safe
- Dry-run tests run in milliseconds
- No network calls means no flakiness
- No side effects means can run anywhere
- Can run in parallel

### 3. Easy to Debug
- Output shows exactly what happened
- Can inspect repository state after test
- Can manually reproduce any test
- Clear failure messages

### 4. Comprehensive Coverage
- Test all code paths
- Test error scenarios
- Test edge cases
- Test flag combinations

### 5. CI/CD Friendly
- No GitHub credentials required for most tests
- No network required for most tests
- Fast execution
- Deterministic results

## Test Organization

```
docs/wip/testing-flags/
├── design.md                           # Feature design (this doc's companion)
├── e2e-testing.md                      # This document
├── test-auto-branch-e2e.sh             # Main test script
├── tests/
│   ├── dry-run/
│   │   ├── test-happy-paths.sh         # Dry-run happy paths
│   │   ├── test-error-cases.sh         # Dry-run error cases
│   │   └── test-patterns.sh            # Pattern generation tests
│   ├── offline/
│   │   ├── test-local-ops.sh           # Offline local operations
│   │   ├── test-collisions.sh          # Collision handling
│   │   └── test-edge-cases.sh          # Edge cases
│   ├── no-edit/
│   │   ├── test-defaults.sh            # Default value generation
│   │   ├── test-explicit.sh            # Explicit flag values
│   │   └── test-validation.sh          # Input validation
│   ├── combined/
│   │   └── test-flag-combinations.sh   # Multiple flags together
│   └── github/
│       └── test-real-github.sh         # Real GitHub integration (optional)
└── lib/
    ├── assertions.sh                   # Test assertion functions
    ├── setup.sh                        # Test environment setup
    └── utils.sh                        # Helper utilities
```

## Next Steps

1. Implement testing flags (see `design.md`)
2. Create test script structure
3. Write dry-run tests first (fastest feedback)
4. Add offline tests (verify git operations)
5. Add no-edit tests (automation workflows)
6. Add combined flag tests
7. Optionally add real GitHub tests
8. Integrate into CI/CD pipeline

## Summary

With testing flags implemented, we can write comprehensive E2E tests that:
- Execute the actual binary (not mocks)
- Run quickly and safely (dry-run mode)
- Verify real git operations (offline mode)
- Support automation (no-edit mode)
- Work in CI/CD (no GitHub required)
- Catch real integration issues
- Provide fast feedback to developers

This makes `gh-arc` fully testable while also providing valuable features to end users.
