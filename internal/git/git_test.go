package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenRepository tests basic repository opening functionality
func TestOpenRepository(t *testing.T) {
	t.Run("open current repository", func(t *testing.T) {
		// This test assumes we're running in the gh-arc git repository
		repo, err := OpenRepository("../..")
		require.NoError(t, err)
		assert.NotNil(t, repo)
		assert.NotNil(t, repo.Repo())
		assert.NotEmpty(t, repo.Path())
	})

	t.Run("non-existent repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := OpenRepository(tmpDir)
		assert.ErrorIs(t, err, ErrNotARepository)
	})
}

// TestFindRepositoryRoot tests repository root detection
func TestFindRepositoryRoot(t *testing.T) {
	t.Run("find root from subdirectory", func(t *testing.T) {
		// Get current working directory
		cwd, err := os.Getwd()
		require.NoError(t, err)

		// Find repository root
		root, err := FindRepositoryRoot(cwd)
		require.NoError(t, err)
		assert.NotEmpty(t, root)

		// Verify .git exists in root
		gitDir := filepath.Join(root, ".git")
		info, err := os.Stat(gitDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("non-repository directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := FindRepositoryRoot(tmpDir)
		assert.ErrorIs(t, err, ErrNotARepository)
	})
}

// TestIsValidRepository tests repository validation
func TestIsValidRepository(t *testing.T) {
	t.Run("valid repository", func(t *testing.T) {
		root, err := FindRepositoryRoot("")
		require.NoError(t, err)
		assert.True(t, IsValidRepository(root))
	})

	t.Run("invalid repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		assert.False(t, IsValidRepository(tmpDir))
	})
}

// TestRepositoryWithTempRepo tests repository operations with a temporary repository
func TestRepositoryWithTempRepo(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Initialize a new git repository
	_, err := git.PlainInit(tmpDir, false)
	require.NoError(t, err)

	// Open the repository
	repo, err := OpenRepository(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, repo)
	assert.Equal(t, tmpDir, repo.Path())

	// Verify it's a valid repository
	assert.True(t, IsValidRepository(tmpDir))

	// Verify we can find the root
	root, err := FindRepositoryRoot(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, tmpDir, root)
}

// TestIsDetachedHead tests detached HEAD detection
func TestIsDetachedHead(t *testing.T) {
	t.Run("not detached in normal branch", func(t *testing.T) {
		// Open the current repository
		repo, err := OpenRepository("../..")
		require.NoError(t, err)

		isDetached, err := repo.IsDetachedHead()
		require.NoError(t, err)
		// In normal development, we're on a branch, not detached
		assert.False(t, isDetached)
	})

	t.Run("newly initialized repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Newly initialized repos might not have HEAD yet
		// Just verify the method doesn't crash
		_, err = repo.IsDetachedHead()
		// Error is acceptable for unborn HEAD
		_ = err
	})
}

// TestGetWorkingDirectoryStatus tests working directory status detection
func TestGetWorkingDirectoryStatus(t *testing.T) {
	t.Run("clean repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		// Create initial commit
		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create and commit a file
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("initial content"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Now check status
		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		status, err := repo.GetWorkingDirectoryStatus()
		require.NoError(t, err)
		assert.True(t, status.IsClean)
		assert.Empty(t, status.StagedFiles)
		assert.Empty(t, status.UnstagedFiles)
		assert.Empty(t, status.UntrackedFiles)
	})

	t.Run("dirty repository with untracked file", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		// Create initial commit
		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("initial content"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Add untracked file
		untrackedFile := filepath.Join(tmpDir, "untracked.txt")
		err = os.WriteFile(untrackedFile, []byte("untracked"), 0644)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		status, err := repo.GetWorkingDirectoryStatus()
		require.NoError(t, err)
		assert.False(t, status.IsClean)
		assert.Empty(t, status.StagedFiles)
		assert.Empty(t, status.UnstagedFiles)
		assert.Contains(t, status.UntrackedFiles, "untracked.txt")
	})

	t.Run("dirty repository with staged file", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("initial content"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Modify and stage a file
		err = os.WriteFile(testFile, []byte("modified content"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		status, err := repo.GetWorkingDirectoryStatus()
		require.NoError(t, err)
		assert.False(t, status.IsClean)
		assert.Contains(t, status.StagedFiles, "test.txt")
		assert.Empty(t, status.UnstagedFiles)
		assert.Empty(t, status.UntrackedFiles)
	})

	t.Run("dirty repository with unstaged modification", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("initial content"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Modify file without staging
		err = os.WriteFile(testFile, []byte("modified content"), 0644)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		status, err := repo.GetWorkingDirectoryStatus()
		require.NoError(t, err)
		assert.False(t, status.IsClean)
		assert.Empty(t, status.StagedFiles)
		assert.Contains(t, status.UnstagedFiles, "test.txt")
		assert.Empty(t, status.UntrackedFiles)
	})
}

// TestGetRepositoryState tests complete repository state detection
func TestGetRepositoryState(t *testing.T) {
	t.Run("complete state of clean repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("initial content"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		commit, err := worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		state, err := repo.GetRepositoryState()
		require.NoError(t, err)
		assert.True(t, state.IsValid)
		assert.False(t, state.IsDetached)
		assert.NotEmpty(t, state.CurrentBranch)
		assert.Equal(t, commit.String(), state.HeadCommit)
		assert.True(t, state.WorkingDir.IsClean)
	})

	t.Run("state of dirty repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("initial content"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Add untracked file to make repository dirty
		untrackedFile := filepath.Join(tmpDir, "untracked.txt")
		err = os.WriteFile(untrackedFile, []byte("untracked"), 0644)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		state, err := repo.GetRepositoryState()
		require.NoError(t, err)
		assert.True(t, state.IsValid)
		assert.False(t, state.WorkingDir.IsClean)
		assert.Contains(t, state.WorkingDir.UntrackedFiles, "untracked.txt")
	})
}

