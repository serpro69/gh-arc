# Implementation Plan: `gh arc land`

> Status: ready
> Design: [design.md](./design.md)
> Date: 2026-04-25

## Prerequisites

The developer should be familiar with:
- The `internal/diff/` package structure — `land` mirrors the same orchestrator pattern
- `internal/github/pullrequest.go` — existing PR types and enrichment methods
- `internal/config/config.go` — config structs, defaults, and validation
- `cmd/diff.go` — the thin command pattern that `cmd/land.go` will follow

## Phase 1: Config Changes

### 1.1 Update `LandConfig` struct

**File:** `internal/config/config.go`

Change `LandConfig`:
- `RequireApproval`: change type from `bool` to `string`
- `RequireCI`: change type from `bool` to `string`
- Remove `DeleteRemoteBranch` field entirely

### 1.2 Update defaults

**File:** `internal/config/config.go` → `setDefaults()`

- `land.requireApproval` → `"strict"` (was `true`)
- `land.requireCI` → `"required"` (was `true`)
- Remove `land.deleteRemoteBranch` default
- Keep `land.defaultMergeMethod` as `"squash"`
- Keep `land.deleteLocalBranch` as `true`

### 1.3 Update validation

**File:** `internal/config/config.go` → `Validate()`

- `defaultMergeMethod`: change valid set from `{"squash", "merge", "rebase"}` to `{"squash", "rebase"}`
- Add validation for `requireApproval`: must be `"strict"`, `"prompt"`, or `"none"`
- Add validation for `requireCI`: must be `"required"`, `"all"`, or `"none"`
- Remove any `deleteRemoteBranch` references

### 1.4 Update README config docs

**File:** `README.md`

Update both JSON and YAML config examples to reflect the new `LandConfig` fields, enum values, and the removal of `deleteRemoteBranch`.

### Verification

- `go test ./internal/config/...` passes
- Existing config tests may need updating for the type changes
- Test that invalid enum values are rejected by `Validate()`

## Phase 2: GitHub API — Merge Method

### 2.1 Add merge types and method

**File:** `internal/github/pullrequest.go` (add to existing file, near the bottom with other PR operations)

Add types:
- `MergeOptions` struct: `Method string`, `CommitTitle string`, `CommitMessage string`
- `MergeResult` struct: `Merged bool`, `SHA string`, `Message string`

Add method `MergePullRequest(ctx, owner, repo string, number int, opts *MergeOptions) (*MergeResult, error)`:
- `PUT /repos/{owner}/{repo}/pulls/{number}/merge`
- Map `MergeOptions` to the API request body (`merge_method`, `commit_title`, `commit_message`)
- Parse response for merge commit SHA
- Map HTTP error codes to semantic errors:
  - 405 → merge method not allowed
  - 409 → merge conflicts
  - 422 → not mergeable
- Add convenience method `MergePullRequestForCurrentRepo()`

### 2.2 Add branch protection query (for `requireCI: "required"`)

**File:** `internal/github/pullrequest.go`

Add method `GetRequiredStatusChecks(ctx, owner, repo, branch string) ([]RequiredCheck, error)`:
- `GET /repos/{owner}/{repo}/branches/{branch}/protection/required_status_checks`
- Branch name must be URL-escaped via `url.PathEscape()` (e.g. `release/1.2` → `release%2F1.2`)
- Returns `[]RequiredCheck` where `RequiredCheck{Context string, AppID *int}` — uses the newer `checks` field with `app_id` when available, falls back to legacy `contexts` field
- If 404 (no branch protection): return empty list (no required checks)
- If 403 (insufficient permissions): return `ErrBranchProtectionPermissionDenied` — callers decide how to handle (the land checks module treats this as blocking unless `--force`)

### Verification

- `go test ./internal/github/...` passes
- Unit tests with `httptest` server mocking the merge endpoint for success, 405, 409, 422 responses
- Unit tests for `GetRequiredStatusChecks` with 200, 404, 403 responses

## Phase 3: Land Package — Core Workflow

### 3.1 Output module

**File:** `internal/land/output.go`

