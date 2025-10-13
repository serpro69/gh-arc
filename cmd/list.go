package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
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
	// TODO: Implement list functionality in subsequent subtasks
	// This includes:
	// 1. GitHub API integration for fetching PRs
	// 2. Fetching and aggregating PR metadata (reviews, checks)
	// 3. Building table output with formatting and colors
	// 4. Implementing filtering and caching logic

	fmt.Println("List command is not yet fully implemented.")
	fmt.Println("Configuration:")
	fmt.Printf("  Author filter: %s\n", getAuthorFilter())
	fmt.Printf("  Status filter: %s\n", getStatusFilter())
	fmt.Printf("  Branch filter: %s\n", getBranchFilter())
	fmt.Printf("  Use cache: %t\n", !listNoCache)
	fmt.Printf("  JSON output: %t\n", GetJSON())

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
