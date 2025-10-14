package template

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/serpro69/gh-arc/internal/diff"
	"github.com/serpro69/gh-arc/internal/github"
)

// Test TemplateGenerator creation
func TestNewTemplateGenerator(t *testing.T) {
	stackingCtx := &StackingContext{
		IsStacking:    true,
		BaseBranch:    "main",
		CurrentBranch: "feature",
	}
	analysis := &diff.CommitAnalysis{
		Title:   "Test feature",
		Summary: "Add test",
	}
	reviewers := []string{"@user1", "@team/reviewers"}

	gen := NewTemplateGenerator(stackingCtx, analysis, reviewers, false, false)

	if gen == nil {
		t.Fatal("NewTemplateGenerator returned nil")
	}
	if gen.stackingContext != stackingCtx {
		t.Error("stacking context not set correctly")
	}
	if gen.analysis != analysis {
		t.Error("analysis not set correctly")
	}
	if len(gen.reviewers) != 2 {
		t.Errorf("reviewers not set correctly, got %d want 2", len(gen.reviewers))
	}
}

// Test template generation without stacking
func TestGenerateTemplateBasic(t *testing.T) {
	analysis := &diff.CommitAnalysis{
		Title:   "Add new feature",
		Summary: "This commit adds a new feature",
	}

	gen := NewTemplateGenerator(nil, analysis, []string{}, false, false)
	content := gen.Generate()

	// Check for required sections
	if !strings.Contains(content, markerTitle) {
		t.Error("Template missing Title marker")
	}
	if !strings.Contains(content, markerSummary) {
		t.Error("Template missing Summary marker")
	}
	if !strings.Contains(content, markerTestPlan) {
		t.Error("Template missing Test Plan marker")
	}
	if !strings.Contains(content, markerReviewers) {
		t.Error("Template missing Reviewers marker")
	}
	if !strings.Contains(content, markerDraft) {
		t.Error("Template missing Draft marker")
	}
	// Ref marker should NOT be present when linearEnabled=false
	if strings.Contains(content, markerRef) {
		t.Error("Template should not have Ref marker when Linear is disabled")
	}

	// Check for pre-filled content
	if !strings.Contains(content, "Add new feature") {
		t.Error("Template missing pre-filled title")
	}
	if !strings.Contains(content, "This commit adds a new feature") {
		t.Error("Template missing pre-filled summary")
	}
	// Check for default draft value (false)
	if !strings.Contains(content, "false") {
		t.Error("Template missing default draft value")
	}
}

// Test template generation with stacking context
func TestGenerateTemplateWithStacking(t *testing.T) {
	stackingCtx := &StackingContext{
		IsStacking:    true,
		BaseBranch:    "feature/parent",
		CurrentBranch: "feature/child",
		ParentPR: &github.PullRequest{
			Number: 100,
			Title:  "Parent feature",
		},
	}

	analysis := &diff.CommitAnalysis{
		Title:   "Child feature",
		Summary: "Builds on parent",
	}

	gen := NewTemplateGenerator(stackingCtx, analysis, []string{"@reviewer1"}, false, false)
	content := gen.Generate()

	// Check for stacking information
	if !strings.Contains(content, "📚 Creating stacked PR") {
		t.Error("Template missing stacking indicator")
	}
	if !strings.Contains(content, "feature/parent") {
		t.Error("Template missing parent branch name")
	}
	if !strings.Contains(content, "PR #100") {
		t.Error("Template missing parent PR number")
	}
	if !strings.Contains(content, "Parent feature") {
		t.Error("Template missing parent PR title")
	}
}

// Test template generation with dependent PRs warning
func TestGenerateTemplateWithDependents(t *testing.T) {
	dependentPRs := []*github.PullRequest{
		{
			Number: 101,
			Title:  "Dependent feature 1",
			User:   github.PRUser{Login: "alice"},
		},
		{
			Number: 102,
			Title:  "Dependent feature 2",
			User:   github.PRUser{Login: "bob"},
		},
	}

	stackingCtx := &StackingContext{
		IsStacking:     false,
		BaseBranch:     "main",
		CurrentBranch:  "feature/parent",
		DependentPRs:   dependentPRs,
		ShowDependents: true,
	}

	gen := NewTemplateGenerator(stackingCtx, nil, []string{}, false, false)
	content := gen.Generate()

	// Check for dependent PR warnings
	if !strings.Contains(content, "⚠️  WARNING: Dependent PRs target this branch:") {
		t.Error("Template missing dependent PRs warning")
	}
	if !strings.Contains(content, "PR #101") {
		t.Error("Template missing dependent PR #101")
	}
	if !strings.Contains(content, "PR #102") {
		t.Error("Template missing dependent PR #102")
	}
	if !strings.Contains(content, "@alice") {
		t.Error("Template missing dependent PR author alice")
	}
}

