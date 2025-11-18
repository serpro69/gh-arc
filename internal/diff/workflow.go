package diff

import (
	"context"
	"errors"
	"fmt"

	"github.com/serpro69/gh-arc/internal/codeowners"
	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/github"
	"github.com/serpro69/gh-arc/internal/logger"
	"github.com/serpro69/gh-arc/internal/template"
)

// DiffWorkflow orchestrates the entire diff command workflow.
// It coordinates between auto-branch detection, base branch detection,
// template generation/editing, and PR creation/updates.
type DiffWorkflow struct {
	repo              *git.Repository
	client            *github.Client
	config            *config.Config
	owner             string
	name              string
	autoBranchDetector *AutoBranchDetector
	baseDetector      *BaseBranchDetector
	dependentDetector *DependentPRDetector
	continueExecutor  *ContinueModeExecutor
	prExecutor        *PRExecutor
}

// DiffOptions contains all options for the diff command
type DiffOptions struct {
	Draft    bool
	Ready    bool
	Edit     bool
	NoEdit   bool
	Continue bool
	Base     string
}

// DiffResult contains the complete results of the diff workflow
type DiffResult struct {
	PR                 *github.PullRequest
	WasCreated         bool
	DraftChanged       bool
	AutoBranchUsed     bool
	AutoBranchName     string
	AutoBranchCheckoutFailed bool
	IsStacking         bool
	BaseBranch         string
	ParentPR           *github.PullRequest
	ReviewersAdded     []string
	Messages           []string
}

// NewDiffWorkflow creates a new diff workflow orchestrator
func NewDiffWorkflow(repo *git.Repository, client *github.Client, cfg *config.Config, owner, name string) *DiffWorkflow {
	return &DiffWorkflow{
		repo:              repo,
		client:            client,
		config:            cfg,
		owner:             owner,
		name:              name,
		autoBranchDetector: NewAutoBranchDetector(repo, &cfg.Diff),
		baseDetector:      NewBaseBranchDetector(repo, client, &cfg.Diff, owner, name),
		dependentDetector: NewDependentPRDetector(client, &cfg.Diff, owner, name),
		continueExecutor:  NewContinueModeExecutor(repo, client, &cfg.Diff, owner, name),
		prExecutor:        NewPRExecutor(client, repo, owner, name),
	}
}

// Execute runs the diff workflow based on the provided options.
// This is the main entry point that routes to the appropriate sub-workflow:
//
// 1. Continue Mode (--continue flag):
//    - Loads saved template from previous validation failure
//    - Allows user to fix errors and retry
//    - See: executeContinueMode()
//
// 2. Fast Path (existing PR, no --edit flag):
//    - Pushes new commits if any
//    - Updates draft status if flags provided
//    - Updates base branch if changed
//    - See: executeFastPath()
//
// 3. Normal Mode (new PR or --edit flag):
//    - Full template generation and editing flow
//    - Handles auto-branch creation if on main
//    - Creates or updates PR with full metadata
//    - See: executeWithTemplateEditing()
func (w *DiffWorkflow) Execute(ctx context.Context, opts *DiffOptions) (*DiffResult, error) {
	logger.Debug().
		Bool("continue", opts.Continue).
		Bool("edit", opts.Edit).
		Bool("draft", opts.Draft).
		Bool("ready", opts.Ready).
		Str("base", opts.Base).
		Msg("Starting diff workflow execution")

	// Route to continue mode if --continue flag is set
	if opts.Continue {
		return w.executeContinueMode(ctx, opts)
	}

	// Get current branch for all other workflows
	currentBranch, err := w.repo.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	logger.Debug().Str("currentBranch", currentBranch).Msg("Current branch detected")

	// Normal mode: perform full analysis and workflow
	return w.executeNormalMode(ctx, opts, currentBranch)
}

// executeContinueMode handles the --continue workflow
func (w *DiffWorkflow) executeContinueMode(ctx context.Context, opts *DiffOptions) (*DiffResult, error) {
	logger.Debug().Msg("Executing continue mode workflow")

	// Get current branch
	currentBranch, err := w.repo.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Execute continue mode
	result, err := w.continueExecutor.Execute(ctx, &ContinueModeOptions{
		CurrentBranch:   currentBranch,
		NoEdit:          opts.NoEdit,
		RequireTestPlan: w.config.Diff.RequireTestPlan,
		Draft:           opts.Draft,
		Ready:           opts.Ready,
	})
	if err != nil {
		return nil, err
	}

	// Build diff result
	return &DiffResult{
		PR:             result.PR,
		WasCreated:     result.WasCreated,
		DraftChanged:   false, // Continue mode doesn't track draft changes separately
		AutoBranchUsed: false,
		IsStacking:     false, // Continue mode doesn't provide stacking info
		BaseBranch:     result.BaseBranch,
		ReviewersAdded: result.ParsedFields.Reviewers,
		Messages:       result.Messages,
	}, nil
}

