package diff

import (
	"context"
	"errors"
	"fmt"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/logger"
)

// Sentinel errors for error type checking with errors.Is()
var (
	// ErrOperationCancelled indicates user cancelled the operation
	ErrOperationCancelled = errors.New("operation cancelled by user")

	// ErrStaleRemote indicates user declined to continue with stale remote
	ErrStaleRemote = errors.New("operation declined due to stale remote")
)

// AutoBranchDetector is the main detector and orchestrator for auto-branch operations.
// It handles detection of commits on main and orchestrates the auto-branch creation flow.
type AutoBranchDetector struct {
	repo   *git.Repository
	config *config.DiffConfig
}

// NewAutoBranchDetector creates a new AutoBranchDetector with the given repository and configuration.
func NewAutoBranchDetector(repo *git.Repository, cfg *config.DiffConfig) *AutoBranchDetector {
	return &AutoBranchDetector{
		repo:   repo,
		config: cfg,
	}
}

// DetectionResult represents the result of detecting commits on main.
type DetectionResult struct {
	OnMainBranch  bool   // Whether the current branch is main/master
	CommitsAhead  int    // Number of commits ahead of origin/main
	DefaultBranch string // The default branch name (main or master)
}

// AutoBranchContext holds state for the auto-branch operation.
// This context is passed through the diff command flow to coordinate
// the push-before-PR and checkout-after-PR operations.
type AutoBranchContext struct {
	BranchName    string // Generated branch name for the auto-branch
	ShouldProceed bool   // Whether the operation should proceed
}

// DetectCommitsOnMain detects if the user is on main/master with unpushed commits.
// It returns a DetectionResult with the current state, or an error if detection fails.
func (d *AutoBranchDetector) DetectCommitsOnMain(ctx context.Context) (*DetectionResult, error) {
	logger := logger.Get()

	// Get current branch
	currentBranch, err := d.repo.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Get default branch
	defaultBranch, err := d.repo.GetDefaultBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get default branch: %w", err)
	}

	// Check if we're on the main branch
	onMainBranch := currentBranch == defaultBranch

	// Initialize result
	result := &DetectionResult{
		OnMainBranch:  onMainBranch,
		CommitsAhead:  0,
		DefaultBranch: defaultBranch,
	}

	// If we're not on main, no need to count commits
	if !onMainBranch {
		logger.Debug().
			Str("currentBranch", currentBranch).
			Str("defaultBranch", defaultBranch).
			Bool("onMainBranch", false).
			Msg("Not on main branch, skipping commit count")
		return result, nil
	}

	// Count commits ahead of origin/defaultBranch
	remoteBranch := "origin/" + defaultBranch
	commitsAhead, err := d.repo.CountCommitsAhead(currentBranch, remoteBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to count commits ahead: %w", err)
	}

	result.CommitsAhead = commitsAhead

	logger.Debug().
		Str("currentBranch", currentBranch).
		Str("defaultBranch", defaultBranch).
		Str("remoteBranch", remoteBranch).
		Bool("onMainBranch", onMainBranch).
		Int("commitsAhead", commitsAhead).
		Msg("Detected commits on main")

	return result, nil
}

// ShouldAutoBranch determines if the auto-branch flow should be activated.
// Returns true if the user is on main/master with unpushed commits.
func (d *AutoBranchDetector) ShouldAutoBranch(result *DetectionResult) bool {
	return result.OnMainBranch && result.CommitsAhead > 0
}