// Test template generation with reviewer suggestions
func TestGenerateTemplateWithReviewers(t *testing.T) {
	reviewers := []string{"@user1", "@user2", "@org/team"}

	gen := NewTemplateGenerator(nil, nil, reviewers, false, false)
	content := gen.Generate()

	// Check for reviewer suggestions
	if !strings.Contains(content, "Suggestions:") {
		t.Error("Template missing reviewer suggestions")
	}
	if !strings.Contains(content, "@user1") {
		t.Error("Template missing reviewer @user1")
	}
	if !strings.Contains(content, "@org/team") {
		t.Error("Template missing reviewer @org/team")
	}
}

// Test parsing empty template
func TestParseTemplateEmpty(t *testing.T) {
	content := `
# Title:

# Summary:

# Test Plan:

# Reviewers:

# Ref:
`

	fields, err := ParseTemplate(content)
	if err != nil {
		t.Fatalf("ParseTemplate failed: %v", err)
	}

	if fields.Title != "" {
		t.Errorf("Expected empty title, got %q", fields.Title)
	}
	if fields.Summary != "" {
		t.Errorf("Expected empty summary, got %q", fields.Summary)
	}
	if fields.TestPlan != "" {
		t.Errorf("Expected empty test plan, got %q", fields.TestPlan)
	}
	if len(fields.Reviewers) != 0 {
		t.Errorf("Expected empty reviewers, got %v", fields.Reviewers)
	}
	if len(fields.Ref) != 0 {
		t.Errorf("Expected empty refs, got %v", fields.Ref)
	}
}

// Test parsing filled template
func TestParseTemplateFilled(t *testing.T) {
	content := `
# Title:
Add authentication system

# Summary:
This PR implements user authentication with JWT tokens.
Includes login, logout, and token refresh endpoints.

# Test Plan:
1. Tested login endpoint with valid credentials
2. Tested logout functionality
3. Verified token refresh works correctly
4. Added unit tests for auth service

# Reviewers:
@alice, @bob, @org/security-team

# Ref:
ENG-123, ENG-124

# Base Branch: main (read-only)
`

	fields, err := ParseTemplate(content)
	if err != nil {
		t.Fatalf("ParseTemplate failed: %v", err)
	}

	// Check title
	if fields.Title != "Add authentication system" {
		t.Errorf("Title = %q, want %q", fields.Title, "Add authentication system")
	}

	// Check summary (multiline)
	expectedSummary := "This PR implements user authentication with JWT tokens.\nIncludes login, logout, and token refresh endpoints."
	if fields.Summary != expectedSummary {
		t.Errorf("Summary = %q, want %q", fields.Summary, expectedSummary)
	}

	// Check test plan (multiline with numbers)
	if !strings.Contains(fields.TestPlan, "Tested login endpoint") {
		t.Error("TestPlan missing expected content")
	}
	if !strings.Contains(fields.TestPlan, "Added unit tests") {
		t.Error("TestPlan missing expected content")
	}

	// Check reviewers
	expectedReviewers := []string{"@alice", "@bob", "@org/security-team"}
	if len(fields.Reviewers) != len(expectedReviewers) {
		t.Errorf("Reviewers count = %d, want %d", len(fields.Reviewers), len(expectedReviewers))
	}
	for i, expected := range expectedReviewers {
		if i < len(fields.Reviewers) && fields.Reviewers[i] != expected {
			t.Errorf("Reviewers[%d] = %q, want %q", i, fields.Reviewers[i], expected)
		}
	}

	// Check refs
	expectedRefs := []string{"ENG-123", "ENG-124"}
	if len(fields.Ref) != len(expectedRefs) {
		t.Errorf("Ref count = %d, want %d", len(fields.Ref), len(expectedRefs))
	}
}

