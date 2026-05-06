package land

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/serpro69/gh-arc/internal/github"
	"github.com/serpro69/gh-arc/internal/template"
)

var (
	ErrMergeAborted = errors.New("merge aborted: commit message empty or unchanged")
)

// MergerClient defines GitHub operations needed by the merge executor.
type MergerClient interface {
	MergePullRequestForCurrentRepo(ctx context.Context, number int, opts *github.MergeOptions) (*github.MergeResult, error)
}

// MergeRequest holds the parameters for a merge operation.
type MergeRequest struct {
	PR     *github.PullRequest
	Method string // "squash" or "rebase"
	Edit   bool
}

// MergeExecutor handles merge commit message preparation and API calls.
type MergeExecutor struct {
	client MergerClient
}

// NewMergeExecutor creates a new MergeExecutor.
func NewMergeExecutor(client MergerClient) *MergeExecutor {
	return &MergeExecutor{client: client}
}

// Execute prepares the commit message and merges the PR via the GitHub API.
func (m *MergeExecutor) Execute(ctx context.Context, req *MergeRequest) (*github.MergeResult, error) {
	title, body, err := m.prepareCommitMessage(req.PR, req.Edit, req.Method == "rebase")
	if err != nil {
		return nil, err
	}

	opts := &github.MergeOptions{
		Method:        req.Method,
		CommitTitle:   title,
		CommitMessage: body,
	}

	result, err := m.client.MergePullRequestForCurrentRepo(ctx, req.PR.Number, opts)
	if err != nil {
		return nil, fmt.Errorf("merge failed: %w", err)
	}

	return result, nil
}

// prepareCommitMessage extracts or edits the commit message for the merge.
// For rebase merges, GitHub controls commit messages so editing is skipped.
func (m *MergeExecutor) prepareCommitMessage(pr *github.PullRequest, edit bool, isRebase bool) (string, string, error) {
	if isRebase {
		if edit {
			fmt.Fprintln(os.Stderr, "⚠ --edit is ignored with --rebase, which preserves individual commits")
		}
		return "", "", nil
	}

	title := pr.Title
	body := pr.Body

	if !edit {
		return title, body, nil
	}

	return m.openEditor(title, body)
}

// openEditor writes the commit message to a temp file, opens $EDITOR,
// and parses the result. First non-empty line = title, rest = body.
func (m *MergeExecutor) openEditor(title, body string) (string, string, error) {
	editor, err := template.GetEditorCommand()
	if err != nil {
		return "", "", err
	}

	content := title + "\n\n" + body

	tmpFile, err := os.CreateTemp("", "gh-arc-merge-*.md")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return "", "", fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	originalContent := content

	// Split editor command to handle values like "code --wait" or "vim -u NONE"
	parts := strings.Fields(editor)
	cmd := exec.Command(parts[0], append(parts[1:], tmpPath)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", "", ErrMergeAborted
	}

	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read edited file: %w", err)
	}

	editedStr := strings.TrimSpace(string(edited))
	if editedStr == "" {
		return "", "", ErrMergeAborted
	}

	if editedStr == strings.TrimSpace(originalContent) {
		return "", "", ErrMergeAborted
	}

	return parseCommitMessage(editedStr)
}

// parseCommitMessage splits editor output into title (first non-empty line)
// and body (everything after the first blank line separator).
func parseCommitMessage(content string) (string, string, error) {
	lines := strings.Split(content, "\n")

	var title string
	bodyStart := len(lines)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			title = trimmed
			bodyStart = i + 1
			break
		}
	}

	if title == "" {
		return "", "", ErrMergeAborted
	}

	// Skip blank lines between title and body
	for bodyStart < len(lines) && strings.TrimSpace(lines[bodyStart]) == "" {
		bodyStart++
	}

	var body string
	if bodyStart < len(lines) {
		body = strings.TrimSpace(strings.Join(lines[bodyStart:], "\n"))
	}

	return title, body, nil
}
