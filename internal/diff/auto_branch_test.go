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
