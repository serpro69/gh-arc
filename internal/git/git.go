// Package git provides Git repository operations for gh-arc.
// It wraps go-git library and provides fallback to git CLI for complex operations.
package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/serpro69/gh-arc/internal/logger"
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
//
// Repository is not safe for concurrent use by multiple goroutines without
// external synchronization. If you need to access a repository from multiple
// goroutines, you must coordinate access using a mutex or other synchronization
// mechanism.
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
	var firstBranch string

	// Single iteration: build map and capture first branch
	err = branches.ForEach(func(ref *plumbing.Reference) error {
		branchName := ref.Name().Short()
		existingBranches[branchName] = true
		if firstBranch == "" {
			firstBranch = branchName
		}
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

// parseConfigKey parses a git config key like "user.name", "remote.origin.url",
// or "url.https://example.com/.insteadOf" into section, subsection, and option components.
// Handles keys with 4+ parts where subsections contain dots.
func parseConfigKey(key string) (section, subsection, option string) {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return "", "", ""
	}

	section = parts[0]
	option = parts[len(parts)-1]

	// Join all middle parts as subsection (handles subsections with dots)
	if len(parts) >= 3 {
		subsection = strings.Join(parts[1:len(parts)-1], ".")
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

	// Build set of commits reachable from base for O(1) membership checking.
	// NOTE: This loads all base commits into memory (each hash is 20 bytes + map overhead).
	// For repositories with very long histories (e.g., 100k+ commits), this could consume
	// significant memory (~2-4MB per 10k commits). This is generally acceptable for most
	// repositories and provides fast lookup. An alternative streaming approach would trade
	// memory efficiency for slower O(n*m) time complexity.
	baseCommits := make(map[plumbing.Hash]bool)
	if base != plumbing.ZeroHash {
		baseIter, err := r.repo.Log(&git.LogOptions{
			From: base,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get log for base: %w", err)
		}
		err = baseIter.ForEach(func(c *object.Commit) error {
			baseCommits[c.Hash] = true
			return nil
		})
		baseIter.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to iterate base commits: %w", err)
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

// DiffStats represents statistics about a diff.
type DiffStats struct {
	FilesChanged int // Number of files changed
	Additions    int // Number of lines added
	Deletions    int // Number of lines deleted
}

// FileChange represents a change to a file in a diff.
type FileChange struct {
	Path      string // File path
	OldPath   string // Old path (for renames)
	IsNew     bool   // True if file is newly created
	IsDeleted bool   // True if file is deleted
	IsRenamed bool   // True if file is renamed
	IsBinary  bool   // True if file is binary
	Additions int    // Lines added
	Deletions int    // Lines deleted
}

// GetDiffBetween generates a unified diff between two refs (branches, tags, or commits).
// Returns the diff as a string in unified diff format.
func (r *Repository) GetDiffBetween(base, head string) (string, error) {
	// Resolve base reference
	baseHash, err := r.repo.ResolveRevision(plumbing.Revision(base))
	if err != nil {
		return "", fmt.Errorf("failed to resolve base ref %s: %w", base, err)
	}

	// Resolve head reference
	headHash, err := r.repo.ResolveRevision(plumbing.Revision(head))
	if err != nil {
		return "", fmt.Errorf("failed to resolve head ref %s: %w", head, err)
	}

	// Get commit objects
	baseCommit, err := r.repo.CommitObject(*baseHash)
	if err != nil {
		return "", fmt.Errorf("failed to get base commit: %w", err)
	}

	headCommit, err := r.repo.CommitObject(*headHash)
	if err != nil {
		return "", fmt.Errorf("failed to get head commit: %w", err)
	}

	// Get trees
	baseTree, err := baseCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("failed to get base tree: %w", err)
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("failed to get head tree: %w", err)
	}

	// Generate patch
	patch, err := baseTree.Patch(headTree)
	if err != nil {
		return "", fmt.Errorf("failed to generate patch: %w", err)
	}

	return patch.String(), nil
}

// GetWorkingDiff returns the diff for unstaged changes in the working directory.
func (r *Repository) GetWorkingDiff() (string, error) {
	// Use git CLI for working diff as go-git doesn't handle this well
	return r.getDiffViaCLI("HEAD", "--")
}

// GetStagedDiff returns the diff for staged changes (in the index).
func (r *Repository) GetStagedDiff() (string, error) {
	// Use git CLI for staged diff as it's more reliable
	return r.getDiffViaCLI("--cached", "HEAD")
}

// GetChangedFiles returns a list of file paths changed between two refs.
// This is a simpler version of GetFilesChanged that only returns file paths.
func (r *Repository) GetChangedFiles(base, head string) ([]string, error) {
	changes, err := r.GetFilesChanged(base, head)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(changes))
	for _, change := range changes {
		paths = append(paths, change.Path)
	}

	return paths, nil
}

// GetFilesChanged returns a list of files changed between two refs.
func (r *Repository) GetFilesChanged(base, head string) ([]FileChange, error) {
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

	// Get commit objects
	baseCommit, err := r.repo.CommitObject(*baseHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get base commit: %w", err)
	}

	headCommit, err := r.repo.CommitObject(*headHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get head commit: %w", err)
	}

	// Get trees
	baseTree, err := baseCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get base tree: %w", err)
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get head tree: %w", err)
	}

	// Get changes
	changes, err := baseTree.Diff(headTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	// Convert to FileChange structs
	var fileChanges []FileChange
	for _, change := range changes {
		from, to, err := change.Files()
		if err != nil {
			continue
		}

		fc := FileChange{}

		// Determine change type
		if from == nil && to != nil {
			// New file
			fc.Path = to.Name
			fc.IsNew = true
			fc.IsBinary = isBinaryFile(to)
		} else if from != nil && to == nil {
			// Deleted file
			fc.Path = from.Name
			fc.IsDeleted = true
			fc.IsBinary = isBinaryFile(from)
		} else if from != nil && to != nil {
			// Modified or renamed file
			fc.Path = to.Name
			fc.OldPath = from.Name
			if from.Name != to.Name {
				fc.IsRenamed = true
			}
			fc.IsBinary = isBinaryFile(to) || isBinaryFile(from)
		}

		// Get line statistics (if not binary)
		if !fc.IsBinary {
			patch, err := change.Patch()
			if err == nil {
				stats := patch.Stats()
				for _, fileStat := range stats {
					fc.Additions += fileStat.Addition
					fc.Deletions += fileStat.Deletion
				}
			}
		}

		fileChanges = append(fileChanges, fc)
	}

	return fileChanges, nil
}

// GetDiffStats calculates statistics for changes between two refs.
func (r *Repository) GetDiffStats(base, head string) (*DiffStats, error) {
	files, err := r.GetFilesChanged(base, head)
	if err != nil {
		return nil, err
	}

	stats := &DiffStats{}
	for _, file := range files {
		stats.FilesChanged++
		stats.Additions += file.Additions
		stats.Deletions += file.Deletions
	}

	return stats, nil
}

// getDiffViaCLI uses git CLI to get diff output.
// This is used for working directory and staged diffs where go-git has limitations.
func (r *Repository) getDiffViaCLI(args ...string) (string, error) {
	cmdArgs := append([]string{"diff"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = r.path

	output, err := cmd.Output()
	if err != nil {
		// Empty diff is not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return string(output), nil
		}
		return "", fmt.Errorf("failed to get diff via CLI: %w", err)
	}

	return string(output), nil
}

// isBinaryFile checks if a file is binary based on its content.
// It streams only the first 8000 bytes to avoid loading large files into memory.
func isBinaryFile(file *object.File) bool {
	if file == nil {
		return false
	}

	// Use streaming reader to avoid loading entire file into memory
	reader, err := file.Reader()
	if err != nil {
		return false
	}
	defer reader.Close()

	// Read only first 8000 bytes (or less if file is smaller)
	buf := make([]byte, 8000)
	n, err := reader.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}

	// Check for null bytes (common indicator of binary files)
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}

	return false
}

// GetMergeBase finds the common ancestor (merge-base) between two refs.
// It uses git CLI since go-git doesn't have reliable merge-base support.
// Returns the SHA of the common ancestor commit.
func (r *Repository) GetMergeBase(ref1, ref2 string) (string, error) {
	if ref1 == "" || ref2 == "" {
		return "", fmt.Errorf("both refs must be non-empty")
	}

	// Use git CLI to find merge-base
	cmd := exec.Command("git", "merge-base", ref1, ref2)
	cmd.Dir = r.path

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means no common ancestor
			if exitErr.ExitCode() == 1 {
				return "", fmt.Errorf("no common ancestor between %s and %s", ref1, ref2)
			}
			// Other exit codes indicate errors
			return "", fmt.Errorf("failed to find merge-base: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to execute git merge-base: %w", err)
	}

	// Parse and return the SHA
	sha := strings.TrimSpace(string(output))
	if sha == "" {
		return "", fmt.Errorf("git merge-base returned empty result")
	}

	return sha, nil
}

// HasUnpushedCommits checks if the local branch has commits that haven't been pushed to remote.
// Returns true if there are unpushed commits, false otherwise.
// If the remote branch doesn't exist yet, returns true (all local commits are "unpushed").
func (r *Repository) HasUnpushedCommits(branchName string) (bool, error) {
	if branchName == "" {
		return false, fmt.Errorf("branch name cannot be empty")
	}

	// Check if remote branch exists
	remoteBranch := fmt.Sprintf("origin/%s", branchName)
	cmd := exec.Command("git", "rev-parse", "--verify", remoteBranch)
	cmd.Dir = r.path
	if err := cmd.Run(); err != nil {
		// Remote branch doesn't exist, so all local commits are unpushed
		return true, nil
	}

	// Count commits that are in local branch but not in remote
	cmd = exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", remoteBranch, branchName))
	cmd.Dir = r.path

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to count unpushed commits: %w", err)
	}

	count := strings.TrimSpace(string(output))
	return count != "0", nil
}

