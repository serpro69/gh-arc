package diff

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/serpro69/gh-arc/internal/github"
	"github.com/serpro69/gh-arc/internal/template"
)

// Test OutputStyle creation
func TestNewOutputStyle(t *testing.T) {
	t.Run("with color enabled", func(t *testing.T) {
		style := NewOutputStyle(true)
		if style == nil {
			t.Fatal("NewOutputStyle returned nil")
		}
		if !style.useColor {
			t.Error("useColor should be true")
		}
	})

	t.Run("with color disabled", func(t *testing.T) {
		style := NewOutputStyle(false)
		if style == nil {
			t.Fatal("NewOutputStyle returned nil")
		}
		if style.useColor {
			t.Error("useColor should be false")
		}
	})
}

// Test basic styling methods
func TestOutputStyleBasicMethods(t *testing.T) {
	style := NewOutputStyle(false) // Use no-color mode for predictable output

	tests := []struct {
		name     string
		method   func(string) string
		input    string
		expected string
	}{
		{
			name:     "Error",
			method:   style.Error,
			input:    "test error",
			expected: "✗ test error",
		},
		{
			name:     "Warning",
			method:   style.Warning,
			input:    "test warning",
			expected: "⚠ test warning",
		},
		{
			name:     "Success",
			method:   style.Success,
			input:    "test success",
			expected: "✓ test success",
		},
		{
			name:     "Info",
			method:   style.Info,
			input:    "test info",
			expected: "ℹ test info",
		},
		{
			name:     "Stack",
			method:   style.Stack,
			input:    "test stack",
			expected: "📚 test stack",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method(tt.input)
			if result != tt.expected {
				t.Errorf("%s() = %q, want %q", tt.name, result, tt.expected)
			}
		})
	}
}

func TestOutputStyleHighlight(t *testing.T) {
	style := NewOutputStyle(false)
	result := style.Highlight("highlighted text")
	if result != "highlighted text" {
		t.Errorf("Highlight() with no color should return unchanged text, got %q", result)
	}
}

func TestOutputStyleDim(t *testing.T) {
	style := NewOutputStyle(false)
	result := style.Dim("dimmed text")
	if result != "dimmed text" {
		t.Errorf("Dim() with no color should return unchanged text, got %q", result)
	}
}

// Test FormatStackingOutput
func TestFormatStackingOutput(t *testing.T) {
	style := NewOutputStyle(false)

	t.Run("with parent PR and dependents", func(t *testing.T) {
		parentPR := &github.PullRequest{
			Number: 100,
			Title:  "Parent feature",
			Base: github.PRBranch{
				Ref: "main",
			},
		}
		dependentPRs := []*github.PullRequest{
			{
				Number: 101,
				Title:  "Child feature 1",
				User: github.PRUser{
					Login: "alice",
				},
			},
			{
				Number: 102,
				Title:  "Child feature 2",
				User: github.PRUser{
					Login: "bob",
				},
			},
		}

		result := FormatStackingOutput("feature-branch", "parent-branch", parentPR, dependentPRs, style)

		expectedParts := []string{
			"Stacking Information",
			"Current: feature-branch",
			"Parent:  parent-branch",
			"(PR #100: Parent feature)",
			"2 dependent PRs target this branch:",
			"PR #101: Child feature 1",
			"@alice",
			"PR #102: Child feature 2",
			"@bob",
		}

		for _, part := range expectedParts {
			if !strings.Contains(result, part) {
				t.Errorf("FormatStackingOutput() missing expected part: %q\nGot: %s", part, result)
			}
		}
	})

	t.Run("targeting trunk without dependents", func(t *testing.T) {
		result := FormatStackingOutput("feature-branch", "main", nil, []*github.PullRequest{}, style)

		expectedParts := []string{
			"Stacking Information",
			"Branch: feature-branch",
			"Base:   main",
			"(trunk)",
		}

		for _, part := range expectedParts {
			if !strings.Contains(result, part) {
				t.Errorf("FormatStackingOutput() missing expected part: %q", part)
			}
		}

		// Should not contain dependent PR warnings
		if strings.Contains(result, "dependent PR") {
			t.Error("FormatStackingOutput() should not mention dependent PRs when there are none")
		}
	})

	t.Run("with single dependent PR", func(t *testing.T) {
		dependentPRs := []*github.PullRequest{
			{Number: 99, Title: "Dependent", User: github.PRUser{Login: "charlie"}},
		}

		result := FormatStackingOutput("feature", "main", nil, dependentPRs, style)

		if !strings.Contains(result, "1 dependent PR targets this branch:") {
			t.Error("FormatStackingOutput() should show singular form for 1 dependent PR")
		}
	})
}

