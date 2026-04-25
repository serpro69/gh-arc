# Design: `gh arc land` — Merge Approved PRs

> Status: design-complete
> Author: serpro69
> Date: 2026-04-25

## Overview

`gh arc land` is the counterpart to `gh arc diff` — together they form the core review loop. It merges the current branch's PR into its base branch after verifying preconditions (approval, CI), then cleans up the local workspace. Without `land`, users must leave the CLI to merge PRs via the GitHub web UI or use `gh pr merge` directly, losing the opinionated verification flow.

The original arcanist `arc land` was one of the most-used commands because it enforced team conventions (approval required, CI green) at merge time. Our version merges via GitHub API since direct pushes to protected branches aren't possible.

## Command Interface

```
gh arc land [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--squash` | bool | Squash merge (default from config) |
| `--rebase` | bool | Rebase merge |
| `--force` | bool | Bypass approval and CI checks |
| `--edit` | bool | Open `$EDITOR` to customize merge commit message |
| `--no-delete` | bool | Keep the local branch after merge |

- `--squash` and `--rebase` are mutually exclusive. When neither is given, the config value `land.defaultMergeMethod` is used.
- Regular merge commits (`--merge`) are intentionally not supported — trunk-based development uses squash or rebase.
- No positional arguments. The PR is always detected from the current branch.

## Config Changes

### `LandConfig` struct

```go
type LandConfig struct {
    DefaultMergeMethod string `mapstructure:"defaultMergeMethod"` // "squash" (default), "rebase"
    DeleteLocalBranch  bool   `mapstructure:"deleteLocalBranch"`  // default: true
    RequireApproval    string `mapstructure:"requireApproval"`    // "strict" (default), "prompt", "none"
    RequireCI          string `mapstructure:"requireCI"`          // "required" (default), "all", "none"
}
```

**Changes from existing config:**

1. **`RequireApproval`**: `bool` → `string` enum.
   - `"strict"` — hard block, `--force` to bypass.
   - `"prompt"` — warn and ask interactive confirmation, `--force` bypasses the prompt.
   - `"none"` — skip approval check entirely.

2. **`RequireCI`**: `bool` → `string` enum.
   - `"required"` — only branch protection required checks must pass.
   - `"all"` — every check run must pass.
   - `"none"` — skip CI check entirely.

3. **`DeleteRemoteBranch`**: removed. Remote branch deletion is deferred to GitHub's repo-level "automatically delete head branches" setting.

4. **`DefaultMergeMethod`**: validation tightened to only accept `"squash"` and `"rebase"`.

No backward compatibility shim for old boolean values — we're in alpha.

### Config defaults

```go
v.SetDefault("land.defaultMergeMethod", "squash")
v.SetDefault("land.deleteLocalBranch", true)
v.SetDefault("land.requireApproval", "strict")
v.SetDefault("land.requireCI", "required")
```

### Config validation

- `defaultMergeMethod` must be `"squash"` or `"rebase"`
- `requireApproval` must be `"strict"`, `"prompt"`, or `"none"`
- `requireCI` must be `"required"`, `"all"`, or `"none"`

## Workflow

### High-level sequence

```
1. Check working directory is clean
2. Get current branch (fail if on trunk)
3. Find open PR for current branch
4. Enrich PR with reviews + CI checks
5. Run pre-merge checks (approval, CI)
6. Prepare commit message (optionally open $EDITOR)
7. Warn about dependent PRs (informational)
8. Call GitHub merge API
9. Checkout default branch + pull latest
10. Delete local branch (unless --no-delete)
11. Print summary
```

### Routing

Unlike `diff` which has multiple modes (continue, fast-path, normal), `land` has a single linear flow. The only branching points are:
- `--force` skips checks 5
- `--edit` adds an editor step at 6
- `--no-delete` skips step 10
- `requireApproval: "prompt"` adds an interactive confirmation at step 5

## Pre-merge Checks

Checks are evaluated in order, failing fast on the first blocking issue (unless `--force`).

### Check 1: Working directory clean

- Uses `repo.GetWorkingDirectoryStatus()`
- Always blocks if dirty, even with `--force`. Merging with uncommitted changes causes problems when switching branches post-merge.
- Not configurable.

### Check 2: Not on trunk

- If current branch is the default branch: hard fail.
- Not bypassable — landing trunk onto itself is nonsensical.

### Check 3: PR exists

- Uses `client.FindExistingPRForCurrentBranch()`
- If no open PR: fail with actionable message suggesting `gh arc diff`.
- Not bypassable.

### Check 4: Approval status

- Evaluates PR reviews directly (not via `DeterminePRStatus()`, which combines approval and CI into a single status — `land` needs them evaluated independently since each has its own config enum).
- Behavior by `requireApproval`:
  - `"strict"`: blocks if not approved. `--force` bypasses.
  - `"prompt"`: warns and asks `Proceed with merge? [y/N]`. `--force` bypasses the prompt. In non-TTY environments (piped input, CI), auto-declines with a message: "Non-interactive environment — use --force to bypass approval check."
  - `"none"`: skips entirely.
- Actionable error messages: "PR needs 1 more approval", "PR has outstanding change requests from @bob".

### Check 5: CI status

- Behavior by `requireCI`:
  - `"required"`: query branch protection rules for required status checks, verify those pass. `--force` bypasses.
  - `"all"`: all check runs must be `success`, `skipped`, or `neutral`. `--force` bypasses.
  - `"none"`: skips entirely.
