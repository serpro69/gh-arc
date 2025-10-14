package diff

import (
	"context"
	"fmt"
	"strings"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/github"
	"github.com/serpro69/gh-arc/internal/logger"
)

// GitRepository defines the interface for git operations needed by base detection
type GitRepository interface {
	Path() string
	GetDefaultBranch() (string, error)
	ListBranches(includeRemote bool) ([]git.BranchInfo, error)
	GetMergeBase(ref1, ref2 string) (string, error)
	GetCommitRange(from, to string) ([]git.CommitInfo, error)
}

// GitHubClient defines the interface for GitHub operations needed by base detection
type GitHubClient interface {
	GetPullRequests(ctx context.Context, owner, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, error)
}

// BaseBranchResult contains the result of base branch detection
type BaseBranchResult struct {
	Base       string                 // The detected base branch name
	IsStacking bool                   // Whether this is a stacked PR
	ParentPR   *github.PullRequest    // The parent PR if stacking
	Method     string                 // How the base was determined
}

// BaseBranchDetector handles intelligent base branch detection for stacking
type BaseBranchDetector struct {
	repo         GitRepository
	gitHubClient GitHubClient
	config       *config.DiffConfig
	owner        string
	repoName     string
}

// NewBaseBranchDetector creates a new base branch detector
func NewBaseBranchDetector(
	repo GitRepository,
	gitHubClient GitHubClient,
	cfg *config.DiffConfig,
	owner string,
	repoName string,
) *BaseBranchDetector {
	return &BaseBranchDetector{
		repo:         repo,
		gitHubClient: gitHubClient,
		config:       cfg,
		owner:        owner,
		repoName:     repoName,
	}
}

// DetectBaseBranch intelligently detects the base branch for a PR,
// considering stacking scenarios
func (d *BaseBranchDetector) DetectBaseBranch(
	ctx context.Context,
	currentBranch string,
	baseFlagValue string,
) (*BaseBranchResult, error) {
	logger.Debug().
		Str("currentBranch", currentBranch).
		Str("baseFlagValue", baseFlagValue).
		Bool("stackingEnabled", d.config.EnableStacking).
		Msg("Starting base branch detection")

	// Priority 1: If --base flag is provided, use it (explicit override)
	if baseFlagValue != "" {
		logger.Info().
			Str("base", baseFlagValue).
			Msg("Using explicitly provided base branch from --base flag")
		return &BaseBranchResult{
			Base:       baseFlagValue,
			IsStacking: false, // Explicit base means user wants to override stacking
			ParentPR:   nil,
			Method:     "explicit-flag",
		}, nil
	}

	// Priority 2: If config has defaultBase, use it
	if d.config.DefaultBase != "" {
		logger.Info().
			Str("base", d.config.DefaultBase).
			Msg("Using default base from configuration")
		return &BaseBranchResult{
			Base:       d.config.DefaultBase,
			IsStacking: false,
			ParentPR:   nil,
			Method:     "config-default",
		}, nil
	}

	// Priority 3: If stacking is disabled, use default branch
	if !d.config.EnableStacking {
		defaultBranch, err := d.repo.GetDefaultBranch()
		if err != nil {
			return nil, fmt.Errorf("failed to get default branch: %w", err)
		}
		logger.Info().
			Str("base", defaultBranch).
			Msg("Using default branch (stacking disabled)")
		return &BaseBranchResult{
			Base:       defaultBranch,
			IsStacking: false,
			ParentPR:   nil,
			Method:     "default-branch-no-stacking",
		}, nil
	}

	// Priority 4: Auto-detect stacking opportunity
	stackingResult, err := d.detectStackingBase(ctx, currentBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to detect stacking base: %w", err)
	}

	if stackingResult != nil {
		logger.Info().
			Str("base", stackingResult.Base).
			Bool("isStacking", stackingResult.IsStacking).
			Msg("Detected stacking opportunity")
		return stackingResult, nil
	}

	// Priority 5: No stacking detected, use default branch
	defaultBranch, err := d.repo.GetDefaultBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get default branch: %w", err)
	}
	logger.Info().
		Str("base", defaultBranch).
		Msg("Using default branch (no stacking detected)")
	return &BaseBranchResult{
		Base:       defaultBranch,
		IsStacking: false,
		ParentPR:   nil,
		Method:     "default-branch",
	}, nil
}