// Test parsing with comments and extra whitespace
func TestParseTemplateWithComments(t *testing.T) {
	content := `
# ========== Header comments
# Pull Request Template
# ==========

# Title:
   Feature title with spaces

# Summary:
# This is a comment line that should be ignored
Summary with trailing spaces

# Test Plan:
# Add your test plan here
Test plan content

# Reviewers:
@user1  ,  @user2

# Ref:
  ENG-100  ,  ENG-200

# Base Branch: main (read-only)
`

	fields, err := ParseTemplate(content)
	if err != nil {
		t.Fatalf("ParseTemplate failed: %v", err)
	}

	// Check trimming
	if fields.Title != "Feature title with spaces" {
		t.Errorf("Title not trimmed correctly: %q", fields.Title)
	}
	if fields.Summary != "Summary with trailing spaces" {
		t.Errorf("Summary not trimmed correctly: %q", fields.Summary)
	}

	// Check reviewers trimmed
	if len(fields.Reviewers) != 2 || fields.Reviewers[0] != "@user1" || fields.Reviewers[1] != "@user2" {
		t.Errorf("Reviewers not parsed correctly: %v", fields.Reviewers)
	}

	// Check refs trimmed
	if len(fields.Ref) != 2 || fields.Ref[0] != "ENG-100" || fields.Ref[1] != "ENG-200" {
		t.Errorf("Refs not parsed correctly: %v", fields.Ref)
	}
}

// Test parseCommaSeparatedList
func TestParseCommaSeparatedList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single item",
			input:    "@user1",
			expected: []string{"@user1"},
		},
		{
			name:     "multiple items",
			input:    "@user1, @user2, @user3",
			expected: []string{"@user1", "@user2", "@user3"},
		},
		{
			name:     "items with extra spaces",
			input:    "  @user1  ,  @user2  ,  @user3  ",
			expected: []string{"@user1", "@user2", "@user3"},
		},
		{
			name:     "trailing comma",
			input:    "@user1, @user2,",
			expected: []string{"@user1", "@user2"},
		},
		{
			name:     "empty items",
			input:    "@user1, , @user2",
			expected: []string{"@user1", "@user2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommaSeparatedList(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("length = %d, want %d", len(result), len(tt.expected))
				return
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("result[%d] = %q, want %q", i, result[i], exp)
				}
			}
		})
	}
}

