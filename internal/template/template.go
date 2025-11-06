package template

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/serpro69/gh-arc/internal/diff"
	"github.com/serpro69/gh-arc/internal/github"
	"github.com/serpro69/gh-arc/internal/logger"
)

const (
	// Template field markers
	markerTitle      = "# Title:"
	markerSummary    = "# Summary:"
	markerTestPlan   = "# Test Plan:"
	markerReviewers  = "# Reviewers:"
	markerRef        = "# Ref:"
	markerDraft      = "# Draft:"
	markerBaseBranch = "# Base Branch:"

	// Template section markers
	sectionStart = "# =========="
	sectionEnd   = "# ----------"

	// Template file prefix
	templatePrefix = "gh-arc-diff-"
)

var (
	// ErrEditorCancelled is returned when user cancels editor
	ErrEditorCancelled = errors.New("editor cancelled: template unchanged or empty")

	// ErrNoEditor is returned when no editor is available
	ErrNoEditor = errors.New("no editor available: set $EDITOR environment variable")
)

// TemplateFields contains all the structured data from the template
type TemplateFields struct {
	Title      string
	Summary    string
	TestPlan   string
	Reviewers  []string // List of @usernames or @org/team
	Ref        []string // Linear issue references
	Draft      bool     // Whether PR should be created as draft
	BaseBranch string   // Read-only display of base branch
}

// StackingContext contains stacking information for template header
type StackingContext struct {
	IsStacking       bool
	BaseBranch       string
	ParentPR         *github.PullRequest
	DependentPRs     []*github.PullRequest
	CurrentBranch    string
	ShowDependents   bool
}

// TemplateGenerator generates PR templates with stacking context
type TemplateGenerator struct {
	stackingContext *StackingContext
	analysis        *diff.CommitAnalysis
	reviewers       []string
	linearEnabled   bool   // Whether to show Linear Ref field
	defaultDraft    bool   // Default draft status from config
}

// NewTemplateGenerator creates a new template generator
func NewTemplateGenerator(stackingCtx *StackingContext, analysis *diff.CommitAnalysis, reviewers []string, linearEnabled bool, defaultDraft bool) *TemplateGenerator {
	return &TemplateGenerator{
		stackingContext: stackingCtx,
		analysis:        analysis,
		reviewers:       reviewers,
		linearEnabled:   linearEnabled,
		defaultDraft:    defaultDraft,
	}
}

// Generate creates the template content
func (g *TemplateGenerator) Generate() string {
	var sb strings.Builder

	// Header with stacking context
	g.writeHeader(&sb)

	// Main template fields
	g.writeFields(&sb)

	// Footer with instructions
	g.writeFooter(&sb)

	return sb.String()
}

func (g *TemplateGenerator) writeHeader(sb *strings.Builder) {
	sb.WriteString(sectionStart + "\n")
	sb.WriteString("# Pull Request Template\n")
	sb.WriteString("#\n")

	// Stacking information in header
	if g.stackingContext != nil && g.stackingContext.IsStacking {
		sb.WriteString(fmt.Sprintf("# 📚 Creating stacked PR on %s", g.stackingContext.BaseBranch))
		if g.stackingContext.ParentPR != nil {
			sb.WriteString(fmt.Sprintf(" (PR #%d: %s)", g.stackingContext.ParentPR.Number, g.stackingContext.ParentPR.Title))
		}
		sb.WriteString("\n#\n")
	} else if g.stackingContext != nil {
		sb.WriteString(fmt.Sprintf("# Creating PR: %s → %s\n", g.stackingContext.CurrentBranch, g.stackingContext.BaseBranch))
		sb.WriteString("#\n")
	}

	// Dependent PRs warning
	if g.stackingContext != nil && g.stackingContext.ShowDependents && len(g.stackingContext.DependentPRs) > 0 {
		sb.WriteString("# ⚠️  WARNING: Dependent PRs target this branch:\n")
		for _, dep := range g.stackingContext.DependentPRs {
			sb.WriteString(fmt.Sprintf("#    • PR #%d: %s (@%s)\n", dep.Number, dep.Title, dep.User.Login))
		}
		sb.WriteString("#\n")
	}

	sb.WriteString("# Fill in the fields below. Lines starting with # are ignored.\n")
	sb.WriteString("# Required fields: Title, Test Plan\n")
	sb.WriteString(sectionEnd + "\n\n")
}