- In-progress checks are treated as blocking (not yet passed).
- Error messages list specific failures: "CI check 'tests' failed", "CI check 'lint' is in progress".

### Dependent PRs (informational, never blocks)

- Uses `client.FindDependentPRs()`
- If found: warn with count, never blocks.
- GitHub auto-retargets child PRs when the parent branch is deleted after merge (requires "automatically delete head branches" enabled in repo settings).

## Merge Execution

### Commit message

**Default (no `--edit`):**
- Squash: title = PR title, body = PR body
- Rebase: GitHub controls commit messages (individual commits preserved)

**With `--edit`:**
- Write temp file: PR title on line 1, blank line, PR body below
- Open `$EDITOR` (reuse `GetEditorCommand()` from `internal/template/`)
- Parse: first line = title, rest = body
- Empty/unchanged = abort merge (no harm done)
- Only meaningful for squash (rebase preserves individual commits)
- If `--edit` is combined with `--rebase`: skip the editor with a warning ("--edit is ignored with --rebase, which preserves individual commits")

### GitHub API

`PUT /repos/{owner}/{repo}/pulls/{number}/merge`

Request body:
- `merge_method`: `"squash"` or `"rebase"`
- `commit_title`: prepared title (squash only)
- `commit_message`: prepared body (squash only)

New method: `client.MergePullRequest(ctx, owner, repo, number, *MergeOptions) (*MergeResult, error)`

### Error handling

| HTTP Status | Meaning | User message |
|-------------|---------|--------------|
| 405 | Merge method not allowed by repo settings | "Squash merges are not allowed on this repo. Try `--rebase` or update repo settings." |
| 409 | Merge conflicts | "PR has merge conflicts. Resolve conflicts and push before landing." |
| 422 | Not mergeable (e.g., branch protection) | "GitHub blocked the merge — check branch protection rules." |

## Post-merge Cleanup

After a successful merge, cleanup runs sequentially. The merge is the critical operation — everything after it is best-effort. Cleanup failures never produce a non-zero exit code if the merge succeeded.

### Step 1: Checkout default branch

- Get branch name from config or `repo.GetDefaultBranch()`
- `git checkout <default-branch>`
- Failure: non-fatal warning with manual command, skip remaining steps.

### Step 2: Pull latest

- `git pull origin <default-branch>`
- Failure: non-fatal warning.

### Step 3: Delete local branch

- Skip if `--no-delete` or `deleteLocalBranch: false`
- Capture branch tip SHA before deletion (for restore message)
- `git branch -D <branch>` (`-D` because local tracking info may not reflect the remote merge)
- On success: `Deleted local branch feature/auth (use git checkout -b feature/auth a1b2c3d to restore)`
- Failure: non-fatal warning.

## Output

### Progress format

Each step prints as it completes:

```
✓ Found PR #42: "Add auth middleware" (feature/auth → main)
✓ Approved by @alice, @bob
✓ All CI checks passed (3/3)
⚠ 1 dependent PR targets this branch — will be retargeted after merge
✓ Squash-merged into main (abc1234)
✓ Switched to main, pulled latest
✓ Deleted local branch feature/auth (use git checkout -b feature/auth a1b2c3d to restore)
```

### Status indicators

- `✓` — success
- `✗` — blocking failure (with actionable guidance on the next line, indented)
- `⚠` — warning (doesn't block)

### Failure examples

```
✓ Found PR #42: "Add auth middleware" (feature/auth → main)
✗ PR needs approval — no reviews yet
  Request a review or use --force to bypass
```

```
✓ Found PR #42: "Add auth middleware" (feature/auth → main)
✓ Approved by @alice
✗ CI check 'tests' failed, 'lint' in progress (1/3 passed)
  Wait for checks to complete or use --force to bypass
```

### Prompt mode

```
✓ Found PR #42: "Add auth middleware" (feature/auth → main)
⚠ PR has no approvals
  Proceed with merge? [y/N]
```

## New GitHub API Methods

### `MergePullRequest`

```
PUT /repos/{owner}/{repo}/pulls/{number}/merge
```

Added to `internal/github/pullrequest.go` or a new `internal/github/merge.go`.

Types:
- `MergeOptions` — merge method, commit title, commit message
- `MergeResult` — merged bool, merge commit SHA, message

### `GetBranchProtection` (for `requireCI: "required"`)

```
GET /repos/{owner}/{repo}/branches/{branch}/protection
```

Needed to discover which status checks are required. Falls back to an empty required-checks list if the API call fails (404 = no branch protection, 403 = insufficient permissions). This means on failure, `requireCI: "required"` effectively behaves like `"none"` — practical because users who lack admin permissions shouldn't be blocked. Users who want strict CI enforcement regardless can use `requireCI: "all"` instead.

## Package Structure

```
internal/land/
├── workflow.go    — LandWorkflow orchestrator, Execute() entry point
├── checks.go     — pre-merge verification (approval, CI, dirty WD, dependent PRs)
├── merge.go      — merge execution (commit message, API call)
├── cleanup.go    — post-merge cleanup (checkout, pull, delete branch)
└── output.go     — terminal output formatting
```

```
cmd/
└── land.go        — thin Cobra command, flag parsing, wires up workflow
```

## Relationship to Arcanist

The original `arc land` pushed the commit directly and closed the Differential revision. Our version merges via GitHub API since direct pushes to protected branches aren't possible. The verification philosophy is the same: enforce team conventions (approval, CI) at merge time, then clean up the local workspace for the next task.
