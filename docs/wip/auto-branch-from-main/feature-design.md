# Feature Design: Auto-Branch from Main

## Status

Proposed

## Date

2025-10-16

## Overview

This feature enables `gh arc diff` to automatically handle the scenario where a user has committed directly to the main/master branch (violating trunk-based development workflow) by intelligently creating a feature branch and resetting main.

## Problem Statement

In trunk-based development workflows, developers, unless they bypass code-reviews and directly push to master, should always work on short-lived feature branches (from which Pull-Requests are made), and not directly on main/master. However, mistakes happen:

1. A developer forgets to create a feature branch and commits directly to `main`
2. They realize the mistake when running `gh arc diff`
3. Currently, `gh arc diff` will fail because it can't create a PR from main to main
4. The developer must manually:
   - Create a feature branch
   - Reset main back to origin/main
   - Checkout the feature branch
   - Push and create the PR

This manual process is error-prone and interrupts the workflow.

## User Stories

### Story 1: Accidental Commits to Main
**As a** developer who accidentally committed to main,
**I want** `gh arc diff` to automatically create a feature branch and clean up main,
**So that** I can continue my workflow without manual git operations.

### Story 2: Configurable Behavior
**As a** developer with specific preferences,
**I want** to configure whether auto-branch behavior happens automatically or requires confirmation,
**So that** I maintain control over my git operations.

### Story 3: Safe Stash Handling
**As a** developer with uncommitted changes,
**I want** my uncommitted changes to be safely preserved during auto-branching,
**So that** I don't lose any work-in-progress.

## Goals

1. **Seamless Workflow**: Automatically detect and fix the "commits on main" mistake
2. **Safety**: Never lose commits or uncommitted changes
3. **Configurability**: Allow users to opt-in/opt-out of automatic behaviors
4. **Transparency**: Clearly communicate what operations are being performed
5. **Consistency**: Follow existing `gh-arc` patterns and conventions

## Non-Goals

1. **Retroactive Fixing**: This feature only handles the current state, not historical mistakes
2. **Multi-Branch Cleanup**: Only handles the current branch (main/master)
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
  autoStashUncommittedChanges: true     # Auto-stash uncommitted changes during branch creation
  autoResetMain: true                   # Auto-reset main to origin/main after successful PR creation
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

4. Check uncommitted changes
   └─> Working directory dirty? YES

5. Handle uncommitted changes
   ├─> Check config: diff.autoStashUncommittedChanges
   │   ├─> true  → Stash changes
   │   └─> false → Prompt: "Stash uncommitted changes? (y/n)"
   │               ├─> y → Stash changes
   │               └─> n → Abort: "Cannot proceed with uncommitted changes"

6. Determine branch name
   ├─> Check config: diff.autoBranchNamePattern
   │   ├─> null   → Prompt: "Enter branch name: "
   │   ├─> ""     → Use default: feature/auto-from-main-<timestamp>
   │   └─> pattern → Apply pattern: feature/{username}-{date}

7. Create feature branch
   ├─> git branch <branch-name>
   ├─> git checkout <branch-name>
   ├─> git stash pop (if stashed)
   └─> Display: "✓ Created feature branch '<branch-name>' with your 2 commits"

8. Continue normal diff flow
   └─> Generate template, open editor, create PR...

9. After successful PR creation
   ├─> Check config: diff.autoResetMain
   │   ├─> true  → Reset main
   │   └─> false → Prompt: "Reset main to origin/main? (y/n)"
   │               ├─> y → Reset main
   │               └─> n → Skip reset
   │
   └─> If resetting:
       ├─> git checkout main
       ├─> git reset --hard origin/main
       ├─> git checkout <feature-branch>
       └─> Display: "✓ Reset main to origin/main"