// Test isTemplateEmpty
func TestIsTemplateEmpty(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "completely empty",
			content:  "",
			expected: true,
		},
		{
			name:     "only whitespace",
			content:  "   \n\n   \n",
			expected: true,
		},
		{
			name:     "only comments",
			content:  "# Comment 1\n# Comment 2\n",
			expected: true,
		},
		{
			name:     "comments and whitespace",
			content:  "# Comment\n\n   \n# Another comment",
			expected: true,
		},
		{
			name:     "has content",
			content:  "# Comment\nActual content\n",
			expected: false,
		},
		{
			name:     "single word",
			content:  "Title",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTemplateEmpty(tt.content)
			if result != tt.expected {
				t.Errorf("isTemplateEmpty() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test ValidateFields without stacking context
func TestValidateFields(t *testing.T) {
	tests := []struct {
		name            string
		fields          *TemplateFields
		requireTestPlan bool
		expectErrors    bool
		errorContains   []string
	}{
		{
			name: "valid with all fields",
			fields: &TemplateFields{
				Title:     "Feature",
				TestPlan:  "Tested manually",
				Reviewers: []string{"@user1"},
			},
			requireTestPlan: true,
			expectErrors:    false,
		},
		{
			name: "missing title",
			fields: &TemplateFields{
				TestPlan: "Tested",
			},
			requireTestPlan: true,
			expectErrors:    true,
			errorContains:   []string{"Title is required"},
		},
		{
			name: "missing test plan when required",
			fields: &TemplateFields{
				Title: "Feature",
			},
			requireTestPlan: true,
			expectErrors:    true,
			errorContains:   []string{"Test Plan is required"},
		},
		{
			name: "missing test plan when not required",
			fields: &TemplateFields{
				Title: "Feature",
			},
			requireTestPlan: false,
			expectErrors:    false,
		},
		{
			name: "invalid reviewer format",
			fields: &TemplateFields{
				Title:     "Feature",
				TestPlan:  "Tested",
				Reviewers: []string{"user1", "@user2"},
			},
			requireTestPlan: true,
			expectErrors:    true,
			errorContains:   []string{"invalid reviewer format: user1"},
		},
		{
			name: "multiple errors",
			fields: &TemplateFields{
				Reviewers: []string{"user1", "user2"},
			},
			requireTestPlan: true,
			expectErrors:    true,
			errorContains:   []string{"Title is required", "Test Plan is required", "invalid reviewer format"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateFields(tt.fields, tt.requireTestPlan, nil)

			if tt.expectErrors && len(errs) == 0 {
				t.Error("Expected validation errors but got none")
			}
			if !tt.expectErrors && len(errs) > 0 {
				t.Errorf("Expected no errors but got: %v", errs)
			}

			for _, contains := range tt.errorContains {
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), contains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing %q but not found in %v", contains, errs)
				}
			}
		})
	}
}

// Test ValidateFields with stacking context
func TestValidateFieldsWithStackingContext(t *testing.T) {
	tests := []struct {
		name            string
		fields          *TemplateFields
		requireTestPlan bool
		stackingCtx     *StackingContext
		expectErrors    bool
		errorContains   []string
	}{
		{
			name: "valid stacked PR with all fields",
			fields: &TemplateFields{
				Title:     "Child feature",
				TestPlan:  "Tested with parent",
				Reviewers: []string{"@user1"},
			},
			requireTestPlan: true,
			stackingCtx: &StackingContext{
				IsStacking:    true,
				BaseBranch:    "feature/parent",
				CurrentBranch: "feature/child",
			},
			expectErrors: false,
		},
		{
			name: "missing title with stacking context",
			fields: &TemplateFields{
				TestPlan: "Tested",
			},
			requireTestPlan: true,
			stackingCtx: &StackingContext{
				IsStacking:    true,
				BaseBranch:    "feature/parent",
				CurrentBranch: "feature/child",
			},
			expectErrors:  true,
			errorContains: []string{"Title is required for stacked PR on feature/parent"},
		},
		{
			name: "missing test plan with stacking context and parent PR",
			fields: &TemplateFields{
				Title: "Child feature",
			},
			requireTestPlan: true,
			stackingCtx: &StackingContext{
				IsStacking:    true,
				BaseBranch:    "feature/parent",
				CurrentBranch: "feature/child",
				ParentPR: &github.PullRequest{
					Number: 100,
					Title:  "Parent feature",
				},
			},
			expectErrors:  true,
			errorContains: []string{"Test Plan is required for stacked PR on feature/parent (PR #100)"},
		},
		{
			name: "missing test plan with stacking context without parent PR",
			fields: &TemplateFields{
				Title: "Feature",
			},
			requireTestPlan: true,
			stackingCtx: &StackingContext{
				IsStacking:    true,
				BaseBranch:    "main",
				CurrentBranch: "feature",
			},
			expectErrors:  true,
			errorContains: []string{"Test Plan is required for stacked PR on main"},
		},
		{
			name: "non-stacking context behaves like nil",
			fields: &TemplateFields{
				Title: "Feature",
			},
			requireTestPlan: true,
			stackingCtx: &StackingContext{
				IsStacking: false,
			},
			expectErrors:  true,
			errorContains: []string{"Test Plan is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateFields(tt.fields, tt.requireTestPlan, tt.stackingCtx)

			if tt.expectErrors && len(errs) == 0 {
				t.Error("Expected validation errors but got none")
			}
			if !tt.expectErrors && len(errs) > 0 {
				t.Errorf("Expected no errors but got: %v", errs)
			}

			for _, contains := range tt.errorContains {
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), contains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing %q but not found in %v", contains, errs)
				}
			}
		})
	}
}

