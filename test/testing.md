# Auto-Branch from Main - End-to-End Testing Guide

This document describes comprehensive end-to-end tests for the auto-branch-from-main feature.

## Test Execution

### Automated Regression Tests

Run the automated test suite:

```bash
# From repository root
./docs/wip/auto-branch-from-main/test-auto-branch.sh

# Run specific test
./docs/wip/auto-branch-from-main/test-auto-branch.sh test_happy_path_with_auto_create

# Use custom test directory (preserved after tests)
TEST_DIR=/tmp/my-test-dir ./docs/wip/auto-branch-from-main/test-auto-branch.sh

# Force cleanup even with custom directory
CLEANUP=1 TEST_DIR=/tmp/my-test-dir ./docs/wip/auto-branch-from-main/test-auto-branch.sh
```

**Directory Behavior:**
- **Default (no TEST_DIR)**: Uses `/tmp/gh-arc-test-$$` and automatically cleans up after tests
- **Custom TEST_DIR**: Preserves the directory after tests for inspection
- **CLEANUP=1**: Forces cleanup even when using custom TEST_DIR

### Manual Testing Prerequisites

For manual testing against a real GitHub repository:

1. **Test Repository**: Create a test repository on GitHub
2. **Authentication**: Ensure `gh auth status` shows you're authenticated
3. **Clean State**: Start with a clean main branch
4. **Configuration**: Set up test configuration in `.arc.json`

## Test Scenarios

### 1. Happy Path - Auto-Create Enabled

**Objective**: Verify automatic branch creation when commits exist on main.

**Setup**:
```bash
cd test-repo
git checkout main
git pull origin main

# Configure auto-create
cat > .arc.json <<EOF
{
  "diff": {
    "autoCreateBranchFromMain": true,
    "autoBranchNamePattern": "feature/auto-from-main-{timestamp}"
  }
}
EOF
```

**Steps**:
1. Create commits on main:
   ```bash
   echo "feature 1" >> feature.txt
   git add feature.txt
   git commit -m "Add feature 1"

   echo "feature 2" >> feature.txt
   git add feature.txt
   git commit -m "Add feature 2"
   ```

2. Run diff command:
   ```bash
   ./gh-arc diff
   ```

**Expected Results**:
- ✓ Detects 2 commits on main
- ✓ Displays commit list with titles and authors
- ✓ Shows generated branch name (e.g., `feature/auto-from-main-1729180000`)
- ✓ Asks for confirmation (if config requires)
- ✓ Opens editor for PR template
- ✓ Pushes new branch to origin
- ✓ Creates PR from new branch to main
- ✓ Checks out new branch locally
- ✓ User is now on feature branch, not main

**Verification**:
```bash
# Verify current branch
git branch --show-current  # Should show feature/auto-from-main-*

# Verify remote branch exists
git ls-remote origin | grep auto-from-main

# Verify PR created
gh pr list --head $(git branch --show-current)

# Verify main branch has no unpushed commits
git checkout main
git log origin/main..HEAD  # Should show nothing
```

---

### 2. Custom Branch Name Patterns

**Objective**: Verify all branch name pattern placeholders work correctly.

**Test 2.1: Date Pattern**
```bash
# Configure
cat > .arc.json <<EOF
{
  "diff": {
    "autoCreateBranchFromMain": true,
    "autoBranchNamePattern": "feature/auto-{date}"
  }
}
EOF

# Create commit
git checkout main
echo "test" >> test.txt
git add test.txt
git commit -m "Test date pattern"

# Run diff
./gh-arc diff
```

**Expected**: Branch name like `feature/auto-2025-10-17`

**Test 2.2: DateTime Pattern**
```bash
# Configure
cat > .arc.json <<EOF
{
  "diff": {
    "autoCreateBranchFromMain": true,
    "autoBranchNamePattern": "feature/{datetime}-auto"
  }
}
EOF

# Create commit and run diff
# ...
```

**Expected**: Branch name like `feature/2025-10-17T143052-auto`

**Test 2.3: Username Pattern**
```bash
# Configure
cat > .arc.json <<EOF
{
  "diff": {
    "autoCreateBranchFromMain": true,
    "autoBranchNamePattern": "feature/{username}/auto-{timestamp}"
  }
}
EOF

# Create commit and run diff
# ...
```

**Expected**: Branch name like `feature/john-doe/auto-1729180000`

**Test 2.4: Random String Pattern**
```bash
# Configure
cat > .arc.json <<EOF
{
  "diff": {
    "autoCreateBranchFromMain": true,
    "autoBranchNamePattern": "feature/auto-{random}"
  }
}
EOF

# Create commit and run diff
# ...
```

