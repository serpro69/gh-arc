package format

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/serpro69/gh-arc/internal/github"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorGray   = "\033[90m"
	ColorBold   = "\033[1m"
)

// Status icons
const (
	IconApproved         = "✓"
	IconChangesRequested = "✗"
	IconPending          = "○"
	IconDraft            = "◐"
	IconSuccess          = "✓"
	IconFailure          = "✗"
	IconInProgress       = "⟳"
	IconNeutral          = "—"
)

// PRFormatterOptions contains options for formatting PR output
type PRFormatterOptions struct {
	UseColor      bool // Enable color output
	MaxTitleWidth int  // Maximum width for title column (0 = no limit)
	SortBy        string // Sort field: "updated", "created", "number"
	SortDesc      bool // Sort in descending order
	ShowSummary   bool // Show summary row at the end
}

// DefaultPRFormatterOptions returns options with sensible defaults
func DefaultPRFormatterOptions() *PRFormatterOptions {
	return &PRFormatterOptions{
		UseColor:      isTerminal(),
		MaxTitleWidth: 60,
		SortBy:        "updated",
		SortDesc:      true,
		ShowSummary:   true,
	}
}

// isTerminal checks if output is going to a terminal
func isTerminal() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// FormatPRTable formats a list of pull requests as a table
func FormatPRTable(prs []*github.PullRequest, opts *PRFormatterOptions) string {
	if opts == nil {
		opts = DefaultPRFormatterOptions()
	}

	// Sort PRs
	sortedPRs := make([]*github.PullRequest, len(prs))
	copy(sortedPRs, prs)
	sortPullRequests(sortedPRs, opts)

	// Create string builder for output
	var output strings.Builder

	// Create table
	table := tablewriter.NewWriter(&output)
	table.SetHeader([]string{"PR#", "Title", "Author", "Status", "Checks", "Branch", "Updated"})
	table.SetBorder(false)
	table.SetColumnSeparator("")
	table.SetCenterSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(false)

	// Count PRs by status
	statusCounts := make(map[string]int)

	// Add rows
	for _, pr := range sortedPRs {
		status := github.DeterminePRStatus(pr.Reviews, pr.Checks)

		// Track status counts
		statusCounts[status.ReviewStatus]++

		// Format columns
		number := fmt.Sprintf("#%d", pr.Number)
		title := formatTitle(pr.Title, pr.Draft, opts)
		author := pr.User.Login
		reviewStatus := formatReviewStatus(status.ReviewStatus, opts)
		checkStatus := formatCheckStatus(status.CheckStatus, opts)
		branch := formatBranch(pr.Head.Ref, pr.Base.Ref)
		updated := formatRelativeTime(pr.UpdatedAt)

		table.Append([]string{
			number,
			title,
			author,
			reviewStatus,
			checkStatus,
			branch,
			updated,
		})
	}

	table.Render()

	// Add summary if requested
	if opts.ShowSummary && len(prs) > 0 {
		output.WriteString("\n")
		output.WriteString(formatSummary(statusCounts, len(prs), opts))
	}

	return output.String()
}

// formatTitle formats the PR title with optional truncation and draft indicator
func formatTitle(title string, isDraft bool, opts *PRFormatterOptions) string {
	// Add draft indicator
	if isDraft {
		if opts.UseColor {
			title = colorize(IconDraft+" "+title, ColorGray)
		} else {
			title = "[DRAFT] " + title
		}
	}

	// Truncate if needed
	if opts.MaxTitleWidth > 0 && len(title) > opts.MaxTitleWidth {
		// Account for ANSI codes in length calculation
		visualLen := visualLength(title)
		if visualLen > opts.MaxTitleWidth {
			title = truncateString(title, opts.MaxTitleWidth-3) + "..."
		}
	}

	return title
}

