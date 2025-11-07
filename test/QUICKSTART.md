# Quick Start Guide for E2E Tests

Get up and running with `gh arc diff` E2E tests in 4 steps.

## Step 0: Install Prerequisites (macOS only)

If you're on macOS, install GNU sed:

```bash
brew install gnu-sed
```

Linux users can skip this step (GNU sed is included by default).

## Step 1: Clone Test Repository

Clone a test repository with GitHub remote:

```bash
# Option 1: Clone the official test repo (if you have access)
git clone git@github.com:0xBAD-dev/gh-arc-test.git /tmp/gh-arc-test

# Option 2: Create your own test repo on GitHub, then clone it
git clone https://github.com/YOUR-USERNAME/your-test-repo.git /tmp/gh-arc-test
```

## Step 2: Authenticate GitHub CLI

Ensure you have the required scopes for E2E testing:

```bash
# Check current authentication
gh auth status

# Refresh with required scopes
gh auth refresh -s read:user

# Or re-login if needed
gh auth login
```

**Required scopes:**
- `repo` - For PR creation and updates
- `read:user` or `user:email` - For reviewer filtering

## Step 3: Build gh-arc

```bash
cd /path/to/gh-arc
go build -o gh-arc
```

## Step 4: Run Tests

From the project root:

```bash
# Run all 22 tests
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh

# Run specific test
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh test_e2e_base_flag_override_stacking

# Debug mode (verbose output)
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --debug

# Keep artifacts for inspection
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --no-cleanup
```

## What Happens

1. **Validates Prerequisites**
   - Checks test repository exists and is initialized
   - Verifies gh CLI authentication (`gh auth status`)
   - Confirms gh-arc binary is built

2. **Runs Test Suite**
   - 22 E2E tests organized into 10 categories
   - Creates real PRs on GitHub
   - Tests all flags and workflows

3. **Automatic Cleanup**
   - Closes created PRs
   - Deletes remote and local branches
   - Removes saved templates from /tmp
   - Resets repository to clean state

## Expected Output

```
[INFO] gh-arc End-to-End Test Suite
[INFO] ============================
[INFO] Test repository: /Users/you/Projects/gh-arc/test/gh-arc-test
[INFO] gh-arc binary: arc

[INFO] gh CLI: authenticated ✓
[INFO] Running all E2E tests...

[INFO] Category: Fast Path

======================================================================
TEST 1/22: E2E: Fast path - push new commits to existing PR
======================================================================
[STEP] Created branch: test-fast-path-commits-1734567890
[STEP] Created commit: Initial commit
[STEP] Initial PR #123 created
[STEP] Created commit: Second commit
[STEP] Created commit: Third commit
[STEP] Running arc diff (fast path - no --edit)...
[STEP] Fast path executed successfully
[STEP] All commits pushed to PR ✓ (3 total)
[PASS] E2E: Fast path - push new commits

...

[INFO] Category: --base Flag

======================================================================
TEST 4/22: E2E: --base flag overrides stacking detection
======================================================================
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

## Common Options

```bash
# Run all tests
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh

# Run specific test
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh test_e2e_stacking_basic

# Debug mode (shows -vvv output from gh arc commands)
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --debug

# Keep test artifacts (PRs, branches)
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --no-cleanup

# Cleanup only (no tests)
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --cleanup-only

# Show help
./test/test-e2e.sh --help

# Use a different test repository
TEST_DIR=/path/to/your/repo ./test/test-e2e.sh
```

## What Gets Tested

### All Flags
- ✅ `--draft` / `--ready` (draft status management)
- ✅ `--edit` (force template regeneration)
- ✅ `--no-edit` (skip editor)
- ✅ `--continue` (retry after validation failure)
- ✅ `--base` (override base branch detection)

### All Workflows
- ✅ Fast path (existing PR, simple updates)
- ✅ Normal mode (new PR with template)
- ✅ Continue mode (validation retry)
- ✅ PR stacking (feature → feature)
- ✅ Auto-branch (commits on main)
- ✅ Draft transitions
- ✅ Reviewer assignment

### Edge Cases
- ✅ Invalid base branch
- ✅ Editor cancellation
- ✅ Multiple validation failures
- ✅ Template preservation
- ✅ Current user filtering
- ✅ Same-commit stacking

## Test Independence

Each test is fully independent:
- Uses unique timestamped branch names
- Creates its own PRs and branches
- Cleans up after itself
- Can run alone or with others

Example of running tests one at a time:
```bash
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh test_e2e_fast_path_push_commits
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh test_e2e_base_flag_override_stacking
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh test_e2e_no_edit_flag_new_pr
```

## Troubleshooting

### TEST_DIR not set
```
ERROR: TEST_DIR environment variable is required
```

**Solution:** Clone a test repository and provide the path:
```bash
git clone git@github.com:0xBAD-dev/gh-arc-test.git /tmp/gh-arc-test
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh
```

### Missing GitHub Token Scopes

If tests fail with authentication errors or warnings about missing scopes:

```bash
# Refresh with required scope
gh auth refresh -s read:user

