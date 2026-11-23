package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/runner"
)

type mockExecutor struct {
	called  bool
	ctx     context.Context
	configs []runner.RunnerConfig
	result  *runner.ExecutionResult
	err     error
}

func (e *mockExecutor) Run(ctx context.Context, configs []runner.RunnerConfig) (*runner.ExecutionResult, error) {
	e.called, e.ctx, e.configs = true, ctx, configs
	return e.result, e.err
}

func TestUnitWorkflowNoRunners(t *testing.T) {
	for _, tt := range []struct {
		name string
		opts *UnitOptions
	}{
		{name: "normal with nil options"},
		{name: "JSON", opts: &UnitOptions{JSONMode: true, Verbose: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			executor := &mockExecutor{}
			workflow := NewUnitWorkflow(executor, &config.Config{}, WithOutput(&output))
			result, err := workflow.Execute(context.Background(), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Success || len(result.Runners) != 0 || executor.called {
				t.Fatalf("unexpected empty-config result: %+v, executor called=%v", result, executor.called)
			}
			if tt.opts != nil && tt.opts.JSONMode {
				if output.Len() != 0 {
					t.Fatalf("JSON mode printed guidance: %s", &output)
				}
			} else if !strings.Contains(output.String(), "No test runners configured") ||
				!strings.Contains(output.String(), `"test"`) || !strings.Contains(output.String(), `"go-test"`) {
				t.Fatalf("missing configuration guidance: %s", &output)
			}
			data, err := runner.FormatJSON(result.ExecutionResult, "unit")
			if err != nil {
				t.Fatal(err)
			}
			var decoded struct {
				Command string            `json:"command"`
				Success bool              `json:"success"`
				Runners []json.RawMessage `json:"runners"`
			}
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Command != "unit" || !decoded.Success || decoded.Runners == nil || len(decoded.Runners) != 0 {
				t.Fatalf("unexpected JSON: %s", data)
			}
		})
	}
}

func TestUnitWorkflowExecution(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Test: config.TestConfig{Runners: []config.TestRunner{
		{Name: "go-test", Command: "go", Args: []string{"test", "./..."}, WorkingDir: "backend", Timeout: "2m30s"},
		{Name: "frontend", Command: "npm", Args: []string{"test"}, WorkingDir: "frontend"},
	}}}
	expected := runner.ExecutionResult{
		Success: false,
		Runners: []runner.RunResult{
			{Name: "go-test", Status: runner.StatusPassed, Duration: time.Second},
			{Name: "frontend", Status: runner.StatusFailed, ExitCode: 3, Duration: 2 * time.Second},
		},
	}
	executor := &mockExecutor{result: &expected}
	var output bytes.Buffer
	workflow := NewUnitWorkflow(executor, cfg, WithOutput(&output))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := workflow.Execute(ctx, &UnitOptions{JSONMode: true})
	if err != nil {
		t.Fatal(err)
	}
	wantConfigs := []runner.RunnerConfig{
		{Name: "go-test", Command: "go", Args: []string{"test", "./..."}, WorkingDir: "backend", Timeout: 150 * time.Second},
		{Name: "frontend", Command: "npm", Args: []string{"test"}, WorkingDir: "frontend"},
	}
	if !executor.called || executor.ctx != ctx || !reflect.DeepEqual(executor.configs, wantConfigs) {
		t.Fatalf("executor did not receive original context and ordered configs: %+v", executor)
	}
	if !reflect.DeepEqual(result.ExecutionResult, expected) || result.FailedCount() != 1 {
		t.Fatalf("executor outcomes changed: %+v", result)
	}
	if output.Len() != 0 {
		t.Fatalf("workflow printed execution output: %s", &output)
	}
}

