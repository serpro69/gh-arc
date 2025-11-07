package diff

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/github"
	"github.com/serpro69/gh-arc/internal/logger"
	"github.com/serpro69/gh-arc/internal/template"
)

// ContinueModeExecutor handles the --continue workflow for diff command.
// This workflow loads a previously saved template, allows editing to fix
// validation errors, and then creates or updates the PR.
type ContinueModeExecutor struct {
	repo               *git.Repository
	client             *github.Client
	config             *config.DiffConfig
	owner              string
	name               string
	autoBranchDetector *AutoBranchDetector
}

// ContinueModeOptions contains options for continue mode execution
type ContinueModeOptions struct {
	CurrentBranch  string
	NoEdit         bool
	RequireTestPlan bool
}

// ContinueModeResult contains the results of continue mode execution
type ContinueModeResult struct {
	PR             *github.PullRequest
	WasCreated     bool
	HeadBranch     string
	BaseBranch     string
	ParsedFields   *template.TemplateFields
	Messages       []string
}

// NewContinueModeExecutor creates a new continue mode executor
func NewContinueModeExecutor(repo *git.Repository, client *github.Client, cfg *config.DiffConfig, owner, name string) *ContinueModeExecutor {
	return &ContinueModeExecutor{
		repo:               repo,
		client:             client,
		config:             cfg,
		owner:              owner,
		name:               name,
		autoBranchDetector: NewAutoBranchDetector(repo, cfg),
	}
}

// handleAutoBranch checks if we're in an auto-branch scenario (on main with commits)
// and prepares the auto-branch context if needed. This handles cases where template
// validation failed before auto-branch execution could happen.
func (e *ContinueModeExecutor) handleAutoBranch(ctx context.Context) (*AutoBranchContext, *DetectionResult, error) {
	logger := logger.Get()

	// Detect if user has commits on main
	detection, err := e.autoBranchDetector.DetectCommitsOnMain(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to detect commits on main: %w", err)
	}

	// Check if auto-branch flow should be activated
	if !e.autoBranchDetector.ShouldAutoBranch(detection) {
		logger.Debug().Msg("Not in auto-branch scenario for continue mode")
		return nil, detection, nil
	}

	logger.Info().Msg("Auto-branch scenario detected in continue mode")

	// Prepare auto-branch context
	autoBranchCtx, err := e.autoBranchDetector.PrepareAutoBranch(ctx, detection)
	if err != nil {
		return nil, detection, err
	}

	logger.Debug().
		Str("branchName", autoBranchCtx.BranchName).
		Msg("Auto-branch context prepared for continue mode")

	return autoBranchCtx, detection, nil
}

