package lint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/runner"
)

type mockLintRepository struct {
	path          string
	defaultBranch string
	defaultCalls  int
	defaultErr    error
	mergeCalls    [][2]string
	mergeErrors   map[string]error
	fileCalls     [][2]string
	changes       []git.FileChange
	filesErr      error
}

func (r *mockLintRepository) Path() string { return r.path }
func (r *mockLintRepository) GetLocalDefaultBranch() (string, error) {
	r.defaultCalls++
	return r.defaultBranch, r.defaultErr
}
func (r *mockLintRepository) GetMergeBase(base, head string) (string, error) {
	r.mergeCalls = append(r.mergeCalls, [2]string{base, head})
	return "merge-base-sha", r.mergeErrors[base]
}
func (r *mockLintRepository) GetFilesChanged(base, head string) ([]git.FileChange, error) {
	r.fileCalls = append(r.fileCalls, [2]string{base, head})
	return r.changes, r.filesErr
}

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

func newTestWorkflow(t *testing.T) (*LintWorkflow, *mockLintRepository, *mockExecutor, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	// Only a root marker is needed: repository operations are mocked.
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	repo := &mockLintRepository{
		path: root, defaultBranch: "main", changes: []git.FileChange{{Path: "file.go"}},
		mergeErrors: make(map[string]error),
	}
	executor := &mockExecutor{result: &runner.ExecutionResult{Success: true}}
	output := new(bytes.Buffer)
	cfg := &config.Config{Lint: config.LintConfig{Runners: []config.LintRunner{{Name: "lint", Command: "linter"}}}}
	return NewLintWorkflow(repo, executor, cfg, WithOutput(output)), repo, executor, output
}

