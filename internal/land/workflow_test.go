package land

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/github"
)

// --- workflow mocks ---

type mockWorkflowRepo struct {
	status          *git.WorkingDirectoryStatus
	statusErr       error
	headSHA         string
	headSHAErr      error
	currentBranch   string
	currentBranchErr error
	defaultBranch   string
	defaultBranchErr error
	checkoutErr     error
	pullErr         error
	branchSHA       string
	branchSHAErr    error
	deleteBranchErr error
}

func (m *mockWorkflowRepo) GetWorkingDirectoryStatus() (*git.WorkingDirectoryStatus, error) {
	return m.status, m.statusErr
}

func (m *mockWorkflowRepo) GetHeadSHA() (string, error) {
	return m.headSHA, m.headSHAErr
}

func (m *mockWorkflowRepo) GetCurrentBranch() (string, error) {
	return m.currentBranch, m.currentBranchErr
}

func (m *mockWorkflowRepo) GetDefaultBranch() (string, error) {
	return m.defaultBranch, m.defaultBranchErr
}

func (m *mockWorkflowRepo) CheckoutBranch(_ string) error {
	return m.checkoutErr
}

func (m *mockWorkflowRepo) PullOrigin(_ string) error {
	return m.pullErr
}

func (m *mockWorkflowRepo) GetBranchSHA(_ string) (string, error) {
	return m.branchSHA, m.branchSHAErr
}

func (m *mockWorkflowRepo) DeleteLocalBranch(_ string) error {
	return m.deleteBranchErr
}

type mockWorkflowClient struct {
	pr                *github.PullRequest
	findPRErr         error
	dependentPRs      []*github.PullRequest
	dependentPRsErr   error
	requiredChecks    []github.RequiredCheck
	requiredChecksErr error
	enrichErr         error
	mergeResult       *github.MergeResult
	mergeErr          error
	mergeCalled       bool
	mergeOpts         *github.MergeOptions
}

func (m *mockWorkflowClient) FindExistingPRForCurrentBranch(_ context.Context, _ string) (*github.PullRequest, error) {
	return m.pr, m.findPRErr
}

func (m *mockWorkflowClient) FindDependentPRsForCurrentBranch(_ context.Context, _ string) ([]*github.PullRequest, error) {
	return m.dependentPRs, m.dependentPRsErr
}

func (m *mockWorkflowClient) GetRequiredStatusChecksForCurrentRepo(_ context.Context, _ string) ([]github.RequiredCheck, error) {
	return m.requiredChecks, m.requiredChecksErr
}

func (m *mockWorkflowClient) EnrichPullRequest(_ context.Context, _, _ string, _ *github.PullRequest) error {
	return m.enrichErr
}

func (m *mockWorkflowClient) MergePullRequestForCurrentRepo(_ context.Context, _ int, opts *github.MergeOptions) (*github.MergeResult, error) {
	m.mergeCalled = true
	m.mergeOpts = opts
	return m.mergeResult, m.mergeErr
}

func defaultConfig() *config.Config {
	return &config.Config{
		Land: config.LandConfig{
			DefaultMergeMethod: "squash",
			DeleteLocalBranch:  true,
			RequireApproval:    config.ApprovalStrict,
			RequireCI:          config.CIModeRequired,
		},
		Output: config.OutputConfig{Color: false},
	}
}

func approvedPR() *github.PullRequest {
	return &github.PullRequest{
		Number: 42,
		Title:  "Add auth middleware",
		Body:   "Adds auth middleware for the API.",
		Head:   github.PRBranch{Ref: "feature/auth", SHA: "abc1234def5678"},
		Base:   github.PRBranch{Ref: "main"},
		Reviews: []github.PRReview{
			{User: github.PRUser{Login: "alice"}, State: "APPROVED"},
		},
		Checks: []github.PRCheck{
			{Name: "tests", Status: "completed", Conclusion: "success"},
		},
	}
}

func happyRepo() *mockWorkflowRepo {
	return &mockWorkflowRepo{
		status:        &git.WorkingDirectoryStatus{IsClean: true},
		headSHA:       "abc1234def5678",
		currentBranch: "feature/auth",
		defaultBranch: "main",
		branchSHA:     "abc1234def5678",
	}
}

func happyClient() *mockWorkflowClient {
	return &mockWorkflowClient{
		pr:             approvedPR(),
		requiredChecks: []github.RequiredCheck{{Context: "tests"}},
		mergeResult:    &github.MergeResult{Merged: true, SHA: "merged123sha456"},
	}
}

