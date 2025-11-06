# Quick Start Guide for Running Tests

## Step 1: Prepare Your Test Repository

Create a separate test repository on GitHub:

```bash
# Option A: Create a new test repo on GitHub
# 1. Go to GitHub and create a new repo (e.g., "gh-arc-test")
# 2. Clone it locally:
cd /tmp
git clone https://github.com/YOUR-USERNAME/gh-arc-test.git
cd gh-arc-test

# Option B: Use an existing repo
cd /path/to/your/existing/test-repo
```

## Step 2: Build gh-arc

```bash
cd /path/to/gh-arc
go build -o gh-arc
```

## Step 3: Run Tests

From anywhere, point the test script to your test repo:

```bash
# Run all tests
TEST_DIR=/tmp/gh-arc-test /path/to/gh-arc/docs/wip/auto-branch-from-main/test-auto-branch.sh

# Or run a specific test
TEST_DIR=/tmp/gh-arc-test /path/to/gh-arc/docs/wip/auto-branch-from-main/test-auto-branch.sh test_stacking_same_commit_scenario
```

## What Happens

1. **Script detects existing repo** - Sees `.git` directory, enters safe mode
2. **Checks prerequisites** - Verifies `git` and `go` are installed
3. **Builds gh-arc** - If not already built
4. **Runs tests** - Executes Go unit tests and shell integration tests
5. **Preserves your repo** - Never deletes or aggressively cleans existing repos

## Expected Output

```
[INFO] Auto-Branch from Main - E2E Test Suite
[INFO] =======================================
[INFO] Setting up test environment...
[INFO] Using existing repository at /tmp/gh-arc-test
[INFO] Test directory will be preserved after tests complete

======================================================================
TEST: Detection on main with commits
======================================================================
[PASS] Detection on main with commits

...

======================================================================
TEST SUMMARY
======================================================================
Total tests:  41
Passed:       40
Failed:       0
Skipped:      1
======================================================================
[ALL TESTS PASSED]

[INFO] Preserving existing repository: /tmp/gh-arc-test
```

## Safety Features

✅ **Existing repos are NEVER deleted** - Even with `CLEANUP=1`
✅ **Safe mode for existing repos** - Only stashes uncommitted changes
✅ **No aggressive cleanup** - Doesn't delete branches or reset hard
✅ **Preserves `.arc.json`** - Removed at end but your repo stays intact

## Troubleshooting

### "Not a valid git repository"
```bash
cd /tmp/gh-arc-test
git init
git remote add origin https://github.com/YOUR-USERNAME/gh-arc-test.git
```

### "Does not have an 'origin' remote"
```bash
cd /tmp/gh-arc-test
git remote add origin https://github.com/YOUR-USERNAME/gh-arc-test.git
```

### "Failed to build gh-arc"
```bash
cd /path/to/gh-arc
go mod download
go build -o gh-arc
```

## Running Individual Test Categories

```bash
# Template handling tests
TEST_DIR=/tmp/gh-arc-test /path/to/gh-arc/docs/wip/auto-branch-from-main/test-auto-branch.sh test_template_extract_base_stacked

# Stacking tests
TEST_DIR=/tmp/gh-arc-test /path/to/gh-arc/docs/wip/auto-branch-from-main/test-auto-branch.sh test_stacking_same_commit_scenario

# Continue mode tests
TEST_DIR=/tmp/gh-arc-test /path/to/gh-arc/docs/wip/auto-branch-from-main/test-auto-branch.sh test_continue_mode_stacked_template
```

## After Tests Complete

Your test repository is preserved exactly as it was. You can:
- Review any stashed changes: `git stash list`
- Check test artifacts: `ls -la /tmp/gh-arc-test/.arc.json`
- Clean up manually: `rm -rf /tmp/gh-arc-test` (when you're done)