func (g *TemplateGenerator) writeFields(sb *strings.Builder) {
	// Title (pre-filled from analysis)
	sb.WriteString(markerTitle + "\n")
	if g.analysis != nil && g.analysis.Title != "" {
		sb.WriteString(g.analysis.Title + "\n")
	}
	sb.WriteString("\n")

	// Summary (pre-filled from analysis)
	sb.WriteString(markerSummary + "\n")
	if g.analysis != nil && g.analysis.Summary != "" {
		sb.WriteString(g.analysis.Summary + "\n")
	} else {
		sb.WriteString("# Optional summary of the changes\n")
	}
	sb.WriteString("\n")

	// Test Plan (empty, required)
	sb.WriteString(markerTestPlan + "\n")
	sb.WriteString("# Describe how you tested these changes\n")
	sb.WriteString("\n")

	// Reviewers (suggestions from CODEOWNERS)
	sb.WriteString(markerReviewers + "\n")
	sb.WriteString("# Comma-separated list of @usernames or @org/team\n")
	if len(g.reviewers) > 0 {
		sb.WriteString("# Suggestions: " + strings.Join(g.reviewers, ", ") + "\n")
	}
	sb.WriteString("\n")

	// Draft (whether PR should be created as draft)
	sb.WriteString(markerDraft + "\n")
	sb.WriteString("# Set to 'true' or 'false' to control draft status\n")
	if g.defaultDraft {
		sb.WriteString("true\n")
	} else {
		sb.WriteString("false\n")
	}
	sb.WriteString("\n")

	// Ref (Linear issue references) - only show if Linear is enabled
	if g.linearEnabled {
		sb.WriteString(markerRef + "\n")
		sb.WriteString("# Comma-separated Linear issue IDs (e.g., ENG-123, ENG-456)\n")
		sb.WriteString("\n")
	}

	// Base Branch (read-only display)
	if g.stackingContext != nil {
		sb.WriteString(markerBaseBranch + " " + g.stackingContext.BaseBranch + " (read-only)\n")
	}
}

func (g *TemplateGenerator) writeFooter(sb *strings.Builder) {
	sb.WriteString("\n")
	sb.WriteString(sectionStart + "\n")

	// Only add vim modeline if vim/nvim is the editor
	editor := os.Getenv("EDITOR")
	if strings.Contains(editor, "vim") || strings.Contains(editor, "nvim") {
		sb.WriteString("# vim: set filetype=gitcommit:\n")
	}
}

// ExtractBranchInfo extracts head and base branch names from template header
// Returns (headBranch, baseBranch, found)
func ExtractBranchInfo(content string) (string, string, bool) {
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := scanner.Text()

		// Look for "# Creating PR: <head> → <base>" or "# 📚 Creating stacked PR on <base>"
		if strings.Contains(line, "# Creating PR:") {
			// Extract "head → base" format
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				branchPart := strings.TrimSpace(parts[1])
				branches := strings.Split(branchPart, "→")
				if len(branches) == 2 {
					head := strings.TrimSpace(branches[0])
					base := strings.TrimSpace(branches[1])
					return head, base, true
				}
			}
		}

		// Look for "# Base Branch: <base> (read-only)"
		if strings.HasPrefix(line, markerBaseBranch) {
			// Extract base from "# Base Branch: main (read-only)"
			basePart := strings.TrimPrefix(line, markerBaseBranch)
			basePart = strings.TrimSpace(basePart)
			if idx := strings.Index(basePart, "(read-only)"); idx > 0 {
				basePart = strings.TrimSpace(basePart[:idx])
			}
			// We found base but not head yet from the format above
			// Continue scanning for the "Creating PR" line
			if basePart != "" {
				// Keep this as fallback base, continue looking for full header
				continue
			}
		}
	}

	return "", "", false
}

