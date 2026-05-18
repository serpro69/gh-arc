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
- **`ExecutionResult`**: Runners ([]RunResult), Success (bool — true iff all runners passed). Add a helper method `FailedCount() int`.
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
- `FormatJSON(result ExecutionResult, command string) ([]byte, error)` — serializes to JSON per the schema in the design doc. The `command` parameter is "lint" or "unit".

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

Add `FixArgs` field to `LintRunner`:

```
FixArgs []string `mapstructure:"fixArgs"`
```

No other changes — the existing fields and validation are sufficient.

#### `docs/arc.schema.json`

Add `fixArgs` to the `lintRunner` definition:

```json
"fixArgs": {
  "type": "array",
  "description": "Arguments appended to the command when --fix is active",
  "items": { "type": "string" }
}
```

#### Testing

Add a config test case that loads a config with `fixArgs` and verifies it deserializes correctly. Add to the existing test file `internal/config/config_test.go`.

### 1.3 Lint Workflow (`internal/lint/`)

The existing `internal/lint/` package has `embed.go` (MegaLinter config). Add `workflow.go` and `types.go`.

#### `types.go` — Lint-Specific Types

- **`LintOptions`**: Fix (bool), All (bool), JSONMode (bool), Verbose (bool).
- **`LintResult`**: wraps `runner.ExecutionResult` + ChangedFileCount (int) + AllMode (bool).

#### `workflow.go` — Lint Orchestrator

The `LintWorkflow` struct holds: repo (`*git.Repository`), config (`*config.Config`).

Constructor: `NewLintWorkflow(repo *git.Repository, cfg *config.Config) *LintWorkflow`.

Method `Execute(ctx context.Context, opts *LintOptions) (*LintResult, error)`:

1. **Resolve runners.** Check `cfg.Lint.Runners`. If empty:
   - Check MegaLinter config: if `enabled == "true"` or (`enabled == "auto"` and Docker is available), print a message that MegaLinter support is coming in a future version, then fall through.
   - Print guidance message (config examples for adding runners). Return success result with zero runners.
2. **Detect changed files** (unless `opts.All`):
   a. Get default branch: `cfg.GitHub.DefaultBranch` (or auto-detect via `repo.GetDefaultBranch()`).
   b. Compute merge-base: `repo.GetMergeBase("origin/"+defaultBranch, "HEAD")`. Fall back to `repo.GetMergeBase(defaultBranch, "HEAD")` if origin ref doesn't exist.
   c. Get changed files: `repo.GetChangedFiles(mergeBase, "HEAD")`.
   d. Filter out deleted files (where `FileChange.IsDeleted == true`).
   e. If no changed files remain, print "No changed files to lint" and return success.
3. **Build runner configs.** For each `config.LintRunner`:
   a. Create `runner.RunnerConfig` with Name, Command, Args, WorkingDir.
   b. If `opts.Fix` and runner's `AutoFix` is true and `FixArgs` is non-empty: set `ExtraArgs` to `FixArgs`.
   c. If not `opts.All`: set `FilePaths` to the changed file paths.
4. **Execute.** Create `runner.Engine` and call `Run()`.
5. **Return** `LintResult`.

**Merge-base fallback:** Try `origin/<default>` first (most accurate for feature branches). If that fails (e.g. no remote), fall back to the local default branch ref. If that also fails, error with a message suggesting `git fetch origin`.

#### Testing `internal/lint/`

The workflow orchestrates git operations and the runner engine. Test with mocks/interfaces:

- Define a `LintRepository` interface with the git methods the workflow needs: `GetDefaultBranch()`, `GetMergeBase()`, `GetChangedFiles()`. The real `git.Repository` satisfies this.
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
   e. Handle errors. If the result is non-nil and `!result.Success`, return exit code 1 (use `cobra.ErrSilent` or similar to avoid double-printing).
   f. In JSON mode, print the JSON output and return.

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

The `UnitWorkflow` struct holds: config (`*config.Config`).

Constructor: `NewUnitWorkflow(cfg *config.Config) *UnitWorkflow`.

Method `Execute(ctx context.Context, opts *UnitOptions) (*UnitResult, error)`:

1. **Resolve runners.** Check `cfg.Test.Runners`. If empty → guidance message, return success.
2. **Build runner configs.** For each `config.TestRunner`:
   a. Create `runner.RunnerConfig` with Name, Command, Args, WorkingDir.
   b. Parse `Timeout` string to `time.Duration` (using `time.ParseDuration`). If parsing fails, error with clear message.
3. **Execute.** Create `runner.Engine` and call `Run()`.
4. **Return** `UnitResult`.

No git repository needed — unit doesn't do file scoping.

#### Testing

- No runners → guidance message, success
- Runners with valid timeout → parsed correctly
- Runners with invalid timeout string → error
- Engine called with correct configs

### 2.2 CLI Command (`cmd/unit.go`)

Same pattern as `cmd/lint.go` but simpler:

1. Define `unitCmd` with `Use: "unit"`, `Args: cobra.NoArgs`, `RunE: runUnit`.
2. In `init()`: add to `rootCmd`. No command-specific flags in v1.
3. `runUnit`:
   a. Load config.
   b. Create `UnitWorkflow` (no git repo needed).
   c. Call `Execute()`.
   d. Handle results same as lint.

---

## Cross-Cutting Concerns

### Error Handling

- **Command not found**: When a runner's binary doesn't exist, report as Error (not Failed). The summary should say `✗ <name>: command not found` with a hint to install the tool or check the config.
- **Timeout**: Report as Error with duration. `✗ <name>: timed out after 5m0s`.
- **No git repo** (lint only): If `git.OpenRepository()` fails, error with "not a git repository" (same pattern as other commands).
- **Merge-base failure** (lint only): If no common ancestor exists (e.g. orphan branch), fall back to linting all files with a warning.

### Output Consistency

Use the same output patterns as `internal/land/output.go`:
- `✓` for success (green if color enabled)
- `✗` for failure (red)
- `⚠` for warnings (yellow)
- `▶` for runner banners

### JSON Mode

Check `cfg.Output.JSON` or a future `--json` flag. When active:
- Suppress all non-JSON output (banners, summaries, runner stdout)
- Print a single JSON object at the end
- Exit code still reflects pass/fail

### Testability

All workflow types accept interfaces for their dependencies (git repo, config). The runner engine accepts `io.Writer` for output. This enables testing without real git repos or real subprocesses for the workflow layer (though engine tests should use real subprocesses).
