package template

import (
	"fmt"
	"strings"

	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/logger"
)

// CommitRepository defines the minimal interface needed for commit analysis.
// This allows for easier testing without depending on the full git.Repository.
type CommitRepository interface {
	GetCommitsBetween(base, head string) ([]git.CommitInfo, error)
}

// CommitAnalysis represents the result of analyzing commits for template pre-filling
type CommitAnalysis struct {
	Title           string   // Suggested PR title
	Summary         string   // Suggested PR summary/body
	BaseBranch      string   // Base branch used for analysis
	CommitCount     int      // Number of commits analyzed
	CommitMessages  []string // All commit messages
	HasMergeCommits bool     // Whether merge commits were found
}

// AnalyzeCommitsForTemplate analyzes commits between base and head branches
// to generate suggested content for PR template
func AnalyzeCommitsForTemplate(repo CommitRepository, baseBranch, headBranch string) (*CommitAnalysis, error) {
	logger.Debug().
		Str("baseBranch", baseBranch).
		Str("headBranch", headBranch).
		Msg("Analyzing commits for template")

	// Get commits between base and head
	commits, err := repo.GetCommitsBetween(baseBranch, headBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit range: %w", err)
	}

	analysis := &CommitAnalysis{
		BaseBranch:     baseBranch,
		CommitCount:    len(commits),
		CommitMessages: make([]string, 0, len(commits)),
	}

	// Handle case with no commits
	if len(commits) == 0 {
		logger.Debug().Msg("No commits found between base and head")
		analysis.Title = generateTitleFromBranch(headBranch)
		analysis.Summary = "No commits found in this branch"
		return analysis, nil
	}

	// Extract commit messages and check for merge commits
	for _, commit := range commits {
		analysis.CommitMessages = append(analysis.CommitMessages, commit.Message)

		// Check if this is a merge commit
		if strings.HasPrefix(strings.TrimSpace(commit.Message), "Merge") {
			analysis.HasMergeCommits = true
		}
	}

	// Generate title and summary based on commit count
	if len(commits) == 1 {
		analysis.Title, analysis.Summary = generateFromSingleCommit(commits[0].Message)
	} else {
		analysis.Title, analysis.Summary = generateFromMultipleCommits(commits)
	}

	logger.Info().
		Str("title", analysis.Title).
		Int("commitCount", analysis.CommitCount).
		Bool("hasMergeCommits", analysis.HasMergeCommits).
		Msg("Commit analysis complete")

	return analysis, nil
}

// generateFromSingleCommit extracts title and body from a single commit message
func generateFromSingleCommit(commitMessage string) (title, summary string) {
	parsed := git.ParseCommitMessage(commitMessage)

	title = parsed.Title
	if title == "" {
		title = "Update code"
	}

	summary = parsed.Body
	return title, summary
}

// generateFromMultipleCommits creates title and summary from multiple commits
func generateFromMultipleCommits(commits []git.CommitInfo) (title, summary string) {
	if len(commits) == 0 {
		return "Update code", "No commits"
	}

	// Use first commit title as PR title (last in slice = first chronologically)
	firstCommit := commits[len(commits)-1]
	parsed := git.ParseCommitMessage(firstCommit.Message)
	title = parsed.Title
	if title == "" {
		title = fmt.Sprintf("Merge %d commits", len(commits))
	}

	// Aggregate all commit messages for summary
	var summaryBuilder strings.Builder
	summaryBuilder.WriteString("## Commits\n\n")

	// List commits in chronological order (reverse the slice)
	for i := len(commits) - 1; i >= 0; i-- {
		commit := commits[i]
		parsed := git.ParseCommitMessage(commit.Message)

		if parsed.Title == "" {
			continue
		}

		summaryBuilder.WriteString("- ")
		summaryBuilder.WriteString(parsed.Title)
		summaryBuilder.WriteString("\n")

		// Add commit body if present (indented)
		if parsed.Body != "" {
			bodyLines := strings.Split(parsed.Body, "\n")
			for _, line := range bodyLines {
				if strings.TrimSpace(line) != "" {
					summaryBuilder.WriteString("  ")
					summaryBuilder.WriteString(line)
					summaryBuilder.WriteString("\n")
				}
			}
		}
	}

	summary = summaryBuilder.String()
	return title, summary
}

// generateTitleFromBranch generates a PR title from branch name when no commits exist
func generateTitleFromBranch(branchName string) string {
	// Remove common prefixes
	branchName = strings.TrimPrefix(branchName, "feature/")
	branchName = strings.TrimPrefix(branchName, "fix/")
	branchName = strings.TrimPrefix(branchName, "bugfix/")
	branchName = strings.TrimPrefix(branchName, "hotfix/")
	branchName = strings.TrimPrefix(branchName, "chore/")
	branchName = strings.TrimPrefix(branchName, "refactor/")

	// Replace hyphens and underscores with spaces
	branchName = strings.ReplaceAll(branchName, "-", " ")
	branchName = strings.ReplaceAll(branchName, "_", " ")

	// Capitalize first letter of each word
	words := strings.Fields(branchName)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}

	title := strings.Join(words, " ")
	if title == "" {
		return "Update code"
	}

	return title
}

// FilterMergeCommits filters out merge commits from commit messages
func FilterMergeCommits(commitMessages []string) []string {
	filtered := make([]string, 0, len(commitMessages))
	for _, msg := range commitMessages {
		if !strings.HasPrefix(strings.TrimSpace(msg), "Merge") {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

// IsEmptyCommitMessage checks if a commit message is effectively empty
func IsEmptyCommitMessage(message string) bool {
	trimmed := strings.TrimSpace(message)
	return trimmed == "" || trimmed == "\n"
}