**Expected**: Branch name like `feature/auto-a7f3k9`

**Test 2.5: Null Pattern (Interactive Prompt)**
```bash
# Configure
cat > .arc.json <<EOF
{
  "diff": {
    "autoCreateBranchFromMain": true,
    "autoBranchNamePattern": null
  }
}
EOF

# Create commit and run diff
./gh-arc diff
# When prompted, enter: my-custom-branch
```

**Expected**: Branch name is exactly `my-custom-branch`

---

### 3. Branch Name Collision Handling

**Objective**: Verify automatic collision resolution.

**Setup**:
```bash
# Create existing branch with same name pattern
git checkout -b feature/auto-from-main-1729180000
git push origin feature/auto-from-main-1729180000
git checkout main
```

**Steps**:
1. Create commit on main
2. Mock system time to generate same timestamp (or use fixed pattern)
3. Run diff command

**Expected Results**:
- ✓ Detects collision
- ✓ Automatically appends `-2` to branch name
- ✓ Creates `feature/auto-from-main-1729180000-2`
- ✓ No user intervention required

**Multiple Collisions**:
```bash
# Create branches -2, -3, -4
git checkout -b feature/auto-from-main-1729180000-2
git push origin feature/auto-from-main-1729180000-2
git checkout -b feature/auto-from-main-1729180000-3
git push origin feature/auto-from-main-1729180000-3
git checkout main

# Create commit and run diff
# Expected: Creates feature/auto-from-main-1729180000-4
```

---

### 4. Stale Remote Warning

**Objective**: Verify warning when origin/main is old.

**Setup**:
```bash
# Configure low threshold
cat > .arc.json <<EOF
{
  "diff": {
    "autoCreateBranchFromMain": true,
    "staleRemoteThresholdHours": 1
  }
}
EOF

# Ensure origin/main is old (wait 1+ hours or manually adjust)
# For testing, temporarily modify the code to use a short threshold
```

**Steps**:
1. Wait for remote to become stale (or use test threshold)
2. Create commit on main
3. Run diff command

**Expected Results**:
- ✓ Displays stale remote warning
- ✓ Shows age of origin/main (e.g., "25 hours old")
- ✓ Suggests running `git fetch origin`
- ✓ Prompts whether to continue or abort
- ✓ If user chooses abort, operation cancels cleanly

**Verification**:
```bash
# After aborting
git branch --show-current  # Should still be main
git log origin/main..HEAD  # Commits still on main
```

---

### 5. Auto-Create Disabled (Interactive Prompt)

**Objective**: Verify interactive prompt when config is disabled.

**Setup**:
```bash
# Configure disabled
cat > .arc.json <<EOF
{
  "diff": {
    "autoCreateBranchFromMain": false
  }
}
EOF
```

**Steps**:
1. Create commits on main
2. Run diff command
3. When prompted "Create feature branch automatically?", respond:

**Test 5.1: User Accepts**
```bash
./gh-arc diff
# Prompt: "Create feature branch automatically? [y/N]"
# Response: y
# Prompt: "Enter branch name (leave empty for auto-generated):"
# Response: my-feature-branch
```

**Expected**:
- ✓ Proceeds with branch creation
- ✓ Uses provided branch name
- ✓ Completes full workflow

**Test 5.2: User Declines**
```bash
./gh-arc diff
# Prompt: "Create feature branch automatically? [y/N]"
# Response: n
```

**Expected**:
- ✓ Operation cancelled
- ✓ Error message: "Operation cancelled by user"
- ✓ User remains on main branch
- ✓ Commits still on main (not pushed)

---

### 6. Already on Feature Branch (Skip Detection)

**Objective**: Verify detection is skipped when not on main.

**Setup**:
```bash
git checkout -b my-feature
echo "feature" >> feature.txt
git add feature.txt
git commit -m "Feature commit"
```

**Steps**:
```bash
./gh-arc diff
```

**Expected Results**:
- ✓ No auto-branch detection occurs
- ✓ Normal diff workflow proceeds
- ✓ PR targets main (or configured base)

---

### 7. No Commits Ahead (Skip Detection)

**Objective**: Verify detection is skipped when no unpushed commits.

**Setup**:
```bash
git checkout main
git pull origin main
# Ensure no local commits
```

**Steps**:
```bash
./gh-arc diff
```

**Expected Results**:
- ✓ Detection runs but finds no commits ahead
- ✓ Normal diff workflow proceeds (may show "no changes" or existing PR)

---

### 8. Master Branch Detection

**Objective**: Verify feature works with `master` as default branch.