// executeNormalMode handles the full diff workflow with optional auto-branching
func (w *DiffWorkflow) executeNormalMode(ctx context.Context, opts *DiffOptions, currentBranch string) (*DiffResult, error) {
	logger.Debug().Msg("Executing normal mode workflow")

	// Step 1: Auto-branch detection and preparation
	autoBranchCtx, detection, err := w.handleAutoBranch(ctx)
	if err != nil {
		return nil, err
	}

	// Determine the PR head branch name (may differ from current git branch for auto-branch flow)
	prHeadBranch := currentBranch
	if autoBranchCtx != nil {
		prHeadBranch = autoBranchCtx.BranchName
	}

	// Step 2: Detect base branch (stacking logic)
	baseResult, err := w.baseDetector.DetectBaseBranch(ctx, currentBranch, opts.Base)
	if err != nil {
		return nil, fmt.Errorf("failed to detect base branch: %w", err)
	}

	logger.Debug().
		Str("baseBranch", baseResult.Base).
		Bool("isStacking", baseResult.IsStacking).
		Msg("Base branch detected")

	// Step 3: Detect dependent PRs on current branch
	dependentInfo, err := w.dependentDetector.DetectDependentPRs(ctx, currentBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to detect dependent PRs: %w", err)
	}

	// Step 4: Check for existing PR
	existingPR, err := w.client.FindExistingPRForCurrentBranch(ctx, prHeadBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing PR: %w", err)
	}

	// Step 5: Route to fast path or normal path
	if existingPR != nil && !opts.Edit {
		return w.executeFastPath(ctx, opts, existingPR, baseResult, currentBranch)
	}

	return w.executeWithTemplateEditing(ctx, opts, existingPR, baseResult, dependentInfo, currentBranch, prHeadBranch, autoBranchCtx, detection)
}

// handleAutoBranch handles auto-branch detection and preparation
func (w *DiffWorkflow) handleAutoBranch(ctx context.Context) (*AutoBranchContext, *DetectionResult, error) {
	// Detect if user has commits on main
	detection, err := w.autoBranchDetector.DetectCommitsOnMain(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to detect commits on main: %w", err)
	}

	// Check if auto-branch flow should be activated
	if !w.autoBranchDetector.ShouldAutoBranch(detection) {
		return nil, detection, nil
	}

	// Prepare auto-branch context
	autoBranchCtx, err := w.autoBranchDetector.PrepareAutoBranch(ctx, detection)
	if err != nil {
		return nil, detection, err
	}

	logger.Debug().
		Str("branchName", autoBranchCtx.BranchName).
		Msg("Auto-branch context prepared")

	return autoBranchCtx, detection, nil
}

// executeFastPath handles existing PR without template editing
func (w *DiffWorkflow) executeFastPath(ctx context.Context, opts *DiffOptions, existingPR *github.PullRequest, baseResult *BaseBranchResult, currentBranch string) (*DiffResult, error) {
	logger.Debug().
		Int("prNumber", existingPR.Number).
		Msg("Executing fast path for existing PR")

	result := &DiffResult{
		PR:           existingPR,
		WasCreated:   false,
		DraftChanged: false,
		IsStacking:   baseResult.IsStacking,
		BaseBranch:   baseResult.Base,
		ParentPR:     baseResult.ParentPR,
		Messages:     []string{},
	}

	// Check for unpushed commits
	hasUnpushed, err := w.repo.HasUnpushedCommits(currentBranch)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to check for unpushed commits")
		hasUnpushed = true // Assume unpushed on error
	}

	// Push new commits if they exist
	if hasUnpushed {
		if err := w.repo.Push(ctx, currentBranch); err != nil {
			return nil, fmt.Errorf("failed to push commits: %w", err)
		}
		result.Messages = append(result.Messages, "Pushed new commits")
	}

	// Check if base changed and update if needed
	if github.DetectBaseChanged(existingPR, baseResult.Base) {
		err := w.client.UpdatePRBaseForCurrentRepo(ctx, existingPR.Number, baseResult.Base)
		if err != nil {
			return nil, fmt.Errorf("failed to update PR base: %w", err)
		}
		result.Messages = append(result.Messages, fmt.Sprintf("Updated base branch: %s → %s", existingPR.Base.Ref, baseResult.Base))
	}

	// Handle draft status changes from flags
	if opts.Ready && existingPR.Draft {
		updatedPR, err := w.prExecutor.UpdateDraftStatus(ctx, existingPR, false)
		if err != nil {
			return nil, fmt.Errorf("failed to mark PR as ready: %w", err)
		}
		result.PR = updatedPR
		result.DraftChanged = true
		result.Messages = append(result.Messages, "Marked PR as ready for review")
	} else if opts.Draft && !existingPR.Draft {
		updatedPR, err := w.prExecutor.UpdateDraftStatus(ctx, existingPR, true)
		if err != nil {
			return nil, fmt.Errorf("failed to convert PR to draft: %w", err)
		}
		result.PR = updatedPR
		result.DraftChanged = true
		result.Messages = append(result.Messages, "Converted PR to draft")
	}

	return result, nil
}