// TestGetCurrentBranch tests getting the current branch name
func TestGetCurrentBranch(t *testing.T) {
	t.Run("current branch in real repo", func(t *testing.T) {
		repo, err := OpenRepository("../..")
		require.NoError(t, err)

		branch, err := repo.GetCurrentBranch()
		require.NoError(t, err)
		assert.NotEmpty(t, branch)
	})

	t.Run("current branch in new repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		branch, err := repo.GetCurrentBranch()
		require.NoError(t, err)
		assert.NotEmpty(t, branch)
		// Default branch is usually "master" in go-git
		assert.Contains(t, branch, "master")
	})
}

// TestGetDefaultBranch tests detecting the default branch
func TestGetDefaultBranch(t *testing.T) {
	t.Run("default branch detection", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		defaultBranch, err := repo.GetDefaultBranch()
		require.NoError(t, err)
		assert.NotEmpty(t, defaultBranch)
	})
}

// TestCreateBranch tests branch creation
func TestCreateBranch(t *testing.T) {
	t.Run("create branch from HEAD", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create new branch from HEAD
		err = repo.CreateBranch("feature-branch", "")
		require.NoError(t, err)

		// Verify branch exists
		branches, err := repo.ListBranches(false)
		require.NoError(t, err)

		found := false
		for _, b := range branches {
			if b.Name == "feature-branch" {
				found = true
				break
			}
		}
		assert.True(t, found, "feature-branch should exist")
	})

	t.Run("create branch from base branch", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		currentBranch, err := repo.GetCurrentBranch()
		require.NoError(t, err)

		// Create new branch from current branch
		err = repo.CreateBranch("feature-branch", currentBranch)
		require.NoError(t, err)

		// Verify branch exists
		branches, err := repo.ListBranches(false)
		require.NoError(t, err)

		found := false
		for _, b := range branches {
			if b.Name == "feature-branch" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

// TestListBranches tests listing branches
func TestListBranches(t *testing.T) {
	t.Run("list local branches", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create a second branch
		err = repo.CreateBranch("dev", "")
		require.NoError(t, err)

		// List branches
		branches, err := repo.ListBranches(false)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(branches), 2)

		// All listed branches should be local
		for _, b := range branches {
			assert.False(t, b.IsRemote)
		}
	})
}

// TestGetGitConfig tests reading git configuration
func TestGetGitConfig(t *testing.T) {
	t.Run("get user config via go-git", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		// Set config
		cfg, err := gitRepo.Config()
		require.NoError(t, err)
		cfg.User.Name = "Test User"
		cfg.User.Email = "test@example.com"
		err = gitRepo.SetConfig(cfg)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		name, err := repo.GetGitConfig("user.name")
		require.NoError(t, err)
		assert.Equal(t, "Test User", name)

		email, err := repo.GetGitConfig("user.email")
		require.NoError(t, err)
		assert.Equal(t, "test@example.com", email)
	})
}

