# Pending Task Master Tasks

> Status: pending
> Source: `.taskmaster/tasks/tasks.json`
> Last synced: 2026-03-11

These tasks were imported from Task Master and represent the remaining feature roadmap for `gh-arc`. Each needs a design/implementation plan before work begins — use the `analysis-process` skill to create `docs/wip/<feature>/` directories with full documentation.

### Completed foundation (tasks 1–5)

For context, the following tasks are already done and form the foundation these tasks build on:

- **Task 1** — CLI framework (Cobra, Viper config, zerolog logging, global flags)
- **Task 2** — GitHub API client (`internal/github/`) with auth, retry, rate limiting, caching, GraphQL
- **Task 3** — Git operations module (`internal/git/`) with go-git, branch ops, commit parsing, diff generation
- **Task 4** — `gh arc diff` command with template editor, stacked PR support, auto-branch from main, CODEOWNERS parsing
- **Task 5** — `gh arc list` command with PR table display, filtering, CI/review status

### Existing configuration

The config system (`internal/config/`) already defines `LandConfig` with `defaultMergeMethod`, `deleteLocalBranch`, `deleteRemoteBranch`, `requireApproval`, `requireCI` — and `TestConfig`/`LintConfig` with runner definitions. These structs exist but their corresponding commands don't yet. See [README.md Configuration](../../README.md#configuration) for the full schema.

### Relationship to arcanist

