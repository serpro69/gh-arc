package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/lint"
	"github.com/serpro69/gh-arc/internal/runner"
)

var (
	lintFix bool
	lintAll bool
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Run configured linters on changed files",
	Long: `Run configured lint runners on committed changes since the merge-base
with the default branch. Deleted files are excluded. Use --all to pass no file
paths and let each runner's configured arguments select its targets.

Configure runners in .arc.json, .arc.yaml, or .arc.yml under lint.runners.
--fix appends a runner's fixArgs only when that runner enables autoFix.
With no runners configured, print setup guidance and exit successfully.

Examples:
  gh arc lint
  gh arc lint --fix
  gh arc lint --all
  gh arc lint --json`,
	Args: cobra.NoArgs,
	RunE: runLint,
}

func init() {
	lintCmd.Flags().BoolVar(&lintFix, "fix", false, "Auto-fix issues for runners with autoFix enabled")
	lintCmd.Flags().BoolVar(&lintAll, "all", false, "Lint all files instead of only committed changes")
	rootCmd.AddCommand(lintCmd)
}

func runLint(cmd *cobra.Command, args []string) error {
	cfg, err := loadRunnerConfig(cmd)
	if err != nil {
		return err
	}
	repo, err := git.OpenRepository(".")
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}

	opts := &lint.LintOptions{
		Fix: lintFix, All: lintAll,
		JSONMode: GetJSON(), Verbose: GetVerbose(),
	}
	engine := runner.NewEngine(runner.EngineOptions{
		JSONMode: opts.JSONMode,
		Quiet:    GetQuiet(),
		Verbose:  opts.Verbose,
		Stdout:   cmd.OutOrStdout(),
		Stderr:   cmd.ErrOrStderr(),
	})
	guidance := cmd.OutOrStdout()
	if GetQuiet() {
		guidance = io.Discard
	}
	workflow := lint.NewLintWorkflow(repo, engine, cfg, lint.WithOutput(guidance))
	result, err := workflow.Execute(cmd.Context(), opts)
	if err != nil {
		return err
	}
	return finishRunnerCommand(cmd, result.ExecutionResult, "lint")
}
