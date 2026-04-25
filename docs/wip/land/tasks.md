# Tasks: `gh arc land`

> Design: [./design.md](./design.md)
> Implementation: [./implementation.md](./implementation.md)
> Status: pending
> Created: 2026-04-25

## Task 1: Config changes
- **Status:** done
- **Depends on:** —
- **Docs:** [implementation.md#phase-1-config-changes](./implementation.md#phase-1-config-changes)

### Subtasks
- [x] 1.1 Update `LandConfig` in `internal/config/config.go`: change `RequireApproval` from `bool` to `string`, change `RequireCI` from `bool` to `string`, remove `DeleteRemoteBranch` field
- [x] 1.2 Update `setDefaults()`: set `land.requireApproval` to `"strict"`, `land.requireCI` to `"required"`, remove `land.deleteRemoteBranch` default
- [x] 1.3 Update `Validate()`: restrict `defaultMergeMethod` to `{"squash", "rebase"}`, add validation for `requireApproval` enum (`"strict"`, `"prompt"`, `"none"`), add validation for `requireCI` enum (`"required"`, `"all"`, `"none"`)
- [x] 1.4 Update existing config tests in `internal/config/config_test.go` for the type changes, add test cases for new enum validation
- [x] 1.5 Update README.md config examples (JSON and YAML) to reflect new `LandConfig` fields and removal of `deleteRemoteBranch`

→ verify: `go test ./internal/config/...` passes, invalid enum values rejected by `Validate()`

## Task 2: GitHub API — merge and branch protection methods
- **Status:** done
- **Depends on:** —
- **Docs:** [implementation.md#phase-2-github-api--merge-method](./implementation.md#phase-2-github-api--merge-method)

### Subtasks
- [x] 2.1 Add `MergeOptions` and `MergeResult` types to `internal/github/pullrequest.go`
- [x] 2.2 Add `MergePullRequest(ctx, owner, repo, number, *MergeOptions) (*MergeResult, error)` method — `PUT /repos/{owner}/{repo}/pulls/{number}/merge` with semantic error mapping (405 → method not allowed, 409 → conflicts, 422 → not mergeable)
- [x] 2.3 Add `MergePullRequestForCurrentRepo()` convenience wrapper
- [x] 2.4 Add `GetRequiredStatusChecks(ctx, owner, repo, branch) ([]string, error)` — `GET /repos/{owner}/{repo}/branches/{branch}/protection/required_status_checks`, graceful fallback on 404/403
- [x] 2.5 Write unit tests with `httptest` server: merge success, 405, 409, 422 responses; required status checks 200, 404, 403 responses

→ verify: `go test ./internal/github/...` passes

## Task 3: Land output module
- **Status:** done
- **Depends on:** —
- **Docs:** [implementation.md#31-output-module](./implementation.md#31-output-module)

### Subtasks
- [x] 3.1 Create `internal/land/output.go` with `OutputStyle` struct following `internal/diff/output.go` pattern
- [x] 3.2 Implement `PrintStep(icon, message)` and `PrintDetail(message)` base methods for `✓`/`✗`/`⚠` prefixed output
- [x] 3.3 Add helper methods: `PrintPRFound()`, `PrintApprovalStatus()`, `PrintCIStatus()`, `PrintDependentPRs()`, `PrintMerged()`, `PrintCheckout()`, `PrintBranchDeleted()`, `PrintCleanupWarning()`
- [x] 3.4 Add `FormatLandResult(*LandResult) string` for final summary formatting
- [x] 3.5 Write tests for output formatting

→ verify: `go test ./internal/land/...` passes

## Task 4: Land checks module
- **Status:** pending
- **Depends on:** Task 1, Task 2
- **Docs:** [implementation.md#32-checks-module](./implementation.md#32-checks-module)

### Subtasks
- [ ] 4.1 Create `internal/land/checks.go` with `PreMergeChecker` struct and `CheckResult` type
- [ ] 4.2 Implement `CheckCleanWorkingDir()` — uses `repo.GetWorkingDirectoryStatus()`, always blocks if dirty
- [ ] 4.3 Implement `CheckNotOnTrunk(currentBranch, defaultBranch)` — compares branch names
- [ ] 4.4 Implement `CheckPRExists(ctx, branchName)` — wraps `FindExistingPRForCurrentBranch()`
- [ ] 4.5 Implement `CheckApproval(ctx, *PullRequest, force)` — evaluates reviews per `requireApproval` config (`"strict"`, `"prompt"`, `"none"`), returns `CheckResult` with `NeedsConfirmation` for prompt mode
- [ ] 4.6 Implement `CheckCI(ctx, *PullRequest, force)` — evaluates checks per `requireCI` config; when `"required"`, calls `GetRequiredStatusChecks()` and filters to only those checks
- [ ] 4.7 Implement `CheckDependentPRs(ctx, branchName)` — wraps `FindDependentPRs()`, informational only
- [ ] 4.8 Write unit tests for each check method: pass, fail, force bypass, prompt mode, each config enum value

→ verify: `go test ./internal/land/...` passes

## Task 5: Land merge module
- **Status:** pending
- **Depends on:** Task 2
- **Docs:** [implementation.md#33-merge-module](./implementation.md#33-merge-module)

### Subtasks
- [ ] 5.1 Create `internal/land/merge.go` with `MergeExecutor` struct and `MergeRequest` type
- [ ] 5.2 Implement `prepareCommitMessage(pr, edit)` — extracts PR title/body, returns directly for non-edit mode
- [ ] 5.3 Implement `openEditor(title, body)` — writes temp file, opens `$EDITOR` (reuse editor detection from `internal/template/`), parses result (first line = title, rest = body), aborts on empty/unchanged
- [ ] 5.4 Implement `Execute(ctx, *MergeRequest)` — coordinates commit message prep and `client.MergePullRequest()` call
- [ ] 5.5 Write unit tests: successful merge, editor abort, merge API errors

→ verify: `go test ./internal/land/...` passes

## Task 6: Land cleanup module
- **Status:** pending
- **Depends on:** —
- **Docs:** [implementation.md#34-cleanup-module](./implementation.md#34-cleanup-module)

### Subtasks
- [ ] 6.1 Create `internal/land/cleanup.go` with `PostMergeCleanup` struct and `CleanupResult` type
- [ ] 6.2 Implement `checkoutBranch(branch)` — runs `git checkout` via CLI
- [ ] 6.3 Implement `pullLatest(branch)` — runs `git pull origin` via CLI
- [ ] 6.4 Implement `deleteLocalBranch(branch)` — captures SHA via `git rev-parse`, runs `git branch -D`, returns SHA for restore message
- [ ] 6.5 Implement `Execute(defaultBranch, featureBranch, noDelete)` — runs steps sequentially, catches errors as warnings in `CleanupResult.Warnings`
- [ ] 6.6 Write unit tests: each step success/failure, `--no-delete` skips deletion, failures are non-fatal

→ verify: `go test ./internal/land/...` passes

## Task 7: Land workflow orchestrator
- **Status:** pending
- **Depends on:** Task 3, Task 4, Task 5, Task 6
- **Docs:** [implementation.md#35-workflow-orchestrator](./implementation.md#35-workflow-orchestrator)

### Subtasks
- [ ] 7.1 Create `internal/land/workflow.go` with `LandWorkflow`, `LandOptions`, and `LandResult` types
- [ ] 7.2 Implement `NewLandWorkflow(repo, client, cfg, owner, name)` constructor — creates sub-components
- [ ] 7.3 Implement `Execute(ctx, *LandOptions)` — the full 10-step sequence: check clean WD → check not on trunk → find PR → enrich PR → check approval → check CI → check dependent PRs → resolve merge method → execute merge → run cleanup
- [ ] 7.4 Add inline output printing during execution (progress steps printed in real time, not buffered)
- [ ] 7.5 Add prompt handling for `requireApproval: "prompt"` — read stdin for `y/N` confirmation; detect non-TTY (`term.IsTerminal(int(os.Stdin.Fd()))`) and auto-decline with message suggesting `--force`
- [ ] 7.6 Write integration-style tests with mocked git/github: full happy path, force bypass, prompt mode, merge failure, cleanup failure (non-fatal)

→ verify: `go test ./internal/land/...` passes

## Task 8: Cobra command wiring
- **Status:** pending
- **Depends on:** Task 7
- **Docs:** [implementation.md#phase-4-command-wiring](./implementation.md#phase-4-command-wiring)

### Subtasks
- [ ] 8.1 Create `cmd/land.go` with `landCmd` Cobra command — `Use`, `Short`, `Long` (with usage examples), `RunE: runLand`
- [ ] 8.2 Define flag variables (`landSquash`, `landRebase`, `landForce`, `landEdit`, `landNoDelete`) and register in `init()`, mark `--squash`/`--rebase` mutually exclusive
- [ ] 8.3 Implement `runLand()` — load config, get repo context, open git repo, create client, create `LandWorkflow`, execute, handle error types with actionable messages
- [ ] 8.4 Register command: `rootCmd.AddCommand(landCmd)` in `init()`

→ verify: `go build -o gh-arc` succeeds, `./gh-arc land --help` shows correct usage and flags

## Task 9: Final verification
- **Status:** pending
- **Depends on:** Task 1, Task 2, Task 3, Task 4, Task 5, Task 6, Task 7, Task 8

### Subtasks
- [ ] 9.1 Run `test` skill to verify all tasks — full test suite, `go vet`, `go fmt`
- [ ] 9.2 Run `document` skill to update any relevant docs
- [ ] 9.3 Run `review-code` skill with Go input to review the implementation
- [ ] 9.4 Run `review-spec` skill to verify implementation matches design and implementation docs
