● Code Review Complete: Auto-Branch-From-Main Feature

I've completed a comprehensive review of both the feature design and implementation plan documents using gemini-2.5-pro. Here's a synthesis of the findings:

---
🎯 Overall Assessment

Quality: HIGH - The design demonstrates excellent safety principles by avoiding destructive operations like git reset --hard and stash operations. The documentation is
thorough, well-structured, and provides clear rationale for design decisions. However, there are critical issues with workflow ordering and several edge cases that need
 addressing.

---
🔴 CRITICAL ISSUES

1. Workflow Order Contradiction Between Documents

Location: implementation-plan.md:397 vs feature-design.md:154-160

Problem: The documents contradict each other on operation order:
- Design document (correct): Push branch → Create PR → Checkout locally
- Implementation plan (incorrect): Create PR → Push branch → Checkout locally

This is blocking because you cannot create a PR from a branch that doesn't exist on the remote yet!

Fix Required:
Update Task 10 in the implementation plan to match the design:
1. After PR template is filled → Push branch to remote
2. Then → Create PR via GitHub API
3. Finally → Checkout local tracking branch

This also requires restructuring error handling for each step.

---
🟠 HIGH PRIORITY ISSUES

2. Orphaned Remote Branch on Partial Failure

Location: feature-design.md:154-160 (steps 6-7)

Problem: If push succeeds but PR creation fails (API outage, rate limit, auth token expires), the user is left with:
- A remote branch with commits but no PR
- Still on main branch locally
- No clear path forward

Current handling (line 182): "Stay on main, display error" - insufficient.

Recommended Fix:
✗ Failed to create Pull Request: <GitHub API Error>

✓ Branch 'feature/auto-from-main-1697654321' was successfully pushed to remote.
  You can create the PR manually by running:
    gh pr create --head feature/auto-from-main-1697654321 --base main

  Or delete the remote branch and start over:
    git push origin --delete feature/auto-from-main-1697654321

3. Race Condition with Stale Remote Tracking

Location: feature-design.md:67-84 (detection logic)

Problem: Detection compares local main to local origin/main tracking branch. If the user hasn't run git fetch recently, this tracking branch could be arbitrarily stale.
 This leads to:
- Creating PRs based on outdated code
- Immediate merge conflicts
- Confusion about what commits are "ahead"

While "Remote Sync" is listed as a non-goal (line 56), the risk should be mitigated.

Recommended Fix:
Before detection, check the age of the origin/main ref:
⚠️  Warning: Your local view of 'origin/main' was last updated 2 days ago.
   It's recommended to run 'git fetch origin' to ensure your new branch
   is based on the latest code.

? Continue anyway? (y/N)

4. Race Condition in Branch Name Uniqueness

Location: implementation-plan.md:275-279 (Task 7)

Problem: Classic TOCTOU (time-of-check-time-of-use) issue:
1. Check if branch exists → doesn't exist
2. [Another developer pushes branch with same name]
3. Try to push → fails

Current error handling treats this as a generic push failure.

Recommended Fix:
In PushBranch (Task 5), specifically detect the "remote ref already exists" error. If detected, automatically retry with an incremented counter (loop back to
EnsureUniqueBranchName). Add retry limit (e.g., 3 attempts).

---
🟡 MEDIUM PRIORITY ISSUES

5. Missing Commit Validation

Location: feature-design.md:121-172 (workflow execution)

Problem: The tool automatically includes ALL commits ahead of origin/main. Users don't get to review what commits are being moved. They might have:
- Temporary/WIP commits they don't want in the PR
- Multiple logical changes that should be separate PRs
- Sensitive data they committed by mistake

Recommended Fix:
After detection, show commit list with confirmation:
The following 2 commits on 'main' will be moved to a new feature branch:
  - a1b2c3d (HEAD -> main) feat: Add new login button
  - d4e5f67 fix: Correct typo in user model

? Proceed with creating a new branch and PR? (Y/n)

6. Brittle Error Checking

Location: implementation-plan.md:459 (Task 11)

Problem: Using strings.Contains(err.Error(), "cancelled by user") is fragile:
- Breaks if error message wording changes
- Not idiomatic Go error handling
- Can't use errors.Is() or errors.As()