```

### Error Handling

| Error Scenario | Detection | Handling |
|---------------|-----------|----------|
| No `origin/main` ref | Check `git rev-parse origin/main` | Use `main` as reset point (assumes offline or first commit) |
| User cancels prompt | Prompt returns false | Abort with clear message: "Operation cancelled by user" |
| Stash fails | `git stash` exits with error | Abort with error: "Failed to stash changes: <error>" |
| Branch already exists | `git branch` fails | Append counter: `feature/auto-from-main-1697654321-2` |
| Reset fails | `git reset --hard` fails | Warn but don't fail PR: "⚠️  Failed to reset main: <error>" |

### User Interaction Examples

#### Example 1: Fully Automatic (All configs = true)

```bash
$ gh arc diff

⚠️  Warning: You have 2 commits on main
✓ Creating feature branch: feature/auto-from-main-1697654321
✓ Stashing uncommitted changes...
✓ Created feature branch 'feature/auto-from-main-1697654321' with your 2 commits

Creating PR with base: main

# ... normal diff flow continues ...

✓ Success!
  PR #42: https://github.com/user/repo/pull/42

✓ Resetting main to origin/main...
✓ Reset main to origin/main
```

#### Example 2: With Prompts (All configs = false)

```bash
$ gh arc diff

⚠️  Warning: You have 2 commits on main

? Create feature branch automatically? (Y/n) y

? You have uncommitted changes. Stash them? (Y/n) y
✓ Stashing uncommitted changes...

? Enter branch name (or press Enter for default): feature/my-fix

✓ Created feature branch 'feature/my-fix' with your 2 commits

Creating PR with base: main

# ... normal diff flow continues ...

✓ Success!
  PR #42: https://github.com/user/repo/pull/42

? Reset main to origin/main? (Y/n) y
✓ Reset main to origin/main
```

#### Example 3: User Declines

```bash
$ gh arc diff

⚠️  Warning: You have 2 commits on main

? Create feature branch automatically? (Y/n) n

✗ Cannot create PR from main to main.
  Please create a feature branch manually:
    git checkout -b feature/my-branch
    git checkout main
    git reset --hard origin/main
    git checkout feature/my-branch
    gh arc diff
```

### Integration with Existing Code

#### Modified Files

1. **`cmd/diff.go`** (diff command)
   - Add detection logic at start of `runDiff()`
   - Call new `autoBranchFromMain()` function when detected
   - Display informational messages

2. **`internal/config/config.go`** (configuration)
   - Add new config fields to `DiffConfig` struct
   - Set defaults in `setDefaults()`
   - Add validation in `Validate()`

3. **`internal/git/git.go`** (git operations)
   - Add `CountCommitsAhead()` method
   - Add `Stash()` and `StashPop()` methods
   - Add `ResetHard()` method
   - Add `CheckoutBranch()` method

4. **`internal/diff/` package** (new files)
   - Create `internal/diff/auto_branch.go` for auto-branch logic
   - Create `internal/diff/auto_branch_test.go` for tests

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
   - Run auto-branch logic
   - Verify branch created, main reset, stash handled

2. **With Uncommitted Changes**
   - Test stash/unstash flow
   - Verify files preserved

3. **With Prompts**
   - Mock user input
   - Test various prompt responses

#### Manual Testing Scenarios

1. **Happy Path**: Commits on main, auto-branch, create PR, reset main
2. **With Stash**: Uncommitted changes during auto-branch
3. **All Prompts Declined**: User says no to everything
4. **Branch Name Collision**: Branch already exists with generated name
5. **Offline Mode**: No origin/main ref available

## Security Considerations

1. **No Data Loss**: All commits are preserved on the feature branch
2. **Stash Safety**: Uncommitted changes are stashed and restored
3. **Reset Safety**: Only resets main after successful PR creation
4. **User Control**: Configs allow disabling automatic behavior
5. **Abort on Error**: Any failure aborts the operation safely

## Performance Considerations

1. **Minimal Overhead**: Detection is a single git command
2. **No Network Calls**: All operations are local git operations
3. **Fast Operations**: Branch creation and reset are O(1) operations

## Backwards Compatibility

1. **Opt-Out by Default**: New behavior requires explicit config or confirmation
2. **Existing Users**: No impact unless they commit directly to main
3. **Config Defaults**: All new config fields have sensible defaults

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

**Approved** - Proceed with implementation as designed.