// TestParseConfigKey tests parsing git config keys
func TestParseConfigKey(t *testing.T) {
	testCases := []struct {
		name           string
		key            string
		expSection     string
		expSubsection  string
		expOption      string
	}{
		{
			name:           "two parts - simple key",
			key:            "user.name",
			expSection:     "user",
			expSubsection:  "",
			expOption:      "name",
		},
		{
			name:           "three parts - subsection key",
			key:            "remote.origin.url",
			expSection:     "remote",
			expSubsection:  "origin",
			expOption:      "url",
		},
		{
			name:           "four parts - subsection with dot",
			key:            "url.https://example.com/.insteadOf",
			expSection:     "url",
			expSubsection:  "https://example.com/",
			expOption:      "insteadOf",
		},
		{
			name:           "five parts - subsection with multiple dots",
			key:            "http.https://weak.example.com.sslVerify",
			expSection:     "http",
			expSubsection:  "https://weak.example.com",
			expOption:      "sslVerify",
		},
		{
			name:           "one part - invalid key",
			key:            "invalid",
			expSection:     "",
			expSubsection:  "",
			expOption:      "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			section, subsection, option := parseConfigKey(tc.key)
			assert.Equal(t, tc.expSection, section)
			assert.Equal(t, tc.expSubsection, subsection)
			assert.Equal(t, tc.expOption, option)
		})
	}
}

// TestGetCommitRange tests getting commits between two branches
func TestGetCommitRange(t *testing.T) {
	t.Run("commits between branches", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit on master
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("initial"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Create feature branch
		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Checkout feature branch
		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: "refs/heads/feature",
		})
		require.NoError(t, err)

		// Add commits to feature branch
		for i := 1; i <= 3; i++ {
			testFile := filepath.Join(tmpDir, "feature.txt")
			err = os.WriteFile(testFile, []byte("feature "+string(rune(i))), 0644)
			require.NoError(t, err)

			_, err = worktree.Add("feature.txt")
			require.NoError(t, err)

			_, err = worktree.Commit("feature commit "+string(rune(i)), &git.CommitOptions{
				Author: &object.Signature{
					Name:  "Test User",
					Email: "test@example.com",
					When:  time.Now().Add(time.Duration(i) * time.Minute),
				},
			})
			require.NoError(t, err)
		}

		// Get commits between master and feature
		commits, err := repo.GetCommitRange("master", "feature")
		require.NoError(t, err)
		assert.Equal(t, 3, len(commits))

		// Verify commits are in correct order (newest first)
		for i, commit := range commits {
			assert.NotEmpty(t, commit.SHA)
			assert.Equal(t, "Test User", commit.Author)
			assert.Equal(t, "test@example.com", commit.Email)
			assert.Contains(t, commit.Message, "feature commit")
			// Commits should be in reverse chronological order
			if i > 0 {
				assert.True(t, commits[i].Date.Before(commits[i-1].Date) || commits[i].Date.Equal(commits[i-1].Date))
			}
		}
	})

	t.Run("no commits between branches", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create branch at same point
		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Get commits - should be empty
		commits, err := repo.GetCommitRange("master", "feature")
		require.NoError(t, err)
		assert.Empty(t, commits)
	})
}

// TestGetCommitsBetween tests getting commits between any refs
func TestGetCommitsBetween(t *testing.T) {
	t.Run("commits between refs", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create base commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("base"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		baseCommit, err := worktree.Commit("base commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Create feature commits
		for i := 1; i <= 2; i++ {
			err = os.WriteFile(testFile, []byte("feature "+string(rune(i))), 0644)
			require.NoError(t, err)

			_, err = worktree.Add("test.txt")
			require.NoError(t, err)

			_, err = worktree.Commit("feature commit "+string(rune(i)), &git.CommitOptions{
				Author: &object.Signature{
					Name:  "Test User",
					Email: "test@example.com",
					When:  time.Now().Add(time.Duration(i) * time.Minute),
				},
			})
			require.NoError(t, err)
		}

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Get commits using commit SHAs
		currentBranch, err := repo.GetCurrentBranch()
		require.NoError(t, err)

		commits, err := repo.GetCommitsBetween(baseCommit.String(), currentBranch)
		require.NoError(t, err)
		assert.Equal(t, 2, len(commits))
	})
}

// TestParseCommitMessage tests parsing commit messages
func TestParseCommitMessage(t *testing.T) {
	testCases := []struct {
		name     string
		message  string
		expTitle string
		expBody  string
	}{
		{
			name:     "simple commit",
			message:  "Add feature",
			expTitle: "Add feature",
			expBody:  "",
		},
		{
			name:     "commit with body",
			message:  "Add feature\n\nThis adds a new feature",
			expTitle: "Add feature",
			expBody:  "This adds a new feature",
		},
		{
			name:     "commit with multiple body paragraphs",
			message:  "Fix bug\n\nFirst paragraph\n\nSecond paragraph",
			expTitle: "Fix bug",
			expBody:  "First paragraph\n\nSecond paragraph",
		},
		{
			name:     "commit with trailing newlines",
			message:  "Update docs\n\n\n",
			expTitle: "Update docs",
			expBody:  "",
		},
		{
			name:     "empty message",
			message:  "",
			expTitle: "",
			expBody:  "",
		},
		{
			name:     "multiline title",
			message:  "First line\nSecond line without blank\nThird line",
			expTitle: "First line",
			expBody:  "Second line without blank\nThird line",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed := ParseCommitMessage(tc.message)
			assert.Equal(t, tc.expTitle, parsed.Title)
			assert.Equal(t, tc.expBody, parsed.Body)
		})
	}
}