// Push pushes commits from the specified branch to its remote tracking branch.
// If the branch has no remote tracking branch, it pushes to origin with the same name.
// Uses context for cancellation support.
//
// If a regular push fails due to non-fast-forward (e.g., after rebasing), it automatically
// retries with --force-with-lease to safely force push the changes.
func (r *Repository) Push(ctx context.Context, branchName string) error {
	if branchName == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Use git CLI for push operations as go-git's push has authentication complexities
	// and doesn't integrate well with gh CLI's existing authentication
	logger.Debug().
		Str("branch", branchName).
		Msg("Attempting push to remote")

	cmd := exec.CommandContext(ctx, "git", "push", "origin", branchName)
	cmd.Dir = r.path

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check for context errors first
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("push operation timed out")
		}
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("push operation cancelled")
		}

		// Check if this is a non-fast-forward error (e.g., after rebase)
		outputStr := string(output)
		if isNonFastForwardError(outputStr) {
			logger.Info().
				Str("branch", branchName).
				Msg("Non-fast-forward detected (likely after rebase), retrying with --force-with-lease")

			// Retry with --force-with-lease for safer force push
			cmd = exec.CommandContext(ctx, "git", "push", "--force-with-lease", "origin", branchName)
			cmd.Dir = r.path

			output, err = cmd.CombinedOutput()
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					return fmt.Errorf("push operation timed out")
				}
				if ctx.Err() == context.Canceled {
					return fmt.Errorf("push operation cancelled")
				}
				return fmt.Errorf("failed to force push branch %s: %w\nOutput: %s", branchName, err, string(output))
			}

			logger.Debug().
				Str("branch", branchName).
				Msg("Successfully force-pushed with --force-with-lease")
			return nil
		}

		// Not a non-fast-forward error, return original error
		return fmt.Errorf("failed to push branch %s: %w\nOutput: %s", branchName, err, string(output))
	}

	logger.Debug().
		Str("branch", branchName).
		Msg("Successfully pushed to remote")
	return nil
}

// isNonFastForwardError checks if git push output indicates a non-fast-forward rejection.
// This typically occurs when the remote branch has diverged from local (e.g., after a rebase).
func isNonFastForwardError(output string) bool {
	// Git push outputs "[rejected]" and "non-fast-forward" when the remote has diverged
	return strings.Contains(output, "[rejected]") && strings.Contains(output, "non-fast-forward")
}
