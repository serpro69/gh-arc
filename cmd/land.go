package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/github"
	"github.com/serpro69/gh-arc/internal/land"
	"github.com/serpro69/gh-arc/internal/logger"
)

var (
	landSquash   bool
	landRebase   bool
	landForce    bool
	landEdit     bool
	landNoDelete bool
)

var landCmd = &cobra.Command{
	Use:   "land",
	Short: "Merge the current branch's pull request",
	Args:  cobra.NoArgs,
	Long: `Merge the current branch's pull request after verifying preconditions.

Checks approval status, CI results, and local/remote HEAD consistency before
merging via the GitHub API. After a successful merge, checks out the default
branch, pulls latest, and optionally deletes the local feature branch.

Pre-merge checks (in order):
  1. Working directory must be clean
  2. Must not be on the default branch
  3. An open PR must exist for the current branch
  4. Local HEAD must match the PR head SHA
  5. Approval status (configurable: strict, prompt, none)
  6. CI status (configurable: required, all, none)

Merge methods:
  Only squash and rebase are supported (no merge commits). The default
  method is configured via land.defaultMergeMethod (default: squash).

Examples:
  # Land with default settings (squash merge)
  gh arc land

  # Use rebase merge instead of squash
  gh arc land --rebase

  # Edit the merge commit message before merging
  gh arc land --edit

  # Bypass approval and CI checks
  gh arc land --force

  # Keep the local branch after merging
  gh arc land --no-delete`,
	RunE: runLand,
}

func init() {
	rootCmd.AddCommand(landCmd)

	landCmd.Flags().BoolVar(&landSquash, "squash", false, "Use squash merge (overrides config default)")
	landCmd.Flags().BoolVar(&landRebase, "rebase", false, "Use rebase merge (overrides config default)")
	landCmd.Flags().BoolVar(&landForce, "force", false, "Bypass approval and CI checks")
	landCmd.Flags().BoolVar(&landEdit, "edit", false, "Open $EDITOR to customize the merge commit message")
	landCmd.Flags().BoolVar(&landNoDelete, "no-delete", false, "Keep the local branch after merge")

	landCmd.MarkFlagsMutuallyExclusive("squash", "rebase")
}

func runLand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	logger.Debug().
		Bool("squash", landSquash).
		Bool("rebase", landRebase).
		Bool("force", landForce).
		Bool("edit", landEdit).
		Bool("no-delete", landNoDelete).
		Msg("Starting land command")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	currentRepo, err := repository.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current repository: %w", err)
	}

	logger.Info().
		Str("owner", currentRepo.Owner).
		Str("repo", currentRepo.Name).
		Msg("Repository detected")

	gitRepo, err := git.OpenRepository(".")
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}

	client, err := github.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	workflow := land.NewLandWorkflow(gitRepo, client, cfg, currentRepo.Owner, currentRepo.Name)

	_, err = workflow.Execute(ctx, &land.LandOptions{
		Squash:   landSquash,
		Rebase:   landRebase,
		Force:    landForce,
		Edit:     landEdit,
		NoDelete: landNoDelete,
	})
	if err != nil {
		if errors.Is(err, land.ErrMergeAborted) {
			fmt.Println("✗ Merge aborted — commit message empty or unchanged")
			return nil
		}

		if errors.Is(err, land.ErrMergeDeclined) {
			return nil
		}

		if errors.Is(err, git.ErrAuthenticationFailed) {
			fmt.Println("\n✗ Authentication failed")
			fmt.Println("Please refresh your GitHub authentication:")
			fmt.Printf("  gh auth refresh --scopes \"repo,read:user\"\n")
			return fmt.Errorf("authentication failed: %w", err)
		}

		if errors.Is(err, land.ErrDirtyWorkingDir) ||
			errors.Is(err, land.ErrOnTrunk) ||
			errors.Is(err, land.ErrNoPRFound) ||
			errors.Is(err, land.ErrLocalHeadMismatch) ||
			errors.Is(err, land.ErrApprovalFailed) ||
			errors.Is(err, land.ErrCIFailed) ||
			errors.Is(err, land.ErrNonInteractive) {
			return fmt.Errorf("land failed: %w", err)
		}

		return err
	}

	return nil
}