// Test FormatPRCreated
func TestFormatPRCreated(t *testing.T) {
	style := NewOutputStyle(false)

	t.Run("stacked PR", func(t *testing.T) {
		pr := &github.PullRequest{
			Number: 123,
			Title:  "New feature",
			Head: github.PRBranch{
				Ref: "feature-branch",
			},
			Base: github.PRBranch{
				Ref: "parent-branch",
			},
			HTMLURL: "https://github.com/user/repo/pull/123",
			Draft:   false,
		}
		parentPR := &github.PullRequest{
			Number: 100,
			Base: github.PRBranch{
				Ref: "parent-branch",
			},
		}

		result := FormatPRCreated(pr, parentPR, style)

		expectedParts := []string{
			"Created stacked PR #123",
			"Stacking on:",
			"PR #100 (parent-branch)",
			"Title:  New feature",
			"Branch: feature-branch → parent-branch",
			"URL:    https://github.com/user/repo/pull/123",
		}

		for _, part := range expectedParts {
			if !strings.Contains(result, part) {
				t.Errorf("FormatPRCreated() missing expected part: %q", part)
			}
		}
	})

	t.Run("non-stacked PR", func(t *testing.T) {
		pr := &github.PullRequest{
			Number: 124,
			Title:  "Direct to main",
			Head: github.PRBranch{
				Ref: "feature",
			},
			Base: github.PRBranch{
				Ref: "main",
			},
			HTMLURL: "https://github.com/user/repo/pull/124",
			Draft:   false,
		}

		result := FormatPRCreated(pr, nil, style)

		if !strings.Contains(result, "Created PR #124") {
			t.Error("FormatPRCreated() should show non-stacked message")
		}
		if strings.Contains(result, "Stacking on") {
			t.Error("FormatPRCreated() should not mention stacking for non-stacked PR")
		}
	})

	t.Run("draft PR", func(t *testing.T) {
		pr := &github.PullRequest{
			Number: 125,
			Title:  "Draft feature",
			Head: github.PRBranch{
				Ref: "feature",
			},
			Base: github.PRBranch{
				Ref: "main",
			},
			HTMLURL: "https://github.com/user/repo/pull/125",
			Draft:   true,
		}

		result := FormatPRCreated(pr, nil, style)

		if !strings.Contains(result, "PR is in draft state") {
			t.Error("FormatPRCreated() should mention draft state")
		}
	})
}

// Test FormatPRUpdated
func TestFormatPRUpdated(t *testing.T) {
	style := NewOutputStyle(false)

	t.Run("with base change", func(t *testing.T) {
		pr := &github.PullRequest{
			Number:  123,
			Title:   "Updated feature",
			HTMLURL: "https://github.com/user/repo/pull/123",
		}

		result := FormatPRUpdated(pr, true, "old-base", "new-base", style)

		expectedParts := []string{
			"Updated PR #123",
			"Title: Updated feature",
			"URL:   https://github.com/user/repo/pull/123",
			"Updated base branch: old-base → new-base",
		}

		for _, part := range expectedParts {
			if !strings.Contains(result, part) {
				t.Errorf("FormatPRUpdated() missing expected part: %q", part)
			}
		}
	})

	t.Run("without base change", func(t *testing.T) {
		pr := &github.PullRequest{
			Number:  124,
			Title:   "Updated feature",
			HTMLURL: "https://github.com/user/repo/pull/124",
		}

		result := FormatPRUpdated(pr, false, "", "", style)

		if strings.Contains(result, "Updated base branch") {
			t.Error("FormatPRUpdated() should not mention base change when none occurred")
		}
	})
}

// Test FormatStackWarning
func TestFormatStackWarning(t *testing.T) {
	style := NewOutputStyle(false)

	t.Run("with details", func(t *testing.T) {
		details := []string{
			"Detail 1",
			"Detail 2",
			"Detail 3",
		}

		result := FormatStackWarning("Warning message", details, style)

		if !strings.Contains(result, "Warning message") {
			t.Error("FormatStackWarning() should contain warning message")
		}
		for _, detail := range details {
			if !strings.Contains(result, detail) {
				t.Errorf("FormatStackWarning() missing detail: %q", detail)
			}
		}
	})

	t.Run("without details", func(t *testing.T) {
		result := FormatStackWarning("Simple warning", []string{}, style)

		if !strings.Contains(result, "Simple warning") {
			t.Error("FormatStackWarning() should contain warning message")
		}
	})
}

