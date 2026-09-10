# Implementation Plan: `gh arc lint` and `gh arc unit`

> Design: [./design.md](./design.md)
> Status: ready
> Created: 2026-05-18

## Phasing

- **Phase 1**: Shared runner engine (`internal/runner/`) + `gh arc lint` command
- **Phase 2**: `gh arc unit` command (reuses the engine)

Phase 1 delivers the core infrastructure and the higher-priority lint command. Phase 2 is a thin wrapper that leverages the engine built in Phase 1.

---

## Phase 1: Runner Engine + Lint Command

### 1.1 Shared Runner Engine (`internal/runner/`)

Create `internal/runner/` with three files: `engine.go`, `types.go`, `output.go`.

#### `types.go` — Data Types

Define the following types:

- **`RunnerConfig`**: Name (string), Command (string), Args ([]string), ExtraArgs ([]string — appended to Args at execution time, used for --fix), WorkingDir (string), Timeout (time.Duration), FilePaths ([]string — changed files appended to command args).
- **`RunResult`**: Name (string), Status (enum: Passed, Failed, Error, Skipped), ExitCode (int), Duration (time.Duration), Err (error — only set for Status=Error, e.g. command not found).
- **`ExecutionResult`**: Runners ([]RunResult), Success (bool — true iff all runners passed), SkipReason (string — optional, set when execution was skipped e.g. "no changed files"). Add a helper method `FailedCount() int`.
- **`EngineOptions`**: JSONMode (bool), Verbose (bool), Stdout (io.Writer), Stderr (io.Writer) — allow injection for testing.

Status should be a string type with constants, not iota, so it serializes cleanly to JSON.

#### `engine.go` — Runner Execution

The `Engine` struct holds `EngineOptions`. Constructor: `NewEngine(opts EngineOptions) *Engine`.

Method `Run(ctx context.Context, configs []RunnerConfig) (*ExecutionResult, error)`:

