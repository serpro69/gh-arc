# Tasks: `gh arc lint` and `gh arc unit`

> Design: [./design.md](./design.md)
> Implementation: [./implementation.md](./implementation.md)
> Status: pending
> Created: 2026-05-18

---

## Phase 1: Shared Runner Engine + Lint Command

### Task 1: Shared runner engine types and execution

- **Status:** pending
- **Depends on:** —
- **Docs:** [implementation.md#11-shared-runner-engine](./implementation.md#11-shared-runner-engine-internalrunner)

#### Subtasks

- [ ] 1.1 Create `internal/runner/types.go` with `RunnerConfig`, `RunResult` (with Status string constants: Passed, Failed, Error, Skipped), `ExecutionResult` (with `FailedCount()` helper), `EngineOptions` (JSONMode, Verbose, Stdout/Stderr as `io.Writer`), and `Executor` interface with `Run(ctx context.Context, configs []RunnerConfig) (*ExecutionResult, error)` method — `Engine` satisfies this interface, enabling mock injection in workflow tests
- [ ] 1.2 Create `internal/runner/engine.go` with `Engine` struct and `NewEngine(opts EngineOptions) *Engine` constructor. Implement `Run(ctx context.Context, configs []RunnerConfig) (*ExecutionResult, error)` — sequential execution, arg building (Args + ExtraArgs + FilePaths), `exec.CommandContext` with optional timeout, exit code classification (0→Passed, ErrNotFound→Error, other→Failed), timeout detection via `ctx.Err()`
- [ ] 1.3 Create `internal/runner/output.go` with `PrintBanner`, `PrintResult`, `PrintSummary`, and `FormatJSON` functions. Follow `internal/land/output.go` patterns (✓/✗/⚠ indicators). All functions write to injected `io.Writer`, not `os.Stdout` directly
- [ ] 1.4 Write tests in `internal/runner/engine_test.go`: runner exits 0 → Passed; runner exits 1 → Failed; non-existent command → Error; timeout → Error; multiple runners with mixed results → correct aggregate; JSON mode produces valid JSON; file paths appended to args; ExtraArgs appended between Args and FilePaths; empty config list → success with no runners

→ verify: `go test ./internal/runner/... -v` passes, all cases green

### Task 2: Config changes for fixArgs

- **Status:** pending
- **Depends on:** —
- **Docs:** [implementation.md#12-config-changes](./implementation.md#12-config-changes)

#### Subtasks

- [ ] 2.1 Add `FixArgs []string \`mapstructure:"fixArgs"\`` and `Timeout string \`mapstructure:"timeout"\`` fields to `LintRunner` in `internal/config/config.go`
- [ ] 2.2 Add `fixArgs` (array of strings) and `timeout` (string, e.g. "30s", "5m") properties to the `lintRunner` definition in `docs/arc.schema.json`
- [ ] 2.3 Add test cases in `internal/config/config_test.go` that load a config with `fixArgs` and `timeout` populated and verify they deserialize into `LintRunner.FixArgs` and `LintRunner.Timeout` correctly

→ verify: `go test ./internal/config/... -v` passes, existing tests unbroken

### Task 3: Lint workflow

- **Status:** pending
- **Depends on:** Task 1, Task 2
- **Docs:** [implementation.md#13-lint-workflow](./implementation.md#13-lint-workflow-internallint)

#### Subtasks

- [ ] 3.1 Create `internal/lint/types.go` with `LintOptions` (Fix, All, JSONMode, Verbose bools) and `LintResult` (wrapping `runner.ExecutionResult`, adding ChangedFileCount int and AllMode bool)
- [ ] 3.2 Define `LintRepository` interface in `internal/lint/workflow.go` with methods: `GetDefaultBranch() (string, error)`, `GetMergeBase(ref1, ref2 string) (string, error)`, `GetFilesChanged(base, head string) ([]git.FileChange, error)`. The real `git.Repository` satisfies this — `GetFilesChanged` returns `[]git.FileChange` with `IsDeleted` metadata needed for filtering
- [ ] 3.3 Implement `LintWorkflow` struct (fields: repo LintRepository, executor runner.Executor, config *config.Config) with `NewLintWorkflow(repo, executor, cfg)` constructor and `Execute(ctx, *LintOptions) (*LintResult, error)` method implementing the 5-step flow: resolve runners → detect changed files (via `GetFilesChanged`, filter `IsDeleted`, extract paths) → build runner configs → execute via injected executor
- [ ] 3.4 Implement merge-base resolution with fallback: try `origin/<defaultBranch>` first, fall back to local `<defaultBranch>`, error with "fetch origin" suggestion if both fail. Handle orphan branch case by falling back to --all with a warning
- [ ] 3.5 Implement `--fix` logic: when `opts.Fix` is true, for each runner where `AutoFix == true` and `FixArgs` is non-empty, set `RunnerConfig.ExtraArgs` to the runner's `FixArgs`
- [ ] 3.6 Implement "no runners" guidance: when no runners configured and MegaLinter not enabled, print config examples showing how to add runners and how to enable MegaLinter, then return success result with zero runners
- [ ] 3.7 Write tests in `internal/lint/workflow_test.go` using mock LintRepository and mock runner.Executor: no runners → guidance + success; runners + changed files → executor receives correct file paths (deleted files excluded); --all → no file paths passed; --fix with autoFix=true → ExtraArgs set; --fix with autoFix=false → ExtraArgs empty; no changed files → early exit; merge-base origin fallback; merge-base failure; JSON mode suppresses guidance messages

→ verify: `go test ./internal/lint/... -v` passes

### Task 4: Lint CLI command

- **Status:** pending
- **Depends on:** Task 3
- **Docs:** [implementation.md#14-cli-command](./implementation.md#14-cli-command-cmdlintgo)

#### Subtasks

- [ ] 4.1 Define `ErrSilentExit` sentinel error in `cmd/` (e.g. in `cmd/errors.go` or `cmd/root.go`) and update `Execute()` in `cmd/root.go` to check `errors.Is(err, ErrSilentExit)` — if true, exit 1 without printing the error message
- [ ] 4.2 Create `cmd/lint.go` following `cmd/land.go` pattern: package-level flag vars (`lintFix`, `lintAll`), `lintCmd` with Use/Short/Long/Args(cobra.NoArgs)/RunE, `init()` registering flags and adding to `rootCmd`
- [ ] 4.3 Implement `runLint(cmd, args)`: load config, open git repo, create `runner.Engine` + `LintWorkflow`, read `cmd.GetJSON()` and `cmd.GetVerbose()` into options, call Execute, return `ErrSilentExit` on lint failure (summary already printed), print JSON via `runner.FormatJSON()` in JSON mode
- [ ] 4.4 Write `cmd/lint_test.go`: test flag parsing (--fix, --all), test that unexpected positional args are rejected (cobra.NoArgs enforcement), test that ErrSilentExit is returned on lint failure

→ verify: `go test ./cmd/... -v` passes; `go build -o gh-arc && ./gh-arc lint --help` shows correct usage; running `./gh-arc lint` with no config prints guidance and exits 0 (not 1)

---

## Phase 2: Unit Command

### Task 5: Unit workflow

- **Status:** pending
- **Depends on:** Task 1
- **Docs:** [implementation.md#21-unit-workflow](./implementation.md#21-unit-workflow-internalunit)

#### Subtasks

- [ ] 5.1 Create `internal/unit/types.go` with `UnitOptions` (JSONMode, Verbose bools) and `UnitResult` (wrapping `runner.ExecutionResult`)
- [ ] 5.2 Implement `UnitWorkflow` struct (fields: executor runner.Executor, config *config.Config) with `NewUnitWorkflow(executor, cfg)` constructor and `Execute(ctx, *UnitOptions) (*UnitResult, error)` method: resolve runners → parse timeouts → build runner configs → execute via injected executor
- [ ] 5.3 Implement timeout parsing: `time.ParseDuration(runner.Timeout)` with clear error on invalid format (e.g., "invalid timeout '5min' for runner 'go-test': use Go duration format like '5m' or '300s'")
- [ ] 5.4 Implement "no runners" guidance message (same pattern as lint but with test runner config examples)
- [ ] 5.5 Write tests in `internal/unit/workflow_test.go` using mock runner.Executor: no runners → guidance + success; runners with valid timeout → parsed correctly and executor receives correct RunnerConfig.Timeout; invalid timeout → descriptive error; executor receives correct configs; JSON mode suppresses guidance

→ verify: `go test ./internal/unit/... -v` passes

### Task 6: Unit CLI command

- **Status:** pending
- **Depends on:** Task 5
- **Docs:** [implementation.md#22-cli-command](./implementation.md#22-cli-command-cmdunitgo)

#### Subtasks

- [ ] 6.1 Create `cmd/unit.go` following `cmd/land.go` pattern: `unitCmd` with Use/Short/Long/Args(cobra.NoArgs)/RunE, `init()` adding to `rootCmd`. No command-specific flags
- [ ] 6.2 Implement `runUnit(cmd, args)`: load config, create `runner.Engine` + `UnitWorkflow` (no git repo needed), read `cmd.GetJSON()` and `cmd.GetVerbose()` into options, call Execute, return `ErrSilentExit` on test failure, print JSON via `runner.FormatJSON()` in JSON mode
- [ ] 6.3 Write `cmd/unit_test.go`: test that unexpected positional args are rejected

→ verify: `go test ./cmd/... -v` passes; `go build -o gh-arc && ./gh-arc unit --help` shows correct usage

---

## Final Verification

### Task 7: Integration verification and documentation

- **Status:** pending
- **Depends on:** Task 1, Task 2, Task 3, Task 4, Task 5, Task 6

#### Subtasks

- [ ] 7.1 Run `test` skill to verify full test suite passes: `go test ./...`
- [ ] 7.2 Run `document` skill to update README.md command list and configuration docs with lint/unit sections
- [ ] 7.3 Run `review-code` skill with Go input to review the implementation
- [ ] 7.4 Run `review-spec` skill to verify implementation matches design.md and implementation.md
