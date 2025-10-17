# ADR 0002: Auto-Branch from Main

## Status

Accepted

## Date

2025-10-17

## Context

In trunk-based development workflows, developers should ideally create feature branches from the main branch and submit PRs from those feature branches. However, developers sometimes accidentally commit directly to the main/master branch instead of creating a feature branch first. This creates several problems:

1. **Direct commits on trunk violate trunk-based development principles** - Main should only receive commits through merged PRs
2. **No PR exists for code review** - Changes committed directly to main bypass the review process
3. **Manual recovery is error-prone** - Users must manually create a feature branch, push it, checkout, and create a PR
4. **Workflow interruption** - The process breaks the development flow and requires multiple manual git operations

Common scenarios where this occurs:
- Developer forgets to create feature branch before starting work
- Developer continues work on main after a `git pull` without switching branches
- New contributors unfamiliar with the workflow
- Multiple commits already made before realizing the mistake

When a user tries to run `gh arc diff` while on main with unpushed commits, the tool should detect this situation and offer to automatically create a feature branch, move the commits to that branch, and proceed with PR creation - all in a single seamless flow.

### Problem Detection Requirements

The tool needs to:
1. Detect when user is on main/master branch
2. Count unpushed commits ahead of origin/main
3. Check if remote tracking branch is stale (could lead to merge conflicts)
4. Display commits that will be moved to help user understand the operation

### User Experience Considerations

The automatic flow should:
- Be configurable (enable/disable, customize branch names)
- Prompt for confirmation when config is disabled
- Support custom branch naming patterns for consistency
- Handle branch name collisions gracefully
- Provide clear error messages with recovery instructions
- Warn about stale remote tracking branches

## Decision

We will implement an **auto-branch detection and creation system** that integrates into the `diff` command workflow. When a user runs `gh arc diff` while on main/master with unpushed commits, the system will:

1. **Detect** the scenario (commits on main)
2. **Prepare** by generating a unique branch name and getting user confirmation
3. **Push** the new branch to remote during the normal diff flow
4. **Create PR** as usual from the new branch
5. **Checkout** the new feature branch locally after PR creation succeeds

### Implementation Architecture

The implementation follows a **two-phase pattern**:

**Phase 1: Detection & Preparation** (before PR creation)
- Occurs early in the diff command flow
- Detects commits on main and prepares `AutoBranchContext`
- Gets user confirmation if config requires it
- Generates unique branch name
- Checks for stale remote tracking branches

**Phase 2: Execution** (split around PR creation)
- **Before PR**: Push new branch to remote
- **After PR succeeds**: Checkout new branch locally

This separation ensures:
- Template editing happens between detection and execution
- PR creation can proceed normally once branch is pushed
- Checkout only happens if PR creation succeeds (safe failure handling)

### Key Components

```go
// AutoBranchDetector - Main orchestrator
type AutoBranchDetector struct {
    repo   *git.Repository
    config *config.DiffConfig
}

// DetectionResult - Detection state
type DetectionResult struct {
    OnMainBranch  bool
    CommitsAhead  int
    DefaultBranch string
}

// AutoBranchContext - Execution state
type AutoBranchContext struct {
    BranchName    string
    ShouldProceed bool
}
```

### Configuration

```yaml
diff:
  autoCreateBranchFromMain: true  # Enable/disable automatic flow
  autoBranchNamePattern: "feature/auto-from-main-{timestamp}"
  staleRemoteThresholdHours: 24   # Warn if remote tracking branch is old
```

### Branch Name Patterns

Support placeholders for flexible naming:
- `{timestamp}` - Unix timestamp
- `{date}` - ISO date (2006-01-02)
- `{datetime}` - ISO datetime (2006-01-02T150405)
- `{username}` - Git user.name (sanitized)
- `{random}` - 6-character random alphanumeric
- `null` - Prompt user for branch name

### Error Handling

Use **sentinel errors** for type-safe handling:
```go
var (
    ErrOperationCancelled = errors.New("operation cancelled by user")
    ErrStaleRemote = errors.New("operation declined due to stale remote")
)
```

Callers use `errors.Is()` to detect specific scenarios and provide appropriate user messages.

## Alternatives Considered

### Option 1: Block and Error Out

