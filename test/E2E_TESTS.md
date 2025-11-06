# End-to-End Test Suite

This test suite runs the **actual** `gh arc` binary and creates **real** PRs on GitHub to verify all bug fixes.

## What Gets Tested

### ✅ Test 1: Continue Mode - Validation Failure Preserves Edits
**Bug Fix:** Edits are preserved across multiple `--continue` attempts when validation fails

**Test Flow:**
1. Create branch with commit
2. Run `arc diff` with incomplete template (no test plan) → validation fails
3. Run `arc diff --continue` with extra content but still no test plan → validation fails
4. Verify extra content is preserved in saved template
5. Run `arc diff --continue` with complete template → PR created
6. **Verify:** PR exists and was created successfully

### ✅ Test 2: Continue Mode with Stacked PR
**Bug Fix:** Continue mode works with stacked PR template format (no head branch in template)

**Test Flow:**
1. Create parent branch and PR
2. Create child branch from parent
3. Run `arc diff` with incomplete template → validation fails
4. Verify saved template has stacked format
5. Run `arc diff --continue` with complete template → stacked PR created
6. **Verify:** Child PR targets parent branch (not main)

### ✅ Test 3: Stacking Detection - Same Commit Scenario
**Bug Fix:** Stacking detection works when parent and main are at same commit (auto-branch case)

**Test Flow:**
1. Create commit on main (simulating unpushed commits)
2. Run `arc diff` → auto-branch created with PR
3. Create child branch from auto-branch
4. Run `arc diff` → child PR created
5. **Verify:** Child PR targets auto-branch (not main)

### ✅ Test 4: Template Sorting by Modification Time
**Bug Fix:** Newest template is loaded when multiple saved templates exist

**Test Flow:**
1. Create branch with commit
2. Create multiple saved templates with different timestamps
3. Run `arc diff --continue` → should load newest template
4. **Verify:** PR body contains content from newest template

### ✅ Test 5: Auto-Branch Creation from Main
**Feature:** Auto-creates branch when commits are on main

**Test Flow:**
1. Ensure on main branch
2. Create commits on main
3. Run `arc diff` → should auto-create branch
4. **Verify:**
   - Now on new branch (not main)
   - PR exists and targets main

## Prerequisites

### 1. Test Repository
You need an **existing GitHub repository** (public or private) for testing:

```bash
# Create a new test repo on GitHub, then:
cd /tmp
git clone https://github.com/YOUR-USERNAME/gh-arc-test.git
cd gh-arc-test
```

### 2. GitHub CLI Authentication
```bash
# Check authentication
gh auth status

# If not authenticated:
gh auth login
```

### 3. Build gh-arc
```bash
cd /path/to/gh-arc
go build -o gh-arc
```

## Running the Tests

### Run All Tests
```bash
cd /path/to/gh-arc
TEST_DIR=/tmp/gh-arc-test ./docs/wip/auto-branch-from-main/test-e2e.sh
```

### Run Specific Test
```bash
TEST_DIR=/tmp/gh-arc-test ./docs/wip/auto-branch-from-main/test-e2e.sh test_e2e_continue_validation_failure
```

### Available Tests
- `test_e2e_continue_validation_failure`
- `test_e2e_continue_stacked_pr`
- `test_e2e_stacking_same_commit`
- `test_e2e_template_sorting`
- `test_e2e_auto_branch_creation`

## Expected Output

```
[INFO] gh-arc End-to-End Test Suite
[INFO] ============================
[INFO] Test repository: /tmp/gh-arc-test
[INFO] gh-arc binary: /path/to/gh-arc/gh-arc

[INFO] gh CLI: authenticated ✓
[INFO] Running all E2E tests...

======================================================================
TEST: E2E: Continue mode preserves edits on validation failure
======================================================================
[STEP] Created branch: test-continue-validation-1234567890
[STEP] Created commit: Test commit for continue mode
[STEP] Running arc diff with incomplete template (expect failure)...
[STEP] Validation failed as expected
[STEP] Running arc diff --continue with extra content (expect failure)...
[STEP] Second validation failed as expected
[STEP] Extra content preserved in saved template ✓
[STEP] Running arc diff --continue with complete template...
[STEP] PR created successfully
[STEP] PR #123 exists for branch test-continue-validation-1234567890
[PASS] E2E: Continue mode preserves edits on validation failure

...

======================================================================
TEST SUMMARY
======================================================================
Total tests:  5
Passed:       5
Failed:       0
======================================================================
[ALL TESTS PASSED]

[WARN] Manual cleanup required:
[INFO]   1. Review PRs: gh pr list
[INFO]   2. Close PRs: gh pr close <number>
[INFO]   3. Delete branches: git branch -D <branch>
```

