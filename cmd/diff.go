package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/serpro69/gh-arc/internal/codeowners"
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

	// Get current branch
	currentBranch, err := gitRepo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	logger.Info().
		Str("branch", currentBranch).
		Msg("Current branch")

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

	// Step 1: Detect base branch (stacking logic)
	baseDetector := diff.NewBaseBranchDetector(gitRepo, client, &cfg.Diff, repo.Owner, repo.Name)
	baseResult, err := baseDetector.DetectBaseBranch(ctx, currentBranch, diffBase)
	if err != nil {
		return fmt.Errorf("failed to detect base branch: %w", err)
	}

	// Display stacking information
	fmt.Println(baseResult.FormatStackingMessage())

	// Step 2: Check for dependent PRs on current branch
	dependentDetector := diff.NewDependentPRDetector(client, &cfg.Diff, repo.Owner, repo.Name)
	dependentInfo, err := dependentDetector.DetectDependentPRs(ctx, currentBranch)
	if err != nil {
		return fmt.Errorf("failed to detect dependent PRs: %w", err)
	}

	// Display dependent PR warning if applicable
	if dependentInfo.HasDependents && dependentDetector.ShouldShowWarning() {
		fmt.Println(dependentInfo.FormatDependentPRsWarning())
	}

	// Step 3: Analyze commits for template pre-filling
	analysis, err := diff.AnalyzeCommitsForTemplate(gitRepo, baseResult.Base, currentBranch)
	if err != nil {
		return fmt.Errorf("failed to analyze commits: %w", err)
	}

	logger.Debug().
		Int("commitCount", analysis.CommitCount).
		Str("baseBranch", analysis.BaseBranch).
		Msg("Analyzed commits for template")

	// Step 4: Check for existing PR
	existingPR, err := client.FindExistingPRForCurrentBranch(ctx, currentBranch)
	if err != nil {
		return fmt.Errorf("failed to check for existing PR: %w", err)
	}

	// Step 5: Determine workflow path
	needsTemplateEditing := existingPR == nil || diffEdit
	skipEditor := diffNoEdit && needsTemplateEditing

	logger.Debug().
		Bool("existingPR", existingPR != nil).
		Bool("needsTemplateEditing", needsTemplateEditing).
		Bool("skipEditor", skipEditor).
		Msg("Determined workflow path")

	// If existing PR and no --edit flag, just push commits (fast path)
	if existingPR != nil && !diffEdit {
		fmt.Printf("✓ PR #%d already exists, pushing new commits\n", existingPR.Number)
		fmt.Printf("  %s\n", existingPR.HTMLURL)

		// Check if base changed and update if needed
		if github.DetectBaseChanged(existingPR, baseResult.Base) {
			fmt.Printf("\n⚠️  Base branch changed: %s → %s\n", existingPR.Base.Ref, baseResult.Base)
			fmt.Println("   Updating PR base...")

			err := client.UpdatePRBaseForCurrentRepo(ctx, existingPR.Number, baseResult.Base)
			if err != nil {
				return fmt.Errorf("failed to update PR base: %w", err)
			}
			fmt.Println("   ✓ Base branch updated")
		}

		// Push new commits to remote
		fmt.Println("\n✓ Pushing new commits...")
		if err := gitRepo.Push(ctx, currentBranch); err != nil {
			return fmt.Errorf("failed to push commits: %w", err)
		}
		fmt.Println("  ✓ Commits pushed successfully")

		return nil
	}

	// Step 6: Generate and edit template
	var templateContent string
	var parsedFields *template.TemplateFields

	// Check if we're in --continue mode
	var savedTemplatePath string
	if diffContinue {
		logger.Debug().Msg("Continue mode: looking for saved template")

		// Find saved templates
		savedTemplates, err := template.FindSavedTemplates()
		if err != nil || len(savedTemplates) == 0 {
			return fmt.Errorf("no saved template found (use 'gh arc diff --edit' to start fresh): %w", err)
		}

		// Use the most recent saved template
		savedTemplatePath = savedTemplates[len(savedTemplates)-1]
		templateContent, err = template.LoadSavedTemplate(savedTemplatePath)
		if err != nil {
			return fmt.Errorf("failed to load saved template (use 'gh arc diff --edit' to start fresh): %w", err)
		}

		// Open editor to allow fixing validation issues
		if !skipEditor {
			templateContent, err = template.OpenEditor(templateContent)
			if err != nil {
				if err == template.ErrEditorCancelled {
					fmt.Println("✗ Editor cancelled, no changes made")
					return nil
				}
				return fmt.Errorf("failed to open editor: %w", err)
			}
		}
	} else {
		// Generate fresh template
		logger.Debug().Msg("Generating fresh template")

		// Get CODEOWNERS reviewer suggestions
		var reviewerSuggestions []string
		co, err := codeowners.ParseCodeowners(".")
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to parse CODEOWNERS file")
		} else {
			// Get trunk branch for stack-aware analysis
			trunkBranch, err := gitRepo.GetDefaultBranch()
			if err != nil {
				logger.Warn().Err(err).Msg("Failed to get default branch")
				trunkBranch = "main" // Fallback
			}

			// Get stack-aware reviewers based on changed files
			considerFullStack := baseResult.IsStacking
			reviewerSuggestions, err = codeowners.GetStackAwareReviewers(
				gitRepo,
				co,
				currentBranch,
				baseResult.Base,
				trunkBranch,
				currentUser,
				considerFullStack,
			)
			if err != nil {
				logger.Warn().Err(err).Msg("Failed to get stack-aware reviewers")
			}

			logger.Info().
				Int("count", len(reviewerSuggestions)).
				Strs("reviewers", reviewerSuggestions).
				Msg("Generated reviewer suggestions from CODEOWNERS")
		}

		// Determine default draft value for template: flags > config
		templateDraftDefault := cfg.Diff.CreateAsDraft
		if diffDraft {
			templateDraftDefault = true
		} else if diffReady {
			templateDraftDefault = false
		}

		// Create template generator
		gen := template.NewTemplateGenerator(
			&template.StackingContext{
				IsStacking:     baseResult.IsStacking,
				BaseBranch:     baseResult.Base,
				ParentPR:       baseResult.ParentPR,
				DependentPRs:   dependentInfo.DependentPRs,
				CurrentBranch:  currentBranch,
				ShowDependents: dependentInfo.HasDependents,
			},
			analysis,
			reviewerSuggestions,
			cfg.Diff.LinearEnabled,
			templateDraftDefault,
		)

		templateContent = gen.Generate()

		// Open editor unless --no-edit flag is set
		if !skipEditor {
			templateContent, err = template.OpenEditor(templateContent)
			if err != nil {
				if err == template.ErrEditorCancelled {
					fmt.Println("✗ Editor cancelled, no changes made")
					return nil
				}
				return fmt.Errorf("failed to open editor: %w", err)
			}
		}
	}

	// Step 7: Parse and validate template
	parsedFields, err = template.ParseTemplate(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	stackingCtx := &template.StackingContext{
		IsStacking:     baseResult.IsStacking,
		BaseBranch:     baseResult.Base,
		ParentPR:       baseResult.ParentPR,
		DependentPRs:   dependentInfo.DependentPRs,
		CurrentBranch:  currentBranch,
		ShowDependents: dependentInfo.HasDependents,
	}

	valid, validationMessage := template.ValidateFieldsWithContext(parsedFields, cfg.Diff.RequireTestPlan, stackingCtx)
	if !valid {
		fmt.Println(validationMessage)

		// Save template for --continue
		savedPath, err := template.SaveTemplate(templateContent)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to save template for retry")
		} else {
			fmt.Printf("\nTemplate saved to: %s\n", savedPath)
			fmt.Println("Fix the issues and run:")
			fmt.Printf("  gh arc diff --continue\n")
		}

		return fmt.Errorf("template validation failed")
	}

	logger.Debug().
		Str("title", parsedFields.Title).
		Int("reviewers", len(parsedFields.Reviewers)).
		Msg("Template validated successfully")

	// Step 8: Create or update PR
	var pr *github.PullRequest

	// Use draft status from template (flags were already applied when generating template)
	isDraft := parsedFields.Draft

	// Build PR title and body from template
	prTitle := parsedFields.Title

	// Append Linear Ref to title if Linear is enabled and Ref values exist
	if cfg.Diff.LinearEnabled && len(parsedFields.Ref) > 0 {
		refStr := strings.Join(parsedFields.Ref, ", ")
		prTitle = fmt.Sprintf("%s [%s]", prTitle, refStr)
	}

	// Build PR body
	prBody := parsedFields.Summary
	if parsedFields.TestPlan != "" {
		prBody += "\n\n## Test Plan\n" + parsedFields.TestPlan
	}
	if len(parsedFields.Ref) > 0 {
		prBody += "\n\n**Ref:** " + parsedFields.Ref[0]
	}

	if existingPR != nil {
		// Update existing PR
		fmt.Printf("\n✓ Updating PR #%d...\n", existingPR.Number)

		draftPtr := &isDraft
		pr, err = client.UpdatePullRequestForCurrentRepo(
			ctx,
			existingPR.Number,
			prTitle,
			prBody,
			draftPtr,
			baseResult.ParentPR,
		)
		if err != nil {
			return fmt.Errorf("failed to update PR: %w", err)
		}

		fmt.Printf("  %s\n", pr.HTMLURL)
	} else {
		// Create new PR
		fmt.Println("\n✓ Creating new PR...")

		// Push branch to remote first so GitHub can find it
		fmt.Println("  Pushing branch to remote...")
		if err := gitRepo.Push(ctx, currentBranch); err != nil {
			return fmt.Errorf("failed to push branch: %w", err)
		}

		pr, err = client.CreatePullRequestForCurrentRepo(
			ctx,
			prTitle,
			currentBranch,
			baseResult.Base,
			prBody,
			isDraft,
			baseResult.ParentPR,
		)
		if err != nil {
			return fmt.Errorf("failed to create PR: %w", err)
		}

		fmt.Printf("  PR #%d: %s\n", pr.Number, pr.HTMLURL)
	}

	// Step 9: Assign reviewers
	if len(parsedFields.Reviewers) > 0 {
		fmt.Println("\n✓ Assigning reviewers...")

		// Parse reviewers into users and teams
		assignment := github.ParseReviewers(parsedFields.Reviewers)

		// Filter out current user
		filteredUsers := []string{}
		for _, user := range assignment.Users {
			if !strings.EqualFold(user, currentUser) {
				filteredUsers = append(filteredUsers, user)
			}
		}
		assignment.Users = filteredUsers

		// Assign reviewers
		if len(assignment.Users) > 0 || len(assignment.Teams) > 0 {
			err = client.AssignReviewersForCurrentRepo(ctx, pr.Number, assignment.Users, assignment.Teams)
			if err != nil {
				// Log warning but don't fail the entire operation
				logger.Warn().
					Err(err).
					Int("pr", pr.Number).
					Msg("Failed to assign some reviewers")
				fmt.Printf("  ⚠️  Warning: %v\n", err)
			} else {
				fmt.Println("  " + github.FormatReviewerAssignment(assignment, baseResult.ParentPR, false))
			}
		}
	}

	// Step 10: Clean up saved template on success
	if diffContinue && savedTemplatePath != "" {
		_ = template.RemoveSavedTemplate(savedTemplatePath)
	}

	// Display final success message
	fmt.Println("\n✓ Success!")
	if isDraft {
		fmt.Println("  PR created as draft")
	}
	if baseResult.IsStacking {
		fmt.Printf("  Stacked on: %s\n", baseResult.Base)
	}

	return nil
}