// TestGetFirstCommitMessage tests getting the first commit message
func TestGetFirstCommitMessage(t *testing.T) {
	t.Run("get first commit message", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create base commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("base"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("base commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Create branch
		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Checkout feature
		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: "refs/heads/feature",
		})
		require.NoError(t, err)

		// Add commits
		messages := []string{"First commit", "Second commit", "Third commit"}
		for _, msg := range messages {
			err = os.WriteFile(testFile, []byte(msg), 0644)
			require.NoError(t, err)

			_, err = worktree.Add("test.txt")
			require.NoError(t, err)

			_, err = worktree.Commit(msg, &git.CommitOptions{
				Author: &object.Signature{
					Name:  "Test User",
					Email: "test@example.com",
					When:  time.Now(),
				},
			})
			require.NoError(t, err)
		}

		// Get first commit message
		firstMsg, err := repo.GetFirstCommitMessage("master", "feature")
		require.NoError(t, err)
		assert.Equal(t, "First commit", firstMsg)
	})

	t.Run("no commits between branches", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create branch at same point
		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Should return empty string
		msg, err := repo.GetFirstCommitMessage("master", "feature")
		require.NoError(t, err)
		assert.Empty(t, msg)
	})
}

// TestGetAllCommitMessages tests getting all commit messages
func TestGetAllCommitMessages(t *testing.T) {
	t.Run("get all commit messages", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create base commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("base"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("base commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Create branch
		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Checkout feature
		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: "refs/heads/feature",
		})
		require.NoError(t, err)

		// Add commits
		expectedMessages := []string{"First commit", "Second commit", "Third commit"}
		for _, msg := range expectedMessages {
			err = os.WriteFile(testFile, []byte(msg), 0644)
			require.NoError(t, err)

			_, err = worktree.Add("test.txt")
			require.NoError(t, err)

			_, err = worktree.Commit(msg, &git.CommitOptions{
				Author: &object.Signature{
					Name:  "Test User",
					Email: "test@example.com",
					When:  time.Now(),
				},
			})
			require.NoError(t, err)
		}

		// Get all commit messages
		messages, err := repo.GetAllCommitMessages("master", "feature")
		require.NoError(t, err)
		assert.Equal(t, 3, len(messages))

		// Messages are returned in reverse chronological order (newest first)
		// So we need to reverse our expected list
		for i := 0; i < len(expectedMessages); i++ {
			assert.Equal(t, expectedMessages[len(expectedMessages)-1-i], messages[i])
		}
	})

	t.Run("no commits between branches", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create branch at same point
		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Should return empty list
		messages, err := repo.GetAllCommitMessages("master", "feature")
		require.NoError(t, err)
		assert.Empty(t, messages)
	})
}

// TestGetDiffBetween tests generating unified diffs between refs
func TestGetDiffBetween(t *testing.T) {
	t.Run("diff between branches with file changes", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("initial content\n"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create feature branch
		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Checkout feature
		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: "refs/heads/feature",
		})
		require.NoError(t, err)

		// Modify file
		err = os.WriteFile(testFile, []byte("initial content\nmodified line\n"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("modify file", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Get diff
		diff, err := repo.GetDiffBetween("master", "feature")
		require.NoError(t, err)
		assert.NotEmpty(t, diff)
		assert.Contains(t, diff, "test.txt")
		assert.Contains(t, diff, "+modified line")
	})

	t.Run("no diff between same refs", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Get diff between same ref
		diff, err := repo.GetDiffBetween("master", "master")
		require.NoError(t, err)
		assert.Empty(t, diff)
	})
}

