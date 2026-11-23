//go:build integration

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestLintCLIProcess runs the real root command in a child process so tests
// observe Execute's os.Exit behavior without replacing production exit logic.
func TestLintCLIProcess(t *testing.T) {
	if os.Getenv("GHARC_LINT_CLI_TEST") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		t.Fatal("missing command arguments")
	}
	args = args[1:]
	rootCmd.AddCommand(&cobra.Command{
		Use: "test-wrapped-silent",
		RunE: func(*cobra.Command, []string) error {
			return fmt.Errorf("wrapped failure: %w", ErrSilentExit)
		},
	})
	rootCmd.SetArgs(args)
	Execute()
	os.Exit(0)
}

func newLintCLIRepo(t *testing.T, settings map[string]any) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false",
			"commit", "--allow-empty", "-m", "initial"},
	} {
		runLintGit(t, root, args...)
	}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".arc.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

func runLintGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func runLintCLI(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, append([]string{"-test.run=^TestLintCLIProcess$", "--"}, args...)...)
	cmd.Dir = dir
	// Keep inherited GHARC settings from changing these fixtures.
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GHARC_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GHARC_LINT_CLI_TEST=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatal(err)
		}
		code = exitErr.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

func TestLintCLIResults(t *testing.T) {
	for _, tt := range []struct {
		name          string
		args          []string
		runner        bool
		fail          bool
		missing       bool
		changed       bool
		configJSON    bool
		configVerbose bool
		wantJSON      bool
		wantStatus    string
		wantText      string
		wantSkip      string
		wantCode      int
	}{
		{name: "no runners", wantText: "No lint runners configured"},
		{name: "no runners JSON verbose", args: []string{"--json", "-vv"}, wantJSON: true},
		{name: "no changed files", runner: true, wantText: "No changed files to lint"},
		{name: "no changes JSON", runner: true, args: []string{"--json"}, wantJSON: true, wantSkip: "no changed files"},
		{name: "committed changes JSON verbose", runner: true, changed: true, args: []string{"--json", "-vv"}, wantJSON: true, wantStatus: "passed"},
		{name: "passing runner", runner: true, args: []string{"--all"}, wantText: "✓ check: passed", wantStatus: "passed"},
		{name: "fix arguments", runner: true, args: []string{"--all", "--fix"}, wantText: "base --fix", wantStatus: "passed"},
		{name: "failed runner", runner: true, fail: true, args: []string{"--all"}, wantText: "failed (exit code 7)", wantCode: 1, wantStatus: "failed"},
		{name: "failed JSON verbose", runner: true, fail: true, args: []string{"--all", "--json", "-vv"}, wantJSON: true, wantCode: 1, wantStatus: "failed"},
		{name: "missing binary JSON", runner: true, missing: true, args: []string{"--all", "--json"}, wantJSON: true, wantCode: 1, wantStatus: "error"},
		{name: "configured JSON verbose", runner: true, configJSON: true, configVerbose: true, args: []string{"--all"}, wantJSON: true, wantStatus: "passed"},
		{name: "explicit JSON false", runner: true, configJSON: true, args: []string{"--all", "--json=false"}, wantText: "✓ check: passed", wantStatus: "passed"},
		{name: "unexpected positional argument", runner: true, args: []string{"unexpected"}, wantCode: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			settings := map[string]any{
				"output": map[string]any{"json": tt.configJSON, "verbose": tt.configVerbose},
				"lint":   map[string]any{"megaLinter": map[string]any{"enabled": "false"}},
			}
			if tt.runner {
				code := 0
				if tt.fail {
					code = 7
				}
				command := "sh"
				if tt.missing {
					command = "gh-arc-test-linter-not-installed"
				}
				settings["lint"].(map[string]any)["runners"] = []map[string]any{{
					"name": "check", "command": command,
					"args":    []string{"-c", fmt.Sprintf(`echo runner-stdout; echo runner-stderr >&2; echo "$@"; exit %d`, code), "check", "base"},
					"autoFix": true, "fixArgs": []string{"--fix"},
				}}
			}
			root := newLintCLIRepo(t, settings)
			if tt.changed {
				runLintGit(t, root, "checkout", "-b", "feature")
				if err := os.WriteFile(filepath.Join(root, "changed.txt"), []byte("change\n"), 0644); err != nil {
					t.Fatal(err)
				}
				runLintGit(t, root, "add", "changed.txt")
				runLintGit(t, root,
					"-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false",
					"commit", "-m", "change",
				)
			}
			stdout, stderr, code := runLintCLI(t, root, append([]string{"lint"}, tt.args...)...)
			if code != tt.wantCode {
				t.Fatalf("exit = %d, want %d; stdout=%s; stderr=%s", code, tt.wantCode, stdout, stderr)
			}
			if tt.wantJSON {
				var result struct {
					Command string `json:"command"`
					Success bool   `json:"success"`
					Skipped string `json:"skipped"`
					Runners []struct {
						Status string `json:"status"`
					} `json:"runners"`
				}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatalf("not a single JSON result: %v\n%s", err, stdout)
				}
				if stderr != "" || result.Command != "lint" || result.Success != (tt.wantCode == 0) || result.Skipped != tt.wantSkip {
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
				if tt.wantStatus != "" && stderr != "runner-stderr\n" {
					t.Fatalf("runner stderr not preserved or duplicate error printed: %q", stderr)
				}
			}
		})
	}
}

func TestLintCLIExitHandling(t *testing.T) {
	root := newLintCLIRepo(t, map[string]any{})
	t.Run("wrapped sentinel remains silent", func(t *testing.T) {
		stdout, stderr, code := runLintCLI(t, root, "test-wrapped-silent")
		if code != 1 || stdout != "" || stderr != "" {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
	t.Run("ordinary errors still print", func(t *testing.T) {
		stdout, stderr, code := runLintCLI(t, root, "lint", "unexpected")
		if code != 1 || stdout != "" || !strings.Contains(stderr, "unexpected") {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
	t.Run("help outside repository", func(t *testing.T) {
		stdout, stderr, code := runLintCLI(t, t.TempDir(), "lint", "--help")
		if code != 0 || stderr != "" || !strings.Contains(stdout, "--fix") || !strings.Contains(stdout, "--all") {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
	t.Run("other command verbose diagnostics preserved", func(t *testing.T) {
		_, stderr, code := runLintCLI(t, root, "version", "-v")
		if code != 0 || !strings.Contains(stderr, "Using config file:") || !strings.Contains(stderr, "Starting gh-arc") {
			t.Fatalf("exit=%d stderr=%q", code, stderr)
		}
	})
}

func TestRunLintErrors(t *testing.T) {
	root := newLintCLIRepo(t, map[string]any{
		"lint": map[string]any{"runners": []map[string]any{{"name": "check", "command": "gh-arc-test-linter-not-installed"}}},
	})
	t.Chdir(root)
	originalAll, originalFix, originalJSON := lintAll, lintFix, jsonOut
	t.Cleanup(func() { lintAll, lintFix, jsonOut = originalAll, originalFix, originalJSON })
	lintAll, lintFix, jsonOut = true, false, false
	command := &cobra.Command{}
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetContext(context.Background())
	if err := runLint(command, nil); !errors.Is(err, ErrSilentExit) {
		t.Fatalf("runner error = %v, want ErrSilentExit", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	command.SetContext(ctx)
	if err := runLint(command, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("command context not propagated: %v", err)
	}
}