**Description**: Detect commits on main and return an error, forcing user to manually fix.

**Pros**:
- Simplest implementation
- Forces users to learn proper workflow
- No automatic operations

**Cons**:
- Poor user experience (workflow interruption)
- Requires users to know git well enough to recover
- Error-prone manual process
- Goes against "stay in CLI" philosophy of gh-arc

**Verdict**: Rejected - defeats the purpose of seamless workflow

### Option 2: Automatic Branch Creation Without Confirmation

**Description**: Silently create branch and move commits whenever detected.

**Pros**:
- Completely seamless, zero interruption
- Simplest user experience

**Cons**:
- Too magical - surprising behavior
- No control for users who want to be prompted
- Could create confusion ("where did my commits go?")
- Violates principle of least surprise

**Verdict**: Rejected - too implicit, needs user awareness

### Option 3: Two-Phase Pattern with Configuration (CHOSEN)

**Description**: Detect and prepare before PR creation, execute around PR creation, make behavior configurable.

**Pros**:
- Balances automation with user control
- Configuration allows customization per project
- Clear two-phase separation makes code maintainable
- Safe failure handling (checkout only after PR succeeds)
- Shows commit list for transparency
- Supports custom branch naming patterns

**Cons**:
- More complex implementation (three integration points)
- State must be carried through diff command flow
- Requires careful error handling

**Verdict**: Accepted - best balance of UX and safety

### Option 4: Create Branch but Don't Checkout

**Description**: Push branch and create PR but leave user on main.

**Pros**:
- Simpler implementation
- User maintains original context

**Cons**:
- Confusing state - commits still exist locally on main
- User would have unpushed commits on main still
- Doesn't fully complete the workflow
- Leaves repository in inconsistent state

**Verdict**: Rejected - incomplete solution

### Option 5: Interactive Rebase Instead of Branch Creation

**Description**: Offer to interactively rebase commits off main instead of creating branch.

**Pros**:
- More explicit git operation
- Users learn git better

**Cons**:
- Complex UX with multiple prompts
- Requires understanding of interactive rebase
- Error-prone for git beginners
- Much slower workflow

**Verdict**: Rejected - too complex for the common case

## Consequences

### Positive

1. **Seamless Recovery** - Users can recover from accidental commits on main without leaving the CLI or interrupting workflow

2. **Configurable Behavior** - Projects can enable/disable and customize the feature per their needs

3. **Safe Failure Handling** - Checkout only occurs after PR creation succeeds; push failures abort cleanly

4. **Branch Name Flexibility** - Pattern system supports many naming conventions with placeholders

5. **Transparency** - Shows commit list before proceeding so user understands what will happen

6. **Stale Remote Detection** - Warns users when their local tracking branches are old (potential merge conflicts)

7. **Collision Handling** - Automatically generates unique branch names when collisions occur

8. **Sentinel Errors** - Type-safe error handling with `errors.Is()` allows appropriate user messaging

9. **Maintains Trunk-Based Philosophy** - Enables proper trunk-based development even when mistakes happen

10. **Consistent with gh-arc Goals** - Keeps developers in CLI without context switching

### Negative

1. **Implementation Complexity** - Requires three integration points in diff command (detect, push, checkout)

2. **State Management** - Must carry `AutoBranchContext` through diff command execution

3. **Potential Confusion** - Users might be surprised by automatic branch creation if not reading prompts carefully

4. **Testing Complexity** - Integration tests require real git repository operations and are harder to write

5. **Interactive Prompt Limitation** - Tests for interactive prompts must be skipped (cannot automate stdin)

6. **Multiple Configurations** - More config options increase documentation and support burden

### Neutral

1. **Behavior Change** - This is a new feature, no existing behavior changes

2. **Git Repository Requirements** - Requires remote tracking branches to exist (standard in GitHub workflows)

3. **Naming Collisions** - While handled automatically, high collision rates could indicate pattern issues

4. **Default Branch Detection** - Must detect main vs master vs other default branches

## Implementation References

### Core Implementation

