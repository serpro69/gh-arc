# Test Coverage for Bug Fixes

This document describes the test coverage for recent bug fixes in `gh arc diff` and related functionality.

## Bug Fixes Covered

### 1. Continue Mode Restructuring
**Issue:** Continue mode was re-running analysis (commit analysis, auto-branch detection) instead of just reusing the saved template.

**Fix:** Restructured continue mode to skip ALL analysis and return early after PR creation.

**Tests:**
- `test_continue_mode_preserves_edits` - Verifies edits are preserved across multiple `--continue` attempts
- `test_continue_mode_saved_template` - Shell test that verifies saved template can be loaded
- All continue mode tests validate that analysis is skipped

### 2. Template Sorting by Modification Time
**Issue:** When multiple saved templates existed, `FindSavedTemplates()` returned them alphabetically (random filename order) instead of by modification time, causing wrong template to be loaded.

**Fix:** Modified `FindSavedTemplates()` to sort by `ModTime` (newest first).

**Tests:**
- `test_template_sorting_modtime` - Unit test for sorting behavior
- `test_continue_mode_newest_template` - Shell integration test with multiple templates

### 3. Save Template on Validation Failure
**Issue:** When validation failed during `--continue`, edits were lost because template wasn't saved.

**Fix:** Save edited template before returning validation error.

**Tests:**
- `test_continue_mode_preserves_edits` - Tests multi-iteration validation failures
- `test_continue_mode_validation_failure` - Shell test for edit preservation

### 4. Stacking Detection for Same-Commit Scenario
**Issue:** Stacking detection failed when local main and parent branch were at same commit (auto-branch scenario).

**Scenario:**
```
main (local): abc123 (1 ahead of origin/main)
feature-auto: abc123 (auto-created from main, has open PR)
feature-child: def456 (branched from feature-auto)

Expected: feature-child stacks on feature-auto
Actual: feature-child targets main (WRONG!)
```

**Fix:** Added second condition in stacking detection: when merge-bases are equal, check if candidate branch HEAD is at merge-base (meaning current was branched from candidate).

**Tests:**
- `test_stacking_same_commit_scenario` - Main test for this bug fix
- `test_stacking_different_mergebase` - Existing behavior still works
- `test_stacking_disabled` - Config flag works
- `test_stacking_no_opportunity` - Correctly detects no stacking

### 5. Continue Mode for Stacked PRs
**Issue:** Continue mode failed for stacked PRs because template header format is different and doesn't include head branch.

**Template Formats:**
- Non-stacked: `# Creating PR: <head> → <base>`
- Stacked: `# 📚 Creating stacked PR on <base> (PR #123: ...)`

**Fix:**
- Use current git branch as head (always available)
- Extract only base branch from template with new `ExtractBaseBranch()` function
- Handle both stacked and non-stacked formats

**Tests:**
- `test_template_extract_base_nonstacked` - Non-stacked format
- `test_template_extract_base_stacked` - Stacked format
- `test_template_extract_base_fallback` - Fallback to `# Base Branch:` marker
- `test_template_extract_base_real` - Real template content
- `test_continue_mode_stacked_template` - Shell integration test

### 6. Push Branch Before Creating PR
**Issue:** Continue mode tried to create PR without pushing branch to remote first, causing GitHub API error: "PullRequest.head is invalid"

**Fix:** Added push logic in continue mode before PR creation.

**Tests:**
- Verified by all Go unit tests passing
- Integration testing requires actual git push operations

## Running the Tests

### Option 1: Run with automatic temp directory (default):
```bash
cd docs/wip/auto-branch-from-main
./test-auto-branch.sh
# Creates /tmp/gh-arc-test-$$, runs tests, auto-cleans up
```

### Option 2: Run with existing repository (SAFE):
```bash
# Point to your existing test repository
TEST_DIR=/path/to/your/test-repo ./test-auto-branch.sh

# The script will:
# - Detect it's an existing repo
# - Use safe mode (no aggressive cleanup)
# - Preserve the repo after tests
# - Never delete it (even with CLEANUP=1)
```

### Option 3: Run specific test:
```bash
./test-auto-branch.sh test_template_extract_base_stacked
```

### Option 4: Create new test repo (preserved):
```bash
TEST_DIR=/tmp/my-arc-test ./test-auto-branch.sh
# Creates new repo if /tmp/my-arc-test doesn't exist
# Preserves it after tests complete
```

### Option 5: Force cleanup of created repos:
```bash
CLEANUP=1 TEST_DIR=/tmp/my-arc-test ./test-auto-branch.sh
# Only works for created repos, NOT existing repos
```

## Test Categories

### Template Handling Tests (8 tests)
Tests for `ExtractBaseBranch()`, template sorting, validation, and parsing.

### Stacking Tests (4 tests)
Tests for stacking detection logic including the same-commit scenario bug fix.

### Continue Mode Integration Tests (4 tests)
Shell-based integration tests that verify actual file operations for continue mode.

### Git Operation Tests (5 tests)
Tests for git operations like `CountCommitsAhead`, `BranchExists`, `GetCommitsBetween`, etc.

### Integration Tests (5 tests)
Full workflow integration tests including auto-branch detection and collision handling.

## Total Test Coverage

- **Total Tests:** 41+ tests
- **New Tests for Bug Fixes:** 16 tests
- **Shell Integration Tests:** 4 tests
- **Go Unit Tests:** 37+ tests

## Test Results Format

Tests output colored results:
- 🟢 **[PASS]** - Test passed
- 🔴 **[FAIL]** - Test failed
- 🟡 **[SKIP]** - Test skipped (e.g., requires user interaction)
- 🔵 **[INFO]** - Informational message

## Exit Codes

- `0` - All tests passed
- `1` - One or more tests failed
- `2` - Setup error (missing prerequisites, build failure, etc.)

## Prerequisites

- `git` - Version control
- `go` - Go compiler (1.23.4+)
- `gh-arc` - Built from source (automatically built by test script)

## Notes

- Tests create temporary directories in `/tmp` by default
- Custom test directories are preserved unless `CLEANUP=1` is set
- Some tests require user interaction and are automatically skipped
- Tests run against a local bare git repository to avoid needing GitHub access
