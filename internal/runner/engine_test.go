package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeScript(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
	return path
}

func newTestEngine(jsonMode bool) (*Engine, *bytes.Buffer) {
	var buf bytes.Buffer
	engine := NewEngine(EngineOptions{
		JSONMode: jsonMode,
		Stdout:   &buf,
		Stderr:   &buf,
	})
	return engine, &buf
}

func TestRun_EmptyConfigs(t *testing.T) {
	engine, _ := newTestEngine(false)
	result, err := engine.Run(context.Background(), nil)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Empty(t, result.Runners)
	assert.Equal(t, 0, result.FailedCount())
}

func TestRun_PassingRunner(t *testing.T) {
	script := writeScript(t, "pass.sh", "#!/bin/sh\necho ok\n")
	engine, buf := newTestEngine(false)

	result, err := engine.Run(context.Background(), []RunnerConfig{
		{Name: "passer", Command: script},
	})

	require.NoError(t, err)
	assert.True(t, result.Success)
	require.Len(t, result.Runners, 1)
	assert.Equal(t, StatusPassed, result.Runners[0].Status)
	assert.Equal(t, 0, result.Runners[0].ExitCode)
	assert.Contains(t, buf.String(), "▶ Running passer...")
	assert.Contains(t, buf.String(), "✓ passer: passed")
	assert.Contains(t, buf.String(), "✓ 1 runner passed")
}

func TestRun_FailingRunner(t *testing.T) {
	script := writeScript(t, "fail.sh", "#!/bin/sh\necho fail >&2\nexit 1\n")
	engine, buf := newTestEngine(false)

	result, err := engine.Run(context.Background(), []RunnerConfig{
		{Name: "failer", Command: script},
	})

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Runners, 1)
	assert.Equal(t, StatusFailed, result.Runners[0].Status)
	assert.Equal(t, 1, result.Runners[0].ExitCode)
	assert.Contains(t, buf.String(), "✗ failer: failed (exit code 1)")
	assert.Contains(t, buf.String(), "✗ 1 runner failed")
}

func TestRun_CommandNotFound(t *testing.T) {
	engine, buf := newTestEngine(false)

	result, err := engine.Run(context.Background(), []RunnerConfig{
		{Name: "missing", Command: "nonexistent-command-xyz-12345"},
	})

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Runners, 1)
	assert.Equal(t, StatusError, result.Runners[0].Status)
	assert.Contains(t, result.Runners[0].Err.Error(), "command not found")
	assert.Contains(t, buf.String(), "✗ missing: command not found")
}

func TestRun_Timeout(t *testing.T) {
	script := writeScript(t, "hang.sh", "#!/bin/sh\nsleep 30\n")
	engine, buf := newTestEngine(false)

	timeout := 200 * time.Millisecond
	result, err := engine.Run(context.Background(), []RunnerConfig{
		{Name: "hanger", Command: script, Timeout: timeout},
	})

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Runners, 1)
	assert.Equal(t, StatusError, result.Runners[0].Status)
	assert.Contains(t, result.Runners[0].Err.Error(), "timed out")
	assert.Contains(t, buf.String(), "✗ hanger: timed out after")
}

func TestRun_MultipleRunnersMixed(t *testing.T) {
	passScript := writeScript(t, "pass.sh", "#!/bin/sh\nexit 0\n")
	failScript := writeScript(t, "fail.sh", "#!/bin/sh\nexit 1\n")
	engine, buf := newTestEngine(false)

	result, err := engine.Run(context.Background(), []RunnerConfig{
		{Name: "lint-a", Command: passScript},
		{Name: "lint-b", Command: failScript},
		{Name: "lint-c", Command: passScript},
	})

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Runners, 3)
	assert.Equal(t, StatusPassed, result.Runners[0].Status)
	assert.Equal(t, StatusFailed, result.Runners[1].Status)
	assert.Equal(t, StatusPassed, result.Runners[2].Status)
	assert.Equal(t, 1, result.FailedCount())
	assert.Contains(t, buf.String(), "✗ 1 of 3 runners failed")
}

func TestRun_ArgsExtraArgsFilePaths(t *testing.T) {
	script := writeScript(t, "echo-args.sh", "#!/bin/sh\nfor arg in \"$@\"; do echo \"$arg\"; done\n")
	engine, buf := newTestEngine(false)

	result, err := engine.Run(context.Background(), []RunnerConfig{
		{
			Name:      "argtest",
			Command:   script,
			Args:      []string{"--lint"},
			ExtraArgs: []string{"--fix"},
			FilePaths: []string{"file1.go", "file2.go"},
		},
	})

	require.NoError(t, err)
	assert.True(t, result.Success)

	output := buf.String()
	assert.Contains(t, output, "--lint")
	assert.Contains(t, output, "--fix")
	assert.Contains(t, output, "file1.go")
	assert.Contains(t, output, "file2.go")

	lines := strings.Split(output, "\n")
	var argLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "--lint" || trimmed == "--fix" || trimmed == "file1.go" || trimmed == "file2.go" {
			argLines = append(argLines, trimmed)
		}
	}
	assert.Equal(t, []string{"--lint", "--fix", "file1.go", "file2.go"}, argLines)
}

