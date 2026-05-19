package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

var _ Executor = (*Engine)(nil)

type Engine struct {
	opts EngineOptions
}

func NewEngine(opts EngineOptions) *Engine {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return &Engine{opts: opts}
}

func (e *Engine) Run(ctx context.Context, configs []RunnerConfig) (*ExecutionResult, error) {
	if len(configs) == 0 {
		return &ExecutionResult{Success: true}, nil
	}

	results := make([]RunResult, 0, len(configs))
	allPassed := true

	for _, cfg := range configs {
		if ctx.Err() != nil {
			allPassed = false
			break
		}
		result := e.runOne(ctx, cfg)
		results = append(results, result)
		if result.Status != StatusPassed {
			allPassed = false
		}
	}

	execResult := &ExecutionResult{
		Runners: results,
		Success: allPassed,
	}

	if !e.opts.JSONMode {
		PrintSummary(e.opts.Stdout, *execResult)
	}

	return execResult, nil
}

func (e *Engine) runOne(ctx context.Context, cfg RunnerConfig) RunResult {
	if !e.opts.JSONMode {
		PrintBanner(e.opts.Stdout, cfg.Name)
	}

	args := make([]string, 0, len(cfg.Args)+len(cfg.ExtraArgs)+len(cfg.FilePaths))
	args = append(args, cfg.Args...)
	args = append(args, cfg.ExtraArgs...)
	args = append(args, cfg.FilePaths...)

	execCtx := ctx
	var cancel context.CancelFunc
	if cfg.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(execCtx, cfg.Command, args...)
	if cfg.Timeout > 0 {
		cmd.WaitDelay = time.Second
	}
	if cfg.WorkingDir != "" {
		cmd.Dir = cfg.WorkingDir
	}

	if e.opts.JSONMode {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	} else {
		cmd.Stdout = e.opts.Stdout
		cmd.Stderr = e.opts.Stderr
	}

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := RunResult{
		Name:     cfg.Name,
		Duration: duration,
	}

	switch {
	case err == nil:
		result.Status = StatusPassed
	case errors.Is(err, exec.ErrNotFound):
		result.Status = StatusError
		result.Err = errors.New("command not found")
	case errors.Is(execCtx.Err(), context.DeadlineExceeded):
		result.Status = StatusError
		result.Err = fmt.Errorf("timed out after %s", cfg.Timeout)
	case errors.Is(execCtx.Err(), context.Canceled):
		result.Status = StatusError
		result.Err = errors.New("execution canceled")
	default:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.Status = StatusFailed
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Status = StatusError
			result.Err = fmt.Errorf("execution error: %w", err)
		}
	}

	if !e.opts.JSONMode {
		PrintResult(e.opts.Stdout, result)
	}

	return result
}
