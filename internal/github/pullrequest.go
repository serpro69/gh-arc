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
