package diff

import (
	"context"
	"testing"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/github"
)

func TestDetectDependentPRs_NoDependents(t *testing.T) {
	mockClient := &mockGitHubClient{
		pullRequests: []*github.PullRequest{
			{
				Number: 123,
				Title:  "Some PR",
				Head:   github.PRBranch{Ref: "feature-a"},
				Base:   github.PRBranch{Ref: "main"},
			},
			{
				Number: 124,
				Title:  "Another PR",
				Head:   github.PRBranch{Ref: "feature-b"},
				Base:   github.PRBranch{Ref: "main"},
			},
		},
	}
	cfg := &config.DiffConfig{
		ShowStackingWarnings: true,
	}

	detector := NewDependentPRDetector(mockClient, cfg, "owner", "repo")

	info, err := detector.DetectDependentPRs(context.Background(), "feature-c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.HasDependents {
		t.Error("expected HasDependents to be false")
	}
	if len(info.DependentPRs) != 0 {
		t.Errorf("expected 0 dependent PRs, got %d", len(info.DependentPRs))
	}
}

func TestDetectDependentPRs_WithDependents(t *testing.T) {
	mockClient := &mockGitHubClient{
		pullRequests: []*github.PullRequest{
			{
				Number: 123,
				Title:  "Parent PR",
				Head:   github.PRBranch{Ref: "feature-parent"},
				Base:   github.PRBranch{Ref: "main"},
			},
			{
				Number: 124,
				Title:  "Child PR 1",
				Head:   github.PRBranch{Ref: "feature-child-1"},
				Base:   github.PRBranch{Ref: "feature-parent"}, // Depends on feature-parent
			},
			{
				Number: 125,
				Title:  "Child PR 2",
				Head:   github.PRBranch{Ref: "feature-child-2"},
				Base:   github.PRBranch{Ref: "feature-parent"}, // Depends on feature-parent
			},
			{
				Number: 126,
				Title:  "Other PR",
				Head:   github.PRBranch{Ref: "feature-other"},
				Base:   github.PRBranch{Ref: "main"},
			},
		},
	}
	cfg := &config.DiffConfig{
		ShowStackingWarnings: true,
	}

	detector := NewDependentPRDetector(mockClient, cfg, "owner", "repo")

	info, err := detector.DetectDependentPRs(context.Background(), "feature-parent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !info.HasDependents {
		t.Error("expected HasDependents to be true")
	}
	if len(info.DependentPRs) != 2 {
		t.Errorf("expected 2 dependent PRs, got %d", len(info.DependentPRs))
	}

	// Check that the correct PRs are detected
	foundPR124 := false
	foundPR125 := false
	for _, pr := range info.DependentPRs {
		if pr.Number == 124 {
			foundPR124 = true
		}
		if pr.Number == 125 {
			foundPR125 = true
		}
	}

	if !foundPR124 {
		t.Error("expected to find PR #124")
	}
	if !foundPR125 {
		t.Error("expected to find PR #125")
	}
}

func TestFormatDependentPRsWarning_NoDependents(t *testing.T) {
	info := &DependentPRInfo{
		HasDependents: false,
		DependentPRs:  []*github.PullRequest{},
	}

	msg := info.FormatDependentPRsWarning()
	if msg != "" {
		t.Errorf("expected empty message for no dependents, got: %s", msg)
	}
}

func TestFormatDependentPRsWarning_WithDependents(t *testing.T) {
	info := &DependentPRInfo{
		HasDependents: true,
		DependentPRs: []*github.PullRequest{
			{
				Number: 124,
				Title:  "Child PR 1",
				Head:   github.PRBranch{Ref: "feature-child-1"},
			},
			{
				Number: 125,
				Title:  "Child PR 2",
				Head:   github.PRBranch{Ref: "feature-child-2"},
			},
		},
	}

	msg := info.FormatDependentPRsWarning()

	if msg == "" {
		t.Error("expected non-empty warning message")
	}

	// Check that message contains warning indicator
	if !contains(msg, "⚠") && !contains(msg, "Warning") {
		t.Error("expected message to contain warning indicator")
	}

	// Check that message contains PR numbers
	if !contains(msg, "#124") {
		t.Error("expected message to contain PR #124")
	}
	if !contains(msg, "#125") {
		t.Error("expected message to contain PR #125")
	}

	// Check that message mentions count
	if !contains(msg, "2") {
		t.Error("expected message to mention count of dependent PRs")
	}

	// Check that message contains branch names (pr.Head.Ref)
	if !contains(msg, "feature-child-1") {
		t.Error("expected message to contain branch name 'feature-child-1'")
	}
	if !contains(msg, "feature-child-2") {
		t.Error("expected message to contain branch name 'feature-child-2'")
	}
}

func TestFormatDependentPRsInfo_NoDependents(t *testing.T) {
	info := &DependentPRInfo{
		HasDependents: false,
		DependentPRs:  []*github.PullRequest{},
	}

	msg := info.FormatDependentPRsInfo()
	if msg != "" {
		t.Errorf("expected empty message for no dependents, got: %s", msg)
	}
}

func TestFormatDependentPRsInfo_WithDependents(t *testing.T) {
	info := &DependentPRInfo{
		HasDependents: true,
		DependentPRs: []*github.PullRequest{
			{
				Number: 124,
				Title:  "Child PR 1",
			},
		},
	}

	msg := info.FormatDependentPRsInfo()

	if msg == "" {
		t.Error("expected non-empty info message")
	}

	// Check that message contains PR number
	if !contains(msg, "#124") {
		t.Error("expected message to contain PR #124")
	}
}

func TestShouldShowWarning(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.DiffConfig
		expected bool
	}{
		{
			name: "warnings enabled",
			config: &config.DiffConfig{
				ShowStackingWarnings: true,
			},
			expected: true,
		},
		{
			name: "warnings disabled",
			config: &config.DiffConfig{
				ShowStackingWarnings: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockGitHubClient{}
			detector := NewDependentPRDetector(mockClient, tt.config, "owner", "repo")

			if got := detector.ShouldShowWarning(); got != tt.expected {
				t.Errorf("ShouldShowWarning() = %v, want %v", got, tt.expected)
			}
		})
	}
}
