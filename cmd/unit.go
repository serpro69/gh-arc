package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/runner"
	"github.com/serpro69/gh-arc/internal/unit"
)

var unitCmd = &cobra.Command{
	Use:   "unit",
	Short: "Run configured test runners",
	Long: `Run all test runners configured under test.runners in .arc.json,
.arc.yaml, or .arc.yml. Test targets are controlled by each runner's args;
they are not restricted to changed files.

Empty or relative working directories resolve from the repository root when
available, or from the invocation directory outside a Git repository. Absolute
working directories are preserved. A Git repository is not required.

With no runners configured, print setup guidance and exit successfully.

Examples:
  gh arc unit
  gh arc unit --json
  gh arc unit -v`,
	Args: cobra.NoArgs,
	RunE: runUnit,
}

func init() {
	rootCmd.AddCommand(unitCmd)
}

func runUnit(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	baseDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	if root, err := git.FindRepositoryRoot(baseDir); err == nil {
		baseDir = root
	} else if !errors.Is(err, git.ErrNotARepository) {
		return fmt.Errorf("failed to locate project directory: %w", err)
	}

	opts := &unit.UnitOptions{JSONMode: GetJSON(), Verbose: GetVerbose()}
	engine := runner.NewEngine(runner.EngineOptions{
		JSONMode: opts.JSONMode,
		Verbose:  opts.Verbose,
		Stdout:   cmd.OutOrStdout(),
		Stderr:   cmd.ErrOrStderr(),
	})
	workflow := unit.NewUnitWorkflow(
		engine,
		cfg,
		unit.WithBaseDir(baseDir),
		unit.WithOutput(cmd.OutOrStdout()),
	)
	result, err := workflow.Execute(cmd.Context(), opts)
	if err != nil {
		return err
	}
	if opts.JSONMode {
		output, err := runner.FormatJSON(result.ExecutionResult, "unit")
		if err != nil {
			return fmt.Errorf("failed to format unit results: %w", err)
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(output)); err != nil {
			return fmt.Errorf("failed to write unit results: %w", err)
		}
	}
	if !result.Success {
		return ErrSilentExit
	}
	return nil
}
