package diff

import (
	"context"
	"fmt"
	"strings"

	"github.com/serpro69/gh-arc/internal/github"
	"github.com/serpro69/gh-arc/internal/logger"
)

// PRExecutor handles Pull Request creation, updates, and related operations.
// It consolidates PR workflow logic that was previously duplicated across
// continue mode, fast path, and normal mode in cmd/diff.go.
// PRGitRepository defines the git operations needed for PR execution
type PRGitRepository interface {
	Push(ctx context.Context, branch string) error
	HasUnpushedCommits(branch string) (bool, error)
}

// PRGitHubClient defines the GitHub operations needed for PR execution
type PRGitHubClient interface {
	CreatePullRequest(ctx context.Context, owner, name, title, head, base, body string, draft bool, parentPR *github.PullRequest) (*github.PullRequest, error)
	UpdatePullRequest(ctx context.Context, owner, name string, number int, title, body string, draft *bool, parentPR *github.PullRequest) (*github.PullRequest, error)
	MarkPRReadyForReview(ctx context.Context, owner, name string, pr *github.PullRequest) (*github.PullRequest, error)
	ConvertPRToDraft(ctx context.Context, owner, name string, pr *github.PullRequest) (*github.PullRequest, error)
	AssignReviewers(ctx context.Context, owner, name string, number int, users, teams []string) error
}

type PRExecutor struct {
	client PRGitHubClient
	repo   PRGitRepository
	owner  string
	name   string
}

// PRRequest contains all information needed to create or update a PR
type PRRequest struct {
	Title       string
	HeadBranch  string
	BaseBranch  string
	Body        string
	Draft       bool
	Reviewers   []string
	ExistingPR  *github.PullRequest
	ParentPR    *github.PullRequest
	CurrentUser string
}

// PRResult contains the results of a PR create/update operation
type PRResult struct {
	PR              *github.PullRequest
	WasCreated      bool
	DraftChanged    bool
	ReviewersAdded  []string
	Pushed          bool
	Messages        []string
}

// NewPRExecutor creates a new PR executor
func NewPRExecutor(client PRGitHubClient, repo PRGitRepository, owner, name string) *PRExecutor {
	return &PRExecutor{
		client: client,
		repo:   repo,
		owner:  owner,
		name:   name,
	}
}

// CreateOrUpdatePR handles the full PR creation/update flow including:
// - Pushing commits to remote
// - Creating new PR or updating existing PR
// - Handling draft ↔ ready transitions
// - Assigning reviewers
func (e *PRExecutor) CreateOrUpdatePR(ctx context.Context, req *PRRequest) (*PRResult, error) {
	result := &PRResult{
		Messages: []string{},
	}

	if req.ExistingPR != nil {
		// Update existing PR
		pr, draftChanged, pushed, err := e.updatePR(ctx, req)
		if err != nil {
			return nil, err
		}
		result.PR = pr
		result.WasCreated = false
		result.DraftChanged = draftChanged
		result.Pushed = pushed
	} else {
		// Create new PR
		pr, err := e.createPR(ctx, req)
		if err != nil {
			return nil, err
		}
		result.PR = pr
		result.WasCreated = true
		result.Pushed = true
	}

	// Assign reviewers
	if len(req.Reviewers) > 0 {
		reviewersAdded, err := e.assignReviewers(ctx, result.PR, req.Reviewers, req.CurrentUser)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to assign some reviewers")
			result.Messages = append(result.Messages, fmt.Sprintf("⚠️  Warning: %v", err))
		} else {
			result.ReviewersAdded = reviewersAdded
		}
	}

	return result, nil
}

// createPR creates a new PR, pushing the branch to remote first
func (e *PRExecutor) createPR(ctx context.Context, req *PRRequest) (*github.PullRequest, error) {
	logger.Debug().
		Str("headBranch", req.HeadBranch).
		Str("baseBranch", req.BaseBranch).
		Bool("draft", req.Draft).
		Msg("Creating new PR")

	// Push branch to remote first
	if err := e.repo.Push(ctx, req.HeadBranch); err != nil {
		return nil, fmt.Errorf("failed to push branch: %w", err)
	}

	// Create PR
	return e.client.CreatePullRequest(
		ctx,
		e.owner, e.name,
		req.Title,
		req.HeadBranch,
		req.BaseBranch,
		req.Body,
		req.Draft,
		req.ParentPR,
	)
}