// Test FormatValidationErrors
func TestFormatValidationErrors(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		result := FormatValidationErrors([]error{}, nil)
		if result != "" {
			t.Errorf("Expected empty string for no errors, got %q", result)
		}
	})

	t.Run("single error without stacking", func(t *testing.T) {
		errs := []error{ErrEditorCancelled}
		result := FormatValidationErrors(errs, nil)

		if !strings.Contains(result, "✗ Template validation failed:") {
			t.Error("Missing validation failed header")
		}
		if !strings.Contains(result, ErrEditorCancelled.Error()) {
			t.Error("Missing error message")
		}
		if !strings.Contains(result, "gh arc diff --continue") {
			t.Error("Missing continue hint")
		}
	})

	t.Run("multiple errors without stacking", func(t *testing.T) {
		errs := ValidateFields(&TemplateFields{}, true, nil)
		result := FormatValidationErrors(errs, nil)

		if !strings.Contains(result, "Title is required") {
			t.Error("Missing title error")
		}
		if !strings.Contains(result, "Test Plan is required") {
			t.Error("Missing test plan error")
		}
	})

	t.Run("errors with stacking context without parent PR", func(t *testing.T) {
		stackingCtx := &StackingContext{
			IsStacking:    true,
			BaseBranch:    "main",
			CurrentBranch: "feature",
		}
		errs := ValidateFields(&TemplateFields{}, true, stackingCtx)
		result := FormatValidationErrors(errs, stackingCtx)

		if !strings.Contains(result, "✗ Template validation failed for stacked PR:") {
			t.Error("Missing stacked PR validation header")
		}
		if !strings.Contains(result, "Stack: feature → main") {
			t.Error("Missing stack hierarchy without PR number")
		}
		if !strings.Contains(result, "Title is required") {
			t.Error("Missing title error")
		}
	})

	t.Run("errors with stacking context with parent PR", func(t *testing.T) {
		stackingCtx := &StackingContext{
			IsStacking:    true,
			BaseBranch:    "feature/parent",
			CurrentBranch: "feature/child",
			ParentPR: &github.PullRequest{
				Number: 100,
				Title:  "Parent feature",
			},
		}
		errs := ValidateFields(&TemplateFields{}, true, stackingCtx)
		result := FormatValidationErrors(errs, stackingCtx)

		if !strings.Contains(result, "✗ Template validation failed for stacked PR:") {
			t.Error("Missing stacked PR validation header")
		}
		if !strings.Contains(result, "Stack: feature/child → feature/parent (PR #100)") {
			t.Error("Missing stack hierarchy with PR number")
		}
	})

	t.Run("non-stacking context behaves like nil", func(t *testing.T) {
		stackingCtx := &StackingContext{
			IsStacking: false,
		}
		errs := ValidateFields(&TemplateFields{}, true, stackingCtx)
		result := FormatValidationErrors(errs, stackingCtx)

		if !strings.Contains(result, "✗ Template validation failed:") {
			t.Error("Should use standard header for non-stacking")
		}
		if strings.Contains(result, "stacked PR") {
			t.Error("Should not mention stacking for non-stacking context")
		}
	})
}

// Test GetStackingInfo
func TestGetStackingInfo(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		result := GetStackingInfo(nil)
		if result != "" {
			t.Errorf("Expected empty string for nil context, got %q", result)
		}
	})

	t.Run("non-stacking context", func(t *testing.T) {
		stackingCtx := &StackingContext{
			IsStacking: false,
		}
		result := GetStackingInfo(stackingCtx)
		if result != "" {
			t.Errorf("Expected empty string for non-stacking, got %q", result)
		}
	})

	t.Run("stacking without parent PR", func(t *testing.T) {
		stackingCtx := &StackingContext{
			IsStacking: true,
			BaseBranch: "main",
		}
		result := GetStackingInfo(stackingCtx)
		expected := "📚 Stacking on main"
		if result != expected {
			t.Errorf("GetStackingInfo() = %q, want %q", result, expected)
		}
	})

	t.Run("stacking with parent PR", func(t *testing.T) {
		stackingCtx := &StackingContext{
			IsStacking: true,
			BaseBranch: "feature/parent",
			ParentPR: &github.PullRequest{
				Number: 100,
				Title:  "Parent feature",
			},
		}
		result := GetStackingInfo(stackingCtx)
		if !strings.Contains(result, "📚 Stacking on feature/parent") {
			t.Error("Missing base branch in stacking info")
		}
		if !strings.Contains(result, "PR #100") {
			t.Error("Missing PR number in stacking info")
		}
		if !strings.Contains(result, "Parent feature") {
			t.Error("Missing PR title in stacking info")
		}
	})
}