// executeWithTemplateEditing handles the full workflow with template generation and editing
func (w *DiffWorkflow) executeWithTemplateEditing(
	ctx context.Context,
	opts *DiffOptions,
	existingPR *github.PullRequest,
	baseResult *BaseBranchResult,
	dependentInfo *DependentPRInfo,
	currentBranch string,
	prHeadBranch string,
	autoBranchCtx *AutoBranchContext,
	detection *DetectionResult,
) (*DiffResult, error) {
	logger.Debug().Msg("Executing workflow with template editing")

	// Step 1: Analyze commits for template pre-filling
	commitAnalysisBase := "origin/" + baseResult.Base
	analysis, err := template.AnalyzeCommitsForTemplate(w.repo, commitAnalysisBase, currentBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze commits: %w", err)
	}

	// Step 2: Get reviewer suggestions
	reviewerSuggestions, err := w.getReviewerSuggestions(ctx, currentBranch, baseResult)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to get reviewer suggestions")
		reviewerSuggestions = []string{}
	}

	// Step 3: Determine default draft value
	templateDraftDefault := w.config.Diff.CreateAsDraft
	if opts.Draft {
		templateDraftDefault = true
	} else if opts.Ready {
		templateDraftDefault = false
	}

	// Step 4: Generate template
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
		w.config.Diff.LinearEnabled,
		templateDraftDefault,
	)

	templateContent := gen.Generate()

	// Step 5: Open editor unless --no-edit
	skipEditor := opts.NoEdit
	if !skipEditor {
		templateContent, err = template.OpenEditor(templateContent)
		if err != nil {
			// Return the error as-is to preserve error type for errors.Is() checks
			return nil, err
		}
	}

	// Step 5.5: Save template immediately after editing to ensure no data loss
	// This guarantees the template is preserved even if validation passes but
	// subsequent operations (auto-branch, PR creation) fail
	var savedTemplatePath string
	if !skipEditor {
		savedTemplatePath, err = template.SaveTemplate(templateContent)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to save template (non-fatal)")
			// Continue - this is not fatal, but user won't have --continue option
		} else {
			logger.Debug().Str("path", savedTemplatePath).Msg("Template saved for --continue")
		}
	}

	// Step 6: Parse and validate template
	parsedFields, err := template.ParseTemplate(templateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	stackingCtx := &template.StackingContext{
		IsStacking:     baseResult.IsStacking,
		BaseBranch:     baseResult.Base,
		ParentPR:       baseResult.ParentPR,
		DependentPRs:   dependentInfo.DependentPRs,
		CurrentBranch:  prHeadBranch,
		ShowDependents: dependentInfo.HasDependents,
	}

	valid, validationMessage := template.ValidateFieldsWithContext(parsedFields, w.config.Diff.RequireTestPlan, stackingCtx)
	if !valid {
		// Template was already saved after editing (Step 5.5)
		// Just inform user about validation failure and saved path
		errMsg := validationMessage
		if savedTemplatePath != "" {
			errMsg += fmt.Sprintf("\n\nTemplate saved to: %s", savedTemplatePath)
			errMsg += "\nFix the issues and run:\n  gh arc diff --continue"
		}
		return nil, fmt.Errorf("%w: %s", template.ErrTemplateValidationFailed, errMsg)
	}

	// Step 7: Build PR title and body
	prTitle := parsedFields.Title
	if w.config.Diff.LinearEnabled && len(parsedFields.Ref) > 0 {
		prTitle = fmt.Sprintf("%s [%s]", prTitle, parsedFields.Ref[0])
	}

	prBody := parsedFields.Summary
	if parsedFields.TestPlan != "" {
		prBody += "\n\n## Test Plan\n" + parsedFields.TestPlan
	}
	if len(parsedFields.Ref) > 0 {
		prBody += "\n\n**Ref:** " + parsedFields.Ref[0]
	}

	// Step 8: Execute auto-branch if needed (push branch to remote)
	finalHeadBranch := prHeadBranch
	autoBranchCheckoutFailed := false
	if autoBranchCtx != nil {
		finalBranchName, err := w.autoBranchDetector.ExecuteAutoBranch(ctx, autoBranchCtx)
		if err != nil {
			// Check if it's a checkout failure (non-fatal)
			var checkoutErr *AutoBranchCheckoutError
			if errors.As(err, &checkoutErr) {
				autoBranchCheckoutFailed = true
				logger.Warn().Err(err).Msg("Auto-branch checkout failed (non-fatal)")
			} else {
				return nil, fmt.Errorf("failed to execute auto-branch: %w", err)
			}
		}
		finalHeadBranch = finalBranchName
		prHeadBranch = finalBranchName
	}

	// Step 9: Get current user for reviewer filtering
	currentUser, err := w.client.GetCurrentUser(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to get current user")
		currentUser = ""
	}

	// Step 10: Create or update PR
	prResult, err := w.prExecutor.CreateOrUpdatePR(ctx, &PRRequest{
		Title:       prTitle,
		HeadBranch:  prHeadBranch,
		BaseBranch:  baseResult.Base,
		Body:        prBody,
		Draft:       parsedFields.Draft,
		Reviewers:   parsedFields.Reviewers,
		ExistingPR:  existingPR,
		ParentPR:    baseResult.ParentPR,
		CurrentUser: currentUser,
	})
	if err != nil {
		// PR creation failed - template remains saved for --continue
		return nil, fmt.Errorf("failed to create or update PR: %w", err)
	}

	// Step 11: Clean up saved template on success
	// Only delete if PR was successfully created and we saved a template
	if savedTemplatePath != "" {
		if err := template.RemoveSavedTemplate(savedTemplatePath); err != nil {
			logger.Warn().Err(err).Str("path", savedTemplatePath).Msg("Failed to delete saved template")
			// Non-fatal - user can manually delete if needed
		} else {
			logger.Debug().Str("path", savedTemplatePath).Msg("Deleted saved template after successful PR creation")
		}
	}

	// Build result
	return &DiffResult{
		PR:                       prResult.PR,
		WasCreated:               prResult.WasCreated,
		DraftChanged:             prResult.DraftChanged,
		AutoBranchUsed:           autoBranchCtx != nil,
		AutoBranchName:           finalHeadBranch,
		AutoBranchCheckoutFailed: autoBranchCheckoutFailed,
		IsStacking:               baseResult.IsStacking,
		BaseBranch:               baseResult.Base,
		ParentPR:                 baseResult.ParentPR,
		ReviewersAdded:           prResult.ReviewersAdded,
		Messages:                 prResult.Messages,
	}, nil
}

