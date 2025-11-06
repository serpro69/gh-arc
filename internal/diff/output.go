package diff

import (
	"fmt"
	"strings"

	"github.com/muesli/termenv"
	"github.com/serpro69/gh-arc/internal/github"
	"github.com/serpro69/gh-arc/internal/template"
)

// OutputStyle defines the styling configuration for terminal output
type OutputStyle struct {
	profile     termenv.Profile
	useColor    bool
	errorStyle  termenv.Style
	warningStyle termenv.Style
	successStyle termenv.Style
	infoStyle   termenv.Style
	highlightStyle termenv.Style
	dimStyle    termenv.Style
}

// NewOutputStyle creates a new OutputStyle with appropriate color profile
func NewOutputStyle(useColor bool) *OutputStyle {
	profile := termenv.ColorProfile()
	if !useColor {
		profile = termenv.Ascii
	}

	return &OutputStyle{
		profile:        profile,
		useColor:       useColor,
		errorStyle:     termenv.Style{}.Foreground(profile.Color("1")),   // Red
		warningStyle:   termenv.Style{}.Foreground(profile.Color("3")),   // Yellow
		successStyle:   termenv.Style{}.Foreground(profile.Color("2")),   // Green
		infoStyle:      termenv.Style{}.Foreground(profile.Color("4")),   // Blue
		highlightStyle: termenv.Style{}.Foreground(profile.Color("6")),   // Cyan
		dimStyle:       termenv.Style{}.Faint(),
	}
}

// Error formats an error message with styling
func (s *OutputStyle) Error(message string) string {
	if !s.useColor {
		return fmt.Sprintf("✗ %s", message)
	}
	return fmt.Sprintf("%s %s", s.errorStyle.Styled("✗"), s.errorStyle.Styled(message))
}

// Warning formats a warning message with styling
func (s *OutputStyle) Warning(message string) string {
	if !s.useColor {
		return fmt.Sprintf("⚠ %s", message)
	}
	return fmt.Sprintf("%s %s", s.warningStyle.Styled("⚠"), s.warningStyle.Styled(message))
}

// Success formats a success message with styling
func (s *OutputStyle) Success(message string) string {
	if !s.useColor {
		return fmt.Sprintf("✓ %s", message)
	}
	return fmt.Sprintf("%s %s", s.successStyle.Styled("✓"), s.successStyle.Styled(message))
}

// Info formats an info message with styling
func (s *OutputStyle) Info(message string) string {
	if !s.useColor {
		return fmt.Sprintf("ℹ %s", message)
	}
	return fmt.Sprintf("%s %s", s.infoStyle.Styled("ℹ"), s.infoStyle.Styled(message))
}

// Highlight highlights text
func (s *OutputStyle) Highlight(text string) string {
	if !s.useColor {
		return text
	}
	return s.highlightStyle.Styled(text)
}

// Dim dims text
func (s *OutputStyle) Dim(text string) string {
	if !s.useColor {
		return text
	}
	return s.dimStyle.Styled(text)
}

// Stack formats a stacking indicator
func (s *OutputStyle) Stack(message string) string {
	if !s.useColor {
		return fmt.Sprintf("📚 %s", message)
	}
	// Purple/Magenta color for stack indicator
	stackStyle := termenv.Style{}.Foreground(s.profile.Color("5"))
	return fmt.Sprintf("%s %s", stackStyle.Styled("📚"), message)
}

// FormatStackingOutput formats the stacking hierarchy for display
// Shows the relationship between current PR and parent/dependent PRs
func FormatStackingOutput(currentBranch string, baseBranch string, parentPR *github.PullRequest, dependentPRs []*github.PullRequest, style *OutputStyle) string {
	var lines []string

	// Show stack header
	lines = append(lines, style.Stack("Stacking Information"))
	lines = append(lines, "")

	// Show current branch targeting base
	if parentPR != nil {
		// Stacked on parent PR
		lines = append(lines, fmt.Sprintf("  Current: %s", style.Highlight(currentBranch)))
		lines = append(lines, fmt.Sprintf("       ↓"))
		lines = append(lines, fmt.Sprintf("  Parent:  %s %s",
			style.Highlight(baseBranch),
			style.Dim(fmt.Sprintf("(PR #%d: %s)", parentPR.Number, parentPR.Title))))
	} else {
		// Targeting trunk
		lines = append(lines, fmt.Sprintf("  Branch: %s", style.Highlight(currentBranch)))
		lines = append(lines, fmt.Sprintf("      ↓"))
		lines = append(lines, fmt.Sprintf("  Base:   %s %s", style.Highlight(baseBranch), style.Dim("(trunk)")))
	}

	// Show dependent PRs if any
	if len(dependentPRs) > 0 {
		lines = append(lines, "")
		if len(dependentPRs) == 1 {
			lines = append(lines, style.Warning(fmt.Sprintf("1 dependent PR targets this branch:")))
		} else {
			lines = append(lines, style.Warning(fmt.Sprintf("%d dependent PRs target this branch:", len(dependentPRs))))
		}
		for _, dep := range dependentPRs {
			lines = append(lines, fmt.Sprintf("    • PR #%d: %s %s",
				dep.Number,
				dep.Title,
				style.Dim(fmt.Sprintf("by @%s", dep.User.Login))))
		}
	}

	return strings.Join(lines, "\n")
}

