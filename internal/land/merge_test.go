package land

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/serpro69/gh-arc/internal/github"
)

// --- mocks ---

type mockMergerClient struct {
	result *github.MergeResult
	err    error
	called bool
	opts   *github.MergeOptions
}

func (m *mockMergerClient) MergePullRequestForCurrentRepo(_ context.Context, _ int, opts *github.MergeOptions) (*github.MergeResult, error) {
	m.called = true
	m.opts = opts
	return m.result, m.err
}

func testPR() *github.PullRequest {
	return &github.PullRequest{
		Number: 42,
		Title:  "Add auth middleware",
		Body:   "This PR adds authentication middleware.",
		Head:   github.PRBranch{Ref: "feature/auth", SHA: "abc1234"},
		Base:   github.PRBranch{Ref: "main"},
	}
}

// --- Execute ---

func TestMergeExecutor_Execute(t *testing.T) {
	t.Run("successful squash merge", func(t *testing.T) {
		client := &mockMergerClient{
			result: &github.MergeResult{Merged: true, SHA: "def5678", Message: "Merged"},
		}
		executor := NewMergeExecutor(client)

		result, err := executor.Execute(context.Background(), &MergeRequest{
			PR:     testPR(),
			Method: "squash",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Merged {
			t.Error("expected merged to be true")
		}
		if result.SHA != "def5678" {
			t.Errorf("expected SHA def5678, got %s", result.SHA)
		}
		if !client.called {
			t.Error("expected client to be called")
		}
		if client.opts.Method != "squash" {
			t.Errorf("expected method squash, got %s", client.opts.Method)
		}
		if client.opts.CommitTitle != "Add auth middleware" {
			t.Errorf("expected commit title from PR, got %q", client.opts.CommitTitle)
		}
		if client.opts.CommitMessage != "This PR adds authentication middleware." {
			t.Errorf("expected commit message from PR body, got %q", client.opts.CommitMessage)
		}
	})

	t.Run("successful rebase merge sends empty title and body", func(t *testing.T) {
		client := &mockMergerClient{
			result: &github.MergeResult{Merged: true, SHA: "def5678"},
		}
		executor := NewMergeExecutor(client)

		result, err := executor.Execute(context.Background(), &MergeRequest{
			PR:     testPR(),
			Method: "rebase",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Merged {
			t.Error("expected merged to be true")
		}
		if client.opts.CommitTitle != "" {
			t.Errorf("expected empty commit title for rebase, got %q", client.opts.CommitTitle)
		}
		if client.opts.CommitMessage != "" {
			t.Errorf("expected empty commit message for rebase, got %q", client.opts.CommitMessage)
		}
	})

	t.Run("merge API error is wrapped", func(t *testing.T) {
		apiErr := &github.MergeConflictError{Err: fmt.Errorf("conflict")}
		client := &mockMergerClient{err: apiErr}
		executor := NewMergeExecutor(client)

		_, err := executor.Execute(context.Background(), &MergeRequest{
			PR:     testPR(),
			Method: "squash",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.As(err, &apiErr) {
			t.Errorf("expected MergeConflictError in chain, got %v", err)
		}
	})

	t.Run("merge method not allowed error", func(t *testing.T) {
		apiErr := &github.MergeMethodNotAllowedError{Method: "Squash"}
		client := &mockMergerClient{err: apiErr}
		executor := NewMergeExecutor(client)

		_, err := executor.Execute(context.Background(), &MergeRequest{
			PR:     testPR(),
			Method: "squash",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var methodErr *github.MergeMethodNotAllowedError
		if !errors.As(err, &methodErr) {
			t.Errorf("expected MergeMethodNotAllowedError in chain, got %v", err)
		}
	})

	t.Run("not mergeable error", func(t *testing.T) {
		apiErr := &github.NotMergeableError{Reason: "branch protection"}
		client := &mockMergerClient{err: apiErr}
		executor := NewMergeExecutor(client)

		_, err := executor.Execute(context.Background(), &MergeRequest{
			PR:     testPR(),
			Method: "squash",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var notMergeableErr *github.NotMergeableError
		if !errors.As(err, &notMergeableErr) {
			t.Errorf("expected NotMergeableError in chain, got %v", err)
		}
	})

	t.Run("rebase with edit flag warns but proceeds", func(t *testing.T) {
		client := &mockMergerClient{
			result: &github.MergeResult{Merged: true, SHA: "abc123"},
		}
		executor := NewMergeExecutor(client)

		result, err := executor.Execute(context.Background(), &MergeRequest{
			PR:     testPR(),
			Method: "rebase",
			Edit:   true,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Merged {
			t.Error("expected merged to be true")
		}
		if client.opts.CommitTitle != "" {
			t.Errorf("expected empty commit title for rebase, got %q", client.opts.CommitTitle)
		}
	})

	t.Run("PR with empty body uses empty commit message", func(t *testing.T) {
		client := &mockMergerClient{
			result: &github.MergeResult{Merged: true, SHA: "abc123"},
		}
		executor := NewMergeExecutor(client)

		pr := testPR()
		pr.Body = ""

		result, err := executor.Execute(context.Background(), &MergeRequest{
			PR:     pr,
			Method: "squash",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Merged {
			t.Error("expected merged to be true")
		}
		if client.opts.CommitMessage != "" {
			t.Errorf("expected empty commit message, got %q", client.opts.CommitMessage)
		}
	})
}

// --- prepareCommitMessage ---

func TestPrepareCommitMessage(t *testing.T) {
	executor := NewMergeExecutor(nil)

	t.Run("squash without edit returns PR title and body", func(t *testing.T) {
		pr := testPR()
		title, body, err := executor.prepareCommitMessage(pr, false, false)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if title != "Add auth middleware" {
			t.Errorf("expected PR title, got %q", title)
		}
		if body != "This PR adds authentication middleware." {
			t.Errorf("expected PR body, got %q", body)
		}
	})

	t.Run("rebase returns empty strings", func(t *testing.T) {
		title, body, err := executor.prepareCommitMessage(testPR(), false, true)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if title != "" {
			t.Errorf("expected empty title for rebase, got %q", title)
		}
		if body != "" {
			t.Errorf("expected empty body for rebase, got %q", body)
		}
	})

	t.Run("rebase with edit skips editor", func(t *testing.T) {
		title, body, err := executor.prepareCommitMessage(testPR(), true, true)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if title != "" {
			t.Errorf("expected empty title for rebase, got %q", title)
		}
		if body != "" {
			t.Errorf("expected empty body for rebase, got %q", body)
		}
	})
}

// --- parseCommitMessage ---

func TestParseCommitMessage(t *testing.T) {
	t.Run("title and body", func(t *testing.T) {
		title, body, err := parseCommitMessage("My Title\n\nSome body text\nMore body")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if title != "My Title" {
			t.Errorf("expected %q, got %q", "My Title", title)
		}
		if body != "Some body text\nMore body" {
			t.Errorf("expected %q, got %q", "Some body text\nMore body", body)
		}
	})

	t.Run("title only", func(t *testing.T) {
		title, body, err := parseCommitMessage("Just a title")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if title != "Just a title" {
			t.Errorf("expected %q, got %q", "Just a title", title)
		}
		if body != "" {
			t.Errorf("expected empty body, got %q", body)
		}
	})

	t.Run("leading blank lines are skipped", func(t *testing.T) {
		title, body, err := parseCommitMessage("\n\nActual Title\n\nBody here")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if title != "Actual Title" {
			t.Errorf("expected %q, got %q", "Actual Title", title)
		}
		if body != "Body here" {
			t.Errorf("expected %q, got %q", "Body here", body)
		}
	})

	t.Run("multiple blank lines between title and body", func(t *testing.T) {
		title, body, err := parseCommitMessage("Title\n\n\n\nBody")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if title != "Title" {
			t.Errorf("expected %q, got %q", "Title", title)
		}
		if body != "Body" {
			t.Errorf("expected %q, got %q", "Body", body)
		}
	})

	t.Run("empty content returns abort error", func(t *testing.T) {
		_, _, err := parseCommitMessage("")

		if !errors.Is(err, ErrMergeAborted) {
			t.Errorf("expected ErrMergeAborted, got %v", err)
		}
	})

	t.Run("whitespace-only content returns abort error", func(t *testing.T) {
		_, _, err := parseCommitMessage("   \n\n   ")

		if !errors.Is(err, ErrMergeAborted) {
			t.Errorf("expected ErrMergeAborted, got %v", err)
		}
	})

	t.Run("body with trailing whitespace is trimmed", func(t *testing.T) {
		title, body, err := parseCommitMessage("Title\n\nBody text\n\n\n")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if title != "Title" {
			t.Errorf("expected %q, got %q", "Title", title)
		}
		if body != "Body text" {
			t.Errorf("expected %q, got %q", "Body text", body)
		}
	})
}