func TestLintWorkflowNoRunners(t *testing.T) {
	for _, tt := range []struct {
		name, enabled string
		docker, json  bool
		wantNotice    bool
	}{
		{name: "disabled", enabled: "false"},
		{name: "enabled", enabled: "true", wantNotice: true},
		{name: "auto without docker", enabled: "auto"},
		{name: "auto with docker", enabled: "auto", docker: true, wantNotice: true},
		{name: "json", enabled: "true", json: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bin := t.TempDir()
			if tt.docker {
				if err := os.WriteFile(filepath.Join(bin, "docker"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin)
			w, _, executor, output := newTestWorkflow(t)
			w.config.Lint.Runners = nil
			w.config.Lint.MegaLinter.Enabled = tt.enabled
			w.repo = nil // No repository access is needed for guidance.
			result, err := w.Execute(context.Background(), &LintOptions{JSONMode: tt.json})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Success || len(result.Runners) != 0 || executor.called {
				t.Fatalf("unexpected result: %+v; executor called: %v", result, executor.called)
			}
			if tt.json {
				if output.Len() != 0 {
					t.Fatalf("JSON mode printed guidance: %s", output)
				}
				assertJSONResult(t, result, "")
			} else if !strings.Contains(output.String(), "No lint runners configured") || !strings.Contains(output.String(), `"runners"`) {
				t.Fatalf("missing configuration guidance: %s", output)
			}
			if got := strings.Contains(output.String(), "coming in a future version"); got != tt.wantNotice {
				t.Fatalf("MegaLinter notice = %v, want %v", got, tt.wantNotice)
			}
		})
	}
}

func TestLintWorkflowChangedFiles(t *testing.T) {
	t.Parallel()
	w, repo, executor, _ := newTestWorkflow(t)
	repo.changes = []git.FileChange{
		{Path: "file.go"}, {Path: "deleted.go", IsDeleted: true},
		{Path: "new name.go", OldPath: "old.go", IsRenamed: true},
		{Path: "-flag.go", IsNew: true},
	}
	executor.result = &runner.ExecutionResult{Runners: []runner.RunResult{{Name: "lint", Status: runner.StatusFailed, ExitCode: 1}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := w.Execute(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || result.FailedCount() != 1 || result.ChangedFileCount != 3 || result.AllMode {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !reflect.DeepEqual(repo.fileCalls, [][2]string{{"merge-base-sha", "HEAD"}}) || repo.defaultCalls != 1 {
		t.Fatalf("unexpected git calls: %+v", repo)
	}
	if executor.ctx != ctx || len(executor.configs) != 1 {
		t.Fatalf("executor did not receive the original context and runner: %+v", executor)
	}
	want := []string{"file.go", "new name.go", "." + string(filepath.Separator) + "-flag.go"}
	if !reflect.DeepEqual(executor.configs[0].FilePaths, want) {
		t.Fatalf("files = %v, want %v", executor.configs[0].FilePaths, want)
	}
}

func TestLintWorkflowLoadedDefaultBranch(t *testing.T) {
	for _, tt := range []struct {
		name, githubJSON, wantRef string
		wantDetection             bool
	}{
		{name: "omitted branch detects master", githubJSON: `{}`, wantRef: "origin/master", wantDetection: true},
		{name: "explicit main is respected", githubJSON: `{"defaultBranch":"main"}`, wantRef: "origin/main"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w, repo, _, _ := newTestWorkflow(t)
			t.Chdir(repo.path)
			content := `{"github":` + tt.githubJSON + `,"lint":{"runners":[{"name":"lint","command":"linter"}]}}`
			if err := os.WriteFile(filepath.Join(repo.path, ".arc.json"), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			w.config = cfg
			repo.defaultBranch = "master"
			if _, err := w.Execute(context.Background(), nil); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(repo.mergeCalls, [][2]string{{tt.wantRef, "HEAD"}}) {
				t.Fatalf("merge-base calls = %v, want %s", repo.mergeCalls, tt.wantRef)
			}
			if (repo.defaultCalls == 1) != tt.wantDetection {
				t.Fatalf("default branch calls = %d, want detection %v", repo.defaultCalls, tt.wantDetection)
			}
		})
	}
}

func TestLintWorkflowNoChangedFiles(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		for _, deletedOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("json=%v/deleted=%v", jsonMode, deletedOnly), func(t *testing.T) {
				t.Parallel()
				w, repo, executor, output := newTestWorkflow(t)
				repo.changes = nil
				if deletedOnly {
					repo.changes = []git.FileChange{{Path: "gone.go", IsDeleted: true}}
				}
				result, err := w.Execute(context.Background(), &LintOptions{JSONMode: jsonMode})
				if err != nil {
					t.Fatal(err)
				}
				if !result.Success || result.SkipReason != "no changed files" || executor.called || result.ChangedFileCount != 0 {
					t.Fatalf("unexpected result: %+v", result)
				}
				if jsonMode && output.Len() != 0 || !jsonMode && !strings.Contains(output.String(), "No changed files to lint") {
					t.Fatalf("unexpected output: %s", output)
				}
				assertJSONResult(t, result, "no changed files")
			})
		}
	}
}

func TestLintWorkflowMergeBase(t *testing.T) {
	missing := func(ref string) error { return &git.RefResolutionError{Ref: ref, Err: errors.New("missing ref")} }
	unexpected := errors.New("git storage failure")
	for _, tt := range []struct {
		name, branch, wantError string
		remoteErr, localErr     error
		wantRefs                []string
		all, json               bool
	}{
		{name: "remote preferred", wantRefs: []string{"origin/main"}},
		{name: "configured branch", branch: "trunk", wantRefs: []string{"origin/trunk"}},
		{name: "local fallback", remoteErr: missing("origin/main"), wantRefs: []string{"origin/main", "main"}},
		{name: "missing refs", remoteErr: missing("origin/main"), localErr: missing("main"), wantRefs: []string{"origin/main", "main"}, wantError: "git fetch origin"},
		{name: "remote orphan", remoteErr: fmt.Errorf("wrapped: %w", git.ErrNoCommonAncestor), wantRefs: []string{"origin/main"}, all: true},
		{name: "local orphan", remoteErr: missing("origin/main"), localErr: git.ErrNoCommonAncestor, wantRefs: []string{"origin/main", "main"}, all: true},
		{name: "orphan JSON", remoteErr: git.ErrNoCommonAncestor, wantRefs: []string{"origin/main"}, all: true, json: true},
		{name: "missing HEAD", remoteErr: missing("HEAD"), wantRefs: []string{"origin/main"}, wantError: "HEAD"},
		{name: "remote failure", remoteErr: unexpected, wantRefs: []string{"origin/main"}, wantError: unexpected.Error()},
		{name: "local failure", remoteErr: missing("origin/main"), localErr: unexpected, wantRefs: []string{"origin/main", "main"}, wantError: unexpected.Error()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w, repo, executor, output := newTestWorkflow(t)
			w.config.GitHub.DefaultBranch = tt.branch
			repo.mergeErrors["origin/main"], repo.mergeErrors["main"] = tt.remoteErr, tt.localErr
			result, err := w.Execute(context.Background(), &LintOptions{JSONMode: tt.json})
			var refs []string
			for _, call := range repo.mergeCalls {
				refs = append(refs, call[0])
				if call[1] != "HEAD" {
					t.Fatalf("unexpected merge-base head: %s", call[1])
				}
			}
			if !reflect.DeepEqual(refs, tt.wantRefs) || tt.branch != "" && repo.defaultCalls != 0 {
				t.Fatalf("unexpected ref selection: %+v", repo)
			}
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) || executor.called {
					t.Fatalf("error = %v, want %q; executor called: %v", err, tt.wantError, executor.called)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.AllMode != tt.all || !executor.called {
				t.Fatalf("unexpected result: %+v", result)
			}
			if tt.all && (len(repo.fileCalls) != 0 || len(executor.configs[0].FilePaths) != 0) {
				t.Fatal("orphan fallback must not scope files")
			}
			if tt.json && output.Len() != 0 || tt.all && !tt.json && !strings.Contains(output.String(), "⚠") {
				t.Fatalf("unexpected output: %s", output)
			}
		})
	}
}

func TestLintWorkflowRunnerOptions(t *testing.T) {
	for _, tt := range []struct {
		name                string
		fix, autoFix        bool
		fixArgs, wantExtras []string
		timeout             string
		wantTimeout         time.Duration
	}{
		{name: "defaults"},
		{name: "fix enabled", fix: true, autoFix: true, fixArgs: []string{"--fix"}, wantExtras: []string{"--fix"}},
		{name: "fix opt out", fix: true, fixArgs: []string{"--fix"}},
		{name: "fix flag absent", autoFix: true, fixArgs: []string{"--fix"}},
		{name: "no fix args", fix: true, autoFix: true},
		{name: "timeout", timeout: "2m30s", wantTimeout: 150 * time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w, repo, executor, _ := newTestWorkflow(t)
			w.config.Lint.Runners = []config.LintRunner{{Name: "check", Command: "tool", Args: []string{"check"}, AutoFix: tt.autoFix, FixArgs: tt.fixArgs, Timeout: tt.timeout}}
			result, err := w.Execute(context.Background(), &LintOptions{All: true, Fix: tt.fix})
			if err != nil {
				t.Fatal(err)
			}
			want := []runner.RunnerConfig{{Name: "check", Command: "tool", Args: []string{"check"}, ExtraArgs: tt.wantExtras, Timeout: tt.wantTimeout, WorkingDir: repo.path}}
			if !reflect.DeepEqual(executor.configs, want) {
				t.Fatalf("configs = %+v, want %+v", executor.configs, want)
			}
			if !result.AllMode || result.ChangedFileCount != 0 || len(repo.mergeCalls) != 0 || repo.defaultCalls != 0 || len(repo.fileCalls) != 0 {
				t.Fatal("all mode must bypass file detection")
			}
		})
	}
}

