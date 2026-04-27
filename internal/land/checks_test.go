package land

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/github"
)

// --- mocks ---

type mockCheckerRepo struct {
	status  *git.WorkingDirectoryStatus
	err     error
	headSHA string
	headErr error
}

func (m *mockCheckerRepo) GetWorkingDirectoryStatus() (*git.WorkingDirectoryStatus, error) {
	return m.status, m.err
}

func (m *mockCheckerRepo) GetHeadSHA() (string, error) {
	return m.headSHA, m.headErr
}

type mockCheckerClient struct {
	pr                *github.PullRequest
	findPRErr         error
	dependentPRs      []*github.PullRequest
	dependentPRsErr   error
	requiredChecks    []github.RequiredCheck
	requiredChecksErr error
}

func (m *mockCheckerClient) FindExistingPRForCurrentBranch(_ context.Context, _ string) (*github.PullRequest, error) {
	return m.pr, m.findPRErr
}

func (m *mockCheckerClient) FindDependentPRsForCurrentBranch(_ context.Context, _ string) ([]*github.PullRequest, error) {
	return m.dependentPRs, m.dependentPRsErr
}

func (m *mockCheckerClient) GetRequiredStatusChecksForCurrentRepo(_ context.Context, _ string) ([]github.RequiredCheck, error) {
	return m.requiredChecks, m.requiredChecksErr
}

func newChecker(repo CheckerRepo, client CheckerClient, cfg *config.LandConfig) *PreMergeChecker {
	return NewPreMergeChecker(repo, client, cfg)
}

func defaultLandConfig() *config.LandConfig {
	return &config.LandConfig{
		DefaultMergeMethod: "squash",
		DeleteLocalBranch:  true,
		RequireApproval:    config.ApprovalStrict,
		RequireCI:          config.CIModeRequired,
	}
}

// --- CheckCleanWorkingDir ---