// FormatPRCreated formats the success message for PR creation with stacking info
func FormatPRCreated(pr *github.PullRequest, parentPR *github.PullRequest, style *OutputStyle) string {
	var lines []string

	// Success header
	if parentPR != nil {
		lines = append(lines, style.Success(fmt.Sprintf("Created stacked PR #%d", pr.Number)))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  %s %s",
			style.Stack("Stacking on:"),
			style.Dim(fmt.Sprintf("PR #%d (%s)", parentPR.Number, parentPR.Base.Ref))))
	} else {
		lines = append(lines, style.Success(fmt.Sprintf("Created PR #%d", pr.Number)))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Title:  %s", pr.Title))
	lines = append(lines, fmt.Sprintf("  Branch: %s → %s",
		style.Highlight(pr.Head.Ref),
		style.Highlight(pr.Base.Ref)))
	lines = append(lines, fmt.Sprintf("  URL:    %s", style.Highlight(pr.HTMLURL)))

	if pr.Draft {
		lines = append(lines, "")
		lines = append(lines, style.Info("PR is in draft state"))
	}

	return strings.Join(lines, "\n")
}

// FormatPRUpdated formats the success message for PR update with stacking info
func FormatPRUpdated(pr *github.PullRequest, baseChanged bool, oldBase, newBase string, style *OutputStyle) string {
	var lines []string

	lines = append(lines, style.Success(fmt.Sprintf("Updated PR #%d", pr.Number)))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Title: %s", pr.Title))
	lines = append(lines, fmt.Sprintf("  URL:   %s", style.Highlight(pr.HTMLURL)))

	if baseChanged {
		lines = append(lines, "")
		lines = append(lines, style.Stack(fmt.Sprintf("Updated base branch: %s → %s",
			oldBase,
			newBase)))
	}

	return strings.Join(lines, "\n")
}

// FormatStackWarning formats a warning about stack operations
func FormatStackWarning(message string, details []string, style *OutputStyle) string {
	var lines []string

	lines = append(lines, style.Warning(message))
	if len(details) > 0 {
		lines = append(lines, "")
		for _, detail := range details {
			lines = append(lines, fmt.Sprintf("  • %s", detail))
		}
	}

	return strings.Join(lines, "\n")
}

// FormatStackConfirmation formats a confirmation prompt for stack operations
func FormatStackConfirmation(operation, currentBranch, baseBranch string, parentPR *github.PullRequest, style *OutputStyle) string {
	var lines []string

	lines = append(lines, style.Info(fmt.Sprintf("About to %s:", operation)))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Branch: %s", style.Highlight(currentBranch)))

	if parentPR != nil {
		lines = append(lines, fmt.Sprintf("  Base:   %s %s",
			style.Highlight(baseBranch),
			style.Dim(fmt.Sprintf("(PR #%d)", parentPR.Number))))
		lines = append(lines, "")
		lines = append(lines, style.Stack(fmt.Sprintf("This will create a stacked PR on #%d", parentPR.Number)))
	} else {
		lines = append(lines, fmt.Sprintf("  Base:   %s", style.Highlight(baseBranch)))
	}

	return strings.Join(lines, "\n")
}

