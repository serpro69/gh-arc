package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
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
