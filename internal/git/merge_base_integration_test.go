//go:build integration

package git

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGetMergeBaseErrorClassification(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-b", "main")
	run("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "initial")
	initial := run("rev-parse", "HEAD")
	run("branch", "feature")
	run("checkout", "--orphan", "orphan")
	run("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "unrelated")
	repo, err := OpenRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, base, head, missing string
		orphan                    bool
	}{
		{name: "common history", base: "main", head: "feature"},
		{name: "unrelated history", base: "main", head: "orphan", orphan: true},
		{name: "missing remote", base: "origin/main", head: "HEAD", missing: "origin/main"},
		{name: "missing head", base: "main", head: "missing", missing: "missing"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			base, err := repo.GetMergeBase(tt.base, tt.head)
			switch {
			case tt.orphan:
				if !errors.Is(err, ErrNoCommonAncestor) {
					t.Fatalf("error = %v, want ErrNoCommonAncestor", err)
				}
			case tt.missing != "":
				var refErr *RefResolutionError
				if !errors.As(err, &refErr) || refErr.Ref != tt.missing || errors.Unwrap(refErr) == nil || errors.Is(err, ErrNoCommonAncestor) {
					t.Fatalf("error = %v, want resolution failure for %s", err, tt.missing)
				}
			default:
				if err != nil || base != initial {
					t.Fatalf("merge base = %s, %v; want %s", base, err, initial)
				}
			}
		})
	}

	t.Run("local default branch without cached remote HEAD", func(t *testing.T) {
		branch, err := repo.GetLocalDefaultBranch()
		if err != nil || branch != "main" {
			t.Fatalf("default branch = %q, %v; want main", branch, err)
		}
	})
	t.Run("cached remote HEAD preferred over local main", func(t *testing.T) {
		// The remote is unavailable; its cached default still takes priority.
		run("remote", "add", "origin", root+"/unavailable")
		run("update-ref", "refs/remotes/origin/trunk", initial)
		run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
		branch, err := repo.GetLocalDefaultBranch()
		if err != nil || branch != "trunk" {
			t.Fatalf("default branch = %q, %v; want trunk", branch, err)
		}
	})
}