// FormatDryRunOutput formats the output for --dry-run mode
func FormatDryRunOutput(currentBranch, detectedBase string, parentPR *github.PullRequest, dependentPRs []*github.PullRequest, analysis *template.CommitAnalysis, style *OutputStyle) string {
	var lines []string

	lines = append(lines, style.Info("Dry run mode - no changes will be made"))
	lines = append(lines, "")
	lines = append(lines, style.Highlight("Detected configuration:"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Current branch: %s", style.Highlight(currentBranch)))
	lines = append(lines, fmt.Sprintf("  Detected base:  %s", style.Highlight(detectedBase)))

	if parentPR != nil {
		lines = append(lines, "")
		lines = append(lines, style.Stack("Stacking detected:"))
		lines = append(lines, fmt.Sprintf("  Parent PR: #%d - %s", parentPR.Number, parentPR.Title))
		lines = append(lines, fmt.Sprintf("  State:     %s", parentPR.State))
		if parentPR.Draft {
			lines = append(lines, style.Warning("  Warning: Parent PR is in draft state"))
		}
	}

	if len(dependentPRs) > 0 {
		lines = append(lines, "")
		lines = append(lines, style.Warning(fmt.Sprintf("Dependent PRs (%d):", len(dependentPRs))))
		for _, dep := range dependentPRs {
			lines = append(lines, fmt.Sprintf("  • #%d: %s", dep.Number, dep.Title))
		}
	}

	if analysis != nil {
		lines = append(lines, "")
		lines = append(lines, style.Highlight("Proposed PR content:"))
		lines = append(lines, fmt.Sprintf("  Title:        %s", analysis.Title))
		lines = append(lines, fmt.Sprintf("  Commit count: %d", analysis.CommitCount))
		if analysis.HasMergeCommits {
			lines = append(lines, style.Warning("  Has merge commits"))
		}
	}

	return strings.Join(lines, "\n")
}

// FormatProgressIndicator formats a progress indicator for operations
func FormatProgressIndicator(operation string, style *OutputStyle) string {
	return style.Dim(fmt.Sprintf("⏳ %s...", operation))
}

// FormatErrorWithContext formats an error with stacking context
func FormatErrorWithContext(err error, style *OutputStyle) string {
	var lines []string

	// Check for specific error types
	switch e := err.(type) {
	case *github.CircularDependencyError:
		lines = append(lines, style.Error("Circular dependency detected"))
		lines = append(lines, "")
		if len(e.Branches) > 0 {
			lines = append(lines, "  Dependency chain:")
			branchChain := "    " + strings.Join(e.Branches, " → ")
			lines = append(lines, branchChain)
		}
		if e.CurrentPR > 0 && e.ConflictingPR > 0 {
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("  Current PR: #%d", e.CurrentPR))
			lines = append(lines, fmt.Sprintf("  Conflicts with: #%d", e.ConflictingPR))
		}
		lines = append(lines, "")
		lines = append(lines, style.Dim("Tip: Check your branch structure to resolve the circular dependency"))

	case *github.InvalidBaseError:
		lines = append(lines, style.Error("Invalid base branch"))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  Branch: %s", style.Highlight(e.BaseBranch)))
		if len(e.ValidBases) > 0 {
			lines = append(lines, "")
			lines = append(lines, "  Valid bases:")
			for _, base := range e.ValidBases {
				lines = append(lines, fmt.Sprintf("    • %s", base))
			}
		}
		lines = append(lines, "")
		lines = append(lines, style.Dim("Tip: Use --base flag to specify a different base branch"))

	case *github.ParentPRConflictError:
		lines = append(lines, style.Error("Parent PR conflict"))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  Parent PR: #%d", e.ParentPR))
		lines = append(lines, fmt.Sprintf("  State:     %s", e.ParentState))
		if e.Reason != "" {
			lines = append(lines, fmt.Sprintf("  Reason:    %s", e.Reason))
		}
		lines = append(lines, "")
		if e.ParentState == "closed" || e.ParentState == "merged" {
			lines = append(lines, style.Dim("Tip: Rebase onto trunk or choose a different parent branch"))
		}

	case *github.StackingError:
		lines = append(lines, style.Error("Stacking error"))
		lines = append(lines, "")
		if e.Operation != "" {
			lines = append(lines, fmt.Sprintf("  Operation: %s", e.Operation))
		}
		lines = append(lines, fmt.Sprintf("  Branch:    %s → %s", e.CurrentBranch, e.BaseBranch))
		if len(e.Context) > 0 {
			lines = append(lines, "")
			lines = append(lines, "  Context:")
			for k, v := range e.Context {
				lines = append(lines, fmt.Sprintf("    %s: %v", k, v))
			}
		}
		lines = append(lines, "")
		lines = append(lines, style.Dim(e.Message))

	default:
		// Generic error formatting
		lines = append(lines, style.Error(err.Error()))
	}

	return strings.Join(lines, "\n")
}

// FormatAutoBranchSuccess formats the success message for auto-branch creation
func FormatAutoBranchSuccess(branchName string, checkoutFailed bool, style *OutputStyle) string {
	var lines []string

	lines = append(lines, style.Success(fmt.Sprintf("Created feature branch: %s", style.Highlight(branchName))))

	if checkoutFailed {
		lines = append(lines, "")
		lines = append(lines, style.Warning("Failed to checkout branch automatically"))
		lines = append(lines, "")
		lines = append(lines, "  Manual checkout:")
		lines = append(lines, fmt.Sprintf("    git checkout %s", branchName))
	}

	return strings.Join(lines, "\n")
}

// FormatAutoBranchWarning formats the warning when commits are detected on main
func FormatAutoBranchWarning(commitsAhead int, defaultBranch string, branchName string, style *OutputStyle) string {
	var lines []string

	lines = append(lines, style.Warning(fmt.Sprintf("You have %d commit(s) on %s", commitsAhead, defaultBranch)))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Will create feature branch: %s", style.Highlight(branchName)))

	return strings.Join(lines, "\n")
}

// FormatFastPathOutput formats the output for fast path (existing PR, no edit)
func FormatFastPathOutput(pr *github.PullRequest, messages []string, style *OutputStyle) string {
	var lines []string

	lines = append(lines, style.Success(fmt.Sprintf("PR #%d already exists", pr.Number)))
	lines = append(lines, fmt.Sprintf("  %s", pr.HTMLURL))

	if len(messages) > 0 {
		lines = append(lines, "")
		for _, msg := range messages {
			lines = append(lines, fmt.Sprintf("  %s", msg))
		}
	}

	return strings.Join(lines, "\n")
}

// FormatValidationErrors formats template validation errors
func FormatValidationErrors(errors []string, templatePath string, style *OutputStyle) string {
	var lines []string

	lines = append(lines, style.Error("Template validation failed:"))
	for _, err := range errors {
		lines = append(lines, fmt.Sprintf("  • %s", err))
	}

	if templatePath != "" {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Template saved to: %s", templatePath))
	}

	lines = append(lines, "")
	lines = append(lines, "Fix the issues and run:")
	lines = append(lines, "  gh arc diff --continue")

	return strings.Join(lines, "\n")
}

// FormatReviewersAssigned formats the message for assigned reviewers
func FormatReviewersAssigned(reviewers []string, style *OutputStyle) string {
	if len(reviewers) == 0 {
		return ""
	}

	return style.Success(fmt.Sprintf("Assigned reviewers: %s", strings.Join(reviewers, ", ")))
}

// FormatDiffResult formats the complete diff workflow result
func FormatDiffResult(result *DiffResult, style *OutputStyle) string {
	var lines []string

	// PR created or updated
	if result.WasCreated {
		lines = append(lines, FormatPRCreated(result.PR, result.ParentPR, style))
	} else {
		lines = append(lines, FormatPRUpdated(result.PR, false, "", result.BaseBranch, style))
	}

	// Auto-branch info
	if result.AutoBranchUsed {
		lines = append(lines, "")
		lines = append(lines, FormatAutoBranchSuccess(result.AutoBranchName, result.AutoBranchCheckoutFailed, style))
	}

	// Draft status change
	if result.DraftChanged {
		lines = append(lines, "")
		if result.PR.Draft {
			lines = append(lines, style.Info("Converted PR to draft"))
		} else {
			lines = append(lines, style.Info("Marked PR as ready for review"))
		}
	}

	// Reviewers
	if len(result.ReviewersAdded) > 0 {
		lines = append(lines, "")
		lines = append(lines, FormatReviewersAssigned(result.ReviewersAdded, style))
	}

	// Additional messages
	if len(result.Messages) > 0 {
		lines = append(lines, "")
		for _, msg := range result.Messages {
			lines = append(lines, fmt.Sprintf("  %s", msg))
		}
	}

	// Stacking info
	if result.IsStacking && result.ParentPR != nil {
		lines = append(lines, "")
		lines = append(lines, style.Stack(fmt.Sprintf("Stacked on PR #%d", result.ParentPR.Number)))
	}

	lines = append(lines, "")
	lines = append(lines, style.Success("Success!"))

	return strings.Join(lines, "\n")
}

// FormatPushingBranch formats the message for pushing a branch
func FormatPushingBranch(branchName string, style *OutputStyle) string {
	return style.Success(fmt.Sprintf("Pushing branch %s to remote...", style.Highlight(branchName)))
}

// FormatPushedSuccessfully formats the success message after pushing
func FormatPushedSuccessfully(style *OutputStyle) string {
	return style.Success("Branch pushed successfully")
}

// FormatNoNewCommits formats the message when there are no new commits to push
func FormatNoNewCommits(style *OutputStyle) string {
	return style.Info("No new commits to push")
}

// FormatBaseChanged formats the message when PR base branch changed
func FormatBaseChanged(oldBase, newBase string, style *OutputStyle) string {
	return style.Warning(fmt.Sprintf("Base branch changed: %s → %s", oldBase, newBase))
}
