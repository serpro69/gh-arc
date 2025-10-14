package diff

import (
	"strings"
	"testing"
	"time"

	"github.com/serpro69/gh-arc/internal/git"
)

// Mock repository for testing
type mockGitRepo struct {
	commits []git.CommitInfo
	err     error
}

func (m *mockGitRepo) GetCommitRange(baseBranch, headBranch string) ([]git.CommitInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.commits, nil
}

func (m *mockGitRepo) Path() string { return "/test/repo" }
func (m *mockGitRepo) GetDefaultBranch() (string, error) { return "main", nil }
func (m *mockGitRepo) ListBranches(includeRemote bool) ([]git.BranchInfo, error) { return nil, nil }
func (m *mockGitRepo) GetMergeBase(ref1, ref2 string) (string, error) { return "abc123", nil }

func TestAnalyzeCommitsForTemplate_SingleCommit(t *testing.T) {
	mockRepo := &mockGitRepo{
		commits: []git.CommitInfo{
			{
				SHA:     "abc123",
				Author:  "Test User",
				Email:   "test@example.com",
				Date:    time.Now(),
				Message: "Fix authentication bug\n\nThis fixes the issue where users couldn't log in.",
			},
		},
	}

	analysis, err := AnalyzeCommitsForTemplate(mockRepo, "main", "feature/auth-fix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis.Title != "Fix authentication bug" {
		t.Errorf("expected title 'Fix authentication bug', got '%s'", analysis.Title)
	}

	if !contains(analysis.Summary, "couldn't log in") {
		t.Errorf("expected summary to contain commit body, got '%s'", analysis.Summary)
	}

	if analysis.CommitCount != 1 {
		t.Errorf("expected commit count 1, got %d", analysis.CommitCount)
	}

	if analysis.BaseBranch != "main" {
		t.Errorf("expected base branch 'main', got '%s'", analysis.BaseBranch)
	}
}