func TestLintWorkflowWorkingDirectories(t *testing.T) {
	for _, tt := range []struct {
		name, dir string
		absolute  bool
		wantFiles []string
	}{
		{name: "root", wantFiles: []string{"frontend/src/app.ts", "frontend-other/file.ts", "backend/file.go"}},
		{name: "explicit root", dir: ".", wantFiles: []string{"frontend/src/app.ts", "frontend-other/file.ts", "backend/file.go"}},
		{name: "relative", dir: "frontend/", wantFiles: []string{"src/app.ts"}},
		{name: "cleaned", dir: "backend/../frontend", wantFiles: []string{"src/app.ts"}},
		{name: "absolute", dir: "frontend", absolute: true, wantFiles: []string{"src/app.ts"}},
		{name: "no matching files", dir: "docs"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w, repo, executor, _ := newTestWorkflow(t)
			root := repo.path
			repo.path = filepath.Join(root, "nested", "invocation")
			if err := os.MkdirAll(repo.path, 0755); err != nil {
				t.Fatal(err)
			}
			dir := filepath.FromSlash(tt.dir)
			if tt.absolute {
				dir = filepath.Join(root, dir)
			}
			w.config.Lint.Runners[0].WorkingDir = dir
			repo.changes = []git.FileChange{{Path: "frontend/src/app.ts"}, {Path: "frontend-other/file.ts"}, {Path: "backend/file.go"}}
			result, err := w.Execute(context.Background(), &LintOptions{JSONMode: true})
			if err != nil {
				t.Fatal(err)
			}
			if result.ChangedFileCount != 3 || result.AllMode || !result.Success {
				t.Fatalf("unexpected result: %+v", result)
			}
			if len(tt.wantFiles) == 0 {
				if executor.called || result.SkipReason == "" {
					t.Fatal("runner with no applicable files must be skipped")
				}
				return
			}
			wantDir := dir
			if !filepath.IsAbs(wantDir) {
				wantDir = filepath.Join(root, wantDir)
			}
			var wantFiles []string
			for _, path := range tt.wantFiles {
				wantFiles = append(wantFiles, filepath.FromSlash(path))
			}
			if len(executor.configs) != 1 || executor.configs[0].WorkingDir != wantDir || !reflect.DeepEqual(executor.configs[0].FilePaths, wantFiles) {
				t.Fatalf("unexpected runner configs: %+v", executor.configs)
			}
		})
	}
}

