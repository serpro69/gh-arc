# Tasks: Testing Flags (--dry-run, --offline, --no-edit)

> Design: [./design.md](./design.md)
> Implementation: [./e2e-testing.md](./e2e-testing.md)
> Status: pending
> Created: 2026-03-11

## Task 1: Create execution context infrastructure
- **Status:** pending
- **Depends on:** —
- **Docs:** [design.md#phase-1-infrastructure](./design.md#phase-1-infrastructure)

### Subtasks
- [ ] 1.1 Add global `--dry-run` and `--offline` persistent flags to `cmd/root.go` via `rootCmd.PersistentFlags()`
- [ ] 1.2 Add `--no-edit`, `--title`, `--body`, `--test-plan`, `--reviewers` flags to `cmd/diff.go`
- [ ] 1.3 Create `internal/context/context.go` with `ExecutionContext` struct holding `DryRun`, `Offline`, `NoEdit` bools and the underlying `context.Context`
- [ ] 1.4 Create `internal/context/errors.go` with sentinel errors `ErrSkippedOffline` and `ErrSkippedDryRun`
- [ ] 1.5 Wire `ExecutionContext` construction from Cobra flags in `cmd/diff.go` `runDiff()` and pass it through `DiffWorkflow`

## Task 2: Wrap Git operations with dry-run/offline checks
- **Status:** pending
- **Depends on:** Task 1
- **Docs:** [design.md#phase-2-wrap-side-effects](./design.md#phase-2-wrap-side-effects)

### Subtasks
- [ ] 2.1 Update `internal/git/git.go` push methods to check `ExecutionContext.DryRun` (log `[DRY RUN] Would push branch: ...` and return nil) and `Offline` (log `[OFFLINE] Skipped push ...` and return `ErrSkippedOffline`)
- [ ] 2.2 Update checkout and branch creation methods in `internal/git/git.go` with dry-run guards (log planned operation, skip execution)
- [ ] 2.3 Update `internal/diff/auto_branch.go` `ExecuteAutoBranch()` and `PushWithRetry()` to respect `ExecutionContext` — skip push/checkout in dry-run, skip push in offline
- [ ] 2.4 Write unit tests for each wrapped git operation verifying no side effects in dry-run mode and local-only execution in offline mode

## Task 3: Wrap GitHub API operations with dry-run/offline checks
- **Status:** pending
- **Depends on:** Task 1
- **Docs:** [design.md#phase-2-wrap-side-effects](./design.md#phase-2-wrap-side-effects)

### Subtasks
- [ ] 3.1 Update `internal/diff/pr_executor.go` `CreateOrUpdatePR()` to check `ExecutionContext` — in dry-run, log the PR title/base/head/draft/reviewers that would be created and return a stub `PRResult`; in offline, return `ErrSkippedOffline`
- [ ] 3.2 Update `UpdateDraftStatus()` and `assignReviewers()` in `pr_executor.go` with the same dry-run/offline guards
- [ ] 3.3 Update `internal/diff/continue_mode.go` `Execute()` to respect dry-run/offline context when calling `prExecutor`
- [ ] 3.4 Write unit tests for PR executor dry-run output format and offline skip behavior

## Task 4: Implement --no-edit template bypass
- **Status:** pending
- **Depends on:** Task 1
- **Docs:** [design.md#1-no-edit-flag](./design.md#1---no-edit-flag)

### Subtasks
- [ ] 4.1 Update `internal/template/template.go` `OpenEditor()` to skip editor invocation when `NoEdit` is true — return the generated template content as-is
- [ ] 4.2 Add logic to merge explicit flag values (`--title`, `--body`, `--test-plan`, `--reviewers`) into the generated template before skipping the editor, overriding commit-extracted defaults
- [ ] 4.3 Add placeholder generation for required fields (Test Plan) when `--no-edit` is used and no explicit value provided — use `[Manual testing required]` default
- [ ] 4.4 Write tests: `--no-edit` returns template unchanged, explicit flags override defaults, placeholder fills required fields

## Task 5: Integrate execution context into diff workflow
- **Status:** pending
- **Depends on:** Task 2, Task 3, Task 4
- **Docs:** [design.md#phase-1-infrastructure](./design.md#phase-1-infrastructure)

### Subtasks
- [ ] 5.1 Update `internal/diff/workflow.go` `DiffWorkflow` struct to carry `ExecutionContext` and thread it through `Execute()`, `executeNormalMode()`, `executeFastPath()`, and `executeWithTemplateEditing()`
- [ ] 5.2 Update `internal/diff/output.go` to format dry-run and offline summary output — show planned operations list and, for offline mode, print manual follow-up commands
- [ ] 5.3 Wire flag precedence: `--dry-run` takes priority (nothing executed), then `--offline` (no network), then `--no-edit` (no editor)
- [ ] 5.4 Write integration-style tests exercising the full workflow in dry-run mode (verify no git/GitHub side effects) and offline mode (verify local branch created but no push)

## Task 6: E2E test infrastructure
- **Status:** pending
- **Depends on:** Task 5
- **Docs:** [e2e-testing.md](./e2e-testing.md)

### Subtasks
- [ ] 6.1 Create `tests/e2e/lib/` with `assertions.sh` (assert_contains, assert_equals, assert_not_contains), `setup.sh` (create temp git repos with simulated remote), and `utils.sh` (helpers for commits, configs)
- [ ] 6.2 Create `tests/e2e/test-dry-run.sh` covering: happy path on main with commits, feature branch normal diff, no changes error, pattern generation variants, invalid config error
- [ ] 6.3 Create `tests/e2e/test-offline.sh` covering: local branch creation, commit migration, collision detection and retry, uncommitted changes error, manual step output
- [ ] 6.4 Create `tests/e2e/test-no-edit.sh` covering: default extraction from commits, placeholder generation, explicit flag values, all flags combined
- [ ] 6.5 Create `tests/e2e/test-combined.sh` covering flag combinations: `--dry-run --no-edit`, `--offline --no-edit`, `--dry-run --offline --no-edit`

## Task 7: CI/CD integration and documentation
- **Status:** pending
- **Depends on:** Task 6
- **Docs:** [e2e-testing.md#cicd-integration](./e2e-testing.md#cicd-integration)

### Subtasks
- [ ] 7.1 Add Makefile targets: `test-e2e-dry-run`, `test-e2e-offline`, `test-e2e` (runs all E2E suites)
- [ ] 7.2 Create `.github/workflows/e2e-tests.yml` running dry-run and offline E2E tests on push/PR, with optional real-GitHub tests on main branch pushes using `E2E_TEST_TOKEN` secret
- [ ] 7.3 Update command help text in `cmd/root.go` and `cmd/diff.go` to document the new flags with examples

## Task 8: Final verification
- **Status:** pending
- **Depends on:** Task 1, Task 2, Task 3, Task 4, Task 5, Task 6, Task 7

### Subtasks
- [ ] 8.1 Run `testing-process` skill to verify all tasks — full test suite, integration tests, edge cases
- [ ] 8.2 Run `documentation-process` skill to update any relevant docs
