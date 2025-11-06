# End-to-End Test Suite

This test suite runs the **actual** `gh arc` binary and creates **REAL** PRs on GitHub to verify all features of the diff command.

## Quick Start

```bash
# Step 1: Clone the test repository
git clone git@github.com:0xBAD-dev/gh-arc-test.git /tmp/gh-arc-test

# Step 2: Run tests
cd /path/to/gh-arc
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh

# Run specific test
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh test_e2e_base_flag_override_stacking

# Debug mode
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --debug

# Keep test artifacts for inspection
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --no-cleanup
```

## Test Categories

### Fast Path Tests (2 tests)
Tests for fast path execution when existing PR needs only simple updates.

**test_e2e_fast_path_push_commits**
- Create PR, add commits, run `arc diff` (no editor)
- Verifies: Commits pushed without opening editor

**test_e2e_fast_path_draft_ready**
- Create draft PR, mark as ready with `--ready` flag
- Verifies: Draft → ready transition works via fast path

### Normal Mode Tests (2 tests)
Tests for standard PR creation and update workflows.

**test_e2e_normal_mode_new_pr**
- Create branch, run `arc diff` with template editing
- Verifies: New PR created with template workflow

**test_e2e_normal_mode_update_with_edit**
- Create PR, add commits, run `arc diff --edit`
- Verifies: PR updated with forced template regeneration

### --base Flag Tests (3 tests)
Tests for `--base` flag to override base branch detection.

**test_e2e_base_flag_override_stacking**
- Create parent PR (feature → main)
- Create child from parent, run `arc diff --base main`
- Verifies: Child PR targets main instead of parent (stacking override)

**test_e2e_base_flag_force_stacking**
- Create target PR, create separate feature branch
- Run `arc diff --base target-branch`
- Verifies: PR targets specified base branch

**test_e2e_base_flag_invalid_branch**
- Run `arc diff --base nonexistent-branch`
- Verifies: Error message shown, no PR created

### --no-edit Flag Tests (2 tests)
Tests for `--no-edit` flag to skip editor.

**test_e2e_no_edit_flag_new_pr**
- Create branch, run `arc diff --no-edit`
- Verifies: PR created without opening editor (auto-generated content)

**test_e2e_no_edit_flag_update_pr**
- Create PR, add commits, run `arc diff --no-edit`
- Verifies: PR updated without opening editor

### Draft PR Tests (1 test)
Tests for draft PR behavior.

**test_e2e_draft_with_fast_path_commits**
- Create draft PR, add commits, run `arc diff`
- Verifies: Commits pushed, PR remains draft (fast path maintains status)

### Flag Combination Tests (3 tests)
Tests for multiple flags used together.

**test_e2e_flag_combination_edit_draft**
- Run `arc diff --edit --draft`
- Verifies: Draft PR created with template editing

**test_e2e_flag_combination_continue_draft**
- Trigger validation failure, run `arc diff --continue --draft`
- Verifies: Draft PR created via continue mode

**test_e2e_flag_combination_base_with_edit**
- Create stacked scenario, run `arc diff --base main --edit`
- Verifies: Base override works with template editing

### Reviewer Tests (2 tests)
Tests for reviewer assignment.

**test_e2e_reviewers_assignment**
- Create PR with reviewers in template
- Verifies: Reviewer assignment workflow works

**test_e2e_reviewers_filters_current_user**
- Add current user as reviewer
- Verifies: Current user filtered from reviewer list

### Stacking Tests (2 tests)
Tests for PR stacking detection and creation.

**test_e2e_stacking_basic**
- Create parent PR (feature → main)
- Create child from parent, run `arc diff`
- Verifies: Child PR targets parent branch

**test_e2e_stacking_same_commit**
- Create commit on main, run `arc diff` (auto-branch)
- Create child from auto-branch
- Verifies: Child PR targets auto-branch (not main)

### Continue Mode Tests (2 tests)
Tests for `--continue` flag after validation failures.

**test_e2e_continue_validation_failure**
- Trigger validation failure, add extra content, fail again
- Run `arc diff --continue` with complete template
- Verifies: Extra content preserved across multiple continue attempts

