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

// TestDetectBaseBranch_IndependentBranches_NoStacking tests that two independent
// branches from main do NOT stack, even when one has an open PR.
// This reproduces the bug from issue #4.
func TestDetectBaseBranch_IndependentBranches_NoStacking(t *testing.T) {
	// Scenario from issue #4:
	// main: A (initial commit)
	// feature-1: A → B → C → D (branched from main at A, has PR #41)
	// feature-2: A → E → F (branched from main at A, NO PR yet)
	// When running arc diff on feature-2, it should NOT stack on feature-1
	mockRepo := &mockRepository{
		defaultBranch: "main",
		branches: []git.BranchInfo{
			{Name: "main", Hash: "aaa111"},       // main at commit A
			{Name: "feature-1", Hash: "ddd444"}, // feature-1 at commit D
			{Name: "feature-2", Hash: "fff666"}, // feature-2 at commit F (current)
		},
		mergeBaseFunc: func(ref1, ref2 string) (string, error) {
			// Both branches diverged from the same point (A) on main
			// All merge-bases should return A
			return "aaa111", nil
		},
	}
	mockClient := &mockGitHubClient{
		pullRequests: []*github.PullRequest{
			{
				Number: 41,
				Title:  "Feature One",
				Head:   github.PRBranch{Ref: "feature-1"},
				Base:   github.PRBranch{Ref: "main"},
			},
		},
	}
	cfg := &config.DiffConfig{
		EnableStacking: true,
	}

	detector := NewBaseBranchDetector(mockRepo, mockClient, cfg, "owner", "repo")

	result, err := detector.DetectBaseBranch(context.Background(), "feature-2", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: Should NOT stack, should use main
	if result.Base != "main" {
		t.Errorf("expected base 'main', got '%s'", result.Base)
	}
	if result.IsStacking {
		t.Error("expected IsStacking to be false for independent branches")
	}
	if result.Method != "default-branch" {
		t.Errorf("expected method 'default-branch', got '%s'", result.Method)
	}
}

// TestDetectBaseBranch_IndependentBranches_DifferentMergeBases_NoStacking tests
// the scenario where main has moved forward after feature-1 was created, and
// feature-2 was created from the updated main. They should NOT stack.
func TestDetectBaseBranch_IndependentBranches_DifferentMergeBases_NoStacking(t *testing.T) {
	// Scenario: main moved forward between branch creations
	// Timeline:
	// 1. main at A
	// 2. feature-1 created from A, makes commits B, C, D
	// 3. main moves to A' (new commits or pull)
	// 4. feature-2 created from A', makes commits E, F
	//
	// Git graph:
	// A --- A' (main)
	// |      \
	// |       E --- F (feature-2, branched from A')
	// |
	// B --- C --- D (feature-1, branched from A)
	//
	// merge-base(feature-2, feature-1) = A (common ancestor)
	// merge-base(feature-2, main) = A' (feature-2 branched from A')
	// These are DIFFERENT! But feature-2 should NOT stack on feature-1.
	mockRepo := &mockRepository{
		defaultBranch: "main",
		branches: []git.BranchInfo{
			{Name: "main", Hash: "aaa222"},       // main at A'
			{Name: "feature-1", Hash: "ddd444"}, // feature-1 at D
			{Name: "feature-2", Hash: "fff666"}, // feature-2 at F (current)
		},
		mergeBaseFunc: func(ref1, ref2 string) (string, error) {
			// feature-2 with main: A' (where feature-2 branched)
			if (ref1 == "feature-2" && ref2 == "main") ||
				(ref1 == "main" && ref2 == "feature-2") {
				return "aaa222", nil
			}
			// feature-2 with feature-1: A (common ancestor before main moved)
			if (ref1 == "feature-2" && ref2 == "feature-1") ||
				(ref1 == "feature-1" && ref2 == "feature-2") {
				return "aaa111", nil
			}
			return "aaa111", nil
		},
		isAncestorFunc: func(ancestorRef, descendantRef string) (bool, error) {
			// A (aaa111) is an ancestor of feature-2 (through A → A' → E → F)
			if ancestorRef == "aaa111" && descendantRef == "feature-2" {
				return true, nil
			}
			return false, nil
		},
	}
	mockClient := &mockGitHubClient{
		pullRequests: []*github.PullRequest{
			{
				Number: 41,
				Title:  "Feature One",
				Head:   github.PRBranch{Ref: "feature-1"},
				Base:   github.PRBranch{Ref: "main"},
			},
		},
	}
	cfg := &config.DiffConfig{
		EnableStacking: true,
	}

	detector := NewBaseBranchDetector(mockRepo, mockClient, cfg, "owner", "repo")

	result, err := detector.DetectBaseBranch(context.Background(), "feature-2", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: Should NOT stack, should use main
	if result.Base != "main" {
		t.Errorf("expected base 'main', got '%s'", result.Base)
	}
	if result.IsStacking {
		t.Error("expected IsStacking to be false for independent branches")
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

// TestDetectBaseBranch_MultiLevelStacking_PicksClosestParent tests that when
// multiple branches form a chain (main → A → B → current), the detection
// picks B (direct parent) over A (grandparent).
func TestDetectBaseBranch_MultiLevelStacking_PicksClosestParent(t *testing.T) {
	// Scenario:
	// main at commitM
	//   └─ gw_sec_policy at commitC (PR #351 → main)
	//       └─ pdbs at commitD (PR #352 → gw_sec_policy)
	//           └─ cert_mgr_cleanup at commitE (current, no PR yet)
	//
	// Both gw_sec_policy and pdbs are valid stacking parents, but pdbs
	// is the direct parent and should be selected.
	mockRepo := &mockRepository{
		defaultBranch: "main",
		branches: []git.BranchInfo{
			{Name: "main", Hash: "commitM"},
			{Name: "gw_sec_policy", Hash: "commitC"},
			{Name: "pdbs", Hash: "commitD"},
			{Name: "cert_mgr_cleanup", Hash: "commitE"},
		},
		mergeBaseFunc: func(ref1, ref2 string) (string, error) {
			pair := ref1 + "+" + ref2
			switch pair {
			// cert_mgr_cleanup with main → commitM
			case "cert_mgr_cleanup+main", "main+cert_mgr_cleanup":
				return "commitM", nil
			// cert_mgr_cleanup with gw_sec_policy → commitC (grandparent divergence)
			case "cert_mgr_cleanup+gw_sec_policy", "gw_sec_policy+cert_mgr_cleanup":
				return "commitC", nil
			// cert_mgr_cleanup with pdbs → commitD (direct parent divergence)
			case "cert_mgr_cleanup+pdbs", "pdbs+cert_mgr_cleanup":
				return "commitD", nil
			// gw_sec_policy with main → commitM
			case "gw_sec_policy+main", "main+gw_sec_policy":
				return "commitM", nil
			// pdbs with main → commitM
			case "pdbs+main", "main+pdbs":
				return "commitM", nil
			default:
				return "commitM", nil
			}
		},
		isAncestorFunc: func(ancestorRef, descendantRef string) (bool, error) {
			// commitC is ancestor of commitD and commitE
			// commitD is ancestor of commitE
			// commitM is ancestor of everything
			ancestors := map[string]map[string]bool{
				"commitM": {"commitC": true, "commitD": true, "commitE": true, "cert_mgr_cleanup": true, "gw_sec_policy": true, "pdbs": true},
				"commitC": {"commitD": true, "commitE": true, "cert_mgr_cleanup": true, "pdbs": true},
				"commitD": {"commitE": true, "cert_mgr_cleanup": true},
			}
			if descendants, ok := ancestors[ancestorRef]; ok {
				return descendants[descendantRef], nil
			}
			return false, nil
		},
	}
	mockClient := &mockGitHubClient{
		pullRequests: []*github.PullRequest{
			{
				Number: 351,
				Title:  "Gateway security policy",
				Head:   github.PRBranch{Ref: "gw_sec_policy"},
				Base:   github.PRBranch{Ref: "main"},
			},
			{
				Number: 352,
				Title:  "Add PDBs",
				Head:   github.PRBranch{Ref: "pdbs"},
				Base:   github.PRBranch{Ref: "gw_sec_policy"},
			},
		},
	}
	cfg := &config.DiffConfig{
		EnableStacking: true,
	}

	detector := NewBaseBranchDetector(mockRepo, mockClient, cfg, "owner", "repo")

	result, err := detector.DetectBaseBranch(context.Background(), "cert_mgr_cleanup", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Base != "pdbs" {
		t.Errorf("expected base 'pdbs' (direct parent), got '%s'", result.Base)
	}
	if !result.IsStacking {
		t.Error("expected IsStacking to be true")
	}
	if result.ParentPR == nil {
		t.Error("expected ParentPR to be set")
	} else if result.ParentPR.Number != 352 {
		t.Errorf("expected ParentPR number 352, got %d", result.ParentPR.Number)
	}
	if result.Method != "auto-detected-stacking" {
		t.Errorf("expected method 'auto-detected-stacking', got '%s'", result.Method)
	}
}
