package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/serpro69/gh-arc/internal/diff"
	gitpkg "github.com/serpro69/gh-arc/internal/git"
)

// TestAutoBranchTemplateGeneration tests that the template is generated correctly
// when using the auto-branch-from-main feature.
//
// This is a regression test for the bug where:
// - Template showed "main → main" instead of "auto-branch → main"
// - Title was "Main" instead of the actual commit message
// - Summary was "No commits found" instead of actual commit summaries
func TestAutoBranchTemplateGeneration(t *testing.T) {
	tests := []struct {
		name              string
		commitMessages    []string
		expectedBranch    string // Should NOT be "main"
		expectCommits     bool   // Should find commits
		expectedTitlePart string // Part of title we expect to see
	}{
		{
			name:              "single_commit_on_main",
			commitMessages:    []string{"feat: add authentication"},
			expectedBranch:    "feature/auto-from-main-", // prefix
			expectCommits:     true,
			expectedTitlePart: "feat: add authentication",
		},
		{
			name: "multiple_commits_on_main",
			commitMessages: []string{
				"feat: add user model",
				"feat: add authentication",
			},
			expectedBranch:    "feature/auto-from-main-",
			expectCommits:     true,
			expectedTitlePart: "feat: add user model", // First chronological commit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test scenario 1: Initial auto-branch creation from main
			t.Run("from_main", func(t *testing.T) {
				runAutoBranchTest(t, tt, false)
			})

			// Test scenario 2: Running diff again from the created feature branch
			// This tests the fix for the reported bug where the template showed
			// "No commits found" when running diff from a feature branch
			t.Run("from_feature_branch", func(t *testing.T) {
				runAutoBranchTest(t, tt, true)
			})
		})
	}
}

