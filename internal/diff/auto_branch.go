package diff

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/logger"
)

const (
	// maxPushAttempts is the maximum number of attempts to push auto-branch with collision retry
	maxPushAttempts = 3
)

// Sentinel errors for error type checking with errors.Is()
var (
	// ErrOperationCancelled indicates user cancelled the operation
	ErrOperationCancelled = errors.New("operation cancelled by user")

	// ErrStaleRemote indicates user declined to continue with stale remote
	ErrStaleRemote = errors.New("operation declined due to stale remote")

	// ErrAutoBranchCheckoutFailed indicates auto-branch checkout failed (non-fatal)
	// This is wrapped with AutoBranchCheckoutError to provide details
	ErrAutoBranchCheckoutFailed = errors.New("auto-branch checkout failed")
)

// AutoBranchCheckoutError is returned when auto-branch checkout fails (non-fatal)
type AutoBranchCheckoutError struct {
	BranchName string
	Err        error
}

func (e *AutoBranchCheckoutError) Error() string {
	return fmt.Sprintf("checkout failed: %v", e.Err)
}

func (e *AutoBranchCheckoutError) Unwrap() error {
	return e.Err
}

func (e *AutoBranchCheckoutError) Is(target error) bool {
	return target == ErrAutoBranchCheckoutFailed
}

// AutoBranchDetector is the main detector and orchestrator for auto-branch operations.
// It handles detection of commits on main and orchestrates the auto-branch creation flow.
type AutoBranchDetector struct {
	repo   *git.Repository
	config *config.DiffConfig
}

// NewAutoBranchDetector creates a new AutoBranchDetector with the given repository and configuration.
func NewAutoBranchDetector(repo *git.Repository, cfg *config.DiffConfig) *AutoBranchDetector {
	return &AutoBranchDetector{
		repo:   repo,
		config: cfg,
	}
}

// DetectionResult represents the result of detecting commits on main.
type DetectionResult struct {
	OnMainBranch  bool   // Whether the current branch is main/master
	CommitsAhead  int    // Number of commits ahead of origin/main
	DefaultBranch string // The default branch name (main or master)
}

// AutoBranchContext holds state for the auto-branch operation.
// This context is passed through the diff command flow to coordinate
// the push-before-PR and checkout-after-PR operations.
type AutoBranchContext struct {
	BranchName    string // Generated branch name for the auto-branch
	ShouldProceed bool   // Whether the operation should proceed
}

// DetectCommitsOnMain detects if the user is on main/master with unpushed commits.
// It returns a DetectionResult with the current state, or an error if detection fails.
func (d *AutoBranchDetector) DetectCommitsOnMain(ctx context.Context) (*DetectionResult, error) {
	logger := logger.Get()

	// Get current branch
	currentBranch, err := d.repo.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Get default branch
	defaultBranch, err := d.repo.GetDefaultBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get default branch: %w", err)
	}

	// Check if we're on the main branch
	onMainBranch := currentBranch == defaultBranch

	// Initialize result
	result := &DetectionResult{
		OnMainBranch:  onMainBranch,
		CommitsAhead:  0,
		DefaultBranch: defaultBranch,
	}

	// If we're not on main, no need to count commits
	if !onMainBranch {
		logger.Debug().
			Str("currentBranch", currentBranch).
			Str("defaultBranch", defaultBranch).
			Bool("onMainBranch", false).
			Msg("Not on main branch, skipping commit count")
		return result, nil
	}

	// Count commits ahead of origin/defaultBranch
	remoteBranch := "origin/" + defaultBranch
	commitsAhead, err := d.repo.CountCommitsAhead(currentBranch, remoteBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to count commits ahead: %w", err)
	}

	result.CommitsAhead = commitsAhead

	logger.Debug().
		Str("currentBranch", currentBranch).
		Str("defaultBranch", defaultBranch).
		Str("remoteBranch", remoteBranch).
		Bool("onMainBranch", onMainBranch).
		Int("commitsAhead", commitsAhead).
		Msg("Detected commits on main")

	return result, nil
}

// ShouldAutoBranch determines if the auto-branch flow should be activated.
// Returns true if the user is on main/master with unpushed commits.
func (d *AutoBranchDetector) ShouldAutoBranch(result *DetectionResult) bool {
	return result.OnMainBranch && result.CommitsAhead > 0
}

// sanitizeBranchName cleans a string to make it valid for use in git branch names.
// It converts to lowercase, replaces spaces with hyphens, and removes invalid characters.
func sanitizeBranchName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace spaces with hyphens
	name = strings.ReplaceAll(name, " ", "-")

	// Replace double dots with single dash
	name = strings.ReplaceAll(name, "..", "-")

	// Remove invalid git branch characters
	invalidChars := []string{"~", "^", ":", "?", "*", "[", "]", "\\"}
	for _, char := range invalidChars {
		name = strings.ReplaceAll(name, char, "")
	}

	return name
}

