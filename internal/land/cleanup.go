package land

import (
	"errors"
	"fmt"
)

var (
	ErrCleanupInvalidArgs = errors.New("cleanup: invalid arguments")
)

// CleanupRepo defines git operations needed by post-merge cleanup.
// TODO: accept context.Context for PullOrigin (network-bound) once the
// existing non-context pattern (CheckoutTrackingBranch etc.) is migrated.
type CleanupRepo interface {
	CheckoutBranch(branch string) error
	PullOrigin(branch string) error
	GetBranchSHA(branch string) (string, error)
	DeleteLocalBranch(branch string) error
}

// CleanupResult holds the outcome of the post-merge cleanup sequence.
type CleanupResult struct {
	CheckedOut       bool
	Pulled           bool
	BranchDeleted    bool
	DeletedBranchSHA string
	Warnings         []string
}

// PostMergeCleanup handles workspace cleanup after a successful merge.
type PostMergeCleanup struct {
	repo CleanupRepo
}

// NewPostMergeCleanup creates a new PostMergeCleanup.
func NewPostMergeCleanup(repo CleanupRepo) *PostMergeCleanup {
	return &PostMergeCleanup{repo: repo}
}

// Execute runs the post-merge cleanup sequence: checkout default branch,
// pull latest, and optionally delete the feature branch.
// Failures are captured as warnings — the merge already succeeded.
func (c *PostMergeCleanup) Execute(defaultBranch, featureBranch string, noDelete bool) (*CleanupResult, error) {
	if defaultBranch == "" || featureBranch == "" {
		return nil, fmt.Errorf("%w: defaultBranch and featureBranch must be non-empty", ErrCleanupInvalidArgs)
	}
	if defaultBranch == featureBranch {
		return nil, fmt.Errorf("%w: refusing to delete the default branch %q", ErrCleanupInvalidArgs, defaultBranch)
	}

	result := &CleanupResult{}

	if err := c.repo.CheckoutBranch(defaultBranch); err != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Failed to checkout %s: %v — run 'git checkout %s' manually", defaultBranch, err, defaultBranch))
		return result, nil
	}
	result.CheckedOut = true

	if err := c.repo.PullOrigin(defaultBranch); err != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Failed to pull latest on %s: %v — run 'git pull origin %s' manually", defaultBranch, err, defaultBranch))
	} else {
		result.Pulled = true
	}

	if noDelete {
		return result, nil
	}

	sha, err := c.deleteLocalBranch(featureBranch)
	if err != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Failed to delete branch %s: %v — run 'git branch -D %s' manually", featureBranch, err, featureBranch))
	} else {
		result.BranchDeleted = true
		result.DeletedBranchSHA = sha
	}

	return result, nil
}

// deleteLocalBranch captures the branch SHA before deletion and returns it
// so the caller can display a restore command.
func (c *PostMergeCleanup) deleteLocalBranch(branch string) (string, error) {
	sha, err := c.repo.GetBranchSHA(branch)
	if err != nil {
		return "", fmt.Errorf("failed to capture branch SHA: %w", err)
	}

	if err := c.repo.DeleteLocalBranch(branch); err != nil {
		return "", err
	}

	return sha, nil
}
