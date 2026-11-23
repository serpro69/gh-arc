package lint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/runner"
)

// LintRepository supplies the repository location and committed changes to lint.
type LintRepository interface {
	Path() string
	GetLocalDefaultBranch() (string, error)
	GetMergeBase(ref1, ref2 string) (string, error)
	GetFilesChanged(base, head string) ([]git.FileChange, error)
}

var _ LintRepository = (*git.Repository)(nil)

// LintWorkflow selects files and configures the shared runner executor.
type LintWorkflow struct {
	repo     LintRepository
	executor runner.Executor
	config   *config.Config
	output   io.Writer
}

// WorkflowOption configures a lint workflow.
type WorkflowOption func(*LintWorkflow)

// WithOutput sets the destination for workflow guidance and warnings.
// The caller configures the executor's output separately.
func WithOutput(w io.Writer) WorkflowOption {
	return func(workflow *LintWorkflow) {
		if w != nil {
			workflow.output = w
		}
	}
}

// NewLintWorkflow creates a workflow using the supplied executor.
func NewLintWorkflow(
	repo LintRepository,
	executor runner.Executor,
	cfg *config.Config,
	options ...WorkflowOption,
) *LintWorkflow {
	w := &LintWorkflow{repo: repo, executor: executor, config: cfg, output: os.Stdout}
	for _, option := range options {
		option(w)
	}
	return w
}