// Test GetDependentPRsWarning
func TestGetDependentPRsWarning(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		result := GetDependentPRsWarning(nil)
		if result != "" {
			t.Errorf("Expected empty string for nil context, got %q", result)
		}
	})

	t.Run("show dependents false", func(t *testing.T) {
		stackingCtx := &StackingContext{
			ShowDependents: false,
			DependentPRs: []*github.PullRequest{
				{Number: 101, Title: "Dependent"},
			},
		}
		result := GetDependentPRsWarning(stackingCtx)
		if result != "" {
			t.Errorf("Expected empty string when ShowDependents is false, got %q", result)
		}
	})

	t.Run("no dependent PRs", func(t *testing.T) {
		stackingCtx := &StackingContext{
			ShowDependents: true,
			DependentPRs:   []*github.PullRequest{},
		}
		result := GetDependentPRsWarning(stackingCtx)
		if result != "" {
			t.Errorf("Expected empty string when no dependent PRs, got %q", result)
		}
	})

	t.Run("single dependent PR", func(t *testing.T) {
		stackingCtx := &StackingContext{
			ShowDependents: true,
			DependentPRs: []*github.PullRequest{
				{
					Number: 101,
					Title:  "Dependent feature",
					User:   github.PRUser{Login: "alice"},
				},
			},
		}
		result := GetDependentPRsWarning(stackingCtx)
		if !strings.Contains(result, "⚠️  WARNING: 1 dependent PR(s) target this branch:") {
			t.Error("Missing warning header for single PR")
		}
		if !strings.Contains(result, "PR #101") {
			t.Error("Missing PR number")
		}
		if !strings.Contains(result, "Dependent feature") {
			t.Error("Missing PR title")
		}
		if !strings.Contains(result, "@alice") {
			t.Error("Missing PR author")
		}
	})

	t.Run("multiple dependent PRs", func(t *testing.T) {
		stackingCtx := &StackingContext{
			ShowDependents: true,
			DependentPRs: []*github.PullRequest{
				{
					Number: 101,
					Title:  "Dependent feature 1",
					User:   github.PRUser{Login: "alice"},
				},
				{
					Number: 102,
					Title:  "Dependent feature 2",
					User:   github.PRUser{Login: "bob"},
				},
			},
		}
		result := GetDependentPRsWarning(stackingCtx)
		if !strings.Contains(result, "⚠️  WARNING: 2 dependent PR(s) target this branch:") {
			t.Error("Missing warning header for multiple PRs")
		}
		if !strings.Contains(result, "PR #101") {
			t.Error("Missing first PR")
		}
		if !strings.Contains(result, "PR #102") {
			t.Error("Missing second PR")
		}
		if !strings.Contains(result, "@alice") {
			t.Error("Missing first author")
		}
		if !strings.Contains(result, "@bob") {
			t.Error("Missing second author")
		}
	})
}

// Test ValidateFieldsWithContext
func TestValidateFieldsWithContext(t *testing.T) {
	t.Run("valid fields returns true", func(t *testing.T) {
		fields := &TemplateFields{
			Title:     "Feature",
			TestPlan:  "Tested",
			Reviewers: []string{"@user1"},
		}
		valid, msg := ValidateFieldsWithContext(fields, true, nil)
		if !valid {
			t.Error("Expected valid=true for valid fields")
		}
		if msg != "" {
			t.Errorf("Expected empty message for valid fields, got %q", msg)
		}
	})

	t.Run("invalid fields returns formatted error", func(t *testing.T) {
		fields := &TemplateFields{}
		valid, msg := ValidateFieldsWithContext(fields, true, nil)
		if valid {
			t.Error("Expected valid=false for invalid fields")
		}
		if msg == "" {
			t.Error("Expected non-empty error message")
		}
		if !strings.Contains(msg, "Title is required") {
			t.Error("Message missing title error")
		}
		if !strings.Contains(msg, "Test Plan is required") {
			t.Error("Message missing test plan error")
		}
	})

	t.Run("invalid fields with stacking context", func(t *testing.T) {
		fields := &TemplateFields{}
		stackingCtx := &StackingContext{
			IsStacking:    true,
			BaseBranch:    "main",
			CurrentBranch: "feature",
		}
		valid, msg := ValidateFieldsWithContext(fields, true, stackingCtx)
		if valid {
			t.Error("Expected valid=false for invalid fields")
		}
		if !strings.Contains(msg, "stacked PR") {
			t.Error("Message should mention stacked PR")
		}
		if !strings.Contains(msg, "Stack: feature → main") {
			t.Error("Message should include stack hierarchy")
		}
	})
}

// Test GetEditorCommand
func TestGetEditorCommand(t *testing.T) {
	// Save and restore original EDITOR
	originalEditor := os.Getenv("EDITOR")
	defer os.Setenv("EDITOR", originalEditor)

	t.Run("with EDITOR set", func(t *testing.T) {
		os.Setenv("EDITOR", "custom-editor")
		editor, err := GetEditorCommand()
		if err != nil {
			t.Fatalf("GetEditorCommand failed: %v", err)
		}
		if editor != "custom-editor" {
			t.Errorf("Editor = %q, want %q", editor, "custom-editor")
		}
	})

	t.Run("without EDITOR, finds fallback", func(t *testing.T) {
		os.Setenv("EDITOR", "")
		editor, err := GetEditorCommand()
		// Should find at least one fallback (vi, vim, nano, emacs)
		if err != nil && err != ErrNoEditor {
			t.Fatalf("GetEditorCommand failed: %v", err)
		}
		// On most systems, at least one editor should be available
		if editor == "" && err == nil {
			t.Error("Expected either an editor or ErrNoEditor")
		}
	})
}

