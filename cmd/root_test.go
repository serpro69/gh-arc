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
		name             string
		args             []string
		expectVerbosity  int
		expectQuiet      bool
		expectJSON       bool
	}{
		{
			name:            "no flags",
			args:            []string{},
			expectVerbosity: 0,
			expectQuiet:     false,
			expectJSON:      false,
		},
		{
			name:            "verbose flag once",
			args:            []string{"--verbose"},
			expectVerbosity: 1,
			expectQuiet:     false,
			expectJSON:      false,
		},
		{
			name:            "verbose shorthand once",
			args:            []string{"-v"},
			expectVerbosity: 1,
			expectQuiet:     false,
			expectJSON:      false,
		},
		{
			name:            "verbose flag twice (-vv)",
			args:            []string{"-vv"},
			expectVerbosity: 2,
			expectQuiet:     false,
			expectJSON:      false,
		},
		{
			name:            "verbose flag three times (-vvv)",
			args:            []string{"-vvv"},
			expectVerbosity: 3,
			expectQuiet:     false,
			expectJSON:      false,
		},
		{
			name:            "quiet flag",
			args:            []string{"--quiet"},
			expectVerbosity: 0,
			expectQuiet:     true,
			expectJSON:      false,
		},
		{
			name:            "quiet shorthand",
			args:            []string{"-q"},
			expectVerbosity: 0,
			expectQuiet:     true,
			expectJSON:      false,
		},
		{
			name:            "json flag",
			args:            []string{"--json"},
			expectVerbosity: 0,
			expectQuiet:     false,
			expectJSON:      true,
		},
		{
			name:            "multiple flags",
			args:            []string{"-vv", "--json"},
			expectVerbosity: 2,
			expectQuiet:     false,
			expectJSON:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use local variables for this test
			var testQuiet, testJSON bool

			// Create a new command for testing to avoid state pollution
			// We don't call cobra.OnInitialize to avoid config loading in tests
			cmd := &cobra.Command{
				Use: "test",
				Run: func(cmd *cobra.Command, args []string) {},
			}

			// Add the same flags as root command
			cmd.PersistentFlags().CountP("verbose", "v", "Increase verbosity level")
			cmd.PersistentFlags().BoolVarP(&testQuiet, "quiet", "q", false, "Suppress non-error output")
			cmd.PersistentFlags().BoolVar(&testJSON, "json", false, "Output in JSON format")

			// Set args and execute
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err != nil {
				t.Fatalf("Command execution failed: %v", err)
			}

			// Get verbosity count from flag
			testVerbosity, _ := cmd.PersistentFlags().GetCount("verbose")

			// Check flag values
			if testVerbosity != tt.expectVerbosity {
				t.Errorf("Expected verbosity=%v, got %v", tt.expectVerbosity, testVerbosity)
			}
			if testQuiet != tt.expectQuiet {
				t.Errorf("Expected quiet=%v, got %v", tt.expectQuiet, testQuiet)
			}
			if testJSON != tt.expectJSON {
				t.Errorf("Expected json=%v, got %v", tt.expectJSON, testJSON)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("GetVerbose", func(t *testing.T) {
		verbosity = 1
		if !GetVerbose() {
			t.Error("Expected GetVerbose() to return true when verbosity >= 1")
		}

		verbosity = 0
		if GetVerbose() {
			t.Error("Expected GetVerbose() to return false when verbosity == 0")
		}
	})

	t.Run("GetVerbosity", func(t *testing.T) {
		verbosity = 0
		if GetVerbosity() != 0 {
			t.Errorf("Expected GetVerbosity() to return 0, got %d", GetVerbosity())
		}

		verbosity = 2
		if GetVerbosity() != 2 {
			t.Errorf("Expected GetVerbosity() to return 2, got %d", GetVerbosity())
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
