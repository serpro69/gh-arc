package codeowners

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/logger"
)

// Owner represents a code owner (user or team)
type Owner struct {
	Name string // @username or @org/team
	Type string // "user" or "team"
}

// Rule represents a CODEOWNERS rule mapping a pattern to owners
type Rule struct {
	Pattern string   // File pattern (glob)
	Owners  []Owner  // List of owners for this pattern
	Line    int      // Line number in CODEOWNERS file (for debugging)
}

// CodeOwners represents a parsed CODEOWNERS file
type CodeOwners struct {
	Rules []Rule
	Path  string // Path to CODEOWNERS file
}

// ParseCodeowners reads and parses a CODEOWNERS file
func ParseCodeowners(repoPath string) (*CodeOwners, error) {
	// Try standard CODEOWNERS locations in order
	locations := []string{
		filepath.Join(repoPath, ".github", "CODEOWNERS"),
		filepath.Join(repoPath, "docs", "CODEOWNERS"),
		filepath.Join(repoPath, "CODEOWNERS"),
	}

	var codeownersPath string
	var file *os.File
	var err error

	for _, loc := range locations {
		file, err = os.Open(loc)
		if err == nil {
			codeownersPath = loc
			break
		}
	}

	if file == nil {
		// CODEOWNERS file not found, return empty result (not an error)
		logger.Debug().Msg("No CODEOWNERS file found")
		return &CodeOwners{Rules: []Rule{}, Path: ""}, nil
	}
	defer file.Close()

	logger.Debug().
		Str("path", codeownersPath).
		Msg("Found CODEOWNERS file")

	co := &CodeOwners{
		Rules: []Rule{},
		Path:  codeownersPath,
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse the rule
		rule, err := parseRule(line, lineNum)
		if err != nil {
			logger.Warn().
				Err(err).
				Int("line", lineNum).
				Str("content", line).
				Msg("Failed to parse CODEOWNERS rule, skipping")
			continue
		}

		co.Rules = append(co.Rules, rule)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading CODEOWNERS file: %w", err)
	}

	logger.Debug().
		Int("rules", len(co.Rules)).
		Msg("Parsed CODEOWNERS file")

	return co, nil
}

// parseRule parses a single CODEOWNERS rule line
// Format: pattern @owner1 @owner2 @org/team
func parseRule(line string, lineNum int) (Rule, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return Rule{}, fmt.Errorf("invalid CODEOWNERS rule: need pattern and at least one owner")
	}

	pattern := parts[0]
	owners := make([]Owner, 0, len(parts)-1)

	for _, ownerStr := range parts[1:] {
		if !strings.HasPrefix(ownerStr, "@") {
			logger.Warn().
				Str("owner", ownerStr).
				Int("line", lineNum).
				Msg("Owner does not start with @, skipping")
			continue
		}

		ownerType := "user"
		if strings.Contains(ownerStr, "/") {
			ownerType = "team"
		}

		owners = append(owners, Owner{
			Name: ownerStr,
			Type: ownerType,
		})
	}

	if len(owners) == 0 {
		return Rule{}, fmt.Errorf("no valid owners found in rule")
	}

	return Rule{
		Pattern: pattern,
		Owners:  owners,
		Line:    lineNum,
	}, nil
}

// GetChangedFiles returns the list of files changed between base and head
func GetChangedFiles(repo *git.Repository, baseBranch, headBranch string) ([]string, error) {
	// Get commit range
	commits, err := repo.GetCommitRange(baseBranch, headBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit range: %w", err)
	}

	if len(commits) == 0 {
		logger.Debug().Msg("No commits in range, no changed files")
		return []string{}, nil
	}

	// Get list of changed files using git diff
	changedFiles, err := repo.GetChangedFiles(baseBranch, headBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	logger.Debug().
		Int("count", len(changedFiles)).
		Str("base", baseBranch).
		Str("head", headBranch).
		Msg("Found changed files")

	return changedFiles, nil
}

// GetReviewers returns suggested reviewers for the given changed files
func (co *CodeOwners) GetReviewers(changedFiles []string, currentUser string) []string {
	if len(co.Rules) == 0 {
		logger.Debug().Msg("No CODEOWNERS rules, no reviewers suggested")
		return []string{}
	}

	reviewerSet := make(map[string]bool)

	// Process rules in reverse order (later rules override earlier ones)
	for i := len(co.Rules) - 1; i >= 0; i-- {
		rule := co.Rules[i]

		for _, file := range changedFiles {
			if matchPattern(rule.Pattern, file) {
				for _, owner := range rule.Owners {
					// Filter out current user
					if strings.EqualFold(owner.Name, currentUser) || strings.EqualFold(owner.Name, "@"+currentUser) {
						continue
					}
					reviewerSet[owner.Name] = true
				}
			}
		}
	}

	// Convert set to slice
	reviewers := make([]string, 0, len(reviewerSet))
	for reviewer := range reviewerSet {
		reviewers = append(reviewers, reviewer)
	}

	logger.Debug().
		Int("count", len(reviewers)).
		Strs("reviewers", reviewers).
		Msg("Suggested reviewers from CODEOWNERS")

	return reviewers
}

// matchPattern checks if a file path matches a CODEOWNERS pattern
// Supports:
// - Exact matches: "README.md"
// - Directory matches: "docs/"
// - Wildcards: "*.go", "src/**/*.js"
// - Leading slash for root-relative: "/config.yml"
func matchPattern(pattern, filePath string) bool {
	// Normalize paths (remove leading ./)
	filePath = strings.TrimPrefix(filePath, "./")
	pattern = strings.TrimPrefix(pattern, "./")

	// Handle directory patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		// Match if file is in this directory
		dirPattern := strings.TrimSuffix(pattern, "/")
		return strings.HasPrefix(filePath, dirPattern+"/") || filePath == dirPattern
	}

	// Handle root-relative patterns (starting with /)
	if strings.HasPrefix(pattern, "/") {
		pattern = strings.TrimPrefix(pattern, "/")
		// Must match from root
		return matchGlob(pattern, filePath)
	}

	// For patterns without leading slash, match anywhere in tree
	// Try exact match first
	if matchGlob(pattern, filePath) {
		return true
	}

	// For patterns without wildcards and without slashes, match basename anywhere
	// e.g., "README.md" matches "docs/README.md"
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "/") {
		basename := filepath.Base(filePath)
		if pattern == basename {
			return true
		}
	}

	// Try matching with pattern as suffix (e.g., "*.md" matches "docs/README.md")
	if strings.Contains(pattern, "*") {
		// For wildcard patterns, check if any part of the path matches
		parts := strings.Split(filePath, "/")
		for i := range parts {
			subPath := strings.Join(parts[i:], "/")
			if matchGlob(pattern, subPath) {
				return true
			}
		}
	}

	return false
}