// TestGetFilesChanged tests listing changed files between refs
func TestGetFilesChanged(t *testing.T) {
	t.Run("new file", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		initialFile := filepath.Join(tmpDir, "initial.txt")
		err = os.WriteFile(initialFile, []byte("initial"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("initial.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create feature branch
		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Checkout feature
		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: "refs/heads/feature",
		})
		require.NoError(t, err)

		// Add new file
		newFile := filepath.Join(tmpDir, "new.txt")
		err = os.WriteFile(newFile, []byte("new content"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("new.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("add new file", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Get files changed
		files, err := repo.GetFilesChanged("master", "feature")
		require.NoError(t, err)
		assert.Equal(t, 1, len(files))
		assert.Equal(t, "new.txt", files[0].Path)
		assert.True(t, files[0].IsNew)
		assert.False(t, files[0].IsDeleted)
		assert.False(t, files[0].IsRenamed)
		assert.False(t, files[0].IsBinary)
	})

	t.Run("modified file", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("line 1\nline 2\nline 3\n"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create feature branch
		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Checkout feature
		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: "refs/heads/feature",
		})
		require.NoError(t, err)

		// Modify file
		err = os.WriteFile(testFile, []byte("line 1\nline 2 modified\nline 3\nline 4\n"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("modify file", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Get files changed
		files, err := repo.GetFilesChanged("master", "feature")
		require.NoError(t, err)
		assert.Equal(t, 1, len(files))
		assert.Equal(t, "test.txt", files[0].Path)
		assert.False(t, files[0].IsNew)
		assert.False(t, files[0].IsDeleted)
		assert.False(t, files[0].IsRenamed)
		assert.Greater(t, files[0].Additions, 0)
		assert.Greater(t, files[0].Deletions, 0)
	})

	t.Run("deleted file", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit with two files
		testFile1 := filepath.Join(tmpDir, "file1.txt")
		testFile2 := filepath.Join(tmpDir, "file2.txt")
		err = os.WriteFile(testFile1, []byte("content 1"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(testFile2, []byte("content 2"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("file1.txt")
		require.NoError(t, err)
		_, err = worktree.Add("file2.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create feature branch
		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Checkout feature
		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: "refs/heads/feature",
		})
		require.NoError(t, err)

		// Delete file
		_, err = worktree.Remove("file2.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("delete file", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Get files changed
		files, err := repo.GetFilesChanged("master", "feature")
		require.NoError(t, err)
		assert.Equal(t, 1, len(files))
		assert.Equal(t, "file2.txt", files[0].Path)
		assert.False(t, files[0].IsNew)
		assert.True(t, files[0].IsDeleted)
		assert.False(t, files[0].IsRenamed)
	})
}

// TestGetDiffStats tests diff statistics calculation
func TestGetDiffStats(t *testing.T) {
	t.Run("calculate stats for multiple file changes", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit with two files
		file1 := filepath.Join(tmpDir, "file1.txt")
		file2 := filepath.Join(tmpDir, "file2.txt")
		err = os.WriteFile(file1, []byte("line 1\nline 2\n"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(file2, []byte("content\n"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("file1.txt")
		require.NoError(t, err)
		_, err = worktree.Add("file2.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create feature branch
		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Checkout feature
		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: "refs/heads/feature",
		})
		require.NoError(t, err)

		// Modify files
		err = os.WriteFile(file1, []byte("line 1 modified\nline 2\nline 3\n"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(file2, []byte("content updated\n"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("file1.txt")
		require.NoError(t, err)
		_, err = worktree.Add("file2.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("modify files", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Get diff stats
		stats, err := repo.GetDiffStats("master", "feature")
		require.NoError(t, err)
		assert.Equal(t, 2, stats.FilesChanged)
		assert.Greater(t, stats.Additions, 0)
		assert.Greater(t, stats.Deletions, 0)
	})

	t.Run("no stats for same refs", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Get stats for same ref
		stats, err := repo.GetDiffStats("master", "master")
		require.NoError(t, err)
		assert.Equal(t, 0, stats.FilesChanged)
		assert.Equal(t, 0, stats.Additions)
		assert.Equal(t, 0, stats.Deletions)
	})
}

// TestGetWorkingDiff tests getting unstaged changes diff
func TestGetWorkingDiff(t *testing.T) {
	t.Run("working diff with modifications", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("initial content\n"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Modify file without staging
		err = os.WriteFile(testFile, []byte("initial content\nmodified\n"), 0644)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Get working diff
		diff, err := repo.GetWorkingDiff()
		require.NoError(t, err)
		assert.NotEmpty(t, diff)
		assert.Contains(t, diff, "test.txt")
		assert.Contains(t, diff, "+modified")
	})

	t.Run("no working diff when clean", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Get working diff (should be empty)
		diff, err := repo.GetWorkingDiff()
		require.NoError(t, err)
		assert.Empty(t, diff)
	})
}

// TestGetStagedDiff tests getting staged changes diff
func TestGetStagedDiff(t *testing.T) {
	t.Run("staged diff with modifications", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("initial content\n"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Modify and stage file
		err = os.WriteFile(testFile, []byte("initial content\nstaged modification\n"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Get staged diff
		diff, err := repo.GetStagedDiff()
		require.NoError(t, err)
		assert.NotEmpty(t, diff)
		assert.Contains(t, diff, "test.txt")
		assert.Contains(t, diff, "+staged modification")
	})

	t.Run("no staged diff when nothing staged", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Get staged diff (should be empty)
		diff, err := repo.GetStagedDiff()
		require.NoError(t, err)
		assert.Empty(t, diff)
	})
}

// TestPush tests pushing commits to remote
func TestPush(t *testing.T) {
	t.Run("empty branch name returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Push with empty branch name should fail
		err = repo.Push(context.Background(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "branch name cannot be empty")
	})

	t.Run("context cancellation", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create a cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Push with cancelled context should fail
		err = repo.Push(ctx, "master")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cancelled")
	})

	t.Run("push with timeout", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create a context with very short timeout (1 nanosecond)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Wait for context to timeout
		<-ctx.Done()

		// Push with timed-out context should fail
		err = repo.Push(ctx, "master")
		assert.Error(t, err)
		// Error message could be either "timed out" or "cancelled" depending on timing
		assert.True(t,
			strings.Contains(err.Error(), "timed out") || strings.Contains(err.Error(), "cancelled"),
			"expected error to contain 'timed out' or 'cancelled', got: %v", err)
	})
}

// TestHasUnpushedCommits tests checking for unpushed commits
func TestHasUnpushedCommits(t *testing.T) {
	t.Run("empty branch name returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Check with empty branch name should fail
		_, err = repo.HasUnpushedCommits("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "branch name cannot be empty")
	})

	t.Run("remote branch doesn't exist - all commits unpushed", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		branch, err := repo.GetCurrentBranch()
		require.NoError(t, err)

		// Check for unpushed commits - should return true since remote doesn't exist
		hasUnpushed, err := repo.HasUnpushedCommits(branch)
		require.NoError(t, err)
		assert.True(t, hasUnpushed, "should have unpushed commits when remote branch doesn't exist")
	})
}

// TestIsNonFastForwardError tests the isNonFastForwardError helper function
func TestIsNonFastForwardError(t *testing.T) {
	testCases := []struct {
		name     string
		output   string
		expected bool
	}{
		{
			name: "non-fast-forward error",
			output: `To github.com:0xBAD-dev/gh-arc-test.git
 ! [rejected]        test_diff -> test_diff (non-fast-forward)
error: failed to push some refs to 'github.com:0xBAD-dev/gh-arc-test.git'
hint: Updates were rejected because the tip of your current branch is behind
hint: its remote counterpart.`,
			expected: true,
		},
		{
			name:     "rejected but not non-fast-forward",
			output:   "! [rejected]        test_diff -> test_diff (fetch first)",
			expected: false,
		},
		{
			name:     "non-fast-forward but not rejected",
			output:   "non-fast-forward update to test_diff",
			expected: false,
		},
		{
			name:     "success output",
			output:   "To github.com:test/repo.git\n   abc1234..def5678  main -> main",
			expected: false,
		},
		{
			name:     "empty output",
			output:   "",
			expected: false,
		},
		{
			name:     "other error",
			output:   "fatal: unable to access 'https://github.com/test/repo.git/': Could not resolve host",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isNonFastForwardError(tc.output)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestCountCommitsAhead tests counting commits ahead of base branch
func TestCountCommitsAhead(t *testing.T) {
	t.Run("feature branch with commits ahead", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit on master
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("initial"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Create feature branch
		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Checkout feature branch
		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: "refs/heads/feature",
		})
		require.NoError(t, err)

		// Add 2 commits to feature branch
		for i := 1; i <= 2; i++ {
			err = os.WriteFile(testFile, []byte("feature "+string(rune(i))), 0644)
			require.NoError(t, err)

			_, err = worktree.Add("test.txt")
			require.NoError(t, err)

			_, err = worktree.Commit("feature commit "+string(rune(i)), &git.CommitOptions{
				Author: &object.Signature{
					Name:  "Test User",
					Email: "test@example.com",
					When:  time.Now().Add(time.Duration(i) * time.Minute),
				},
			})
			require.NoError(t, err)
		}

		// Count commits ahead
		count, err := repo.CountCommitsAhead("feature", "master")
		require.NoError(t, err)
		assert.Equal(t, 2, count, "feature branch should be 2 commits ahead of master")
	})

	t.Run("non-existent base branch returns 0", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		branch, err := repo.GetCurrentBranch()
		require.NoError(t, err)

		// Count commits ahead of non-existent branch
		count, err := repo.CountCommitsAhead(branch, "origin/main")
		require.NoError(t, err)
		assert.Equal(t, 0, count, "should return 0 when base branch doesn't exist")
	})

	t.Run("equal branches return 0", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create branch at same point
		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Count commits - should be 0 since they're equal
		count, err := repo.CountCommitsAhead("feature", "master")
		require.NoError(t, err)
		assert.Equal(t, 0, count, "equal branches should return 0 commits ahead")
	})

	t.Run("empty branch name returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Empty branchName
		_, err = repo.CountCommitsAhead("", "master")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "branch name cannot be empty")

		// Empty baseBranch
		_, err = repo.CountCommitsAhead("master", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "base branch name cannot be empty")
	})
}

// TestBranchExists tests checking if a branch exists
func TestBranchExists(t *testing.T) {
	t.Run("local branch exists returns true", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create a feature branch
		err = repo.CreateBranch("feature", "master")
		require.NoError(t, err)

		// Check that both master and feature exist
		exists, err := repo.BranchExists("master")
		require.NoError(t, err)
		assert.True(t, exists, "master branch should exist")

		exists, err = repo.BranchExists("feature")
		require.NoError(t, err)
		assert.True(t, exists, "feature branch should exist")
	})

	t.Run("local branch doesn't exist returns false", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Check for non-existent branch
		exists, err := repo.BranchExists("nonexistent")
		require.NoError(t, err)
		assert.False(t, exists, "nonexistent branch should not exist")
	})

	t.Run("remote branch exists returns true", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Create a bare repository to act as remote
		remoteDir := t.TempDir()
		_, err = git.PlainInit(remoteDir, true)
		require.NoError(t, err)

		// Add remote
		_, err = gitRepo.CreateRemote(&gitconfig.RemoteConfig{
			Name: "origin",
			URLs: []string{remoteDir},
		})
		require.NoError(t, err)

		// Push to remote
		err = gitRepo.Push(&git.PushOptions{
			RemoteName: "origin",
			RefSpecs: []gitconfig.RefSpec{
				gitconfig.RefSpec("refs/heads/master:refs/heads/master"),
			},
		})
		require.NoError(t, err)

		// Fetch to update remote tracking branches
		err = gitRepo.Fetch(&git.FetchOptions{
			RemoteName: "origin",
			RefSpecs: []gitconfig.RefSpec{
				gitconfig.RefSpec("+refs/heads/*:refs/remotes/origin/*"),
			},
		})
		// Fetch can return "already up-to-date" which is not an error
		if err != nil && err != git.NoErrAlreadyUpToDate {
			require.NoError(t, err)
		}

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Check if remote branch exists
		exists, err := repo.BranchExists("origin/master")
		require.NoError(t, err)
		assert.True(t, exists, "origin/master should exist after push and fetch")
	})

	t.Run("remote branch doesn't exist returns false", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Check for non-existent remote branch (no remote configured)
		exists, err := repo.BranchExists("origin/main")
		require.NoError(t, err)
		assert.False(t, exists, "origin/main should not exist when no remote is configured")
	})

	t.Run("empty branch name returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Empty branch name should return error
		_, err = repo.BranchExists("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "branch name cannot be empty")
	})
}

// TestPushBranch tests pushing a branch to a specific remote branch
func TestPushBranch(t *testing.T) {
	t.Run("empty local ref returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Empty local ref should fail
		err = repo.PushBranch(context.Background(), "", "remote-branch")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "local ref cannot be empty")
	})

	t.Run("empty remote branch returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Empty remote branch should fail
		err = repo.PushBranch(context.Background(), "local-branch", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote branch name cannot be empty")
	})

	t.Run("context cancellation", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Create a cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Push with cancelled context should fail
		err = repo.PushBranch(ctx, "master", "remote-master")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cancelled")
	})
}

