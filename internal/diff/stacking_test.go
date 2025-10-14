package diff

import (
	"strings"
	"testing"
	"time"

	"github.com/serpro69/gh-arc/internal/github"
)

func TestFormatDependentPRWarning(t *testing.T) {
	tests := []struct {
		name          string
		dependentPRs  []*github.PullRequest
		expectedParts []string // Parts that should be in the output
	}{
		{
			name:          "no dependent PRs",
			dependentPRs:  []*github.PullRequest{},
			expectedParts: nil, // Empty string expected
		},
		{
			name: "single dependent PR",
			dependentPRs: []*github.PullRequest{
				{Number: 123, Title: "Feature A"},
			},
			expectedParts: []string{"Warning", "1 dependent PR", "#123"},
		},
		{
			name: "multiple dependent PRs",
			dependentPRs: []*github.PullRequest{
				{Number: 123, Title: "Feature A"},
				{Number: 124, Title: "Feature B"},
			},
			expectedParts: []string{"Warning", "2 dependent PRs", "#123", "#124"},
		},
		{
			name: "many dependent PRs",
			dependentPRs: []*github.PullRequest{
				{Number: 101, Title: "Feature 1"},
				{Number: 102, Title: "Feature 2"},
				{Number: 103, Title: "Feature 3"},
			},
			expectedParts: []string{"Warning", "3 dependent PRs", "#101", "#102", "#103"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDependentPRWarning(tt.dependentPRs)

			if len(tt.expectedParts) == 0 {
				// Expecting empty string
				if result != "" {
					t.Errorf("FormatDependentPRWarning() = %q, want empty string", result)
				}
				return
			}

			// Check all expected parts are present
			for _, part := range tt.expectedParts {
				if !strings.Contains(result, part) {
					t.Errorf("FormatDependentPRWarning() = %q, should contain %q", result, part)
				}
			}
		})
	}
}

func TestShowStackingStatus(t *testing.T) {
	tests := []struct {
		name          string
		parentBranch  string
		parentPR      *github.PullRequest
		expectedParts []string
	}{
		{
			name:          "no parent PR",
			parentBranch:  "main",
			parentPR:      nil,
			expectedParts: []string{"Creating PR", "base:", "main"},
		},
		{
			name:         "with parent PR",
			parentBranch: "feature/auth",
			parentPR: &github.PullRequest{
				Number: 122,
				Title:  "Add authentication system",
			},
			expectedParts: []string{"Stacking", "feature/auth", "#122", "Add authentication system"},
		},
		{
			name:         "parent PR with long title",
			parentBranch: "feature/complex",
			parentPR: &github.PullRequest{
				Number: 999,
				Title:  "Implement comprehensive authentication and authorization system",
			},
			expectedParts: []string{"Stacking", "feature/complex", "#999"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShowStackingStatus(tt.parentBranch, tt.parentPR)

			for _, part := range tt.expectedParts {
				if !strings.Contains(result, part) {
					t.Errorf("ShowStackingStatus() = %q, should contain %q", result, part)
				}
			}
		})
	}
}

func TestIsParentBranch(t *testing.T) {
	tests := []struct {
		name         string
		dependentPRs []*github.PullRequest
		expected     bool
	}{
		{
			name:         "no dependent PRs",
			dependentPRs: []*github.PullRequest{},
			expected:     false,
		},
		{
			name:         "nil dependent PRs",
			dependentPRs: nil,
			expected:     false,
		},
		{
			name: "one dependent PR",
			dependentPRs: []*github.PullRequest{
				{Number: 123},
			},
			expected: true,
		},
		{
			name: "multiple dependent PRs",
			dependentPRs: []*github.PullRequest{
				{Number: 123},
				{Number: 124},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsParentBranch(tt.dependentPRs)
			if result != tt.expected {
				t.Errorf("IsParentBranch() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatDependentPRList(t *testing.T) {
	tests := []struct {
		name          string
		dependentPRs  []*github.PullRequest
		expectedParts []string
		shouldBeEmpty bool
	}{
		{
			name:          "no dependent PRs",
			dependentPRs:  []*github.PullRequest{},
			shouldBeEmpty: true,
		},
		{
			name: "single PR with details",
			dependentPRs: []*github.PullRequest{
				{
					Number: 123,
					Title:  "Add login feature",
					User: github.PRUser{
						Login: "johndoe",
					},
				},
			},
			expectedParts: []string{"#123", "Add login feature", "@johndoe", "Dependent PRs"},
		},
		{
			name: "multiple PRs with different authors",
			dependentPRs: []*github.PullRequest{
				{
					Number: 123,
					Title:  "Feature A",
					User: github.PRUser{
						Login: "alice",
					},
				},
				{
					Number: 124,
					Title:  "Feature B",
					User: github.PRUser{
						Login: "bob",
					},
				},
			},
			expectedParts: []string{"#123", "Feature A", "@alice", "#124", "Feature B", "@bob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDependentPRList(tt.dependentPRs)

			if tt.shouldBeEmpty {
				if result != "" {
					t.Errorf("FormatDependentPRList() = %q, want empty string", result)
				}
				return
			}

			for _, part := range tt.expectedParts {
				if !strings.Contains(result, part) {
					t.Errorf("FormatDependentPRList() = %q, should contain %q", result, part)
				}
			}
		})
	}
}

func TestFormatDependentPRWarning_EdgeCases(t *testing.T) {
	t.Run("PR numbers in order", func(t *testing.T) {
		prs := []*github.PullRequest{
			{Number: 100},
			{Number: 200},
			{Number: 300},
		}

		result := FormatDependentPRWarning(prs)

		// Check that all PR numbers appear in the result
		if !strings.Contains(result, "#100") || !strings.Contains(result, "#200") || !strings.Contains(result, "#300") {
			t.Errorf("FormatDependentPRWarning() should contain all PR numbers, got: %s", result)
		}
	})
}

func TestShowStackingStatus_EmptyParentBranch(t *testing.T) {
	result := ShowStackingStatus("", nil)

	// Should handle empty parent branch gracefully
	if result == "" {
		t.Error("ShowStackingStatus() should not return empty string for empty parent branch")
	}
}

// Test PR creation helper
func createTestPR(number int, title, author string) *github.PullRequest {
	return &github.PullRequest{
		Number: number,
		Title:  title,
		User: github.PRUser{
			Login: author,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
