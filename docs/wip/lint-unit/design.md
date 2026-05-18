# Design: `gh arc lint` and `gh arc unit`

> Status: approved
> Created: 2026-05-18
> Task 8 from [docs/archive/tm_tasks.md](../../archive/tm_tasks.md)

## Overview

Two commands — `gh arc lint` and `gh arc unit` — that run configured linters and test runners against the current project. Both share a common runner execution engine. The design prioritizes language-agnostic extensibility: `gh-arc` is a general-purpose trunk-based workflow tool, not tied to any specific language ecosystem.

## Motivation

Running linters and tests is a core part of the review workflow. Without `lint` and `unit`, developers must leave the `gh-arc` CLI to run project-specific tooling manually. The original arcanist had `arc lint` and `arc unit` with pluggable engines — teams could integrate their own tools. Our version follows the same extensible pattern using declarative runner configuration instead of code-level plugin interfaces.

## Arcanist Reference

- `arc lint` had a rich plugin system (`ArcanistLinter`) where each linter implemented a PHP interface. This was powerful but required dedicated teams to maintain linter wrappers. Our version avoids this by treating linters as opaque subprocesses configured via JSON.
- `arc unit` used `ArcanistUnitTestEngine` to detect and run tests for changed files. Our version is simpler — it runs configured test runners without language-specific file scoping, since mapping changed files to test targets is inherently language-specific and fragile.

## Architecture

### Shared Runner Engine (`internal/runner/`)

The core abstraction used by both commands. It takes a list of runner configurations and executes them sequentially, streaming output and collecting results.

```
cmd/lint.go ──┐
              ├──> internal/lint/    ──> internal/runner/Engine
cmd/unit.go ──┘    internal/unit/
```

**Key types:**

