package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/serpro69/gh-arc/internal/logger"
	"github.com/spf13/cobra"
)

// AuthStatus represents the authentication status
type AuthStatus struct {
	Authenticated bool   `json:"authenticated"`
	User          string `json:"user,omitempty"`
	Error         string `json:"error,omitempty"`
}

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Verify GitHub authentication status",
	Long: `Check if the GitHub CLI is properly authenticated and display
the current user information. This command verifies that gh-arc
can communicate with the GitHub API.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug().Msg("Checking GitHub authentication status")

		// Create GitHub API client
		client, err := api.DefaultRESTClient()
		if err != nil {
			status := AuthStatus{
				Authenticated: false,
				Error:         err.Error(),
			}
			return outputAuthStatus(status)
		}

		// Get authenticated user
		var response struct {
			Login string `json:"login"`
		}

		err = client.Get("user", &response)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to get authenticated user")
			status := AuthStatus{
				Authenticated: false,
				Error:         "Not authenticated or unable to reach GitHub API",
			}
			return outputAuthStatus(status)
		}

		logger.Info().Str("user", response.Login).Msg("Successfully authenticated")

		status := AuthStatus{
			Authenticated: true,
			User:          response.Login,
		}

		return outputAuthStatus(status)
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
}

// outputAuthStatus outputs the authentication status based on the JSON flag
func outputAuthStatus(status AuthStatus) error {
	if GetJSON() {
		output, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
	} else {
		if status.Authenticated {
			fmt.Printf("✓ Authenticated as %s\n", status.User)
		} else {
			fmt.Printf("✗ Not authenticated\n")
			if status.Error != "" {
				fmt.Printf("Error: %s\n", status.Error)
			}
			fmt.Println("\nPlease authenticate using: gh auth login")
		}
	}

	// Return error if not authenticated (for exit code)
	if !status.Authenticated {
		return fmt.Errorf("not authenticated")
	}

	return nil
}
