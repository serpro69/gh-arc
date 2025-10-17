package diff

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/logger"
)

// Sentinel errors for error type checking with errors.Is()
var (
	// ErrOperationCancelled indicates user cancelled the operation
	ErrOperationCancelled = errors.New("operation cancelled by user")

	// ErrStaleRemote indicates user declined to continue with stale remote
	ErrStaleRemote = errors.New("operation declined due to stale remote")
)

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
