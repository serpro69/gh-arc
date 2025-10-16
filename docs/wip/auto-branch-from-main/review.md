● Code Review: Auto-Branch from Main Feature

I've completed a comprehensive review of the auto-branch-from-main design and
implementation plan using Zen with gemini-2.5-pro. Here are the findings
organized by severity:

🔴 CRITICAL ISSUES (Must Fix Before Implementation)

1. Unsafe origin/main Fallback - DATA LOSS RISK

Location: feature-design.md:189, implementation-plan.md:323

Problem: The error handling plan states "Use main as reset point" when
origin/main doesn't exist. This is extremely dangerous:
- Resetting main to itself does nothing
- Could reset to wrong commit if local main is ahead
- Masks critical repository state issues

Impact: Potential data loss, incorrect repository state

Fix: Abort explicitly when upstream branch cannot be found:
-| No `origin/main` ref | Use `main` as reset point (assumes offline or first
commit) |
+| No `origin/main` ref | Abort with error: "Cannot find remote branch
'origin/main'. Please run 'git fetch' and try again." |

Validation: ✅ Independently confirmed and validated by expert analysis

---
2. Missing Stash Conflict Handling - UNCOMMITTED WORK LOSS

Location: implementation-plan-part2.md:745-749

Problem: StashPop failure only logs a warning. Stash pop can fail due to merge
conflicts, leaving user's work "stuck" in the stash and not applied to their new
 branch.

Impact: Users lose track of uncommitted work, potential data loss