func TestRun_WorkingDir(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, "pwd.sh", "#!/bin/sh\npwd\n")
	engine, buf := newTestEngine(false)

	result, err := engine.Run(context.Background(), []RunnerConfig{
		{Name: "dirtest", Command: script, WorkingDir: dir},
	})

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, buf.String(), dir)
}

func TestRun_JSONModeSuppressesOutput(t *testing.T) {
	script := writeScript(t, "noisy.sh", "#!/bin/sh\necho 'SHOULD NOT APPEAR'\nexit 0\n")
	engine, buf := newTestEngine(true)

	result, err := engine.Run(context.Background(), []RunnerConfig{
		{Name: "quiet", Command: script},
	})

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Empty(t, buf.String(), "JSON mode should suppress all output")
}

func TestRun_ContextCancellation(t *testing.T) {
	script := writeScript(t, "pass.sh", "#!/bin/sh\nexit 0\n")
	engine, _ := newTestEngine(false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := engine.Run(ctx, []RunnerConfig{
		{Name: "should-skip-a", Command: script},
		{Name: "should-skip-b", Command: script},
	})

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Empty(t, result.Runners, "no runners should execute when context is already cancelled")
}

func TestFormatJSON_WithErrorField(t *testing.T) {
	result := ExecutionResult{
		Runners: []RunResult{
			{Name: "missing", Status: StatusError, Err: fmt.Errorf("command not found"), Duration: 0},
		},
		Success: false,
	}

	data, err := FormatJSON(result, "lint")
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))

	runners := parsed["runners"].([]any)
	runner := runners[0].(map[string]any)
	assert.Equal(t, "command not found", runner["error"])
}

func TestFormatJSON_NoErrorFieldWhenNil(t *testing.T) {
	result := ExecutionResult{
		Runners: []RunResult{
			{Name: "ok", Status: StatusPassed, Duration: time.Second},
		},
		Success: true,
	}

	data, err := FormatJSON(result, "lint")
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))

	runners := parsed["runners"].([]any)
	runner := runners[0].(map[string]any)
	_, hasError := runner["error"]
	assert.False(t, hasError, "error field should be omitted when nil")
}

func TestFormatJSON_WithRunners(t *testing.T) {
	result := ExecutionResult{
		Runners: []RunResult{
			{Name: "golangci-lint", Status: StatusFailed, ExitCode: 1, Duration: 1450 * time.Millisecond},
			{Name: "shellcheck", Status: StatusPassed, ExitCode: 0, Duration: 320 * time.Millisecond},
		},
		Success: false,
	}

	data, err := FormatJSON(result, "lint")
	require.NoError(t, err)

	var parsed jsonOutput
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, "lint", parsed.Command)
	assert.False(t, parsed.Success)
	require.Len(t, parsed.Runners, 2)
	assert.Equal(t, "golangci-lint", parsed.Runners[0].Name)
	assert.Equal(t, "failed", parsed.Runners[0].Status)
	assert.Equal(t, 1, parsed.Runners[0].ExitCode)
	assert.Equal(t, int64(1450), parsed.Runners[0].DurationMs)
	assert.Equal(t, "shellcheck", parsed.Runners[1].Name)
	assert.Equal(t, "passed", parsed.Runners[1].Status)
	assert.Empty(t, parsed.Skipped)
}

func TestFormatJSON_EmptyRunners(t *testing.T) {
	result := ExecutionResult{
		Success: true,
	}

	data, err := FormatJSON(result, "lint")
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, "lint", parsed["command"])
	assert.Equal(t, true, parsed["success"])
	runners := parsed["runners"].([]any)
	assert.Empty(t, runners)
	_, hasSkipped := parsed["skipped"]
	assert.False(t, hasSkipped, "skipped field should be omitted when empty")
}

func TestFormatJSON_WithSkipReason(t *testing.T) {
	result := ExecutionResult{
		Success:    true,
		SkipReason: "no changed files",
	}

	data, err := FormatJSON(result, "lint")
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, "no changed files", parsed["skipped"])
}

func TestFormatJSON_UnitCommand(t *testing.T) {
	result := ExecutionResult{
		Runners: []RunResult{
			{Name: "go-test", Status: StatusPassed, Duration: 5 * time.Second},
		},
		Success: true,
	}

	data, err := FormatJSON(result, "unit")
	require.NoError(t, err)

	var parsed jsonOutput
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, "unit", parsed.Command)
}

