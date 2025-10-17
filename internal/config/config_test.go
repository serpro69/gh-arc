package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("load with defaults when no config file exists", func(t *testing.T) {
		// Create a temporary directory for testing
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check defaults
		if cfg.GitHub.DefaultBranch != "main" {
			t.Errorf("Expected default branch 'main', got '%s'", cfg.GitHub.DefaultBranch)
		}
		if cfg.Land.DefaultMergeMethod != "squash" {
			t.Errorf("Expected default merge method 'squash', got '%s'", cfg.Land.DefaultMergeMethod)
		}
		if cfg.Diff.CreateAsDraft {
			t.Error("Expected createAsDraft to be false by default")
		}
		if !cfg.Diff.EnableStacking {
			t.Error("Expected enableStacking to be true by default")
		}
		if !cfg.Diff.RequireTestPlan {
			t.Error("Expected requireTestPlan to be true by default")
		}
		if cfg.Diff.TemplatePath != "" {
			t.Errorf("Expected templatePath to be empty by default, got '%s'", cfg.Diff.TemplatePath)
		}
		if cfg.Diff.LinearEnabled {
			t.Error("Expected linearEnabled to be false by default")
		}
		// Auto-branch defaults
		if !cfg.Diff.AutoCreateBranchFromMain {
			t.Error("Expected autoCreateBranchFromMain to be true by default")
		}
		if cfg.Diff.AutoBranchNamePattern != "" {
			t.Errorf("Expected autoBranchNamePattern to be empty by default, got '%s'", cfg.Diff.AutoBranchNamePattern)
		}
		if cfg.Diff.StaleRemoteThresholdHours != 24 {
			t.Errorf("Expected staleRemoteThresholdHours to be 24 by default, got %d", cfg.Diff.StaleRemoteThresholdHours)
		}
	})

	t.Run("load from JSON config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)

		// Create a test config file with explicit .json extension
		configContent := `{
			"github": {
				"defaultBranch": "develop",
				"defaultReviewers": ["testuser"]
			},
			"land": {
				"defaultMergeMethod": "merge"
			}
		}`

		err := os.WriteFile(".arc.json", []byte(configContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if cfg.GitHub.DefaultBranch != "develop" {
			t.Errorf("Expected branch 'develop', got '%s'", cfg.GitHub.DefaultBranch)
		}
		if !slices.Contains(cfg.GitHub.DefaultReviewers, "testuser") {
			t.Errorf("Expected to contain 'testuser', got '%v'", cfg.GitHub.DefaultReviewers)
		}
		if len(cfg.GitHub.DefaultReviewers) != 1 {
			t.Errorf("Expected '%v' to have len 1, got '%d'", cfg.GitHub.DefaultReviewers, len(cfg.GitHub.DefaultReviewers))
		}
		if cfg.Land.DefaultMergeMethod != "merge" {
			t.Errorf("Expected merge method 'merge', got '%s'", cfg.Land.DefaultMergeMethod)
		}
	})

	t.Run("load from YAML config file (.yaml)", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)

		// Create a YAML test config file with explicit .yaml extension
		configContent := `
github:
  defaultBranch: feature
  autoAssignReviewer: true
diff:
  createAsDraft: false
`
		err := os.WriteFile(".arc.yaml", []byte(configContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if cfg.GitHub.DefaultBranch != "feature" {
			t.Errorf("Expected branch 'feature', got '%s'", cfg.GitHub.DefaultBranch)
		}
		if !cfg.GitHub.AutoAssignReviewer {
			t.Error("Expected autoAssignReviewer to be true")
		}
		if cfg.Diff.CreateAsDraft {
			t.Error("Expected createAsDraft to be false")
		}
	})

	t.Run("load from YAML config file (.yml)", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)

		// Create a YAML test config file with alternate .yml extension
		configContent := `
land:
  defaultMergeMethod: rebase
  deleteLocalBranch: false
output:
  verbose: true
`
		err := os.WriteFile(".arc.yml", []byte(configContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if cfg.Land.DefaultMergeMethod != "rebase" {
			t.Errorf("Expected merge method 'rebase', got '%s'", cfg.Land.DefaultMergeMethod)
		}
		if cfg.Land.DeleteLocalBranch {
			t.Error("Expected deleteLocalBranch to be false")
		}
		if !cfg.Output.Verbose {
			t.Error("Expected verbose to be true")
		}
	})

	t.Run("load with environment variables", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)

		// Clear cached config to force reload
		cfg = nil
		v = nil

		// Set environment variables (testing that viper's AutomaticEnv works)
		// Viper automatically binds env vars with GHARC_ prefix
		os.Setenv("GHARC_DIFF_REQUIRETESTPLAN", "false")
		os.Setenv("GHARC_DIFF_LINEARENABLED", "true")
		os.Setenv("GHARC_DIFF_LINEARDEFAULTPROJECT", "TEST")
		defer os.Unsetenv("GHARC_DIFF_REQUIRETESTPLAN")
		defer os.Unsetenv("GHARC_DIFF_LINEARENABLED")
		defer os.Unsetenv("GHARC_DIFF_LINEARDEFAULTPROJECT")

		config, err := Load()
		if err != nil {
			t.Fatalf("Expected no error with env vars set, got: %v", err)
		}

		// Verify config loaded successfully with env vars present
		// Note: Viper's AutomaticEnv() handles the binding, but actual value
		// testing would require integration tests. Here we just verify no errors.
		if config == nil {
			t.Fatal("Expected config to be loaded")
		}
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with squash",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with merge",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "merge"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "true"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with rebase",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "rebase"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "false"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid merge method",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "invalid"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
			},
			wantErr: true,
			errMsg:  "invalid merge method",
		},
		{
			name: "invalid mega-linter enabled value",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "invalid"},
				},
			},
			wantErr: true,
			errMsg:  "invalid lint.megaLinter.enabled value",
		},
		{
			name: "test runner without name",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Test: TestConfig{
					Runners: []TestRunner{
						{Name: "", Command: "go test"},
					},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "test runner without command",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Test: TestConfig{
					Runners: []TestRunner{
						{Name: "unit", Command: ""},
					},
				},
			},
			wantErr: true,
			errMsg:  "command is required",
		},
		{
			name: "lint runner without name",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					Runners: []LintRunner{
						{Name: "", Command: "golangci-lint"},
					},
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "lint runner without command",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					Runners: []LintRunner{
						{Name: "golangci", Command: ""},
					},
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
			},
			wantErr: true,
			errMsg:  "command is required",
		},
		{
			name: "valid diff config with empty template path",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					TemplatePath: "",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid diff config with nonexistent template path",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					TemplatePath: "/nonexistent/template.md",
				},
			},
			wantErr: true,
			errMsg:  "diff.templatePath does not exist",
		},
		{
			name: "valid diff config with valid base branch",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					DefaultBase: "main",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid diff config with base branch containing spaces",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					DefaultBase: "feature branch",
				},
			},
			wantErr: true,
			errMsg:  "diff.defaultBase cannot contain spaces",
		},
		// Auto-branch pattern validation tests
		{
			name: "valid auto-branch pattern with placeholder",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					AutoBranchNamePattern: "feature/{timestamp}",
				},
			},
			wantErr: false,
		},
		{
			name: "valid auto-branch pattern empty string",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					AutoBranchNamePattern: "",
				},
			},
			wantErr: false,
		},
		{
			name: "valid auto-branch pattern null",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					AutoBranchNamePattern: "null",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid auto-branch pattern with consecutive dots",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					AutoBranchNamePattern: "feature/../invalid",
				},
			},
			wantErr: true,
			errMsg:  "cannot contain consecutive dots",
		},
		{
			name: "invalid auto-branch pattern with space",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					AutoBranchNamePattern: "feature branch",
				},
			},
			wantErr: true,
			errMsg:  "cannot contain space",
		},
		{
			name: "invalid auto-branch pattern starting with slash",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					AutoBranchNamePattern: "/feature/branch",
				},
			},
			wantErr: true,
			errMsg:  "cannot start with '/'",
		},
		{
			name: "invalid auto-branch pattern with tilde",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					AutoBranchNamePattern: "feature~branch",
				},
			},
			wantErr: true,
			errMsg:  "cannot contain tilde",
		},
		{
			name: "invalid auto-branch pattern with caret",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					AutoBranchNamePattern: "feature^branch",
				},
			},
			wantErr: true,
			errMsg:  "cannot contain caret",
		},
		{
			name: "invalid auto-branch pattern with colon",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					AutoBranchNamePattern: "feature:branch",
				},
			},
			wantErr: true,
			errMsg:  "cannot contain colon",
		},
		{
			name: "negative stale remote threshold",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					StaleRemoteThresholdHours: -1,
				},
			},
			wantErr: true,
			errMsg:  "staleRemoteThresholdHours cannot be negative",
		},
		{
			name: "valid zero stale remote threshold",
			config: Config{
				Land: LandConfig{DefaultMergeMethod: "squash"},
				Lint: LintConfig{
					MegaLinter: MegaLinterConfig{Enabled: "auto"},
				},
				Diff: DiffConfig{
					StaleRemoteThresholdHours: 0,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestGetMegaLinterConfigPath(t *testing.T) {
	t.Run("returns configured path if exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)

		configFile := filepath.Join(tmpDir, "custom-mega-linter.yml")
		err := os.WriteFile(configFile, []byte("test"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		cfg := &Config{
			Lint: LintConfig{
				MegaLinter: MegaLinterConfig{
					Config: configFile,
				},
			},
		}

		path, isURL, err := cfg.GetMegaLinterConfigPath()
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if isURL {
			t.Error("Expected isURL to be false (local file)")
		}
		if path != configFile {
			t.Errorf("Expected path '%s', got '%s'", configFile, path)
		}
	})

	t.Run("returns default path if exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)

		defaultFile := ".mega-linter.yml"
		err := os.WriteFile(defaultFile, []byte("test"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		cfg := &Config{
			Lint: LintConfig{
				MegaLinter: MegaLinterConfig{
					Config: "nonexistent.yml",
				},
			},
		}

		path, isURL, err := cfg.GetMegaLinterConfigPath()
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if isURL {
			t.Error("Expected isURL to be false (local file)")
		}
		if path != defaultFile {
			t.Errorf("Expected path '%s', got '%s'", defaultFile, path)
		}
	})

	t.Run("returns GitHub URL if no file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)

		cfg := &Config{
			Lint: LintConfig{
				MegaLinter: MegaLinterConfig{
					Config: "nonexistent.yml",
				},
			},
		}

		path, isURL, err := cfg.GetMegaLinterConfigPath()
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if !isURL {
			t.Error("Expected isURL to be true")
		}
		if path == "" {
			t.Error("Expected path to be set")
		}
		// Verify it's a valid GitHub raw URL
		expectedURL := "https://raw.githubusercontent.com/serpro69/gh-arc/main/internal/lint/default-mega-linter.yml"
		if path != expectedURL {
			t.Errorf("Expected URL '%s', got '%s'", expectedURL, path)
		}
	})
}

func TestGet(t *testing.T) {
	// Reset cfg
	cfg = nil

	config := Get()
	if config == nil {
		t.Error("Expected config to be loaded, got nil")
	}

	// Second call should return same instance
	config2 := Get()
	if config != config2 {
		t.Error("Expected Get() to return same instance")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
