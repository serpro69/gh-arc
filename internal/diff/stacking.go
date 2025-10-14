package diff

import (
	"fmt"
	"strings"

	"github.com/serpro69/gh-arc/internal/github"
)

// FormatDependentPRWarning generates a user-friendly warning message about dependent PRs
// that target the current branch as their base
func FormatDependentPRWarning(dependentPRs []*github.PullRequest) string {
	if len(dependentPRs) == 0 {
		return ""
	}

	// Build PR reference list: #123, #124, #125
	var prRefs []string
	for _, pr := range dependentPRs {
		prRefs = append(prRefs, fmt.Sprintf("#%d", pr.Number))
	}

	count := len(dependentPRs)
	prList := strings.Join(prRefs, ", ")

	if count == 1 {
		return fmt.Sprintf("⚠️  Warning: 1 dependent PR targets this branch: %s", prList)
	}

	return fmt.Sprintf("⚠️  Warning: %d dependent PRs target this branch: %s", count, prList)
}

// ShowStackingStatus displays information about stacking relationships
// Shows which parent PR the current branch is stacking on
func ShowStackingStatus(parentBranch string, parentPR *github.PullRequest) string {
	if parentPR == nil {
		return fmt.Sprintf("Creating PR (base: %s)", parentBranch)
	}

	return fmt.Sprintf("📚 Stacking on %s (PR #%d: %s)", parentBranch, parentPR.Number, parentPR.Title)
}

// IsParentBranch checks if the given branch has any dependent PRs
// Returns true if there are PRs targeting this branch as their base
func IsParentBranch(dependentPRs []*github.PullRequest) bool {
	return len(dependentPRs) > 0
}

// FormatDependentPRList formats dependent PRs with more details for verbose output
// Includes PR number, title, and author
func FormatDependentPRList(dependentPRs []*github.PullRequest) string {
	if len(dependentPRs) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "Dependent PRs targeting this branch:")

	for _, pr := range dependentPRs {
		line := fmt.Sprintf("  • #%d: %s (by @%s)", pr.Number, pr.Title, pr.User.Login)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
