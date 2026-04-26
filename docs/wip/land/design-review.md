# Design Review: `gh arc land`

> Date: 2026-04-26
> Scope: `docs/wip/land/design.md`, `implementation.md`, `tasks.md`
> Mode: WIP design review via `kk:design`

## Summary

The design is coherent overall and the task breakdown is implementable, but several merge-safety details should be resolved before finishing the workflow and command wiring tasks.

## Findings

### P1 - Missing check that local HEAD is the PR head

- Location: `design.md` workflow and pre-merge checks, especially lines 83-94 and 105-142.
- Issue: The current pre-merge flow checks a clean working directory, branch name, PR existence, approval, and CI, but it does not verify that the local branch HEAD equals the remote PR head SHA.
- Impact: A user can have a clean local branch with unpushed commits and run `gh arc land`; the GitHub API will merge the stale remote PR head, not the local commits the user expects to land.
- Suggested design change: Add a non-bypassable pre-merge check after PR discovery: compare local `HEAD` to `pr.Head.SHA`, or compare local branch against its upstream. If they differ, block with guidance to run `gh arc diff` or `git push`.

### P1 - `requireCI: "required"` intentionally degrades to no CI on 403

- Location: `design.md` lines 267-273 and `implementation.md` lines 81-85.
- Issue: The design says an insufficient-permissions response from the branch-protection API should return an empty required-checks list, making `requireCI: "required"` behave like `"none"`.
- Impact: The default CI mode can silently stop enforcing CI for users without branch-protection read permission. That weakens the central promise of `land`: enforce team conventions before merge.
- Suggested design change: Treat 403 as an explicit "cannot verify required checks" failure unless `--force` is used, or add a separate config value for permissive fallback. Keep 404 as "no branch protection".

### P2 - Required checks are specified as names only

- Location: `implementation.md` lines 81-83 and `design.md` lines 135-142.
- Issue: The design models required checks as `[]string` contexts. GitHub required checks can also be app-scoped.
- Impact: If two GitHub Apps publish the same check name, the preflight may accept the wrong producer or report misleading results.
- Suggested design change: Model required checks as `{context, app_id}` and preserve app metadata from check runs where possible.

### P2 - Branch path escaping is not called out

- Location: `implementation.md` line 82 and `design.md` line 270.
- Issue: The branch-protection endpoint includes `{branch}` in a path segment, but the design does not require URL-escaping branch names.
- Impact: Common branch names such as `release/1.2` can produce the wrong REST path and make required-check detection unreliable.
- Suggested design change: State explicitly that branch path values must be URL-escaped, and require tests for slash-containing base branches.

### P3 - Dependent PR retargeting message is overconfident

- Location: `design.md` lines 144-148 and 214-219.
- Issue: The design says dependent PRs "will be retargeted after merge", but this depends on GitHub deleting the remote branch, which this command does not do directly.
- Impact: Users may believe dependent PRs were retargeted when the repository setting did not delete the head branch.
- Suggested design change: Phrase this as "may be retargeted if GitHub auto-deletes the merged branch" and keep it informational.

## Suggested Task Updates

- Add a Task 4 follow-up for local HEAD vs PR head validation.
- Add a Task 2 follow-up for required-check branch escaping and 403 behavior.
- Add a Task 2 or Task 4 follow-up for app-scoped required checks.
- Add a Task 3/7 output wording follow-up for dependent PR retargeting.