**Setup**:
```bash
# In repository using 'master' instead of 'main'
git checkout master
git pull origin master
```

**Steps**:
1. Create commits on master
2. Run diff command

**Expected Results**:
- ✓ Detects commits on master (not main)
- ✓ Shows "DefaultBranch: master" in detection
- ✓ Creates feature branch from master
- ✓ PR targets master

---

### 9. Username Sanitization

**Objective**: Verify special characters in username are sanitized.

**Setup**:
```bash
# Configure git with special characters
git config user.name "John Doe (Admin)"

cat > .arc.json <<EOF
{
  "diff": {
    "autoCreateBranchFromMain": true,
    "autoBranchNamePattern": "feature/{username}/auto"
  }
}
EOF
```

**Steps**:
1. Create commit on main
2. Run diff command

**Expected Results**:
- ✓ Username sanitized: `john-doe-admin`
- ✓ Branch name: `feature/john-doe-admin/auto`
- ✓ No invalid characters in branch name

**Test Other Special Cases**:
```bash
git config user.name "user@company.com"
# Expected: feature/user-company-com/auto

git config user.name "user.name.test"
# Expected: feature/user-name-test/auto

git config user.name "user_underscore"
# Expected: feature/user-underscore/auto
```

---

### 10. Error Recovery - Push Fails

**Objective**: Verify graceful handling when push fails.

**Setup**:
```bash
# Simulate push failure (requires network disconnection or GitHub downtime)
# Or use git hooks to reject push
```

**Steps**:
1. Create commits on main
2. Disconnect network or configure pre-push hook to fail
3. Run diff command

**Expected Results**:
- ✓ Push failure detected
- ✓ Error message with details
- ✓ Operation aborts cleanly
- ✓ User remains on main branch
- ✓ Local branch not created
- ✓ PR not created
- ✓ Clear recovery instructions shown

---

### 11. Error Recovery - PR Creation Fails

**Objective**: Verify handling when PR creation fails after push succeeds.

**Setup**:
```bash
# This requires simulating GitHub API failure
# Can be tested by:
# 1. Invalid authentication
# 2. Rate limit exceeded
# 3. Repository permissions issue
```

**Expected Results**:
- ✓ Branch pushed to remote (verified with `git ls-remote`)
- ✓ PR creation fails with error
- ✓ User NOT checked out to new branch (stays on main)
- ✓ Error message includes manual PR creation command
- ✓ User can manually create PR with `gh pr create`

---

### 12. Error Recovery - Checkout Fails

**Objective**: Verify non-fatal checkout failure handling.

**Setup**:
```bash
# Simulate checkout failure (uncommitted changes, etc.)
git checkout main
echo "uncommitted" >> test.txt
# Don't commit
```

**Steps**:
1. With uncommitted changes, try to run diff
2. If diff proceeds, PR should be created
3. Checkout will fail due to uncommitted changes

**Expected Results**:
- ✓ PR created successfully
- ✓ Checkout failure is non-fatal
- ✓ Warning message shown
- ✓ Manual checkout command provided
- ✓ User can resolve uncommitted changes and manually checkout

---

### 13. Multiple Commits Display

**Objective**: Verify commit list display is clear and informative.

**Setup**:
```bash
git checkout main

# Create multiple commits with different authors/times
git commit --allow-empty -m "Commit 1: Feature implementation"
sleep 2
git commit --allow-empty -m "Commit 2: Add tests"
sleep 2
git commit --allow-empty -m "Commit 3: Update documentation"
```

**Steps**:
```bash
./gh-arc diff
```

**Expected Results**:
- ✓ Shows all 3 commits in list
- ✓ Displays commit hashes (short form)
- ✓ Shows commit messages
- ✓ Shows author names
- ✓ Shows relative timestamps
- ✓ Formatted in readable table/list

**Example Output**:
```
Detected 3 unpushed commits on main:

  abc1234  Commit 3: Update documentation    (John Doe, 2 minutes ago)
  def5678  Commit 2: Add tests               (John Doe, 4 minutes ago)
  ghi9012  Commit 1: Feature implementation  (John Doe, 6 minutes ago)
```

---

### 14. Configuration Variations

**Objective**: Test various configuration combinations.

**Test 14.1: Minimal Config (Defaults)**
```bash
cat > .arc.json <<EOF
{
  "diff": {}
}
EOF
```
**Expected**: Uses default pattern, auto-create disabled, 24h threshold

**Test 14.2: All Options Specified**
```bash
cat > .arc.json <<EOF
{
  "diff": {
    "autoCreateBranchFromMain": true,
    "autoBranchNamePattern": "feat/{username}/{date}-{random}",
    "staleRemoteThresholdHours": 48
  }
}
EOF
```
**Expected**: All custom values used

