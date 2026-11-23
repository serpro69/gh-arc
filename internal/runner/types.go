package runner

import (
	"context"
	"io"
	"time"
)

type Status string

const (
	StatusPassed  Status = "passed"
	StatusFailed  Status = "failed"
	StatusError   Status = "error"
	StatusSkipped Status = "skipped"
)

type RunnerConfig struct {
	Name       string
	Command    string
	Args       []string
	ExtraArgs  []string
	WorkingDir string
	Timeout    time.Duration
	FilePaths  []string
}

type RunResult struct {
	Name     string
	Status   Status
	ExitCode int
	Duration time.Duration
	Err      error
}

type ExecutionResult struct {
	Runners    []RunResult
	Success    bool
	SkipReason string
}

func (r *ExecutionResult) FailedCount() int {
	count := 0
	for _, runner := range r.Runners {
		if runner.Status == StatusFailed || runner.Status == StatusError {
			count++
		}
	}
	return count
}

type EngineOptions struct {
	JSONMode bool
	Quiet    bool // suppress stdout and successful framing; retain stderr and failures
	Verbose  bool // reserved for CLI verbosity; currently does not alter runner output
	Stdout   io.Writer
	Stderr   io.Writer
}

type Executor interface {
	Run(ctx context.Context, configs []RunnerConfig) (*ExecutionResult, error)
}