// GenerateBranchName generates a branch name based on the configured pattern.
// Returns (generated name, shouldPrompt, error).
// If shouldPrompt is true, the caller should prompt the user for a branch name.
func (d *AutoBranchDetector) GenerateBranchName() (string, bool, error) {
	pattern := d.config.AutoBranchNamePattern

	// Pattern "null" triggers user prompt
	if pattern == "null" {
		return "", true, nil
	}

	// Empty pattern uses default
	if pattern == "" {
		timestamp := time.Now().Unix()
		return fmt.Sprintf("feature/auto-from-main-%d", timestamp), false, nil
	}

	// Apply placeholders
	result := pattern

	// {timestamp} - Unix timestamp
	if strings.Contains(result, "{timestamp}") {
		timestamp := time.Now().Unix()
		result = strings.ReplaceAll(result, "{timestamp}", fmt.Sprintf("%d", timestamp))
	}

	// {date} - ISO date format (2006-01-02)
	if strings.Contains(result, "{date}") {
		date := time.Now().Format("2006-01-02")
		result = strings.ReplaceAll(result, "{date}", date)
	}

	// {datetime} - ISO datetime format (2006-01-02T150405)
	if strings.Contains(result, "{datetime}") {
		datetime := time.Now().Format("2006-01-02T150405")
		result = strings.ReplaceAll(result, "{datetime}", datetime)
	}

	// {username} - Git user.name, sanitized
	if strings.Contains(result, "{username}") {
		username, err := d.repo.GetGitConfig("user.name")
		if err != nil {
			// If we can't get username, use "user" as fallback
			username = "user"
		}
		sanitized := sanitizeBranchName(username)
		result = strings.ReplaceAll(result, "{username}", sanitized)
	}

	// {random} - 6-character random alphanumeric string
	if strings.Contains(result, "{random}") {
		random := generateRandomString(6)
		result = strings.ReplaceAll(result, "{random}", random)
	}

	return result, false, nil
}

// generateRandomString creates a random alphanumeric string of the specified length.
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

// EnsureUniqueBranchName checks if a branch name exists and appends a counter if needed.
// Returns a unique branch name or an error if unable to generate one.
func (d *AutoBranchDetector) EnsureUniqueBranchName(baseName string) (string, error) {
	// Check if base name is unique
	exists, err := d.repo.BranchExists(baseName)
	if err != nil {
		return "", fmt.Errorf("failed to check if branch exists: %w", err)
	}

	if !exists {
		return baseName, nil
	}

	// Try appending counters until we find a unique name
	// Safety limit: stop after 100 attempts
	for i := 1; i <= 100; i++ {
		candidate := fmt.Sprintf("%s-%d", baseName, i)
		exists, err := d.repo.BranchExists(candidate)
		if err != nil {
			return "", fmt.Errorf("failed to check if branch exists: %w", err)
		}

		if !exists {
			logger.Get().Debug().
				Str("baseName", baseName).
				Str("uniqueName", candidate).
				Int("attempts", i).
				Msg("Generated unique branch name")
			return candidate, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique branch name after 100 attempts (base: %s)", baseName)
}

// promptYesNo displays a yes/no prompt and reads the user's response.
// Returns true for yes, false for no.
// The defaultYes parameter determines the default if user presses Enter without input.
func promptYesNo(message string, defaultYes bool) (bool, error) {
	// Display prompt with appropriate default indicator
	prompt := message + " "
	if defaultYes {
		prompt += "(Y/n): "
	} else {
		prompt += "(y/N): "
	}

	fmt.Print(prompt)

	// Read user input
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read user input: %w", err)
	}

	// Trim whitespace and convert to lowercase
	input = strings.ToLower(strings.TrimSpace(input))

	// Handle empty input (use default)
	if input == "" {
		return defaultYes, nil
	}

	// Parse response
	switch input {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		// Invalid input, re-prompt
		fmt.Println("Please answer 'y' or 'n'")
		return promptYesNo(message, defaultYes)
	}
}

// promptBranchName prompts the user to enter a branch name.
// Returns empty string if user presses Enter (caller should generate default).
// Re-prompts if input contains spaces or other invalid characters.
func promptBranchName() (string, error) {
	fmt.Print("Enter branch name (or press Enter for default): ")

	// Read user input
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read user input: %w", err)
	}

	// Trim whitespace
	input = strings.TrimSpace(input)

	// Empty input means use default
	if input == "" {
		return "", nil
	}

	// Validate: reject names with spaces
	if strings.Contains(input, " ") {
		fmt.Println("Branch name cannot contain spaces. Please try again.")
		return promptBranchName()
	}

	return input, nil
}