Create `OutputStyle` struct following the same pattern as `internal/diff/output.go`:
- `PrintStep(icon, message string)` — prints `✓`/`✗`/`⚠` prefixed lines
- `PrintDetail(message string)` — prints indented detail/guidance lines
- Helper methods for each step: `PrintPRFound()`, `PrintApprovalStatus()`, `PrintCIStatus()`, `PrintDependentPRs()`, `PrintMerged()`, `PrintCheckout()`, `PrintBranchDeleted()`, `PrintCleanupWarning()`
- `FormatLandResult(*LandResult) string` for the final summary

### 3.2 Checks module

**File:** `internal/land/checks.go`

Create types:
- `PreMergeChecker` struct holding `*git.Repository`, `*github.Client`, `*config.LandConfig`, owner/name
- `CheckResult` struct: `Passed bool`, `Messages []string`, `NeedsConfirmation bool`

Create methods:
- `CheckCleanWorkingDir() error` — uses `repo.GetWorkingDirectoryStatus()`, returns error if dirty
- `CheckNotOnTrunk(currentBranch, defaultBranch string) error` — compares branch names
- `CheckPRExists(ctx, branchName string) (*github.PullRequest, error)` — wraps `FindExistingPRForCurrentBranch()`
- `CheckLocalHeadMatchesPR(*github.PullRequest) error` — compares local `HEAD` SHA to `pr.Head.SHA`; hard fail if different (not bypassable). Uses `repo.GetHeadSHA()`
- `CheckApproval(ctx, *github.PullRequest, force bool) (*CheckResult, error)` — evaluates reviews based on `requireApproval` config
- `CheckCI(ctx, *github.PullRequest, force bool) (*CheckResult, error)` — evaluates checks based on `requireCI` config. When `"required"`, calls `GetRequiredStatusChecks()` and filters checks to only those in the required list. On 403 (permission denied), returns a blocking `CheckResult` with guidance instead of silently degrading; `--force` bypasses.
- `CheckDependentPRs(ctx, branchName string) ([]*github.PullRequest, error)` — wraps `FindDependentPRs()`, purely informational

Each check method is independently testable. The workflow orchestrator calls them in sequence.

### 3.3 Merge module

**File:** `internal/land/merge.go`

Create types:
- `MergeExecutor` struct holding `*github.Client`, owner/name
- `MergeRequest` struct: the PR, merge method, commit title/message, whether to open editor

Create methods:
- `Execute(ctx, *MergeRequest) (*MergeResult, error)` — main entry point
- `prepareCommitMessage(pr *github.PullRequest, edit bool, isRebase bool) (title, body string, error)` — extracts PR title/body, optionally opens `$EDITOR`. If `edit` is true but `isRebase` is also true, skip editor with a warning ("--edit is ignored with --rebase")
- `openEditor(title, body string) (string, string, error)` — writes temp file, opens editor, parses result. Reuse `GetEditorCommand()` from `internal/template/template.go:627` for editor detection

The editor temp file format:
```
<PR title>

<PR body>
```
First non-empty line after parsing = title, rest = body. If the file is empty or unchanged, return an abort error.

### 3.4 Cleanup module

**File:** `internal/land/cleanup.go`

Create types:
- `PostMergeCleanup` struct holding `*git.Repository`, `*config.LandConfig`
- `CleanupResult` struct: `CheckedOut bool`, `Pulled bool`, `BranchDeleted bool`, `DeletedBranchSHA string`, `Warnings []string`

Create methods:
- `Execute(defaultBranch, featureBranch string, noDelete bool) (*CleanupResult, error)`
- `checkoutBranch(branch string) error` — runs `git checkout <branch>` via CLI
- `pullLatest(branch string) error` — runs `git pull origin <branch>` via CLI
- `deleteLocalBranch(branch string) (sha string, error)` — captures SHA via `git rev-parse`, then runs `git branch -D`

Each step catches errors and adds to `CleanupResult.Warnings` rather than failing the whole operation. The merge already succeeded.

### 3.5 Workflow orchestrator

**File:** `internal/land/workflow.go`