func TestCheckCleanWorkingDir(t *testing.T) {
	t.Run("clean directory passes", func(t *testing.T) {
		repo := &mockCheckerRepo{status: &git.WorkingDirectoryStatus{IsClean: true}}
		checker := newChecker(repo, nil, defaultLandConfig())
		if err := checker.CheckCleanWorkingDir(); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("dirty directory fails", func(t *testing.T) {
		repo := &mockCheckerRepo{status: &git.WorkingDirectoryStatus{IsClean: false}}
		checker := newChecker(repo, nil, defaultLandConfig())
		err := checker.CheckCleanWorkingDir()
		if !errors.Is(err, ErrDirtyWorkingDir) {
			t.Errorf("expected ErrDirtyWorkingDir, got %v", err)
		}
	})

	t.Run("repo error propagates", func(t *testing.T) {
		repo := &mockCheckerRepo{err: errors.New("git broken")}
		checker := newChecker(repo, nil, defaultLandConfig())
		err := checker.CheckCleanWorkingDir()
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "git broken") {
			t.Errorf("expected wrapped error, got %v", err)
		}
	})
}

// --- CheckNotOnTrunk ---

func TestCheckNotOnTrunk(t *testing.T) {
	checker := newChecker(nil, nil, defaultLandConfig())

	t.Run("feature branch passes", func(t *testing.T) {
		if err := checker.CheckNotOnTrunk("feature/auth", "main"); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("on main fails", func(t *testing.T) {
		err := checker.CheckNotOnTrunk("main", "main")
		if !errors.Is(err, ErrOnTrunk) {
			t.Errorf("expected ErrOnTrunk, got %v", err)
		}
	})

	t.Run("on master fails", func(t *testing.T) {
		err := checker.CheckNotOnTrunk("master", "master")
		if !errors.Is(err, ErrOnTrunk) {
			t.Errorf("expected ErrOnTrunk, got %v", err)
		}
	})
}

// --- CheckPRExists ---

func TestCheckPRExists(t *testing.T) {
	ctx := context.Background()

	t.Run("PR found", func(t *testing.T) {
		pr := &github.PullRequest{Number: 42, Title: "Add auth"}
		client := &mockCheckerClient{pr: pr}
		checker := newChecker(nil, client, defaultLandConfig())

		got, err := checker.CheckPRExists(ctx, "feature/auth")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Number != 42 {
			t.Errorf("expected PR #42, got #%d", got.Number)
		}
	})

	t.Run("no PR found", func(t *testing.T) {
		client := &mockCheckerClient{pr: nil}
		checker := newChecker(nil, client, defaultLandConfig())

		_, err := checker.CheckPRExists(ctx, "feature/auth")
		if !errors.Is(err, ErrNoPRFound) {
			t.Errorf("expected ErrNoPRFound, got %v", err)
		}
	})

	t.Run("API error propagates", func(t *testing.T) {
		client := &mockCheckerClient{findPRErr: errors.New("network error")}
		checker := newChecker(nil, client, defaultLandConfig())

		_, err := checker.CheckPRExists(ctx, "feature/auth")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// --- CheckLocalHeadMatchesPR ---

func TestCheckLocalHeadMatchesPR(t *testing.T) {
	sha := "abc1234567890def"

	t.Run("matching HEAD passes", func(t *testing.T) {
		repo := &mockCheckerRepo{headSHA: sha}
		checker := newChecker(repo, nil, defaultLandConfig())
		pr := &github.PullRequest{Head: github.PRBranch{SHA: sha}}
		if err := checker.CheckLocalHeadMatchesPR(pr); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("mismatched HEAD fails", func(t *testing.T) {
		repo := &mockCheckerRepo{headSHA: sha}
		checker := newChecker(repo, nil, defaultLandConfig())
		pr := &github.PullRequest{Head: github.PRBranch{SHA: "ffff000111222333"}}
		err := checker.CheckLocalHeadMatchesPR(pr)
		if !errors.Is(err, ErrLocalHeadMismatch) {
			t.Errorf("expected ErrLocalHeadMismatch, got %v", err)
		}
	})

	t.Run("repo error propagates", func(t *testing.T) {
		repo := &mockCheckerRepo{headErr: errors.New("git broken")}
		checker := newChecker(repo, nil, defaultLandConfig())
		pr := &github.PullRequest{Head: github.PRBranch{SHA: sha}}
		err := checker.CheckLocalHeadMatchesPR(pr)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "git broken") {
			t.Errorf("expected wrapped error, got %v", err)
		}
	})
}

// --- CheckApproval ---

func TestCheckApproval(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	approvedPR := &github.PullRequest{
		Reviews: []github.PRReview{
			{User: github.PRUser{Login: "alice"}, State: "APPROVED", SubmittedAt: now},
			{User: github.PRUser{Login: "bob"}, State: "APPROVED", SubmittedAt: now},
		},
	}

	noReviewsPR := &github.PullRequest{}

	changesRequestedPR := &github.PullRequest{
		Reviews: []github.PRReview{
			{User: github.PRUser{Login: "alice"}, State: "APPROVED", SubmittedAt: now},
			{User: github.PRUser{Login: "bob"}, State: "CHANGES_REQUESTED", SubmittedAt: now},
		},
	}

	// Superseded changes request (user approved after requesting changes)
	supersededPR := &github.PullRequest{
		Reviews: []github.PRReview{
			{User: github.PRUser{Login: "alice"}, State: "CHANGES_REQUESTED", SubmittedAt: now.Add(-time.Hour)},
			{User: github.PRUser{Login: "alice"}, State: "APPROVED", SubmittedAt: now},
		},
	}

	t.Run("strict/approved passes", func(t *testing.T) {
		cfg := defaultLandConfig()
		checker := newChecker(nil, nil, cfg)
		result, err := checker.CheckApproval(ctx, approvedPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("expected passed")
		}
		assertMessageContains(t, result, "Approved by")
	})

	t.Run("strict/no reviews fails", func(t *testing.T) {
		cfg := defaultLandConfig()
		checker := newChecker(nil, nil, cfg)
		result, err := checker.CheckApproval(ctx, noReviewsPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Passed {
			t.Error("expected not passed")
		}
		if result.NeedsConfirmation {
			t.Error("strict mode should not need confirmation")
		}
		assertMessageContains(t, result, "no reviews yet")
	})

	t.Run("strict/changes requested fails", func(t *testing.T) {
		cfg := defaultLandConfig()
		checker := newChecker(nil, nil, cfg)
		result, err := checker.CheckApproval(ctx, changesRequestedPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Passed {
			t.Error("expected not passed")
		}
		assertMessageContains(t, result, "change requests")
		assertMessageContains(t, result, "@bob")
	})

	t.Run("strict/force bypasses", func(t *testing.T) {
		cfg := defaultLandConfig()
		checker := newChecker(nil, nil, cfg)
		result, err := checker.CheckApproval(ctx, noReviewsPR, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("force should bypass")
		}
		assertMessageContains(t, result, "--force")
	})

	t.Run("prompt/no reviews needs confirmation", func(t *testing.T) {
		cfg := defaultLandConfig()
		cfg.RequireApproval = config.ApprovalPrompt
		checker := newChecker(nil, nil, cfg)
		result, err := checker.CheckApproval(ctx, noReviewsPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Passed {
			t.Error("expected not passed")
		}
		if !result.NeedsConfirmation {
			t.Error("prompt mode should need confirmation")
		}
	})

	t.Run("prompt/force bypasses without confirmation", func(t *testing.T) {
		cfg := defaultLandConfig()
		cfg.RequireApproval = config.ApprovalPrompt
		checker := newChecker(nil, nil, cfg)
		result, err := checker.CheckApproval(ctx, noReviewsPR, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("force should bypass")
		}
		if result.NeedsConfirmation {
			t.Error("force should not need confirmation")
		}
	})

	t.Run("none skips entirely", func(t *testing.T) {
		cfg := defaultLandConfig()
		cfg.RequireApproval = config.ApprovalNone
		checker := newChecker(nil, nil, cfg)
		result, err := checker.CheckApproval(ctx, noReviewsPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("none mode should pass")
		}
		assertMessageContains(t, result, "skipped")
	})

	t.Run("superseded changes request treated as approved", func(t *testing.T) {
		cfg := defaultLandConfig()
		checker := newChecker(nil, nil, cfg)
		result, err := checker.CheckApproval(ctx, supersededPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("superseded changes request should pass")
		}
		assertMessageContains(t, result, "Approved by")
	})
}

// --- CheckCI ---

func TestCheckCI(t *testing.T) {
	ctx := context.Background()

	allPassingPR := &github.PullRequest{
		Base: github.PRBranch{Ref: "main"},
		Checks: []github.PRCheck{
			{Name: "tests", Status: "completed", Conclusion: "success"},
			{Name: "lint", Status: "completed", Conclusion: "success"},
			{Name: "build", Status: "completed", Conclusion: "success"},
		},
	}

	failingPR := &github.PullRequest{
		Base: github.PRBranch{Ref: "main"},
		Checks: []github.PRCheck{
			{Name: "tests", Status: "completed", Conclusion: "failure"},
			{Name: "lint", Status: "in_progress", Conclusion: ""},
			{Name: "build", Status: "completed", Conclusion: "success"},
		},
	}

	noChecksPR := &github.PullRequest{
		Base: github.PRBranch{Ref: "main"},
	}

	skippedAndNeutralPR := &github.PullRequest{
		Base: github.PRBranch{Ref: "main"},
		Checks: []github.PRCheck{
			{Name: "tests", Status: "completed", Conclusion: "success"},
			{Name: "optional", Status: "completed", Conclusion: "skipped"},
			{Name: "info", Status: "completed", Conclusion: "neutral"},
		},
	}

	t.Run("all mode/all passing", func(t *testing.T) {
		cfg := defaultLandConfig()
		cfg.RequireCI = config.CIModeAll
		checker := newChecker(nil, &mockCheckerClient{}, cfg)
		result, err := checker.CheckCI(ctx, allPassingPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("expected passed")
		}
		assertMessageContains(t, result, "All CI checks passed (3/3)")
	})

	t.Run("all mode/failures block", func(t *testing.T) {
		cfg := defaultLandConfig()
		cfg.RequireCI = config.CIModeAll
		checker := newChecker(nil, &mockCheckerClient{}, cfg)
		result, err := checker.CheckCI(ctx, failingPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Passed {
			t.Error("expected not passed")
		}
		assertMessageContains(t, result, "'tests' failed")
		assertMessageContains(t, result, "'lint' in progress")
		assertMessageContains(t, result, "(1/3 passed)")
	})

	t.Run("all mode/force bypasses", func(t *testing.T) {
		cfg := defaultLandConfig()
		cfg.RequireCI = config.CIModeAll
		checker := newChecker(nil, &mockCheckerClient{}, cfg)
		result, err := checker.CheckCI(ctx, failingPR, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("force should bypass")
		}
		assertMessageContains(t, result, "--force")
	})

	t.Run("all mode/skipped and neutral count as pass", func(t *testing.T) {
		cfg := defaultLandConfig()
		cfg.RequireCI = config.CIModeAll
		checker := newChecker(nil, &mockCheckerClient{}, cfg)
		result, err := checker.CheckCI(ctx, skippedAndNeutralPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("skipped/neutral should count as passed")
		}
	})

	t.Run("required mode/all required pass", func(t *testing.T) {
		cfg := defaultLandConfig()
		client := &mockCheckerClient{requiredChecks: requiredChecks("tests", "build")}
		checker := newChecker(nil, client, cfg)
		result, err := checker.CheckCI(ctx, allPassingPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("expected passed")
		}
		assertMessageContains(t, result, "All CI checks passed (2/2)")
	})

	t.Run("required mode/required check fails", func(t *testing.T) {
		cfg := defaultLandConfig()
		client := &mockCheckerClient{requiredChecks: requiredChecks("tests")}
		checker := newChecker(nil, client, cfg)
		result, err := checker.CheckCI(ctx, failingPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Passed {
			t.Error("expected not passed")
		}
		assertMessageContains(t, result, "'tests' failed")
	})

	t.Run("required mode/non-required failure ignored", func(t *testing.T) {
		cfg := defaultLandConfig()
		client := &mockCheckerClient{requiredChecks: requiredChecks("build")}
		checker := newChecker(nil, client, cfg)
		result, err := checker.CheckCI(ctx, failingPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("non-required failure should not block")
		}
	})

	t.Run("required mode/no required checks configured", func(t *testing.T) {
		cfg := defaultLandConfig()
		client := &mockCheckerClient{requiredChecks: []github.RequiredCheck{}}
		checker := newChecker(nil, client, cfg)
		result, err := checker.CheckCI(ctx, allPassingPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("no required checks means pass")
		}
		assertMessageContains(t, result, "No required CI checks configured")
	})

	t.Run("required mode/missing required check treated as pending", func(t *testing.T) {
		cfg := defaultLandConfig()
		client := &mockCheckerClient{requiredChecks: requiredChecks("tests", "deploy")}
		checker := newChecker(nil, client, cfg)
		result, err := checker.CheckCI(ctx, allPassingPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Passed {
			t.Error("missing required check should block")
		}
		assertMessageContains(t, result, "'deploy' in progress")
	})

	t.Run("required mode/API error propagates", func(t *testing.T) {
		cfg := defaultLandConfig()
		client := &mockCheckerClient{requiredChecksErr: errors.New("api error")}
		checker := newChecker(nil, client, cfg)
		_, err := checker.CheckCI(ctx, allPassingPR, false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("required mode/403 permission denied blocks", func(t *testing.T) {
		cfg := defaultLandConfig()
		client := &mockCheckerClient{
			requiredChecksErr: fmt.Errorf("%w: branch \"main\"", github.ErrBranchProtectionPermissionDenied),
		}
		checker := newChecker(nil, client, cfg)
		result, err := checker.CheckCI(ctx, allPassingPR, false)
		if err != nil {
			t.Fatalf("expected CheckResult not error, got %v", err)
		}
		if result.Passed {
			t.Error("403 should block when not forced")
		}
		assertMessageContains(t, result, "insufficient permissions")
		assertMessageContains(t, result, "requireCI")
	})

	t.Run("required mode/403 permission denied bypassed with force", func(t *testing.T) {
		cfg := defaultLandConfig()
		client := &mockCheckerClient{
			requiredChecksErr: fmt.Errorf("%w: branch \"main\"", github.ErrBranchProtectionPermissionDenied),
		}
		checker := newChecker(nil, client, cfg)
		result, err := checker.CheckCI(ctx, allPassingPR, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("force should bypass 403")
		}
		assertMessageContains(t, result, "--force")
	})

	t.Run("none skips entirely", func(t *testing.T) {
		cfg := defaultLandConfig()
		cfg.RequireCI = config.CIModeNone
		checker := newChecker(nil, &mockCheckerClient{}, cfg)
		result, err := checker.CheckCI(ctx, noChecksPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("none mode should pass")
		}
		assertMessageContains(t, result, "skipped")
	})

	t.Run("all mode/no checks found with force", func(t *testing.T) {
		cfg := defaultLandConfig()
		cfg.RequireCI = config.CIModeAll
		checker := newChecker(nil, &mockCheckerClient{}, cfg)
		result, err := checker.CheckCI(ctx, noChecksPR, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("force should bypass")
		}
	})

	t.Run("all mode/no checks found without force", func(t *testing.T) {
		cfg := defaultLandConfig()
		cfg.RequireCI = config.CIModeAll
		checker := newChecker(nil, &mockCheckerClient{}, cfg)
		result, err := checker.CheckCI(ctx, noChecksPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("no checks with no required checks should pass")
		}
	})

	t.Run("all mode/retried check uses latest result", func(t *testing.T) {
		retriedPR := &github.PullRequest{
			Base: github.PRBranch{Ref: "main"},
			Checks: []github.PRCheck{
				{Name: "tests", Status: "completed", Conclusion: "failure", StartedAt: time.Now().Add(-time.Hour)},
				{Name: "tests", Status: "completed", Conclusion: "success", StartedAt: time.Now()},
				{Name: "lint", Status: "completed", Conclusion: "success", StartedAt: time.Now()},
			},
		}
		cfg := defaultLandConfig()
		cfg.RequireCI = config.CIModeAll
		checker := newChecker(nil, &mockCheckerClient{}, cfg)
		result, err := checker.CheckCI(ctx, retriedPR, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("retried check should use latest (successful) result")
		}
		assertMessageContains(t, result, "All CI checks passed (2/2)")
	})
}

// --- CheckDependentPRs ---

func TestCheckDependentPRs(t *testing.T) {
	ctx := context.Background()

	t.Run("no dependent PRs", func(t *testing.T) {
		client := &mockCheckerClient{}
		checker := newChecker(nil, client, defaultLandConfig())
		deps, err := checker.CheckDependentPRs(ctx, "feature/auth")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(deps) != 0 {
			t.Errorf("expected 0 deps, got %d", len(deps))
		}
	})

	t.Run("found dependent PRs", func(t *testing.T) {
		client := &mockCheckerClient{
			dependentPRs: []*github.PullRequest{
				{Number: 43, Title: "Auth tests"},
				{Number: 44, Title: "Auth docs"},
			},
		}
		checker := newChecker(nil, client, defaultLandConfig())
		deps, err := checker.CheckDependentPRs(ctx, "feature/auth")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(deps) != 2 {
			t.Errorf("expected 2 deps, got %d", len(deps))
		}
	})

	t.Run("API error propagates", func(t *testing.T) {
		client := &mockCheckerClient{dependentPRsErr: errors.New("network error")}
		checker := newChecker(nil, client, defaultLandConfig())
		_, err := checker.CheckDependentPRs(ctx, "feature/auth")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// --- evaluateReviews ---

func TestEvaluateReviews(t *testing.T) {
	now := time.Now()

	t.Run("empty reviews", func(t *testing.T) {
		approvers, changesRequested := evaluateReviews(nil)
		if len(approvers) != 0 || len(changesRequested) != 0 {
			t.Error("expected empty results for nil reviews")
		}
	})

	t.Run("only latest review per user counts", func(t *testing.T) {
		reviews := []github.PRReview{
			{User: github.PRUser{Login: "alice"}, State: "CHANGES_REQUESTED", SubmittedAt: now.Add(-time.Hour)},
			{User: github.PRUser{Login: "alice"}, State: "APPROVED", SubmittedAt: now},
		}
		approvers, changesRequested := evaluateReviews(reviews)
		if len(approvers) != 1 || approvers[0] != "@alice" {
			t.Errorf("expected [@alice], got %v", approvers)
		}
		if len(changesRequested) != 0 {
			t.Errorf("expected no changes requested, got %v", changesRequested)
		}
	})

	t.Run("COMMENTED reviews ignored", func(t *testing.T) {
		reviews := []github.PRReview{
			{User: github.PRUser{Login: "alice"}, State: "COMMENTED", SubmittedAt: now},
		}
		approvers, changesRequested := evaluateReviews(reviews)
		if len(approvers) != 0 || len(changesRequested) != 0 {
			t.Error("COMMENTED should be ignored")
		}
	})

	t.Run("COMMENTED after CHANGES_REQUESTED preserves block", func(t *testing.T) {
		reviews := []github.PRReview{
			{User: github.PRUser{Login: "alice"}, State: "CHANGES_REQUESTED", SubmittedAt: now.Add(-time.Hour)},
			{User: github.PRUser{Login: "alice"}, State: "COMMENTED", SubmittedAt: now},
		}
		approvers, changesRequested := evaluateReviews(reviews)
		if len(approvers) != 0 {
			t.Errorf("expected no approvers, got %v", approvers)
		}
		if len(changesRequested) != 1 || changesRequested[0] != "@alice" {
			t.Errorf("expected [@alice] in changes requested, got %v", changesRequested)
		}
	})

	t.Run("DISMISSED reviews ignored", func(t *testing.T) {
		reviews := []github.PRReview{
			{User: github.PRUser{Login: "alice"}, State: "CHANGES_REQUESTED", SubmittedAt: now.Add(-time.Hour)},
			{User: github.PRUser{Login: "alice"}, State: "DISMISSED", SubmittedAt: now},
		}
		approvers, changesRequested := evaluateReviews(reviews)
		if len(approvers) != 0 {
			t.Errorf("expected no approvers, got %v", approvers)
		}
		if len(changesRequested) != 1 || changesRequested[0] != "@alice" {
			t.Errorf("DISMISSED should not clear CHANGES_REQUESTED, got %v", changesRequested)
		}
	})

	t.Run("deterministic ordering", func(t *testing.T) {
		reviews := []github.PRReview{
			{User: github.PRUser{Login: "charlie"}, State: "APPROVED", SubmittedAt: now},
			{User: github.PRUser{Login: "alice"}, State: "APPROVED", SubmittedAt: now},
			{User: github.PRUser{Login: "bob"}, State: "APPROVED", SubmittedAt: now},
		}
		approvers, _ := evaluateReviews(reviews)
		expected := []string{"@alice", "@bob", "@charlie"}
		if len(approvers) != 3 {
			t.Fatalf("expected 3 approvers, got %d", len(approvers))
		}
		for i, want := range expected {
			if approvers[i] != want {
				t.Errorf("approvers[%d] = %q, want %q", i, approvers[i], want)
			}
		}
	})
}

// --- CheckCI app-aware matching ---

func TestCheckCI_AppAwareMatching(t *testing.T) {
	ctx := context.Background()
	appID42 := 42
	appID99 := 99

	t.Run("required check with app_id matches correct app", func(t *testing.T) {
		pr := &github.PullRequest{
			Base: github.PRBranch{Ref: "main"},
			Checks: []github.PRCheck{
				{Name: "ci/tests", Status: "completed", Conclusion: "success", App: &struct {
					ID int `json:"id"`
				}{ID: 42}},
			},
		}
		cfg := defaultLandConfig()
		client := &mockCheckerClient{requiredChecks: []github.RequiredCheck{
			{Context: "ci/tests", AppID: &appID42},
		}}
		checker := newChecker(nil, client, cfg)
		result, err := checker.CheckCI(ctx, pr, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("expected passed: correct app satisfies the check")
		}
	})

	t.Run("required check with app_id rejects wrong app", func(t *testing.T) {
		pr := &github.PullRequest{
			Base: github.PRBranch{Ref: "main"},
			Checks: []github.PRCheck{
				{Name: "ci/tests", Status: "completed", Conclusion: "success", App: &struct {
					ID int `json:"id"`
				}{ID: 99}},
			},
		}
		cfg := defaultLandConfig()
		client := &mockCheckerClient{requiredChecks: []github.RequiredCheck{
			{Context: "ci/tests", AppID: &appID42},
		}}
		checker := newChecker(nil, client, cfg)
		result, err := checker.CheckCI(ctx, pr, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Passed {
			t.Error("expected not passed: wrong app should not satisfy the check")
		}
	})

	t.Run("required check without app_id matches any app", func(t *testing.T) {
		pr := &github.PullRequest{
			Base: github.PRBranch{Ref: "main"},
			Checks: []github.PRCheck{
				{Name: "ci/tests", Status: "completed", Conclusion: "success", App: &struct {
					ID int `json:"id"`
				}{ID: 99}},
			},
		}
		cfg := defaultLandConfig()
		client := &mockCheckerClient{requiredChecks: []github.RequiredCheck{
			{Context: "ci/tests"},
		}}
		checker := newChecker(nil, client, cfg)
		result, err := checker.CheckCI(ctx, pr, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Passed {
			t.Error("expected passed: nil app_id should match any app")
		}
	})

	t.Run("same-named checks from different apps matched independently", func(t *testing.T) {
		pr := &github.PullRequest{
			Base: github.PRBranch{Ref: "main"},
			Checks: []github.PRCheck{
				{Name: "ci/tests", Status: "completed", Conclusion: "success", App: &struct {
					ID int `json:"id"`
				}{ID: 42}},
				{Name: "ci/tests", Status: "completed", Conclusion: "failure", App: &struct {
					ID int `json:"id"`
				}{ID: 99}},
			},
		}
		cfg := defaultLandConfig()
		client := &mockCheckerClient{requiredChecks: []github.RequiredCheck{
			{Context: "ci/tests", AppID: &appID42},
			{Context: "ci/tests", AppID: &appID99},
		}}
		checker := newChecker(nil, client, cfg)
		result, err := checker.CheckCI(ctx, pr, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Passed {
			t.Error("expected not passed: second app's check failed")
		}
	})
}

// --- helpers ---

func requiredChecks(names ...string) []github.RequiredCheck {
	checks := make([]github.RequiredCheck, len(names))
	for i, n := range names {
		checks[i] = github.RequiredCheck{Context: n}
	}
	return checks
}

func assertMessageContains(t *testing.T, result *CheckResult, substr string) {
	t.Helper()
	for _, msg := range result.Messages {
		if strings.Contains(msg, substr) {
			return
		}
	}
	t.Errorf("expected message containing %q, got %v", substr, result.Messages)
}