1. If `len(configs) == 0`, return an empty `ExecutionResult` (success=true, no runners). The caller (lint/unit workflow) handles the "no runners configured" messaging.
2. Iterate configs sequentially. For each:
   a. Build the full arg list: `config.Args + config.ExtraArgs + config.FilePaths`.
   b. Create `exec.CommandContext`. If `config.Timeout > 0`, wrap `ctx` with `context.WithTimeout`. Set `cmd.Dir` to `config.WorkingDir` if non-empty.
   c. In normal mode: set `cmd.Stdout` and `cmd.Stderr` to `engine.Stdout/Stderr` (defaults to `os.Stdout/os.Stderr`). Print banner before execution.
   d. In JSON mode: capture stdout/stderr into buffers (don't stream). Suppress banners.
   e. Run the command, measure duration with `time.Now()` before/after.
   f. Classify result: exit code 0 → Passed; `exec.ErrNotFound` → Error (command not found); any other non-zero → Failed.
   g. Print per-runner summary line (normal mode only).
3. Build and return `ExecutionResult`.

Handle the `exec.ErrNotFound` case explicitly — a missing binary is an Error, not a lint failure. The summary should distinguish: `✗ golangci-lint: command not found` vs `✗ golangci-lint: failed (exit code 1)`.

Handle timeout via context cancellation: if `ctx.Err() == context.DeadlineExceeded`, set status to Error with message "timed out after Xs".

#### `output.go` — Formatting

Formatting functions used by the engine:

- `PrintBanner(w io.Writer, name string)` — prints `▶ Running <name>...` with a newline.
- `PrintResult(w io.Writer, result RunResult)` — prints the ✓/✗ summary line with duration.
- `PrintSummary(w io.Writer, result ExecutionResult)` — prints the overall summary: `━━━` separator + `✓ N runners passed` or `✗ M of N runners failed`.
- `FormatJSON(result ExecutionResult, command string) ([]byte, error)` — serializes to JSON per the schema in the design doc. The `command` parameter is "lint" or "unit". When `result.SkipReason` is non-empty, includes a `"skipped"` field in the output.

Follow the output patterns from `internal/land/output.go` (✓/✗/⚠ indicators). Use `fmt.Fprintf` to the injected writer, not direct `fmt.Println`, so output is testable.

#### Testing `internal/runner/`

Test the engine with a real subprocess — create small shell scripts or Go test helpers that exit with known codes:

- Runner that exits 0 → Passed
- Runner that exits 1 → Failed
- Runner with non-existent command → Error (command not found)
- Runner that times out → Error (timeout)
- Multiple runners with mixed results → correct aggregate
- JSON output mode → valid JSON with correct fields
- File paths are appended to args correctly

Use `t.TempDir()` for working directory tests. Inject `bytes.Buffer` as Stdout/Stderr for output assertion.

### 1.2 Config Changes

#### `internal/config/config.go`

Add two fields to `LintRunner`:

```
FixArgs []string `mapstructure:"fixArgs"`
Timeout string   `mapstructure:"timeout"`
```

`Timeout` is symmetric with `TestRunner.Timeout` — a Go duration string parsed at workflow time. Without it, a hung linter blocks the pipeline indefinitely.

#### `docs/arc.schema.json`

Add `fixArgs` and `timeout` to the `lintRunner` definition:

```json
"fixArgs": {
  "type": "array",
  "description": "Arguments appended to the command when --fix is active",
  "items": { "type": "string" }
},
"timeout": {
  "type": "string",
  "description": "Timeout duration (e.g. \"30s\", \"5m\")"
}
```

#### Testing

Add config test cases that load a config with `fixArgs` and `timeout` and verify they deserialize into `LintRunner.FixArgs` and `LintRunner.Timeout` correctly. Add to the existing test file `internal/config/config_test.go`.

### 1.3 Lint Workflow (`internal/lint/`)

The existing `internal/lint/` package has `embed.go` (MegaLinter config). Add `workflow.go` and `types.go`.

#### `types.go` — Lint-Specific Types

- **`LintOptions`**: Fix (bool), All (bool), JSONMode (bool), Verbose (bool).
- **`LintResult`**: wraps `runner.ExecutionResult` + ChangedFileCount (int) + AllMode (bool).

#### `workflow.go` — Lint Orchestrator

The `LintWorkflow` struct holds: repo (`LintRepository` interface), executor (`Executor` interface — see below), config (`*config.Config`).

Constructor: `NewLintWorkflow(repo LintRepository, executor Executor, cfg *config.Config) *LintWorkflow`.

Method `Execute(ctx context.Context, opts *LintOptions) (*LintResult, error)`:

1. **Resolve runners.** Check `cfg.Lint.Runners`. If empty:
   - Check MegaLinter config: if `enabled == "true"` or (`enabled == "auto"` and Docker is available), and not JSON mode, print a message that MegaLinter support is coming in a future version, then fall through.
   - If not JSON mode, print guidance message (config examples for adding runners). Return success result with zero runners (in JSON mode, the empty ExecutionResult serializes to `{"command":"lint","success":true,"runners":[]}` — no guidance printed).
2. **Detect changed files** (unless `opts.All`):
   a. Get default branch: `cfg.GitHub.DefaultBranch` (or auto-detect via `repo.GetDefaultBranch()`).
   b. Compute merge-base: `repo.GetMergeBase("origin/"+defaultBranch, "HEAD")`. Fall back to `repo.GetMergeBase(defaultBranch, "HEAD")` if origin ref doesn't exist.
   c. Get changed files: `repo.GetFilesChanged(mergeBase, "HEAD")` — returns `[]git.FileChange` with deletion metadata.
   d. Filter out entries where `FileChange.IsDeleted == true`. Extract `.Path` from remaining entries.
   e. If no changed files remain: if not JSON mode, print "No changed files to lint". Return success result with `SkipReason: "no changed files"` (FormatJSON includes this as `"skipped"` in JSON output).
3. **Build runner configs.** For each `config.LintRunner`:
   a. Create `runner.RunnerConfig` with Name, Command, Args, WorkingDir.
   b. If runner's `Timeout` is non-empty, parse via `time.ParseDuration`. If parsing fails, error with clear message (same pattern as unit workflow). Set `RunnerConfig.Timeout`.
   c. If `opts.Fix` and runner's `AutoFix` is true and `FixArgs` is non-empty: set `ExtraArgs` to `FixArgs`.
   d. If not `opts.All`: set `FilePaths` to the changed file paths. File paths are repo-relative (from `GetFilesChanged`). If `WorkingDir` is non-empty and differs from repo root, paths must be adjusted relative to `WorkingDir` (e.g., repo-relative `frontend/src/App.tsx` becomes `src/App.tsx` when `WorkingDir` is `frontend/`). If a changed file falls outside the runner's `WorkingDir`, skip it for that runner.
4. **Execute.** Call `executor.Run()` with configs + changed file paths.
5. **Return** `LintResult`.

**Engine injection:** The workflow does not create the `runner.Engine` directly. Instead, it depends on an `Executor` interface (defined in `internal/runner/types.go`):

```
type Executor interface {
    Run(ctx context.Context, configs []RunnerConfig) (*ExecutionResult, error)
}
```

`Engine` satisfies this interface. In production, `cmd/lint.go` creates the engine and passes it to the workflow. In tests, a mock executor verifies the configs/file paths without running real subprocesses.

**Merge-base fallback — two distinct failure modes:**

1. **Ref resolution failure** (neither `origin/<default>` nor `<default>` can be resolved): error with a message suggesting `git fetch origin`. This means the default branch ref doesn't exist locally.
2. **No common ancestor** (both refs resolve but `GetMergeBase` fails, e.g. orphan branch): fall back to linting all files with a `⚠` warning. This is a valid but unusual state — the branch has no shared history with the default branch.

#### Testing `internal/lint/`

The workflow orchestrates git operations and the runner engine. Test with mocks/interfaces:

- Define a `LintRepository` interface with the git methods the workflow needs: `GetDefaultBranch() (string, error)`, `GetMergeBase(ref1, ref2 string) (string, error)`, `GetFilesChanged(base, head string) ([]git.FileChange, error)`. The real `git.Repository` satisfies this.
- Define a mock `Executor` that records the `[]RunnerConfig` it receives, allowing assertions on file paths, extra args, etc. without running subprocesses.
- Test cases:
  - No runners configured → guidance message, success
  - Runners configured, changed files found → engine called with correct file paths
  - `--all` mode → engine called without file paths
  - `--fix` mode → ExtraArgs set correctly on runners with autoFix=true, omitted on others
  - No changed files → early exit with success
  - Merge-base resolution failure → appropriate error

### 1.4 CLI Command (`cmd/lint.go`)

Follow the pattern from `cmd/land.go`:

1. Define package-level flag vars: `lintFix bool`, `lintAll bool`.
2. Define `lintCmd` with `Use: "lint"`, `Short`, `Long` (with examples), `Args: cobra.NoArgs`, `RunE: runLint`.
3. In `init()`: add to `rootCmd`, register `--fix` and `--all` flags.
4. `runLint` function:
   a. Load config (`config.Load()`).
   b. Open git repo (`git.OpenRepository(".")`).
   c. Create `LintWorkflow`.
   d. Call `Execute()` with options mapped from flags.
   e. Handle errors. If the result is non-nil and `!result.Success`, return `ErrSilentExit` (a sentinel error defined in `cmd/` that `Execute()` recognizes — it exits 1 without printing the error, since the lint summary already reported failures).
   f. Read JSON mode via `GetJSON()` (not `cfg.Output.JSON` — the package-level helper in `cmd/root.go` resolves flag-vs-config precedence). Read verbose via `GetVerbose()`. Pass both into `LintOptions`.
   g. In JSON mode, the workflow returns a `LintResult` and the CLI layer calls `runner.FormatJSON()` to print the single JSON object.

#### Testing

Test the cobra command wiring: flag parsing, mutual exclusions if any, that `runLint` is callable. Don't test the workflow here — that's tested in `internal/lint/`.

---

## Phase 2: Unit Command

### 2.1 Unit Workflow (`internal/unit/`)

Create `internal/unit/` with `workflow.go` and `types.go`.

#### `types.go`

- **`UnitOptions`**: JSONMode (bool), Verbose (bool).
- **`UnitResult`**: wraps `runner.ExecutionResult`.

#### `workflow.go`

The `UnitWorkflow` struct holds: executor (`runner.Executor` interface), config (`*config.Config`).

Constructor: `NewUnitWorkflow(executor runner.Executor, cfg *config.Config) *UnitWorkflow`.

Method `Execute(ctx context.Context, opts *UnitOptions) (*UnitResult, error)`:

1. **Resolve runners.** Check `cfg.Test.Runners`. If empty: if not JSON mode, print guidance message. Return success result with zero runners.
2. **Build runner configs.** For each `config.TestRunner`:
   a. Create `runner.RunnerConfig` with Name, Command, Args, WorkingDir.
   b. Parse `Timeout` string to `time.Duration` (using `time.ParseDuration`). If parsing fails, error with clear message.
3. **Execute.** Call `executor.Run()` with configs, no file paths.
4. **Return** `UnitResult`.

No git repository needed — unit doesn't do file scoping. Same `Executor` interface as lint for testability.

#### Testing

Use a mock `Executor` (same approach as lint):
- No runners → guidance message, success
- Runners with valid timeout → parsed correctly, executor receives correct timeout on RunnerConfig
- Runners with invalid timeout string → error
- Executor receives correct configs

### 2.2 CLI Command (`cmd/unit.go`)

Same pattern as `cmd/lint.go` but simpler:

1. Define `unitCmd` with `Use: "unit"`, `Args: cobra.NoArgs`, `RunE: runUnit`.
2. In `init()`: add to `rootCmd`. No command-specific flags in v1.
3. `runUnit`:
   a. Load config.
   b. Create `runner.Engine` and `UnitWorkflow` (no git repo needed).
   c. Read `GetJSON()` and `GetVerbose()` into `UnitOptions`.
   d. Call `Execute()`.
   e. Handle results same as lint (return `ErrSilentExit` on test failure).

---

## Cross-Cutting Concerns

### Error Handling

- **Command not found**: When a runner's binary doesn't exist, report as Error (not Failed). The summary should say `✗ <name>: command not found` with a hint to install the tool or check the config.
- **Timeout**: Report as Error with duration. `✗ <name>: timed out after 5m0s`.
- **No git repo** (lint only): If `git.OpenRepository()` fails, error with "not a git repository" (same pattern as other commands).
- **Ref resolution failure** (lint only): If neither `origin/<default>` nor local `<default>` can be resolved, error with a message suggesting `git fetch origin`.
- **No common ancestor** (lint only): If both refs resolve but `GetMergeBase` fails (e.g. orphan branch), fall back to linting all files with a `⚠` warning.

### Output Consistency

Use the same output patterns as `internal/land/output.go`:
- `✓` for success (green if color enabled)
- `✗` for failure (red)
- `⚠` for warnings (yellow)
- `▶` for runner banners

### JSON Mode

Read via `GetJSON()` (package-level helper in `cmd/root.go:128` — resolves `--json` flag vs `cfg.Output.JSON` precedence). Similarly, use `GetVerbose()` for verbosity (`cmd/root.go:118`). These are package-level functions in the `cmd` package, not methods on `*cobra.Command`.

When JSON mode is active:
- Suppress **all** non-JSON output — banners, runner stdout/stderr, summaries, and guidance messages
- Print a single JSON object to stdout at the end
- Exit code still reflects pass/fail
- JSON is emitted for every code path: no runners configured → `{"command":"lint","success":true,"runners":[]}`, no changed files → add `"skipped":"no changed files"`, runners executed → full results

### Testability

All workflow types accept interfaces for their dependencies (git repo, config). The runner engine accepts `io.Writer` for output. This enables testing without real git repos or real subprocesses for the workflow layer (though engine tests should use real subprocesses).