// ExtractBaseBranch extracts only the base branch name from template header
// This is useful for continue mode where head branch is obtained from git
// Returns (baseBranch, found)
func ExtractBaseBranch(content string) (string, bool) {
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := scanner.Text()

		// Look for "# Creating PR: <head> → <base>"
		if strings.Contains(line, "# Creating PR:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				branchPart := strings.TrimSpace(parts[1])
				branches := strings.Split(branchPart, "→")
				if len(branches) == 2 {
					base := strings.TrimSpace(branches[1])
					return base, true
				}
			}
		}

		// Look for "# 📚 Creating stacked PR on <base>" or "# 📚 Creating stacked PR on <base> (PR #123: ...)"
		if strings.Contains(line, "# 📚 Creating stacked PR on") {
			// Extract base from "# 📚 Creating stacked PR on feature/parent" or
			// "# 📚 Creating stacked PR on feature/parent (PR #123: Title)"
			parts := strings.Split(line, " on ")
			if len(parts) >= 2 {
				basePart := strings.TrimSpace(parts[1])
				// Remove optional PR info in parentheses
				if idx := strings.Index(basePart, "("); idx > 0 {
					basePart = strings.TrimSpace(basePart[:idx])
				}
				if basePart != "" {
					return basePart, true
				}
			}
		}

		// Look for "# Base Branch: <base> (read-only)"
		if strings.HasPrefix(line, markerBaseBranch) {
			basePart := strings.TrimPrefix(line, markerBaseBranch)
			basePart = strings.TrimSpace(basePart)
			if idx := strings.Index(basePart, "(read-only)"); idx > 0 {
				basePart = strings.TrimSpace(basePart[:idx])
			}
			if basePart != "" {
				return basePart, true
			}
		}
	}

	return "", false
}

// ParseTemplate parses the template content into structured fields
func ParseTemplate(content string) (*TemplateFields, error) {
	fields := &TemplateFields{
		Reviewers: []string{},
		Ref:       []string{},
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	currentSection := ""
	var currentValue strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Check for section markers (they are comment lines)
		if strings.HasPrefix(line, markerTitle) {
			// Save previous section
			if currentSection != "" {
				fields.setField(currentSection, strings.TrimSpace(currentValue.String()))
			}
			currentSection = markerTitle
			currentValue.Reset()
			continue
		}
		if strings.HasPrefix(line, markerSummary) {
			if currentSection != "" {
				fields.setField(currentSection, strings.TrimSpace(currentValue.String()))
			}
			currentSection = markerSummary
			currentValue.Reset()
			continue
		}
		if strings.HasPrefix(line, markerTestPlan) {
			if currentSection != "" {
				fields.setField(currentSection, strings.TrimSpace(currentValue.String()))
			}
			currentSection = markerTestPlan
			currentValue.Reset()
			continue
		}
		if strings.HasPrefix(line, markerReviewers) {
			if currentSection != "" {
				fields.setField(currentSection, strings.TrimSpace(currentValue.String()))
			}
			currentSection = markerReviewers
			currentValue.Reset()
			continue
		}
		if strings.HasPrefix(line, markerDraft) {
			if currentSection != "" {
				fields.setField(currentSection, strings.TrimSpace(currentValue.String()))
			}
			currentSection = markerDraft
			currentValue.Reset()
			continue
		}
		if strings.HasPrefix(line, markerRef) {
			if currentSection != "" {
				fields.setField(currentSection, strings.TrimSpace(currentValue.String()))
			}
			currentSection = markerRef
			currentValue.Reset()
			continue
		}
		if strings.HasPrefix(line, markerBaseBranch) {
			// Base branch is read-only, save previous section and clear current
			if currentSection != "" {
				fields.setField(currentSection, strings.TrimSpace(currentValue.String()))
			}
			currentSection = ""
			currentValue.Reset()
			continue
		}

		// Skip section dividers
		if strings.HasPrefix(line, sectionStart) || strings.HasPrefix(line, sectionEnd) {
			continue
		}

		// Skip other comment lines (but not section markers which we already handled)
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Accumulate content for current section
		if currentSection != "" {
			if currentValue.Len() > 0 {
				currentValue.WriteString("\n")
			}
			currentValue.WriteString(line)
		}
	}

	// Save last section
	if currentSection != "" {
		fields.setField(currentSection, strings.TrimSpace(currentValue.String()))
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error parsing template: %w", err)
	}

	return fields, nil
}

// setField sets the appropriate field based on the marker
func (f *TemplateFields) setField(marker, value string) {
	switch marker {
	case markerTitle:
		f.Title = value
	case markerSummary:
		f.Summary = value
	case markerTestPlan:
		f.TestPlan = value
	case markerReviewers:
		f.Reviewers = parseCommaSeparatedList(value)
	case markerDraft:
		// Parse boolean value (default to false if invalid)
		f.Draft = strings.EqualFold(strings.TrimSpace(value), "true")
	case markerRef:
		f.Ref = parseCommaSeparatedList(value)
	}
}

