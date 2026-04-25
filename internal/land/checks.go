package land

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/github"
)

var (
	ErrDirtyWorkingDir = errors.New("working directory has uncommitted changes")
	ErrOnTrunk         = errors.New("cannot land from the default branch")
	ErrNoPRFound       = errors.New("no open pull request found for current branch")
)

// CheckerRepo defines git operations needed by pre-merge checks.
type CheckerRepo interface {
	GetWorkingDirectoryStatus() (*git.WorkingDirectoryStatus, error)
}

// CheckerClient defines GitHub operations needed by pre-merge checks.
type CheckerClient interface {
	FindExistingPRForCurrentBranch(ctx context.Context, branchName string) (*github.PullRequest, error)
	FindDependentPRsForCurrentBranch(ctx context.Context, branchName string) ([]*github.PullRequest, error)
	GetRequiredStatusChecksForCurrentRepo(ctx context.Context, branch string) ([]string, error)
}

// CheckResult holds the outcome of a single pre-merge check.
type CheckResult struct {
	Passed            bool
	Messages          []string
	NeedsConfirmation bool
}

// PreMergeChecker runs pre-merge validations against a PR.
type PreMergeChecker struct {
	repo   CheckerRepo
	client CheckerClient
	config *config.LandConfig
}

// NewPreMergeChecker creates a new PreMergeChecker.
func NewPreMergeChecker(repo CheckerRepo, client CheckerClient, cfg *config.LandConfig) *PreMergeChecker {
	return &PreMergeChecker{
		repo:   repo,
		client: client,
		config: cfg,
	}
}

// CheckCleanWorkingDir verifies the working directory has no uncommitted changes.
// Always blocks — not bypassable with --force.
func (c *PreMergeChecker) CheckCleanWorkingDir() error {
	status, err := c.repo.GetWorkingDirectoryStatus()
	if err != nil {
		return fmt.Errorf("failed to check working directory: %w", err)
	}
	if !status.IsClean {
		return ErrDirtyWorkingDir
	}
	return nil
}

// CheckNotOnTrunk verifies the current branch is not the default branch.
// Not bypassable — landing trunk onto itself is nonsensical.
func (c *PreMergeChecker) CheckNotOnTrunk(currentBranch, defaultBranch string) error {
	if currentBranch == defaultBranch {
		return fmt.Errorf("%w: currently on %q", ErrOnTrunk, currentBranch)
	}
	return nil
}

// CheckPRExists finds the open PR for the given branch.
func (c *PreMergeChecker) CheckPRExists(ctx context.Context, branchName string) (*github.PullRequest, error) {
	pr, err := c.client.FindExistingPRForCurrentBranch(ctx, branchName)
	if err != nil {
		return nil, fmt.Errorf("failed to find pull request: %w", err)
	}
	if pr == nil {
		return nil, fmt.Errorf("%w: run 'gh arc diff' to create one", ErrNoPRFound)
	}
	return pr, nil
}

// CheckApproval evaluates the PR's approval status based on requireApproval config.
func (c *PreMergeChecker) CheckApproval(_ context.Context, pr *github.PullRequest, force bool) (*CheckResult, error) {
	if c.config.RequireApproval == config.ApprovalNone {
		return &CheckResult{Passed: true, Messages: []string{"Approval check skipped (requireApproval: none)"}}, nil
	}

	approvers, changesRequestedBy := evaluateReviews(pr.Reviews)

	if len(changesRequestedBy) > 0 {
		msg := fmt.Sprintf("PR has outstanding change requests from %s", formatUsers(changesRequestedBy))
		return c.approvalResult(msg, force), nil
	}

	if len(approvers) == 0 {
		msg := "PR needs approval — no reviews yet"
		return c.approvalResult(msg, force), nil
	}

	return &CheckResult{
		Passed:   true,
		Messages: []string{fmt.Sprintf("Approved by %s", formatUsers(approvers))},
	}, nil
}

func (c *PreMergeChecker) approvalResult(msg string, force bool) *CheckResult {
	if force {
		return &CheckResult{Passed: true, Messages: []string{msg + " (bypassed with --force)"}}
	}
	result := &CheckResult{Passed: false, Messages: []string{msg}}
	if c.config.RequireApproval == config.ApprovalPrompt {
		result.NeedsConfirmation = true
	}
	return result
}

