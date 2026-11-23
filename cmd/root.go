package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/logger"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	verbosity int // Verbosity level: 0=warn, 1=info, 2=debug, 3=trace
	quiet     bool
	jsonOut   bool

	// cfg holds the loaded configuration
	cfg               *config.Config
	configErr         error
	configInitialized bool
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if configErr != nil {
			if isRunnerCommand(cmd) {
				return configErr
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %v\n", configErr)
		}
		if verbosity > 0 && !jsonOut && !quiet && config.GetConfigFilePath() != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), "Using config file:", config.GetConfigFilePath())
		}
		// Initialize logger after config is loaded
		logger.Init(logger.Config{
			Verbosity: verbosity,
			Quiet:     quiet,
			JSON:      jsonOut,
			Writer:    cmd.ErrOrStderr(),
		})

		// Log startup if verbosity >= 1
		if verbosity >= 1 && !jsonOut {
			logger.Info().
				Str("command", cmd.Name()).
				Msg("Starting gh-arc")
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	execute(os.Args[1:])
}

func execute(args []string) {
	rootCmd.SetArgs(args)
	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		if isRunnerCommand(cmd) {
			// Flag errors can occur before Cobra invokes initializers.
			if !configInitialized {
				initConfig()
			}
			// A flag error may have stopped parsing before --json was reached.
			if requested, ok := runnerJSONFlag(args); ok {
				jsonOut = requested
			}
			err = reportRunnerError(cmd, err)
		}
		if !errors.Is(err, ErrSilentExit) {
			fmt.Fprintln(rootCmd.ErrOrStderr(), err)
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
	configInitialized = true
	configErr = nil
	var err error
	cfg, err = config.Load()
	if err != nil {
		configErr = fmt.Errorf("failed to load configuration: %w", err)
		return
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

	if err := cfg.Validate(); err != nil {
		configErr = fmt.Errorf("invalid configuration: %w", err)
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