// matchGlob performs glob-style pattern matching
// Supports: *, **, ?, [abc], [a-z]
func matchGlob(pattern, str string) bool {
	// Handle ** (match any directories)
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]

			// Remove trailing slash from prefix, leading slash from suffix
			prefix = strings.TrimSuffix(prefix, "/")
			suffix = strings.TrimPrefix(suffix, "/")

			// Check if str starts with prefix and ends with suffix
			if prefix != "" && !strings.HasPrefix(str, prefix) {
				return false
			}
			if suffix != "" {
				// Match suffix against end of path
				if suffix == "" {
					return true
				}
				// For suffix matching, we need to handle wildcards in the suffix
				return matchGlobSimple(suffix, filepath.Base(str))
			}
			return true
		}
	}

	// Use filepath.Match for simple glob patterns
	matched, err := filepath.Match(pattern, str)
	if err != nil {
		logger.Debug().
			Err(err).
			Str("pattern", pattern).
			Str("str", str).
			Msg("Error matching glob pattern")
		return false
	}

	return matched
}

// matchGlobSimple performs simple glob matching without **
func matchGlobSimple(pattern, str string) bool {
	matched, _ := filepath.Match(pattern, str)
	return matched
}

// GetStackAwareReviewers returns reviewers considering the entire stack or just current PR
// When considerFullStack is true, it suggests reviewers for all files changed in the stack
// When false, it only considers files changed in the current PR
func GetStackAwareReviewers(repo *git.Repository, co *CodeOwners, currentBranch, baseBranch, trunkBranch, currentUser string, considerFullStack bool) ([]string, error) {
	var changedFiles []string
	var err error

	if considerFullStack && baseBranch != trunkBranch {
		// For stacked PRs, consider files from trunk to current branch
		changedFiles, err = GetChangedFiles(repo, trunkBranch, currentBranch)
	} else {
		// Just current PR: base to current
		changedFiles, err = GetChangedFiles(repo, baseBranch, currentBranch)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	reviewers := co.GetReviewers(changedFiles, currentUser)
	return reviewers, nil
}

// FindCodeownersFile searches for CODEOWNERS file in standard locations
func FindCodeownersFile(repoPath string) (string, error) {
	locations := []string{
		filepath.Join(repoPath, ".github", "CODEOWNERS"),
		filepath.Join(repoPath, "docs", "CODEOWNERS"),
		filepath.Join(repoPath, "CODEOWNERS"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc, nil
		}
	}

	return "", fmt.Errorf("CODEOWNERS file not found")
}

// ValidateOwner checks if an owner string is valid
func ValidateOwner(owner string) bool {
	if !strings.HasPrefix(owner, "@") {
		return false
	}

	// Remove @ prefix for validation
	name := strings.TrimPrefix(owner, "@")

	// Must not be empty
	if name == "" {
		return false
	}

	// For teams (org/team), validate both parts
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		if len(parts) != 2 {
			return false
		}
		if parts[0] == "" || parts[1] == "" {
			return false
		}
	}

	return true
}

// DeduplicateReviewers removes duplicate reviewers from a list
func DeduplicateReviewers(reviewers []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(reviewers))

	for _, reviewer := range reviewers {
		if !seen[reviewer] {
			seen[reviewer] = true
			result = append(result, reviewer)
		}
	}

	return result
}

// FilterCurrentUser removes the current user from the reviewers list
func FilterCurrentUser(reviewers []string, currentUser string) []string {
	result := make([]string, 0, len(reviewers))

	// Normalize current user (add @ if missing)
	if currentUser != "" && !strings.HasPrefix(currentUser, "@") {
		currentUser = "@" + currentUser
	}

	for _, reviewer := range reviewers {
		if strings.EqualFold(reviewer, currentUser) {
			continue
		}
		result = append(result, reviewer)
	}

	return result
}

// GetOwnersForFile returns the owners for a specific file
func (co *CodeOwners) GetOwnersForFile(filePath string) []Owner {
	// Process rules in reverse order (later rules take precedence)
	for i := len(co.Rules) - 1; i >= 0; i-- {
		rule := co.Rules[i]
		if matchPattern(rule.Pattern, filePath) {
			return rule.Owners
		}
	}

	return []Owner{}
}

// HasCodeowners checks if a CODEOWNERS file exists
func HasCodeowners(repoPath string) bool {
	_, err := FindCodeownersFile(repoPath)
	return err == nil
}
