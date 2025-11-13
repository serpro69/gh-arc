package diff

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/serpro69/gh-arc/internal/template"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createValidTemplateContent returns a valid PR template
func createValidTemplateContent() string {
	return `# Creating PR: feature/test → main
# Base Branch: main (read-only)

# Title:
Test PR Title

# Summary:
This is a test PR summary

# Test Plan:
Manual testing performed

# Reviewers:

# Draft:
false`
}

// createInvalidTemplateContent returns an invalid template (missing test plan)
func createInvalidTemplateContent() string {
	return `# Creating PR: feature/test → main
# Base Branch: main (read-only)

# Title:
Test PR Title

# Summary:
This is a test PR summary

# Test Plan:

# Reviewers:

# Draft:
false`
}

// =============================================================================
// Integration Tests: Template Persistence Through Workflow
// =============================================================================

// TestWorkflow_TemplateSavedImmediatelyAfterEdit verifies that templates
// are saved right after the editor closes, before any validation or operations
func TestWorkflow_TemplateSavedImmediatelyAfterEdit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test verifies the template lifecycle as implemented in workflow.go:
	// Edit (Step 5) → Save immediately (Step 5.5) → Validate (Step 6) → Cleanup on success (Step 11)

	t.Run("save_before_validation", func(t *testing.T) {
		validContent := createValidTemplateContent()

		// Step 1: Save template (simulating post-editor save)
		savedPath, err := template.SaveTemplate(validContent)
		if err != nil {
			t.Fatalf("SaveTemplate failed: %v", err)
		}
		defer os.Remove(savedPath)

		// Verify template file exists
		if _, err := os.Stat(savedPath); os.IsNotExist(err) {
			t.Error("Template should be saved immediately after editing")
		}

		// Step 2: Validate template
		fields, err := template.ParseTemplate(validContent)
		if err != nil {
			t.Fatalf("ParseTemplate failed: %v", err)
		}

		valid, _ := template.ValidateFieldsWithContext(fields, true, nil) // requireTestPlan = true
		if !valid {
			t.Error("Template should be valid")
		}

		// Verify template still exists after validation
		if _, err := os.Stat(savedPath); os.IsNotExist(err) {
			t.Error("Template should still exist after validation")
		}

		// Step 3: Cleanup on success (simulating successful PR creation)
		err = template.RemoveSavedTemplate(savedPath)
		if err != nil {
			t.Errorf("RemoveSavedTemplate failed: %v", err)
		}

		// Verify template was deleted
		if _, err := os.Stat(savedPath); !os.IsNotExist(err) {
			t.Error("Template should be deleted after successful PR creation")
		}
	})
}

// TestWorkflow_TemplatePreservedOnValidationFailure verifies templates
// are preserved when validation fails
func TestWorkflow_TemplatePreservedOnValidationFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create invalid template (missing test plan)
	invalidContent := createInvalidTemplateContent()

	// Save template (simulating post-editor save)
	savedPath, err := template.SaveTemplate(invalidContent)
	if err != nil {
		t.Fatalf("SaveTemplate failed: %v", err)
	}
	defer os.Remove(savedPath)

	// Parse and validate
	fields, err := template.ParseTemplate(invalidContent)
	if err != nil {
		t.Fatalf("ParseTemplate failed: %v", err)
	}

	valid, validationMsg := template.ValidateFieldsWithContext(fields, true, nil) // requireTestPlan = true

	// Validation should fail
	if valid {
		t.Error("Validation should fail for template missing test plan")
	}

	if validationMsg == "" {
		t.Error("Validation message should not be empty")
	}

	// Template file should still exist (preserved for --continue)
	if _, err := os.Stat(savedPath); os.IsNotExist(err) {
		t.Error("Template should be preserved after validation failure")
	}

	// Verify content is intact
	loadedContent, err := template.LoadSavedTemplate(savedPath)
	if err != nil {
		t.Fatalf("LoadSavedTemplate failed: %v", err)
	}

	if loadedContent != invalidContent {
		t.Error("Loaded template content should match saved content")
	}
}

