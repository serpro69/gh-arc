package cmd

import (
	"fmt"
	"os"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/logger"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	verbosity int  // Verbosity level: 0=warn, 1=info, 2=debug, 3=trace
	quiet     bool
	jsonOut   bool

	// cfg holds the loaded configuration
	cfg *config.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gh-arc",
	Short: "GitHub CLI extension for trunk-based development workflow",
	Long: `gh-arc is a GitHub CLI extension implementing an opinionated trunk-based
development workflow. It wraps GitHub to provide a simplified command-line API
for code review and revision control operations.

Inspired by Arcanist, gh-arc enables developers to work within the command line
during the entire development workflow without switching contexts or opening
external tools for code-review processes.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize logger after config is loaded
		logger.Init(logger.Config{
			Verbosity: verbosity,
			Quiet:     quiet,
			JSON:      jsonOut,
		})

		// Log startup if verbosity >= 1
		if verbosity >= 1 {
			logger.Info().
				Str("command", cmd.Name()).
				Msg("Starting gh-arc")
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if !quiet {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Define persistent flags that will be global for the application
	rootCmd.PersistentFlags().CountP("verbose", "v", "Increase verbosity level (can be repeated: -v=info, -vv=debug, -vvv=trace)")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output in JSON format")

	// Bind verbosity flag to variable
	verbosity, _ = rootCmd.PersistentFlags().GetCount("verbose")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Load configuration
	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		return
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid config: %v\n", err)
	}

	// Apply config values to flags if flags weren't explicitly set
	if !rootCmd.PersistentFlags().Changed("verbose") {
		if cfg.Output.Verbose {
			verbosity = 1 // Map old boolean verbose to level 1
		}
	} else {
		// Get the count from the flag
		verbosity, _ = rootCmd.PersistentFlags().GetCount("verbose")
	}
	if !rootCmd.PersistentFlags().Changed("quiet") {
		quiet = cfg.Output.Quiet
	}
	if !rootCmd.PersistentFlags().Changed("json") {
		jsonOut = cfg.Output.JSON
	}

	// Print config file used if verbosity > 0
	if verbosity > 0 && config.GetConfigFilePath() != "" {
		fmt.Fprintln(os.Stderr, "Using config file:", config.GetConfigFilePath())
	}
}

// GetVerbosity returns the verbosity level (0=warn, 1=info, 2=debug, 3=trace)
func GetVerbosity() int {
	return verbosity
}

// GetVerbose returns true if verbosity level is >= 1
func GetVerbose() bool {
	return verbosity >= 1
}

// GetQuiet returns the quiet flag value
func GetQuiet() bool {
	return quiet
}

// GetJSON returns the JSON output flag value
func GetJSON() bool {
	return jsonOut
}

// GetConfig returns the loaded configuration
func GetConfig() *config.Config {
	if cfg == nil {
		// If config hasn't been loaded, load it
		cfg, _ = config.Load()
	}
	return cfg
}