// parseCommaSeparatedList splits and cleans a comma-separated list
func parseCommaSeparatedList(value string) []string {
	if value == "" {
		return []string{}
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// OpenEditor opens the template in the user's $EDITOR
func OpenEditor(templateContent string) (string, error) {
	// Get editor from environment
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Try common fallbacks
		for _, fallback := range []string{"vi", "vim", "nano", "emacs"} {
			if _, err := exec.LookPath(fallback); err == nil {
				editor = fallback
				break
			}
		}
	}

	if editor == "" {
		return "", ErrNoEditor
	}

	logger.Debug().
		Str("editor", editor).
		Msg("Opening editor")

	// Create temporary file
	tmpFile, err := os.CreateTemp("", templatePrefix+"*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up after we're done

	// Write template content
	if _, err := tmpFile.WriteString(templateContent); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write template: %w", err)
	}
	tmpFile.Close()

	// Get original file stat for comparison
	originalStat, err := os.Stat(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat template file: %w", err)
	}

	// Open editor
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor exited with error: %w", err)
	}

	// Check if file was modified
	newStat, err := os.Stat(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat template file after editing: %w", err)
	}

	// Check if file is empty or unchanged
	if newStat.Size() == 0 {
		return "", ErrEditorCancelled
	}

	// Read the edited content
	editedContent, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read edited template: %w", err)
	}

	// Check if content is effectively empty (only comments)
	if isTemplateEmpty(string(editedContent)) {
		return "", ErrEditorCancelled
	}

	// Check if content is unchanged (compare sizes and mod times as heuristic)
	if originalStat.Size() == newStat.Size() && originalStat.ModTime().Equal(newStat.ModTime()) {
		logger.Debug().Msg("Template unchanged after editing")
		// Still return content - user may have just reviewed it
	}

	return string(editedContent), nil
}

// isTemplateEmpty checks if template content is effectively empty (only comments/whitespace)
func isTemplateEmpty(content string) bool {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// If we find any non-comment, non-empty line, it's not empty
		if line != "" && !strings.HasPrefix(line, "#") {
			return false
		}
	}
	return true
}

// SaveTemplate saves template content to a file for --continue support
func SaveTemplate(content string) (string, error) {
	tmpFile, err := os.CreateTemp("", templatePrefix+"saved-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create saved template file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		return "", fmt.Errorf("failed to write saved template: %w", err)
	}

	return tmpFile.Name(), nil
}

// LoadSavedTemplate loads a previously saved template
func LoadSavedTemplate(path string) (string, error) {
	if path == "" {
		return "", errors.New("no saved template path provided")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read saved template: %w", err)
	}

	return string(content), nil
}

// RemoveSavedTemplate removes a saved template file
func RemoveSavedTemplate(path string) error {
	if path == "" {
		return nil
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove saved template: %w", err)
	}

	return nil
}

// FindSavedTemplates finds all saved template files in temp directory
func FindSavedTemplates() ([]string, error) {
	tmpDir := os.TempDir()
	pattern := filepath.Join(tmpDir, templatePrefix+"saved-*.md")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find saved templates: %w", err)
	}

	// Sort by modification time (newest first)
	// This ensures we always get the most recently saved template
	type fileWithTime struct {
		path    string
		modTime time.Time
	}

	var files []fileWithTime
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			// Skip files we can't stat
			logger.Warn().Err(err).Str("path", path).Msg("Failed to stat template file")
			continue
		}
		files = append(files, fileWithTime{
			path:    path,
			modTime: info.ModTime(),
		})
	}

	// Sort by modification time, newest first
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	// Extract sorted paths
	sortedPaths := make([]string, len(files))
	for i, f := range files {
		sortedPaths[i] = f.path
	}

	return sortedPaths, nil
}

// GetEditorCommand returns the editor command to use
func GetEditorCommand() (string, error) {
	editor := os.Getenv("EDITOR")
	if editor != "" {
		return editor, nil
	}

	// Try common fallbacks
	for _, fallback := range []string{"vi", "vim", "nano", "emacs"} {
		if _, err := exec.LookPath(fallback); err == nil {
			return fallback, nil
		}
	}

	return "", ErrNoEditor
}

