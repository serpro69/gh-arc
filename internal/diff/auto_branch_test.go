package diff

import (
	"testing"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
)

func TestShouldAutoBranch(t *testing.T) {
	tests := []struct {
		name     string
		result   *DetectionResult
		expected bool
	}{
		{
			name: "on main with commits",
			result: &DetectionResult{
				OnMainBranch:  true,
				CommitsAhead:  2,
				DefaultBranch: "main",
			},
			expected: true,
		},
		{
			name: "on feature branch",
			result: &DetectionResult{
				OnMainBranch:  false,
				CommitsAhead:  2,
				DefaultBranch: "main",
			},
			expected: false,
		},
		{
			name: "on main with no commits",
			result: &DetectionResult{
				OnMainBranch:  true,
				CommitsAhead:  0,
				DefaultBranch: "main",
			},
			expected: false,
		},
		{
			name: "on master with commits",
			result: &DetectionResult{
				OnMainBranch:  true,
				CommitsAhead:  1,
				DefaultBranch: "master",
			},
			expected: true,
		},
		{
			name: "on feature with no commits",
			result: &DetectionResult{
				OnMainBranch:  false,
				CommitsAhead:  0,
				DefaultBranch: "main",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create detector with nil repo and config since ShouldAutoBranch
			// doesn't use them - it only checks the DetectionResult
			detector := &AutoBranchDetector{
				repo:   nil,
				config: &config.DiffConfig{},
			}

			result := detector.ShouldAutoBranch(tt.result)
			if result != tt.expected {
				t.Errorf("ShouldAutoBranch() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestNewAutoBranchDetector(t *testing.T) {
	// Create a mock repository and config
	repo := &git.Repository{}
	cfg := &config.DiffConfig{
		AutoCreateBranchFromMain: true,
	}

	detector := NewAutoBranchDetector(repo, cfg)

	if detector == nil {
		t.Fatal("NewAutoBranchDetector() returned nil")
	}
	if detector.repo != repo {
		t.Error("NewAutoBranchDetector() did not set repo correctly")
	}
	if detector.config != cfg {
		t.Error("NewAutoBranchDetector() did not set config correctly")
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "spaces to hyphens",
			input:    "John Doe",
			expected: "john-doe",
		},
		{
			name:     "double dots to dash",
			input:    "test..name",
			expected: "test-name",
		},
		{
			name:     "colon removed",
			input:    "feature:test",
			expected: "featuretest",
		},
		{
			name:     "multiple invalid chars",
			input:    "my*branch?",
			expected: "mybranch",
		},
		{
			name:     "tilde and caret",
			input:    "test~branch^name",
			expected: "testbranchname",
		},
		{
			name:     "square bracket and backslash",
			input:    "test[name]\\branch",
			expected: "testnamebranch",
		},
		{
			name:     "already valid",
			input:    "feature-branch",
			expected: "feature-branch",
		},
		{
			name:     "mixed case to lowercase",
			input:    "Feature-Branch",
			expected: "feature-branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeBranchName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeBranchName(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateBranchName(t *testing.T) {
	tests := []struct {
		name              string
		pattern           string
		expectedSubstring string // Substring to check for in result
		shouldPrompt      bool
		shouldError       bool
	}{
		{
			name:              "empty pattern uses default",
			pattern:           "",
			expectedSubstring: "feature/auto-from-main-",
			shouldPrompt:      false,
			shouldError:       false,
		},
		{
			name:              "null pattern triggers prompt",
			pattern:           "null",
			expectedSubstring: "",
			shouldPrompt:      true,
			shouldError:       false,
		},
		{
			name:              "timestamp placeholder",
			pattern:           "feature/{timestamp}",
			expectedSubstring: "feature/",
			shouldPrompt:      false,
			shouldError:       false,
		},
		{
			name:              "date placeholder",
			pattern:           "feature/{date}",
			expectedSubstring: "feature/",
			shouldPrompt:      false,
			shouldError:       false,
		},
		{
			name:              "datetime placeholder",
			pattern:           "feature/{datetime}",
			expectedSubstring: "feature/",
			shouldPrompt:      false,
			shouldError:       false,
		},
		{
			name:              "random placeholder",
			pattern:           "feature/{random}",
			expectedSubstring: "feature/",
			shouldPrompt:      false,
			shouldError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := &AutoBranchDetector{
				repo: nil, // Will be set when needed
				config: &config.DiffConfig{
					AutoBranchNamePattern: tt.pattern,
				},
			}

			result, shouldPrompt, err := detector.GenerateBranchName()

			if tt.shouldError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if shouldPrompt != tt.shouldPrompt {
				t.Errorf("shouldPrompt = %v, expected %v", shouldPrompt, tt.shouldPrompt)
			}
			if tt.expectedSubstring != "" && result != "" {
				if len(result) < len(tt.expectedSubstring) {
					t.Errorf("result %q too short, expected to contain %q", result, tt.expectedSubstring)
				}
				if result[:len(tt.expectedSubstring)] != tt.expectedSubstring {
					t.Errorf("result %q does not start with %q", result, tt.expectedSubstring)
				}
			}
		})
	}
}

func TestEnsureUniqueBranchName(t *testing.T) {
	tests := []struct {
		name             string
		baseName         string
		existingBranches []string
		expected         string
		shouldError      bool
	}{
		{
			name:             "unique branch name",
			baseName:         "feature/test",
			existingBranches: []string{"main", "develop"},
			expected:         "feature/test",
			shouldError:      false,
		},
		{
			name:             "append -1 for first collision",
			baseName:         "feature/test",
			existingBranches: []string{"feature/test"},
			expected:         "feature/test-1",
			shouldError:      false,
		},
		{
			name:             "append -2 for second collision",
			baseName:         "feature/test",
			existingBranches: []string{"feature/test", "feature/test-1"},
			expected:         "feature/test-2",
			shouldError:      false,
		},
		{
			name:             "append -3 for third collision",
			baseName:         "feature/test",
			existingBranches: []string{"feature/test", "feature/test-1", "feature/test-2"},
			expected:         "feature/test-3",
			shouldError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock repository that tracks existing branches
			// For now, we'll test the logic when we implement it
			// This test will be updated with proper mocking
			t.Skip("Requires repository mocking - will implement after method is added")
		})
	}
}

func TestDisplayCommitList(t *testing.T) {
	tests := []struct {
		name    string
		commits []git.CommitInfo
		// Output verification would require capturing stdout or mock
		// We'll test that it doesn't panic and handles various inputs
	}{
		{
			name: "single commit",
			commits: []git.CommitInfo{
				{SHA: "abc123def456", Message: "Add new feature"},
			},
		},
		{
			name: "multiple commits",
			commits: []git.CommitInfo{
				{SHA: "abc123def456", Message: "Add new feature"},
				{SHA: "def456ghi789", Message: "Fix bug in handler"},
			},
		},
		{
			name: "commit with long message",
			commits: []git.CommitInfo{
				{SHA: "abc123def456", Message: "This is a very long commit message that should be truncated because it exceeds the 80 character limit"},
			},
		},
		{
			name:    "empty commit list",
			commits: []git.CommitInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that displayCommitList doesn't panic
			// Actual output verification would require stdout capture
			// which is complex and better tested manually
			displayCommitList(tt.commits)
		})
	}
}

func TestPromptFunctions(t *testing.T) {
	// promptYesNo and promptBranchName require stdin interaction
	// These are better tested manually
	// Mark as skip with explanation
	t.Skip("Prompt functions require stdin interaction - tested manually")
}