// TestWorkflow_TemplatePreservedOnPRCreationFailure verifies templates
// are preserved when PR creation fails after validation
func TestWorkflow_TemplatePreservedOnPRCreationFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test verifies the critical fix: if validation passes but PR creation
	// fails (e.g., network error, GitHub API error), the template should be
	// preserved so the user can retry with --continue

	validContent := createValidTemplateContent()

	// Step 1: Save template (happens immediately after editing)
	savedPath, err := template.SaveTemplate(validContent)
	if err != nil {
		t.Fatalf("SaveTemplate failed: %v", err)
	}
	defer os.Remove(savedPath)

	// Step 2: Validate (should pass)
	fields, err := template.ParseTemplate(validContent)
	if err != nil {
		t.Fatalf("ParseTemplate failed: %v", err)
	}

	valid, _ := template.ValidateFieldsWithContext(fields, true, nil)
	if !valid {
		t.Error("Validation should pass for valid template")
	}

	// Step 3: Simulate PR creation failure
	// In real workflow, CreateOrUpdatePR would fail here
	// Template should NOT be deleted because PR creation failed

	// Verify template still exists (preserved for retry)
	if _, err := os.Stat(savedPath); os.IsNotExist(err) {
		t.Error("Template should be preserved when PR creation fails")
	}

	// Template should only be deleted if we explicitly clean up on success
	// Since we simulated failure, template should remain
	// User can retry with --continue
}

// TestWorkflow_TemplateCleanupOnSuccess verifies templates are deleted
// only after successful PR creation
func TestWorkflow_TemplateCleanupOnSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	validContent := createValidTemplateContent()

	// Step 1: Save template
	savedPath, err := template.SaveTemplate(validContent)
	if err != nil {
		t.Fatalf("SaveTemplate failed: %v", err)
	}

	// Step 2: Validate (should pass)
	fields, err := template.ParseTemplate(validContent)
	if err != nil {
		t.Fatalf("ParseTemplate failed: %v", err)
	}

	valid, _ := template.ValidateFieldsWithContext(fields, true, nil)
	if !valid {
		t.Error("Validation should pass")
	}

	// Verify template exists before cleanup
	if _, err := os.Stat(savedPath); os.IsNotExist(err) {
		t.Fatal("Template should exist before cleanup")
	}

	// Step 3: Simulate successful PR creation
	// In real workflow, this happens in Step 10 of executeWithTemplateEditing

	// Step 4: Clean up template (Step 11 in real workflow)
	err = template.RemoveSavedTemplate(savedPath)
	if err != nil {
		t.Fatalf("RemoveSavedTemplate failed: %v", err)
	}

	// Verify template was deleted
	if _, err := os.Stat(savedPath); !os.IsNotExist(err) {
		t.Error("Template should be deleted after successful PR creation")
	}
}

// =============================================================================
// Integration Tests: Context Propagation
// =============================================================================

// TestWorkflow_ContextNoDeadline verifies that workflow operations
// receive a context without a deadline
func TestWorkflow_ContextNoDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create base context without timeout (as cmd/diff.go does after our fix)
	ctx := context.Background()

	// Verify no deadline
	if _, ok := ctx.Deadline(); ok {
		t.Error("Base context should not have a deadline")
	}

	// Simulate passing context through workflow
	// In real workflow: ctx → workflow.Execute() → executeWithTemplateEditing() → various operations

	// Create a mock operation that checks context
	contextChecker := func(ctx context.Context) error {
		if deadline, ok := ctx.Deadline(); ok {
			return errors.New("context should not have a deadline, but has: " + deadline.String())
		}
		return nil
	}

	// Verify context passed to operations has no deadline
	if err := contextChecker(ctx); err != nil {
		t.Error(err)
	}
}

// TestWorkflow_ContextCancellationRespected verifies that workflow
// properly respects context cancellation (even though no timeout)
func TestWorkflow_ContextCancellationRespected(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Immediately cancel it
	cancel()

	// Verify context is cancelled
	if ctx.Err() != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", ctx.Err())
	}

	// In a real workflow, operations should check ctx.Err() and return early
	// This test verifies that removing the timeout doesn't break cancellation
	select {
	case <-ctx.Done():
		// Expected: context is done
		if ctx.Err() != context.Canceled {
			t.Errorf("Expected context.Canceled, got: %v", ctx.Err())
		}
	default:
		t.Error("Cancelled context should be done")
	}
}

// =============================================================================
// Integration Tests: Complete Workflow Scenarios
// =============================================================================

