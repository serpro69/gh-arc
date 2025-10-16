# Feature Design: Auto-Branch from Main

## Status

Proposed (Simplified)

## Date

2025-10-16 (Revised)

## Overview

This feature enables `gh arc diff` to automatically handle the scenario where a user has committed directly to the main/master branch by creating a feature branch remotely and switching to track it locally, without any destructive local git operations.

## Problem Statement

In trunk-based development workflows, developers should always work on short-lived feature branches, not directly on main/master. However, mistakes happen:

1. A developer forgets to create a feature branch and commits directly to `main`
2. They realize the mistake when running `gh arc diff`
3. Currently, `gh arc diff` will fail because it can't create a PR from main to main
4. The developer must manually:
   - Create a feature branch
   - Push the branch
   - Create the PR

This manual process interrupts the workflow.

## User Stories

### Story 1: Accidental Commits to Main
**As a** developer who accidentally committed to main,
**I want** `gh arc diff` to automatically create a feature branch and PR,
**So that** I can continue my workflow without manual git operations.

### Story 2: Configurable Behavior
**As a** developer with specific preferences,
**I want** to configure whether auto-branch behavior happens automatically or requires confirmation,
**So that** I maintain control over my git operations.

### Story 3: Simple and Safe
**As a** developer,
**I want** the auto-branch flow to avoid destructive operations like `git reset --hard`,
**So that** I can trust the tool won't lose my work.

## Goals

1. **Seamless Workflow**: Automatically detect and fix the "commits on main" mistake
2. **Safety**: Never lose commits or use destructive operations (`git reset --hard`)
3. **Simplicity**: Minimal git operations, easy to understand flow
4. **Configurability**: Allow users to opt-in/opt-out of automatic behaviors
5. **Transparency**: Clearly communicate what operations are being performed

## Non-Goals

1. **Automatic Main Reset**: We don't automatically reset main to origin/main (user can do this manually or it syncs on next pull)
2. **Uncommitted Changes Handling**: Since we're not changing HEAD, uncommitted changes are unaffected and need no special handling
3. **Remote Sync**: Does not fetch from origin or handle remote sync issues
4. **Conflict Resolution**: Does not handle merge conflicts or complex git states

## Design Details

### Detection Logic

The feature activates when all these conditions are met:

1. User runs `gh arc diff`
2. Current branch is the default branch (main, master, trunk, etc.)
3. Local default branch has commits ahead of `origin/<default-branch>`

```go
// Pseudo-code for detection
func detectCommitsOnMain(repo GitRepository) (bool, int, error) {
    currentBranch := repo.GetCurrentBranch()
    defaultBranch := repo.GetDefaultBranch()

    if currentBranch != defaultBranch {
        return false, 0, nil
    }

    // Check if ahead of origin
    commitCount := repo.CountCommitsAhead(defaultBranch, "origin/" + defaultBranch)

    return commitCount > 0, commitCount, nil
}
```

### Configuration Schema

New configuration fields in `.arc.json` / `.arc.yaml`:

```yaml
diff:
  # Existing fields...
  createAsDraft: false
  enableStacking: true

  # New fields for auto-branch from main
  autoCreateBranchFromMain: true        # Auto-create branch when commits on main detected
  autoBranchNamePattern: ""             # Pattern for branch name (empty = default, null = prompt)
```

### Branch Naming Patterns

The `autoBranchNamePattern` config supports:

- **Empty string `""`** (default): Use automatic pattern `feature/auto-from-main-<timestamp>`
- **`null`**: Prompt user for branch name interactively
- **Pattern string**: Use pattern with placeholders:
  - `{timestamp}`: Unix timestamp (e.g., `1697654321`)
  - `{date}`: ISO date (e.g., `2025-10-16`)
  - `{datetime}`: ISO datetime (e.g., `2025-10-16T143022`)
  - `{username}`: Git user.name
  - `{random}`: Random 6-character alphanumeric

Example patterns:
```yaml
autoBranchNamePattern: "feature/{username}-{date}"        # feature/john-2025-10-16
autoBranchNamePattern: "auto/{datetime}"                  # auto/2025-10-16T143022
autoBranchNamePattern: "fix/emergency-{random}"           # fix/emergency-a7k3m9
```

### Workflow Execution

#### Step-by-Step Flow

