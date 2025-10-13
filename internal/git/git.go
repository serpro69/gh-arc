// Package git provides Git repository operations for gh-arc.
// It wraps go-git library and provides fallback to git CLI for complex operations.
package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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

// GetCurrentBranch returns the current branch name.
// If the repository is in a detached HEAD state, it returns the HEAD commit SHA.
func (r *Repository) GetCurrentBranch() (string, error) {
	head, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Check if HEAD is detached
	if !head.Name().IsBranch() {
		// Return the commit SHA for detached HEAD
		return head.Hash().String(), nil
	}

	// Return the branch name
	return head.Name().Short(), nil
}

// GetDefaultBranch attempts to detect the default branch (main, master, or custom).
// It checks remote HEAD first, then falls back to common branch names.
func (r *Repository) GetDefaultBranch() (string, error) {
	// First, try to get the default branch from remote HEAD
	remotes, err := r.repo.Remotes()
	if err == nil && len(remotes) > 0 {
		// Try origin first
		for _, remote := range remotes {
			if remote.Config().Name == "origin" {
				refs, err := remote.List(&git.ListOptions{})
				if err == nil {
					for _, ref := range refs {
						if ref.Name() == plumbing.HEAD {
							// Extract branch name from symbolic ref
							if ref.Type() == plumbing.SymbolicReference {
								target := ref.Target()
								if target.IsBranch() {
									return target.Short(), nil
								}
							}
						}
					}
				}
			}
		}
	}

	// Fallback: check for common default branch names
	branches, err := r.repo.Branches()
	if err != nil {
		return "", fmt.Errorf("failed to list branches: %w", err)
	}

	commonDefaults := []string{"main", "master", "trunk", "development"}
	existingBranches := make(map[string]bool)

	err = branches.ForEach(func(ref *plumbing.Reference) error {
		existingBranches[ref.Name().Short()] = true
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to iterate branches: %w", err)
	}

	// Check common default branch names in order
	for _, defaultName := range commonDefaults {
		if existingBranches[defaultName] {
			return defaultName, nil
		}
	}

	// If no common default found, return the first branch
	var firstBranch string
	branches, _ = r.repo.Branches()
	_ = branches.ForEach(func(ref *plumbing.Reference) error {
		if firstBranch == "" {
			firstBranch = ref.Name().Short()
		}
		return nil
	})

	if firstBranch != "" {
		return firstBranch, nil
	}

	return "", fmt.Errorf("no branches found in repository")
}

// CreateBranch creates a new branch from the specified base branch.
// If baseBranch is empty, it creates from the current HEAD.
func (r *Repository) CreateBranch(name string, baseBranch string) error {
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Get the base commit
	var baseHash plumbing.Hash
	if baseBranch == "" {
		// Use current HEAD
		head, err := r.repo.Head()
		if err != nil {
			return fmt.Errorf("failed to get HEAD: %w", err)
		}
		baseHash = head.Hash()
	} else {
		// Resolve the base branch reference
		ref, err := r.repo.Reference(plumbing.NewBranchReferenceName(baseBranch), true)
		if err != nil {
			return fmt.Errorf("failed to resolve base branch %s: %w", baseBranch, err)
		}
		baseHash = ref.Hash()
	}

	// Create the new branch reference
	refName := plumbing.NewBranchReferenceName(name)
	ref := plumbing.NewHashReference(refName, baseHash)
	err := r.repo.Storer.SetReference(ref)
	if err != nil {
		return fmt.Errorf("failed to create branch %s: %w", name, err)
	}

	return nil
}

// BranchInfo represents information about a branch.
type BranchInfo struct {
	Name     string // Branch name
	IsRemote bool   // True if this is a remote branch
	Hash     string // Commit SHA
}

// ListBranches returns a list of all branches (local and optionally remote).
func (r *Repository) ListBranches(includeRemote bool) ([]BranchInfo, error) {
	var branches []BranchInfo

	// Get local branches
	branchIter, err := r.repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}

	err = branchIter.ForEach(func(ref *plumbing.Reference) error {
		branches = append(branches, BranchInfo{
			Name:     ref.Name().Short(),
			IsRemote: false,
			Hash:     ref.Hash().String(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate branches: %w", err)
	}

	// Get remote branches if requested
	if includeRemote {
		refs, err := r.repo.References()
		if err != nil {
			return nil, fmt.Errorf("failed to get references: %w", err)
		}

		err = refs.ForEach(func(ref *plumbing.Reference) error {
			if ref.Name().IsRemote() {
				branches = append(branches, BranchInfo{
					Name:     ref.Name().Short(),
					IsRemote: true,
					Hash:     ref.Hash().String(),
				})
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to iterate remote branches: %w", err)
		}
	}

	return branches, nil
}

// GetGitConfig reads a git configuration value.
// It uses go-git's config first, then falls back to git CLI if not found.
func (r *Repository) GetGitConfig(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("config key cannot be empty")
	}

	// Try go-git config first
	cfg, err := r.repo.Config()
	if err == nil {
		// Handle common config keys
		switch key {
		case "user.name":
			if cfg.User.Name != "" {
				return cfg.User.Name, nil
			}
		case "user.email":
			if cfg.User.Email != "" {
				return cfg.User.Email, nil
			}
		}

		// For other keys, try raw config
		if cfg.Raw != nil {
			section, subsection, option := parseConfigKey(key)
			if section != "" && option != "" {
				if subsection != "" {
					if sec := cfg.Raw.Section(section).Subsection(subsection); sec != nil {
						if val := sec.Option(option); val != "" {
							return val, nil
						}
					}
				} else {
					if val := cfg.Raw.Section(section).Option(option); val != "" {
						return val, nil
					}
				}
			}
		}
	}

	// Fallback to git CLI
	return r.getConfigViaCLI(key)
}

// getConfigViaCLI uses git CLI to get config value.
func (r *Repository) getConfigViaCLI(key string) (string, error) {
	cmd := exec.Command("git", "config", "--get", key)
	cmd.Dir = r.path

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means key not found
			if exitErr.ExitCode() == 1 {
				return "", fmt.Errorf("config key %s not found", key)
			}
		}
		return "", fmt.Errorf("failed to get config via CLI: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// parseConfigKey parses a git config key like "user.name" or "remote.origin.url"
// into section, subsection, and option components.
func parseConfigKey(key string) (section, subsection, option string) {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return "", "", ""
	}

	section = parts[0]
	option = parts[len(parts)-1]

	if len(parts) == 3 {
		subsection = parts[1]
	}

	return section, subsection, option
}

// CommitInfo represents information about a commit.
type CommitInfo struct {
	SHA     string    // Commit SHA
	Author  string    // Author name
	Email   string    // Author email
	Date    time.Time // Commit date
	Message string    // Full commit message
}

// CommitMessage represents a parsed commit message.
type CommitMessage struct {
	Title string // First line of commit message
	Body  string // Rest of commit message (after title)
}

// GetCommitRange returns commits between two branches.
// It returns commits that are in headBranch but not in baseBranch.
func (r *Repository) GetCommitRange(baseBranch, headBranch string) ([]CommitInfo, error) {
	// Resolve base branch reference
	baseRef, err := r.repo.Reference(plumbing.NewBranchReferenceName(baseBranch), true)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base branch %s: %w", baseBranch, err)
	}

	// Resolve head branch reference
	headRef, err := r.repo.Reference(plumbing.NewBranchReferenceName(headBranch), true)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve head branch %s: %w", headBranch, err)
	}

	// Get commits exclusive to head branch
	return r.getCommitsExclusiveTo(baseRef.Hash(), headRef.Hash())
}

// GetCommitsBetween returns commits between two branch/commit references.
// This is similar to GetCommitRange but accepts any ref (branch, tag, commit SHA).
func (r *Repository) GetCommitsBetween(base, head string) ([]CommitInfo, error) {
	// Resolve base reference
	baseHash, err := r.repo.ResolveRevision(plumbing.Revision(base))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base ref %s: %w", base, err)
	}

	// Resolve head reference
	headHash, err := r.repo.ResolveRevision(plumbing.Revision(head))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve head ref %s: %w", head, err)
	}

	return r.getCommitsExclusiveTo(*baseHash, *headHash)
}

// getCommitsExclusiveTo is a helper that returns commits reachable from head but not from base.
func (r *Repository) getCommitsExclusiveTo(base, head plumbing.Hash) ([]CommitInfo, error) {
	// Get commit iterator from head
	logIter, err := r.repo.Log(&git.LogOptions{
		From: head,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get log: %w", err)
	}
	defer logIter.Close()

	// Build set of commits reachable from base
	baseCommits := make(map[plumbing.Hash]bool)
	if base != plumbing.ZeroHash {
		baseIter, err := r.repo.Log(&git.LogOptions{
			From: base,
		})
		if err == nil {
			_ = baseIter.ForEach(func(c *object.Commit) error {
				baseCommits[c.Hash] = true
				return nil
			})
			baseIter.Close()
		}
	}

	// Collect commits exclusive to head
	var commits []CommitInfo
	err = logIter.ForEach(func(c *object.Commit) error {
		// Stop if we've reached a commit that's in base
		if baseCommits[c.Hash] {
			return nil
		}

		commits = append(commits, CommitInfo{
			SHA:     c.Hash.String(),
			Author:  c.Author.Name,
			Email:   c.Author.Email,
			Date:    c.Author.When,
			Message: c.Message,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return commits, nil
}

// ParseCommitMessage parses a commit message into title and body.
func ParseCommitMessage(message string) CommitMessage {
	lines := strings.Split(message, "\n")
	if len(lines) == 0 {
		return CommitMessage{}
	}

	title := strings.TrimSpace(lines[0])

	// Find the start of the body (skip empty lines after title)
	bodyStart := 1
	for bodyStart < len(lines) && strings.TrimSpace(lines[bodyStart]) == "" {
		bodyStart++
	}

	var body string
	if bodyStart < len(lines) {
		body = strings.TrimSpace(strings.Join(lines[bodyStart:], "\n"))
	}

	return CommitMessage{
		Title: title,
		Body:  body,
	}
}

// GetFirstCommitMessage returns the message of the first commit in the range.
// Useful for generating PR descriptions from a single commit.
// Returns empty string if there are no commits between the branches.
func (r *Repository) GetFirstCommitMessage(baseBranch, headBranch string) (string, error) {
	commits, err := r.GetCommitRange(baseBranch, headBranch)
	if err != nil {
		return "", err
	}

	if len(commits) == 0 {
		return "", nil
	}

	// Return the last commit (commits are in reverse chronological order)
	return commits[len(commits)-1].Message, nil
}

// GetAllCommitMessages returns all commit messages in the range.
// Useful for generating PR descriptions from multiple commits.
// Returns empty slice if there are no commits between the branches.
func (r *Repository) GetAllCommitMessages(baseBranch, headBranch string) ([]string, error) {
	commits, err := r.GetCommitRange(baseBranch, headBranch)
	if err != nil {
		return nil, err
	}

	if len(commits) == 0 {
		return []string{}, nil
	}

	messages := make([]string, len(commits))
	for i, commit := range commits {
		messages[i] = commit.Message
	}

	return messages, nil
}
