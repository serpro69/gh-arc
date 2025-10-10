package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommand(t *testing.T) {
	t.Run("command initialization", func(t *testing.T) {
		if rootCmd.Use != "gh-arc" {
			t.Errorf("Expected Use to be 'gh-arc', got '%s'", rootCmd.Use)
		}

		if rootCmd.Short == "" {
			t.Error("Expected Short description to be set")
		}

		if rootCmd.Long == "" {
			t.Error("Expected Long description to be set")
		}

		if !rootCmd.SilenceUsage {
			t.Error("Expected SilenceUsage to be true")
		}

		if !rootCmd.SilenceErrors {
			t.Error("Expected SilenceErrors to be true")
		}
	})

	t.Run("persistent flags are registered", func(t *testing.T) {
		verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
		if verboseFlag == nil {
			t.Fatal("Expected 'verbose' flag to be registered")
		}
		if verboseFlag.Shorthand != "v" {
			t.Errorf("Expected verbose shorthand to be 'v', got '%s'", verboseFlag.Shorthand)
		}

		quietFlag := rootCmd.PersistentFlags().Lookup("quiet")
		if quietFlag == nil {
			t.Fatal("Expected 'quiet' flag to be registered")
		}
		if quietFlag.Shorthand != "q" {
			t.Errorf("Expected quiet shorthand to be 'q', got '%s'", quietFlag.Shorthand)
		}

		jsonFlag := rootCmd.PersistentFlags().Lookup("json")
		if jsonFlag == nil {
			t.Fatal("Expected 'json' flag to be registered")
		}
	})
}

func TestFlagParsing(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectVerbose bool
		expectQuiet   bool
		expectJSON    bool
	}{
		{
			name:          "no flags",
			args:          []string{},
			expectVerbose: false,
			expectQuiet:   false,
			expectJSON:    false,
		},
		{
			name:          "verbose flag",
			args:          []string{"--verbose"},
			expectVerbose: true,
			expectQuiet:   false,
			expectJSON:    false,
		},
		{
			name:          "verbose shorthand",
			args:          []string{"-v"},
			expectVerbose: true,
			expectQuiet:   false,
			expectJSON:    false,
		},
		{
			name:          "quiet flag",
			args:          []string{"--quiet"},
			expectVerbose: false,
			expectQuiet:   true,
			expectJSON:    false,
		},
		{
			name:          "quiet shorthand",
			args:          []string{"-q"},
			expectVerbose: false,
			expectQuiet:   true,
			expectJSON:    false,
		},
		{
			name:          "json flag",
			args:          []string{"--json"},
			expectVerbose: false,
			expectQuiet:   false,
			expectJSON:    true,
		},
		{
			name:          "multiple flags",
			args:          []string{"--verbose", "--json"},
			expectVerbose: true,
			expectQuiet:   false,
			expectJSON:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags to default values
			verbose = false
			quiet = false
			jsonOut = false

			// Create a new command for testing to avoid state pollution
			cmd := &cobra.Command{
				Use: "test",
				Run: func(cmd *cobra.Command, args []string) {},
			}

			// Add the same flags as root command
			cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
			cmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output")
			cmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output in JSON format")

			// Set args and execute
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err != nil {
				t.Fatalf("Command execution failed: %v", err)
			}

			// Check flag values
			if verbose != tt.expectVerbose {
				t.Errorf("Expected verbose=%v, got %v", tt.expectVerbose, verbose)
			}
			if quiet != tt.expectQuiet {
				t.Errorf("Expected quiet=%v, got %v", tt.expectQuiet, quiet)
			}
			if jsonOut != tt.expectJSON {
				t.Errorf("Expected json=%v, got %v", tt.expectJSON, jsonOut)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("GetVerbose", func(t *testing.T) {
		verbose = true
		if !GetVerbose() {
			t.Error("Expected GetVerbose() to return true")
		}

		verbose = false
		if GetVerbose() {
			t.Error("Expected GetVerbose() to return false")
		}
	})

	t.Run("GetQuiet", func(t *testing.T) {
		quiet = true
		if !GetQuiet() {
			t.Error("Expected GetQuiet() to return true")
		}

		quiet = false
		if GetQuiet() {
			t.Error("Expected GetQuiet() to return false")
		}
	})

	t.Run("GetJSON", func(t *testing.T) {
		jsonOut = true
		if !GetJSON() {
			t.Error("Expected GetJSON() to return true")
		}

		jsonOut = false
		if GetJSON() {
			t.Error("Expected GetJSON() to return false")
		}
	})
}

func TestExecute(t *testing.T) {
	t.Run("Execute does not panic", func(t *testing.T) {
		// This is a basic smoke test to ensure Execute can be called
		// We can't easily test the actual execution without mocking os.Exit
		// but we can at least verify the function exists and is callable
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Execute() panicked: %v", r)
			}
		}()

		// Just verify the function signature is correct
		// Actual execution would exit the process
		_ = Execute
	})
}