// CheckCI evaluates CI check status based on requireCI config.
func (c *PreMergeChecker) CheckCI(ctx context.Context, pr *github.PullRequest, force bool) (*CheckResult, error) {
	if c.config.RequireCI == config.CIModeNone {
		return &CheckResult{Passed: true, Messages: []string{"CI check skipped (requireCI: none)"}}, nil
	}

	relevantChecks, err := c.resolveRelevantChecks(ctx, pr)
	if err != nil {
		return nil, err
	}

	if relevantChecks == nil {
		if force {
			return &CheckResult{Passed: true, Messages: []string{"no CI checks found (bypassed with --force)"}}, nil
		}
		return &CheckResult{Passed: true, Messages: []string{"No required CI checks configured"}}, nil
	}

	var failed, inProgress []string
	passed := 0
	for _, check := range relevantChecks {
		if check.Status != "completed" {
			inProgress = append(inProgress, check.Name)
			continue
		}
		switch check.Conclusion {
		case "success", "skipped", "neutral":
			passed++
		default:
			failed = append(failed, check.Name)
		}
	}

	total := len(relevantChecks)

	if len(failed) == 0 && len(inProgress) == 0 {
		return &CheckResult{
			Passed:   true,
			Messages: []string{fmt.Sprintf("All CI checks passed (%d/%d)", passed, total)},
		}, nil
	}

	msg := formatCIFailureMessage(failed, inProgress, passed, total)
	if force {
		return &CheckResult{Passed: true, Messages: []string{msg + " (bypassed with --force)"}}, nil
	}
	return &CheckResult{Passed: false, Messages: []string{msg}}, nil
}

// resolveRelevantChecks returns the checks to evaluate, or nil when there are
// no checks to enforce (empty required list or no checks present).
func (c *PreMergeChecker) resolveRelevantChecks(ctx context.Context, pr *github.PullRequest) ([]github.PRCheck, error) {
	if c.config.RequireCI == config.CIModeRequired {
		return c.resolveRequiredChecks(ctx, pr)
	}
	if len(pr.Checks) == 0 {
		return nil, nil
	}
	return pr.Checks, nil
}

func (c *PreMergeChecker) resolveRequiredChecks(ctx context.Context, pr *github.PullRequest) ([]github.PRCheck, error) {
	required, err := c.client.GetRequiredStatusChecksForCurrentRepo(ctx, pr.Base.Ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get required status checks: %w", err)
	}
	if len(required) == 0 {
		return nil, nil
	}

	requiredSet := make(map[string]bool, len(required))
	for _, r := range required {
		requiredSet[r] = true
	}

	found := make(map[string]bool)
	var relevant []github.PRCheck
	for _, check := range pr.Checks {
		if requiredSet[check.Name] {
			relevant = append(relevant, check)
			found[check.Name] = true
		}
	}

	// Synthesize placeholder entries for required checks that haven't reported yet
	for _, r := range required {
		if !found[r] {
			relevant = append(relevant, github.PRCheck{
				Name:   r,
				Status: "pending",
			})
		}
	}

	return relevant, nil
}

func formatCIFailureMessage(failed, inProgress []string, passed, total int) string {
	var parts []string
	if len(failed) > 0 {
		parts = append(parts, fmt.Sprintf("'%s' failed", strings.Join(failed, "', '")))
	}
	if len(inProgress) > 0 {
		parts = append(parts, fmt.Sprintf("'%s' in progress", strings.Join(inProgress, "', '")))
	}
	return fmt.Sprintf("CI check %s (%d/%d passed)", strings.Join(parts, ", "), passed, total)
}

// CheckDependentPRs finds PRs that target the given branch.
// Purely informational — never blocks.
func (c *PreMergeChecker) CheckDependentPRs(ctx context.Context, branchName string) ([]*github.PullRequest, error) {
	deps, err := c.client.FindDependentPRsForCurrentBranch(ctx, branchName)
	if err != nil {
		return nil, fmt.Errorf("failed to find dependent PRs: %w", err)
	}
	return deps, nil
}

// evaluateReviews returns the latest review state per reviewer.
// Only the most recent review from each user counts.
func evaluateReviews(reviews []github.PRReview) (approvers []string, changesRequestedBy []string) {
	latest := make(map[string]github.PRReview)
	for _, r := range reviews {
		if existing, ok := latest[r.User.Login]; ok {
			if r.SubmittedAt.After(existing.SubmittedAt) {
				latest[r.User.Login] = r
			}
		} else {
			latest[r.User.Login] = r
		}
	}

	for _, r := range latest {
		switch r.State {
		case "APPROVED":
			approvers = append(approvers, "@"+r.User.Login)
		case "CHANGES_REQUESTED":
			changesRequestedBy = append(changesRequestedBy, "@"+r.User.Login)
		}
	}
	return approvers, changesRequestedBy
}

func formatUsers(users []string) string {
	return strings.Join(users, ", ")
}