Recommended Fix:
Define sentinel error in internal/diff/auto_branch.go:
var ErrOperationCancelled = errors.New("operation cancelled by user")
Return this from prompt functions, check with errors.Is(err, diff.ErrOperationCancelled).

7. Authentication Failures Not Addressed

Location: Throughout error handling sections

Problem: Git push and GitHub API calls require authentication, but no error scenarios mention:
- Expired credentials
- Missing SSH keys
- Insufficient permissions

Recommended Fix:
Add to error handling table (feature-design.md:176):
| Auth failure (push)   | git push exits 128      | Check gh auth status, provide login instructions |
| Auth failure (API)    | GitHub returns 401/403  | Run 'gh auth refresh', check scopes              |

8. Integration Test Coverage Gaps

Location: implementation-plan.md:490-528 (Task 12)

Problem: Only 4 integration tests listed, missing critical error scenarios:
- Network failure during push
- PR creation failure after successful push
- Auth failure at each step
- Stale remote handling
- Push succeeding but checkout failing

Recommended Fix:
Expand Task 12 to include 8-10 integration tests covering all error paths from the error handling table.

---
🟢 LOW PRIORITY ISSUES

9. Branch Counter Limit Documentation

Location: implementation-plan.md:277 (Task 7)

Issue: 100-attempt limit is arbitrary. Error message should be clear if hit.

Fix: Add note that error should say: "Failed to find unique branch name after 100 attempts. Please check remote for branches matching pattern."

10. Improved User Messaging for Main State

Location: feature-design.md:168-171

Issue: Message "Your local 'main' branch is still ahead of origin/main" might confuse users unfamiliar with git internals.

Recommendation: Consider adding a helper command like gh arc sync-main or auto-syncing on next operation to reduce cognitive load.

11. ADR Lacks Alternative Analysis

Location: implementation-plan.md:574-607 (Task 14)

Issue: ADR should detail WHY alternatives were rejected (e.g., why not git reset --hard?).

Fix: Expand ADR to include 2-3 alternatives with pros/cons explaining the rejection rationale.

12. Task Dependencies Not Explicit

Location: Throughout Phase 2 and 3

Issue: Task 7 uses Task 4's BranchExists(), but this dependency isn't explicit.

Recommendation: Add "Dependencies" section to each task or create a dependency graph.

13. Pattern Validation Edge Case

Location: implementation-plan.md:59-81 (Task 2)

Issue: Pattern validation checks for invalid characters but doesn't validate that a pattern can actually generate a valid branch name (e.g., {username} where username
is all invalid chars).

Fix: Add test case to validate generated names are git-compatible.

14. Configuration Migration Not Documented

Location: Throughout

Issue: No mention of how existing .arc.json files without new fields will behave.

Recommendation: Add explicit backwards compatibility note: "New fields have defaults; existing configs work without updates."

---
⭐ TOP 3 PRIORITIES FOR IMMEDIATE ACTION

1. Fix workflow order contradiction (Critical) - Implementation plan must match design document's push-before-PR order
2. Handle orphaned remote branch scenario (High) - Provide clear recovery path when push succeeds but PR creation fails
3. Mitigate stale remote tracking risk (High) - Warn users if origin/main is significantly out of date

---
✅ POSITIVE ASPECTS

1. Safety First: Eliminating git reset --hard and stash operations is excellent - builds trust and prevents data loss
2. Exceptional Documentation: Both documents are clear, detailed, and well-structured - a model for feature planning
3. Comprehensive Testing Strategy: Good mix of unit, integration, and manual testing
4. User-Centric Design: Thoughtful prompts, error messages, and configurability options
5. Structured Implementation: 14 well-defined tasks with clear deliverables

---
📋 RECOMMENDED NEXT STEPS

1. Immediately: Fix the critical workflow order issue in implementation plan (Task 10)
2. Before implementation: Address all HIGH priority issues in both documents
3. During implementation: Use MEDIUM priority fixes as acceptance criteria
4. Before PR: Ensure all integration tests from expanded Task 12 are passing
5. Post-implementation: Create comprehensive ADR with alternatives analysis

The feature is well-designed and ready for implementation once these issues are addressed. The safety-first approach is commendable and should be maintained throughout.

