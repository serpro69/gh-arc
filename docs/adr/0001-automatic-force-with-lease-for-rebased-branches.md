# ADR 0001: Automatic Force-With-Lease for Rebased Branches

## Status

Accepted

## Date

2025-10-16

## Context

In trunk-based development workflows (which `gh-arc` is designed to support), developers frequently rebase their feature branches to maintain a clean, linear commit history. This is particularly common when:

1. The base branch (e.g., `main`) has received updates since the feature branch was created
2. A developer wants to squash or reorganize commits before merging
3. Following code review feedback that suggests commit history improvements

When a user rebases a branch that already has an associated pull request, the git commit hashes change. Attempting to push these rebased commits to the remote branch results in a non-fast-forward error:

```
! [rejected]        feature-branch -> feature-branch (non-fast-forward)
error: failed to push some refs to 'github.com:user/repo.git'
hint: Updates were rejected because the tip of your current branch is behind
hint: its remote counterpart.
```

This error occurs because git detects that the local and remote histories have diverged, even though logically the changes represent the same work. Without automatic handling, users must manually intervene with `git push --force` or `git push --force-with-lease`, which:

- Interrupts the workflow
- Requires users to understand git's push mechanics
- Can lead to confusion for less experienced git users
- Breaks the seamless experience we want to provide

This problem was identified during implementation of the `diff` command, which handles both PR creation and updates.

## Decision

We will implement automatic force-push handling with `--force-with-lease` when non-fast-forward errors are detected in the `Push()` function.

The implementation will follow a **try-catch pattern**:

1. Attempt a regular `git push` first
2. If the push fails, check if the error is a non-fast-forward rejection
3. If yes, automatically retry with `git push --force-with-lease`
4. Log the transition clearly for debugging and transparency

### Implementation Details

```go
// Push() in internal/git/git.go
func (r *Repository) Push(ctx context.Context, branchName string) error {
    // First attempt: regular push
    cmd := exec.CommandContext(ctx, "git", "push", "origin", branchName)
    output, err := cmd.CombinedOutput()

    if err != nil {
        // Check for non-fast-forward error
        if isNonFastForwardError(string(output)) {
            logger.Info().Msg("Non-fast-forward detected, retrying with --force-with-lease")
            // Retry with --force-with-lease
            cmd = exec.CommandContext(ctx, "git", "push", "--force-with-lease", "origin", branchName)
            // ... handle retry
        }
    }
}

// Helper to detect non-fast-forward errors
func isNonFastForwardError(output string) bool {
    return strings.Contains(output, "[rejected]") &&
           strings.Contains(output, "non-fast-forward")
}
```

## Alternatives Considered

### Option 1: Always Use Force-With-Lease for Existing PRs

**Description**: Detect if a PR exists for the current branch and always use `--force-with-lease` for updates.

**Pros**:
- Simplest implementation
- No need for error detection
- Consistent behavior for PR updates

**Cons**:
- Unnecessarily uses force-push when not needed (no rebase occurred)
- Masks potential issues that might need user attention
- Goes against git best practices of preferring regular pushes when possible

**Verdict**: Rejected - too aggressive and hides information from users

### Option 2: Detect Divergence Before Pushing

**Description**: Compare local and remote commit hashes before attempting push to detect divergence.

**Pros**:
- Proactive rather than reactive
- Could provide more detailed information to user
- Avoids failed push attempts

**Cons**:
- More complex implementation requiring additional git operations
- Adds latency to every push operation
- Still needs the same logic to handle the actual push
- Adds complexity without clear benefit

**Verdict**: Rejected - over-engineered for the problem

### Option 3: Try-Catch Pattern (CHOSEN)

**Description**: Attempt regular push first, detect non-fast-forward errors, retry with force-with-lease.

**Pros**:
- Uses regular push when possible (best practice)
- Only uses force-push when actually needed
- Simple error detection logic
- Transparent through logging
- Matches user expectations from similar tools
- Minimal performance impact (only one extra attempt on rebase)

**Cons**:
- Requires parsing git error output
- One failed push attempt when rebased (minor)

**Verdict**: Accepted - balanced approach that follows git best practices

### Option 4: Prompt User When Divergence Detected

**Description**: Detect non-fast-forward and ask user what to do.

**Pros**:
- Gives user full control
- Educational for less experienced users
- Most conservative approach

**Cons**:
- Interrupts workflow (major issue for our use case)
- Requires interactive prompt handling
- Inconsistent with trunk-based development philosophy
- Breaks automation and CI/CD integration
- Different from Phabricator/Arcanist behavior we're emulating

**Verdict**: Rejected - defeats the purpose of seamless workflow

### Option 5: Use --force Instead of --force-with-lease

**Description**: Use regular `--force` for the retry instead of `--force-with-lease`.

**Pros**:
- Simpler flag, no lease checking

**Cons**:
- Unsafe: can overwrite other people's changes without warning
- Violates git safety best practices
- Could lead to data loss in collaborative scenarios

**Verdict**: Rejected - too dangerous

## Consequences

### Positive

1. **Seamless Workflow**: Users can rebase and update PRs without manual intervention or workflow interruption

2. **Safety**: Using `--force-with-lease` instead of `--force` provides protection against accidentally overwriting collaborators' changes

3. **Matches Industry Patterns**: Behavior aligns with Phabricator/Arcanist, which `gh-arc` is modeled after

4. **Transparent**: Clear logging shows when force-push is used, aiding debugging

5. **Git Best Practices**: Prefers regular push when possible, only uses force-push when necessary

6. **Minimal Performance Impact**: Only adds overhead when rebasing has actually occurred

### Negative

1. **Error Parsing Dependency**: Relies on parsing git's error output, which could theoretically change in future git versions (though `[rejected]` and `non-fast-forward` are stable strings)

2. **One Failed Attempt**: When rebased, there's always one failed push before the successful force-push (minimal impact)

3. **Less Explicit**: Users might not realize a force-push occurred unless they check logs with `-v` flag

### Neutral

1. **Behavior Change**: Existing users (if any) will see automatic behavior where they previously had to intervene manually

2. **Testing**: Requires integration testing with actual git operations to fully verify

## Implementation References

- Implementation: `internal/git/git.go` - `Push()` function and `isNonFastForwardError()` helper
- Tests: `internal/git/git_test.go` - `TestIsNonFastForwardError()`
- Commit: `8476a57` - "feat: add automatic force-with-lease for rebased branches"

## Related Documentation

- [Trunk-Based Development](https://trunkbaseddevelopment.com/)
- [Git push --force-with-lease](https://git-scm.com/docs/git-push#Documentation/git-push.txt---force-with-leaseltrefnamegt)
- [Phabricator Arcanist Workflows](https://we.phorge.it/book/phorge/article/arcanist/)

## Notes

This decision supports the core philosophy of `gh-arc`: enabling developers to stay in their command-line environment during the entire development workflow, without context switching to browsers or manual git operations. The automatic handling of rebased branches is a key part of providing that seamless experience.
