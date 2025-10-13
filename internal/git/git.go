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

// WorkingDirectoryStatus represents the status of the working directory.
type WorkingDirectoryStatus struct {
	IsClean        bool     // True if no changes in working directory
	StagedFiles    []string // Files in staging area
	UnstagedFiles  []string // Modified files not staged
	UntrackedFiles []string // Files not tracked by git
}

// RepositoryState represents the complete state of a repository.
type RepositoryState struct {
	IsValid       bool                   // True if this is a valid repository
	IsDetached    bool                   // True if HEAD is detached
	CurrentBranch string                 // Current branch name (empty if detached)
	HeadCommit    string                 // SHA of HEAD commit
	WorkingDir    WorkingDirectoryStatus // Working directory status
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

// IsDetachedHead checks if the repository is in a detached HEAD state.
func (r *Repository) IsDetachedHead() (bool, error) {
	head, err := r.repo.Head()
	if err != nil {
		return false, fmt.Errorf("failed to get HEAD: %w", err)
	}

	// If HEAD reference name is not a branch reference, it's detached
	return !head.Name().IsBranch(), nil
}

// GetWorkingDirectoryStatus returns the status of the working directory.
func (r *Repository) GetWorkingDirectoryStatus() (*WorkingDirectoryStatus, error) {
	worktree, err := r.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	wdStatus := &WorkingDirectoryStatus{
		IsClean:        status.IsClean(),
		StagedFiles:    make([]string, 0),
		UnstagedFiles:  make([]string, 0),
		UntrackedFiles: make([]string, 0),
	}

	// Iterate through file statuses
	for filePath, fileStatus := range status {
		// Check staging area status
		if fileStatus.Staging != git.Unmodified && fileStatus.Staging != git.Untracked {
			wdStatus.StagedFiles = append(wdStatus.StagedFiles, filePath)
		}

		// Check worktree status
		if fileStatus.Worktree == git.Modified || fileStatus.Worktree == git.Deleted {
			wdStatus.UnstagedFiles = append(wdStatus.UnstagedFiles, filePath)
		}

		// Check untracked files
		if fileStatus.Staging == git.Untracked {
			wdStatus.UntrackedFiles = append(wdStatus.UntrackedFiles, filePath)
		}
	}

	return wdStatus, nil
}

// GetRepositoryState returns the complete state of the repository,
// combining all state checks into a single status struct.
func (r *Repository) GetRepositoryState() (*RepositoryState, error) {
	state := &RepositoryState{
		IsValid: true,
	}

	// Check if HEAD is detached
	isDetached, err := r.IsDetachedHead()
	if err != nil {
		return nil, fmt.Errorf("failed to check detached HEAD: %w", err)
	}
	state.IsDetached = isDetached

	// Get HEAD reference
	head, err := r.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Get current branch name (if not detached)
	if !isDetached {
		state.CurrentBranch = head.Name().Short()
	}

	// Get HEAD commit SHA
	state.HeadCommit = head.Hash().String()

	// Get working directory status
	wdStatus, err := r.GetWorkingDirectoryStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory status: %w", err)
	}
	state.WorkingDir = *wdStatus

	return state, nil
}