# Verify scopes
gh auth status
```

### gh CLI not authenticated
```bash
# Check status
gh auth status

# If not authenticated:
gh auth login
# Follow prompts
```

### Binary not found
```bash
cd /path/to/gh-arc
go build -o gh-arc

# Verify
./gh-arc --version
```

### Tests fail to create PRs

**Check permissions:**
```bash
# Verify you can create PRs in test repo
cd test/gh-arc-test
gh pr list
```

**Check rate limits:**
```bash
gh api rate_limit
```

### Tests hang on editor

Tests use custom editor scripts that auto-complete templates. If hanging:
1. Use `--debug` to see what's happening
2. Check that editor scripts have execute permissions
3. Verify `EDITOR` environment variable isn't interfering

### Cleanup issues

If cleanup fails and PRs/branches remain:

```bash
# Manual cleanup
cd /tmp/gh-arc-test

# List remaining PRs
gh pr list

# Close specific PR and delete branch
gh pr close 123 --delete-branch

# Or use cleanup-only mode
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --cleanup-only
```

## Performance

- **Full suite:** ~2-4 minutes (22 tests)
- **Single test:** ~5-10 seconds
- **Depends on:** GitHub API latency, network speed

## Safety Features

✅ **Unique branch names** - Timestamped, no conflicts
✅ **Automatic cleanup** - PRs and branches removed
✅ **Idempotent** - Can rerun without side effects
✅ **Preserves existing work** - Only touches test branches
✅ **Exit trap** - Cleanup runs even on script failure

## Advanced Usage

### Keep artifacts for debugging
```bash
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --no-cleanup

# Inspect created PRs
cd /tmp/gh-arc-test
gh pr list

# View PR details
gh pr view 123

# Check branches
git branch | grep test-
```

### Debug specific test
```bash
# Run with debug output
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --debug test_e2e_base_flag_override_stacking

# Keep artifacts
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh --no-cleanup --debug test_e2e_continue_validation_failure

# Check saved templates
ls -la /tmp/gh-arc-saved-*.md
```

### Run tests in parallel (NOT RECOMMENDED)

Tests are NOT designed for parallel execution:
- Git operations are not thread-safe
- Tests share the same test directory
- Cleanup logic assumes sequential execution

Run tests sequentially only.

## Integration with CI/CD

Tests are CI-ready:

```yaml
# Example GitHub Actions
- name: Run E2E Tests
  run: |
    git clone git@github.com:0xBAD-dev/gh-arc-test.git /tmp/gh-arc-test
    go build -o gh-arc
    TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh
  env:
    GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Features for CI:
- ✅ Exit code 0 on success, 1 on failure
- ✅ Automatic cleanup (no manual steps)
- ✅ Colored output for readability
- ✅ Structured logging (INFO, PASS, FAIL, STEP)

## Next Steps

- Read [E2E_TESTS.md](./E2E_TESTS.md) for detailed test descriptions
- Read [TEST_COVERAGE.md](./TEST_COVERAGE.md) for coverage analysis
- Contribute by adding new tests for uncovered scenarios

## Need Help?

```bash
./test/test-e2e.sh --help
```

Or check the documentation:
- [E2E_TESTS.md](./E2E_TESTS.md) - Comprehensive test documentation
- [TEST_COVERAGE.md](./TEST_COVERAGE.md) - Coverage breakdown
- [testing.md](./testing.md) - General testing guide
