package diff

import (
	"context"
	"testing"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/github"
)

// mockRepository implements a minimal mock for git.Repository
type mockRepository struct {
	defaultBranch  string
	branches       []git.BranchInfo
	mergeBaseFunc  func(ref1, ref2 string) (string, error)
	commitRange    func(from, to string) ([]git.CommitInfo, error)
	isAncestorFunc func(ancestorRef, descendantRef string) (bool, error)
}

func (m *mockRepository) Path() string {
	return "/mock/repo"
}

func (m *mockRepository) GetDefaultBranch() (string, error) {
	return m.defaultBranch, nil
}

func (m *mockRepository) ListBranches(includeRemote bool) ([]git.BranchInfo, error) {
	return m.branches, nil
}

func (m *mockRepository) GetMergeBase(ref1, ref2 string) (string, error) {
	if m.mergeBaseFunc != nil {
		return m.mergeBaseFunc(ref1, ref2)
	}
	return "abc123", nil
}

func (m *mockRepository) GetCommitRange(from, to string) ([]git.CommitInfo, error) {
	if m.commitRange != nil {
		return m.commitRange(from, to)
	}
	return []git.CommitInfo{{SHA: "commit1"}}, nil
}

func (m *mockRepository) GetCommitsBetween(base, head string) ([]git.CommitInfo, error) {
	if m.commitRange != nil {
		return m.commitRange(base, head)
	}
	return []git.CommitInfo{{SHA: "commit1"}}, nil
}

func (m *mockRepository) IsAncestor(ancestorRef, descendantRef string) (bool, error) {
	if m.isAncestorFunc != nil {
		return m.isAncestorFunc(ancestorRef, descendantRef)
	}
	// Default: assume ancestor relationship exists if commitRange returns commits
	if m.commitRange != nil {
		commits, err := m.commitRange(ancestorRef, descendantRef)
		if err != nil {
			return false, err
		}
		return len(commits) > 0, nil
	}
	return true, nil
}

// mockGitHubClient implements a minimal mock for github.Client
type mockGitHubClient struct {
	pullRequests []*github.PullRequest
}

func (m *mockGitHubClient) GetPullRequests(ctx context.Context, owner, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, error) {
	return m.pullRequests, nil
}

func TestDetectBaseBranch_ExplicitFlag(t *testing.T) {
	mockRepo := &mockRepository{
		defaultBranch: "main",
	}
	mockClient := &mockGitHubClient{}
	cfg := &config.DiffConfig{
		EnableStacking: true,
	}

	detector := NewBaseBranchDetector(mockRepo, mockClient, cfg, "owner", "repo")

	result, err := detector.DetectBaseBranch(context.Background(), "feature-branch", "custom-base")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Base != "custom-base" {
		t.Errorf("expected base 'custom-base', got '%s'", result.Base)
	}
	if result.IsStacking {
		t.Error("expected IsStacking to be false with explicit flag")
	}
	if result.Method != "explicit-flag" {
		t.Errorf("expected method 'explicit-flag', got '%s'", result.Method)
	}
}