- `internal/diff/auto_branch.go` - Main implementation (503 lines)
  - `AutoBranchDetector` type
  - `DetectCommitsOnMain()` - Detection logic
  - `PrepareAutoBranch()` - Preparation phase
  - `GenerateBranchName()` - Branch name generation with patterns
  - `EnsureUniqueBranchName()` - Collision handling
  - `CheckStaleRemote()` - Remote age validation
  - `displayCommitList()` - User-facing commit display
  - `promptYesNo()` and `promptBranchName()` - Interactive prompts
  - Sentinel errors: `ErrOperationCancelled`, `ErrStaleRemote`

- `internal/diff/auto_branch_test.go` - Unit tests (338 lines)
  - `TestShouldAutoBranch()` - Decision logic tests
  - `TestSanitizeBranchName()` - Name sanitization tests
  - `TestGenerateBranchName()` - Pattern placeholder tests
  - `TestDisplayCommitList()` - Display function tests

- `internal/diff/auto_branch_integration_test.go` - Integration tests (923 lines)
  - Full workflow tests with real git repositories
  - Test helpers: `createTestRepo()`, `createCommit()`, `createOldCommit()`, `mockRemoteBranch()`
  - 20+ test cases covering happy paths and error scenarios
  - 82.6% coverage achieved

### Git Support

- `internal/git/git.go` - Enhanced git operations
  - `CountCommitsAhead()` - Count commits ahead of remote
  - `BranchExists()` - Check if branch exists locally
  - `GetCommitsBetween()` - Get commit list for display
  - `GetRemoteRefAge()` - Calculate age of remote tracking branch
  - `PushBranch()` - Push with refspec
  - `CheckoutTrackingBranch()` - Create and checkout tracking branch

### Configuration

- `internal/config/config.go` - Configuration support
  - `DiffConfig.AutoCreateBranchFromMain` (default: false)
  - `DiffConfig.AutoBranchNamePattern` (default: "feature/auto-from-main-{timestamp}")
  - `DiffConfig.StaleRemoteThresholdHours` (default: 24)
  - Validation for pattern field

### Integration

- `cmd/diff.go` - Three integration points
  - Detection phase: Call detector and prepare context (after repo open)
  - Push phase: Push new branch if context exists (before PR creation)
  - Checkout phase: Checkout new branch if context exists (after PR succeeds)

### Documentation

- `docs/contributing/ARCHITECTURE.md` - Architecture documentation
  - Auto-branch module overview with flow diagram
  - Type definitions and method descriptions
  - Branch name patterns and placeholders
  - Configuration fields
  - Integration points with code examples
  - Two-phase pattern explanation
  - Error handling strategy

- `docs/adr/0002-auto-branch-from-main.md` - This document

### Commits

- Configuration and validation
- Git repository methods
- Auto-branch core implementation
- Auto-branch integration into diff command
- Comprehensive integration tests
- Architecture documentation
- This ADR

## Related Documentation

- [Trunk-Based Development](https://trunkbaseddevelopment.com/)
- [Git Remote Tracking Branches](https://git-scm.com/book/en/v2/Git-Branching-Remote-Branches)
- [Phabricator Arcanist Diff Workflow](https://we.phorge.it/book/phorge/article/arcanist_diff/)
- [GitHub Pull Requests Best Practices](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests)

## Notes

This feature embodies the core philosophy of `gh-arc`: enabling developers to maintain trunk-based development workflows entirely from the command line, with automatic handling of common mistakes and workflow interruptions.

The two-phase execution pattern (detect/prepare → push → checkout) was chosen specifically to:
1. Allow template editing between detection and execution
2. Enable safe failure handling (abort on push failure, non-fatal checkout failure)
3. Integrate cleanly with existing diff command flow
4. Maintain code maintainability with clear separation of concerns

The decision to make the feature configurable rather than always-on respects different team preferences and allows gradual adoption. The pattern system for branch naming provides flexibility for teams with naming conventions while still providing sensible defaults.

The sentinel error pattern (`ErrOperationCancelled`, `ErrStaleRemote`) was chosen to enable type-safe error handling with `errors.Is()`, allowing the diff command to detect specific scenarios and provide appropriate, actionable error messages to users.

Future enhancements could include:
- Linear integration for automatic ticket linking in branch names
- Git hooks for preventing commits on main entirely
- Analytics/telemetry for feature usage and collision rates
- Additional branch naming pattern placeholders (e.g., `{commit-message}`)