Create types:
- `LandWorkflow` struct: `*git.Repository`, `*github.Client`, `*config.Config`, owner, name, plus sub-components (`PreMergeChecker`, `MergeExecutor`, `PostMergeCleanup`, `OutputStyle`)
- `LandOptions` struct: `Squash bool`, `Rebase bool`, `Force bool`, `Edit bool`, `NoDelete bool`
- `LandResult` struct: `PR *github.PullRequest`, `MergeMethod string`, `MergeCommitSHA string`, `DefaultBranch string`, `DeletedBranch string`, `DeletedBranchSHA string`, `DependentPRCount int`, `CleanupWarnings []string`, `Messages []string`

Constructor: `NewLandWorkflow(repo, client, cfg, owner, name) *LandWorkflow`

Main method: `Execute(ctx, *LandOptions) (*LandResult, error)`

Sequence inside `Execute()`:
1. `checker.CheckCleanWorkingDir()` — fail if dirty
2. `repo.GetCurrentBranch()` + `checker.CheckNotOnTrunk()` — fail if on trunk
3. `checker.CheckPRExists(ctx, branch)` — fail if no PR; print `✓ Found PR #N: "title"`
4. `checker.CheckLocalHeadMatchesPR(pr)` — fail if local HEAD differs from PR head SHA (not bypassable)
5. `client.EnrichPullRequest()` — fetch reviews + checks in parallel
6. `checker.CheckApproval()` — print result, handle strict/prompt/force. Prompt mode: detect non-TTY and auto-decline with `--force` suggestion
7. `checker.CheckCI()` — print result, handle required/all/force
8. `checker.CheckDependentPRs()` — print warning if found
9. Resolve merge method: flag override → config default. If `--edit` and rebase: warn and skip editor
10. `merger.Execute()` — prepare commit message (skip editor for rebase), call merge API; print `✓ Squash-merged into main (sha)`
11. `cleanup.Execute()` — checkout, pull, delete; print each step
12. Return `LandResult`

Output is printed inline during execution (not buffered), so the user sees progress in real time.

### Verification

- `go test ./internal/land/...` passes
- Unit tests for each sub-module with mocked dependencies
- Checks module: test each check in isolation (pass, fail, force bypass, prompt mode)
- Merge module: test commit message preparation, editor abort
- Cleanup module: test each step success/failure independently
- Workflow: integration-style tests verifying the full sequence with mocked git/github

## Phase 4: Command Wiring

### 4.1 Cobra command

**File:** `cmd/land.go`

Follow the `cmd/diff.go` pattern:
- Define `landCmd` with `Use`, `Short`, `Long`, `RunE`
- Define flag variables: `landSquash`, `landRebase`, `landForce`, `landEdit`, `landNoDelete`
- Register flags in `init()`, mark `--squash`/`--rebase` mutually exclusive
- `runLand()` function:
  1. `context.Background()` (no global timeout, editor sessions can take time)
  2. Load config, get repo context, open git repo, create github client
  3. Create `LandWorkflow`, execute with `LandOptions`
  4. Handle specific error types with actionable messages (similar to how `cmd/diff.go` handles `ErrOperationCancelled`, `ErrStaleRemote`, etc.)
  5. Print final output

### 4.2 Register command

**File:** `cmd/land.go` → `init()`

Add `rootCmd.AddCommand(landCmd)` to register the land command.

### Verification

- `go build -o gh-arc` succeeds
- `./gh-arc land --help` shows correct usage, flags, and description
- `./gh-arc land` from a branch with no PR shows the expected error
- `go test ./cmd/...` passes (if cmd tests exist)

## Phase 5: Final Verification

### 5.1 Full test suite

- `go test ./...` — all tests pass
- `go vet ./...` — no issues
- `go fmt ./...` — properly formatted

### 5.2 Manual testing scenarios

Test from an actual repository with:
1. Happy path: approved PR, CI passing → merge, checkout main, delete branch
2. No approval + `--force` → bypasses and merges
3. Failing CI + `requireCI: "all"` → blocks with check names
4. Prompt mode: `requireApproval: "prompt"` → shows confirmation
5. `--edit` → opens editor with PR title/body
6. `--no-delete` → keeps local branch after merge
7. `--rebase` → uses rebase merge method
8. Dirty working directory → blocks with message
9. No PR for branch → fails with `gh arc diff` suggestion
10. Dependent PRs → shows warning, proceeds

### 5.3 Documentation review

- README config section reflects new enum values
- `gh arc land --help` text is clear and complete