func TestLintWorkflowMultipleRunners(t *testing.T) {
	t.Parallel()
	w, _, executor, _ := newTestWorkflow(t)
	w.config.Lint.Runners = []config.LintRunner{
		{Name: "first", Command: "first"},
		{Name: "unaffected", Command: "never-run", WorkingDir: "docs"},
		{Name: "last", Command: "last"},
	}
	if _, err := w.Execute(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(executor.configs) != 2 || executor.configs[0].Name != "first" || executor.configs[1].Name != "last" {
		t.Fatalf("unexpected runner order: %+v", executor.configs)
	}
}

func TestLintWorkflowErrors(t *testing.T) {
	failure := errors.New("injected failure")
	for _, tt := range []struct {
		name string
		set  func(*LintWorkflow, *mockLintRepository, *mockExecutor)
		want string
	}{
		{name: "default branch", set: func(_ *LintWorkflow, r *mockLintRepository, _ *mockExecutor) { r.defaultErr = failure }, want: "default branch"},
		{name: "file detection", set: func(_ *LintWorkflow, r *mockLintRepository, _ *mockExecutor) { r.filesErr = failure }, want: "changed files"},
		{name: "executor", set: func(_ *LintWorkflow, _ *mockLintRepository, e *mockExecutor) { e.err = failure }, want: "execute lint runners"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w, repo, executor, _ := newTestWorkflow(t)
			tt.set(w, repo, executor)
			_, err := w.Execute(context.Background(), nil)
			if !errors.Is(err, failure) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want wrapped failure with %q", err, tt.want)
			}
		})
	}
	t.Run("invalid timeout before any execution", func(t *testing.T) {
		w, _, executor, _ := newTestWorkflow(t)
		w.config.Lint.Runners = append(w.config.Lint.Runners, config.LintRunner{Name: "bad", Command: "linter", Timeout: "5min"})
		_, err := w.Execute(context.Background(), nil)
		if err == nil || !strings.Contains(err.Error(), `invalid timeout "5min" for runner "bad"`) || !strings.Contains(err.Error(), "'5m'") || executor.called {
			t.Fatalf("unexpected timeout handling: error=%v, executor called=%v", err, executor.called)
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		w, repo, executor, _ := newTestWorkflow(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := w.Execute(ctx, nil)
		if !errors.Is(err, context.Canceled) || executor.called || len(repo.mergeCalls) != 0 {
			t.Fatalf("unexpected cancellation handling: %v", err)
		}
	})
}

func assertJSONResult(t *testing.T, result *LintResult, skipped string) {
	t.Helper()
	data, err := runner.FormatJSON(result.ExecutionResult, "lint")
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Command string            `json:"command"`
		Success bool              `json:"success"`
		Runners []json.RawMessage `json:"runners"`
		Skipped string            `json:"skipped"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Command != "lint" || !decoded.Success || decoded.Runners == nil || len(decoded.Runners) != 0 || decoded.Skipped != skipped {
		t.Fatalf("unexpected JSON: %s", data)
	}
}