**Test 14.3: Invalid Pattern**
```bash
cat > .arc.json <<EOF
{
  "diff": {
    "autoCreateBranchFromMain": true,
    "autoBranchNamePattern": "feature//invalid//"
  }
}
EOF
```
**Expected**: Validation error or sanitized to `feature/invalid`

---

## Performance Testing

### Large Number of Commits

**Objective**: Verify performance with many commits.

**Setup**:
```bash
git checkout main

# Create 50 commits
for i in {1..50}; do
  echo "commit $i" >> commits.txt
  git add commits.txt
  git commit -m "Commit $i"
done
```

**Expected**:
- ✓ Detects all 50 commits
- ✓ Displays commit list (may truncate if too long)
- ✓ Operation completes in reasonable time (<30s)
- ✓ No memory issues

---

## Security Testing

### 1. Malicious Branch Names

**Objective**: Verify injection prevention.

**Tests**:
```bash
# Test command injection
cat > .arc.json <<EOF
{
  "diff": {
    "autoBranchNamePattern": "feature/\$(rm -rf /tmp/test)"
  }
}
EOF

# Test path traversal
cat > .arc.json <<EOF
{
  "diff": {
    "autoBranchNamePattern": "../../etc/passwd"
  }
}
EOF

# Test null bytes
cat > .arc.json <<EOF
{
  "diff": {
    "autoBranchNamePattern": "feature/test\u0000malicious"
  }
}
EOF
```

**Expected**:
- ✓ All special characters sanitized
- ✓ No command execution
- ✓ No path traversal
- ✓ Safe branch names created

---

## Regression Test Checklist

Run this checklist after any changes to the auto-branch feature:

- [ ] Happy path with auto-create enabled
- [ ] All pattern placeholders ({timestamp}, {date}, {datetime}, {username}, {random})
- [ ] Branch name collision with retry
- [ ] Stale remote warning
- [ ] Auto-create disabled with user prompt (accept/decline)
- [ ] Already on feature branch (skip)
- [ ] No commits ahead (skip)
- [ ] Master branch detection
- [ ] Username sanitization
- [ ] Push failure recovery
- [ ] PR creation failure recovery
- [ ] Checkout failure (non-fatal)
- [ ] Multiple commits display
- [ ] All configuration variations
- [ ] Security: malicious branch names

---

## Troubleshooting Test Failures

### Test Hangs During Prompt

**Symptom**: Test script hangs waiting for input.

**Solution**: Ensure test is providing input via stdin or skipping interactive tests.

### Push Fails with "remote branch exists"

**Symptom**: Push fails even though collision handling should work.

**Solution**: Check that remote branch was deleted from previous test. Clean up:
```bash
git push origin --delete feature/auto-from-main-*
```

### Detection Not Triggering

**Symptom**: Auto-branch detection doesn't occur.

**Solution**: Verify:
1. Currently on main/master branch
2. Commits exist ahead of origin/main
3. Configuration is correct in .arc.json

### PR Creation Fails in Tests

**Symptom**: GitHub API returns errors during testing.

**Solution**:
1. Check authentication: `gh auth status`
2. Verify test repository exists and you have write access
3. Check rate limits: `gh api rate_limit`

---

## Test Cleanup

After each test, clean up:

```bash
# Delete test branches
git branch -D feature/auto-from-main-* 2>/dev/null || true
git push origin --delete feature/auto-from-main-* 2>/dev/null || true

# Close test PRs
gh pr list --json number --jq '.[].number' | xargs -I {} gh pr close {}

# Reset main branch
git checkout main
git reset --hard origin/main

# Remove test files
rm -f .arc.json feature.txt test.txt commits.txt
```

---

## Test Reporting

Document results using this template:

```markdown
## Test Run: YYYY-MM-DD

**Environment**:
- OS: Linux/macOS/Windows
- Go Version: 1.23.4
- Git Version: 2.43.0
- gh-arc Version: 0.1.0

**Results**:

| Test Scenario | Status | Notes |
|--------------|--------|-------|
| Happy path auto-create | ✓ Pass | Branch created: feature/auto-from-main-1729180000 |
| Custom patterns | ✓ Pass | All 5 patterns tested |
| Collision handling | ✓ Pass | Appended -2 correctly |
| Stale remote warning | ✓ Pass | Warning shown at 25 hours |
| ... | ... | ... |

**Failures**: 0
**Skipped**: 2 (interactive prompt tests)
**Duration**: 5m 32s
```