## What Gets Created

Each test creates **real** PRs on GitHub:
- Multiple branches (unique timestamped names)
- Multiple PRs targeting main or other feature branches
- Saved template files in `/tmp` (automatically cleaned up)

## Manual Cleanup

After tests complete, you need to manually clean up:

### 1. View Created PRs
```bash
cd /tmp/gh-arc-test
gh pr list
```

### 2. Close PRs
```bash
# Close specific PR
gh pr close 123

# Or close all test PRs (careful!)
gh pr list --json number --jq '.[].number' | xargs -I {} gh pr close {}
```

### 3. Delete Local Branches
```bash
# List branches
git branch

# Delete specific branch
git branch -D test-continue-validation-1234567890

# Or delete all test-* branches
git branch | grep 'test-' | xargs git branch -D
```

### 4. Delete Remote Branches
```bash
# Delete specific remote branch
git push origin --delete test-continue-validation-1234567890

# Or delete all test-* remote branches
git branch -r | grep 'origin/test-' | sed 's/origin\///' | xargs -I {} git push origin --delete {}
```

### 5. Delete Test Repository (when completely done)
```bash
# Remove local clone
rm -rf /tmp/gh-arc-test

# Delete on GitHub (via web UI or):
gh repo delete YOUR-USERNAME/gh-arc-test
```

## Troubleshooting

### "gh CLI is not authenticated"
```bash
gh auth login
# Follow prompts to authenticate
```

### "TEST_DIR environment variable is required"
```bash
TEST_DIR=/tmp/gh-arc-test ./test-e2e.sh
```

### "TEST_DIR must be a git repository"
```bash
cd /tmp/gh-arc-test
git status  # Should show it's a git repo
```

### "gh-arc binary not found"
```bash
cd /path/to/gh-arc
go build -o gh-arc
```

### "validation failed" but test expects success
Check the custom editor script modifications. The test might need adjustment for your specific setup.

### Tests hang waiting for editor
The tests use a custom `EDITOR` script that auto-completes templates. If it hangs, check that the script has execute permissions.

## How the Tests Work

### Custom Editor Script
Tests set a custom `EDITOR` that programmatically modifies templates:
- `remove_test_plan` - Removes test plan to trigger validation failure
- `add_test_plan` - Adds test plan to pass validation
- `add_extra_content` - Adds content to verify preservation
- `complete_template` - Fills all required fields

### Verification Methods
- `gh pr list --head <branch>` - Check if PR exists
- `gh pr view <number> --json baseRefName` - Verify PR targets correct base
- `gh pr view <number> --json body` - Check PR body content
- File system checks for saved templates

### Unique Branch Names
Each test uses timestamped branch names to avoid conflicts:
```bash
test-continue-validation-1234567890
test-stacked-parent-1234567890
test-stacked-child-1234567891
```

## Re-Running Tests

Tests can be re-run multiple times. Each run creates new branches and PRs with unique timestamps.

## Notes

- ⚠️ Tests create REAL PRs - GitHub rate limits apply
- ⚠️ Tests modify your test repository (creates branches/commits)
- ✅ Tests preserve existing branches (uses unique names)
- ✅ Your test repo is not deleted (manual cleanup)
- ⏱️ Tests take 1-2 minutes to complete (depends on GitHub API)

## Success Criteria

All tests pass when:
- ✅ PRs are created successfully
- ✅ Stacked PRs target the correct base branch
- ✅ Continue mode preserves edits across validation failures
- ✅ Newest template is loaded when multiple exist
- ✅ Auto-branch creation works from main