func newTestWorkflow(repo *mockWorkflowRepo, client *mockWorkflowClient, cfg *config.Config) *LandWorkflow {
	wf := NewLandWorkflow(repo, client, cfg, "owner", "repo")
	wf.output.writer = &bytes.Buffer{}
	return wf
}

func outputText(wf *LandWorkflow) string {
	return wf.output.writer.(*bytes.Buffer).String()
}

// --- happy path ---

func TestLandWorkflow_HappyPath(t *testing.T) {
	repo := happyRepo()
	client := happyClient()
	wf := newTestWorkflow(repo, client, defaultConfig())

	result, err := wf.Execute(context.Background(), &LandOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.PR.Number != 42 {
		t.Errorf("expected PR #42, got #%d", result.PR.Number)
	}
	if result.MergeMethod != "squash" {
		t.Errorf("expected squash, got %s", result.MergeMethod)
	}
	if result.MergeCommitSHA != "merged123sha456" {
		t.Errorf("expected merged123sha456, got %s", result.MergeCommitSHA)
	}
	if result.DefaultBranch != "main" {
		t.Errorf("expected main, got %s", result.DefaultBranch)
	}
	if result.DeletedBranch != "feature/auth" {
		t.Errorf("expected feature/auth, got %s", result.DeletedBranch)
	}
	if !client.mergeCalled {
		t.Error("expected merge to be called")
	}

	out := outputText(wf)
	if !strings.Contains(out, "Found PR #42") {
		t.Errorf("expected PR found output, got: %s", out)
	}
	if !strings.Contains(out, "Squash-merged into main") {
		t.Errorf("expected merge output, got: %s", out)
	}
}

// --- merge method resolution ---

func TestLandWorkflow_MergeMethodFlags(t *testing.T) {
	t.Run("--squash overrides config", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Land.DefaultMergeMethod = "rebase"
		client := happyClient()
		wf := newTestWorkflow(happyRepo(), client, cfg)

		_, err := wf.Execute(context.Background(), &LandOptions{Squash: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.mergeOpts.Method != "squash" {
			t.Errorf("expected squash, got %s", client.mergeOpts.Method)
		}
	})

	t.Run("--rebase overrides config", func(t *testing.T) {
		client := happyClient()
		wf := newTestWorkflow(happyRepo(), client, defaultConfig())

		_, err := wf.Execute(context.Background(), &LandOptions{Rebase: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.mergeOpts.Method != "rebase" {
			t.Errorf("expected rebase, got %s", client.mergeOpts.Method)
		}
	})

	t.Run("config default used when no flags", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Land.DefaultMergeMethod = "rebase"
		client := happyClient()
		wf := newTestWorkflow(happyRepo(), client, cfg)

		_, err := wf.Execute(context.Background(), &LandOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.mergeOpts.Method != "rebase" {
			t.Errorf("expected rebase, got %s", client.mergeOpts.Method)
		}
	})
}

// --- dirty working directory ---

func TestLandWorkflow_DirtyWorkingDir(t *testing.T) {
	repo := happyRepo()
	repo.status = &git.WorkingDirectoryStatus{IsClean: false}
	wf := newTestWorkflow(repo, happyClient(), defaultConfig())

	_, err := wf.Execute(context.Background(), &LandOptions{})
	if !errors.Is(err, ErrDirtyWorkingDir) {
		t.Errorf("expected ErrDirtyWorkingDir, got %v", err)
	}
	if !strings.Contains(outputText(wf), "uncommitted changes") {
		t.Error("expected dirty WD message in output")
	}
}

// --- on trunk ---

func TestLandWorkflow_OnTrunk(t *testing.T) {
	repo := happyRepo()
	repo.currentBranch = "main"
	wf := newTestWorkflow(repo, happyClient(), defaultConfig())

	_, err := wf.Execute(context.Background(), &LandOptions{})
	if !errors.Is(err, ErrOnTrunk) {
		t.Errorf("expected ErrOnTrunk, got %v", err)
	}
}

// --- no PR ---

func TestLandWorkflow_NoPR(t *testing.T) {
	client := happyClient()
	client.pr = nil
	wf := newTestWorkflow(happyRepo(), client, defaultConfig())

	_, err := wf.Execute(context.Background(), &LandOptions{})
	if !errors.Is(err, ErrNoPRFound) {
		t.Errorf("expected ErrNoPRFound, got %v", err)
	}
	if !strings.Contains(outputText(wf), "No open pull request") {
		t.Error("expected no PR message in output")
	}
}

// --- local HEAD mismatch ---

func TestLandWorkflow_LocalHeadMismatch(t *testing.T) {
	repo := happyRepo()
	repo.headSHA = "different1234567"
	wf := newTestWorkflow(repo, happyClient(), defaultConfig())

	_, err := wf.Execute(context.Background(), &LandOptions{})
	if !errors.Is(err, ErrLocalHeadMismatch) {
		t.Errorf("expected ErrLocalHeadMismatch, got %v", err)
	}
}

// --- approval checks ---

func TestLandWorkflow_ApprovalStrict_NoApproval(t *testing.T) {
	client := happyClient()
	client.pr.Reviews = nil
	wf := newTestWorkflow(happyRepo(), client, defaultConfig())

	_, err := wf.Execute(context.Background(), &LandOptions{})
	if !errors.Is(err, ErrApprovalFailed) {
		t.Errorf("expected ErrApprovalFailed, got %v", err)
	}
	out := outputText(wf)
	if !strings.Contains(out, "no reviews yet") {
		t.Errorf("expected no-reviews message, got: %s", out)
	}
}

func TestLandWorkflow_ApprovalStrict_ForceBypass(t *testing.T) {
	client := happyClient()
	client.pr.Reviews = nil
	wf := newTestWorkflow(happyRepo(), client, defaultConfig())

	result, err := wf.Execute(context.Background(), &LandOptions{Force: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeCommitSHA != "merged123sha456" {
		t.Errorf("expected merge to succeed with --force")
	}
}

func TestLandWorkflow_ApprovalPrompt_Confirmed(t *testing.T) {
	cfg := defaultConfig()
	cfg.Land.RequireApproval = config.ApprovalPrompt
	client := happyClient()
	client.pr.Reviews = nil
	wf := newTestWorkflow(happyRepo(), client, cfg)
	wf.stdin = strings.NewReader("y\n")
	wf.isTerminal = func() bool { return true }

	result, err := wf.Execute(context.Background(), &LandOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeCommitSHA != "merged123sha456" {
		t.Errorf("expected merge to succeed after confirmation")
	}
}

func TestLandWorkflow_ApprovalPrompt_Declined(t *testing.T) {
	cfg := defaultConfig()
	cfg.Land.RequireApproval = config.ApprovalPrompt
	client := happyClient()
	client.pr.Reviews = nil
	wf := newTestWorkflow(happyRepo(), client, cfg)
	wf.stdin = strings.NewReader("n\n")
	wf.isTerminal = func() bool { return true }

	_, err := wf.Execute(context.Background(), &LandOptions{})
	if !errors.Is(err, ErrMergeDeclined) {
		t.Errorf("expected ErrMergeDeclined, got %v", err)
	}
}

func TestLandWorkflow_ApprovalPrompt_NonTTY_AutoDeclines(t *testing.T) {
	cfg := defaultConfig()
	cfg.Land.RequireApproval = config.ApprovalPrompt
	client := happyClient()
	client.pr.Reviews = nil
	wf := newTestWorkflow(happyRepo(), client, cfg)
	wf.isTerminal = func() bool { return false }

	_, err := wf.Execute(context.Background(), &LandOptions{})
	if !errors.Is(err, ErrNonInteractive) {
		t.Errorf("expected ErrNonInteractive, got %v", err)
	}
	out := outputText(wf)
	if !strings.Contains(out, "Non-interactive") {
		t.Errorf("expected non-interactive message, got: %s", out)
	}
}

func TestLandWorkflow_ApprovalNone_SkipsCheck(t *testing.T) {
	cfg := defaultConfig()
	cfg.Land.RequireApproval = config.ApprovalNone
	client := happyClient()
	client.pr.Reviews = nil
	wf := newTestWorkflow(happyRepo(), client, cfg)

	result, err := wf.Execute(context.Background(), &LandOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeCommitSHA != "merged123sha456" {
		t.Errorf("expected merge to succeed with approval: none")
	}
}

// --- CI checks ---

func TestLandWorkflow_CIFailed(t *testing.T) {
	client := happyClient()
	client.pr.Checks = []github.PRCheck{
		{Name: "tests", Status: "completed", Conclusion: "failure"},
	}
	wf := newTestWorkflow(happyRepo(), client, defaultConfig())

	_, err := wf.Execute(context.Background(), &LandOptions{})
	if !errors.Is(err, ErrCIFailed) {
		t.Errorf("expected ErrCIFailed, got %v", err)
	}
	out := outputText(wf)
	if !strings.Contains(out, "tests") {
		t.Errorf("expected CI failure details, got: %s", out)
	}
}

func TestLandWorkflow_CIForceBypass(t *testing.T) {
	client := happyClient()
	client.pr.Checks = []github.PRCheck{
		{Name: "tests", Status: "completed", Conclusion: "failure"},
	}
	wf := newTestWorkflow(happyRepo(), client, defaultConfig())

	result, err := wf.Execute(context.Background(), &LandOptions{Force: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeCommitSHA != "merged123sha456" {
		t.Errorf("expected merge to succeed with --force")
	}
}

func TestLandWorkflow_CINone_SkipsCheck(t *testing.T) {
	cfg := defaultConfig()
	cfg.Land.RequireCI = config.CIModeNone
	client := happyClient()
	client.pr.Checks = []github.PRCheck{
		{Name: "tests", Status: "completed", Conclusion: "failure"},
	}
	wf := newTestWorkflow(happyRepo(), client, cfg)

	result, err := wf.Execute(context.Background(), &LandOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MergeCommitSHA != "merged123sha456" {
		t.Errorf("expected merge to succeed with CI: none")
	}
}

// --- dependent PRs ---

func TestLandWorkflow_DependentPRs_Informational(t *testing.T) {
	client := happyClient()
	client.dependentPRs = []*github.PullRequest{
		{Number: 99, Title: "Child PR"},
	}
	wf := newTestWorkflow(happyRepo(), client, defaultConfig())

	result, err := wf.Execute(context.Background(), &LandOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DependentPRCount != 1 {
		t.Errorf("expected 1 dependent PR, got %d", result.DependentPRCount)
	}
	out := outputText(wf)
	if !strings.Contains(out, "dependent PR") {
		t.Errorf("expected dependent PR warning, got: %s", out)
	}
}

// --- merge failure ---

func TestLandWorkflow_MergeFailure(t *testing.T) {
	client := happyClient()
	client.mergeResult = nil
	client.mergeErr = errors.New("409 conflict")
	wf := newTestWorkflow(happyRepo(), client, defaultConfig())

	_, err := wf.Execute(context.Background(), &LandOptions{})
	if err == nil {
		t.Fatal("expected error for merge failure")
	}
	out := outputText(wf)
	if !strings.Contains(out, "Merge failed") {
		t.Errorf("expected merge failure output, got: %s", out)
	}
}

// --- cleanup failure (non-fatal) ---

func TestLandWorkflow_CleanupFailure_NonFatal(t *testing.T) {
	repo := happyRepo()
	repo.pullErr = errors.New("network timeout")
	wf := newTestWorkflow(repo, happyClient(), defaultConfig())

	result, err := wf.Execute(context.Background(), &LandOptions{})
	if err != nil {
		t.Fatalf("cleanup failure should not fail the workflow: %v", err)
	}
	if result.MergeCommitSHA != "merged123sha456" {
		t.Error("expected merge to have succeeded")
	}
	if len(result.CleanupWarnings) == 0 {
		t.Error("expected cleanup warnings")
	}
}

// --- --no-delete ---

func TestLandWorkflow_NoDelete(t *testing.T) {
	wf := newTestWorkflow(happyRepo(), happyClient(), defaultConfig())

	result, err := wf.Execute(context.Background(), &LandOptions{NoDelete: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DeletedBranch != "" {
		t.Errorf("expected no branch deletion, got %s", result.DeletedBranch)
	}
}

func TestLandWorkflow_ConfigDeleteLocalBranchFalse(t *testing.T) {
	cfg := defaultConfig()
	cfg.Land.DeleteLocalBranch = false
	wf := newTestWorkflow(happyRepo(), happyClient(), cfg)

	result, err := wf.Execute(context.Background(), &LandOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DeletedBranch != "" {
		t.Errorf("expected no branch deletion, got %s", result.DeletedBranch)
	}
}

// --- enrich failure ---

func TestLandWorkflow_EnrichFailure(t *testing.T) {
	client := happyClient()
	client.enrichErr = errors.New("API rate limited")
	wf := newTestWorkflow(happyRepo(), client, defaultConfig())

	_, err := wf.Execute(context.Background(), &LandOptions{})
	if err == nil {
		t.Fatal("expected error for enrich failure")
	}
	if !strings.Contains(err.Error(), "enrich") {
		t.Errorf("expected enrich error, got: %v", err)
	}
}