// Test FormatStackConfirmation
func TestFormatStackConfirmation(t *testing.T) {
	style := NewOutputStyle(false)

	t.Run("with parent PR", func(t *testing.T) {
		parentPR := &github.PullRequest{
			Number: 100,
		}

		result := FormatStackConfirmation("create PR", "feature", "parent-branch", parentPR, style)

		expectedParts := []string{
			"About to create PR:",
			"Branch: feature",
			"Base:   parent-branch",
			"(PR #100)",
			"This will create a stacked PR on #100",
		}

		for _, part := range expectedParts {
			if !strings.Contains(result, part) {
				t.Errorf("FormatStackConfirmation() missing expected part: %q", part)
			}
		}
	})

	t.Run("without parent PR", func(t *testing.T) {
		result := FormatStackConfirmation("create PR", "feature", "main", nil, style)

		if !strings.Contains(result, "Base:   main") {
			t.Error("FormatStackConfirmation() should show base branch")
		}
		if strings.Contains(result, "stacked PR") {
			t.Error("FormatStackConfirmation() should not mention stacking without parent PR")
		}
	})
}

// Test FormatDryRunOutput
func TestFormatDryRunOutput(t *testing.T) {
	style := NewOutputStyle(false)

	t.Run("complete scenario", func(t *testing.T) {
		parentPR := &github.PullRequest{
			Number: 100,
			Title:  "Parent PR",
			State:  "open",
			Draft:  false,
		}
		dependentPRs := []*github.PullRequest{
			{Number: 101, Title: "Dependent 1"},
			{Number: 102, Title: "Dependent 2"},
		}
		analysis := &template.CommitAnalysis{
			Title:           "Proposed PR title",
			CommitCount:     5,
			HasMergeCommits: false,
		}

		result := FormatDryRunOutput("feature", "parent", parentPR, dependentPRs, analysis, style)

		expectedParts := []string{
			"Dry run mode - no changes will be made",
			"Detected configuration:",
			"Current branch: feature",
			"Detected base:  parent",
			"Stacking detected:",
			"Parent PR: #100 - Parent PR",
			"State:     open",
			"Dependent PRs (2):",
			"#101: Dependent 1",
			"#102: Dependent 2",
			"Proposed PR content:",
			"Title:        Proposed PR title",
			"Commit count: 5",
		}

		for _, part := range expectedParts {
			if !strings.Contains(result, part) {
				t.Errorf("FormatDryRunOutput() missing expected part: %q", part)
			}
		}
	})

	t.Run("with draft parent PR", func(t *testing.T) {
		parentPR := &github.PullRequest{
			Number: 100,
			Title:  "Draft Parent",
			State:  "open",
			Draft:  true,
		}

		result := FormatDryRunOutput("feature", "parent", parentPR, []*github.PullRequest{}, nil, style)

		if !strings.Contains(result, "Warning: Parent PR is in draft state") {
			t.Error("FormatDryRunOutput() should warn about draft parent PR")
		}
	})

	t.Run("with merge commits", func(t *testing.T) {
		analysis := &template.CommitAnalysis{
			Title:           "PR with merges",
			CommitCount:     3,
			HasMergeCommits: true,
		}

		result := FormatDryRunOutput("feature", "main", nil, []*github.PullRequest{}, analysis, style)

		if !strings.Contains(result, "Has merge commits") {
			t.Error("FormatDryRunOutput() should indicate merge commits")
		}
	})
}

// Test FormatProgressIndicator
func TestFormatProgressIndicator(t *testing.T) {
	style := NewOutputStyle(false)

	result := FormatProgressIndicator("analyzing stack", style)

	if !strings.Contains(result, "⏳") {
		t.Error("FormatProgressIndicator() should contain hourglass emoji")
	}
	if !strings.Contains(result, "analyzing stack") {
		t.Error("FormatProgressIndicator() should contain operation text")
	}
	if !strings.Contains(result, "...") {
		t.Error("FormatProgressIndicator() should contain ellipsis")
	}
}

