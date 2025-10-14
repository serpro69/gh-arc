package cmd

import (
	"context"
	"fmt"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/serpro69/gh-arc/internal/github"
	"github.com/serpro69/gh-arc/internal/logger"
)

var (
	// diffCmd flags
	diffDraft    bool
	diffReady    bool
	diffEdit     bool
	diffNoEdit   bool
	diffContinue bool
	diffBase     string
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Submit code for review by creating or updating a Pull Request",
	Long: `Submit code for review by creating or updating a GitHub Pull Request.

Opens your $EDITOR with a structured template to collect PR metadata including:
  - Title (pre-filled from commit messages)
  - Summary (generated from commits)
  - Test Plan (required by default)
  - Reviewers (suggestions from CODEOWNERS)
  - Ref (Linear issue references)

Stacked PR Support:
  gh-arc automatically detects when your branch should stack on another feature
  branch with an existing PR, enabling trunk-based development workflows.

Workflow:
  1. No existing PR → Opens template editor → Creates new PR
  2. Existing PR, no --edit → Just pushes new commits (fast path)
  3. Existing PR with --edit → Opens template → Updates PR metadata

Authentication Requirements:
  This command requires GitHub authentication with the following OAuth scopes:
  - repo (full control of private repositories)
  - read:user (to read user profile data)

  If you encounter authentication errors, refresh your token:
    gh auth refresh --scopes "repo,read:user"

Examples:
  # Create a new PR (opens editor for metadata)
  gh arc diff

  # Update existing PR with new commits (no editor)
  gh arc diff

  # Force template editing to update PR metadata
  gh arc diff --edit

  # Create a draft PR
  gh arc diff --draft

  # Skip editor and accept pre-filled template
  gh arc diff --no-edit

  # Override base branch (break out of stack)
  gh arc diff --base main

  # Create stacked PR on specific branch
  gh arc diff --base feature/parent-branch

  # Retry editing after validation error
  gh arc diff --continue

Stacking Examples:
  # Create base PR
  git checkout -b feature/auth
  gh arc diff  # → PR: feature/auth → main

  # Create stacked PR (automatic detection)
  git checkout -b feature/auth-tests
  gh arc diff  # → Detects stacking: feature/auth-tests → feature/auth

  # Update parent PR (warns about dependent PRs)
  git checkout feature/auth
  gh arc diff --edit  # → Warning: 1 dependent PR targets this branch`,
	RunE: runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)

	// Define command-specific flags
	diffCmd.Flags().BoolVar(&diffDraft, "draft", false, "Create or update as draft PR")
	diffCmd.Flags().BoolVar(&diffReady, "ready", false, "Create or update as ready-for-review PR")
	diffCmd.Flags().BoolVar(&diffEdit, "edit", false, "Force template editing even when PR exists (regenerate from commits and update PR metadata)")
	diffCmd.Flags().BoolVar(&diffNoEdit, "no-edit", false, "Skip editor and accept pre-filled template as-is (only for new PRs or with --edit)")
	diffCmd.Flags().BoolVar(&diffContinue, "continue", false, "Retry template editing after validation failure")
	diffCmd.Flags().StringVar(&diffBase, "base", "", "Override detected base branch for stacking (e.g., --base=main)")

	// Mark mutually exclusive flags
	diffCmd.MarkFlagsMutuallyExclusive("draft", "ready")
	diffCmd.MarkFlagsMutuallyExclusive("no-edit", "edit")
}

// runDiff executes the diff command
func runDiff(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	logger.Debug().
		Bool("draft", diffDraft).
		Bool("ready", diffReady).
		Bool("edit", diffEdit).
		Bool("no-edit", diffNoEdit).
		Bool("continue", diffContinue).
		Str("base", diffBase).
		Msg("Starting diff command")

	// Get current repository
	repo, err := repository.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current repository: %w", err)
	}

	logger.Info().
		Str("owner", repo.Owner).
		Str("repo", repo.Name).
		Msg("Repository detected")

	// Create GitHub client
	client, err := github.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Get current user for metadata
	currentUser, err := client.GetCurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	logger.Debug().
		Str("user", currentUser).
		Msg("Authenticated user")

	// TODO: Implement diff workflow
	// 1. Detect base branch (stacking logic)
	// 2. Check for existing PR
	// 3. Determine workflow path (simple push vs template editing)
	// 4. Execute appropriate workflow

	return fmt.Errorf("diff command not yet implemented - this is just the command structure")
}
