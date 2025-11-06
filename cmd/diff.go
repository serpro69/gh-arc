package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/diff"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/github"
	"github.com/serpro69/gh-arc/internal/logger"
	"github.com/serpro69/gh-arc/internal/template"
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
	// Create context with timeout to prevent hanging on slow operations
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	logger.Debug().
		Bool("draft", diffDraft).
		Bool("ready", diffReady).
		Bool("edit", diffEdit).
		Bool("no-edit", diffNoEdit).
		Bool("continue", diffContinue).
		Str("base", diffBase).
		Msg("Starting diff command")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get current repository
	repo, err := repository.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current repository: %w", err)
	}

	logger.Info().
		Str("owner", repo.Owner).
		Str("repo", repo.Name).
		Msg("Repository detected")

	// Open git repository
	gitRepo, err := git.OpenRepository(".")
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}

	// Create GitHub client
	client, err := github.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Create output formatter
	output := diff.NewOutputStyle(true) // TODO: Make color configurable

	// Create diff workflow
	workflow := diff.NewDiffWorkflow(gitRepo, client, cfg, repo.Owner, repo.Name)

	// Execute workflow
	result, err := workflow.Execute(ctx, &diff.DiffOptions{
		Draft:    diffDraft,
		Ready:    diffReady,
		Edit:     diffEdit,
		NoEdit:   diffNoEdit,
		Continue: diffContinue,
		Base:     diffBase,
	})
	if err != nil {
		// Check for specific error types to provide better error messages
		if errors.Is(err, diff.ErrOperationCancelled) {
			fmt.Println("\n✗ Cannot create PR from main to main.")
			fmt.Println("Please create a feature branch manually:")
			fmt.Printf("  gh arc work feature/my-branch\n")
			fmt.Printf("  gh arc diff\n")
			return fmt.Errorf("operation cancelled")
		}

		if errors.Is(err, diff.ErrStaleRemote) {
			fmt.Println("\n✗ Operation aborted due to stale remote tracking branch.")
			fmt.Println("Please update your local repository:")
			fmt.Printf("  git fetch origin\n")
			fmt.Printf("  gh arc diff\n")
			return fmt.Errorf("stale remote")
		}

		if errors.Is(err, git.ErrAuthenticationFailed) {
			fmt.Println("\n✗ Authentication failed")
			fmt.Println("Please refresh your GitHub authentication:")
			fmt.Printf("  gh auth refresh --scopes \"repo,read:user\"\n")
			fmt.Printf("  gh arc diff\n")
			return fmt.Errorf("authentication failed: %w", err)
		}

		if errors.Is(err, template.ErrEditorCancelled) {
			fmt.Println("✗ Editor cancelled, no changes made")
			return nil
		}

		// Generic error - check if it's a validation error
		if strings.Contains(err.Error(), "Template validation failed") {
			// Validation error already formatted nicely
			fmt.Println("\n" + err.Error())
			return fmt.Errorf("template validation failed")
		}

		// Other errors
		return err
	}

	// Display results
	fmt.Println()
	fmt.Println(diff.FormatDiffResult(result, output))

	return nil
}