// CheckStaleRemote checks if the remote tracking branch is stale (not updated recently).
// Returns (shouldContinue, error).
// If shouldContinue is false, the operation should be aborted (user declined or error).
func (d *AutoBranchDetector) CheckStaleRemote(ctx context.Context, defaultBranch string) (bool, error) {
	logger := logger.Get()

	// Get threshold from config
	threshold := d.config.StaleRemoteThresholdHours

	// If threshold is 0, skip check (disabled)
	if threshold == 0 {
		return true, nil
	}

	// Get age of remote ref
	remoteBranch := "origin/" + defaultBranch
	age, err := d.repo.GetRemoteRefAge(remoteBranch)
	if err != nil {
		// If remote doesn't exist, skip check (might be offline or first commit)
		if strings.Contains(err.Error(), "remote ref not found") {
			logger.Debug().
				Str("remoteBranch", remoteBranch).
				Msg("Remote ref not found, skipping stale check")
			return true, nil
		}
		// Other errors are real problems
		return false, fmt.Errorf("failed to check remote ref age: %w", err)
	}

	// Convert threshold to duration
	thresholdDuration := time.Duration(threshold) * time.Hour

	// Check if remote is stale
	if age > thresholdDuration {
		// Calculate human-readable time
		days := int(age.Hours() / 24)
		hours := int(age.Hours()) % 24

		var ageStr string
		if days > 0 {
			ageStr = fmt.Sprintf("%d day(s) and %d hour(s)", days, hours)
		} else {
			ageStr = fmt.Sprintf("%d hour(s)", hours)
		}

		// Display warning
		fmt.Printf("\n⚠️  Warning: Your local tracking branch '%s' is %s old.\n", remoteBranch, ageStr)
		fmt.Println("This means your local repository may be out of sync with the remote.")
		fmt.Println("Consider running 'git fetch origin' to update your local tracking branches.")
		fmt.Println()

		// Prompt user
		cont, err := promptYesNo("Continue anyway?", false)
		if err != nil {
			return false, err
		}

		if !cont {
			// User declined
			return false, ErrStaleRemote
		}

		// User accepted, log warning
		logger.Warn().
			Str("remoteBranch", remoteBranch).
			Dur("age", age).
			Msg("User chose to continue despite stale remote")
	}

	return true, nil
}

// displayCommitList displays a list of commits that will be moved to the new branch.
// This helps users understand what commits are being moved.
func displayCommitList(commits []git.CommitInfo) {
	if len(commits) == 0 {
		return
	}

	// Display header
	fmt.Printf("\nThe following %d commit(s) on the main branch will be moved to a new feature branch:\n\n", len(commits))

	// Display each commit
	for _, commit := range commits {
		// Get short hash (first 7 characters)
		shortHash := commit.SHA
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		// Get first line of commit message
		message := commit.Message
		if idx := strings.Index(message, "\n"); idx != -1 {
			message = message[:idx]
		}

		// Truncate message if too long
		if len(message) > 80 {
			message = message[:77] + "..."
		}

		// Display formatted line
		fmt.Printf("  - %s %s\n", shortHash, message)
	}

	fmt.Println()
}

// PrepareAutoBranch prepares the auto-branch operation by checking for stale remote,
// prompting user if needed, generating branch name, and returning the context for execution.
// Returns AutoBranchContext with branch name and proceed flag, or error if preparation fails.
func (d *AutoBranchDetector) PrepareAutoBranch(ctx context.Context, detection *DetectionResult) (*AutoBranchContext, error) {
	logger := logger.Get()

	// Check stale remote first
	shouldContinue, err := d.CheckStaleRemote(ctx, detection.DefaultBranch)
	if err != nil {
		return nil, err
	}
	if !shouldContinue {
		return nil, ErrStaleRemote
	}

	// Check if should proceed based on config
	if !d.config.AutoCreateBranchFromMain {
		// Config disabled, need to prompt user

		// Get commit list to display
		remoteBranch := "origin/" + detection.DefaultBranch
		commits, err := d.repo.GetCommitsBetween(remoteBranch, detection.DefaultBranch)
		if err != nil {
			logger.Warn().
				Err(err).
				Msg("Failed to get commit list, continuing without display")
		} else {
			// Display commits that will be moved
			displayCommitList(commits)
		}

		// Prompt user for confirmation
		cont, err := promptYesNo("Create feature branch automatically?", true)
		if err != nil {
			return nil, err
		}

		if !cont {
			return nil, ErrOperationCancelled
		}
	}

	// Generate branch name (or prompt if pattern is "null")
	branchName, shouldPrompt, err := d.GenerateBranchName()
	if err != nil {
		return nil, fmt.Errorf("failed to generate branch name: %w", err)
	}

	if shouldPrompt {
		// Pattern is "null", prompt user for branch name
		input, err := promptBranchName()
		if err != nil {
			return nil, err
		}

		if input == "" {
			// User pressed Enter, generate default
			timestamp := time.Now().Unix()
			branchName = fmt.Sprintf("feature/auto-from-main-%d", timestamp)
		} else {
			branchName = input
		}
	}

	// Ensure branch name is unique
	uniqueName, err := d.EnsureUniqueBranchName(branchName)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure unique branch name: %w", err)
	}

	logger.Debug().
		Str("branchName", uniqueName).
		Msg("Prepared auto-branch operation")

	return &AutoBranchContext{
		BranchName:    uniqueName,
		ShouldProceed: true,
	}, nil
}

