package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Global flags
	verbose bool
	quiet   bool
	jsonOut bool
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
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output in JSON format")

	// Bind flags to viper for configuration file support
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Set config name and type
	viper.SetConfigName(".arc")
	viper.SetConfigType("json")

	// Add config search paths
	viper.AddConfigPath(".")                        // Current directory
	viper.AddConfigPath("$HOME/.config/gh-arc")     // User config directory
	viper.AddConfigPath("/etc/gh-arc")              // System-wide config

	// Read in environment variables with GHARC_ prefix
	viper.SetEnvPrefix("GHARC")
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

// GetVerbose returns the verbose flag value
func GetVerbose() bool {
	return verbose
}

// GetQuiet returns the quiet flag value
func GetQuiet() bool {
	return quiet
}

// GetJSON returns the JSON output flag value
func GetJSON() bool {
	return jsonOut
}