// getReviewerSuggestions gets reviewer suggestions from CODEOWNERS and config
func (w *DiffWorkflow) getReviewerSuggestions(ctx context.Context, currentBranch string, baseResult *BaseBranchResult) ([]string, error) {
	var reviewerSuggestions []string

	// Get current user for filtering
	currentUser, err := w.client.GetCurrentUser(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to get current user")
		currentUser = ""
	}

	// Parse CODEOWNERS file
	co, err := codeowners.ParseCodeowners(".")
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to parse CODEOWNERS file")
	} else {
		// Get trunk branch
		trunkBranch, err := w.repo.GetDefaultBranch()
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to get default branch")
			trunkBranch = "main"
		}

		// Get stack-aware reviewers
		considerFullStack := baseResult.IsStacking
		reviewers, err := codeowners.GetStackAwareReviewers(
			w.repo,
			co,
			currentBranch,
			baseResult.Base,
			trunkBranch,
			currentUser,
			considerFullStack,
		)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to get stack-aware reviewers")
		} else {
			reviewerSuggestions = append(reviewerSuggestions, reviewers...)
		}
	}

	// Append default reviewers from config
	reviewerSuggestions = append(reviewerSuggestions, w.config.GitHub.DefaultReviewers...)

	// Remove duplicates
	reviewerSuggestions = codeowners.DeduplicateReviewers(reviewerSuggestions)

	logger.Debug().
		Int("count", len(reviewerSuggestions)).
		Strs("reviewers", reviewerSuggestions).
		Msg("Generated reviewer suggestions")

	return reviewerSuggestions, nil
}