// updatePR updates an existing PR, handling draft status transitions properly.
// Returns the updated PR, whether draft status changed, whether commits were pushed, and any error.
//
// Draft status transitions require special handling:
// - Draft → Ready: Update metadata first, then mark ready via GraphQL
// - Ready → Draft: Update metadata first, then convert to draft via GraphQL
// - No change: Simple update via REST API
func (e *PRExecutor) updatePR(ctx context.Context, req *PRRequest) (*github.PullRequest, bool, bool, error) {
	existing := req.ExistingPR
	draftChanged := false
	pushed := false

	logger.Debug().
		Int("prNumber", existing.Number).
		Bool("currentDraft", existing.Draft).
		Bool("wantDraft", req.Draft).
		Msg("Updating existing PR")

	// Check for unpushed commits and push if needed
	hasUnpushed, err := e.repo.HasUnpushedCommits(req.HeadBranch)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to check for unpushed commits")
		hasUnpushed = true // Assume unpushed on error to be safe
	}

	if hasUnpushed {
		logger.Debug().Msg("Pushing unpushed commits")
		if err := e.repo.Push(ctx, req.HeadBranch); err != nil {
			return nil, false, false, fmt.Errorf("failed to push commits: %w", err)
		}
		pushed = true
	}

	// Handle draft status transitions
	if existing.Draft && !req.Draft {
		// Draft → Ready transition (requires GraphQL)
		draftChanged = true
		logger.Debug().Msg("Transitioning PR from draft to ready")

		// First update title/body while keeping as draft
		if req.Title != "" || req.Body != "" {
			tempDraft := true
			_, err = e.client.UpdatePullRequest(
				ctx,
				e.owner, e.name,
				existing.Number,
				req.Title,
				req.Body,
				&tempDraft,
				req.ParentPR,
			)
			if err != nil {
				return nil, false, pushed, fmt.Errorf("failed to update PR metadata: %w", err)
			}
		}

		// Then mark as ready using GraphQL
		pr, err := e.client.MarkPRReadyForReview(ctx, e.owner, e.name, existing)
		if err != nil {
			return nil, false, pushed, fmt.Errorf("failed to mark PR as ready: %w", err)
		}
		return pr, draftChanged, pushed, nil

	} else if !existing.Draft && req.Draft {
		// Ready → Draft transition (requires GraphQL)
		draftChanged = true
		logger.Debug().Msg("Converting PR from ready to draft")

		// First update title/body while keeping as ready
		if req.Title != "" || req.Body != "" {
			tempDraft := false
			_, err = e.client.UpdatePullRequest(
				ctx,
				e.owner, e.name,
				existing.Number,
				req.Title,
				req.Body,
				&tempDraft,
				req.ParentPR,
			)
			if err != nil {
				return nil, false, pushed, fmt.Errorf("failed to update PR metadata: %w", err)
			}
		}

		// Then convert to draft using GraphQL
		pr, err := e.client.ConvertPRToDraft(ctx, e.owner, e.name, existing)
		if err != nil {
			return nil, false, pushed, fmt.Errorf("failed to convert PR to draft: %w", err)
		}
		return pr, draftChanged, pushed, nil

	} else {
		// No draft status change - simple update via REST API
		logger.Debug().Msg("Updating PR without draft status change")
		draftPtr := &req.Draft
		pr, err := e.client.UpdatePullRequest(
			ctx,
			e.owner, e.name,
			existing.Number,
			req.Title,
			req.Body,
			draftPtr,
			req.ParentPR,
		)
		if err != nil {
			return nil, false, pushed, fmt.Errorf("failed to update PR: %w", err)
		}
		return pr, draftChanged, pushed, nil
	}
}

// assignReviewers assigns reviewers to a PR, filtering out the current user.
// Returns the list of reviewers that were successfully assigned.
func (e *PRExecutor) assignReviewers(ctx context.Context, pr *github.PullRequest, reviewers []string, currentUser string) ([]string, error) {
	if len(reviewers) == 0 {
		return nil, nil
	}

	logger.Debug().
		Int("prNumber", pr.Number).
		Strs("reviewers", reviewers).
		Str("currentUser", currentUser).
		Msg("Assigning reviewers")

	// Parse reviewers into users and teams
	assignment := github.ParseReviewers(reviewers)

	// Filter out current user (can't self-assign as reviewer)
	filteredUsers := []string{}
	for _, user := range assignment.Users {
		if !strings.EqualFold(user, currentUser) {
			filteredUsers = append(filteredUsers, user)
		}
	}
	assignment.Users = filteredUsers

	// Assign if there are any reviewers
	if len(assignment.Users) > 0 || len(assignment.Teams) > 0 {
		err := e.client.AssignReviewers(
			ctx,
			e.owner, e.name,
			pr.Number,
			assignment.Users, assignment.Teams,
		)
		if err != nil {
			return nil, err
		}
		logger.Debug().
			Strs("users", assignment.Users).
			Strs("teams", assignment.Teams).
			Msg("Successfully assigned reviewers")
		return reviewers, nil
	}

	return nil, nil
}

// UpdateDraftStatus updates only the draft status of an existing PR.
// This is a convenience method for the fast path when only draft status needs to change.
func (e *PRExecutor) UpdateDraftStatus(ctx context.Context, pr *github.PullRequest, wantDraft bool) (*github.PullRequest, error) {
	if pr.Draft == wantDraft {
		// No change needed
		return pr, nil
	}

	logger.Debug().
		Int("prNumber", pr.Number).
		Bool("currentDraft", pr.Draft).
		Bool("wantDraft", wantDraft).
		Msg("Updating PR draft status")

	if pr.Draft && !wantDraft {
		// Draft → Ready
		return e.client.MarkPRReadyForReview(ctx, e.owner, e.name, pr)
	} else {
		// Ready → Draft
		return e.client.ConvertPRToDraft(ctx, e.owner, e.name, pr)
	}
}
