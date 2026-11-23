package unit

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/runner"
)

// UnitWorkflow configures the shared executor to run all test runners.
type UnitWorkflow struct {
	executor runner.Executor
	config   *config.Config
	output   io.Writer
	baseDir  string
}

// WorkflowOption configures a unit workflow.
type WorkflowOption func(*UnitWorkflow)

// WithOutput sets the destination for setup guidance.
// The caller configures the executor's output separately.
func WithOutput(w io.Writer) WorkflowOption {
	return func(workflow *UnitWorkflow) {
		if w != nil {
			workflow.output = w
		}
	}
}

// WithBaseDir resolves empty and relative runner working directories from dir.
// Absolute working directories are preserved. The caller selects the project
// root or invocation directory; this workflow does not require a Git repository.
func WithBaseDir(dir string) WorkflowOption {
	return func(workflow *UnitWorkflow) { workflow.baseDir = dir }
}

// NewUnitWorkflow creates a workflow using the supplied executor.
func NewUnitWorkflow(executor runner.Executor, cfg *config.Config, options ...WorkflowOption) *UnitWorkflow {
	w := &UnitWorkflow{executor: executor, config: cfg, output: os.Stdout}
	for _, option := range options {
		option(w)
	}
	return w
}

// Execute runs the configured test runners without changed-file scoping.
func (w *UnitWorkflow) Execute(ctx context.Context, opts *UnitOptions) (*UnitResult, error) {
	if opts == nil {
		opts = &UnitOptions{}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(w.config.Test.Runners) == 0 {
		if !opts.JSONMode {
			if err := w.printGuidance(); err != nil {
				return nil, err
			}
		}
		return &UnitResult{ExecutionResult: runner.ExecutionResult{Success: true}}, nil
	}

	configs := make([]runner.RunnerConfig, 0, len(w.config.Test.Runners))
	for _, testRunner := range w.config.Test.Runners {
		workingDir := testRunner.WorkingDir
		if w.baseDir != "" && !filepath.IsAbs(workingDir) {
			workingDir = filepath.Join(w.baseDir, workingDir)
		}
		cfg := runner.RunnerConfig{
			Name:       testRunner.Name,
			Command:    testRunner.Command,
			Args:       testRunner.Args,
			WorkingDir: workingDir,
		}
		if testRunner.Timeout != "" {
			timeout, err := time.ParseDuration(testRunner.Timeout)
			if err != nil {
				return nil, fmt.Errorf(
					"invalid timeout %q for runner %q: use Go duration format like '5m' or '300s': %w",
					testRunner.Timeout, testRunner.Name, err,
				)
			}
			cfg.Timeout = timeout
		}
		configs = append(configs, cfg)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	execution, err := w.executor.Run(ctx, configs)
	if err != nil {
		return nil, fmt.Errorf("failed to execute test runners: %w", err)
	}
	return &UnitResult{ExecutionResult: *execution}, nil
}

func (w *UnitWorkflow) printGuidance() error {
	_, err := io.WriteString(w.output, `No test runners configured.

Add runners to .arc.json:
  "test": {
    "runners": [
      { "name": "go-test", "command": "go", "args": ["test", "./..."], "timeout": "5m" }
    ]
  }
`)
	if err != nil {
		return fmt.Errorf("failed to write unit output: %w", err)
	}
	return nil
}