// Test FormatErrorWithContext
func TestFormatErrorWithContext(t *testing.T) {
	style := NewOutputStyle(false)

	t.Run("CircularDependencyError", func(t *testing.T) {
		err := &github.CircularDependencyError{
			Message:       "Circular dependency in stack",
			Branches:      []string{"branch-a", "branch-b", "branch-c", "branch-a"},
			CurrentPR:     100,
			ConflictingPR: 101,
		}

		result := FormatErrorWithContext(err, style)

		expectedParts := []string{
			"Circular dependency detected",
			"Dependency chain:",
			"branch-a → branch-b → branch-c → branch-a",
			"Current PR: #100",
			"Conflicts with: #101",
			"Tip: Check your branch structure",
		}

		for _, part := range expectedParts {
			if !strings.Contains(result, part) {
				t.Errorf("FormatErrorWithContext(CircularDependencyError) missing: %q", part)
			}
		}
	})

	t.Run("InvalidBaseError", func(t *testing.T) {
		err := &github.InvalidBaseError{
			Message:    "Base branch does not exist",
			BaseBranch: "non-existent",
			ValidBases: []string{"main", "develop", "feature-parent"},
		}

		result := FormatErrorWithContext(err, style)

		expectedParts := []string{
			"Invalid base branch",
			"Branch: non-existent",
			"Valid bases:",
			"main",
			"develop",
			"feature-parent",
			"Tip: Use --base flag",
		}

		for _, part := range expectedParts {
			if !strings.Contains(result, part) {
				t.Errorf("FormatErrorWithContext(InvalidBaseError) missing: %q", part)
			}
		}
	})

	t.Run("ParentPRConflictError", func(t *testing.T) {
		err := &github.ParentPRConflictError{
			Message:     "Parent PR is closed",
			ParentPR:    100,
			ParentState: "closed",
			Reason:      "PR was rejected",
		}

		result := FormatErrorWithContext(err, style)

		expectedParts := []string{
			"Parent PR conflict",
			"Parent PR: #100",
			"State:     closed",
			"Reason:    PR was rejected",
			"Tip: Rebase onto trunk",
		}

		for _, part := range expectedParts {
			if !strings.Contains(result, part) {
				t.Errorf("FormatErrorWithContext(ParentPRConflictError) missing: %q", part)
			}
		}
	})

	t.Run("StackingError with context", func(t *testing.T) {
		err := &github.StackingError{
			Message:       "Failed to create stacked PR",
			CurrentBranch: "feature-a",
			BaseBranch:    "feature-b",
			Operation:     "create",
			Context: map[string]interface{}{
				"attempt": 2,
				"detail":  "timeout",
			},
		}

		result := FormatErrorWithContext(err, style)

		expectedParts := []string{
			"Stacking error",
			"Operation: create",
			"Branch:    feature-a → feature-b",
			"Context:",
		}

		for _, part := range expectedParts {
			if !strings.Contains(result, part) {
				t.Errorf("FormatErrorWithContext(StackingError) missing: %q", part)
			}
		}
	})

	t.Run("generic error", func(t *testing.T) {
		err := errors.New("generic error message")

		result := FormatErrorWithContext(err, style)

		if !strings.Contains(result, "generic error message") {
			t.Error("FormatErrorWithContext() should handle generic errors")
		}
	})
}

// Test edge cases
func TestFormatStackingOutputEdgeCases(t *testing.T) {
	style := NewOutputStyle(false)

	t.Run("empty branch names", func(t *testing.T) {
		result := FormatStackingOutput("", "", nil, []*github.PullRequest{}, style)
		if result == "" {
			t.Error("FormatStackingOutput() should handle empty branches gracefully")
		}
	})

	t.Run("nil dependent PRs", func(t *testing.T) {
		result := FormatStackingOutput("feature", "main", nil, nil, style)
		if result == "" {
			t.Error("FormatStackingOutput() should handle nil dependents")
		}
	})
}

// Test with color enabled
func TestOutputStyleWithColor(t *testing.T) {
	style := NewOutputStyle(true)

	t.Run("color enabled style creates successfully", func(t *testing.T) {
		if style == nil {
			t.Fatal("NewOutputStyle(true) should create a valid style")
		}
		if !style.useColor {
			t.Error("style.useColor should be true when color is enabled")
		}

		// Just verify the methods work without errors
		_ = style.Error("test")
		_ = style.Warning("test")
		_ = style.Success("test")
		_ = style.Info("test")
		_ = style.Highlight("test")
		_ = style.Dim("test")
		_ = style.Stack("test")
	})
}

// Benchmark tests
func BenchmarkFormatStackingOutput(b *testing.B) {
	style := NewOutputStyle(false)
	parentPR := &github.PullRequest{
		Number: 100,
		Title:  "Parent",
		Base:   github.PRBranch{Ref: "main"},
	}
	dependentPRs := []*github.PullRequest{
		{Number: 101, Title: "Child 1", User: github.PRUser{Login: "alice"}},
		{Number: 102, Title: "Child 2", User: github.PRUser{Login: "bob"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatStackingOutput("feature", "parent", parentPR, dependentPRs, style)
	}
}

func BenchmarkFormatErrorWithContext(b *testing.B) {
	style := NewOutputStyle(false)
	err := &github.CircularDependencyError{
		Message:  "Circular dependency",
		Branches: []string{"a", "b", "c", "a"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatErrorWithContext(err, style)
	}
}

// Helper function to create test PR
func createTestPRForOutput(number int, title, headRef, baseRef string, draft bool) *github.PullRequest {
	return &github.PullRequest{
		Number: number,
		Title:  title,
		Head: github.PRBranch{
			Ref: headRef,
		},
		Base: github.PRBranch{
			Ref: baseRef,
		},
		HTMLURL:   "https://github.com/test/repo/pull/" + string(rune(number)),
		Draft:     draft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
