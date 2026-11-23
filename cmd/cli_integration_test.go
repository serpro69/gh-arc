//go:build integration

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestCLIProcess runs the real root command in a child process so tests
// observe Execute's os.Exit behavior without replacing production exit logic.
func TestCLIProcess(t *testing.T) {
	if os.Getenv("GHARC_CLI_TEST") != "1" {
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
	execute(args)
	os.Exit(0)
}

func newCLIRepo(t *testing.T, settings map[string]any) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false",
			"commit", "--allow-empty", "-m", "initial"},
	} {
		runGit(t, root, args...)
	}
	writeCLIConfig(t, root, settings)
	return root
}

func writeCLIConfig(t *testing.T, dir string, settings map[string]any) {
	t.Helper()
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".arc.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func runCLI(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, append([]string{"-test.run=^TestCLIProcess$", "--"}, args...)...)
	cmd.Dir = dir
	// Keep inherited GHARC settings from changing these fixtures.
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GHARC_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GHARC_CLI_TEST=1")
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

func TestCLIMalformedConfig(t *testing.T) {
	for _, command := range []string{"lint", "unit"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			root := newCLIRepo(t, map[string]any{})
			if err := os.WriteFile(filepath.Join(root, ".arc.json"), []byte("{"), 0o644); err != nil {
				t.Fatal(err)
			}
			// A valid lower-priority config cannot turn this into an empty success.
			if err := os.WriteFile(filepath.Join(root, ".arc.yaml"), []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, code := runCLI(t, root, command)
			if code != 1 || stdout != "" || !strings.Contains(stderr, ".arc.json") {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestCLIQuiet(t *testing.T) {
	for _, command := range []string{"lint", "unit"} {
		for _, tt := range []struct {
			name        string
			runner      bool
			fail        bool
			configQuiet bool
			args        []string
			wantJSON    bool
			wantNormal  bool
		}{
			{name: "no runners", args: []string{"--quiet"}},
			{name: "success", runner: true, args: []string{"--quiet", "-vv"}},
			{name: "failure", runner: true, fail: true, args: []string{"--quiet"}},
			{name: "configured quiet", runner: true, configQuiet: true},
			{name: "explicit false", runner: true, configQuiet: true, args: []string{"--quiet=false"}, wantNormal: true},
			{name: "JSON takes precedence", runner: true, args: []string{"--quiet", "--json"}, wantJSON: true},
		} {
			t.Run(command+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				section := "test"
				args := []string{command}
				if command == "lint" {
					section = "lint"
					args = append(args, "--all")
				}
				settings := map[string]any{"output": map[string]any{"quiet": tt.configQuiet}}
				wantCode := 0
				if tt.runner {
					script := "echo tool-stdout"
					if tt.fail {
						script += "; echo tool-stderr >&2; exit 7"
						wantCode = 1
					}
					settings[section] = map[string]any{"runners": []map[string]any{{
						"name": "check", "command": "sh", "args": []string{"-c", script},
					}}}
				}
				root := newCLIRepo(t, settings)
				stdout, stderr, code := runCLI(t, root, append(args, tt.args...)...)
				if code != wantCode {
					t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
				}
				switch {
				case tt.wantJSON:
					var result struct {
						Command string `json:"command"`
						Success bool   `json:"success"`
					}
					if err := json.Unmarshal([]byte(stdout), &result); err != nil || !result.Success || result.Command != command {
						t.Fatalf("quiet JSON result: %s; %v", stdout, err)
					}
				case tt.wantNormal:
					if !strings.Contains(stdout, "tool-stdout") || !strings.Contains(stdout, "✓ check: passed") {
						t.Fatalf("explicit quiet=false did not restore output: %q", stdout)
					}
				default:
					if stdout != "" {
						t.Fatalf("quiet stdout = %q", stdout)
					}
				}
				if tt.fail {
					if !strings.Contains(stderr, "tool-stderr") || !strings.Contains(stderr, "failed (exit code 7)") {
						t.Fatalf("quiet suppressed failure diagnostics: %q", stderr)
					}
				} else if stderr != "" {
					t.Fatalf("unexpected stderr: %q", stderr)
				}
			})
		}
	}

	t.Run("setup error remains visible", func(t *testing.T) {
		root := newCLIRepo(t, map[string]any{})
		stdout, stderr, code := runCLI(t, root, "unit", "--quiet", "unexpected")
		if code != 1 || stdout != "" || !strings.Contains(stderr, "unexpected") {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
}

func TestCLISetupErrors(t *testing.T) {
	for _, command := range []string{"lint", "unit"} {
		for _, tt := range []struct {
			name       string
			failure    string
			args       []string
			configJSON bool
			wantJSON   bool
			wantError  string
		}{
			{name: "invalid timeout flag", failure: "timeout", args: []string{"--json", "-vv"}, wantJSON: true, wantError: "invalid timeout"},
			{name: "invalid timeout config", failure: "timeout", configJSON: true, wantJSON: true, wantError: "invalid timeout"},
			{name: "invalid config flag", failure: "validation", args: []string{"--json", "-vv"}, wantJSON: true, wantError: "invalid configuration"},
			{name: "invalid config preference", failure: "validation", configJSON: true, wantJSON: true, wantError: "invalid configuration"},
			{name: "quiet error", failure: "validation", args: []string{"--json", "--quiet"}, wantJSON: true, wantError: "invalid configuration"},
			{name: "explicit JSON false", failure: "validation", configJSON: true, args: []string{"--json=false"}, wantError: "invalid configuration"},
			{name: "malformed config", failure: "malformed", args: []string{"--json", "-vv"}, wantJSON: true, wantError: ".arc.json"},
			{name: "malformed config normal", failure: "malformed", wantError: ".arc.json"},
			{name: "argument error", args: []string{"--json", "unexpected"}, wantJSON: true, wantError: "unexpected"},
			{name: "configured argument error", configJSON: true, args: []string{"unexpected"}, wantJSON: true, wantError: "unexpected"},
			{name: "flag error", args: []string{"--json", "--unknown"}, wantJSON: true, wantError: "unknown flag"},
			{name: "flag error before JSON", args: []string{"--unknown", "--json"}, wantJSON: true, wantError: "unknown flag"},
			{name: "flag error explicit false", configJSON: true, args: []string{"--unknown", "--json=false"}, wantError: "unknown flag"},
			{name: "JSON after separator is positional", args: []string{"--", "--json"}, wantError: "unknown command"},
			{name: "configured flag error", configJSON: true, args: []string{"--unknown"}, wantJSON: true, wantError: "unknown flag"},
		} {
			t.Run(command+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				section := "test"
				args := []string{command}
				if command == "lint" {
					section = "lint"
					args = append(args, "--all")
				}
				check := map[string]any{"name": "check", "command": "sh", "args": []string{"-c", "echo ran > runner-started"}}
				settings := map[string]any{
					section:  map[string]any{"runners": []map[string]any{check}},
					"output": map[string]any{"json": tt.configJSON},
				}
				if tt.failure == "timeout" {
					check["timeout"] = "5min"
				}
				if tt.failure == "validation" {
					settings["land"] = map[string]any{"requireCI": "invalid"}
				}
				root := newCLIRepo(t, settings)
				if tt.failure == "malformed" {
					if err := os.WriteFile(filepath.Join(root, ".arc.json"), []byte("{"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				stdout, stderr, code := runCLI(t, root, append(args, tt.args...)...)
				if tt.wantJSON {
					assertCLIErrorJSON(t, command, stdout, stderr, code, tt.wantError)
				} else if code != 1 || stdout != "" || strings.Count(stderr, tt.wantError) != 1 {
					t.Fatalf("expected one ordinary error: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
				}
				if _, err := os.Stat(filepath.Join(root, "runner-started")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("runner started despite setup failure: %v", err)
				}
			})
		}
	}

	t.Run("lint outside repository", func(t *testing.T) {
		root := t.TempDir()
		writeCLIConfig(t, root, map[string]any{})
		stdout, stderr, code := runCLI(t, root, "lint", "--json", "-vv")
		assertCLIErrorJSON(t, "lint", stdout, stderr, code, "git repository")
	})
	t.Run("lint missing base", func(t *testing.T) {
		root := newCLIRepo(t, map[string]any{
			"output": map[string]any{"json": true},
			"github": map[string]any{"defaultBranch": "absent"},
			"lint":   map[string]any{"runners": []map[string]any{{"name": "check", "command": "sh"}}},
		})
		stdout, stderr, code := runCLI(t, root, "lint", "-vv")
		assertCLIErrorJSON(t, "lint", stdout, stderr, code, "git fetch origin")
	})
}

func assertCLIErrorJSON(t *testing.T, command, stdout, stderr string, code int, message string) {
	t.Helper()
	var result struct {
		Command string            `json:"command"`
		Success bool              `json:"success"`
		Runners []json.RawMessage `json:"runners"`
		Error   string            `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected one JSON error object: %v; stdout=%q stderr=%q", err, stdout, stderr)
	}
	if code != 1 || stderr != "" || result.Command != command || result.Success || result.Runners == nil || len(result.Runners) != 0 || !strings.Contains(result.Error, message) {
		t.Fatalf("unexpected JSON error: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