func runAutoBranchTest(t *testing.T, tt struct {
	name              string
	commitMessages    []string
	expectedBranch    string
	expectCommits     bool
	expectedTitlePart string
}, checkoutFeatureBranch bool) {
	// Create temporary repository
	tmpDir := t.TempDir()
	gitRepo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to init repository: %v", err)
	}

	// Configure git user
	cfg, err := gitRepo.Config()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}
	cfg.User.Name = "Test User"
	cfg.User.Email = "test@example.com"
	if err := gitRepo.SetConfig(cfg); err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	// Create initial commit on main
	worktree, err := gitRepo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	initialFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(initialFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if _, err := worktree.Add("README.md"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	initialCommit, err := worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	// Create main branch reference
	mainRef := plumbing.NewHashReference("refs/heads/main", initialCommit)
	if err := gitRepo.Storer.SetReference(mainRef); err != nil {
		t.Fatalf("Failed to set main reference: %v", err)
	}

	// Create origin remote with main branch
	if _, err := gitRepo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://github.com/test/repo.git"},
	}); err != nil {
		t.Fatalf("Failed to create remote: %v", err)
	}

	// Create remote tracking branch (origin/main at initial commit)
	originMainRefName := plumbing.NewRemoteReferenceName("origin", "main")
	originMainRef := plumbing.NewHashReference(originMainRefName, initialCommit)
	if err := gitRepo.Storer.SetReference(originMainRef); err != nil {
		t.Fatalf("Failed to set origin/main reference: %v", err)
	}

	// Checkout main
	if err := worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("main"),
	}); err != nil {
		t.Fatalf("Failed to checkout main: %v", err)
	}

	// Create test commits on main (simulating unpushed commits)
	var lastCommit plumbing.Hash
	for i, msg := range tt.commitMessages {
		testFile := filepath.Join(tmpDir, "feature.txt")
		content := []byte(msg + "\n" + time.Now().String())
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("Failed to write file %d: %v", i, err)
		}
		if _, err := worktree.Add("feature.txt"); err != nil {
			t.Fatalf("Failed to add file %d: %v", i, err)
		}
		lastCommit, err = worktree.Commit(msg, &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		if err != nil {
			t.Fatalf("Failed to create commit %d: %v", i, err)
		}
	}

	// If testing from feature branch, create and checkout the feature branch
	var featureBranchName string
	if checkoutFeatureBranch {
		// Generate a feature branch name
		featureBranchName = "feature/auto-from-main-test"

		// Create feature branch at same commit as main
		featureBranchRef := plumbing.NewHashReference(
			plumbing.NewBranchReferenceName(featureBranchName),
			lastCommit,
		)
		if err := gitRepo.Storer.SetReference(featureBranchRef); err != nil {
			t.Fatalf("Failed to create feature branch reference: %v", err)
		}

		// Create remote tracking branch for feature (simulating pushed branch)
		originFeatureRefName := plumbing.NewRemoteReferenceName("origin", featureBranchName)
		originFeatureRef := plumbing.NewHashReference(originFeatureRefName, lastCommit)
		if err := gitRepo.Storer.SetReference(originFeatureRef); err != nil {
			t.Fatalf("Failed to set origin/%s reference: %v", featureBranchName, err)
		}

		// Checkout feature branch
		if err := worktree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(featureBranchName),
		}); err != nil {
			t.Fatalf("Failed to checkout feature branch: %v", err)
		}
	}

	// Now test the commit analysis flow
	repo, err := gitpkg.OpenRepository(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open repository: %v", err)
	}

	currentBranch, err := repo.GetCurrentBranch()
	if err != nil {
		t.Fatalf("Failed to get current branch: %v", err)
	}

	expectedCurrentBranch := "main"
	if checkoutFeatureBranch {
		expectedCurrentBranch = featureBranchName
	}
	if currentBranch != expectedCurrentBranch {
		t.Fatalf("Expected current branch to be %q, got %q", expectedCurrentBranch, currentBranch)
	}

	// Test commit analysis with remote base (the fix)
	// Always use origin/main as base to capture all unpushed commits
	commitAnalysisBase := "origin/main"

	analysis, err := diff.AnalyzeCommitsForTemplate(repo, commitAnalysisBase, currentBranch)
	if err != nil {
		t.Fatalf("Failed to analyze commits: %v", err)
	}

	// Verify commits were found
	if tt.expectCommits && analysis.CommitCount == 0 {
		t.Error("Expected to find commits, but got 0")
	}
	if tt.expectCommits && analysis.CommitCount != len(tt.commitMessages) {
		t.Errorf("Expected %d commits in analysis, got %d", len(tt.commitMessages), analysis.CommitCount)
	}

	// Verify title contains actual commit message (not "Main" or branch name)
	if tt.expectCommits {
		if analysis.Title == "" {
			t.Error("Expected non-empty title")
		}
		if !strings.Contains(analysis.Title, tt.expectedTitlePart) {
			t.Errorf("Expected title to contain %q, got %q", tt.expectedTitlePart, analysis.Title)
		}
		// Should not be derived from branch name
		if checkoutFeatureBranch && strings.Contains(analysis.Title, "Auto From Main") {
			t.Errorf("Title should not be derived from branch name, got %q", analysis.Title)
		}
		if !checkoutFeatureBranch && strings.Contains(analysis.Title, "Main") {
			t.Errorf("Title should not be 'Main', got %q", analysis.Title)
		}
	}

	// Verify summary is not "No commits found"
	// Note: Empty summary is OK for commits without body text
	if tt.expectCommits {
		if strings.Contains(analysis.Summary, "No commits found") {
			t.Errorf("Expected meaningful summary, got %q", analysis.Summary)
		}
		// For multiple commits, summary should list commits
		if len(tt.commitMessages) > 1 && !strings.Contains(analysis.Summary, "## Commits") {
			t.Errorf("Expected summary to contain commit list, got %q", analysis.Summary)
		}
	}

	t.Logf("✓ Current branch: %s", currentBranch)
	t.Logf("✓ Commit count: %d", analysis.CommitCount)
	t.Logf("✓ Title: %s", analysis.Title)
	t.Logf("✓ Summary: %s", analysis.Summary)
}