// Test WriteTemplateTo and ReadTemplateFrom
func TestTemplateIO(t *testing.T) {
	content := "Test template content\n"

	var buf bytes.Buffer

	// Test writing
	err := WriteTemplateTo(&buf, content)
	if err != nil {
		t.Fatalf("WriteTemplateTo failed: %v", err)
	}

	// Test reading
	readContent, err := ReadTemplateFrom(&buf)
	if err != nil {
		t.Fatalf("ReadTemplateFrom failed: %v", err)
	}

	if readContent != content {
		t.Errorf("Read content = %q, want %q", readContent, content)
	}
}

// Test SaveTemplate and LoadSavedTemplate
func TestSaveAndLoadTemplate(t *testing.T) {
	content := "Saved template content\nLine 2\n"

	// Save template
	path, err := SaveTemplate(content)
	if err != nil {
		t.Fatalf("SaveTemplate failed: %v", err)
	}
	defer os.Remove(path)

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Saved template file does not exist")
	}

	// Load template
	loaded, err := LoadSavedTemplate(path)
	if err != nil {
		t.Fatalf("LoadSavedTemplate failed: %v", err)
	}

	if loaded != content {
		t.Errorf("Loaded content = %q, want %q", loaded, content)
	}
}

// Test RemoveSavedTemplate
func TestRemoveSavedTemplate(t *testing.T) {
	content := "Template to remove\n"

	// Save template
	path, err := SaveTemplate(content)
	if err != nil {
		t.Fatalf("SaveTemplate failed: %v", err)
	}

	// Remove template
	err = RemoveSavedTemplate(path)
	if err != nil {
		t.Fatalf("RemoveSavedTemplate failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Saved template file still exists after removal")
	}

	// Removing non-existent file should not error
	err = RemoveSavedTemplate(path)
	if err != nil {
		t.Errorf("RemoveSavedTemplate on non-existent file failed: %v", err)
	}

	// Empty path should not error
	err = RemoveSavedTemplate("")
	if err != nil {
		t.Errorf("RemoveSavedTemplate with empty path failed: %v", err)
	}
}

// Test FindSavedTemplates
func TestFindSavedTemplates(t *testing.T) {
	// Create some saved templates
	content1 := "Template 1\n"
	content2 := "Template 2\n"

	path1, err := SaveTemplate(content1)
	if err != nil {
		t.Fatalf("SaveTemplate 1 failed: %v", err)
	}
	defer os.Remove(path1)

	path2, err := SaveTemplate(content2)
	if err != nil {
		t.Fatalf("SaveTemplate 2 failed: %v", err)
	}
	defer os.Remove(path2)

	// Find saved templates
	found, err := FindSavedTemplates()
	if err != nil {
		t.Fatalf("FindSavedTemplates failed: %v", err)
	}

	// Should find at least our two templates
	if len(found) < 2 {
		t.Errorf("Found %d templates, want at least 2", len(found))
	}

	// Check that our paths are in the results
	foundPath1, foundPath2 := false, false
	for _, p := range found {
		if p == path1 {
			foundPath1 = true
		}
		if p == path2 {
			foundPath2 = true
		}
	}

	if !foundPath1 {
		t.Error("path1 not found in FindSavedTemplates results")
	}
	if !foundPath2 {
		t.Error("path2 not found in FindSavedTemplates results")
	}
}

// Test Draft field parsing
func TestParseDraftField(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name: "draft true",
			content: `
# Title:
Test

# Draft:
true

# Base Branch: main (read-only)
`,
			expected: true,
		},
		{
			name: "draft True (capitalized)",
			content: `
# Title:
Test

# Draft:
True

# Base Branch: main (read-only)
`,
			expected: true,
		},
		{
			name: "draft false",
			content: `
# Title:
Test

# Draft:
false

# Base Branch: main (read-only)
`,
			expected: false,
		},
		{
			name: "draft False (capitalized)",
			content: `
# Title:
Test

# Draft:
False

# Base Branch: main (read-only)
`,
			expected: false,
		},
		{
			name: "draft invalid value defaults to false",
			content: `
# Title:
Test

# Draft:
invalid

# Base Branch: main (read-only)
`,
			expected: false,
		},
		{
			name: "draft empty defaults to false",
			content: `
# Title:
Test

# Draft:

# Base Branch: main (read-only)
`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := ParseTemplate(tt.content)
			if err != nil {
				t.Fatalf("ParseTemplate failed: %v", err)
			}
			if fields.Draft != tt.expected {
				t.Errorf("Draft = %v, want %v", fields.Draft, tt.expected)
			}
		})
	}
}

