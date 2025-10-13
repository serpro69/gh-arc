package filter

import (
	"path/filepath"
	"strings"

	"github.com/serpro69/gh-arc/internal/github"
)

// PRFilter contains criteria for filtering pull requests
type PRFilter struct {
	Author        string // Filter by author login (supports "me")
	Status        string // Filter by status (draft, approved, changes_requested, review_required)
	Branch        string // Filter by branch pattern (supports wildcards)
	CurrentUser   string // Current authenticated user login (for "me")
}

// FilterPullRequests applies filters to a list of pull requests
func FilterPullRequests(prs []*github.PullRequest, filter *PRFilter) []*github.PullRequest {
	if filter == nil {
		return prs
	}

	var filtered []*github.PullRequest

	for _, pr := range prs {
		if !matchesFilter(pr, filter) {
			continue
		}
		filtered = append(filtered, pr)
	}

	return filtered
}

// matchesFilter checks if a PR matches all filter criteria
func matchesFilter(pr *github.PullRequest, filter *PRFilter) bool {
	// Filter by author
	if filter.Author != "" {
		author := filter.Author
		if author == "me" {
			author = filter.CurrentUser
		}
		if !strings.EqualFold(pr.User.Login, author) {
			return false
		}
	}

	// Filter by status
	if filter.Status != "" {
		status := github.DeterminePRStatus(pr.Reviews, pr.Checks)
		if !matchesStatus(status, filter.Status) {
			return false
		}
	}

	// Filter by branch
	if filter.Branch != "" {
		if !matchesBranch(pr.Head.Ref, filter.Branch) {
			return false
		}
	}

	return true
}

// matchesStatus checks if a PR status matches the filter
func matchesStatus(status github.PRStatus, filterStatus string) bool {
	filterStatus = strings.ToLower(filterStatus)

	switch filterStatus {
	case "draft":
		// Match if it's a draft (we'd need to check pr.Draft in the caller)
		// For now, match against status fields
		return false // This will be handled in matchesFilter with pr.Draft
	case "approved":
		return status.ReviewStatus == "approved"
	case "changes_requested":
		return status.ReviewStatus == "changes_requested"
	case "review_required":
		return status.ReviewStatus == "review_required"
	case "pending":
		return status.ReviewStatus == "pending"
	case "commented":
		return status.ReviewStatus == "commented"
	default:
		return false
	}
}

// matchesBranch checks if a branch name matches a pattern (with wildcards)
func matchesBranch(branch, pattern string) bool {
	// Use filepath.Match for wildcard matching
	// It supports * and ? wildcards
	matched, err := filepath.Match(pattern, branch)
	if err != nil {
		// If pattern is invalid, do exact match
		return strings.EqualFold(branch, pattern)
	}
	return matched
}

// MatchesDraft checks if a PR matches the draft filter
func MatchesDraft(pr *github.PullRequest, filterStatus string) bool {
	if filterStatus == "draft" {
		return pr.Draft
	}
	return true // Not filtering by draft, so it matches
}