func TestExecutionResult_FailedCount(t *testing.T) {
	tests := []struct {
		name     string
		runners  []RunResult
		expected int
	}{
		{
			name:     "no runners",
			runners:  nil,
			expected: 0,
		},
		{
			name: "all passed",
			runners: []RunResult{
				{Status: StatusPassed},
				{Status: StatusPassed},
			},
			expected: 0,
		},
		{
			name: "one failed",
			runners: []RunResult{
				{Status: StatusPassed},
				{Status: StatusFailed},
			},
			expected: 1,
		},
		{
			name: "error counts as failed",
			runners: []RunResult{
				{Status: StatusError},
				{Status: StatusPassed},
			},
			expected: 1,
		},
		{
			name: "mixed failures and errors",
			runners: []RunResult{
				{Status: StatusFailed},
				{Status: StatusError},
				{Status: StatusPassed},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ExecutionResult{Runners: tt.runners}
			assert.Equal(t, tt.expected, result.FailedCount())
		})
	}
}

func TestPrintBanner(t *testing.T) {
	var buf bytes.Buffer
	PrintBanner(&buf, "golangci-lint")
	assert.Equal(t, "▶ Running golangci-lint...\n", buf.String())
}

func TestPrintResult_Passed(t *testing.T) {
	var buf bytes.Buffer
	PrintResult(&buf, RunResult{
		Name:     "eslint",
		Status:   StatusPassed,
		Duration: 320 * time.Millisecond,
	})
	assert.Contains(t, buf.String(), "✓ eslint: passed [0.3s]")
}

func TestPrintResult_Failed(t *testing.T) {
	var buf bytes.Buffer
	PrintResult(&buf, RunResult{
		Name:     "golangci-lint",
		Status:   StatusFailed,
		ExitCode: 1,
		Duration: 1450 * time.Millisecond,
	})
	assert.Contains(t, buf.String(), "✗ golangci-lint: failed (exit code 1) [1.4s]")
}

func TestPrintResult_Error(t *testing.T) {
	var buf bytes.Buffer
	PrintResult(&buf, RunResult{
		Name:   "missing-tool",
		Status: StatusError,
		Err:    fmt.Errorf("command not found"),
	})
	assert.Contains(t, buf.String(), "✗ missing-tool: command not found")
}

func TestPrintResult_ErrorNilErr(t *testing.T) {
	var buf bytes.Buffer
	PrintResult(&buf, RunResult{
		Name:   "broken",
		Status: StatusError,
	})
	assert.Contains(t, buf.String(), "✗ broken: error")
}

func TestPrintResult_Skipped(t *testing.T) {
	var buf bytes.Buffer
	PrintResult(&buf, RunResult{
		Name:   "optional",
		Status: StatusSkipped,
	})
	assert.Contains(t, buf.String(), "⚠ optional: skipped")
}

func TestPrintSummary_AllPassed(t *testing.T) {
	var buf bytes.Buffer
	PrintSummary(&buf, ExecutionResult{
		Runners: []RunResult{
			{Status: StatusPassed},
			{Status: StatusPassed},
		},
		Success: true,
	})
	output := buf.String()
	assert.Contains(t, output, "━━━")
	assert.Contains(t, output, "✓ 2 runners passed")
}

func TestPrintSummary_SomeFailed(t *testing.T) {
	var buf bytes.Buffer
	PrintSummary(&buf, ExecutionResult{
		Runners: []RunResult{
			{Status: StatusPassed},
			{Status: StatusFailed},
			{Status: StatusPassed},
		},
	})
	output := buf.String()
	assert.Contains(t, output, "━━━")
	assert.Contains(t, output, "✗ 1 of 3 runners failed")
}

func TestPrintSummary_SingleRunner(t *testing.T) {
	var buf bytes.Buffer
	PrintSummary(&buf, ExecutionResult{
		Runners: []RunResult{{Status: StatusPassed}},
		Success: true,
	})
	assert.Contains(t, buf.String(), "✓ 1 runner passed")
}

func TestPrintSummary_EmptyRunners(t *testing.T) {
	var buf bytes.Buffer
	PrintSummary(&buf, ExecutionResult{})
	assert.Empty(t, buf.String())
}

func TestNewEngine_DefaultWriters(t *testing.T) {
	engine := NewEngine(EngineOptions{})
	assert.Equal(t, os.Stdout, engine.opts.Stdout)
	assert.Equal(t, os.Stderr, engine.opts.Stderr)
}

func TestNewEngine_CustomWriters(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	engine := NewEngine(EngineOptions{Stdout: stdout, Stderr: stderr})
	assert.Equal(t, io.Writer(stdout), engine.opts.Stdout)
	assert.Equal(t, io.Writer(stderr), engine.opts.Stderr)
}