// Test Draft field generation with defaultDraft
func TestGenerateDraftField(t *testing.T) {
	t.Run("defaultDraft false", func(t *testing.T) {
		gen := NewTemplateGenerator(nil, nil, []string{}, false, false)
		content := gen.Generate()

		if !strings.Contains(content, markerDraft) {
			t.Error("Template missing Draft marker")
		}
		// Check that false is the default value
		lines := strings.Split(content, "\n")
		foundDraftMarker := false
		for i, line := range lines {
			if strings.HasPrefix(line, markerDraft) {
				foundDraftMarker = true
				// Next non-comment line should be "false"
				for j := i + 1; j < len(lines); j++ {
					if !strings.HasPrefix(lines[j], "#") && strings.TrimSpace(lines[j]) != "" {
						if strings.TrimSpace(lines[j]) != "false" {
							t.Errorf("Expected 'false' after Draft marker, got %q", lines[j])
						}
						break
					}
				}
				break
			}
		}
		if !foundDraftMarker {
			t.Error("Draft marker not found in template")
		}
	})

	t.Run("defaultDraft true", func(t *testing.T) {
		gen := NewTemplateGenerator(nil, nil, []string{}, false, true)
		content := gen.Generate()

		if !strings.Contains(content, markerDraft) {
			t.Error("Template missing Draft marker")
		}
		// Check that true is the default value
		lines := strings.Split(content, "\n")
		foundDraftMarker := false
		for i, line := range lines {
			if strings.HasPrefix(line, markerDraft) {
				foundDraftMarker = true
				// Next non-comment line should be "true"
				for j := i + 1; j < len(lines); j++ {
					if !strings.HasPrefix(lines[j], "#") && strings.TrimSpace(lines[j]) != "" {
						if strings.TrimSpace(lines[j]) != "true" {
							t.Errorf("Expected 'true' after Draft marker, got %q", lines[j])
						}
						break
					}
				}
				break
			}
		}
		if !foundDraftMarker {
			t.Error("Draft marker not found in template")
		}
	})
}

// Test conditional Linear Ref field
func TestConditionalLinearRefField(t *testing.T) {
	t.Run("linearEnabled false - Ref not shown", func(t *testing.T) {
		gen := NewTemplateGenerator(nil, nil, []string{}, false, false)
		content := gen.Generate()

		if strings.Contains(content, markerRef) {
			t.Error("Template should not contain Ref marker when Linear is disabled")
		}
	})

	t.Run("linearEnabled true - Ref shown", func(t *testing.T) {
		gen := NewTemplateGenerator(nil, nil, []string{}, true, false)
		content := gen.Generate()

		if !strings.Contains(content, markerRef) {
			t.Error("Template missing Ref marker when Linear is enabled")
		}
		// Check for Linear-specific comment
		if !strings.Contains(content, "Linear") {
			t.Error("Template should mention Linear when Ref field is shown")
		}
	})

	t.Run("linearEnabled true - Ref parsed correctly", func(t *testing.T) {
		content := `
# Title:
Test feature

# Draft:
false

# Ref:
ENG-123, ENG-456

# Base Branch: main (read-only)
`
		fields, err := ParseTemplate(content)
		if err != nil {
			t.Fatalf("ParseTemplate failed: %v", err)
		}

		expectedRefs := []string{"ENG-123", "ENG-456"}
		if len(fields.Ref) != len(expectedRefs) {
			t.Errorf("Ref count = %d, want %d", len(fields.Ref), len(expectedRefs))
		}
		for i, expected := range expectedRefs {
			if i < len(fields.Ref) && fields.Ref[i] != expected {
				t.Errorf("Ref[%d] = %q, want %q", i, fields.Ref[i], expected)
			}
		}
	})
}

// Benchmark template generation
func BenchmarkGenerateTemplate(b *testing.B) {
	stackingCtx := &StackingContext{
		IsStacking:    true,
		BaseBranch:    "main",
		CurrentBranch: "feature",
	}
	analysis := &diff.CommitAnalysis{
		Title:   "Test feature",
		Summary: "Add test functionality",
	}
	reviewers := []string{"@user1", "@user2"}

	gen := NewTemplateGenerator(stackingCtx, analysis, reviewers, false, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gen.Generate()
	}
}

// Benchmark template parsing
func BenchmarkParseTemplate(b *testing.B) {
	content := `
# Title:
Feature title

# Summary:
Feature summary

# Test Plan:
Test plan content

# Reviewers:
@user1, @user2

# Ref:
ENG-123, ENG-124
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseTemplate(content)
	}
}
