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
	rootCmd.SetArgs(args)
	Execute()
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
