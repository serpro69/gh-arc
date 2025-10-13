// Package git provides Git repository operations for gh-arc.
// It wraps go-git library and provides fallback to git CLI for complex operations.
package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

var (
	// ErrNotARepository is returned when the path is not a git repository
	ErrNotARepository = errors.New("not a git repository")

	// ErrDetachedHead is returned when repository is in detached HEAD state
	ErrDetachedHead = errors.New("repository is in detached HEAD state")

	// ErrInvalidBranch is returned when branch name is invalid or doesn't exist
	ErrInvalidBranch = errors.New("invalid or non-existent branch")
)

// Repository represents a Git repository and provides methods for Git operations.
type Repository struct {
	repo *git.Repository
	path string
}

// OpenRepository opens a Git repository at the given path.
// If path is empty, it attempts to find the repository in the current directory.
func OpenRepository(path string) (*Repository, error) {
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Try to open the repository
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, ErrNotARepository
		}
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	return &Repository{
		repo: repo,
		path: path,
	}, nil
}

// FindRepositoryRoot traverses up directories looking for .git folder
// and returns the root path of the repository.
func FindRepositoryRoot(startPath string) (string, error) {
	if startPath == "" {
		var err error
		startPath, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	currentPath := absPath
	for {
		gitDir := filepath.Join(currentPath, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return currentPath, nil
		}

		// Move up one directory
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// Reached the root of the filesystem
			return "", ErrNotARepository
		}
		currentPath = parentPath
	}
}

// IsValidRepository checks if the given path is a valid Git repository.
func IsValidRepository(path string) bool {
	_, err := OpenRepository(path)
	return err == nil
}

// Path returns the path to the repository.
func (r *Repository) Path() string {
	return r.path
}

// Repo returns the underlying go-git Repository object.
// This is useful for advanced operations not covered by this package.
func (r *Repository) Repo() *git.Repository {
	return r.repo
}