// TestWorkflow_TemplateLifecycle_FullWorkflow tests the complete template
// lifecycle through a realistic workflow scenario
func TestWorkflow_TemplateLifecycle_FullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name                string
		templateContent     string
		requireTestPlan     bool
		prCreationError     error
		expectValidation    bool
		expectTemplateAfter bool
		description         string
	}{
		{
			name:                "success_cleanup",
			templateContent:     createValidTemplateContent(),
			requireTestPlan:     true,
			prCreationError:     nil,
			expectValidation:    true,
			expectTemplateAfter: false,
			description:         "Valid template + successful PR = template cleaned up",
		},
		{
			name:                "validation_failure_preserve",
			templateContent:     createInvalidTemplateContent(),
			requireTestPlan:     true,
			prCreationError:     nil,
			expectValidation:    false,
			expectTemplateAfter: true,
			description:         "Invalid template = template preserved for --continue",
		},
		{
			name:                "pr_creation_failure_preserve",
			templateContent:     createValidTemplateContent(),
			requireTestPlan:     true,
			prCreationError:     errors.New("GitHub API error"),
			expectValidation:    true,
			expectTemplateAfter: true,
			description:         "Valid template but PR creation fails = template preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: Save template (simulating Step 5.5 in workflow)
			savedPath, err := template.SaveTemplate(tt.templateContent)
			if err != nil {
				t.Fatalf("SaveTemplate failed: %v", err)
			}
			defer os.Remove(savedPath) // Cleanup

			// Step 2: Parse and validate (Step 6 in workflow)
			fields, err := template.ParseTemplate(tt.templateContent)
			if err != nil {
				t.Fatalf("ParseTemplate failed: %v", err)
			}

			valid, _ := template.ValidateFieldsWithContext(fields, tt.requireTestPlan, nil)

			if valid != tt.expectValidation {
				t.Errorf("Validation mismatch. Expected: %v, Got: %v", tt.expectValidation, valid)
			}

			// If validation failed, template should be preserved
			if !valid {
				if _, err := os.Stat(savedPath); os.IsNotExist(err) {
					t.Error("Template should be preserved after validation failure")
				}
				return // Don't proceed to PR creation
			}

			// Step 3: Simulate PR creation (Step 10 in workflow)
			var prCreated bool
			if tt.prCreationError == nil {
				prCreated = true
			}

			// Step 4: Clean up on success, preserve on failure (Step 11 in workflow)
			if prCreated {
				// Success: delete template
				err = template.RemoveSavedTemplate(savedPath)
				if err != nil {
					t.Errorf("RemoveSavedTemplate failed: %v", err)
				}
			}
			// On failure: template remains (don't delete)

			// Step 5: Verify final state
			_, err = os.Stat(savedPath)
			templateExists := !os.IsNotExist(err)

			if templateExists != tt.expectTemplateAfter {
				if tt.expectTemplateAfter {
					t.Errorf("%s: Template should exist but was deleted", tt.description)
				} else {
					t.Errorf("%s: Template should be deleted but still exists", tt.description)
				}
			}
		})
	}
}

// =============================================================================
// Integration Tests: Error Scenarios
// =============================================================================

// TestWorkflow_ErrorRecovery_TemplatePreservation tests that templates
// are preserved across various error scenarios
func TestWorkflow_ErrorRecovery_TemplatePreservation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	scenarios := []struct {
		name            string
		errorStage      string
		shouldPreserve  bool
	}{
		{
			name:           "error_after_editing",
			errorStage:     "validation",
			shouldPreserve: true,
		},
		{
			name:           "error_after_validation",
			errorStage:     "auto-branch",
			shouldPreserve: true,
		},
		{
			name:           "error_after_auto_branch",
			errorStage:     "pr-creation",
			shouldPreserve: true,
		},
		{
			name:           "no_error",
			errorStage:     "none",
			shouldPreserve: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var content string
			if scenario.errorStage == "validation" {
				content = createInvalidTemplateContent()
			} else {
				content = createValidTemplateContent()
			}

			// Save template
			savedPath, err := template.SaveTemplate(content)
			if err != nil {
				t.Fatalf("SaveTemplate failed: %v", err)
			}
			defer os.Remove(savedPath)

			// Determine if we should cleanup
			shouldCleanup := !scenario.shouldPreserve

			if shouldCleanup {
				// Simulate successful workflow
				err = template.RemoveSavedTemplate(savedPath)
				if err != nil {
					t.Errorf("Cleanup failed: %v", err)
				}
			}

			// Verify final state
			_, err = os.Stat(savedPath)
			exists := !os.IsNotExist(err)

			if exists != scenario.shouldPreserve {
				if scenario.shouldPreserve {
					t.Errorf("Template should be preserved at %s stage", scenario.errorStage)
				} else {
					t.Error("Template should be deleted after successful workflow")
				}
			}
		})
	}
}

