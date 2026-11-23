//go:build integration

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestUnitCLIResults(t *testing.T) {
	for _, tt := range []struct {
		name          string
		args          []string
		runner        bool
		fail          bool
		missing       bool
		configJSON    bool
		configVerbose bool
		wantJSON      bool
		wantStatus    string
		wantText      string
		wantCode      int
	}{
		{name: "no runners", wantText: "No test runners configured"},
		{name: "empty JSON verbose", args: []string{"--json", "-vv"}, wantJSON: true},
		{name: "passing runner", runner: true, wantStatus: "passed", wantText: "✓ check: passed"},
		{name: "failed runner", runner: true, fail: true, wantStatus: "failed", wantText: "failed (exit code 7)", wantCode: 1},
		{name: "failed JSON verbose", runner: true, fail: true, args: []string{"--json", "-vv"}, wantJSON: true, wantStatus: "failed", wantCode: 1},
		{name: "missing binary JSON", runner: true, missing: true, args: []string{"--json"}, wantJSON: true, wantStatus: "error", wantCode: 1},
		{name: "configured JSON verbose", runner: true, configJSON: true, configVerbose: true, wantJSON: true, wantStatus: "passed"},
		{name: "explicit JSON false", runner: true, configJSON: true, args: []string{"--json=false"}, wantStatus: "passed", wantText: "✓ check: passed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			settings := map[string]any{
				"output": map[string]any{"json": tt.configJSON, "verbose": tt.configVerbose},
			}
			if tt.runner {
				code := 0
				if tt.fail {
					code = 7
				}
				command := "sh"
				if tt.missing {
					command = "gh-arc-test-unit-runner-not-installed"
				}
				settings["test"] = map[string]any{"runners": []map[string]any{{
					"name": "check", "command": command,
					"args": []string{"-c", fmt.Sprintf("echo runner-stdout; echo runner-stderr >&2; exit %d", code)},
				}}}
			}
			root := t.TempDir() // Unit must work without a Git repository.
			writeCLIConfig(t, root, settings)
			stdout, stderr, code := runCLI(t, root, append([]string{"unit"}, tt.args...)...)
			if code != tt.wantCode {
				t.Fatalf("exit=%d, want %d; stdout=%s stderr=%s", code, tt.wantCode, stdout, stderr)
			}
			if tt.wantJSON {
				var result struct {
					Command string `json:"command"`
					Success bool   `json:"success"`
					Runners []struct {
						Status string `json:"status"`
					} `json:"runners"`
				}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatalf("not a single JSON result: %v\n%s", err, stdout)
				}
				if stderr != "" || result.Command != "unit" || result.Success != (tt.wantCode == 0) {
					t.Fatalf("unexpected JSON result or diagnostic: %s\n%s", stdout, stderr)
				}
				if tt.wantStatus == "" {
					if result.Runners == nil || len(result.Runners) != 0 {
						t.Fatalf("expected empty runner array: %s", stdout)
					}
				} else if len(result.Runners) != 1 || result.Runners[0].Status != tt.wantStatus {
					t.Fatalf("unexpected runner status: %s", stdout)
				}
			} else {
				if !strings.Contains(stdout, tt.wantText) {
					t.Fatalf("stdout missing %q: %s", tt.wantText, stdout)
				}
				wantStderr := ""
				if tt.runner {
					wantStderr = "runner-stderr\n"
				}
				if stderr != wantStderr {
					t.Fatalf("stderr=%q, want %q", stderr, wantStderr)
				}
			}
		})
	}
}

func TestUnitCLIWorkingDirectory(t *testing.T) {
	for _, tt := range []struct {
		name       string
		repository bool
		workingDir string
		absolute   bool
	}{
		{name: "nested with omitted directory", repository: true},
		{name: "nested with relative directory", repository: true, workingDir: "backend"},
		{name: "nested with absolute directory", repository: true, absolute: true},
		{name: "outside Git with omitted directory"},
		{name: "outside Git with relative directory", workingDir: "backend"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			invocationDir := root
			if tt.repository {
				runGit(t, root, "init", "-b", "main")
				invocationDir = filepath.Join(root, "nested", "deeper")
				if err := os.MkdirAll(invocationDir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			workingDir := tt.workingDir
			expectedDir := filepath.Join(root, workingDir)
			if tt.absolute {
				workingDir = t.TempDir()
				expectedDir = workingDir
			}
			if err := os.MkdirAll(expectedDir, 0755); err != nil {
				t.Fatal(err)
			}
			// Only the correct directory has this marker. Running from the
			// nested invocation directory must fail rather than falsely pass.
			if err := os.WriteFile(filepath.Join(expectedDir, "unit-scope-marker"), []byte(tt.name), 0644); err != nil {
				t.Fatal(err)
			}
			writeCLIConfig(t, root, map[string]any{
				"test": map[string]any{"runners": []map[string]any{{
					"name": "scope", "command": "sh", "args": []string{"-c", "cat unit-scope-marker"},
					"workingDir": workingDir,
				}}},
			})
			stdout, stderr, code := runCLI(t, invocationDir, "unit")
			if code != 0 || stderr != "" || !strings.Contains(stdout, tt.name) {
				t.Fatalf("runner used wrong working directory: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestUnitCLIArguments(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, map[string]any{})
	for _, arg := range []string{"unexpected", "--all", "--fix"} {
		t.Run(arg, func(t *testing.T) {
			stdout, stderr, code := runCLI(t, root, "unit", arg)
			if code != 1 || stdout != "" || stderr == "" {
				t.Fatalf("unexpected argument accepted: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
	t.Run("help outside Git", func(t *testing.T) {
		stdout, stderr, code := runCLI(t, root, "unit", "--help")
		if code != 0 || stderr != "" || !strings.Contains(stdout, "gh-arc unit") || strings.Contains(stdout, "--fix") {
			t.Fatalf("unexpected help: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
}

func TestRunUnitErrors(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, map[string]any{
		"test": map[string]any{"runners": []map[string]any{{"name": "check", "command": "gh-arc-test-unit-runner-not-installed"}}},
	})
	t.Chdir(root)
	originalJSON := jsonOut
	t.Cleanup(func() { jsonOut = originalJSON })
	jsonOut = true
	command := &cobra.Command{}
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetContext(context.Background())
	if err := runUnit(command, nil); !errors.Is(err, ErrSilentExit) {
		t.Fatalf("runner error=%v, want ErrSilentExit", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	command.SetContext(ctx)
	if err := runUnit(command, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("command context not propagated: %v", err)
	}
	command.SetContext(context.Background())
	command.SetOut(unitFailingWriter{})
	if err := runUnit(command, nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("JSON output write error not preserved: %v", err)
	}
}

type unitFailingWriter struct{}

func (unitFailingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