// TestCheckoutTrackingBranch tests checking out a tracking branch
func TestCheckoutTrackingBranch(t *testing.T) {
	t.Run("empty branch name returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Empty branch name should fail
		err = repo.CheckoutTrackingBranch("", "origin/main")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "branch name cannot be empty")
	})

	t.Run("empty remote branch returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Empty remote branch should fail
		err = repo.CheckoutTrackingBranch("local-branch", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote branch name cannot be empty")
	})

	t.Run("checkout tracking branch successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Create a bare repository to act as remote
		remoteDir := t.TempDir()
		_, err = git.PlainInit(remoteDir, true)
		require.NoError(t, err)

		// Add remote
		_, err = gitRepo.CreateRemote(&gitconfig.RemoteConfig{
			Name: "origin",
			URLs: []string{remoteDir},
		})
		require.NoError(t, err)

		// Push to remote
		err = gitRepo.Push(&git.PushOptions{
			RemoteName: "origin",
			RefSpecs: []gitconfig.RefSpec{
				gitconfig.RefSpec("refs/heads/master:refs/heads/master"),
			},
		})
		require.NoError(t, err)

		// Fetch to update remote tracking branches
		err = gitRepo.Fetch(&git.FetchOptions{
			RemoteName: "origin",
			RefSpecs: []gitconfig.RefSpec{
				gitconfig.RefSpec("+refs/heads/*:refs/remotes/origin/*"),
			},
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			require.NoError(t, err)
		}

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Checkout tracking branch
		err = repo.CheckoutTrackingBranch("local-master", "origin/master")
		require.NoError(t, err)

		// Verify we're on the new branch
		currentBranch, err := repo.GetCurrentBranch()
		require.NoError(t, err)
		assert.Equal(t, "local-master", currentBranch)
	})
}

