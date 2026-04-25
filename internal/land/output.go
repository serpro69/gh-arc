package land

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/muesli/termenv"
	"github.com/serpro69/gh-arc/internal/github"
)

// LandResult represents the outcome of a land workflow execution.
type LandResult struct {
	PR               *github.PullRequest
	MergeMethod      string
	MergeCommitSHA   string
	DefaultBranch    string
	DeletedBranch    string
	DeletedBranchSHA string
	DependentPRCount int
	CleanupWarnings  []string
	Messages         []string
}

// OutputStyle handles styled terminal output for the land workflow.
// Unlike diff's OutputStyle which returns strings, land's Print* methods
// write directly to the configured writer for real-time progress output.
type OutputStyle struct {
	profile      termenv.Profile
	useColor     bool
	writer       io.Writer
	errorStyle   termenv.Style
	warningStyle termenv.Style
	successStyle termenv.Style
}

// NewOutputStyle creates a new OutputStyle with appropriate color profile.
func NewOutputStyle(useColor bool) *OutputStyle {
	profile := termenv.ColorProfile()
	if !useColor {
		profile = termenv.Ascii
	}

	return &OutputStyle{
		profile:      profile,
		useColor:     useColor,
		writer:       os.Stdout,
		errorStyle:   termenv.Style{}.Foreground(profile.Color("1")),
		warningStyle: termenv.Style{}.Foreground(profile.Color("3")),
		successStyle: termenv.Style{}.Foreground(profile.Color("2")),
	}
}

// PrintStep prints a step with an icon prefix (✓, ✗, or ⚠).
func (o *OutputStyle) PrintStep(icon, message string) {
	fmt.Fprintln(o.writer, o.formatWithIcon(icon, message))
}

// PrintDetail prints an indented detail/guidance line beneath a step.
func (o *OutputStyle) PrintDetail(message string) {
	fmt.Fprintf(o.writer, "  %s\n", message)
}

func (o *OutputStyle) formatWithIcon(icon, message string) string {
	if !o.useColor {
		return fmt.Sprintf("%s %s", icon, message)
	}
	var style termenv.Style
	switch icon {
	case "✓":
		style = o.successStyle
	case "✗":
		style = o.errorStyle
	case "⚠":
		style = o.warningStyle
	default:
		return fmt.Sprintf("%s %s", icon, message)
	}
	return fmt.Sprintf("%s %s", style.Styled(icon), style.Styled(message))
}

// PrintPRFound prints the step for locating the PR on the current branch.
func (o *OutputStyle) PrintPRFound(pr *github.PullRequest) {
	o.PrintStep("✓", fmt.Sprintf("Found PR #%d: %q (%s → %s)",
		pr.Number, pr.Title, pr.Head.Ref, pr.Base.Ref))
}

// PrintApprovalStatus prints the approval check result.
func (o *OutputStyle) PrintApprovalStatus(passed bool, message string) {
	if passed {
		o.PrintStep("✓", message)
	} else {
		o.PrintStep("✗", message)
	}
}

// PrintCIStatus prints the CI check result.
func (o *OutputStyle) PrintCIStatus(passed bool, message string) {
	if passed {
		o.PrintStep("✓", message)
	} else {
		o.PrintStep("✗", message)
	}
}

// PrintDependentPRs prints a warning about dependent PRs that will be retargeted.
func (o *OutputStyle) PrintDependentPRs(count int) {
	if count == 0 {
		return
	}
	if count == 1 {
		o.PrintStep("⚠", "1 dependent PR targets this branch — will be retargeted after merge")
	} else {
		o.PrintStep("⚠", fmt.Sprintf("%d dependent PRs target this branch — will be retargeted after merge", count))
	}
}

// PrintMerged prints the successful merge step.
func (o *OutputStyle) PrintMerged(method, baseBranch, sha string) {
	shortSHA := truncateSHA(sha)
	verb := "Squash-merged"
	if method == "rebase" {
		verb = "Rebased"
	}
	o.PrintStep("✓", fmt.Sprintf("%s into %s (%s)", verb, baseBranch, shortSHA))
}

// PrintCheckout prints the checkout and pull step.
func (o *OutputStyle) PrintCheckout(branch string) {
	o.PrintStep("✓", fmt.Sprintf("Switched to %s, pulled latest", branch))
}

// PrintBranchDeleted prints the branch deletion step with a restore command.
func (o *OutputStyle) PrintBranchDeleted(branch, sha string) {
	shortSHA := truncateSHA(sha)
	o.PrintStep("✓", fmt.Sprintf("Deleted local branch %s (use git checkout -b %s %s to restore)",
		branch, branch, shortSHA))
}

// PrintCleanupWarning prints a non-fatal cleanup warning.
func (o *OutputStyle) PrintCleanupWarning(message string) {
	o.PrintStep("⚠", message)
}

// FormatLandResult formats a compact final summary from the land result.
// The detailed step-by-step output is printed in real-time by Print* methods
// during workflow execution; this produces a summary for the end of output.
func FormatLandResult(result *LandResult, style *OutputStyle) string {
	var lines []string

	if result.PR != nil {
		verb := "squash-merged"
		if result.MergeMethod == "rebase" {
			verb = "rebased"
		}
		shortSHA := truncateSHA(result.MergeCommitSHA)
		lines = append(lines, style.formatWithIcon("✓",
			fmt.Sprintf("PR #%d %s into %s (%s)",
				result.PR.Number, verb, result.DefaultBranch, shortSHA)))
	}

	if result.DeletedBranch != "" {
		shortSHA := truncateSHA(result.DeletedBranchSHA)
		lines = append(lines, style.formatWithIcon("✓",
			fmt.Sprintf("Cleaned up branch %s (restore: git checkout -b %s %s)",
				result.DeletedBranch, result.DeletedBranch, shortSHA)))
	}

	if result.DependentPRCount > 0 {
		noun := "PR"
		if result.DependentPRCount > 1 {
			noun = "PRs"
		}
		lines = append(lines, style.formatWithIcon("⚠",
			fmt.Sprintf("%d dependent %s will be retargeted", result.DependentPRCount, noun)))
	}

	for _, w := range result.CleanupWarnings {
		lines = append(lines, style.formatWithIcon("⚠", w))
	}

	return strings.Join(lines, "\n")
}

func truncateSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