```
1. User runs: gh arc diff
   └─> Current branch: main
   └─> Commits ahead: 2

2. Detect situation
   ├─> On main? YES
   ├─> Commits ahead? YES (2 commits)
   └─> Feature enabled? Check config

3. Check config: diff.autoCreateBranchFromMain
   ├─> true  → Proceed
   └─> false → Prompt: "Create feature branch automatically? (y/n)"
               ├─> y → Proceed
               └─> n → Abort: "Cannot create PR from main to main"

4. Determine branch name
   ├─> Check config: diff.autoBranchNamePattern
   │   ├─> null   → Prompt: "Enter branch name: "
   │   ├─> ""     → Use default: feature/auto-from-main-<timestamp>
   │   └─> pattern → Apply pattern: feature/{username}-{date}
   │
   └─> Ensure branch name is unique (check local and remote)

5. Continue normal diff flow
   └─> Generate template, open editor, etc.

6. Push branch to remote
   ├─> git push origin HEAD:refs/heads/<branch-name>
   └─> Display: "✓ Pushed branch '<branch-name>' to remote"

7. Create Pull Request via GitHub API
   └─> Create PR: <branch-name> → main

8. Switch to new branch locally
   ├─> git checkout -b <branch-name> origin/<branch-name>
   └─> Display: "✓ Switched to feature branch '<branch-name>'"

9. Display success message
   ├─> Show PR URL
   └─> Show informational message:
       "ℹ️  Note: Your local 'main' branch is still ahead of origin/main.
        You can reset it manually with:
          git checkout main && git reset --hard origin/main
        Or it will sync automatically when you run 'gh arc land'."
```

### Error Handling

| Error Scenario | Detection | Handling |
|---------------|-----------|----------|
| No commits ahead | `CountCommitsAhead` returns 0 | Display message: "No changes to diff" and exit |
| User cancels prompt | Prompt returns false | Abort with clear message: "Operation cancelled by user" |
| Branch already exists | Check before push | Append counter: `feature/auto-from-main-1697654321-2` |
| Push fails | `git push` exits with error | Stay on main, display error, exit with error code |
| PR creation fails | GitHub API error | Stay on main, don't create local branch, display error |
| Checkout fails | `git checkout` fails | Display error with manual recovery instructions |

### User Interaction Examples

#### Example 1: Fully Automatic (Config = true, default pattern)

```bash
$ gh arc diff

⚠️  Warning: You have 2 commits on main
✓ Creating feature branch: feature/auto-from-main-1697654321

Creating PR with base: main

# ... normal diff flow (template editor, etc.) ...

✓ Pushed branch 'feature/auto-from-main-1697654321' to remote
✓ Created PR #42: https://github.com/user/repo/pull/42
✓ Switched to feature branch 'feature/auto-from-main-1697654321'

ℹ️  Note: Your local 'main' branch is still ahead of origin/main.
   You can reset it manually with:
     git checkout main && git reset --hard origin/main
   Or it will sync automatically when you run 'gh arc land'.
```

#### Example 2: With Prompts (Config = false)

```bash
$ gh arc diff

⚠️  Warning: You have 2 commits on main

? Create feature branch automatically? (Y/n) y

? Enter branch name (or press Enter for default): feature/my-fix

✓ Creating feature branch: feature/my-fix

Creating PR with base: main

# ... normal diff flow ...

✓ Pushed branch 'feature/my-fix' to remote
✓ Created PR #42: https://github.com/user/repo/pull/42
✓ Switched to feature branch 'feature/my-fix'

ℹ️  Note: Your local 'main' branch is still ahead of origin/main.
   You can reset it manually with:
     git checkout main && git reset --hard origin/main
   Or it will sync automatically when you run 'gh arc land'.
```

#### Example 3: User Declines

```bash
$ gh arc diff

⚠️  Warning: You have 2 commits on main

? Create feature branch automatically? (Y/n) n

✗ Cannot create PR from main to main.
  Please create a feature branch manually:
    git checkout -b feature/my-branch
    git push origin feature/my-branch
    gh arc diff
```

#### Example 4: Push Failure

```bash
$ gh arc diff

⚠️  Warning: You have 2 commits on main
✓ Creating feature branch: feature/auto-from-main-1697654321

Creating PR with base: main

# ... normal diff flow ...

✗ Failed to push branch to remote: permission denied

  You are still on the 'main' branch.
  You can try pushing manually:
    git checkout -b feature/auto-from-main-1697654321
    git push origin feature/auto-from-main-1697654321
    gh arc diff
```