// TestGetRemoteRefAge tests getting the age of a remote ref
func TestGetRemoteRefAge(t *testing.T) {
	t.Run("empty ref name returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Empty ref name should fail
		_, err = repo.GetRemoteRefAge("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote ref name cannot be empty")
	})

	t.Run("non-existent remote ref returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Non-existent remote ref should fail
		_, err = repo.GetRemoteRefAge("origin/main")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote ref not found")
	})

	t.Run("fresh remote ref returns small age", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitRepo, err := git.PlainInit(tmpDir, false)
		require.NoError(t, err)

		// Enable reflog for all refs (including remote tracking branches)
		cfg, err := gitRepo.Config()
		require.NoError(t, err)
		cfg.Raw.AddOption("core", "", "logAllRefUpdates", "true")
		err = gitRepo.SetConfig(cfg)
		require.NoError(t, err)

		worktree, err := gitRepo.Worktree()
		require.NoError(t, err)

		// Create initial commit
		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		_, err = worktree.Add("test.txt")
		require.NoError(t, err)

		_, err = worktree.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		// Create a bare repository to act as remote
		remoteDir := t.TempDir()
		_, err = git.PlainInit(remoteDir, true)
		require.NoError(t, err)

		// Add remote using git CLI (to ensure proper reflog setup)
		cmd := exec.Command("git", "remote", "add", "origin", remoteDir)
		cmd.Dir = tmpDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to add remote: %v\nOutput: %s", err, output)
		}

		// Push to remote using git CLI (creates reflog entries)
		cmd = exec.Command("git", "push", "origin", "master")
		cmd.Dir = tmpDir
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to push: %v\nOutput: %s", err, output)
		}

		// Fetch from remote using git CLI (creates reflog entries for remote tracking branches)
		cmd = exec.Command("git", "fetch", "origin")
		cmd.Dir = tmpDir
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to fetch: %v\nOutput: %s", err, output)
		}

		repo, err := OpenRepository(tmpDir)
		require.NoError(t, err)

		// Get age of remote ref
		age, err := repo.GetRemoteRefAge("origin/master")
		require.NoError(t, err)

		// Age should be very small (within 10 seconds)
		assert.Less(t, age, 10*time.Second, "fresh remote ref should have age < 10 seconds")
		assert.GreaterOrEqual(t, age, time.Duration(0), "age should be non-negative")
	})
}