**test_e2e_continue_stacked_pr**
- Create parent PR, create child, trigger validation failure
- Run `arc diff --continue`
- Verifies: Stacked PR created, targets correct parent

### Auto-Branch Tests (1 test)
Tests for auto-branch creation from main.

**test_e2e_auto_branch_creation**
- Create commits on main, run `arc diff`
- Verifies: Auto-branch created, PR targets main

### Template Tests (1 test)
Tests for template system behavior.

**test_e2e_template_sorting**
- Create multiple saved templates
- Run `arc diff --continue`
- Verifies: Newest template loaded by modification time

### Error Handling Tests (1 test)
Tests for error scenarios.

**test_e2e_error_editor_cancelled**
- Run `arc diff` with editor that exits with error
- Verifies: Cancellation handled gracefully, no PR created

## Total Test Count: 22 tests

## Prerequisites

### 1. Test Repository

Clone a test repository with GitHub remote (or create your own):

```bash
# Option 1: Clone the official test repo (if you have access)
git clone git@github.com:0xBAD-dev/gh-arc-test.git /tmp/gh-arc-test

# Option 2: Create your own test repo on GitHub, then:
git clone https://github.com/YOUR-USERNAME/your-test-repo.git /tmp/gh-arc-test
```

### 2. GitHub CLI Authentication

Ensure you have the required scopes:

```bash
# Check authentication and scopes
gh auth status

# If not authenticated:
gh auth login

# If missing 'read:user' scope, refresh with:
gh auth refresh -s read:user
```

**Required scopes:**
- `repo` - For PR creation/updates
- `read:user` or `user:email` - For reviewer filtering

### 3. Build gh-arc
```bash
cd /path/to/gh-arc
go build -o gh-arc
```

## Running Tests

### All Tests
```bash
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh
```

### Custom Repository
```bash
TEST_DIR=/path/to/your/repo ./test/test-e2e.sh
```

### Specific Test
```bash
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh test_e2e_base_flag_override_stacking
```

### Debug Mode
```bash
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --debug
```

### Keep Artifacts (No Cleanup)
```bash
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --no-cleanup
```

### Cleanup Only (No Tests)
```bash
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --cleanup-only
```

## Expected Output

```
[INFO] gh-arc End-to-End Test Suite
[INFO] ============================
[INFO] Test repository: /Users/sergio/Projects/personal/gh-arc/test/gh-arc-test
[INFO] gh-arc binary: arc

[INFO] gh CLI: authenticated ✓
[INFO] Running all E2E tests...

[INFO] Category: Fast Path

======================================================================
TEST 1/22: E2E: Fast path - push new commits to existing PR
======================================================================
[STEP] Created branch: test-fast-path-commits-1234567890
[STEP] Created commit: Initial commit
[STEP] Initial PR #123 created
[STEP] Created commit: Second commit
[STEP] Created commit: Third commit
[STEP] Running arc diff (fast path - no --edit)...
[STEP] Fast path executed successfully
[STEP] All commits pushed to PR ✓ (3 total)
[PASS] E2E: Fast path - push new commits

...

======================================================================
TEST SUMMARY
======================================================================
Total tests:  22
Passed:       22
Failed:       0
======================================================================
[ALL TESTS PASSED]

[INFO] Cleaning up test artifacts...
[SUCCESS] Cleanup complete
```

## What Gets Created

Each test creates **real** GitHub artifacts:
- Branches with unique timestamped names (e.g., `test-base-parent-1234567890`)
- PRs targeting main or other feature branches
- Commits with test content
- Saved template files in `/tmp` (auto-cleaned)

## Automatic Cleanup

Tests automatically clean up after themselves:
- Close created PRs
- Delete remote branches
- Delete local branches
- Reset repository to clean state
- Remove saved templates from `/tmp`

Use `--no-cleanup` to keep artifacts for debugging.

## Manual Cleanup (if --no-cleanup used)

### View Created Artifacts
```bash
cd test/gh-arc-test

# List PRs
gh pr list

# List branches
git branch | grep test-
```

### Close PRs and Delete Branches
```bash
# Close specific PR and delete its branch
gh pr close 123 --delete-branch

# Or use the cleanup-only mode
./test/test-e2e.sh --cleanup-only
```

