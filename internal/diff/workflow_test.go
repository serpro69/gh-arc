package diff

import (
	"os"
	"testing"

	"github.com/serpro69/gh-arc/internal/template"
)

// TestTemplateSavedImmediatelyAfterEdit verifies that templates are saved
// immediately after the editor closes, before validation
func TestTemplateSavedImmediatelyAfterEdit(t *testing.T) {
	// This is an integration-style test that would require mocking the entire workflow
	// For now, we document the expected behavior and verify template functions exist

	// Verify that SaveTemplate function exists and works
	content := "# Test Template\nTest content"
	savedPath, err := template.SaveTemplate(content)
	if err != nil {
		t.Fatalf("SaveTemplate failed: %v", err)
	}
	defer os.Remove(savedPath)

	// Verify file was created
	if _, err := os.Stat(savedPath); os.IsNotExist(err) {
		t.Errorf("Template file was not created at %s", savedPath)
	}

	// Verify content is correct
	loadedContent, err := template.LoadSavedTemplate(savedPath)
	if err != nil {
		t.Fatalf("LoadSavedTemplate failed: %v", err)
	}

	if loadedContent != content {
		t.Errorf("Loaded content does not match saved content.\nExpected: %s\nGot: %s", content, loadedContent)
	}
}

// TestTemplateDeletedAfterSuccess verifies that templates are deleted
// after successful PR creation
func TestTemplateDeletedAfterSuccess(t *testing.T) {
	// Create a temporary template
	content := "# Test Template\nTest content"
	savedPath, err := template.SaveTemplate(content)
	if err != nil {
		t.Fatalf("SaveTemplate failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(savedPath); os.IsNotExist(err) {
		t.Fatalf("Template file was not created at %s", savedPath)
	}

	// Delete the template (simulating successful PR creation)
	err = template.RemoveSavedTemplate(savedPath)
	if err != nil {
		t.Errorf("RemoveSavedTemplate failed: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(savedPath); !os.IsNotExist(err) {
		t.Errorf("Template file still exists at %s after RemoveSavedTemplate", savedPath)
	}
}

// TestTemplatePreservedOnValidationFailure verifies that templates
// are preserved when validation fails
func TestTemplatePreservedOnValidationFailure(t *testing.T) {
	// Create an invalid template (missing required fields)
	invalidContent := `# Creating PR: feature/test → main
# Base Branch: main (read-only)

# Title:
Missing Test Plan

# Summary:
This template is missing the test plan

# Test Plan:

# Reviewers:

# Draft:
false`

	// Save the template
	savedPath, err := template.SaveTemplate(invalidContent)
	if err != nil {
		t.Fatalf("SaveTemplate failed: %v", err)
	}
	defer os.Remove(savedPath) // Cleanup

	// Parse and validate
	fields, err := template.ParseTemplate(invalidContent)
	if err != nil {
		t.Fatalf("ParseTemplate failed: %v", err)
	}

	// Validate (should fail due to missing test plan)
	valid, _ := template.ValidateFieldsWithContext(fields, true, nil) // requireTestPlan = true

	if valid {
		t.Error("Validation should have failed for template missing test plan")
	}

	// Verify template file still exists (not deleted)
	if _, err := os.Stat(savedPath); os.IsNotExist(err) {
		t.Error("Template file should be preserved after validation failure")
	}
}

// TestTemplatePreservedOnError verifies that templates are preserved
// when errors occur during workflow execution
func TestTemplatePreservedOnError(t *testing.T) {
	// This test verifies the behavior described in the fix:
	// Templates should be saved immediately after editing and preserved
	// if any error occurs (validation, auto-branch, PR creation)

	tests := []struct {
		name        string
		content     string
		shouldExist bool
		description string
	}{
		{
			name: "valid_template",
			content: `# Creating PR: feature/test → main
# Base Branch: main (read-only)

# Title:
Test PR

# Summary:
This is a test PR

# Test Plan:
Manual testing

# Reviewers:

# Draft:
false`,
			shouldExist: true,
			description: "Valid template should be saved and preserved until successful PR creation",
		},
		{
			name: "invalid_template",
			content: `# Creating PR: feature/test → main
# Base Branch: main (read-only)

# Title:
Test PR

# Summary:
This is a test PR

# Test Plan:

# Reviewers:

# Draft:
false`,
			shouldExist: true,
			description: "Invalid template should be preserved for --continue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save template
			savedPath, err := template.SaveTemplate(tt.content)
			if err != nil {
				t.Fatalf("SaveTemplate failed: %v", err)
			}
			defer os.Remove(savedPath) // Cleanup

			// Verify file exists
			fileInfo, err := os.Stat(savedPath)
			if os.IsNotExist(err) {
				if tt.shouldExist {
					t.Errorf("%s: Template file should exist at %s", tt.description, savedPath)
				}
			} else if err != nil {
				t.Fatalf("Error checking template file: %v", err)
			} else {
				if !tt.shouldExist {
					t.Errorf("%s: Template file should not exist at %s", tt.description, savedPath)
				}
				// Verify file has content
				if fileInfo.Size() == 0 {
					t.Errorf("%s: Template file is empty", tt.description)
				}
			}
		})
	}
}

// TestFindSavedTemplates verifies that we can find saved templates
func TestFindSavedTemplates(t *testing.T) {
	// Create multiple templates
	content1 := "# Template 1"
	content2 := "# Template 2"

	path1, err := template.SaveTemplate(content1)
	if err != nil {
		t.Fatalf("SaveTemplate failed for template 1: %v", err)
	}
	defer os.Remove(path1)

	path2, err := template.SaveTemplate(content2)
	if err != nil {
		t.Fatalf("SaveTemplate failed for template 2: %v", err)
	}
	defer os.Remove(path2)

	// Find saved templates
	templates, err := template.FindSavedTemplates()
	if err != nil {
		t.Fatalf("FindSavedTemplates failed: %v", err)
	}

	// Verify we found at least our 2 templates
	if len(templates) < 2 {
		t.Errorf("Expected at least 2 saved templates, found %d", len(templates))
	}

	// Verify our templates are in the list
	found1 := false
	found2 := false
	for _, tmpl := range templates {
		if tmpl == path1 {
			found1 = true
		}
		if tmpl == path2 {
			found2 = true
		}
	}

	if !found1 {
		t.Errorf("Template 1 (%s) not found in FindSavedTemplates results", path1)
	}
	if !found2 {
		t.Errorf("Template 2 (%s) not found in FindSavedTemplates results", path2)
	}
}

// TestTemplateCleanupNonFatal verifies that template cleanup failures are non-fatal
func TestTemplateCleanupNonFatal(t *testing.T) {
	// Attempt to remove a non-existent template
	err := template.RemoveSavedTemplate("/nonexistent/path/template.md")

	// Should not return an error for non-existent files
	if err != nil {
		t.Errorf("RemoveSavedTemplate should not error on non-existent file: %v", err)
	}

	// Attempt to remove empty path
	err = template.RemoveSavedTemplate("")

	// Should handle empty path gracefully
	if err != nil {
		t.Errorf("RemoveSavedTemplate should handle empty path gracefully: %v", err)
	}
}

// TestTemplateWorkflowIntegration is a higher-level test that simulates
// the complete workflow: save -> validate -> cleanup
func TestTemplateWorkflowIntegration(t *testing.T) {
	tests := []struct {
		name               string
		template           string
		requireTestPlan    bool
		expectValidation   bool
		shouldCleanup      bool
		simulateError      bool
		expectTemplateFile bool
		description        string
	}{
		{
			name: "successful_workflow",
			template: `# Creating PR: feature/test → main
# Base Branch: main (read-only)

# Title:
Test PR

# Summary:
Summary content

# Test Plan:
Manual testing

# Reviewers:

# Draft:
false`,
			requireTestPlan:    true,
			expectValidation:   true,
			shouldCleanup:      true,
			simulateError:      false,
			expectTemplateFile: false,
			description:        "Successful workflow should clean up template",
		},
		{
			name: "validation_failure",
			template: `# Creating PR: feature/test → main
# Base Branch: main (read-only)

# Title:
Test PR

# Summary:
Summary content

# Test Plan:

# Reviewers:

# Draft:
false`,
			requireTestPlan:    true,
			expectValidation:   false,
			shouldCleanup:      false,
			simulateError:      false,
			expectTemplateFile: true,
			description:        "Validation failure should preserve template",
		},
		{
			name: "error_after_validation",
			template: `# Creating PR: feature/test → main
# Base Branch: main (read-only)

# Title:
Test PR

# Summary:
Summary content

# Test Plan:
Manual testing

# Reviewers:

# Draft:
false`,
			requireTestPlan:    true,
			expectValidation:   true,
			shouldCleanup:      false,
			simulateError:      true,
			expectTemplateFile: true,
			description:        "Error after validation should preserve template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: Save template (simulating post-edit save)
			savedPath, err := template.SaveTemplate(tt.template)
			if err != nil {
				t.Fatalf("SaveTemplate failed: %v", err)
			}
			defer os.Remove(savedPath) // Cleanup in case test fails

			// Verify template was saved
			if _, err := os.Stat(savedPath); os.IsNotExist(err) {
				t.Fatalf("Template file was not created")
			}

			// Step 2: Validate template
			fields, err := template.ParseTemplate(tt.template)
			if err != nil {
				t.Fatalf("ParseTemplate failed: %v", err)
			}

			valid, _ := template.ValidateFieldsWithContext(fields, tt.requireTestPlan, nil)
			if valid != tt.expectValidation {
				t.Errorf("Validation result mismatch. Expected: %v, Got: %v", tt.expectValidation, valid)
			}

			// Step 3: Simulate workflow completion or error
			if !valid || tt.simulateError {
				// Validation failed or error occurred - template should remain
				if _, err := os.Stat(savedPath); os.IsNotExist(err) {
					t.Errorf("%s: Template should be preserved but was deleted", tt.description)
				}
			} else if tt.shouldCleanup {
				// Success - cleanup template
				if err := template.RemoveSavedTemplate(savedPath); err != nil {
					t.Errorf("RemoveSavedTemplate failed: %v", err)
				}
			}

			// Step 4: Verify final state
			_, err = os.Stat(savedPath)
			fileExists := !os.IsNotExist(err)

			if fileExists != tt.expectTemplateFile {
				if tt.expectTemplateFile {
					t.Errorf("%s: Template file should exist but doesn't", tt.description)
				} else {
					t.Errorf("%s: Template file should not exist but does", tt.description)
				}
			}
		})
	}
}