// PushWithRetry pushes the auto-created branch to remote with collision retry logic.
// If a branch name collision is detected, it regenerates a unique name and retries.
// Returns the final branch name used (which may differ from context.BranchName if collision occurred).
func (d *AutoBranchDetector) PushWithRetry(ctx context.Context, context *AutoBranchContext, maxAttempts int) (string, error) {
	logger := logger.Get()
	originalBranchName := context.BranchName
	currentBranchName := context.BranchName
	var pushErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.Debug().
			Str("branchName", currentBranchName).
			Int("attempt", attempt).
			Int("maxAttempts", maxAttempts).
			Msg("Attempting to push auto-branch")

		// Attempt push using HEAD:branchName format to push from current HEAD to remote branch
		pushErr = d.repo.PushBranch(ctx, "HEAD", currentBranchName)

		if pushErr == nil {
			// Success!
			logger.Info().
				Str("branchName", currentBranchName).
				Int("attempt", attempt).
				Msg("Successfully pushed auto-branch to remote")
			return currentBranchName, nil
		}

		// Check error type
		if errors.Is(pushErr, git.ErrRemoteBranchExists) {
			// Branch name collision - regenerate and retry
			logger.Warn().
				Str("branchName", currentBranchName).
				Int("attempt", attempt).
				Msg("Branch name collision detected during push")

			if attempt < maxAttempts {
				// Generate new unique name
				newName, err := d.EnsureUniqueBranchName(originalBranchName)
				if err != nil {
					return "", fmt.Errorf("failed to generate unique branch name after collision: %w", err)
				}

				currentBranchName = newName
				logger.Debug().
					Str("newBranchName", newName).
					Msg("Generated new branch name after collision")
				// Loop will retry with new name
			} else {
				// Exhausted retries
				return "", fmt.Errorf("failed to push branch after %d attempts due to name collisions: %w", maxAttempts, pushErr)
			}
		} else if errors.Is(pushErr, git.ErrAuthenticationFailed) {
			// Authentication error - cannot retry
			return "", fmt.Errorf("authentication failed when pushing branch: %w", pushErr)
		} else {
			// Other error - cannot retry
			return "", fmt.Errorf("failed to push branch: %w", pushErr)
		}
	}

	// Should not reach here, but safety catch
	return "", fmt.Errorf("failed to push branch after %d attempts: %w", maxAttempts, pushErr)
}

// ExecuteAutoBranch executes the full auto-branch flow:
// 1. Pushes the branch to remote (with collision retry)
// 2. Checks out the tracking branch locally
// Returns the final branch name used (may differ from context.BranchName if collision occurred).
// Checkout failures are non-fatal and return an error that should be displayed as a warning.
func (d *AutoBranchDetector) ExecuteAutoBranch(ctx context.Context, context *AutoBranchContext) (string, error) {
	logger := logger.Get()

	logger.Debug().
		Str("branchName", context.BranchName).
		Msg("Executing auto-branch flow")

	// Step 1: Push branch to remote with collision retry
	finalBranchName, err := d.PushWithRetry(ctx, context, maxPushAttempts)
	if err != nil {
		return "", fmt.Errorf("failed to push auto-branch: %w", err)
	}

	// Update context with final branch name (in case collision occurred)
	context.BranchName = finalBranchName

	// Step 2: Checkout tracking branch locally
	err = d.repo.CheckoutTrackingBranch(finalBranchName, "origin/"+finalBranchName)
	if err != nil {
		// Checkout failed, but don't fail the entire operation
		// User can manually checkout the branch
		logger.Warn().
			Err(err).
			Str("branchName", finalBranchName).
			Msg("Failed to checkout tracking branch (non-fatal)")
		// Return typed error so caller can handle it gracefully
		return finalBranchName, &AutoBranchCheckoutError{BranchName: finalBranchName, Err: err}
	}

	logger.Info().
		Str("branchName", finalBranchName).
		Msg("Successfully executed auto-branch flow")

	return finalBranchName, nil
}