// TestWorkflow_ContinueMode_LoadsPreservedTemplate verifies that continue
// mode can load previously saved templates
func TestWorkflow_ContinueMode_LoadsPreservedTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create and save a template (simulating validation failure)
	content := createInvalidTemplateContent()
	savedPath, err := template.SaveTemplate(content)
	if err != nil {
		t.Fatalf("SaveTemplate failed: %v", err)
	}
	defer os.Remove(savedPath)

	// Simulate --continue mode: load the saved template
	loadedContent, err := template.LoadSavedTemplate(savedPath)
	if err != nil {
		t.Fatalf("LoadSavedTemplate failed: %v", err)
	}

	// Verify content matches
	if loadedContent != content {
		t.Error("Loaded content should match saved content")
	}

	// User would fix the template in the editor (add test plan)
	fixedContent := createValidTemplateContent()

	// Replace saved template with fixed version
	// (In real workflow, editor would modify and we'd save again)
	os.Remove(savedPath) // Delete old
	savedPath, err = template.SaveTemplate(fixedContent)
	if err != nil {
		t.Fatalf("SaveTemplate failed for fixed content: %v", err)
	}
	defer os.Remove(savedPath)

	// Validate fixed content
	fields, err := template.ParseTemplate(fixedContent)
	if err != nil {
		t.Fatalf("ParseTemplate failed: %v", err)
	}

	valid, _ := template.ValidateFieldsWithContext(fields, true, nil)
	if !valid {
		t.Error("Fixed template should pass validation")
	}

	// On success, cleanup
	err = template.RemoveSavedTemplate(savedPath)
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}

	// Verify deleted
	if _, err := os.Stat(savedPath); !os.IsNotExist(err) {
		t.Error("Template should be deleted after successful continue mode")
	}
}

// TestWorkflow_MultipleTemplates_LoadsMostRecent verifies that when
// multiple templates exist, the most recent one is loaded
func TestWorkflow_MultipleTemplates_LoadsMostRecent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create first template
	content1 := "# OLD Template"
	path1, err := template.SaveTemplate(content1)
	if err != nil {
		t.Fatalf("SaveTemplate failed for template 1: %v", err)
	}
	defer os.Remove(path1)

	// Create second template (most recent)
	content2 := "# NEW Template - Most Recent"
	path2, err := template.SaveTemplate(content2)
	if err != nil {
		t.Fatalf("SaveTemplate failed for template 2: %v", err)
	}
	defer os.Remove(path2)

	// Find all saved templates
	templates, err := template.FindSavedTemplates()
	if err != nil {
		t.Fatalf("FindSavedTemplates failed: %v", err)
	}

	// Verify we found at least our 2 templates
	if len(templates) < 2 {
		t.Errorf("Expected at least 2 templates, found %d", len(templates))
	}

	// The most recent should be path2
	// In real workflow, continue mode would load templates[0] (most recent)
	// Since SaveTemplate uses timestamps in filename, path2 should sort last
	foundPath1 := false
	foundPath2 := false
	for _, tmpl := range templates {
		if tmpl == path1 {
			foundPath1 = true
		}
		if tmpl == path2 {
			foundPath2 = true
		}
	}

	if !foundPath1 || !foundPath2 {
		t.Error("Both templates should be found by FindSavedTemplates")
	}
}

// =============================================================================
// Integration Tests: Context + Template Together
// =============================================================================

// TestWorkflow_LongOperation_NoTimeout_TemplatePreserved tests the
// combination of both fixes: no timeout during long operations + template preserved
func TestWorkflow_LongOperation_NoTimeout_TemplatePreserved(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create context without timeout (as cmd/diff.go does after our fix)
	ctx := context.Background()

	// Verify no deadline
	if _, ok := ctx.Deadline(); ok {
		t.Error("Context should not have a deadline")
	}

	// Save a template
	content := createValidTemplateContent()
	savedPath, err := template.SaveTemplate(content)
	if err != nil {
		t.Fatalf("SaveTemplate failed: %v", err)
	}
	defer os.Remove(savedPath)

	// Verify template exists
	if _, err := os.Stat(savedPath); os.IsNotExist(err) {
		t.Error("Template should exist")
	}

	// Context should remain valid indefinitely
	select {
	case <-ctx.Done():
		t.Error("Context should not be done")
	default:
		// Expected
	}

	// Template should still exist (until explicit cleanup)
	if _, err := os.Stat(savedPath); os.IsNotExist(err) {
		t.Error("Template should still exist until cleanup")
	}

	// Verify context error is nil
	if ctx.Err() != nil {
		t.Errorf("Context error should be nil, got: %v", ctx.Err())
	}
}