func TestUnitWorkflowTimeouts(t *testing.T) {
	for _, tt := range []struct {
		name, timeout string
		want          time.Duration
		invalid       bool
	}{
		{name: "omitted"},
		{name: "minutes", timeout: "5m", want: 5 * time.Minute},
		{name: "fractional seconds", timeout: "1.5s", want: 1500 * time.Millisecond},
		{name: "invalid duration", timeout: "5min", invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config.Config{Test: config.TestConfig{Runners: []config.TestRunner{
				{Name: "first", Command: "test"},
				{Name: "go-test", Command: "go", Timeout: tt.timeout},
			}}}
			executor := &mockExecutor{result: &runner.ExecutionResult{Success: true}}
			workflow := NewUnitWorkflow(executor, cfg, WithOutput(io.Discard))
			result, err := workflow.Execute(context.Background(), nil)
			if tt.invalid {
				if err == nil || !strings.Contains(err.Error(), `invalid timeout "5min" for runner "go-test"`) ||
					!strings.Contains(err.Error(), "'5m' or '300s'") || errors.Unwrap(err) == nil {
					t.Fatalf("expected descriptive wrapped timeout error, got %v", err)
				}
				if executor.called || result != nil {
					t.Fatal("invalid timeout must reject the whole configuration before execution")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !result.Success || len(executor.configs) != 2 || executor.configs[1].Timeout != tt.want {
				t.Fatalf("unexpected parsed config/result: %+v, %+v", executor.configs, result)
			}
		})
	}
}

func TestUnitWorkflowBaseDir(t *testing.T) {
	baseDir := t.TempDir() // Deliberately has no .git directory.
	invocationDir := filepath.Join(baseDir, "nested")
	if err := os.Mkdir(invocationDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(invocationDir)
	absoluteDir := t.TempDir()
	cfg := &config.Config{Test: config.TestConfig{Runners: []config.TestRunner{
		{Name: "default", Command: "go", Args: []string{"test", "./..."}},
		{Name: "relative", Command: "go", WorkingDir: "backend"},
		{Name: "absolute", Command: "go", WorkingDir: absoluteDir},
	}}}
	executor := &mockExecutor{result: &runner.ExecutionResult{Success: true}}
	workflow := NewUnitWorkflow(executor, cfg, WithBaseDir(baseDir), WithOutput(io.Discard))
	result, err := workflow.Execute(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || len(executor.configs) != 3 {
		t.Fatalf("unexpected result/configs: %+v, %+v", result, executor.configs)
	}
	wantDirs := []string{baseDir, filepath.Join(baseDir, "backend"), absoluteDir}
	for i, want := range wantDirs {
		if got := executor.configs[i].WorkingDir; got != want {
			t.Errorf("runner %s working directory = %q, want %q", executor.configs[i].Name, got, want)
		}
	}
	if cfg.Test.Runners[0].WorkingDir != "" || cfg.Test.Runners[1].WorkingDir != "backend" {
		t.Fatal("workflow mutated configured working directories")
	}
}

func TestUnitWorkflowErrors(t *testing.T) {
	t.Parallel()
	failure := errors.New("executor failure")
	cfg := &config.Config{Test: config.TestConfig{Runners: []config.TestRunner{{Name: "test", Command: "test"}}}}
	executor := &mockExecutor{err: failure}
	workflow := NewUnitWorkflow(executor, cfg, WithOutput(io.Discard))
	result, err := workflow.Execute(context.Background(), nil)
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "failed to execute test runners") || result != nil {
		t.Fatalf("executor error was not preserved: result=%+v, err=%v", result, err)
	}

	executor.called = false
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := workflow.Execute(ctx, nil); !errors.Is(err, context.Canceled) || executor.called {
		t.Fatalf("canceled workflow executed: called=%v, err=%v", executor.called, err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestUnitWorkflowGuidanceWriteError(t *testing.T) {
	t.Parallel()
	workflow := NewUnitWorkflow(nil, &config.Config{}, WithOutput(failingWriter{}))
	if _, err := workflow.Execute(context.Background(), nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("guidance write error was not preserved: %v", err)
	}
	result, err := workflow.Execute(context.Background(), &UnitOptions{JSONMode: true})
	if err != nil || !result.Success {
		t.Fatalf("JSON mode must not write guidance: result=%+v, err=%v", result, err)
	}
}