- `RunnerConfig` — unified execution config: name, command, args, workingDir, timeout, optional extra args (used by lint's --fix)
- `RunResult` — per-runner outcome: name, status (pass/fail/error/skipped), exit code, duration
- `ExecutionResult` — aggregate of all RunResults, overall success bool
- `Engine` — the executor, takes `[]RunnerConfig` + `EngineOptions`

**Execution flow per runner:**

1. Print banner: `▶ Running <name>...`
2. Build command: `command + args` (+ extra args if provided) (+ file paths for lint)
3. Execute via `os/exec`, streaming stdout/stderr directly to terminal (preserving TTY colors)
4. Capture exit code and duration
5. Print summary line: `✓ <name>: passed` or `✗ <name>: failed (exit code N)`
6. After all runners: print overall summary

**`--json` mode:** Suppresses **all** non-JSON output — banners, runner stdout/stderr, summaries, and guidance messages. Returns a single JSON object to stdout with execution metadata only — not parsed lint/test findings. Users who need structured issue output configure the underlying tool to emit JSON (e.g., `eslint -f json` in the runner's args).

JSON is emitted for every code path, including edge cases:
- No runners configured → `{"command":"lint","success":true,"runners":[]}`
- No changed files → `{"command":"lint","success":true,"runners":[],"skipped":"no changed files"}`
- Runners executed → full runner results

```json
{
  "command": "lint",
  "success": false,
  "runners": [
    { "name": "golangci-lint", "status": "failed", "exit_code": 1, "duration_ms": 1450 },
    { "name": "eslint", "status": "passed", "exit_code": 0, "duration_ms": 320 }
  ]
}
```

**Design rationale — passthrough over parsing:** Modern linters produce rich, colorized, IDE-clickable output with code snippets and fix hints. Parsing this into a unified schema would destroy that context and require fragile per-tool parsers. Streaming native output preserves the developer's familiar experience. The framing (banners + summaries) adds multi-runner clarity without interfering.

### Lint Layer (`internal/lint/`)

Wraps the runner engine with lint-specific concerns.

**Workflow:**

1. **Resolve runners.** Load `LintConfig.Runners` from config. If empty and MegaLinter is enabled → MegaLinter runner (deferred to v2). If nothing → print guidance message with config examples, exit 0.
2. **Detect changed files.** Compute merge-base between HEAD and the default branch via `git.GetMergeBase()`, then `git.GetFilesChanged(mergeBase, "HEAD")` (returns `[]git.FileChange` with deletion metadata). Filter out entries where `IsDeleted == true`, then extract paths from the remaining entries. If `--all` flag → skip detection, pass no file paths (runner lints everything).
3. **Convert configs.** Map `config.LintRunner` → `runner.RunnerConfig`. When `--fix` is active and the runner's `autoFix` is true, append `fixArgs` to the command args.
4. **Execute.** Call `Engine.Run()` with configs + changed file paths.
5. **Return results.**

**Changed-file detection detail:** Uses merge-base (not the default branch tip) to correctly handle the case where the default branch has advanced since the feature branch was created. This matches how GitHub computes the PR diff.

**`--fix` mechanism:** Each `LintRunner` declares `fixArgs` (e.g., `["--fix"]` for golangci-lint). The `--fix` CLI flag enables fix mode globally. Per-runner `autoFix: bool` controls opt-in — if `autoFix: false` (the default), the runner skips fix mode even when `--fix` is passed. This gives per-runner granularity.

### Unit Layer (`internal/unit/`)

Wraps the runner engine with test-specific concerns. Simpler than lint — no file scoping, no fix mode.

**Workflow:**

1. **Resolve runners.** Load `TestConfig.Runners`. If empty → guidance message, exit 0.
2. **Convert configs.** Map `config.TestRunner` → `runner.RunnerConfig`. Parse the `Timeout` string (e.g., `"5m"`) into a `time.Duration`.
3. **Execute.** Call `Engine.Run()` with configs, no file paths.
4. **Return results.**

**No changed-file scoping by design.** Mapping changed files to test targets is inherently language-specific (Go uses packages, Jest uses file paths, Python uses module paths). Rather than building per-ecosystem adapters, v1 runs all configured tests. Users scope via runner args if needed.

## CLI Interface

### `gh arc lint`

```
Usage:
  gh arc lint [flags]

Flags:
  --fix     Auto-fix issues where supported (uses runner fixArgs)
  --all     Lint all files, not just changed ones

Global flags inherited:
  --json    Output execution results as JSON
  -v        Verbose logging
```

No positional arguments accepted (`cobra.NoArgs`).

### `gh arc unit`

```
Usage:
  gh arc unit [flags]

Global flags inherited:
  --json    Output execution results as JSON
  -v        Verbose logging
```

No positional arguments accepted (`cobra.NoArgs`). No command-specific flags in v1.

## Configuration

### Existing Config (no changes)

```json
{
  "lint": {
    "runners": [
      { "name": "golangci-lint", "command": "golangci-lint", "args": ["run"] }
    ],
    "megaLinter": { "enabled": "auto", "config": ".mega-linter.yml", "fixIssues": false }
  },
  "test": {
    "runners": [
      { "name": "go-test", "command": "go", "args": ["test", "./..."], "timeout": "5m" }
    ]
  }
}
```

### Config Additions: `fixArgs` and `timeout`

`LintRunner` gains two fields — `fixArgs` and `timeout`:

```json
{
  "name": "golangci-lint",
  "command": "golangci-lint",
  "args": ["run"],
  "fixArgs": ["--fix"],
  "autoFix": true,
  "timeout": "5m"
}
```

- **`fixArgs`**: When `--fix` is active and `autoFix: true`, these args are appended. The executed command becomes: `golangci-lint run --fix <changed-files>`.
- **`timeout`**: Go duration string (e.g., `"5m"`, `"30s"`). Symmetric with `TestRunner.Timeout`. A hung linter without a timeout blocks the entire pipeline indefinitely. If omitted, the runner has no timeout.

## Exit Codes

- **0** — all runners passed (exit code 0)
- **1** — one or more runners reported issues (any non-zero exit code)

Both `lint` and `unit` use the same exit code strategy.

**Implementation note:** Cobra v1.10.1 has no `ErrSilent` sentinel. The project's `Execute()` in `cmd/root.go` prints any returned error to stderr before calling `os.Exit(1)`. Since lint/unit already print their own summary output, returning a normal error would double-print. Define a package-level sentinel `ErrSilentExit` in `cmd/` that `Execute()` recognizes and skips printing — it just exits with code 1.

## Multi-Runner Behavior

All configured runners execute regardless of individual failures. The final summary reports combined results. This maximizes feedback per invocation — a single `gh arc lint` run tells you everything that's wrong, not just the first thing.

## Output Examples

### Normal mode (multiple runners)

```
▶ Running golangci-lint...
internal/lint/workflow.go:42:3: unused variable 'x' (deadcode)
internal/runner/engine.go:15:1: missing function comment (golint)

✗ golangci-lint: failed (exit code 1) [1.4s]

▶ Running shellcheck...
All files passed.

✓ shellcheck: passed [0.3s]

━━━
✗ 1 of 2 runners failed
```

### JSON mode

```json
{
  "command": "lint",
  "success": false,
  "runners": [
    { "name": "golangci-lint", "status": "failed", "exit_code": 1, "duration_ms": 1450 },
    { "name": "shellcheck", "status": "passed", "exit_code": 0, "duration_ms": 320 }
  ]
}
```

### No runners configured

```
No lint runners configured.

Add runners to .arc.json:
  "lint": {
    "runners": [
      { "name": "golangci-lint", "command": "golangci-lint", "args": ["run"] }
    ]
  }

Or enable MegaLinter (requires Docker):
  "lint": { "megaLinter": { "enabled": "true" } }
```

## Deferred to Future Versions

- **MegaLinter integration** — config and embedded defaults exist; Docker execution engine deferred to v2
- **`--coverage` flag for unit** — report test coverage
- **Changed-file scoping for unit** — per-ecosystem adapters to map files → test targets
- **Parallel runner execution** — requires output buffering to prevent interleaving
- **`--dry-run` integration** — when testing-flags feature lands, lint/unit should respect it
- **File extension filtering** — runners declare which extensions they handle, skip irrelevant files

## Related Features

- **testing-flags** (`docs/wip/testing-flags/`): Covers `--dry-run`/`--offline`/`--no-edit` for E2E testing. Complementary to lint/unit — these are local-only commands that don't hit the network, so `--offline` is irrelevant. `--dry-run` could be useful for showing what would be linted without running it, but this is deferred.