func TestDetectBaseBranch_ConfigDefault(t *testing.T) {
	mockRepo := &mockRepository{
		defaultBranch: "main",
	}
	mockClient := &mockGitHubClient{}
	cfg := &config.DiffConfig{
		EnableStacking: true,
		DefaultBase:    "develop",
	}

	detector := NewBaseBranchDetector(mockRepo, mockClient, cfg, "owner", "repo")

	result, err := detector.DetectBaseBranch(context.Background(), "feature-branch", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Base != "develop" {
		t.Errorf("expected base 'develop', got '%s'", result.Base)
	}
	if result.IsStacking {
		t.Error("expected IsStacking to be false with config default")
	}
	if result.Method != "config-default" {
		t.Errorf("expected method 'config-default', got '%s'", result.Method)
	}
}

func TestDetectBaseBranch_StackingDisabled(t *testing.T) {
	mockRepo := &mockRepository{
		defaultBranch: "main",
	}
	mockClient := &mockGitHubClient{}
	cfg := &config.DiffConfig{
		EnableStacking: false,
	}

	detector := NewBaseBranchDetector(mockRepo, mockClient, cfg, "owner", "repo")

	result, err := detector.DetectBaseBranch(context.Background(), "feature-branch", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Base != "main" {
		t.Errorf("expected base 'main', got '%s'", result.Base)
	}
	if result.IsStacking {
		t.Error("expected IsStacking to be false when stacking disabled")
	}
	if result.Method != "default-branch-no-stacking" {
		t.Errorf("expected method 'default-branch-no-stacking', got '%s'", result.Method)
	}
}

func TestDetectBaseBranch_NoStackingOpportunity(t *testing.T) {
	mockRepo := &mockRepository{
		defaultBranch: "main",
		branches: []git.BranchInfo{
			{Name: "main", Hash: "abc123"},
			{Name: "feature-branch", Hash: "def456"},
			{Name: "other-branch", Hash: "ghi789"}, // No PR for this branch
		},
	}
	mockClient := &mockGitHubClient{
		pullRequests: []*github.PullRequest{},
	}
	cfg := &config.DiffConfig{
		EnableStacking: true,
	}

	detector := NewBaseBranchDetector(mockRepo, mockClient, cfg, "owner", "repo")

	result, err := detector.DetectBaseBranch(context.Background(), "feature-branch", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Base != "main" {
		t.Errorf("expected base 'main', got '%s'", result.Base)
	}
	if result.IsStacking {
		t.Error("expected IsStacking to be false with no stacking opportunity")
	}
	if result.Method != "default-branch" {
		t.Errorf("expected method 'default-branch', got '%s'", result.Method)
	}
}

func TestDetectBaseBranch_StackingDetected(t *testing.T) {
	mockRepo := &mockRepository{
		defaultBranch: "main",
		branches: []git.BranchInfo{
			{Name: "main", Hash: "abc123"},
			{Name: "feature-parent", Hash: "def456"},
			{Name: "feature-child", Hash: "ghi789"},
		},
		mergeBaseFunc: func(ref1, ref2 string) (string, error) {
			// Simulate that feature-child branched from feature-parent
			if (ref1 == "feature-child" && ref2 == "feature-parent") ||
				(ref1 == "feature-parent" && ref2 == "feature-child") {
				return "def456", nil // merge-base is feature-parent's head
			}
			if (ref1 == "feature-child" && ref2 == "main") ||
				(ref1 == "main" && ref2 == "feature-child") {
				return "abc123", nil // merge-base with main is different
			}
			return "abc123", nil
		},
		commitRange: func(from, to string) ([]git.CommitInfo, error) {
			// Simulate commits exist between merge-base and feature-child
			return []git.CommitInfo{{SHA: "commit1"}}, nil
		},
	}
	mockClient := &mockGitHubClient{
		pullRequests: []*github.PullRequest{
			{
				Number: 123,
				Title:  "Parent PR",
				Head:   github.PRBranch{Ref: "feature-parent"},
				Base:   github.PRBranch{Ref: "main"},
			},
		},
	}
	cfg := &config.DiffConfig{
		EnableStacking: true,
	}

	detector := NewBaseBranchDetector(mockRepo, mockClient, cfg, "owner", "repo")

	result, err := detector.DetectBaseBranch(context.Background(), "feature-child", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Base != "feature-parent" {
		t.Errorf("expected base 'feature-parent', got '%s'", result.Base)
	}
	if !result.IsStacking {
		t.Error("expected IsStacking to be true")
	}
	if result.ParentPR == nil {
		t.Error("expected ParentPR to be set")
	} else if result.ParentPR.Number != 123 {
		t.Errorf("expected ParentPR number 123, got %d", result.ParentPR.Number)
	}
	if result.Method != "auto-detected-stacking" {
		t.Errorf("expected method 'auto-detected-stacking', got '%s'", result.Method)
	}
}

func TestDetectBaseBranch_StackingDetected_SameCommitAsMain(t *testing.T) {
	// Test case for auto-branch scenario where:
	// - main (local) is at commit abc123 (1 commit ahead of origin/main)
	// - feature-auto is at commit abc123 (auto-created from main, has open PR)
	// - feature-child is at commit def456 (branched from feature-auto)
	// Expected: feature-child should stack on feature-auto, not main
	mockRepo := &mockRepository{
		defaultBranch: "main",
		branches: []git.BranchInfo{
			{Name: "main", Hash: "abc123"},
			{Name: "feature-auto", Hash: "abc123"}, // Same commit as main!
			{Name: "feature-child", Hash: "def456"},
		},
		mergeBaseFunc: func(ref1, ref2 string) (string, error) {
			// feature-child was branched from feature-auto
			if (ref1 == "feature-child" && ref2 == "feature-auto") ||
				(ref1 == "feature-auto" && ref2 == "feature-child") {
				return "abc123", nil // merge-base is abc123
			}
			// feature-child with main also has same merge-base
			if (ref1 == "feature-child" && ref2 == "main") ||
				(ref1 == "main" && ref2 == "feature-child") {
				return "abc123", nil // merge-base is also abc123!
			}
			return "abc123", nil
		},
		commitRange: func(from, to string) ([]git.CommitInfo, error) {
			// Simulate commits exist between abc123 and feature-child
			return []git.CommitInfo{{SHA: "commit1"}}, nil
		},
	}
	mockClient := &mockGitHubClient{
		pullRequests: []*github.PullRequest{
			{
				Number: 456,
				Title:  "Auto-branch PR",
				Head:   github.PRBranch{Ref: "feature-auto"},
				Base:   github.PRBranch{Ref: "main"},
			},
		},
	}
	cfg := &config.DiffConfig{
		EnableStacking: true,
	}

	detector := NewBaseBranchDetector(mockRepo, mockClient, cfg, "owner", "repo")

	result, err := detector.DetectBaseBranch(context.Background(), "feature-child", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Base != "feature-auto" {
		t.Errorf("expected base 'feature-auto', got '%s'", result.Base)
	}
	if !result.IsStacking {
		t.Error("expected IsStacking to be true")
	}
	if result.ParentPR == nil {
		t.Error("expected ParentPR to be set")
	} else if result.ParentPR.Number != 456 {
		t.Errorf("expected ParentPR number 456, got %d", result.ParentPR.Number)
	}
	if result.Method != "auto-detected-stacking" {
		t.Errorf("expected method 'auto-detected-stacking', got '%s'", result.Method)
	}
}

func TestFormatStackingMessage_NoStacking(t *testing.T) {
	result := &BaseBranchResult{
		Base:       "main",
		IsStacking: false,
		Method:     "default-branch",
	}

	msg := result.FormatStackingMessage()
	expected := "Creating PR with base: main"
	if msg != expected {
		t.Errorf("expected message '%s', got '%s'", expected, msg)
	}
}

func TestFormatStackingMessage_WithStacking(t *testing.T) {
	result := &BaseBranchResult{
		Base:       "feature-parent",
		IsStacking: true,
		ParentPR: &github.PullRequest{
			Number: 123,
			Title:  "Parent PR",
		},
		Method: "auto-detected-stacking",
	}

	msg := result.FormatStackingMessage()

	// Check that message contains key information
	if msg == "" {
		t.Error("expected non-empty message")
	}

	// Should contain stacking indicator
	if !contains(msg, "🔗") && !contains(msg, "stacking") {
		t.Error("expected message to indicate stacking")
	}

	// Should contain base branch
	if !contains(msg, "feature-parent") {
		t.Error("expected message to contain base branch")
	}

	// Should contain PR reference
	if !contains(msg, "#123") {
		t.Error("expected message to contain PR number")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