func TestAnalyzeCommitsForTemplate_MultipleCommits(t *testing.T) {
	mockRepo := &mockGitRepo{
		commits: []git.CommitInfo{
			{
				SHA:     "def456",
				Author:  "Test User",
				Email:   "test@example.com",
				Date:    time.Now().Add(-1 * time.Hour),
				Message: "Add tests for authentication",
			},
			{
				SHA:     "abc123",
				Author:  "Test User",
				Email:   "test@example.com",
				Date:    time.Now().Add(-2 * time.Hour),
				Message: "Fix authentication bug\n\nThis fixes the issue.",
			},
		},
	}

	analysis, err := AnalyzeCommitsForTemplate(mockRepo, "main", "feature/auth-fix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Title should be from first commit (chronologically)
	if analysis.Title != "Fix authentication bug" {
		t.Errorf("expected title 'Fix authentication bug', got '%s'", analysis.Title)
	}

	// Summary should list all commits
	if !contains(analysis.Summary, "Fix authentication bug") {
		t.Errorf("expected summary to contain first commit title")
	}

	if !contains(analysis.Summary, "Add tests for authentication") {
		t.Errorf("expected summary to contain second commit title")
	}

	if analysis.CommitCount != 2 {
		t.Errorf("expected commit count 2, got %d", analysis.CommitCount)
	}
}

func TestAnalyzeCommitsForTemplate_NoCommits(t *testing.T) {
	mockRepo := &mockGitRepo{
		commits: []git.CommitInfo{},
	}

	analysis, err := AnalyzeCommitsForTemplate(mockRepo, "main", "feature/new-feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should generate title from branch name
	if analysis.Title != "New Feature" {
		t.Errorf("expected title 'New Feature' from branch name, got '%s'", analysis.Title)
	}

	if !contains(analysis.Summary, "No commits") {
		t.Errorf("expected summary to indicate no commits, got '%s'", analysis.Summary)
	}

	if analysis.CommitCount != 0 {
		t.Errorf("expected commit count 0, got %d", analysis.CommitCount)
	}
}

func TestAnalyzeCommitsForTemplate_MergeCommits(t *testing.T) {
	mockRepo := &mockGitRepo{
		commits: []git.CommitInfo{
			{
				SHA:     "ghi789",
				Author:  "Test User",
				Email:   "test@example.com",
				Date:    time.Now(),
				Message: "Merge branch 'feature/other' into feature/main",
			},
			{
				SHA:     "def456",
				Author:  "Test User",
				Email:   "test@example.com",
				Date:    time.Now().Add(-1 * time.Hour),
				Message: "Add feature implementation",
			},
			{
				SHA:     "abc123",
				Author:  "Test User",
				Email:   "test@example.com",
				Date:    time.Now().Add(-2 * time.Hour),
				Message: "Initial commit",
			},
		},
	}

	analysis, err := AnalyzeCommitsForTemplate(mockRepo, "main", "feature/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !analysis.HasMergeCommits {
		t.Error("expected HasMergeCommits to be true")
	}

	if analysis.CommitCount != 3 {
		t.Errorf("expected commit count 3, got %d", analysis.CommitCount)
	}
}

func TestGenerateTemplateContent(t *testing.T) {
	analysis := &CommitAnalysis{
		Title:       "Fix authentication bug",
		Summary:     "This fixes the issue where users couldn't log in.",
		BaseBranch:  "main",
		CommitCount: 1,
	}

	fields := GenerateTemplateContent(analysis)

	if fields.Title != analysis.Title {
		t.Errorf("expected title '%s', got '%s'", analysis.Title, fields.Title)
	}

	if fields.Summary != analysis.Summary {
		t.Errorf("expected summary '%s', got '%s'", analysis.Summary, fields.Summary)
	}

	if fields.BaseBranch != "main" {
		t.Errorf("expected base branch 'main', got '%s'", fields.BaseBranch)
	}

	if fields.TestPlan != "" {
		t.Errorf("expected empty test plan, got '%s'", fields.TestPlan)
	}

	if fields.Reviewers != "" {
		t.Errorf("expected empty reviewers, got '%s'", fields.Reviewers)
	}

	if fields.Ref != "" {
		t.Errorf("expected empty ref, got '%s'", fields.Ref)
	}
}

func TestGenerateTitleFromBranch(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		expected string
	}{
		{
			name:     "feature branch",
			branch:   "feature/add-authentication",
			expected: "Add Authentication",
		},
		{
			name:     "fix branch",
			branch:   "fix/login-bug",
			expected: "Login Bug",
		},
		{
			name:     "bugfix branch",
			branch:   "bugfix/crash-on-startup",
			expected: "Crash On Startup",
		},
		{
			name:     "hotfix branch",
			branch:   "hotfix/critical-issue",
			expected: "Critical Issue",
		},
		{
			name:     "chore branch",
			branch:   "chore/update-dependencies",
			expected: "Update Dependencies",
		},
		{
			name:     "refactor branch",
			branch:   "refactor/cleanup-code",
			expected: "Cleanup Code",
		},
		{
			name:     "branch with underscores",
			branch:   "feature/add_new_feature",
			expected: "Add New Feature",
		},
		{
			name:     "simple branch name",
			branch:   "my-branch",
			expected: "My Branch",
		},
		{
			name:     "empty branch name",
			branch:   "",
			expected: "Update code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateTitleFromBranch(tt.branch)
			if got != tt.expected {
				t.Errorf("generateTitleFromBranch(%s) = %s, want %s", tt.branch, got, tt.expected)
			}
		})
	}
}

func TestFilterMergeCommits(t *testing.T) {
	messages := []string{
		"Add feature",
		"Merge branch 'feature/auth' into main",
		"Fix bug",
		"Merge pull request #123 from user/branch",
		"Update documentation",
	}

	filtered := FilterMergeCommits(messages)

	if len(filtered) != 3 {
		t.Errorf("expected 3 non-merge commits, got %d", len(filtered))
	}

	for _, msg := range filtered {
		if strings.HasPrefix(strings.TrimSpace(msg), "Merge") {
			t.Errorf("filtered list should not contain merge commit: %s", msg)
		}
	}
}

func TestIsEmptyCommitMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected bool
	}{
		{
			name:     "empty string",
			message:  "",
			expected: true,
		},
		{
			name:     "only whitespace",
			message:  "   ",
			expected: true,
		},
		{
			name:     "only newline",
			message:  "\n",
			expected: true,
		},
		{
			name:     "non-empty message",
			message:  "Fix bug",
			expected: false,
		},
		{
			name:     "message with whitespace",
			message:  "  Fix bug  ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsEmptyCommitMessage(tt.message)
			if got != tt.expected {
				t.Errorf("IsEmptyCommitMessage(%q) = %v, want %v", tt.message, got, tt.expected)
			}
		})
	}
}

func TestGenerateFromSingleCommit_EmptyMessage(t *testing.T) {
	title, summary := generateFromSingleCommit("")

	if title != "Update code" {
		t.Errorf("expected default title 'Update code', got '%s'", title)
	}

	if summary != "" {
		t.Errorf("expected empty summary for empty message, got '%s'", summary)
	}
}

func TestGenerateFromSingleCommit_OnlyTitle(t *testing.T) {
	title, summary := generateFromSingleCommit("Fix authentication bug")

	if title != "Fix authentication bug" {
		t.Errorf("expected title 'Fix authentication bug', got '%s'", title)
	}

	if summary != "Fix authentication bug" {
		t.Errorf("expected summary to match title when no body present, got '%s'", summary)
	}
}

func TestGenerateFromSingleCommit_WithBody(t *testing.T) {
	message := "Fix authentication bug\n\nThis fixes the issue where users couldn't log in.\nAlso updates tests."
	title, summary := generateFromSingleCommit(message)

	if title != "Fix authentication bug" {
		t.Errorf("expected title 'Fix authentication bug', got '%s'", title)
	}

	if !contains(summary, "couldn't log in") {
		t.Errorf("expected summary to contain body text")
	}

	if contains(summary, "Fix authentication bug") {
		t.Errorf("summary should not duplicate the title")
	}
}

func TestGenerateFromMultipleCommits_Empty(t *testing.T) {
	commits := []git.CommitInfo{}
	title, summary := generateFromMultipleCommits(commits)

	if title != "Update code" {
		t.Errorf("expected default title 'Update code', got '%s'", title)
	}

	if summary != "No commits" {
		t.Errorf("expected 'No commits' summary, got '%s'", summary)
	}
}

func TestGenerateFromMultipleCommits_ChronologicalOrder(t *testing.T) {
	// Commits are in reverse chronological order (most recent first)
	commits := []git.CommitInfo{
		{
			SHA:     "ccc",
			Message: "Third commit",
			Date:    time.Now(),
		},
		{
			SHA:     "bbb",
			Message: "Second commit",
			Date:    time.Now().Add(-1 * time.Hour),
		},
		{
			SHA:     "aaa",
			Message: "First commit",
			Date:    time.Now().Add(-2 * time.Hour),
		},
	}

	title, summary := generateFromMultipleCommits(commits)

	// Title should be from first commit chronologically (last in array)
	if title != "First commit" {
		t.Errorf("expected title 'First commit', got '%s'", title)
	}

	// Summary should list commits in chronological order
	lines := strings.Split(summary, "\n")
	firstCommitIdx := -1
	secondCommitIdx := -1
	thirdCommitIdx := -1

	for i, line := range lines {
		if contains(line, "First commit") {
			firstCommitIdx = i
		}
		if contains(line, "Second commit") {
			secondCommitIdx = i
		}
		if contains(line, "Third commit") {
			thirdCommitIdx = i
		}
	}

	if firstCommitIdx == -1 || secondCommitIdx == -1 || thirdCommitIdx == -1 {
		t.Error("expected all commits to be in summary")
	}

	if !(firstCommitIdx < secondCommitIdx && secondCommitIdx < thirdCommitIdx) {
		t.Error("commits should be in chronological order in summary")
	}
}