### Integration with Existing Code

#### Modified Files

1. **`cmd/diff.go`** (diff command)
   - Add detection logic at start of `runDiff()`
   - Call new `autoBranchFromMain()` function when detected
   - Display informational messages
   - Add logic after PR creation to push branch and switch locally

2. **`internal/config/config.go`** (configuration)
   - Add new config fields to `DiffConfig` struct
   - Set defaults in `setDefaults()`
   - Add validation in `Validate()`

3. **`internal/git/git.go`** (git operations)
   - Add `CountCommitsAhead()` method
   - Add `BranchExists()` method (check local and remote)
   - Add `CheckoutBranch()` method with tracking

4. **`internal/diff/` package** (new files)
   - Create `internal/diff/auto_branch.go` for auto-branch logic
   - Create `internal/diff/auto_branch_test.go` for tests

### Differences from Original Design

This simplified design differs from the original in these key ways:

1. **No `git reset --hard`**: The riskiest operation is eliminated entirely
2. **No stash operations**: Since we're not changing HEAD, uncommitted changes are unaffected
3. **No main branch cleanup**: User's main branch is left as-is, syncs naturally on next pull
4. **Simpler flow**: Fewer git operations, easier to understand and debug
5. **Push happens after PR metadata**: We complete the normal diff flow (template, etc.) then push
6. **Stays on main if push fails**: More predictable error recovery

### Testing Strategy

#### Unit Tests

1. **Branch Name Generation**
   - Test pattern parsing with all placeholders
   - Test default pattern generation
   - Test branch name collision handling

2. **Configuration Loading**
   - Test default values
   - Test config file loading
   - Test validation

3. **Detection Logic**
   - Test when on main with commits ahead
   - Test when on feature branch (should not trigger)
   - Test when on main but up-to-date (should not trigger)

#### Integration Tests

1. **Full Auto-Branch Flow**
   - Create test repo with commits on main
   - Run detection and auto-branch flow
   - Verify branch pushed to remote
   - Verify local branch created tracking remote
   - Verify main unchanged

2. **Custom Branch Name Pattern**
   - Test each placeholder type
   - Verify generated names match patterns

3. **Error Scenarios**
   - Push failure (stay on main)
   - PR creation failure (don't create local branch)
   - Checkout failure (display recovery instructions)

#### Manual Testing Scenarios

1. **Happy Path**: Commits on main, auto-branch, create PR, push, switch
2. **All Prompts Declined**: User says no to everything
3. **Branch Name Collision**: Branch already exists with generated name
4. **Network Failure**: Simulate push failure
5. **With Uncommitted Changes**: Verify they're preserved (no stash needed)

## Security Considerations

1. **No Data Loss**: All commits are preserved, no destructive operations
2. **No Stash Required**: Uncommitted changes stay in working directory
3. **Main Unchanged**: Local main branch remains exactly as user left it
4. **User Control**: Configs allow disabling automatic behavior
5. **Abort on Error**: Any failure leaves user in safe state (on main branch)

## Performance Considerations

1. **Minimal Overhead**: Detection is a single git command
2. **One Network Call**: Only the push operation requires network
3. **Fast Operations**: Branch operations are O(1)

## Backwards Compatibility

1. **Opt-In Behavior**: Only activates when commits detected on main
2. **Existing Users**: No impact unless they commit directly to main
3. **Config Defaults**: New config fields have sensible defaults

## Documentation Updates Required

1. **README.md**: Add section explaining the feature
2. **ARCHITECTURE.md**: Document new `auto_branch.go` module
3. **User Guide**: Add troubleshooting section for this scenario
4. **Config Reference**: Document new configuration options

## Future Enhancements

1. **Multi-Branch Support**: Handle multiple default branches (main, master, develop)
2. **Custom Remote**: Support remotes other than `origin`
3. **Integration with `gh arc work`**: Auto-suggest proper workflow
4. **Metrics**: Track how often this feature is used

## References

- [Trunk-Based Development Best Practices](https://trunkbaseddevelopment.com/)
- [Git Branching Strategies](https://git-scm.com/book/en/v2/Git-Branching-Branching-Workflows)
- [Arcanist Workflows](https://we.phorge.it/book/phorge/article/arcanist/)

## Open Questions

None - all questions resolved during design discussion.

## Decision

**Approved** - Proceed with simplified implementation as designed.