These commands mirror [phorgeit/arcanist](https://github.com/phorgeit/arcanist) equivalents, adapted for GitHub. The original `arc land` verified Differential revisions before pushing; our version verifies GitHub PR approvals and CI checks before merging. The original `arc cover` used SVN/Git blame to suggest reviewers for Phabricator Differential; ours targets GitHub collaborators. The philosophy is the same: keep developers in the terminal for the entire review lifecycle.

---

## Task 6: `gh arc land` — Merge approved PRs to main
- **Priority:** high
- **Depends on:** Task 2 (GitHub client), Task 3 (Git operations)

**Motivation:** This is the counterpart to `gh arc diff` — together they form the core review loop. Without `land`, users must leave the CLI to merge PRs via the GitHub web UI or use `gh pr merge` directly (losing the opinionated verification flow). The original arcanist `arc land` was one of the most-used commands because it enforced team conventions (approval required, CI green) at merge time.

**Summary:** Accept PR number or detect from current branch. Verify the PR is approved (at least one approval, no outstanding change requests) and all CI/check runs pass. Offer merge method selection (squash by default, configurable via `land.defaultMergeMethod`). After merge: switch to default branch, pull latest, delete local feature branch, optionally delete remote branch.

**Key considerations for design:**
- Config already defines `LandConfig` with all relevant settings — the command should respect these defaults while allowing flag overrides
- Must handle stacked PRs gracefully — when landing a parent PR, dependent PRs need their base branch updated (or at minimum, warn the user). This is the inverse of the stacking detection in `diff`
- Should detect when the user is on the branch being landed vs. specifying a PR number, similar to how `diff` auto-detects the current branch's PR
- The `--no-delete` flag should override both `deleteLocalBranch` and `deleteRemoteBranch` config
- Consider what happens when approval/CI checks fail — clear error messages with actionable guidance ("PR needs 1 more approval", "CI check 'tests' is failing")

**Arcanist reference:** `arc land` pushed the commit directly and closed the Differential revision. Our version merges via GitHub API since direct pushes to protected branches aren't possible.

---

## Task 7: `gh arc cover` — Reviewer suggestion system
- **Priority:** medium
- **Depends on:** Task 2 (GitHub client), Task 3 (Git operations)

**Motivation:** Finding the right reviewer is a common friction point. The original `arc cover` was simple but effective — it used version control history to suggest people who had recently touched the relevant code. CODEOWNERS files provide team-level ownership, but blame analysis fills in the gaps for files without explicit owners.

**Summary:** Analyze changed files in the current branch (or a specified PR), run `git blame` to find recent contributors, parse CODEOWNERS for explicit ownership, query GitHub API to validate suggested reviewers are actual collaborators, score candidates based on recency and coverage, and display ranked suggestions.

**Key considerations for design:**
- The `internal/codeowners/` package already exists with parsing and pattern matching — this command should reuse it rather than duplicating
- `diff` already has reviewer suggestion logic (used to pre-fill the template) — `cover` should share the same scoring engine but provide a standalone, more detailed view
- Need to filter out the current user from suggestions (already handled in `pr_executor.assignReviewers()`)
- Consider team handles from CODEOWNERS — should these be expanded to individual members or suggested as-is?
- Output should show why each person is suggested (e.g., "modified auth.go 12 times in last 3 months", "CODEOWNERS match for internal/github/*")
- The `--count` flag controls how many suggestions to show (default 5)

**Arcanist reference:** `arc cover` was straightforward blame analysis. Our version adds CODEOWNERS integration since that's standard on GitHub.

---

## Task 8: `gh arc lint` / `gh arc unit` — Code quality integration
- **Priority:** medium
- **Depends on:** Task 3 (Git operations)

**Motivation:** Running linters and tests against only changed files is faster than whole-project runs and matches the review-focused workflow. The original arcanist had pluggable lint/unit engines — teams could integrate their own tools. Our version should follow the same extensible pattern, with Go tools (golangci-lint, `go test`) as built-in defaults.

**Summary:** Two commands sharing a common plugin architecture. `lint` detects changed files from git diff and runs configured linters; `unit` detects changed packages and runs test runners. Both parse tool output and display results in a consistent format.

**Key considerations for design:**
- Config already defines `TestConfig` and `LintConfig` with runner arrays — each runner specifies command, args, working directory, and timeout. The implementation should execute these configured runners
- The plugin interface should be simple: detect (can this tool run?), execute (run it), parse (interpret output). Avoid over-engineering the abstraction — start with golangci-lint and `go test`, add more later
- Changed file detection differs between the two: `lint` operates on individual files, `unit` operates on Go packages containing changed files
- Consider `--fix` flag for lint (auto-fix where supported), `--coverage` for unit
- `MegaLinter` integration is mentioned in config — this could be a lint plugin that delegates to MegaLinter's Docker-based approach. Decide during design whether to include this in v1 or defer
- Results should include file:line:column for IDE-clickable output

**Arcanist reference:** `arc lint` and `arc unit` had a rich plugin system (ArcanistLinter, ArcanistUnitTestEngine). We want something simpler since Go tooling is more standardized, but the extensibility principle is the same.

---

## Task 9: Branch and patch management commands
- **Priority:** low
- **Depends on:** Task 2 (GitHub client), Task 3 (Git operations)

**Motivation:** These are convenience commands that round out the daily workflow. `branch` provides a quick overview of what you're working on (like `git branch` but with PR context). `patch` and `export` support the reviewer side — applying someone else's changes locally for testing. `amend` addresses the common need to update commit messages after review feedback.

**Summary:** Four related but independent commands:
- **`gh arc branch`** — List local branches enriched with PR status, CI state, and review status. Mark branches safe to delete (already merged). `--cleanup` flag to bulk-delete merged branches.
- **`gh arc patch`** — Apply a PR's changes to the working copy for local testing. Accept PR number or URL.
- **`gh arc export`** — Download a PR as a patch file (`.patch` or `.diff` format).
- **`gh arc amend`** — Update the most recent commit message, optionally incorporating review feedback context.

**Key considerations for design:**
- These are four separate commands that could be implemented incrementally — consider whether they should be one task or split into separate design docs
- `branch` is the most complex — it needs to batch GitHub API calls efficiently (one call per branch for PR lookup would be slow). Consider using GraphQL to fetch all open PRs in one call and match to local branches
- `patch` could delegate to `gh pr checkout` if available, with a fallback to fetching the diff and applying with `git apply`
- `amend` is tricky with squash-merge workflows — amending a commit message locally doesn't affect the PR merge commit. Decide whether this should also update the PR title/description via API
- These are all low priority because `gh` CLI already covers some of this ground (`gh pr checkout`, `gh pr diff`). The value-add is the integrated, opinionated experience

**Arcanist reference:** `arc patch` applied a Differential revision's changes. `arc export` generated patches. `arc amend` updated commit messages with revision metadata. `arc branch` didn't exist in arcanist (Phabricator didn't have a branch-centric workflow).

---

## Task 10: Auxiliary features and polish
- **Priority:** low
- **Depends on:** Task 1 (CLI framework), Task 2 (GitHub client)

**Motivation:** These are quality-of-life features that improve the overall user experience but aren't core to the review workflow. Shell completion makes the tool discoverable. Gist management is a natural fit since `gh-arc` already wraps GitHub. The config system makes onboarding easier. Styling and progress indicators make the tool feel polished.

**Summary:** Shell completion generation (bash/zsh/fish/powershell via Cobra's built-in generators), gist management (create/list/view/edit/delete), interactive configuration wizard (`gh arc config init`), lipgloss-based terminal styling, and progress indicators for long operations.

**Key considerations for design:**
- Shell completion already has a workaround documented in README.md (wrapper script + completion from releases) — the built-in command formalizes this
- Gist commands could be thin wrappers around `gh gist` — decide whether there's enough value-add to justify a separate implementation vs. delegating
- The config wizard should generate `.arc.json`/`.arc.yaml` and explain what each setting does — useful for onboarding new team members
- Lipgloss styling should be added incrementally to existing commands, not just new ones — but avoid a massive refactor. Define a style system in `internal/ui/` and migrate commands over time
- Progress indicators (spinners) are most valuable for `diff` (pushing, creating PR) and `land` (merging, cleanup) — the operations that hit the network
- **Overlap with testing-flags:** The dry-run mode mentioned here overlaps with the `testing-flags` WIP feature. During design, reference `docs/wip/testing-flags/design.md` to avoid duplicating that work

---

## Task 11: Persistent caching system (TieredCache)
- **Priority:** medium
- **Depends on:** Task 2 (GitHub client)

**Motivation:** The current in-memory cache (`internal/cache/`) only lasts for a single command invocation. Repeated `gh arc list` or `gh arc diff` calls hit the GitHub API every time, which is slow and eats into rate limits. A persistent file-based cache layer would make subsequent invocations near-instant for unchanged data, especially useful for `list` which fetches multiple PRs with reviews and CI status.

**Summary:** Add `FileCache` implementing the existing `Cache` interface with file-based persistence under `~/.cache/gh-arc/`. Wrap both in a `TieredCache` (L1 memory, L2 file). Add `gh arc cache clear/stats` management commands and a global `--no-cache` flag. Implement pruning (LRU, max size, max age), cache versioning, and per-user key namespacing to prevent cross-account pollution.

**Key considerations for design:**
- The existing `Cache` interface in `internal/github/cache.go` already defines `Get`/`Set` with TTL and ETag support — `FileCache` should implement the same interface cleanly
- File locking is critical for concurrent access (multiple terminal sessions running `gh arc` simultaneously)
- Different data types need different TTLs: branch/ref data can be cached longer (hours), PR data should be shorter (minutes), review/CI status should be very short or use ETags for conditional requests
- The `--no-cache` flag should be a persistent Cobra flag on root, similar to `--verbose`
- Cache invalidation on write: when `diff` creates/updates a PR or `land` merges one, the relevant cache entries must be invalidated
- Consider whether the cache should be opt-in or opt-out — file caching adds complexity and potential staleness issues

---

## Task 12: Enhanced auth subcommands
- **Priority:** medium
- **Depends on:** Task 2 (GitHub client), Task 5 (`list` command, for existing auth patterns)

**Motivation:** The current `gh arc auth` is a single command that verifies API access. Users frequently need to refresh their OAuth scopes (the README already documents `gh auth refresh --scopes "user:email,read:user"`) and check their auth status. Wrapping these operations in `gh arc auth status` and `gh arc auth refresh` keeps users in the `gh arc` workflow and ensures the correct scopes are always included.

**Summary:** Refactor `gh arc auth` from a single `RunE` command to a parent command with two subcommands. `auth status` combines the existing API verification with `gh auth status` output. `auth refresh` wraps `gh auth refresh` and automatically appends the required OAuth scopes (`user:email`, `read:user`).

**Key considerations for design:**
- Backward compatibility: currently `gh arc auth` runs verification directly. After refactoring, bare `gh arc auth` (no subcommand) should either show help or default to `auth status` — decide which is less surprising
- The refresh command should merge user-specified scopes with required ones, not replace them
- Both subcommands depend on `gh` CLI being installed — need `exec.LookPath` check with a helpful error message if missing
- Error messages should distinguish between "gh CLI not installed", "not logged in", "logged in but missing scopes", and "logged in with correct scopes but API error"
- This is a relatively small, self-contained refactor — good candidate for an early win

---

## Task 13: Integration testing infrastructure
- **Priority:** medium
- **Depends on:** Task 2 (GitHub client), Task 3 (Git operations)

**Motivation:** The current test suite uses mocks and httptest for GitHub API interactions. This catches interface-level bugs but misses real API behavior (pagination, rate limiting edge cases, field format changes). A real integration test suite using ephemeral GitHub repositories would provide confidence that the tool works end-to-end against the actual API.

**Summary:** Create a `testutil` package with helpers for creating/destroying test repositories, validating test environment (token scopes), and cleaning up orphaned resources. Implement Phase 1 integration tests covering core API operations (PR CRUD, reviews, check runs). Set up CI/CD pipeline with `//go:build integration` tag separation.

**Key considerations for design:**
- **Overlap with testing-flags:** The `docs/wip/testing-flags/` feature covers E2E testing from the binary/CLI level using `--dry-run`/`--offline` flags. This task is about API-level integration tests using real GitHub calls. They're complementary, not duplicative — but the design should clarify the boundary
- Ephemeral test repos must be reliably cleaned up — orphan detection should scan for repos matching a naming pattern (e.g., `gh-arc-integration-test-*`) and delete stale ones
- Tests should be gated behind `GITHUB_INTEGRATION_TOKEN` env var — never run against real repos by accident
- Rate limiting is a real concern for integration tests — consider test design that minimizes API calls (reuse repos across tests in a suite, batch operations)
- The `testify/suite` pattern works well for shared setup/teardown of test repositories
- CI should run integration tests only on main branch pushes (not on every PR) to avoid token exposure and rate limit issues