// formatReviewStatus formats the review status with color and icon
func formatReviewStatus(status string, opts *PRFormatterOptions) string {
	var icon, color string

	switch status {
	case "approved":
		icon = IconApproved
		color = ColorGreen
	case "changes_requested":
		icon = IconChangesRequested
		color = ColorRed
	case "review_required":
		icon = IconPending
		color = ColorYellow
	case "commented":
		icon = IconPending
		color = ColorBlue
	case "pending":
		icon = IconPending
		color = ColorYellow
	default:
		icon = IconPending
		color = ColorGray
	}

	// Format the status name
	statusName := strings.ReplaceAll(status, "_", " ")
	statusName = strings.Title(statusName)

	if opts.UseColor {
		return colorize(icon+" "+statusName, color)
	}
	return icon + " " + statusName
}

// formatCheckStatus formats the check status with color and icon
func formatCheckStatus(status string, opts *PRFormatterOptions) string {
	var icon, color string

	switch status {
	case "success":
		icon = IconSuccess
		color = ColorGreen
	case "failure":
		icon = IconFailure
		color = ColorRed
	case "in_progress":
		icon = IconInProgress
		color = ColorYellow
	case "pending":
		icon = IconPending
		color = ColorGray
	case "neutral":
		icon = IconNeutral
		color = ColorGray
	default:
		icon = IconPending
		color = ColorGray
	}

	// Format the status name
	statusName := strings.ReplaceAll(status, "_", " ")
	statusName = strings.Title(statusName)

	if opts.UseColor {
		return colorize(icon+" "+statusName, color)
	}
	return icon + " " + statusName
}

// formatBranch formats the branch information
func formatBranch(head, base string) string {
	return fmt.Sprintf("%s → %s", head, base)
}

// formatRelativeTime formats a timestamp as relative time (e.g., "2 hours ago")
func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// formatSummary creates a summary line showing PR counts by status
func formatSummary(statusCounts map[string]int, total int, opts *PRFormatterOptions) string {
	var parts []string

	// Order: approved, changes_requested, review_required, commented, pending
	statusOrder := []string{"approved", "changes_requested", "review_required", "commented", "pending"}

	for _, status := range statusOrder {
		if count, ok := statusCounts[status]; ok && count > 0 {
			statusName := strings.ReplaceAll(status, "_", " ")
			part := fmt.Sprintf("%d %s", count, statusName)

			if opts.UseColor {
				var color string
				switch status {
				case "approved":
					color = ColorGreen
				case "changes_requested":
					color = ColorRed
				case "review_required", "pending":
					color = ColorYellow
				case "commented":
					color = ColorBlue
				default:
					color = ColorGray
				}
				part = colorize(part, color)
			}

			parts = append(parts, part)
		}
	}

	summary := fmt.Sprintf("Total: %d PRs", total)
	if len(parts) > 0 {
		summary += " (" + strings.Join(parts, ", ") + ")"
	}

	return summary
}

// sortPullRequests sorts PRs based on the specified field and direction
func sortPullRequests(prs []*github.PullRequest, opts *PRFormatterOptions) {
	sort.Slice(prs, func(i, j int) bool {
		var less bool

		switch opts.SortBy {
		case "created":
			less = prs[i].CreatedAt.Before(prs[j].CreatedAt)
		case "number":
			less = prs[i].Number < prs[j].Number
		case "updated":
			fallthrough
		default:
			less = prs[i].UpdatedAt.Before(prs[j].UpdatedAt)
		}

		// Reverse if descending
		if opts.SortDesc {
			return !less
		}
		return less
	})
}

// colorize applies ANSI color codes to text
func colorize(text, color string) string {
	return color + text + ColorReset
}

// visualLength returns the visual length of a string, excluding ANSI codes
func visualLength(s string) int {
	// Simple implementation that doesn't account for ANSI codes
	// For a more accurate implementation, we'd need to strip ANSI codes first
	inEscape := false
	length := 0

	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		length++
	}

	return length
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	// Try to truncate at a word boundary
	if maxLen > 10 {
		lastSpace := strings.LastIndex(s[:maxLen], " ")
		if lastSpace > maxLen/2 {
			return s[:lastSpace]
		}
	}

	return s[:maxLen]
}