// Execute runs configured linters on committed changes, or all files in all mode.
func (w *LintWorkflow) Execute(ctx context.Context, opts *LintOptions) (*LintResult, error) {
	if opts == nil {
		opts = &LintOptions{}
	}
	result := &LintResult{ExecutionResult: runner.ExecutionResult{Success: true}, AllMode: opts.All}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(w.config.Lint.Runners) == 0 {
		if err := w.printGuidance(opts.JSONMode); err != nil {
			return nil, err
		}
		return result, nil
	}

	var paths []string
	if !result.AllMode {
		base, err := w.mergeBase()
		switch {
		case errors.Is(err, git.ErrNoCommonAncestor):
			result.AllMode = true
			if err := w.print(opts.JSONMode, "⚠ No common ancestor with the default branch in local history (possibly shallow); linting all files.\n"); err != nil {
				return nil, err
			}
		case err != nil:
			return nil, fmt.Errorf("failed to determine lint base: %w", err)
		default:
			changes, err := w.repo.GetFilesChanged(base, "HEAD")
			if err != nil {
				return nil, fmt.Errorf("failed to get changed files: %w", err)
			}
			for _, change := range changes {
				if !change.IsDeleted {
					paths = append(paths, change.Path)
				}
			}
			result.ChangedFileCount = len(paths)
			if len(paths) == 0 {
				result.SkipReason = "no changed files"
				if err := w.print(opts.JSONMode, "No changed files to lint\n"); err != nil {
					return nil, err
				}
				return result, nil
			}
		}
	}

	root, err := git.FindRepositoryRoot(w.repo.Path())
	if err != nil {
		return nil, fmt.Errorf("failed to locate repository root: %w", err)
	}
	configs, err := w.runnerConfigs(opts, root, paths, result.AllMode)
	if err != nil {
		return nil, err
	}
	if len(configs) == 0 {
		result.SkipReason = "no changed files in runner working directories"
		if err := w.print(opts.JSONMode, "No changed files in runner working directories to lint\n"); err != nil {
			return nil, err
		}
		return result, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	execution, err := w.executor.Run(ctx, configs)
	if err != nil {
		return nil, fmt.Errorf("failed to execute lint runners: %w", err)
	}
	result.ExecutionResult = *execution
	return result, nil
}

func (w *LintWorkflow) mergeBase() (string, error) {
	branch := w.config.GitHub.DefaultBranch
	if branch == "" {
		var err error
		branch, err = w.repo.GetLocalDefaultBranch()
		if err != nil {
			return "", fmt.Errorf("failed to get default branch: %w", err)
		}
	}
	remote := "origin/" + branch
	base, err := w.repo.GetMergeBase(remote, "HEAD")
	var refErr *git.RefResolutionError
	if !errors.As(err, &refErr) || refErr.Ref != remote {
		return base, err
	}
	base, err = w.repo.GetMergeBase(branch, "HEAD")
	if errors.As(err, &refErr) && refErr.Ref == branch {
		return "", fmt.Errorf("cannot resolve default branch %q or %q; run 'git fetch origin': %w", remote, branch, err)
	}
	return base, err
}

func (w *LintWorkflow) runnerConfigs(opts *LintOptions, root string, paths []string, all bool) ([]runner.RunnerConfig, error) {
	configs := make([]runner.RunnerConfig, 0, len(w.config.Lint.Runners))
	for _, lintRunner := range w.config.Lint.Runners {
		workingDir := lintRunner.WorkingDir
		if !filepath.IsAbs(workingDir) {
			workingDir = filepath.Join(root, workingDir)
		}
		cfg := runner.RunnerConfig{
			Name: lintRunner.Name, Command: lintRunner.Command,
			Args: lintRunner.Args, WorkingDir: workingDir,
		}
		if lintRunner.Timeout != "" {
			timeout, err := time.ParseDuration(lintRunner.Timeout)
			if err != nil {
				return nil, fmt.Errorf(
					"invalid timeout %q for runner %q: use Go duration format like '5m' or '300s': %w",
					lintRunner.Timeout, lintRunner.Name, err,
				)
			}
			cfg.Timeout = timeout
		}
		if opts.Fix && lintRunner.AutoFix && len(lintRunner.FixArgs) > 0 {
			cfg.ExtraArgs = lintRunner.FixArgs
		}
		if !all {
			// Compare physical directory paths so absolute workingDir aliases
			// (for example /tmp and /private/tmp) select the same changed files.
			pathRoot, pathDir := root, workingDir
			resolvedDir, err := filepath.EvalSymlinks(workingDir)
			switch {
			case err == nil:
				pathDir = resolvedDir
				pathRoot, err = filepath.EvalSymlinks(root)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve repository directory: %w", err)
				}
			case !errors.Is(err, os.ErrNotExist):
				return nil, fmt.Errorf("failed to resolve working directory for runner %q: %w", cfg.Name, err)
			}
			for _, path := range paths {
				rel, err := filepath.Rel(pathDir, filepath.Join(pathRoot, filepath.FromSlash(path)))
				if err != nil {
					return nil, fmt.Errorf("failed to normalize file %q for runner %q: %w", path, cfg.Name, err)
				}
				if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					continue
				}
				// An explicit relative path also prevents filenames beginning
				// with '-' from being interpreted as runner options.
				if strings.HasPrefix(rel, "-") {
					rel = "." + string(filepath.Separator) + rel
				}
				cfg.FilePaths = append(cfg.FilePaths, rel)
			}
			if len(cfg.FilePaths) == 0 {
				continue
			}
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}

func (w *LintWorkflow) print(jsonMode bool, message string) error {
	if jsonMode {
		return nil
	}
	if _, err := io.WriteString(w.output, message); err != nil {
		return fmt.Errorf("failed to write lint output: %w", err)
	}
	return nil
}

func (w *LintWorkflow) printGuidance(jsonMode bool) error {
	if jsonMode {
		return nil
	}
	enabled := w.config.Lint.MegaLinter.Enabled == "true"
	if w.config.Lint.MegaLinter.Enabled == "auto" {
		_, err := exec.LookPath("docker")
		enabled = err == nil
	}
	if enabled {
		if err := w.print(false, "MegaLinter support is coming in a future version.\n\n"); err != nil {
			return err
		}
	}
	return w.print(false, `No lint runners configured.

Add runners to .arc.json:
  "lint": {
    "runners": [
      { "name": "eslint", "command": "eslint", "args": [] }
    ]
  }
`)
}
