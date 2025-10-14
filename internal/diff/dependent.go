package diff

import (
	"context"
	"fmt"
	"strings"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/github"
	"github.com/serpro69/gh-arc/internal/logger"
)

// DependentPRInfo contains information about PRs that depend on the current branch
type DependentPRInfo struct {
	DependentPRs []*github.PullRequest // PRs that target this branch
	HasDependents bool                  // Whether there are any dependent PRs
}

// DependentPRDetector handles detection of PRs that stack on top of the current branch
type DependentPRDetector struct {
	gitHubClient GitHubClient
	config       *config.DiffConfig
	owner        string
	repoName     string
}

// NewDependentPRDetector creates a new dependent PR detector
func NewDependentPRDetector(
	gitHubClient GitHubClient,
	cfg *config.DiffConfig,
	owner string,
	repoName string,
) *DependentPRDetector {
	return &DependentPRDetector{
		gitHubClient: gitHubClient,
		config:       cfg,
		owner:        owner,
		repoName:     repoName,
	}
}

// DetectDependentPRs finds all PRs that target the given branch as their base
func (d *DependentPRDetector) DetectDependentPRs(
	ctx context.Context,
	branchName string,
) (*DependentPRInfo, error) {
	logger.Debug().
		Str("branch", branchName).
		Msg("Detecting dependent PRs")

	// Get all open PRs
	openPRs, err := d.gitHubClient.GetPullRequests(ctx, d.owner, d.repoName, &github.PullRequestListOptions{
		State:   "open",
		PerPage: 100,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get open PRs: %w", err)
	}

	// Find PRs that target this branch
	var dependentPRs []*github.PullRequest
	for _, pr := range openPRs {
		if pr.Base.Ref == branchName {
			dependentPRs = append(dependentPRs, pr)
			logger.Debug().
				Int("prNumber", pr.Number).
				Str("prTitle", pr.Title).
				Str("headBranch", pr.Head.Ref).
				Msg("Found dependent PR")
		}
	}

	logger.Info().
		Int("count", len(dependentPRs)).
		Msg("Detected dependent PRs")

	return &DependentPRInfo{
		DependentPRs:  dependentPRs,
		HasDependents: len(dependentPRs) > 0,
	}, nil
}

// FormatDependentPRsWarning returns a user-friendly warning message about dependent PRs
func (info *DependentPRInfo) FormatDependentPRsWarning() string {
	if !info.HasDependents {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n⚠️  Warning: This branch has dependent PRs\n")
	sb.WriteString(fmt.Sprintf("   %d PR(s) target this branch:\n", len(info.DependentPRs)))

	for _, pr := range info.DependentPRs {
		sb.WriteString(fmt.Sprintf("   - #%d: %s (branch: %s)\n", pr.Number, pr.Title, pr.Head))
	}

	sb.WriteString("\n   Changes to this branch will affect the dependent PRs.\n")
	sb.WriteString("   Consider updating dependent PRs if you modify the base commits.\n")

	return sb.String()
}

// FormatDependentPRsInfo returns a user-friendly informational message about dependent PRs
func (info *DependentPRInfo) FormatDependentPRsInfo() string {
	if !info.HasDependents {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\nℹ️  This branch has %d dependent PR(s):\n", len(info.DependentPRs)))

	for _, pr := range info.DependentPRs {
		sb.WriteString(fmt.Sprintf("   - #%d: %s\n", pr.Number, pr.Title))
	}

	return sb.String()
}

// ShouldShowWarning determines if the warning should be shown based on configuration
func (d *DependentPRDetector) ShouldShowWarning() bool {
	return d.config.ShowStackingWarnings
}
