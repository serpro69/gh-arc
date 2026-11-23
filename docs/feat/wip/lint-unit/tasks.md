# Tasks: `gh arc lint` and `gh arc unit`

> Design: [./design.md](./design.md)
> Implementation: [./implementation.md](./implementation.md)
> Status: pending
> Created: 2026-05-18

---

## Phase 1: Shared Runner Engine + Lint Command

### Task 1: Shared runner engine types and execution

- **Status:** done
- **Depends on:** —
- **Docs:** [implementation.md#11-shared-runner-engine](./implementation.md#11-shared-runner-engine-internalrunner)

#### Subtasks

- [x] 1.1 Create `internal/runner/types.go` with `RunnerConfig`, `RunResult` (with Status string constants: Passed, Failed, Error, Skipped), `ExecutionResult` (with `FailedCount()` helper and `SkipReason string` for cases like "no changed files"), `EngineOptions` (JSONMode, Verbose, Stdout/Stderr as `io.Writer`), and `Executor` interface with `Run(ctx context.Context, configs []RunnerConfig) (*ExecutionResult, error)` method — `Engine` satisfies this interface, enabling mock injection in workflow tests
- [x] 1.2 Create `internal/runner/engine.go` with `Engine` struct and `NewEngine(opts EngineOptions) *Engine` constructor. Implement `Run(ctx context.Context, configs []RunnerConfig) (*ExecutionResult, error)` — sequential execution, arg building (Args + ExtraArgs + FilePaths), `exec.CommandContext` with optional timeout, exit code classification (0→Passed, ErrNotFound→Error, other→Failed), timeout detection via `ctx.Err()`
- [x] 1.3 Create `internal/runner/output.go` with `PrintBanner`, `PrintResult`, `PrintSummary`, and `FormatJSON` functions. `FormatJSON` includes `"skipped"` field when `ExecutionResult.SkipReason` is non-empty. Follow `internal/land/output.go` patterns (✓/✗/⚠ indicators). All functions write to injected `io.Writer`, not `os.Stdout` directly
- [x] 1.4 Write tests in `internal/runner/engine_test.go`: runner exits 0 → Passed; runner exits 1 → Failed; non-existent command → Error; timeout → Error; multiple runners with mixed results → correct aggregate; JSON mode produces valid JSON; file paths appended to args; ExtraArgs appended between Args and FilePaths; empty config list → success with no runners

→ verify: `go test ./internal/runner/... -v` passes, all cases green

### Task 2: Config changes for fixArgs

- **Status:** done
- **Depends on:** —
- **Docs:** [implementation.md#12-config-changes](./implementation.md#12-config-changes)

#### Subtasks

- [x] 2.1 Add `FixArgs []string \`mapstructure:"fixArgs"\`` and `Timeout string \`mapstructure:"timeout"\`` fields to `LintRunner` in `internal/config/config.go`
- [x] 2.2 Add `fixArgs` (array of strings) and `timeout` (string, e.g. "30s", "5m") properties to the `lintRunner` definition in `docs/arc.schema.json`
- [x] 2.3 Add test cases in `internal/config/config_test.go` that load a config with `fixArgs` and `timeout` populated and verify they deserialize into `LintRunner.FixArgs` and `LintRunner.Timeout` correctly

→ verify: `go test ./internal/config/... -v` passes, existing tests unbroken

### Task 3: Lint workflow

- **Status:** done
- **Depends on:** Task 1, Task 2
- **Docs:** [implementation.md#13-lint-workflow](./implementation.md#13-lint-workflow-internallint)

#### Subtasks

- [x] 3.1 Create `internal/lint/types.go` with `LintOptions` (Fix, All, JSONMode, Verbose bools) and `LintResult` (wrapping `runner.ExecutionResult`, adding ChangedFileCount int and AllMode bool)
- [x] 3.2 Define `LintRepository` interface in `internal/lint/workflow.go` with methods: `Path() string`, `GetLocalDefaultBranch() (string, error)`, `GetMergeBase(ref1, ref2 string) (string, error)`, `GetFilesChanged(base, head string) ([]git.FileChange, error)`. The real `git.Repository` satisfies this — `GetFilesChanged` returns `[]git.FileChange` with `IsDeleted` metadata needed for filtering
- [x] 3.3 Implement `LintWorkflow` struct (fields: repo LintRepository, executor runner.Executor, config *config.Config) with `NewLintWorkflow(repo, executor, cfg)` constructor and `Execute(ctx, *LintOptions) (*LintResult, error)` method implementing the 5-step flow: resolve runners (suppress guidance in JSON mode) → detect changed files (via `GetFilesChanged`, filter `IsDeleted`, extract paths; set `SkipReason` on result if no files) → build runner configs (parse Timeout like unit workflow, normalize file paths relative to runner WorkingDir) → execute via injected executor
- [x] 3.4 Implement merge-base resolution with two distinct failure modes: (1) ref resolution — try `origin/<defaultBranch>` first, fall back to local `<defaultBranch>`, error with "fetch origin" suggestion if neither ref exists; (2) no common ancestor — if both refs resolve but `GetMergeBase` fails (orphan branch), fall back to linting all files with a `⚠` warning
- [x] 3.5 Implement `--fix` logic: when `opts.Fix` is true, for each runner where `AutoFix == true` and `FixArgs` is non-empty, set `RunnerConfig.ExtraArgs` to the runner's `FixArgs`
- [x] 3.6 Implement "no runners" guidance: when no runners configured and MegaLinter not enabled, print config examples (only if not JSON mode), then return success result with zero runners. In JSON mode, return empty result silently — FormatJSON handles the output
- [x] 3.7 Write tests in `internal/lint/workflow_test.go` using mock LintRepository and mock runner.Executor: no runners → guidance + success; runners + changed files → executor receives correct file paths (deleted files excluded); --all → no file paths passed; --fix with autoFix=true → ExtraArgs set; --fix with autoFix=false → ExtraArgs empty; no changed files → early exit with SkipReason set; merge-base ref fallback; merge-base orphan fallback to --all; invalid lint timeout → error; valid lint timeout → RunnerConfig.Timeout set; JSON mode suppresses guidance messages; workingDir path normalization (file outside workingDir skipped, file inside adjusted)

→ verify: `go test ./internal/lint/... -v` passes

#### Execution context

- 2026-11-23 — Observed: implementation starts from `57bbea2eaa375b4004689c03faa4043c36b77f26`; Tasks 1–2 are complete. Scope is Task 3 only; CLI wiring remains Task 4. The pre-existing untracked `.codex/scripts/guard/__pycache__/` is unrelated.
- 2026-11-23 — Observed: `git.Repository.GetMergeBase` returns untyped errors. Add a no-common-ancestor sentinel and a ref-resolution error so lint can distinguish missing default refs from unrelated history and other git failures without matching error text. Existing successful merge-base behavior remains unchanged.
- 2026-11-23 — Implementation detail: extend `LintRepository` with the existing `Path()` method to locate the repository root, resolve runner working directories from that root, and support invocation from subdirectories. Inject workflow guidance output through an optional constructor option. No new external dependencies are needed.
- 2026-11-23 — Inferred from changed-file scoping: omit a runner when no changed files remain inside its working directory; executing it with empty file arguments would unexpectedly lint everything. Explicit/all-mode fallback still executes every runner without file arguments.
- 2026-11-23 — Verification planned: mock-based workflow tests, real-repository merge-base error classification, then the full Go suite and vet because the git helper is shared. No deployment or migration is involved; CLI behavior cannot be exercised until Task 4.
- 2026-11-23 — Verified: `go test ./internal/lint/... -v` and `go test -tags=integration ./internal/git -run '^TestGetMergeBaseErrorClassification$' -v` passed (captured run `ddbabe30-a061-4738-961c-94d5c6b33c96`). `TMPDIR=/private/tmp go test ./...`, `go vet ./...`, and `go test -race ./internal/lint` passed (run `31ece013-6ccf-4634-b622-89f21d218009`). The first full-suite attempt failed only the existing config-test path comparisons described under Task 7; vet/race did not run in that failed attempt.
- 2026-11-23 — Integration note for Task 4: create the engine with matching JSON/verbose flags and output writers; pass `lint.WithOutput(cmd.OutOrStdout())` for workflow guidance. The workflow resolves relative runner working directories against the repository root and returns effective `AllMode` after an orphan-history fallback. Only the CLI emits JSON via `runner.FormatJSON`.
- 2026-11-23 — Review correction: the existing `GetDefaultBranch()` contacts origin, conflicting with the design's local-only lint behavior. `LintRepository` uses `GetLocalDefaultBranch()` instead of the planned getter; it exposes the existing local CLI lookup (cached origin/HEAD, then local branch names). This interface adjustment preserves the specified offline behavior and leaves diff/land's existing lookup unchanged. Integration coverage checks local fallback and a cached custom default with an unavailable remote.
- 2026-11-23 — Review correction: `config.Load()` previously filled an omitted `github.defaultBranch` with `main`, preventing the planned autodetection. The default is now empty; explicit configured branches retain precedence. Repository search found no other production consumer of this field (diff/land detect independently). Updated the schema, README, config expectation and testing example; added a real-loader workflow regression for omitted/master and explicit/main cases.
- 2026-11-23 — Review follow-up: CI now runs `go test -tags=integration ./...` so the real-git regression is exercised. `LintResult` documents omitted unaffected runners; the orphan warning explicitly refers to local, possibly shallow history. These clarify scope without adding skipped runner output or shallow-clone special handling.
- 2026-11-23 — Final verification: after the review corrections, `TMPDIR=/private/tmp go test -tags=integration ./...`, `go vet ./...`, and `go test -race ./internal/lint` all passed in captured run `b1fe826b-d689-45b4-9a8c-b571861c660f` (exit 0, complete capture). `git diff --check` passed. Hosted CI and actual linter/CLI invocation were not run; Task 4 remains pending.
- 2026-11-23 — Isolated review: internal `code-reviewer` (GPT-6.1 Sol) approved the final 13-file increment; PAL `clink`/Claude also approved after its config-default finding was corrected. Backend: clink, configured model unknown, observed `claude-opus-5-5` (Anthropic; initially inferred from the shipped client mapping). Codex was excluded as the authoring provider; agy was unavailable; gemini was retired. The external review executed, but PAL removed read-event traces and denied an optional shell inspection, so source receipt is unverified and no corroboration is claimed. Reviews were source-only; test execution is the author's evidence above. No systemic findings to index.

### Task 4: Lint CLI command

- **Status:** pending
- **Depends on:** Task 3
- **Docs:** [implementation.md#14-cli-command](./implementation.md#14-cli-command-cmdlintgo)

#### Subtasks

- [ ] 4.1 Define `ErrSilentExit` sentinel error in `cmd/` (e.g. in `cmd/errors.go` or `cmd/root.go`) and update `Execute()` in `cmd/root.go` to check `errors.Is(err, ErrSilentExit)` — if true, exit 1 without printing the error message
- [ ] 4.2 Create `cmd/lint.go` following `cmd/land.go` pattern: package-level flag vars (`lintFix`, `lintAll`), `lintCmd` with Use/Short/Long/Args(cobra.NoArgs)/RunE, `init()` registering flags and adding to `rootCmd`
- [ ] 4.3 Implement `runLint(cmd, args)`: load config, open git repo, create `runner.Engine` + `LintWorkflow`, read `GetJSON()` and `GetVerbose()` (package-level helpers in `cmd/`) into options, call Execute, return `ErrSilentExit` on lint failure (summary already printed), print JSON via `runner.FormatJSON()` in JSON mode
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
- [ ] 5.4 Implement "no runners" guidance message (same pattern as lint — only print if not JSON mode, with test runner config examples)
- [ ] 5.5 Write tests in `internal/unit/workflow_test.go` using mock runner.Executor: no runners → guidance + success; runners with valid timeout → parsed correctly and executor receives correct RunnerConfig.Timeout; invalid timeout → descriptive error; executor receives correct configs; JSON mode suppresses guidance

→ verify: `go test ./internal/unit/... -v` passes

### Task 6: Unit CLI command

- **Status:** pending
- **Depends on:** Task 5
- **Docs:** [implementation.md#22-cli-command](./implementation.md#22-cli-command-cmdunitgo)

#### Subtasks

- [ ] 6.1 Create `cmd/unit.go` following `cmd/land.go` pattern: `unitCmd` with Use/Short/Long/Args(cobra.NoArgs)/RunE, `init()` adding to `rootCmd`. No command-specific flags
- [ ] 6.2 Implement `runUnit(cmd, args)`: load config, create `runner.Engine` + `UnitWorkflow` (no git repo needed), read `GetJSON()` and `GetVerbose()` into options, call Execute, return `ErrSilentExit` on test failure, print JSON via `runner.FormatJSON()` in JSON mode
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

#### Execution context

- 2026-11-23 — Observed, deferred (owner: unassigned): two existing `TestGetMegaLinterConfigPath` cases compare temporary paths literally after `os.Chdir`; on macOS, `/var/...` and `/private/var/...` name the same location but fail those assertions. Captured run `5e81347c-7471-4bf4-89d2-885fbf8b7a09` records the failures. This test-portability fix is outside Task 3; use `TMPDIR=/private/tmp` for current verification. Next action: canonicalize fixture/expected paths in those cases, then verify the config suite with a symlinked temporary root as well as a canonical one. Do not weaken file-location assertions.
- 2026-11-23 — External-review follow-up (owner: unassigned, Task 7 docs): the inherited local default-branch helper selects the first local branch when cached origin/HEAD and common local defaults are absent. Source inspection suggests this can select an unpushed feature branch and yield an empty self-diff; detached CI checkouts without local refs instead fail detection. No new guard is included in Task 3, whose existing lookup strategy is retained. Document explicit `github.defaultBranch` configuration or populating cached origin/HEAD; verify with a fixture lacking both cached HEAD and common local defaults before deciding whether a guard is needed. Absolute workingDir paths involving symlink aliases and shallow-history exit behavior were not independently exercised.
- 2026-11-23 — Optional external-review cleanup (owner: unassigned): several long workflow/test lines and the illustrative `main` fallback in `CONTRIBUTING.md` remain. Deferred as nonblocking style/example work; inspect and adjust when editing those examples, preserving their behavior. The workflow's omitted-runner behavior is documented, and adding per-runner skipped output is outside this task.
