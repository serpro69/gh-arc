package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPackageCompiles is a basic test to ensure the package compiles correctly
func TestPackageCompiles(t *testing.T) {
	// This test primarily ensures imports are correct and package compiles
	assert.NotNil(t, ErrNotARepository)
	assert.NotNil(t, ErrDetachedHead)
	assert.NotNil(t, ErrInvalidBranch)
}

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
