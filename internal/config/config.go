package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	GitHub GitHubConfig `mapstructure:"github"`
	Diff   DiffConfig   `mapstructure:"diff"`
	Land   LandConfig   `mapstructure:"land"`
	Test   TestConfig   `mapstructure:"test"`
	Lint   LintConfig   `mapstructure:"lint"`
	Output OutputConfig `mapstructure:"output"`
}

// GitHubConfig contains GitHub-related settings
type GitHubConfig struct {
	DefaultBranch      string `mapstructure:"defaultBranch"`
	DefaultReviewer    string `mapstructure:"defaultReviewer"`
	AutoAssignReviewer bool   `mapstructure:"autoAssignReviewer"`
}

// DiffConfig contains PR creation settings
type DiffConfig struct {
	CreateAsDraft          bool `mapstructure:"createAsDraft"`
	AutoUpdatePR           bool `mapstructure:"autoUpdatePR"`
	IncludeCommitMessages  bool `mapstructure:"includeCommitMessages"`
}

// LandConfig contains merge settings
type LandConfig struct {
	DefaultMergeMethod string `mapstructure:"defaultMergeMethod"`
	DeleteLocalBranch  bool   `mapstructure:"deleteLocalBranch"`
	DeleteRemoteBranch bool   `mapstructure:"deleteRemoteBranch"`
	RequireApproval    bool   `mapstructure:"requireApproval"`
	RequireCI          bool   `mapstructure:"requireCI"`
}

// TestConfig contains test execution settings
type TestConfig struct {
	Runners []TestRunner `mapstructure:"runners"`
}

// TestRunner represents a test runner configuration
type TestRunner struct {
	Name       string   `mapstructure:"name"`
	Command    string   `mapstructure:"command"`
	Args       []string `mapstructure:"args"`
	WorkingDir string   `mapstructure:"workingDir"`
	Timeout    string   `mapstructure:"timeout"`
}

// LintConfig contains linting settings
type LintConfig struct {
	Runners    []LintRunner    `mapstructure:"runners"`
	MegaLinter MegaLinterConfig `mapstructure:"megaLinter"`
}

// LintRunner represents a linter runner configuration
type LintRunner struct {
	Name       string   `mapstructure:"name"`
	Command    string   `mapstructure:"command"`
	Args       []string `mapstructure:"args"`
	WorkingDir string   `mapstructure:"workingDir"`
	AutoFix    bool     `mapstructure:"autoFix"`
}

// MegaLinterConfig contains mega-linter specific settings
type MegaLinterConfig struct {
	Enabled   string `mapstructure:"enabled"` // auto, true, false
	Config    string `mapstructure:"config"`
	FixIssues bool   `mapstructure:"fixIssues"`
}

// OutputConfig contains output preferences
type OutputConfig struct {
	Verbose bool `mapstructure:"verbose"`
	Quiet   bool `mapstructure:"quiet"`
	JSON    bool `mapstructure:"json"`
	Color   bool `mapstructure:"color"`
}

var (
	// cfg holds the loaded configuration
	cfg *Config
	// v is the viper instance
	v *viper.Viper
)

// Load loads the configuration from files and environment variables
func Load() (*Config, error) {
	v = viper.New()

	// Set defaults
	setDefaults(v)

	// Set config name and type
	v.SetConfigName(".arc")
	v.SetConfigType("json")

	// Add config search paths
	v.AddConfigPath(".")                    // Current directory
	v.AddConfigPath("$HOME/.config/gh-arc") // User config directory
	v.AddConfigPath("/etc/gh-arc")          // System-wide config

	// Read in environment variables with GHARC_ prefix
	v.SetEnvPrefix("GHARC")
	v.AutomaticEnv()

	// Try to read config file (ignore error if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found; using defaults
	}

	// Unmarshal config into struct
	cfg = &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// Get returns the loaded configuration
func Get() *Config {
	if cfg == nil {
		// If config hasn't been loaded, load it with defaults
		cfg, _ = Load()
	}
	return cfg
}

// GetConfigFilePath returns the path to the config file being used
func GetConfigFilePath() string {
	if v == nil {
		return ""
	}
	return v.ConfigFileUsed()
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// GitHub defaults
	v.SetDefault("github.defaultBranch", "main")
	v.SetDefault("github.defaultReviewer", "")
	v.SetDefault("github.autoAssignReviewer", false)

	// Diff defaults
	v.SetDefault("diff.createAsDraft", true)
	v.SetDefault("diff.autoUpdatePR", true)
	v.SetDefault("diff.includeCommitMessages", true)

	// Land defaults
	v.SetDefault("land.defaultMergeMethod", "squash")
	v.SetDefault("land.deleteLocalBranch", true)
	v.SetDefault("land.deleteRemoteBranch", true)
	v.SetDefault("land.requireApproval", true)
	v.SetDefault("land.requireCI", true)

	// Test defaults (empty runners - auto-detect)
	v.SetDefault("test.runners", []TestRunner{})

	// Lint defaults
	v.SetDefault("lint.runners", []LintRunner{})
	v.SetDefault("lint.megaLinter.enabled", "auto")
	v.SetDefault("lint.megaLinter.config", ".mega-linter.yml")
	v.SetDefault("lint.megaLinter.fixIssues", false)

	// Output defaults
	v.SetDefault("output.verbose", false)
	v.SetDefault("output.quiet", false)
	v.SetDefault("output.json", false)
	v.SetDefault("output.color", true)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate merge method
	validMergeMethods := map[string]bool{
		"squash": true,
		"merge":  true,
		"rebase": true,
	}
	if !validMergeMethods[c.Land.DefaultMergeMethod] {
		return fmt.Errorf("invalid merge method: %s (must be squash, merge, or rebase)", c.Land.DefaultMergeMethod)
	}

	// Validate mega-linter enabled value
	validEnabledValues := map[string]bool{
		"auto":  true,
		"true":  true,
		"false": true,
	}
	if !validEnabledValues[c.Lint.MegaLinter.Enabled] {
		return fmt.Errorf("invalid lint.megaLinter.enabled value: %s (must be auto, true, or false)", c.Lint.MegaLinter.Enabled)
	}

	// Validate test runners
	for i, runner := range c.Test.Runners {
		if runner.Name == "" {
			return fmt.Errorf("test runner %d: name is required", i)
		}
		if runner.Command == "" {
			return fmt.Errorf("test runner %s: command is required", runner.Name)
		}
	}

	// Validate lint runners
	for i, runner := range c.Lint.Runners {
		if runner.Name == "" {
			return fmt.Errorf("lint runner %d: name is required", i)
		}
		if runner.Command == "" {
			return fmt.Errorf("lint runner %s: command is required", runner.Name)
		}
	}

	return nil
}

// GetMegaLinterConfigPath returns the path/URL to the mega-linter config file
// Returns (path/url, isURL, error)
// If no local config exists, returns the GitHub raw URL to gh-arc's default config
func (c *Config) GetMegaLinterConfigPath() (string, bool, error) {
	configPath := c.Lint.MegaLinter.Config

	// Check if config file exists in project
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return configPath, false, nil
		}
	}

	// Check for .mega-linter.yml in project root
	defaultPath := ".mega-linter.yml"
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath, false, nil
	}

	// Use gh-arc's default via GitHub raw URL
	defaultURL := "https://raw.githubusercontent.com/serpro69/gh-arc/main/internal/lint/default-mega-linter.yml"
	return defaultURL, true, nil
}