### Clean Saved Templates
```bash
# Remove templates created today
find /tmp -name "gh-arc-saved-*.md" -type f -mtime -1 -delete
```

## Test Independence and Idempotency

### Independent Execution
Each test:
- Uses unique timestamped branch names
- Has its own cleanup logic
- Can run independently without dependencies

### Idempotent Design
Tests can be run multiple times:
- Each run creates new branches/PRs with unique names
- Cleanup resets repository to clean state
- No shared state between test runs

## Troubleshooting

### TEST_DIR Not Set
```
ERROR: TEST_DIR environment variable is required
```

**Solution:** Clone a test repository and set TEST_DIR:
```bash
git clone git@github.com:0xBAD-dev/gh-arc-test.git /tmp/gh-arc-test
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh
```

### Missing GitHub Token Scopes

If you see warnings about missing scopes or reviewer filtering:

```bash
# Refresh authentication with required scope
gh auth refresh -s read:user
```

### gh CLI Not Authenticated
```bash
gh auth login
# Follow prompts to authenticate
```

### Binary Not Found
```bash
cd /path/to/gh-arc
go build -o gh-arc
```

### Tests Hanging
- Check that custom editor scripts have execute permissions
- Use `--debug` mode for verbose output
- Check GitHub API rate limits

### Validation Failures
- Verify template modifications in debug output
- Check that test plan is added correctly
- Use `--no-cleanup` to inspect saved templates

## How Tests Work

### Custom Editor Scripts
Tests use programmatic editor scripts to modify templates:
- `complete_template` - Fills all required fields
- `remove_test_plan` - Triggers validation failure
- `add_test_plan` - Fixes validation
- `add_extra_content` - Tests content preservation

### Verification Methods
- `gh pr list --head <branch>` - PR existence
- `gh pr view <number> --json baseRefName` - Base branch
- `gh pr view <number> --json isDraft` - Draft status
- `gh pr view <number> --json commits` - Commit count
- `gh pr view <number> --json body` - PR body content
- File system checks for saved templates

### Unique Identifiers
Branch names use Unix timestamps for uniqueness:
```
test-base-parent-1734567890
test-no-edit-new-1734567891
```

## Continuous Integration

Tests are designed for CI environments:
- Exit code 0 on success, 1 on failure
- All output to stdout/stderr
- JSON output available (via gh CLI)
- Automatic cleanup on EXIT trap

## Coverage Summary

**Flags Tested:**
- ✅ `--draft` / `--ready` (draft status)
- ✅ `--edit` (force template regeneration)
- ✅ `--no-edit` (skip editor)
- ✅ `--continue` (retry after validation)
- ✅ `--base` (override base branch)

**Workflows Tested:**
- ✅ Fast path (existing PR, no editor)
- ✅ Normal mode (new PR, template editing)
- ✅ Continue mode (validation retry)
- ✅ PR stacking (feature → feature)
- ✅ Auto-branch (commits on main)
- ✅ Draft transitions (draft ↔ ready)
- ✅ Reviewer assignment

**Edge Cases Tested:**
- ✅ Invalid base branch
- ✅ Editor cancellation
- ✅ Multiple validation failures
- ✅ Template preservation
- ✅ Current user filtering
- ✅ Same-commit stacking

## Performance

Expected run time for full suite:
- **22 tests** × ~5-10 seconds each
- **Total: ~2-4 minutes** (depends on GitHub API latency)

Individual tests: 5-10 seconds each

## Notes

- ⚠️ Tests create REAL PRs - GitHub rate limits apply
- ⚠️ Tests modify test repository (creates branches/commits)
- ✅ Tests preserve existing branches (uses unique names)
- ✅ Automatic cleanup on success or failure
- ⏱️ Full suite takes 2-4 minutes

## Success Criteria

All tests pass when:
- ✅ All PRs created successfully
- ✅ Stacked PRs target correct base branches
- ✅ Draft transitions work correctly
- ✅ Base override works with `--base` flag
- ✅ Skip editor works with `--no-edit`
- ✅ Continue mode preserves edits
- ✅ Template system loads newest template
- ✅ Auto-branch creates branches from main
- ✅ Error handling shows appropriate messages
- ✅ Reviewer assignment and filtering works
