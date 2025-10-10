package cmd

import (
	"fmt"

	"github.com/serpro69/gh-arc/internal/version"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long: `Print version information for gh-arc including version number,
git commit hash, build date, Go version, and platform.

Use --json flag for machine-readable JSON output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		info := version.GetVersion()

		if GetJSON() {
			jsonOutput, err := info.JSON()
			if err != nil {
				return fmt.Errorf("failed to format version as JSON: %w", err)
			}
			fmt.Println(jsonOutput)
		} else {
			fmt.Println(info.String())
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