Fix: Provide urgent warning with explicit recovery instructions:
if result.StashCreated {
    err = d.repo.StashPop()
    if err != nil {
        logger.Error().Err(err).Msg("Failed to pop stash due to conflict or
error")
        fmt.Printf("\n❌ CRITICAL: Failed to restore your uncommitted
changes.\n")
        fmt.Println("   Your changes are safe in the stash.")
        fmt.Println("   To restore them manually and resolve conflicts, run:")
        fmt.Println("     git stash apply")
        // Do not drop the stash - let user do it after they're sure
    }
}

Validation: ✅ Independently confirmed, expert provided excellent implementation
 details

---
3. Unbounded Recursion in Prompts - STACK OVERFLOW RISK

Location: implementation-plan-part2.md:522

Problem: promptYesNo and promptBranchName use recursion for invalid input
without depth limit:
default:
    fmt.Println("Please answer 'y' or 'n'")
    return promptYesNo(message, defaultYes)  // Unbounded recursion!

Impact: Stack overflow crash with continuous invalid input

Fix: Use iterative loop instead of recursion:
func promptYesNo(message string, defaultYes bool) (bool, error) {
    defaultStr := "Y/n"
    if !defaultYes {
        defaultStr = "y/N"
    }
    reader := bufio.NewReader(os.Stdin)

    for { // Use loop instead of recursion
        fmt.Printf("%s (%s) ", message, defaultStr)
        response, err := reader.ReadString('\n')
        if err != nil {
            return false, fmt.Errorf("failed to read input: %w", err)
        }

        response = strings.TrimSpace(strings.ToLower(response))
        if response == "" {
            return defaultYes, nil
        }

        switch response {
        case "y", "yes":
            return true, nil
        case "n", "no":
            return false, nil
        default:
            fmt.Println("Please answer 'y' or 'n'")
            // Loop continues, no recursion
        }
    }
}

Validation: ✅ Independently confirmed and validated by expert

---
4. No Rollback on Partial Failure - INCONSISTENT STATE

Location: implementation-plan-part2.md:677, ExecuteAutoBranch method

Problem: If operations fail midway (e.g., CreateBranch fails after Stash), the
function exits, leaving repository in altered state (changes stashed, but still
on main).

Impact: Confusing state, requires manual cleanup

Fix: Implement rollback mechanism using defer or error handling:
// In ExecuteAutoBranch
if !status.IsClean {
    // ... stash changes ...
    result.StashCreated = true
}

branchName, err := d.GetBranchName()
if err != nil {
    // Rollback: pop stash if we created one
    if result.StashCreated {
        if popErr := d.repo.StashPop(); popErr != nil {
            logger.Warn().Err(popErr).Msg("Failed to auto-pop stash during
rollback")
        }
    }
    return nil, fmt.Errorf("failed to get branch name: %w", err)
}

// Continue this pattern for subsequent failures

Validation: ✅ Independently confirmed and validated by expert

---
🟠 HIGH SEVERITY ISSUES

5. Username Injection Vulnerability

Location: implementation-plan-part2.md:239-241

Problem: {username} placeholder reads from git config user.name with basic
blacklist sanitization. Malicious usernames could bypass sanitization.

Fix: Implement whitelist-based sanitization:
import "regexp"

var invalidBranchChars = regexp.MustCompile(`[^a-zA-Z0-9/_-]`)

func sanitize(value string) string {
    // Replace all invalid characters with hyphen
    sanitized := invalidBranchChars.ReplaceAllString(value, "-")
    // Enforce max length
    if len(sanitized) > 50 {
        sanitized = sanitized[:50]
    }
    return sanitized
}

Validation: ✅ My finding expanded with excellent implementation by expert

---
6. Detached HEAD State Not Handled

Location: feature-design.md:70, throughout detection logic

Problem: If repository is in detached HEAD state, GetCurrentBranch() will fail
or return non-branch name, causing unexpected failures.

Fix: Explicitly check for detached HEAD:
// In DetectCommitsOnMain
currentBranch, err := d.repo.GetCurrentBranch()
if err != nil {
    if errors.Is(err, git.ErrDetachedHead) {
        logger.Debug().Msg("Detached HEAD state detected, skipping auto-branch")
        return &DetectionResult{}, nil // Not an error, just not applicable
    }
    return nil, fmt.Errorf("failed to get current branch: %w", err)
}

Validation: ✅ Independently confirmed and validated by expert

---
7. Brittle Error Checking with String Matching

Location: implementation-plan-part2.md:1020

Problem: Checking for user cancellation via strings.Contains(err.Error(),
"cancelled by user") is fragile.

Fix: Use sentinel errors:
// In internal/diff/auto_branch.go
var ErrOperationCancelled = errors.New("auto-branch cancelled by user")

// In prompt function when user says no:
return nil, ErrOperationCancelled

// In cmd/diff.go
if errors.Is(err, diff.ErrOperationCancelled) {
    // Show manual instructions
    return nil
}

Validation: ✅ Expert insight - excellent improvement I hadn't explicitly
detailed

---
8. Placeholder Validation Missing

Location: implementation-plan.md:108-123

Problem: Pattern validation checks invalid characters but not whether
placeholders are valid:
autoBranchNamePattern: "feature/{invalid_placeholder}" # Would pass validation

Fix: Validate known placeholders during config validation:
validPlaceholders := []string{"{timestamp}", "{date}", "{datetime}",
"{username}", "{random}"}
// Check that pattern only contains valid placeholders

Validation: ✅ Independently identified

---
9. Incomplete Error Handling for Edge Cases

Location: feature-design.md:187-193

Missing scenarios: Shallow clones, multiple remotes, protected branches,
submodules, worktrees

Fix: Document behavior and add detection for these cases in implementation

Validation: ✅ Independently identified

---
10. Test Coverage Gaps

Location: implementation-plan-part2.md:1420 (integration tests)

Missing test scenarios:
- Stash pop with conflicts
- Branch creation failure after stash
- PR creation failure after branch creation
- Network failures during reset
- Concurrent git operations

Fix: Add negative test cases for all failure paths in Task 20

Validation: ✅ Independently identified

---
🟡 MEDIUM SEVERITY ISSUES

11. Hardcoded "origin" Remote

Location: Throughout implementation (e.g., implementation-plan.md:928)

Problem: Assumes remote named "origin" exists, fails for users with different
remote names

Fix: Detect upstream tracking branch dynamically:
git rev-parse --abbrev-ref --symbolic-full-name @{u}

Validation: ✅ Both independently identified and validated by expert

---
12. Operations Not Idempotent

Problem: Running command twice after failure could cause issues

Fix: Check state before each operation, make operations safe to retry

Validation: ✅ Independently identified

---
13. No Dry-Run Mode

Problem: Destructive operations without preview option

Fix: Add --dry-run flag to show what would happen

Validation: ✅ Independently identified

---
14. Magic String "null" for Pattern

Location: feature-design.md:111

Problem: Using string literal "null" to trigger prompt is not immediately
obvious

Fix: Document exceptionally well (plan already does this), or add alternative
like "<prompt>"

Validation: ✅ Both independently identified and validated by expert

---
15-17. Additional Medium Issues

- Inconsistent error message severity handling
- Insufficient input sanitization across the board
- Sequential git operations (could optimize with parallelization)

Validation: ✅ Independently identified

---
✅ POSITIVE ASPECTS

1. Excellent structure: 22 tasks across 6 logical phases with atomic commits
2. User-centric design: Highly configurable (auto vs interactive modes)
3. Clear messaging: Transparent user-facing output and progress indicators
4. TDD approach: Tests alongside each implementation task
5. Delayed reset: Waits until after PR success (smart design choice)
6. Comprehensive documentation: README, ADR, architecture docs planned
7. Good error recovery: Manual instructions provided for failures
8. Structured logging: Proper use of zerolog throughout

---
🎯 TOP 3 PRIORITY FIXES

1. Fix Unsafe origin/main Fallback - Abort when remote branch not found to
prevent data loss
2. Implement Safe Stash Conflict Handling - Provide clear recovery instructions
for stash conflicts
3. Remove Recursive Prompts - Use iterative loops to prevent stack overflow

---
Summary

The design and implementation plan are exceptionally well-structured and
comprehensive. The breakdown into phases, detailed tasks with code examples,
tests, and commit messages demonstrates mature engineering practices. However,
there are 4 critical safety/security issues that must be addressed before
implementation begins.

The primary concerns are around:
- Data loss prevention (origin/main fallback, stash conflicts)
- Robustness (recursion limits, rollback mechanisms)
- Security (input sanitization, injection vulnerabilities)
- Edge case handling (detached HEAD, multiple remotes, conflicts)

With these issues addressed, this will be an excellent, production-ready feature
 that significantly improves the trunk-based development workflow for gh-arc
users.