// ValidateFields validates that required template fields are filled
// stackingCtx can be nil for non-stacking scenarios
func ValidateFields(fields *TemplateFields, requireTestPlan bool, stackingCtx *StackingContext) []error {
	var errs []error

	// Title is always required
	if fields.Title == "" {
		if stackingCtx != nil && stackingCtx.IsStacking {
			errs = append(errs, fmt.Errorf("Title is required for stacked PR on %s", stackingCtx.BaseBranch))
		} else {
			errs = append(errs, errors.New("Title is required"))
		}
	}

	// Test Plan required if configured
	if requireTestPlan && fields.TestPlan == "" {
		if stackingCtx != nil && stackingCtx.IsStacking && stackingCtx.ParentPR != nil {
			errs = append(errs, fmt.Errorf("Test Plan is required for stacked PR on %s (PR #%d)",
				stackingCtx.BaseBranch, stackingCtx.ParentPR.Number))
		} else if stackingCtx != nil && stackingCtx.IsStacking {
			errs = append(errs, fmt.Errorf("Test Plan is required for stacked PR on %s", stackingCtx.BaseBranch))
		} else {
			errs = append(errs, errors.New("Test Plan is required"))
		}
	}

	// Validate reviewer format (should start with @)
	for _, reviewer := range fields.Reviewers {
		if !strings.HasPrefix(reviewer, "@") {
			errs = append(errs, fmt.Errorf("invalid reviewer format: %s (should start with @)", reviewer))
		}
	}

	return errs
}

// FormatValidationErrors formats validation errors for display with stacking context
// stackingCtx can be nil for non-stacking scenarios
func FormatValidationErrors(errs []error, stackingCtx *StackingContext) string {
	if len(errs) == 0 {
		return ""
	}

	var sb strings.Builder

	// Header with stacking context
	if stackingCtx != nil && stackingCtx.IsStacking {
		sb.WriteString("✗ Template validation failed for stacked PR:\n")
		if stackingCtx.ParentPR != nil {
			sb.WriteString(fmt.Sprintf("  Stack: %s → %s (PR #%d)\n",
				stackingCtx.CurrentBranch, stackingCtx.BaseBranch, stackingCtx.ParentPR.Number))
		} else {
			sb.WriteString(fmt.Sprintf("  Stack: %s → %s\n",
				stackingCtx.CurrentBranch, stackingCtx.BaseBranch))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("✗ Template validation failed:\n")
	}

	// List errors
	for _, err := range errs {
		sb.WriteString(fmt.Sprintf("  • %s\n", err.Error()))
	}

	// Recovery suggestion
	sb.WriteString("\nUse 'gh arc diff --continue' to retry editing.\n")

	return sb.String()
}

// GetStackingInfo returns a formatted string with stacking information for display
func GetStackingInfo(stackingCtx *StackingContext) string {
	if stackingCtx == nil || !stackingCtx.IsStacking {
		return ""
	}

	if stackingCtx.ParentPR != nil {
		return fmt.Sprintf("📚 Stacking on %s (PR #%d: %s)",
			stackingCtx.BaseBranch, stackingCtx.ParentPR.Number, stackingCtx.ParentPR.Title)
	}

	return fmt.Sprintf("📚 Stacking on %s", stackingCtx.BaseBranch)
}

// GetDependentPRsWarning returns a formatted warning about dependent PRs
func GetDependentPRsWarning(stackingCtx *StackingContext) string {
	if stackingCtx == nil || !stackingCtx.ShowDependents || len(stackingCtx.DependentPRs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("⚠️  WARNING: %d dependent PR(s) target this branch:\n",
		len(stackingCtx.DependentPRs)))

	for _, dep := range stackingCtx.DependentPRs {
		sb.WriteString(fmt.Sprintf("   • PR #%d: %s (@%s)\n",
			dep.Number, dep.Title, dep.User.Login))
	}

	return sb.String()
}

// ValidateFieldsWithContext is a convenience wrapper that combines validation and formatting
func ValidateFieldsWithContext(fields *TemplateFields, requireTestPlan bool, stackingCtx *StackingContext) (bool, string) {
	errs := ValidateFields(fields, requireTestPlan, stackingCtx)
	if len(errs) == 0 {
		return true, ""
	}

	return false, FormatValidationErrors(errs, stackingCtx)
}

// WriteTemplateTo writes template content to a writer (for testing)
func WriteTemplateTo(w io.Writer, templateContent string) error {
	_, err := w.Write([]byte(templateContent))
	return err
}

// ReadTemplateFrom reads template content from a reader (for testing)
func ReadTemplateFrom(r io.Reader) (string, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