// Execute runs the complete continue mode workflow:
// 1. Load saved template
// 2. Open editor (unless --no-edit)
// 3. Parse and validate template
// 4. Create or update PR
// 5. Assign reviewers
func (e *ContinueModeExecutor) Execute(ctx context.Context, opts *ContinueModeOptions) (*ContinueModeResult, error) {
	logger.Debug().Msg("Executing continue mode workflow")

	// Step 1: Find and load saved template
	savedTemplates, err := template.FindSavedTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to find saved template (use 'gh arc diff --edit' to start fresh): %w", err)
	} else if len(savedTemplates) == 0 {
		return nil, fmt.Errorf("no saved template found (use 'gh arc diff --edit' to start fresh)")
	}

	// Use the most recent template (FindSavedTemplates returns sorted by newest first)
	savedTemplatePath := savedTemplates[0]
	templateContent, err := template.LoadSavedTemplate(savedTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load saved template (use 'gh arc diff --edit' to start fresh): %w", err)
	}

	logger.Debug().
		Str("templatePath", savedTemplatePath).
		Msg("Loaded saved template")

	// Step 2: Check for auto-branch scenario
	autoBranchCtx, _, err := e.handleAutoBranch(ctx)
	if err != nil {
		return nil, err
	}

	// Step 3: Determine PR head branch
	var prHeadBranch string
	if autoBranchCtx != nil {
		// Auto-branch scenario: use generated branch name
		prHeadBranch = autoBranchCtx.BranchName
		logger.Debug().
			Str("branchName", prHeadBranch).
			Msg("Using auto-generated branch name for PR head")
	} else {
		// Normal scenario: use current git branch
		prHeadBranch = opts.CurrentBranch
		logger.Debug().
			Str("branchName", prHeadBranch).
			Msg("Using current git branch for PR head")
	}

	// Step 4: Extract base branch from template
	prBaseBranch, found := template.ExtractBaseBranch(templateContent)
	if !found {
		return nil, fmt.Errorf("failed to extract base branch from template (template may be corrupted)")
	}

	logger.Debug().
		Str("headBranch", prHeadBranch).
		Str("baseBranch", prBaseBranch).
		Msg("Determined branch configuration for PR")

	// Step 3: Open editor to allow fixing validation issues (unless --no-edit)
	if !opts.NoEdit {
		templateContent, err = template.OpenEditor(templateContent)
		if err != nil {
			if err == template.ErrEditorCancelled {
				return nil, template.ErrEditorCancelled
			}
			return nil, fmt.Errorf("failed to open editor: %w", err)
		}
	}

	// Step 4: Parse and validate template
	parsedFields, err := template.ParseTemplate(templateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Validate required fields (no stacking context in continue mode)
	validationErrors := template.ValidateFields(parsedFields, opts.RequireTestPlan, nil)
	if len(validationErrors) > 0 {
		// Save the edited template so user's changes are preserved for next --continue
		// Delete the old template first
		if err := os.Remove(savedTemplatePath); err != nil {
			logger.Warn().Err(err).Msg("Failed to remove old saved template")
		}

		// Save the new edited template
		newTemplatePath, err := template.SaveTemplate(templateContent)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to save edited template")
			// Continue with validation error even if save fails
		} else {
			logger.Debug().Str("path", newTemplatePath).Msg("Saved edited template for retry")
		}

		// Return validation error with helpful message
		errMsg := "Template validation failed:\n"
		for _, e := range validationErrors {
			errMsg += fmt.Sprintf("  • %s\n", e)
		}
		if newTemplatePath != "" {
			errMsg += fmt.Sprintf("\nTemplate saved to: %s\n", newTemplatePath)
		}
		errMsg += "Fix the issues and run:\n  gh arc diff --continue"

		return nil, fmt.Errorf("%w: %s", template.ErrTemplateValidationFailed, errMsg)
	}

	logger.Debug().
		Str("title", parsedFields.Title).
		Int("reviewers", len(parsedFields.Reviewers)).
		Msg("Template validated successfully")

	// Step 5: Delete saved template after successful validation
	if err := os.Remove(savedTemplatePath); err != nil {
		logger.Warn().Err(err).Msg("Failed to remove saved template")
	}

	// Step 6: Execute auto-branch if needed (push branch to remote and checkout)
	autoBranchCheckoutFailed := false
	if autoBranchCtx != nil {
		logger.Info().
			Str("branchName", autoBranchCtx.BranchName).
			Msg("Executing auto-branch for continue mode")

		finalBranchName, err := e.autoBranchDetector.ExecuteAutoBranch(ctx, autoBranchCtx)
		if err != nil {
			// Check if it's a checkout failure (non-fatal)
			var checkoutErr *AutoBranchCheckoutError
			if errors.As(err, &checkoutErr) {
				autoBranchCheckoutFailed = true
				logger.Warn().Err(err).Msg("Auto-branch checkout failed (non-fatal)")
				// Continue with PR creation
			} else {
				return nil, fmt.Errorf("failed to execute auto-branch: %w", err)
			}
		}
		prHeadBranch = finalBranchName
		logger.Info().
			Str("branchName", finalBranchName).
			Msg("Auto-branch executed successfully")
	}

	// Step 7: Check for existing PR
	existingPR, err := e.client.FindExistingPR(ctx, e.owner, e.name, prHeadBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing PR: %w", err)
	}

	// Step 7: Build PR title and body
	prTitle := parsedFields.Title
	prBody := parsedFields.Summary
	if parsedFields.TestPlan != "" {
		prBody += "\n\n## Test Plan\n" + parsedFields.TestPlan
	}
	if len(parsedFields.Ref) > 0 {
		prBody += "\n\n**Ref:** " + parsedFields.Ref[0]
	}

	// Step 8: Create PR executor and execute PR creation/update
	prExecutor := NewPRExecutor(e.client, e.repo, e.owner, e.name)

	// Get current user for reviewer filtering
	currentUser, err := e.client.GetCurrentUser(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to get current user, reviewer filtering may not work")
		currentUser = ""
	}

	prResult, err := prExecutor.CreateOrUpdatePR(ctx, &PRRequest{
		Title:       prTitle,
		HeadBranch:  prHeadBranch,
		BaseBranch:  prBaseBranch,
		Body:        prBody,
		Draft:       parsedFields.Draft,
		Reviewers:   parsedFields.Reviewers,
		ExistingPR:  existingPR,
		ParentPR:    nil, // No parent PR in continue mode
		CurrentUser: currentUser,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create or update PR: %w", err)
	}

	logger.Info().
		Int("prNumber", prResult.PR.Number).
		Bool("wasCreated", prResult.WasCreated).
		Msg("Continue mode execution completed")

	// Build result with auto-branch warnings if applicable
	messages := prResult.Messages
	if autoBranchCheckoutFailed {
		checkoutMsg := fmt.Sprintf("⚠️  Note: Failed to checkout %s locally. Run 'git checkout %s' manually.", prHeadBranch, prHeadBranch)
		messages = append(messages, checkoutMsg)
	}

	result := &ContinueModeResult{
		PR:           prResult.PR,
		WasCreated:   prResult.WasCreated,
		HeadBranch:   prHeadBranch,
		BaseBranch:   prBaseBranch,
		ParsedFields: parsedFields,
		Messages:     messages,
	}

	return result, nil
}
