package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
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

	// Create GitHub client (needed for both continue and normal flows)
	client, err := github.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// CONTINUE MODE: Skip all analysis and just use saved template
	if diffContinue {
		logger.Debug().Msg("Continue mode: loading saved template and skipping analysis")

		// Find and load saved template
		savedTemplates, err := template.FindSavedTemplates()
		if err != nil {
			return fmt.Errorf("failed to find saved template (use 'gh arc diff --edit' to start fresh): %w", err)
		} else if len(savedTemplates) == 0 {
			return fmt.Errorf("no saved template found (use 'gh arc diff --edit' to start fresh)")
		}

		savedTemplatePath := savedTemplates[len(savedTemplates)-1]
		templateContent, err := template.LoadSavedTemplate(savedTemplatePath)
		if err != nil {
			return fmt.Errorf("failed to load saved template (use 'gh arc diff --edit' to start fresh): %w", err)
		}

		// Extract branch information from template
		prHeadBranch, prBaseBranch, found := template.ExtractBranchInfo(templateContent)
		if !found {
			return fmt.Errorf("failed to extract branch info from template (template may be corrupted)")
		}

		logger.Debug().
			Str("headBranch", prHeadBranch).
			Str("baseBranch", prBaseBranch).
			Msg("Extracted branch info from saved template")

		// Open editor to allow fixing validation issues (unless --no-edit)
		if !diffNoEdit {
			templateContent, err = template.OpenEditor(templateContent)
			if err != nil {
				if err == template.ErrEditorCancelled {
					fmt.Println("✗ Editor cancelled, no changes made")
					return nil
				}
				return fmt.Errorf("failed to open editor: %w", err)
			}
		}

		// Parse and validate template
		parsedFields, err := template.ParseTemplate(templateContent)
		if err != nil {
			return fmt.Errorf("failed to parse template: %w", err)
		}

		// Validate required fields (no stacking context in continue mode)
		validationErrors := template.ValidateFields(parsedFields, cfg.Diff.RequireTestPlan, nil)
		if len(validationErrors) > 0 {
			fmt.Println("\n✗ Template validation failed:")
			for _, errMsg := range validationErrors {
				fmt.Printf("  • %s\n", errMsg)
			}
			fmt.Println("\nTemplate saved. Fix the issues and run:")
			fmt.Println("  gh arc diff --continue")
			return fmt.Errorf("template validation failed")
		}

		// Delete saved template after successful validation
		if err := os.Remove(savedTemplatePath); err != nil {
			logger.Warn().Err(err).Msg("Failed to remove saved template")
		}

		// Check for existing PR
		existingPR, err := client.FindExistingPR(ctx, repo.Owner, repo.Name, prHeadBranch)
		if err != nil {
			return fmt.Errorf("failed to check for existing PR: %w", err)
		}

		// Create or update PR
		if existingPR != nil {
			fmt.Printf("\n✓ Found existing PR #%d\n", existingPR.Number)
			fmt.Printf("  Updating PR with new information...\n")

			// Determine draft status pointer for update
			var draftPtr *bool
			if parsedFields.Draft != existingPR.Draft {
				draftPtr = &parsedFields.Draft
			}

			// Update PR
			updatedPR, err := client.UpdatePullRequest(
				ctx,
				repo.Owner, repo.Name,
				existingPR.Number,
				parsedFields.Title,
				parsedFields.Summary,
				draftPtr,
				nil, // No parent PR in continue mode
			)
			if err != nil {
				return fmt.Errorf("failed to update PR: %w", err)
			}

			// Handle draft status transitions
			if parsedFields.Draft && !existingPR.Draft {
				_, err := client.ConvertPRToDraft(ctx, repo.Owner, repo.Name, existingPR)
				if err != nil {
					logger.Warn().Err(err).Msg("Failed to convert PR to draft")
				} else {
					fmt.Println("  ✓ Converted PR to draft")
				}
			} else if !parsedFields.Draft && existingPR.Draft {
				_, err := client.MarkPRReadyForReview(ctx, repo.Owner, repo.Name, existingPR)
				if err != nil {
					logger.Warn().Err(err).Msg("Failed to mark PR as ready")
				} else {
					fmt.Println("  ✓ Marked PR as ready for review")
				}
			}

			// Assign reviewers (parse users and teams)
			if len(parsedFields.Reviewers) > 0 {
				assignment := github.ParseReviewers(parsedFields.Reviewers)
				err := client.AssignReviewers(
					ctx,
					repo.Owner, repo.Name,
					existingPR.Number,
					assignment.Users, assignment.Teams,
				)
				if err != nil {
					logger.Warn().Err(err).Msg("Failed to assign reviewers")
				} else {
					fmt.Printf("  ✓ Assigned reviewers: %s\n", strings.Join(parsedFields.Reviewers, ", "))
				}
			}

			fmt.Printf("\n✓ PR updated: %s\n", updatedPR.HTMLURL)
		} else {
			fmt.Println("\nCreating new PR...")

			// Create PR
			newPR, err := client.CreatePullRequest(
				ctx,
				repo.Owner, repo.Name,
				parsedFields.Title,
				prHeadBranch,
				prBaseBranch,
				parsedFields.Summary,
				parsedFields.Draft,
				nil, // No parent PR in continue mode
			)
			if err != nil {
				return fmt.Errorf("failed to create PR: %w", err)
			}

			// Assign reviewers (parse users and teams)
			if len(parsedFields.Reviewers) > 0 {
				assignment := github.ParseReviewers(parsedFields.Reviewers)
				err := client.AssignReviewers(
					ctx,
					repo.Owner, repo.Name,
					newPR.Number,
					assignment.Users, assignment.Teams,
				)
				if err != nil {
					logger.Warn().Err(err).Msg("Failed to assign reviewers")
				} else {
					fmt.Printf("  ✓ Assigned reviewers: %s\n", strings.Join(parsedFields.Reviewers, ", "))
				}
			}

			fmt.Printf("\n✓ PR created: %s\n", newPR.HTMLURL)
		}

		return nil
	}

	// NORMAL MODE: Do full analysis and template generation
	logger.Debug().Msg("Normal mode: performing full analysis")

	// Auto-branch detection: Check if user has commits on main/master
	var autoBranchContext *diff.AutoBranchContext
	autoBranchDetector := diff.NewAutoBranchDetector(gitRepo, &cfg.Diff)
	detection, err := autoBranchDetector.DetectCommitsOnMain(ctx)
	if err != nil {
		return fmt.Errorf("failed to detect commits on main: %w", err)
	}

	// If user has commits on main, prepare auto-branch creation
	if autoBranchDetector.ShouldAutoBranch(detection) {
		fmt.Printf("\n⚠️  Warning: You have %d commit(s) on %s\n", detection.CommitsAhead, detection.DefaultBranch)

		autoBranchContext, err = autoBranchDetector.PrepareAutoBranch(ctx, detection)
		if err != nil {
			// Check for user cancellation
			if errors.Is(err, diff.ErrOperationCancelled) {
				fmt.Println("\n✗ Cannot create PR from main to main.")
				fmt.Println("Please create a feature branch manually:")
				fmt.Printf("  gh arc work feature/my-branch\n")
				fmt.Printf("  gh arc diff\n")
				return fmt.Errorf("operation cancelled")
			}

			// Check for stale remote rejection
			if errors.Is(err, diff.ErrStaleRemote) {
				fmt.Println("\n✗ Operation aborted due to stale remote tracking branch.")
				fmt.Println("Please update your local repository:")
				fmt.Printf("  git fetch origin\n")
				fmt.Printf("  gh arc diff\n")
				return fmt.Errorf("stale remote")
			}

			// Other errors
			return fmt.Errorf("auto-branch preparation failed: %w", err)
		}

		fmt.Printf("✓ Will create feature branch: %s\n\n", autoBranchContext.BranchName)
	}

	// Determine the PR head branch name (may differ from current git branch for auto-branch flow)
	prHeadBranch := currentBranch
	if autoBranchContext != nil {
		prHeadBranch = autoBranchContext.BranchName
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
	// Always use remote tracking branch as base to capture all unpushed commits
	// This handles both auto-branch scenarios and normal feature branches
	commitAnalysisBase := "origin/" + baseResult.Base

	analysis, err := diff.AnalyzeCommitsForTemplate(gitRepo, commitAnalysisBase, currentBranch)
	if err != nil {
		return fmt.Errorf("failed to analyze commits: %w", err)
	}

	logger.Debug().
		Int("commitCount", analysis.CommitCount).
		Str("baseBranch", analysis.BaseBranch).
		Msg("Analyzed commits for template")

	// Step 4: Check for existing PR
	existingPR, err := client.FindExistingPRForCurrentBranch(ctx, prHeadBranch)
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

	// If existing PR and no --edit flag, handle fast path (push commits and/or update draft status)
	if existingPR != nil && !diffEdit {
		fmt.Printf("✓ PR #%d already exists\n", existingPR.Number)
		fmt.Printf("  %s\n", existingPR.HTMLURL)

		// Debug: Log PR draft status
		logger.Debug().
			Bool("prIsDraft", existingPR.Draft).
			Bool("readyFlag", diffReady).
			Bool("draftFlag", diffDraft).
			Msg("Current PR status and flags")

		// Check for unpushed commits
		hasUnpushed, err := gitRepo.HasUnpushedCommits(currentBranch)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to check for unpushed commits")
			hasUnpushed = true // Assume unpushed on error to be safe
		}

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

		// Handle draft status changes from flags
		if diffReady && existingPR.Draft {
			// Mark draft PR as ready for review
			fmt.Println("\n✓ Marking PR as ready for review...")
			logger.Debug().Msg("Will update PR from draft to ready using GraphQL")

			updatedPR, err := client.MarkPRReadyForReviewForCurrentRepo(ctx, existingPR)
			if err != nil {
				logger.Error().
					Err(err).
					Int("prNumber", existingPR.Number).
					Msg("Failed to mark PR as ready")
				return fmt.Errorf("failed to mark PR as ready: %w", err)
			}

			logger.Debug().
				Int("prNumber", updatedPR.Number).
				Bool("resultDraft", updatedPR.Draft).
				Msg("PR marked as ready successfully")

			fmt.Println("  ✓ PR status updated")
		} else if diffDraft && !existingPR.Draft {
			// Convert ready PR to draft using GraphQL
			fmt.Println("\n✓ Converting PR to draft...")
			logger.Debug().Msg("Will convert PR from ready to draft using GraphQL")

			updatedPR, err := client.ConvertPRToDraftForCurrentRepo(ctx, existingPR)
			if err != nil {
				logger.Error().
					Err(err).
					Int("prNumber", existingPR.Number).
					Msg("Failed to convert PR to draft")
				return fmt.Errorf("failed to convert PR to draft: %w", err)
			}

			logger.Debug().
				Int("prNumber", updatedPR.Number).
				Bool("resultDraft", updatedPR.Draft).
				Msg("PR converted to draft successfully")

			fmt.Println("  ✓ PR status updated")
		} else {
			logger.Debug().
				Bool("diffReady", diffReady).
				Bool("existingPRDraft", existingPR.Draft).
				Bool("diffDraft", diffDraft).
				Msg("No draft status update needed")
		}

		// Push new commits if they exist
		if hasUnpushed {
			fmt.Println("\n✓ Pushing new commits...")
			if err := gitRepo.Push(ctx, currentBranch); err != nil {
				return fmt.Errorf("failed to push commits: %w", err)
			}
			fmt.Println("  ✓ Commits pushed successfully")
		} else if !diffReady && !diffDraft {
			// Only show "no new commits" if we're not updating draft status
			fmt.Println("\n✓ No new commits to push")
		}

		return nil
	}

	// Step 6: Generate and edit template
	var templateContent string
	var parsedFields *template.TemplateFields

	// Generate fresh template
	logger.Debug().Msg("Generating fresh template")

	var reviewerSuggestions []string

	// Get CODEOWNERS reviewer suggestions
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

	// Append default reviewers from config, if any
	reviewerSuggestions = append(reviewerSuggestions, cfg.GitHub.DefaultReviewers...)
	logger.Info().
		Int("count", len(cfg.GitHub.DefaultReviewers)).
		Strs("reviewers", reviewerSuggestions).
		Msg("Appended defaultReviewers from configuration")

	// Remove duplicates from final reviewers array
	reviewerSuggestions = codeowners.DeduplicateReviewers(reviewerSuggestions)

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
			CurrentBranch:  prHeadBranch,
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
		CurrentBranch:  prHeadBranch,
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

		// Handle draft→ready transition specially (requires GraphQL)
		if existingPR.Draft && !isDraft {
			// First, update title/body/etc with PATCH (keep as draft)
			if prTitle != "" || prBody != "" {
				logger.Debug().Msg("Updating PR title/body before marking ready")
				tempDraft := true
				_, err = client.UpdatePullRequestForCurrentRepo(
					ctx,
					existingPR.Number,
					prTitle,
					prBody,
					&tempDraft,
					baseResult.ParentPR,
				)
				if err != nil {
					return fmt.Errorf("failed to update PR metadata: %w", err)
				}
			}

			// Then mark as ready using GraphQL
			logger.Debug().Msg("Marking PR as ready for review using GraphQL")
			pr, err = client.MarkPRReadyForReviewForCurrentRepo(ctx, existingPR)
			if err != nil {
				return fmt.Errorf("failed to mark PR as ready: %w", err)
			}
		} else if !existingPR.Draft && isDraft {
			// Handle ready→draft transition (requires GraphQL)
			// First, update title/body/etc with PATCH (keep as ready)
			if prTitle != "" || prBody != "" {
				logger.Debug().Msg("Updating PR title/body before converting to draft")
				tempDraft := false
				_, err = client.UpdatePullRequestForCurrentRepo(
					ctx,
					existingPR.Number,
					prTitle,
					prBody,
					&tempDraft,
					baseResult.ParentPR,
				)
				if err != nil {
					return fmt.Errorf("failed to update PR metadata: %w", err)
				}
			}

			// Then convert to draft using GraphQL
			logger.Debug().Msg("Converting PR to draft using GraphQL")
			pr, err = client.ConvertPRToDraftForCurrentRepo(ctx, existingPR)
			if err != nil {
				return fmt.Errorf("failed to convert PR to draft: %w", err)
			}
		} else {
			// Normal update (no draft status change)
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
		}

		fmt.Printf("  %s\n", pr.HTMLURL)
	} else {
		// Create new PR
		fmt.Println("\n✓ Creating new PR...")

		// Push branch to remote first so GitHub can find it
		fmt.Println("  Pushing branch to remote...")

		// Check if auto-branch flow is active
		if autoBranchContext != nil {
			// Auto-branch push with collision handling
			originalBranchName := autoBranchContext.BranchName
			maxAttempts := 3
			var pushErr error

			for attempt := 1; attempt <= maxAttempts; attempt++ {
				pushErr = gitRepo.PushBranch(ctx, "HEAD", autoBranchContext.BranchName)

				if pushErr == nil {
					// Success - display message
					fmt.Printf("  ✓ Pushed branch '%s' to remote\n", autoBranchContext.BranchName)

					// If branch name changed due to collision resolution
					if autoBranchContext.BranchName != originalBranchName {
						fmt.Println("  (Resolved branch name collision by appending counter)")
					}

					// Update branch variables for PR creation and subsequent operations
					prHeadBranch = autoBranchContext.BranchName
					currentBranch = autoBranchContext.BranchName
					break
				}

				// Check error type
				if errors.Is(pushErr, git.ErrRemoteBranchExists) {
					// Branch name collision - regenerate and retry
					logger.Warn().
						Str("branchName", autoBranchContext.BranchName).
						Int("attempt", attempt).
						Msg("Branch name collision detected")

					if attempt < maxAttempts {
						fmt.Printf("  ⚠️  Branch name collision, generating new name (attempt %d/%d)...\n", attempt, maxAttempts)

						// Generate new unique name
						newName, err := autoBranchDetector.EnsureUniqueBranchName(originalBranchName)
						if err != nil {
							return fmt.Errorf("failed to generate unique branch name: %w", err)
						}

						autoBranchContext.BranchName = newName
						logger.Debug().
							Str("newBranchName", newName).
							Msg("Generated new branch name after collision")
					}
				} else if errors.Is(pushErr, git.ErrAuthenticationFailed) {
					// Authentication error - show recovery instructions
					fmt.Println("\n✗ Authentication failed when pushing branch")
					fmt.Println("Please refresh your GitHub authentication:")
					fmt.Printf("  gh auth refresh --scopes \"repo,read:user\"\n")
					fmt.Printf("  gh arc diff\n")
					return fmt.Errorf("authentication failed: %w", pushErr)
				} else {
					// Generic error - break retry loop
					break
				}
			}

			// If we exhausted retries or hit non-retryable error
			if pushErr != nil {
				fmt.Printf("\n✗ Failed to push branch after %d attempts\n", maxAttempts)
				fmt.Println("Manual recovery steps:")
				fmt.Printf("  1. Create branch manually: git checkout -b %s\n", autoBranchContext.BranchName)
				fmt.Printf("  2. Push branch: git push origin %s\n", autoBranchContext.BranchName)
				fmt.Printf("  3. Create PR: gh arc diff\n")
				return fmt.Errorf("failed to push branch: %w", pushErr)
			}
		} else {
			// Normal push flow (non-auto-branch)
			if err := gitRepo.Push(ctx, currentBranch); err != nil {
				return fmt.Errorf("failed to push branch: %w", err)
			}
		}

		pr, err = client.CreatePullRequestForCurrentRepo(
			ctx,
			prTitle,
			prHeadBranch,
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

	// Step 10: Checkout auto-branch tracking branch (if applicable)
	if autoBranchContext != nil {
		fmt.Println("\n✓ Checking out feature branch...")

		err = gitRepo.CheckoutTrackingBranch(autoBranchContext.BranchName, "origin/"+autoBranchContext.BranchName)
		if err != nil {
			// Checkout failed, but don't fail the entire operation
			// User can manually checkout the branch
			logger.Warn().
				Err(err).
				Str("branchName", autoBranchContext.BranchName).
				Msg("Failed to checkout tracking branch")

			fmt.Printf("  ⚠️  Failed to checkout branch '%s'\n", autoBranchContext.BranchName)
			fmt.Println("  Manual checkout:")
			fmt.Printf("    git checkout %s\n", autoBranchContext.BranchName)
		} else {
			fmt.Printf("  ✓ Switched to feature branch '%s'\n", autoBranchContext.BranchName)

			// Display info about main branch still being ahead
			fmt.Printf("\n  Note: You are now on branch '%s'\n", autoBranchContext.BranchName)
			fmt.Printf("        The '%s' branch may still have unpushed commits.\n", detection.DefaultBranch)
			fmt.Printf("        To clean up %s, run: git checkout %s && git reset --hard origin/%s\n",
				detection.DefaultBranch, detection.DefaultBranch, detection.DefaultBranch)
		}
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
