package format

import (
	"strings"
	"testing"
	"time"

	"github.com/serpro69/gh-arc/internal/github"
)

func TestDefaultPRFormatterOptions(t *testing.T) {
	opts := DefaultPRFormatterOptions()

	if opts.MaxTitleWidth != 60 {
		t.Errorf("Expected MaxTitleWidth 60, got %d", opts.MaxTitleWidth)
	}

	if opts.SortBy != "updated" {
		t.Errorf("Expected SortBy 'updated', got %s", opts.SortBy)
	}

	if !opts.SortDesc {
		t.Error("Expected SortDesc to be true")
	}

	if !opts.ShowSummary {
		t.Error("Expected ShowSummary to be true")
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{"just now", now.Add(-30 * time.Second), "just now"},
		{"1 minute ago", now.Add(-1 * time.Minute), "1 minute ago"},
		{"5 minutes ago", now.Add(-5 * time.Minute), "5 minutes ago"},
		{"1 hour ago", now.Add(-1 * time.Hour), "1 hour ago"},
		{"3 hours ago", now.Add(-3 * time.Hour), "3 hours ago"},
		{"1 day ago", now.Add(-24 * time.Hour), "1 day ago"},
		{"3 days ago", now.Add(-72 * time.Hour), "3 days ago"},
		{"1 week ago", now.Add(-7 * 24 * time.Hour), "1 week ago"},
		{"2 weeks ago", now.Add(-14 * 24 * time.Hour), "2 weeks ago"},
		{"1 month ago", now.Add(-30 * 24 * time.Hour), "1 month ago"},
		{"3 months ago", now.Add(-90 * 24 * time.Hour), "3 months ago"},
		{"1 year ago", now.Add(-365 * 24 * time.Hour), "1 year ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRelativeTime(tt.time)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFormatReviewStatus(t *testing.T) {
	opts := &PRFormatterOptions{UseColor: false}

	tests := []struct {
		status   string
		expected string
	}{
		{"approved", IconApproved + " Approved"},
		{"changes_requested", IconChangesRequested + " Changes Requested"},
		{"review_required", IconPending + " Review Required"},
		{"commented", IconPending + " Commented"},
		{"pending", IconPending + " Pending"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := formatReviewStatus(tt.status, opts)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFormatReviewStatusWithColor(t *testing.T) {
	opts := &PRFormatterOptions{UseColor: true}

	tests := []struct {
		status        string
		expectIcon    string
		expectInColor bool
	}{
		{"approved", IconApproved, true},
		{"changes_requested", IconChangesRequested, true},
		{"review_required", IconPending, true},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := formatReviewStatus(tt.status, opts)

			// Should contain the icon
			if !strings.Contains(result, tt.expectIcon) {
				t.Errorf("Expected result to contain icon %s", tt.expectIcon)
			}

			// Should contain color codes if color is enabled
			if tt.expectInColor && !strings.Contains(result, "\033[") {
				t.Error("Expected result to contain ANSI color codes")
			}

			// Should contain reset code
			if tt.expectInColor && !strings.Contains(result, ColorReset) {
				t.Error("Expected result to contain color reset code")
			}
		})
	}
}

func TestFormatCheckStatus(t *testing.T) {
	opts := &PRFormatterOptions{UseColor: false}

	tests := []struct {
		status   string
		expected string
	}{
		{"success", IconSuccess + " Success"},
		{"failure", IconFailure + " Failure"},
		{"in_progress", IconInProgress + " In Progress"},
		{"pending", IconPending + " Pending"},
		{"neutral", IconNeutral + " Neutral"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := formatCheckStatus(tt.status, opts)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFormatBranch(t *testing.T) {
	tests := []struct {
		head     string
		base     string
		expected string
	}{
		{"feature-branch", "main", "feature-branch → main"},
		{"fix/bug-123", "develop", "fix/bug-123 → develop"},
		{"test", "master", "test → master"},
	}

	for _, tt := range tests {
		t.Run(tt.head+"->"+tt.base, func(t *testing.T) {
			result := formatBranch(tt.head, tt.base)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFormatTitle(t *testing.T) {
	opts := &PRFormatterOptions{
		UseColor:      false,
		MaxTitleWidth: 30,
	}

	tests := []struct {
		name     string
		title    string
		isDraft  bool
		expected string
	}{
		{
			name:     "short title",
			title:    "Fix bug",
			isDraft:  false,
			expected: "Fix bug",
		},
		{
			name:     "draft title",
			title:    "Work in progress",
			isDraft:  true,
			expected: "[DRAFT] Work in progress",
		},
		{
			name:     "long title truncated",
			title:    "This is a very long pull request title that should be truncated",
			isDraft:  false,
			expected: "This is a very long pull...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTitle(tt.title, tt.isDraft, opts)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFormatSummary(t *testing.T) {
	opts := &PRFormatterOptions{UseColor: false}

	statusCounts := map[string]int{
		"approved":           5,
		"changes_requested":  2,
		"review_required":    3,
	}

	result := formatSummary(statusCounts, 10, opts)

	if !strings.Contains(result, "Total: 10 PRs") {
		t.Errorf("Expected summary to contain total count, got: %s", result)
	}

	if !strings.Contains(result, "5 approved") {
		t.Errorf("Expected summary to contain approved count, got: %s", result)
	}

	if !strings.Contains(result, "2 changes requested") {
		t.Errorf("Expected summary to contain changes requested count, got: %s", result)
	}

	if !strings.Contains(result, "3 review required") {
		t.Errorf("Expected summary to contain review required count, got: %s", result)
	}
}

func TestSortPullRequests(t *testing.T) {
	now := time.Now()

	prs := []*github.PullRequest{
		{Number: 3, UpdatedAt: now.Add(-1 * time.Hour), CreatedAt: now.Add(-2 * time.Hour)},
		{Number: 1, UpdatedAt: now.Add(-3 * time.Hour), CreatedAt: now.Add(-4 * time.Hour)},
		{Number: 2, UpdatedAt: now.Add(-2 * time.Hour), CreatedAt: now.Add(-3 * time.Hour)},
	}

	t.Run("sort by updated desc", func(t *testing.T) {
		opts := &PRFormatterOptions{SortBy: "updated", SortDesc: true}
		sorted := make([]*github.PullRequest, len(prs))
		copy(sorted, prs)
		sortPullRequests(sorted, opts)

		// Most recent first
		if sorted[0].Number != 3 {
			t.Errorf("Expected PR #3 first, got #%d", sorted[0].Number)
		}
		if sorted[2].Number != 1 {
			t.Errorf("Expected PR #1 last, got #%d", sorted[2].Number)
		}
	})

	t.Run("sort by number asc", func(t *testing.T) {
		opts := &PRFormatterOptions{SortBy: "number", SortDesc: false}
		sorted := make([]*github.PullRequest, len(prs))
		copy(sorted, prs)
		sortPullRequests(sorted, opts)

		if sorted[0].Number != 1 {
			t.Errorf("Expected PR #1 first, got #%d", sorted[0].Number)
		}
		if sorted[2].Number != 3 {
			t.Errorf("Expected PR #3 last, got #%d", sorted[2].Number)
		}
	})
}

func TestFormatPRTable(t *testing.T) {
	now := time.Now()

	prs := []*github.PullRequest{
		{
			Number:    123,
			Title:     "Add new feature",
			UpdatedAt: now.Add(-2 * time.Hour),
			User:      github.PRUser{Login: "alice"},
			Head:      github.PRBranch{Ref: "feature"},
			Base:      github.PRBranch{Ref: "main"},
			Draft:     false,
			Reviews: []github.PRReview{
				{State: "APPROVED"},
			},
			Checks: []github.PRCheck{
				{Status: "completed", Conclusion: "success"},
			},
		},
		{
			Number:    124,
			Title:     "Fix bug",
			UpdatedAt: now.Add(-1 * time.Hour),
			User:      github.PRUser{Login: "bob"},
			Head:      github.PRBranch{Ref: "bugfix"},
			Base:      github.PRBranch{Ref: "main"},
			Draft:     true,
			Reviews:   []github.PRReview{},
			Checks:    []github.PRCheck{},
		},
	}

	opts := &PRFormatterOptions{
		UseColor:      false,
		MaxTitleWidth: 60,
		SortBy:        "updated",
		SortDesc:      true,
		ShowSummary:   true,
	}

	result := FormatPRTable(prs, opts)

	// Check that output contains PR numbers
	if !strings.Contains(result, "#123") {
		t.Error("Expected output to contain #123")
	}
	if !strings.Contains(result, "#124") {
		t.Error("Expected output to contain #124")
	}

	// Check that output contains titles
	if !strings.Contains(result, "Add new feature") {
		t.Error("Expected output to contain 'Add new feature'")
	}
	if !strings.Contains(result, "Fix bug") {
		t.Error("Expected output to contain 'Fix bug'")
	}

	// Check that output contains authors
	if !strings.Contains(result, "alice") {
		t.Error("Expected output to contain 'alice'")
	}
	if !strings.Contains(result, "bob") {
		t.Error("Expected output to contain 'bob'")
	}

	// Check summary
	if !strings.Contains(result, "Total: 2 PRs") {
		t.Error("Expected output to contain summary")
	}
}

func TestVisualLength(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"plain text", "Hello World", 11},
		{"text with ANSI codes", "\033[32mHello\033[0m", 5},
		{"text with multiple codes", "\033[1m\033[32mBold Green\033[0m\033[0m", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := visualLength(tt.input)
			if result != tt.expected {
				t.Errorf("Expected length %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"no truncation needed", "short", 10, "short"},
		{"truncate at boundary", "this is a long string", 10, "this is a "}, // Keep trailing space from word boundary
		{"truncate in word", "hello", 3, "hel"},
		{"exact length", "exact", 5, "exact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestColorize(t *testing.T) {
	text := "Hello"
	result := colorize(text, ColorGreen)

	if !strings.HasPrefix(result, ColorGreen) {
		t.Error("Expected result to start with color code")
	}

	if !strings.HasSuffix(result, ColorReset) {
		t.Error("Expected result to end with reset code")
	}

	if !strings.Contains(result, text) {
		t.Error("Expected result to contain original text")
	}
}
