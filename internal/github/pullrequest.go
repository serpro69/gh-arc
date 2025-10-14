package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/serpro69/gh-arc/internal/logger"
)

// PullRequest represents a GitHub pull request with all relevant information
type PullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"` // open, closed
	Draft     bool      `json:"draft"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	User      PRUser    `json:"user"`
	Head      PRBranch  `json:"head"`
	Base      PRBranch  `json:"base"`
	HTMLURL   string    `json:"html_url"`

	// Additional fields populated by separate API calls
	Reviews    []PRReview    `json:"-"` // Not included in list PR response
	Checks     []PRCheck     `json:"-"` // Not included in list PR response
	Reviewers  []PRReviewer  `json:"-"` // Not included in list PR response
}

// PRUser represents a user associated with a pull request
type PRUser struct {
	Login string `json:"login"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// PRBranch represents a branch in a pull request (head or base)
type PRBranch struct {
	Ref  string       `json:"ref"`  // branch name
	SHA  string       `json:"sha"`  // commit SHA
	Repo PRRepository `json:"repo"` // repository info
}

// PRRepository represents minimal repository information
type PRRepository struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"` // owner/name format
	Owner    PRUser `json:"owner"`
}

// PRReview represents a pull request review
type PRReview struct {
	ID          int       `json:"id"`
	User        PRUser    `json:"user"`
	State       string    `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED, PENDING
	SubmittedAt time.Time `json:"submitted_at"`
}

// PRCheck represents a check run status
type PRCheck struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`      // queued, in_progress, completed
	Conclusion  string    `json:"conclusion"`  // success, failure, neutral, cancelled, skipped, timed_out, action_required
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// PRReviewer represents a requested reviewer
type PRReviewer struct {
	Login string `json:"login"`
	Type  string `json:"type"` // User or Team
}

// PullRequestListOptions contains options for listing pull requests
type PullRequestListOptions struct {
	State     string // open, closed, all (default: open)
	Sort      string // created, updated, popularity, long-running (default: created)
	Direction string // asc, desc (default: desc)
	PerPage   int    // Results per page (default: 30, max: 100)
	Page      int    // Page number (default: 1)
}

// DefaultPullRequestListOptions returns options with sensible defaults
func DefaultPullRequestListOptions() *PullRequestListOptions {
	return &PullRequestListOptions{
		State:     "open",
		Sort:      "updated",
		Direction: "desc",
		PerPage:   30,
		Page:      1,
	}
}

// GetPullRequests fetches pull requests from the specified repository
// It handles pagination, rate limiting, and retries automatically
func (c *Client) GetPullRequests(ctx context.Context, owner, repo string, opts *PullRequestListOptions) ([]*PullRequest, error) {
	if opts == nil {
		opts = DefaultPullRequestListOptions()
	}

	// Build query parameters
	path := fmt.Sprintf("repos/%s/%s/pulls?state=%s&sort=%s&direction=%s&per_page=%d&page=%d",
		owner, repo, opts.State, opts.Sort, opts.Direction, opts.PerPage, opts.Page)

	logger.Debug().
		Str("owner", owner).
		Str("repo", repo).
		Str("path", path).
		Msg("Fetching pull requests")

	// Check circuit breaker
	if !c.circuitBreaker.Allow() {
		return nil, fmt.Errorf("circuit breaker is open, requests are temporarily blocked")
	}

	// Create retry policy
	policy := &RetryPolicy{
		MaxRetries: c.config.MaxRetries,
		BaseDelay:  c.config.BaseDelay,
		MaxDelay:   c.config.MaxDelay,
	}

	var prs []*PullRequest
	var lastErr error

	// Execute with retry logic
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		// Make the API request
		err := c.restClient.Get(path, &prs)

		if err == nil {
			// Success
			c.circuitBreaker.RecordSuccess()
			logger.Debug().
				Int("count", len(prs)).
				Msg("Successfully fetched pull requests")
			return prs, nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryableError(err) {
			c.circuitBreaker.RecordFailure()
			logger.Error().
				Err(err).
				Msg("Non-retryable error fetching pull requests")
			return nil, fmt.Errorf("failed to fetch pull requests: %w", err)
		}

		// If this was the last attempt, don't wait
		if attempt == policy.MaxRetries {
			break
		}

		// Calculate backoff delay
		backoff := policy.calculateBackoff(attempt)

		logger.Debug().
			Int("attempt", attempt+1).
			Dur("backoff", backoff).
			Msg("Retrying pull request fetch after error")

		// Wait before retrying, respecting context cancellation
		select {
		case <-ctx.Done():
			c.circuitBreaker.RecordFailure()
			return nil, fmt.Errorf("request canceled: %w", ctx.Err())
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	// All retries exhausted
	c.circuitBreaker.RecordFailure()
	logger.Error().
		Err(lastErr).
		Int("attempts", policy.MaxRetries+1).
		Msg("Max retries exceeded fetching pull requests")

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// GetPullRequestsWithPagination fetches all pull requests across multiple pages
// It automatically handles pagination by following the Link header
func (c *Client) GetPullRequestsWithPagination(ctx context.Context, owner, repo string, opts *PullRequestListOptions) ([]*PullRequest, error) {
	if opts == nil {
		opts = DefaultPullRequestListOptions()
	}

	var allPRs []*PullRequest
	page := 1

	for {
		opts.Page = page

		// Fetch current page
		prs, err := c.GetPullRequests(ctx, owner, repo, opts)
		if err != nil {
			return allPRs, err
		}

		// If no results, we're done
		if len(prs) == 0 {
			break
		}

		allPRs = append(allPRs, prs...)

		// If we got fewer results than requested, we've reached the last page
		if len(prs) < opts.PerPage {
			break
		}

		page++
	}

	logger.Debug().
		Int("total", len(allPRs)).
		Int("pages", page).
		Msg("Fetched all pull requests with pagination")

	return allPRs, nil
}

// parseLinkHeader parses the Link header from GitHub API responses
// It returns a map of rel values to URLs
// Format: <https://api.github.com/...?page=2>; rel="next", <https://api.github.com/...?page=5>; rel="last"
func parseLinkHeader(linkHeader string) map[string]string {
	links := make(map[string]string)

	if linkHeader == "" {
		return links
	}

	// Split by comma to get individual links
	parts := strings.Split(linkHeader, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)

		// Split by semicolon to separate URL and rel
		sections := strings.Split(part, ";")
		if len(sections) != 2 {
			continue
		}

		// Extract URL (remove < and >)
		url := strings.TrimSpace(sections[0])
		url = strings.TrimPrefix(url, "<")
		url = strings.TrimSuffix(url, ">")

		// Extract rel value
		rel := strings.TrimSpace(sections[1])
		rel = strings.TrimPrefix(rel, "rel=\"")
		rel = strings.TrimSuffix(rel, "\"")

		links[rel] = url
	}

	return links
}

// GetPullRequestsRaw fetches pull requests and returns the raw HTTP response
// This is useful for accessing pagination headers and other metadata
func (c *Client) GetPullRequestsRaw(ctx context.Context, owner, repo string, opts *PullRequestListOptions) ([]*PullRequest, *http.Response, error) {
	if opts == nil {
		opts = DefaultPullRequestListOptions()
	}

	// This is a placeholder - the actual implementation would need to use
	// the lower-level HTTP client to capture the response
	// For now, we'll just call the regular GetPullRequests
	prs, err := c.GetPullRequests(ctx, owner, repo, opts)
	return prs, nil, err
}

// GetCurrentRepositoryPullRequests fetches PRs for the current repository context
func (c *Client) GetCurrentRepositoryPullRequests(ctx context.Context, opts *PullRequestListOptions) ([]*PullRequest, error) {
	if c.repo == nil {
		return nil, fmt.Errorf("no repository context set")
	}

	return c.GetPullRequests(ctx, c.repo.Owner, c.repo.Name, opts)
}

// GetCurrentRepositoryPullRequestsWithPagination fetches all PRs for the current repository
func (c *Client) GetCurrentRepositoryPullRequestsWithPagination(ctx context.Context, opts *PullRequestListOptions) ([]*PullRequest, error) {
	if c.repo == nil {
		return nil, fmt.Errorf("no repository context set")
	}

	return c.GetPullRequestsWithPagination(ctx, c.repo.Owner, c.repo.Name, opts)
}

// GetPullRequestReviews fetches reviews for a specific pull request
func (c *Client) GetPullRequestReviews(ctx context.Context, owner, repo string, number int) ([]PRReview, error) {
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, number)

	logger.Debug().
		Str("owner", owner).
		Str("repo", repo).
		Int("pr", number).
		Msg("Fetching PR reviews")

	var reviews []PRReview
	err := c.restClient.Get(path, &reviews)
	if err != nil {
		logger.Error().
			Err(err).
			Int("pr", number).
			Msg("Failed to fetch PR reviews")
		return nil, fmt.Errorf("failed to fetch reviews for PR #%d: %w", number, err)
	}

	logger.Debug().
		Int("pr", number).
		Int("count", len(reviews)).
		Msg("Successfully fetched PR reviews")

	return reviews, nil
}

// GetPullRequestChecks fetches check runs for a specific pull request's head commit
func (c *Client) GetPullRequestChecks(ctx context.Context, owner, repo, sha string) ([]PRCheck, error) {
	path := fmt.Sprintf("repos/%s/%s/commits/%s/check-runs", owner, repo, sha)

	logger.Debug().
		Str("owner", owner).
		Str("repo", repo).
		Str("sha", sha).
		Msg("Fetching PR check runs")

	var response struct {
		TotalCount int       `json:"total_count"`
		CheckRuns  []PRCheck `json:"check_runs"`
	}

	err := c.restClient.Get(path, &response)
	if err != nil {
		logger.Error().
			Err(err).
			Str("sha", sha).
			Msg("Failed to fetch PR check runs")
		return nil, fmt.Errorf("failed to fetch check runs for SHA %s: %w", sha, err)
	}

	logger.Debug().
		Str("sha", sha).
		Int("count", response.TotalCount).
		Msg("Successfully fetched PR check runs")

	return response.CheckRuns, nil
}

// GetPullRequestRequestedReviewers fetches requested reviewers for a specific pull request
func (c *Client) GetPullRequestRequestedReviewers(ctx context.Context, owner, repo string, number int) ([]PRReviewer, error) {
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/requested_reviewers", owner, repo, number)

	logger.Debug().
		Str("owner", owner).
		Str("repo", repo).
		Int("pr", number).
		Msg("Fetching PR requested reviewers")

	var response struct {
		Users []struct {
			Login string `json:"login"`
		} `json:"users"`
		Teams []struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"teams"`
	}

	err := c.restClient.Get(path, &response)
	if err != nil {
		logger.Error().
			Err(err).
			Int("pr", number).
			Msg("Failed to fetch PR requested reviewers")
		return nil, fmt.Errorf("failed to fetch requested reviewers for PR #%d: %w", number, err)
	}

	// Combine users and teams into a single list
	var reviewers []PRReviewer
	for _, user := range response.Users {
		reviewers = append(reviewers, PRReviewer{
			Login: user.Login,
			Type:  "User",
		})
	}
	for _, team := range response.Teams {
		reviewers = append(reviewers, PRReviewer{
			Login: team.Slug,
			Type:  "Team",
		})
	}

	logger.Debug().
		Int("pr", number).
		Int("count", len(reviewers)).
		Msg("Successfully fetched PR requested reviewers")

	return reviewers, nil
}

// PRStatus represents the aggregated status of a pull request
type PRStatus struct {
	ReviewStatus string // approved, changes_requested, review_required, commented, pending
	CheckStatus  string // success, failure, pending, in_progress, neutral
}

// DeterminePRStatus analyzes reviews and checks to determine overall PR status
func DeterminePRStatus(reviews []PRReview, checks []PRCheck) PRStatus {
	status := PRStatus{
		ReviewStatus: "pending",
		CheckStatus:  "pending",
	}

	// Determine review status
	// Priority: CHANGES_REQUESTED > APPROVED > COMMENTED > PENDING
	hasApproval := false
	hasChangesRequested := false
	hasCommented := false

	for _, review := range reviews {
		switch review.State {
		case "CHANGES_REQUESTED":
			hasChangesRequested = true
		case "APPROVED":
			hasApproval = true
		case "COMMENTED":
			hasCommented = true
		}
	}

	if hasChangesRequested {
		status.ReviewStatus = "changes_requested"
	} else if hasApproval {
		status.ReviewStatus = "approved"
	} else if hasCommented {
		status.ReviewStatus = "commented"
	} else if len(reviews) > 0 {
		status.ReviewStatus = "pending"
	} else {
		status.ReviewStatus = "review_required"
	}

	// Determine check status
	if len(checks) == 0 {
		status.CheckStatus = "pending"
		return status
	}

	hasFailure := false
	hasInProgress := false
	allSuccess := true

	for _, check := range checks {
		if check.Status != "completed" {
			hasInProgress = true
			allSuccess = false
			continue
		}

		switch check.Conclusion {
		case "failure", "timed_out", "action_required":
			hasFailure = true
			allSuccess = false
		case "success":
			// Keep tracking
		case "neutral", "cancelled", "skipped":
			allSuccess = false
		}
	}

	if hasFailure {
		status.CheckStatus = "failure"
	} else if hasInProgress {
		status.CheckStatus = "in_progress"
	} else if allSuccess {
		status.CheckStatus = "success"
	} else {
		status.CheckStatus = "neutral"
	}

	return status
}

// EnrichPullRequest fetches and adds additional metadata to a pull request
// This includes reviews, checks, and requested reviewers
func (c *Client) EnrichPullRequest(ctx context.Context, owner, repo string, pr *PullRequest) error {
	if pr == nil {
		return fmt.Errorf("pull request is nil")
	}

	// Use error group pattern for parallel API calls
	type result struct {
		reviews   []PRReview
		checks    []PRCheck
		reviewers []PRReviewer
		err       error
	}

	resultChan := make(chan result, 1)

	go func() {
		res := result{}

		// Create a channel for each API call
		reviewsChan := make(chan []PRReview, 1)
		checksChan := make(chan []PRCheck, 1)
		reviewersChan := make(chan []PRReviewer, 1)
		errChan := make(chan error, 3)

		// Fetch reviews in parallel
		go func() {
			reviews, err := c.GetPullRequestReviews(ctx, owner, repo, pr.Number)
			if err != nil {
				errChan <- err
				return
			}
			reviewsChan <- reviews
		}()

		// Fetch checks in parallel
		go func() {
			checks, err := c.GetPullRequestChecks(ctx, owner, repo, pr.Head.SHA)
			if err != nil {
				errChan <- err
				return
			}
			checksChan <- checks
		}()

		// Fetch requested reviewers in parallel
		go func() {
			reviewers, err := c.GetPullRequestRequestedReviewers(ctx, owner, repo, pr.Number)
			if err != nil {
				errChan <- err
				return
			}
			reviewersChan <- reviewers
		}()

		// Collect results
		for i := 0; i < 3; i++ {
			select {
			case reviews := <-reviewsChan:
				res.reviews = reviews
			case checks := <-checksChan:
				res.checks = checks
			case reviewers := <-reviewersChan:
				res.reviewers = reviewers
			case err := <-errChan:
				// Log error but don't fail the entire operation
				logger.Warn().
					Err(err).
					Int("pr", pr.Number).
					Msg("Failed to fetch some PR metadata")
			case <-ctx.Done():
				res.err = ctx.Err()
				resultChan <- res
				return
			}
		}

		resultChan <- res
	}()

	// Wait for results
	res := <-resultChan
	if res.err != nil {
		return res.err
	}

	// Update PR with fetched metadata
	pr.Reviews = res.reviews
	pr.Checks = res.checks
	pr.Reviewers = res.reviewers

	logger.Debug().
		Int("pr", pr.Number).
		Int("reviews", len(pr.Reviews)).
		Int("checks", len(pr.Checks)).
		Int("reviewers", len(pr.Reviewers)).
		Msg("Successfully enriched PR with metadata")

	return nil
}

// EnrichPullRequests enriches multiple pull requests with metadata in parallel
func (c *Client) EnrichPullRequests(ctx context.Context, owner, repo string, prs []*PullRequest) error {
	if len(prs) == 0 {
		return nil
	}

	// Use a semaphore to limit concurrent API calls
	// GitHub API has rate limits, so we don't want to overwhelm it
	maxConcurrent := 5
	semaphore := make(chan struct{}, maxConcurrent)
	errChan := make(chan error, len(prs))
	doneChan := make(chan struct{})

	// Track active goroutines
	activeCount := 0

	for _, pr := range prs {
		activeCount++
		go func(pr *PullRequest) {
			semaphore <- struct{}{} // Acquire
			defer func() {
				<-semaphore // Release
				doneChan <- struct{}{}
			}()

			if err := c.EnrichPullRequest(ctx, owner, repo, pr); err != nil {
				errChan <- fmt.Errorf("failed to enrich PR #%d: %w", pr.Number, err)
			}
		}(pr)
	}

	// Wait for all goroutines to complete
	for i := 0; i < activeCount; i++ {
		select {
		case <-doneChan:
			// One more completed
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Check for errors
	close(errChan)
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		// Log all errors but return the first one
		for _, err := range errors {
			logger.Error().Err(err).Msg("Error enriching PR")
		}
		return errors[0]
	}

	logger.Debug().
		Int("count", len(prs)).
		Msg("Successfully enriched all PRs with metadata")

	return nil
}

// FindExistingPR finds an existing pull request for the given branch
// Returns the PR if found, nil if not found, or error if there was an API issue
func (c *Client) FindExistingPR(ctx context.Context, owner, repo, branchName string) (*PullRequest, error) {
	logger.Debug().
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", branchName).
		Msg("Finding existing PR for branch")

	// Get all open PRs
	opts := &PullRequestListOptions{
		State:     "open",
		Sort:      "updated",
		Direction: "desc",
		PerPage:   100,
		Page:      1,
	}

	prs, err := c.GetPullRequests(ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull requests: %w", err)
	}

	// Find PR matching the branch
	for _, pr := range prs {
		if pr.Head.Ref == branchName {
			logger.Info().
				Int("number", pr.Number).
				Str("branch", branchName).
				Msg("Found existing PR for branch")
			return pr, nil
		}
	}

	logger.Debug().
		Str("branch", branchName).
		Msg("No existing PR found for branch")
	return nil, nil
}

// FindExistingPRForCurrentBranch finds an existing PR for the current repository's branch
func (c *Client) FindExistingPRForCurrentBranch(ctx context.Context, branchName string) (*PullRequest, error) {
	if c.repo == nil {
		return nil, fmt.Errorf("no repository context set")
	}

	return c.FindExistingPR(ctx, c.repo.Owner, c.repo.Name, branchName)
}

// DetectBaseChanged checks if the base branch of an existing PR differs from the detected base
// Returns true if the base has changed and needs updating
func DetectBaseChanged(existingPR *PullRequest, detectedBase string) bool {
	if existingPR == nil {
		return false
	}

	changed := existingPR.Base.Ref != detectedBase

	if changed {
		logger.Info().
			Int("pr", existingPR.Number).
			Str("currentBase", existingPR.Base.Ref).
			Str("detectedBase", detectedBase).
			Msg("Detected base branch change")
	}

	return changed
}

// UpdatePRBase updates the base branch of an existing pull request
// This is useful for stacked PRs when the parent branch changes
func (c *Client) UpdatePRBase(ctx context.Context, owner, repo string, number int, newBase string) error {
	logger.Info().
		Int("pr", number).
		Str("newBase", newBase).
		Msg("Updating PR base branch")

	path := fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, number)

	// Prepare update payload
	payload := map[string]interface{}{
		"base": newBase,
	}

	// Use Do method with PATCH
	err := c.Do(ctx, "PATCH", path, payload, nil)
	if err != nil {
		return fmt.Errorf("failed to update PR base: %w", err)
	}

	logger.Info().
		Int("pr", number).
		Str("newBase", newBase).
		Msg("Successfully updated PR base branch")

	return nil
}

// UpdatePRBaseForCurrentRepo updates the base branch for a PR in the current repository
func (c *Client) UpdatePRBaseForCurrentRepo(ctx context.Context, number int, newBase string) error {
	if c.repo == nil {
		return fmt.Errorf("no repository context set")
	}

	return c.UpdatePRBase(ctx, c.repo.Owner, c.repo.Name, number, newBase)
}

// FindDependentPRs finds all pull requests that target the given branch as their base
// These are "child" PRs in a stacked PR workflow - PRs that depend on the given branch
func (c *Client) FindDependentPRs(ctx context.Context, owner, repo, baseBranch string) ([]*PullRequest, error) {
	logger.Debug().
		Str("owner", owner).
		Str("repo", repo).
		Str("baseBranch", baseBranch).
		Msg("Finding dependent PRs targeting branch as base")

	// Get all open PRs
	opts := &PullRequestListOptions{
		State:     "open",
		Sort:      "updated",
		Direction: "desc",
		PerPage:   100,
		Page:      1,
	}

	prs, err := c.GetPullRequests(ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull requests: %w", err)
	}

	// Filter for PRs where base.ref matches the given branch
	var dependentPRs []*PullRequest
	for _, pr := range prs {
		if pr.Base.Ref == baseBranch {
			dependentPRs = append(dependentPRs, pr)
		}
	}

	if len(dependentPRs) > 0 {
		logger.Info().
			Str("baseBranch", baseBranch).
			Int("count", len(dependentPRs)).
			Msg("Found dependent PRs targeting branch")
	} else {
		logger.Debug().
			Str("baseBranch", baseBranch).
			Msg("No dependent PRs found")
	}

	return dependentPRs, nil
}

// FindDependentPRsForCurrentBranch finds dependent PRs for the current repository's branch
func (c *Client) FindDependentPRsForCurrentBranch(ctx context.Context, branchName string) ([]*PullRequest, error) {
	if c.repo == nil {
		return nil, fmt.Errorf("no repository context set")
	}

	return c.FindDependentPRs(ctx, c.repo.Owner, c.repo.Name, branchName)
}

// DetectRebase checks if the base commit SHA has changed, indicating a rebase occurred
// This is different from DetectBaseChanged which only checks branch names
// Returns true if the base SHA differs, suggesting a rebase on the same branch
func DetectRebase(existingPR *PullRequest, currentBaseSHA string) bool {
	if existingPR == nil || currentBaseSHA == "" {
		return false
	}

	// Compare the PR's base SHA with the current base commit SHA
	rebased := existingPR.Base.SHA != currentBaseSHA

	if rebased {
		// Show short SHA for readability (7 chars if available, otherwise full SHA)
		prSHA := existingPR.Base.SHA
		if len(prSHA) > 7 {
			prSHA = prSHA[:7]
		}

		currSHA := currentBaseSHA
		if len(currSHA) > 7 {
			currSHA = currSHA[:7]
		}

		logger.Info().
			Int("pr", existingPR.Number).
			Str("prBaseSHA", prSHA).
			Str("currentBaseSHA", currSHA).
			Msg("Detected base rebase - SHA changed")
	}

	return rebased
}

// StackedPRUpdateResult represents the result of a stacked PR update operation
type StackedPRUpdateResult struct {
	UpdatedBase    bool     // Whether the base was actually updated
	OldBase        string   // Previous base branch name
	NewBase        string   // New base branch name
	RebaseDetected bool     // Whether a rebase was detected
	Error          error    // Any error that occurred
}

// HandleStackedPRUpdate orchestrates the complete workflow for updating stacked PR bases
// It detects base changes, optionally prompts user, updates PR base, and displays results
// Returns StackedPRUpdateResult with details of what was updated
func (c *Client) HandleStackedPRUpdate(
	ctx context.Context,
	existingPR *PullRequest,
	detectedBase string,
	currentBaseSHA string,
	promptUser bool,
) (*StackedPRUpdateResult, error) {
	result := &StackedPRUpdateResult{
		UpdatedBase:    false,
		OldBase:        "",
		NewBase:        detectedBase,
		RebaseDetected: false,
	}

	if existingPR == nil {
		result.Error = fmt.Errorf("existing PR is nil")
		return result, result.Error
	}

	result.OldBase = existingPR.Base.Ref

	// Check for base branch name change
	baseChanged := DetectBaseChanged(existingPR, detectedBase)

	// Check for rebase (SHA change on same branch)
	rebased := false
	if currentBaseSHA != "" {
		rebased = DetectRebase(existingPR, currentBaseSHA)
		result.RebaseDetected = rebased
	}

	// If neither changed, nothing to do
	if !baseChanged && !rebased {
		logger.Debug().
			Int("pr", existingPR.Number).
			Msg("No base changes detected")
		return result, nil
	}

	// Log what changed
	if baseChanged {
		logger.Info().
			Int("pr", existingPR.Number).
			Str("from", result.OldBase).
			Str("to", result.NewBase).
			Msg("Base branch change detected")
	}

	if rebased {
		logger.Info().
			Int("pr", existingPR.Number).
			Str("branch", existingPR.Base.Ref).
			Msg("Base rebase detected (SHA changed)")
	}

	// TODO: Add user prompt support when promptUser is true
	// For now, we always proceed with the update
	if promptUser {
		logger.Debug().Msg("User prompting not yet implemented, proceeding with update")
	}

	// Only update if the branch name changed
	// Rebase detection is informational - the PR base ref doesn't need updating
	if baseChanged {
		if c.repo == nil {
			result.Error = fmt.Errorf("no repository context set")
			return result, result.Error
		}

		err := c.UpdatePRBase(ctx, c.repo.Owner, c.repo.Name, existingPR.Number, detectedBase)
		if err != nil {
			result.Error = fmt.Errorf("failed to update PR base: %w", err)
			return result, result.Error
		}

		result.UpdatedBase = true

		logger.Info().
			Int("pr", existingPR.Number).
			Str("newBase", detectedBase).
			Msg("Successfully updated stacked PR base")
	}

	return result, nil
}
