package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/serpro69/gh-arc/internal/cache"
	"github.com/serpro69/gh-arc/internal/filter"
	"github.com/serpro69/gh-arc/internal/format"
	"github.com/serpro69/gh-arc/internal/github"
)

var (
	// listCmd flags
	listAuthor  string
	listStatus  string
	listBranch  string
	listNoCache bool
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Display pending Pull Requests",
	Long: `Display pending Pull Requests with status information, CI states, and review status.

Shows PRs in a formatted table with the following information:
  - PR number and title
  - Author
  - Status (draft, approved, changes requested, review required)
  - CI/Checks status
  - Reviewers and review states
  - Branch information
  - Last updated timestamp

The command caches results for 60 seconds to reduce API calls. Use --no-cache
to force a fresh fetch from GitHub.

Examples:
  # List all open PRs
  gh arc list

  # List PRs by specific author
  gh arc list --author octocat

  # List only PRs awaiting review
  gh arc list --status review_required

  # List PRs from a specific branch
  gh arc list --branch feature/new-feature

  # Output as JSON for scripting
  gh arc list --json

  # Force fresh data from GitHub API
  gh arc list --no-cache`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Define command-specific flags
	listCmd.Flags().StringVarP(&listAuthor, "author", "a", "", "Filter PRs by author (use 'me' for current user)")
	listCmd.Flags().StringVarP(&listStatus, "status", "s", "", "Filter PRs by status (draft, approved, changes_requested, review_required)")
	listCmd.Flags().StringVarP(&listBranch, "branch", "b", "", "Filter PRs by branch name (supports wildcards)")
	listCmd.Flags().BoolVar(&listNoCache, "no-cache", false, "Skip cache and fetch fresh data from GitHub API")
}

// runList executes the list command
func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get current repository
	repo, err := repository.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current repository: %w", err)
	}

	owner, repoName := repo.Owner, repo.Name

	// Create GitHub client
	client, err := github.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Get current user for "me" filter
	currentUser, err := client.GetCurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Initialize cache
	prCache, err := cache.New()
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	// Clean expired cache entries
	_ = prCache.CleanExpired()

	// Generate cache key
	cacheKey := cache.GenerateKey("prs", owner, repoName, "open")

	var prs []*github.PullRequest

	// Try to get from cache if not disabled
	if !listNoCache {
		hit, err := prCache.Get(cacheKey, &prs)
		if err != nil {
			// Log error but continue with fresh fetch
			fmt.Fprintf(os.Stderr, "Cache error: %v\n", err)
		} else if hit {
			// Cache hit - use cached data
			// Still need to filter
			prs = applyFilters(prs, currentUser)
			return outputResults(prs)
		}
	}

	// Fetch PRs from GitHub
	opts := &github.PullRequestListOptions{
		State: "open",
	}

	prs, err = client.GetPullRequests(ctx, owner, repoName, opts)
	if err != nil {
		return fmt.Errorf("failed to fetch pull requests: %w", err)
	}

	// Enrich PRs with metadata (reviews, checks, reviewers)
	if err := client.EnrichPullRequests(ctx, owner, repoName, prs); err != nil {
		return fmt.Errorf("failed to enrich pull requests: %w", err)
	}

	// Cache the results
	if !listNoCache {
		if err := prCache.Set(cacheKey, prs); err != nil {
			// Log error but continue
			fmt.Fprintf(os.Stderr, "Failed to cache results: %v\n", err)
		}
	}

	// Apply filters
	prs = applyFilters(prs, currentUser)

	return outputResults(prs)
}

// applyFilters applies command-line filters to the PR list
func applyFilters(prs []*github.PullRequest, currentUser string) []*github.PullRequest {
	prFilter := &filter.PRFilter{
		Author:      listAuthor,
		Status:      listStatus,
		Branch:      listBranch,
		CurrentUser: currentUser,
	}

	// Apply general filters
	prs = filter.FilterPullRequests(prs, prFilter)

	// Apply draft filter separately if needed
	if listStatus == "draft" {
		var draftPRs []*github.PullRequest
		for _, pr := range prs {
			if filter.MatchesDraft(pr, listStatus) {
				draftPRs = append(draftPRs, pr)
			}
		}
		return draftPRs
	}

	return prs
}

// outputResults outputs the PR list in the requested format
func outputResults(prs []*github.PullRequest) error {
	if GetJSON() {
		// JSON output
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(prs); err != nil {
			return fmt.Errorf("failed to encode JSON: %w", err)
		}
		return nil
	}

	// Table output
	opts := format.DefaultPRFormatterOptions()
	output := format.FormatPRTable(prs, opts)
	fmt.Print(output)

	return nil
}

// getAuthorFilter returns the author filter value or "all" if not set
func getAuthorFilter() string {
	if listAuthor == "" {
		return "all"
	}
	return listAuthor
}

// getStatusFilter returns the status filter value or "all" if not set
func getStatusFilter() string {
	if listStatus == "" {
		return "all"
	}
	return listStatus
}

// getBranchFilter returns the branch filter value or "all" if not set
func getBranchFilter() string {
	if listBranch == "" {
		return "all"
	}
	return listBranch
}
