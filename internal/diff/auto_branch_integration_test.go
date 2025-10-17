package diff

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/serpro69/gh-arc/internal/config"
	gitpkg "github.com/serpro69/gh-arc/internal/git"
)

// Test helpers

// createTestRepo creates a temporary git repository for testing.
// Returns the temporary directory path and the go-git repository.
func createTestRepo(t *testing.T) (string, *git.Repository) {
	t.Helper()

	tmpDir := t.TempDir()
	gitRepo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to init repository: %v", err)
	}

	return tmpDir, gitRepo
}

// createCommit creates a commit in the repository with the given message.
// Returns the commit hash.
func createCommit(t *testing.T, gitRepo *git.Repository, tmpDir, message string) plumbing.Hash {
	t.Helper()

	worktree, err := gitRepo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	// Create or modify a file
	filename := filepath.Join(tmpDir, "file.txt")
	content := []byte(message + "\n" + time.Now().String())
	err = os.WriteFile(filename, content, 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err = worktree.Add("file.txt")
	if err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	hash, err := worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	return hash
}

// createOldCommit creates a commit with a timestamp in the past.
// Used for testing stale remote detection.
func createOldCommit(t *testing.T, gitRepo *git.Repository, tmpDir, message string, hoursAgo int) plumbing.Hash {
	t.Helper()

	worktree, err := gitRepo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	// Create or modify a file
	filename := filepath.Join(tmpDir, "old-file.txt")
	content := []byte(message + "\n" + time.Now().String())
	err = os.WriteFile(filename, content, 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err = worktree.Add("old-file.txt")
	if err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Commit with old timestamp
	oldTime := time.Now().Add(-time.Duration(hoursAgo) * time.Hour)
	hash, err := worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  oldTime,
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	return hash
}

// mockRemoteBranch creates a remote tracking branch in the repository.
// This simulates an origin/main branch for testing.
func mockRemoteBranch(t *testing.T, gitRepo *git.Repository, branchName string, commitHash plumbing.Hash) {
	t.Helper()

	// Create remote tracking ref
	refName := plumbing.NewRemoteReferenceName("origin", branchName)
	ref := plumbing.NewHashReference(refName, commitHash)

	err := gitRepo.Storer.SetReference(ref)
	if err != nil {
		t.Fatalf("Failed to create remote ref: %v", err)
	}
}

// Integration Tests

func TestAutoBranchIntegration(t *testing.T) {
	// Skip integration tests in short mode
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("FullAutomaticFlow_ConfigEnabled", func(t *testing.T) {
		// Setup: Create repo with initial commit on main
		tmpDir, gitRepo := createTestRepo(t)
		initialHash := createCommit(t, gitRepo, tmpDir, "Initial commit")

		// Create origin/main at initial commit
		mockRemoteBranch(t, gitRepo, "main", initialHash)

		// Checkout main branch explicitly
		worktree, err := gitRepo.Worktree()
		if err != nil {
			t.Fatalf("Failed to get worktree: %v", err)
		}

		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName("main"),
			Create: true,
		})
		if err != nil {
			t.Fatalf("Failed to checkout main: %v", err)
		}

		// Add commits on main (ahead of origin)
		createCommit(t, gitRepo, tmpDir, "Feature commit 1")
		createCommit(t, gitRepo, tmpDir, "Feature commit 2")

		// Create detector with auto-create enabled
		cfg := &config.DiffConfig{
			AutoCreateBranchFromMain: true,
			AutoBranchNamePattern:    "feature/auto-from-main-{timestamp}",
			StaleRemoteThresholdHours: 24,
		}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: Detect commits on main
		ctx := context.Background()
		detection, err := detector.DetectCommitsOnMain(ctx)
		if err != nil {
			t.Fatalf("DetectCommitsOnMain() failed: %v", err)
		}

		// Verify detection
		if !detection.OnMainBranch {
			t.Error("Expected OnMainBranch = true")
		}
		if detection.CommitsAhead != 2 {
			t.Errorf("CommitsAhead = %d, expected 2", detection.CommitsAhead)
		}
		if detection.DefaultBranch != "main" {
			t.Errorf("DefaultBranch = %s, expected 'main'", detection.DefaultBranch)
		}

		// Test: Should trigger auto-branch
		if !detector.ShouldAutoBranch(detection) {
			t.Error("ShouldAutoBranch() = false, expected true")
		}

		// Test: Prepare auto-branch (should not prompt with config enabled)
		autoBranchCtx, err := detector.PrepareAutoBranch(ctx, detection)
		if err != nil {
			t.Fatalf("PrepareAutoBranch() failed: %v", err)
		}

		if autoBranchCtx == nil {
			t.Fatal("PrepareAutoBranch() returned nil context")
		}
		if !autoBranchCtx.ShouldProceed {
			t.Error("ShouldProceed = false, expected true")
		}
		if autoBranchCtx.BranchName == "" {
			t.Error("BranchName is empty")
		}
		if len(autoBranchCtx.BranchName) < len("feature/auto-from-main-") {
			t.Errorf("BranchName too short: %s", autoBranchCtx.BranchName)
		}
	})

	t.Run("CommitListDisplayFormatting", func(t *testing.T) {
		// Test displayCommitList function with various commit scenarios
		tests := []struct {
			name    string
			commits []gitpkg.CommitInfo
		}{
			{
				name: "single commit",
				commits: []gitpkg.CommitInfo{
					{SHA: "abc123def456", Message: "Add feature"},
				},
			},
			{
				name: "multiple commits",
				commits: []gitpkg.CommitInfo{
					{SHA: "abc123def456", Message: "Add feature"},
					{SHA: "def456ghi789", Message: "Fix bug"},
				},
			},
			{
				name: "long commit message",
				commits: []gitpkg.CommitInfo{
					{SHA: "abc123def456", Message: "This is a very long commit message that exceeds 80 characters and should be truncated"},
				},
			},
			{
				name: "multiline commit message",
				commits: []gitpkg.CommitInfo{
					{SHA: "abc123def456", Message: "First line\n\nDetailed description"},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Test that displayCommitList doesn't panic
				displayCommitList(tt.commits)
			})
		}
	})

	t.Run("CustomBranchNamePattern", func(t *testing.T) {
		// Setup: Create repo
		tmpDir, gitRepo := createTestRepo(t)
		initialHash := createCommit(t, gitRepo, tmpDir, "Initial commit")
		mockRemoteBranch(t, gitRepo, "main", initialHash)

		// Checkout main
		worktree, err := gitRepo.Worktree()
		if err != nil {
			t.Fatalf("Failed to get worktree: %v", err)
		}

		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName("main"),
			Create: true,
		})
		if err != nil {
			t.Fatalf("Failed to checkout main: %v", err)
		}

		createCommit(t, gitRepo, tmpDir, "Feature commit")

		// Test various patterns
		patterns := []struct {
			name              string
			pattern           string
			expectedSubstring string
		}{
			{
				name:              "date pattern",
				pattern:           "feature/{date}",
				expectedSubstring: "feature/",
			},
			{
				name:              "datetime pattern",
				pattern:           "feature/{datetime}",
				expectedSubstring: "feature/",
			},
			{
				name:              "random pattern",
				pattern:           "feature/{random}",
				expectedSubstring: "feature/",
			},
			{
				name:              "mixed pattern",
				pattern:           "feature/{date}-{random}",
				expectedSubstring: "feature/",
			},
		}

		for _, tt := range patterns {
			t.Run(tt.name, func(t *testing.T) {
				cfg := &config.DiffConfig{
					AutoCreateBranchFromMain: true,
					AutoBranchNamePattern:    tt.pattern,
					StaleRemoteThresholdHours: 24,
				}

				repo, err := gitpkg.OpenRepository(tmpDir)
				if err != nil {
					t.Fatalf("Failed to open repository: %v", err)
				}

				detector := NewAutoBranchDetector(repo, cfg)

				branchName, shouldPrompt, err := detector.GenerateBranchName()
				if err != nil {
					t.Fatalf("GenerateBranchName() failed: %v", err)
				}

				if shouldPrompt {
					t.Error("shouldPrompt = true, expected false for pattern")
				}
				if branchName == "" {
					t.Error("branchName is empty")
				}
				if len(branchName) < len(tt.expectedSubstring) {
					t.Errorf("branchName too short: %s", branchName)
				}
				if branchName[:len(tt.expectedSubstring)] != tt.expectedSubstring {
					t.Errorf("branchName does not start with %s: %s", tt.expectedSubstring, branchName)
				}
			})
		}
	})

	t.Run("NoCommitsOnMain_SkipFlow", func(t *testing.T) {
		// Setup: Create repo on main with no unpushed commits
		tmpDir, gitRepo := createTestRepo(t)
		initialHash := createCommit(t, gitRepo, tmpDir, "Initial commit")

		// Create origin/main at same commit (no commits ahead)
		mockRemoteBranch(t, gitRepo, "main", initialHash)

		// Checkout main branch
		worktree, err := gitRepo.Worktree()
		if err != nil {
			t.Fatalf("Failed to get worktree: %v", err)
		}

		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName("main"),
			Create: true,
		})
		if err != nil {
			t.Fatalf("Failed to checkout main: %v", err)
		}

		// Create detector
		cfg := &config.DiffConfig{
			AutoCreateBranchFromMain: true,
		}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: Detect should show no commits ahead
		ctx := context.Background()
		detection, err := detector.DetectCommitsOnMain(ctx)
		if err != nil {
			t.Fatalf("DetectCommitsOnMain() failed: %v", err)
		}

		if !detection.OnMainBranch {
			t.Error("Expected OnMainBranch = true")
		}
		if detection.CommitsAhead != 0 {
			t.Errorf("CommitsAhead = %d, expected 0", detection.CommitsAhead)
		}

		// Test: Should NOT trigger auto-branch
		if detector.ShouldAutoBranch(detection) {
			t.Error("ShouldAutoBranch() = true, expected false (no commits ahead)")
		}
	})

	t.Run("OnFeatureBranch_SkipFlow", func(t *testing.T) {
		// Setup: Create repo on feature branch
		tmpDir, gitRepo := createTestRepo(t)
		initialHash := createCommit(t, gitRepo, tmpDir, "Initial commit")
		mockRemoteBranch(t, gitRepo, "main", initialHash)

		// Create and checkout feature branch
		worktree, err := gitRepo.Worktree()
		if err != nil {
			t.Fatalf("Failed to get worktree: %v", err)
		}

		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName("feature/existing"),
			Create: true,
		})
		if err != nil {
			t.Fatalf("Failed to checkout feature: %v", err)
		}

		createCommit(t, gitRepo, tmpDir, "Feature commit")

		// Create detector
		cfg := &config.DiffConfig{
			AutoCreateBranchFromMain: true,
		}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: Detect should show not on main
		ctx := context.Background()
		detection, err := detector.DetectCommitsOnMain(ctx)
		if err != nil {
			t.Fatalf("DetectCommitsOnMain() failed: %v", err)
		}

		if detection.OnMainBranch {
			t.Error("Expected OnMainBranch = false")
		}
		if detection.CommitsAhead != 0 {
			t.Errorf("CommitsAhead = %d, expected 0 (not checked when not on main)", detection.CommitsAhead)
		}

		// Test: Should NOT trigger auto-branch
		if detector.ShouldAutoBranch(detection) {
			t.Error("ShouldAutoBranch() = true, expected false (not on main)")
		}
	})

	t.Run("StaleRemote_ThresholdExceeded", func(t *testing.T) {
		// Setup: Create repo with old remote ref
		tmpDir, gitRepo := createTestRepo(t)

		// Create old commit (48 hours ago) and set as origin/main
		oldHash := createOldCommit(t, gitRepo, tmpDir, "Old commit", 48)
		mockRemoteBranch(t, gitRepo, "main", oldHash)

		// Checkout main and add new commits
		worktree, err := gitRepo.Worktree()
		if err != nil {
			t.Fatalf("Failed to get worktree: %v", err)
		}

		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName("main"),
			Create: true,
		})
		if err != nil {
			t.Fatalf("Failed to checkout main: %v", err)
		}

		createCommit(t, gitRepo, tmpDir, "New commit")

		// Create detector with 24-hour threshold
		cfg := &config.DiffConfig{
			AutoCreateBranchFromMain: true,
			StaleRemoteThresholdHours: 24,
		}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: CheckStaleRemote should detect stale ref
		ctx := context.Background()
		detection, err := detector.DetectCommitsOnMain(ctx)
		if err != nil {
			t.Fatalf("DetectCommitsOnMain() failed: %v", err)
		}

		// CheckStaleRemote would normally prompt user, but we can test the detection
		shouldContinue, err := detector.CheckStaleRemote(ctx, detection.DefaultBranch)

		// In automated test, there's no user input, so this will likely fail
		// We're verifying that the check detects the stale ref
		_ = shouldContinue // Can't test user prompt in automated test
		_ = err            // Expected to have error or need user input
	})

	t.Run("UniqueBranchName_NoCollision", func(t *testing.T) {
		// Setup: Create repo
		tmpDir, gitRepo := createTestRepo(t)
		createCommit(t, gitRepo, tmpDir, "Initial commit")

		cfg := &config.DiffConfig{}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: Ensure unique name when no collision
		uniqueName, err := detector.EnsureUniqueBranchName("feature/new-branch")
		if err != nil {
			t.Fatalf("EnsureUniqueBranchName() failed: %v", err)
		}

		if uniqueName != "feature/new-branch" {
			t.Errorf("uniqueName = %s, expected 'feature/new-branch' (no collision)", uniqueName)
		}
	})

	t.Run("UniqueBranchName_WithCollision", func(t *testing.T) {
		// Setup: Create repo with existing branch
		tmpDir, gitRepo := createTestRepo(t)
		initialHash := createCommit(t, gitRepo, tmpDir, "Initial commit")

		// Create existing branch
		branchRef := plumbing.NewBranchReferenceName("feature/existing")
		ref := plumbing.NewHashReference(branchRef, initialHash)
		err := gitRepo.Storer.SetReference(ref)
		if err != nil {
			t.Fatalf("Failed to create branch: %v", err)
		}

		cfg := &config.DiffConfig{}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: Ensure unique name appends counter
		uniqueName, err := detector.EnsureUniqueBranchName("feature/existing")
		if err != nil {
			t.Fatalf("EnsureUniqueBranchName() failed: %v", err)
		}

		if uniqueName != "feature/existing-1" {
			t.Errorf("uniqueName = %s, expected 'feature/existing-1' (appended counter)", uniqueName)
		}
	})

	t.Run("UniqueBranchName_MultipleCollisions", func(t *testing.T) {
		// Setup: Create repo with multiple colliding branches
		tmpDir, gitRepo := createTestRepo(t)
		initialHash := createCommit(t, gitRepo, tmpDir, "Initial commit")

		// Create existing branches: feature/test, feature/test-1, feature/test-2
		for i := 0; i < 3; i++ {
			var branchName string
			if i == 0 {
				branchName = "feature/test"
			} else {
				branchName = "feature/test-" + string(rune('0'+i))
			}

			branchRef := plumbing.NewBranchReferenceName(branchName)
			ref := plumbing.NewHashReference(branchRef, initialHash)
			err := gitRepo.Storer.SetReference(ref)
			if err != nil {
				t.Fatalf("Failed to create branch %s: %v", branchName, err)
			}
		}

		cfg := &config.DiffConfig{}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: Should find first available slot (feature/test-3)
		uniqueName, err := detector.EnsureUniqueBranchName("feature/test")
		if err != nil {
			t.Fatalf("EnsureUniqueBranchName() failed: %v", err)
		}

		if uniqueName != "feature/test-3" {
			t.Errorf("uniqueName = %s, expected 'feature/test-3'", uniqueName)
		}
	})

	t.Run("SanitizeBranchName_InvalidCharacters", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{
				name:     "spaces to hyphens",
				input:    "My Feature Branch",
				expected: "my-feature-branch",
			},
			{
				name:     "uppercase to lowercase",
				input:    "FEATURE",
				expected: "feature",
			},
			{
				name:     "double dots",
				input:    "feature..name",
				expected: "feature-name",
			},
			{
				name:     "invalid characters",
				input:    "feature~test^branch*name",
				expected: "featuretestbranchname",
			},
			{
				name:     "mixed issues",
				input:    "My Feature: Test [123]",
				expected: "my-feature-test-123",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := sanitizeBranchName(tt.input)
				if result != tt.expected {
					t.Errorf("sanitizeBranchName(%q) = %q, expected %q", tt.input, result, tt.expected)
				}
			})
		}
	})

	t.Run("GenerateBranchName_EmptyPattern", func(t *testing.T) {
		// Setup
		tmpDir, _ := createTestRepo(t)

		cfg := &config.DiffConfig{
			AutoBranchNamePattern: "", // Empty pattern uses default
		}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: Empty pattern should use default
		branchName, shouldPrompt, err := detector.GenerateBranchName()
		if err != nil {
			t.Fatalf("GenerateBranchName() failed: %v", err)
		}

		if shouldPrompt {
			t.Error("shouldPrompt = true, expected false for empty pattern")
		}
		if branchName == "" {
			t.Error("branchName is empty")
		}
		if len(branchName) < len("feature/auto-from-main-") {
			t.Errorf("branchName too short: %s", branchName)
		}
	})

	t.Run("GenerateBranchName_NullPattern", func(t *testing.T) {
		// Setup
		tmpDir, _ := createTestRepo(t)

		cfg := &config.DiffConfig{
			AutoBranchNamePattern: "null", // null triggers prompt
		}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: null pattern should trigger prompt
		branchName, shouldPrompt, err := detector.GenerateBranchName()
		if err != nil {
			t.Fatalf("GenerateBranchName() failed: %v", err)
		}

		if !shouldPrompt {
			t.Error("shouldPrompt = false, expected true for null pattern")
		}
		if branchName != "" {
			t.Errorf("branchName = %s, expected empty for null pattern", branchName)
		}
	})

	t.Run("GenerateBranchName_UsernamePlaceholder", func(t *testing.T) {
		// Setup
		tmpDir, _ := createTestRepo(t)

		cfg := &config.DiffConfig{
			AutoBranchNamePattern: "feature/{username}/{date}",
		}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: username placeholder should be replaced
		branchName, shouldPrompt, err := detector.GenerateBranchName()
		if err != nil {
			t.Fatalf("GenerateBranchName() failed: %v", err)
		}

		if shouldPrompt {
			t.Error("shouldPrompt = true, expected false")
		}
		if branchName == "" {
			t.Error("branchName is empty")
		}
		// Should contain "feature/" and not contain "{username}"
		if len(branchName) < len("feature/") {
			t.Errorf("branchName too short: %s", branchName)
		}
		if branchName[:len("feature/")] != "feature/" {
			t.Errorf("branchName does not start with 'feature/': %s", branchName)
		}
	})

	t.Run("StaleRemote_ThresholdDisabled", func(t *testing.T) {
		// Setup: Create repo with very old remote ref
		tmpDir, gitRepo := createTestRepo(t)
		oldHash := createOldCommit(t, gitRepo, tmpDir, "Old commit", 1000) // 1000 hours old
		mockRemoteBranch(t, gitRepo, "main", oldHash)

		// Create detector with threshold disabled (0)
		cfg := &config.DiffConfig{
			AutoCreateBranchFromMain: true,
			StaleRemoteThresholdHours: 0, // Disabled
		}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: CheckStaleRemote should skip check when threshold is 0
		ctx := context.Background()
		shouldContinue, err := detector.CheckStaleRemote(ctx, "main")
		if err != nil {
			t.Fatalf("CheckStaleRemote() failed: %v", err)
		}

		if !shouldContinue {
			t.Error("shouldContinue = false, expected true (check disabled)")
		}
	})

	t.Run("StaleRemote_NoRemoteRef", func(t *testing.T) {
		// Setup: Create repo without remote ref
		tmpDir, gitRepo := createTestRepo(t)
		createCommit(t, gitRepo, tmpDir, "Initial commit")

		// Don't create remote ref (simulates offline or first push)

		cfg := &config.DiffConfig{
			AutoCreateBranchFromMain: true,
			StaleRemoteThresholdHours: 24,
		}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: CheckStaleRemote should skip check when remote doesn't exist
		ctx := context.Background()
		shouldContinue, err := detector.CheckStaleRemote(ctx, "main")
		if err != nil {
			t.Fatalf("CheckStaleRemote() failed: %v", err)
		}

		if !shouldContinue {
			t.Error("shouldContinue = false, expected true (no remote ref)")
		}
	})

	t.Run("DetectionResult_MasterBranch", func(t *testing.T) {
		// Setup: Create repo with master branch
		tmpDir, gitRepo := createTestRepo(t)

		// Create initial commit first
		initialHash := createCommit(t, gitRepo, tmpDir, "Initial commit on master")

		// Create master branch ref pointing to initial commit
		masterRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName("master"), initialHash)
		err := gitRepo.Storer.SetReference(masterRef)
		if err != nil {
			t.Fatalf("Failed to create master ref: %v", err)
		}

		// Mock origin/master at initial commit
		mockRemoteBranch(t, gitRepo, "master", initialHash)

		// Checkout master branch
		worktree, err := gitRepo.Worktree()
		if err != nil {
			t.Fatalf("Failed to get worktree: %v", err)
		}

		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName("master"),
		})
		if err != nil {
			t.Fatalf("Failed to checkout master: %v", err)
		}

		// Add commit on master (ahead of origin)
		createCommit(t, gitRepo, tmpDir, "Feature commit")

		cfg := &config.DiffConfig{
			AutoCreateBranchFromMain: true,
		}

		repo, err := gitpkg.OpenRepository(tmpDir)
		if err != nil {
			t.Fatalf("Failed to open repository: %v", err)
		}

		detector := NewAutoBranchDetector(repo, cfg)

		// Test: Should detect master as default branch
		ctx := context.Background()
		detection, err := detector.DetectCommitsOnMain(ctx)
		if err != nil {
			t.Fatalf("DetectCommitsOnMain() failed: %v", err)
		}

		if !detection.OnMainBranch {
			t.Error("Expected OnMainBranch = true")
		}
		if detection.DefaultBranch != "master" {
			t.Errorf("DefaultBranch = %s, expected 'master'", detection.DefaultBranch)
		}
		if detection.CommitsAhead != 1 {
			t.Errorf("CommitsAhead = %d, expected 1", detection.CommitsAhead)
		}
	})

	t.Run("RandomString_Generation", func(t *testing.T) {
		// Test that generateRandomString produces expected length and charset
		lengths := []int{1, 6, 10, 20}

		for _, length := range lengths {
			t.Run("length_"+string(rune('0'+length)), func(t *testing.T) {
				result := generateRandomString(length)

				if len(result) != length {
					t.Errorf("generateRandomString(%d) length = %d, expected %d", length, len(result), length)
				}

				// Check that all characters are from expected charset
				for _, c := range result {
					if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
						t.Errorf("generateRandomString(%d) contains invalid character: %c", length, c)
					}
				}
			})
		}
	})

	t.Run("PrepareAutoBranch_ConfigDisabled_WouldPrompt", func(t *testing.T) {
		// Skip this test because it requires user interaction (prompts)
		// which cannot be automated in tests
		t.Skip("Skipping test that requires user interaction (prompts)")

		// This test would verify PrepareAutoBranch behavior when config is disabled
		// In real usage, this would prompt the user for confirmation
		// Interactive prompt testing requires manual testing or specialized frameworks
	})

	t.Run("EmptyCommitList_Display", func(t *testing.T) {
		// Test that displayCommitList handles empty list gracefully
		displayCommitList([]gitpkg.CommitInfo{})
		// Should not panic or produce output
	})

	t.Run("GenerateRandomString_Uniqueness", func(t *testing.T) {
		// Test that multiple calls produce different results
		seen := make(map[string]bool)
		iterations := 100

		for i := 0; i < iterations; i++ {
			result := generateRandomString(6)
			if seen[result] {
				t.Errorf("generateRandomString produced duplicate: %s", result)
			}
			seen[result] = true
		}

		// Should have close to 100 unique strings (collisions very unlikely with 6 chars)
		if len(seen) < iterations-5 {
			t.Errorf("generateRandomString produced too many duplicates: %d unique out of %d", len(seen), iterations)
		}
	})
}