// detectStackingBase attempts to detect if current branch should stack on another feature branch
func (d *BaseBranchDetector) detectStackingBase(
	ctx context.Context,
	currentBranch string,
) (*BaseBranchResult, error) {
	logger.Debug().
		Str("currentBranch", currentBranch).
		Msg("Attempting to detect stacking opportunity")

	// Get default branch (main/master)
	defaultBranch, err := d.repo.GetDefaultBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get default branch: %w", err)
	}

	// Get all local branches
	branchInfos, err := d.repo.ListBranches(false)
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}

	// Get all open PRs for this repository
	openPRs, err := d.gitHubClient.GetPullRequests(ctx, d.owner, d.repoName, &github.PullRequestListOptions{
		State:   "open",
		PerPage: 100,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get open PRs: %w", err)
	}

	// Create a map of branch -> PR for quick lookup
	branchToPR := make(map[string]*github.PullRequest)
	for _, pr := range openPRs {
		branchToPR[pr.Head.Ref] = pr
	}

	// Find merge-base with default branch to understand divergence point
	mergeBaseWithDefault, err := d.repo.GetMergeBase(currentBranch, defaultBranch)
	if err != nil {
		logger.Debug().
			Err(err).
			Str("currentBranch", currentBranch).
			Str("defaultBranch", defaultBranch).
			Msg("Failed to get merge-base with default branch")
		return nil, nil // Not an error, just no stacking detected
	}

	// Check each local branch (excluding current and default)
	for _, branchInfo := range branchInfos {
		branch := branchInfo.Name

		// Skip current branch and default branch
		if branch == currentBranch || branch == defaultBranch {
			continue
		}

		// Skip branches without open PRs
		parentPR, hasPR := branchToPR[branch]
		if !hasPR {
			continue
		}

		// Get merge-base between current branch and candidate parent branch
		mergeBaseWithCandidate, err := d.repo.GetMergeBase(currentBranch, branch)
		if err != nil {
			logger.Debug().
				Err(err).
				Str("currentBranch", currentBranch).
				Str("candidateBranch", branch).
				Msg("Failed to get merge-base with candidate branch")
			continue
		}

		// Check if current branch diverged from the candidate branch
		// If merge-base(current, candidate) is different from merge-base(current, default),
		// then current likely diverged from candidate
		// Stacking condition:
		// 1. merge-base(current, candidate) != merge-base(current, default)
		//    This means current diverged from candidate, not just from default
		// 2. merge-base(current, candidate) is the head of candidate OR close to it
		//    This means current was branched off from candidate
		if mergeBaseWithCandidate != mergeBaseWithDefault {
			// Check if candidate branch is an ancestor of current branch
			isAncestor, err := d.isAncestor(mergeBaseWithCandidate, currentBranch)
			if err != nil {
				logger.Debug().
					Err(err).
					Msg("Failed to check ancestor relationship")
				continue
			}

			if isAncestor {
				// Safely truncate SHA for logging
				shortSHA := mergeBaseWithCandidate
				if len(shortSHA) > 8 {
					shortSHA = shortSHA[:8]
				}
				logger.Info().
					Str("parentBranch", branch).
					Str("parentPR", fmt.Sprintf("#%d", parentPR.Number)).
					Str("mergeBase", shortSHA).
					Msg("Detected stacking opportunity")

				return &BaseBranchResult{
					Base:       branch,
					IsStacking: true,
					ParentPR:   parentPR,
					Method:     "auto-detected-stacking",
				}, nil
			}
		}
	}

	// No stacking opportunity detected
	logger.Debug().Msg("No stacking opportunity detected")
	return nil, nil
}

// isAncestor checks if ancestorSHA is an ancestor of descendantBranch
func (d *BaseBranchDetector) isAncestor(ancestorSHA, descendantBranch string) (bool, error) {
	// Use git merge-base --is-ancestor
	// This returns 0 if ancestorSHA is an ancestor of descendantBranch
	commits, err := d.repo.GetCommitRange(ancestorSHA, descendantBranch)
	if err != nil {
		return false, err
	}

	// If there are commits between ancestor and descendant, ancestor is indeed an ancestor
	return len(commits) > 0, nil
}

// FormatStackingMessage returns a user-friendly message about the stacking result
func (r *BaseBranchResult) FormatStackingMessage() string {
	if !r.IsStacking {
		return fmt.Sprintf("Creating PR with base: %s", r.Base)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔗 Detected stacking opportunity\n"))
	sb.WriteString(fmt.Sprintf("   Base branch: %s\n", r.Base))
	if r.ParentPR != nil {
		sb.WriteString(fmt.Sprintf("   Parent PR: #%d - %s\n", r.ParentPR.Number, r.ParentPR.Title))
	}
	sb.WriteString(fmt.Sprintf("   This PR will stack on top of the parent PR\n"))
	sb.WriteString(fmt.Sprintf("   (Use --base flag to override)\n"))

	return sb.String()
}
