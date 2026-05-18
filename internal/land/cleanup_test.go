package land

import (
	"errors"
	"testing"
)

type mockCleanupRepo struct {
	checkoutErr       error
	pullErr           error
	pruneErr          error
	branchSHA         string
	branchSHAErr      error
	deleteBranchErr   error
	checkoutCalls     []string
	pullCalls         []string
	pruneCalls        int
	branchSHACalls    []string
	deleteBranchCalls []string
}

func (m *mockCleanupRepo) CheckoutBranch(branch string) error {
	m.checkoutCalls = append(m.checkoutCalls, branch)
	return m.checkoutErr
}

func (m *mockCleanupRepo) PullOrigin(branch string) error {
	m.pullCalls = append(m.pullCalls, branch)
	return m.pullErr
}

func (m *mockCleanupRepo) PruneRemoteRefs() error {
	m.pruneCalls++
	return m.pruneErr
}

func (m *mockCleanupRepo) GetBranchSHA(branch string) (string, error) {
	m.branchSHACalls = append(m.branchSHACalls, branch)
	return m.branchSHA, m.branchSHAErr
}

func (m *mockCleanupRepo) DeleteLocalBranch(branch string) error {
	m.deleteBranchCalls = append(m.deleteBranchCalls, branch)
	return m.deleteBranchErr
}

func TestPostMergeCleanup_Execute_HappyPath(t *testing.T) {
	mock := &mockCleanupRepo{branchSHA: "abc1234def5678"}
	cleanup := NewPostMergeCleanup(mock)

	result, err := cleanup.Execute("main", "feature/auth", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.CheckedOut {
		t.Error("expected CheckedOut to be true")
	}
	if !result.Pulled {
		t.Error("expected Pulled to be true")
	}
	if !result.RemotePruned {
		t.Error("expected RemotePruned to be true")
	}
	if !result.BranchDeleted {
		t.Error("expected BranchDeleted to be true")
	}
	if result.DeletedBranchSHA != "abc1234def5678" {
		t.Errorf("expected DeletedBranchSHA abc1234def5678, got %s", result.DeletedBranchSHA)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings, got %v", result.Warnings)
	}

	if len(mock.checkoutCalls) != 1 || mock.checkoutCalls[0] != "main" {
		t.Errorf("expected checkout called with 'main', got %v", mock.checkoutCalls)
	}
	if len(mock.pullCalls) != 1 || mock.pullCalls[0] != "main" {
		t.Errorf("expected pull called with 'main', got %v", mock.pullCalls)
	}
	if mock.pruneCalls != 1 {
		t.Errorf("expected prune called once, got %d", mock.pruneCalls)
	}
	if len(mock.deleteBranchCalls) != 1 || mock.deleteBranchCalls[0] != "feature/auth" {
		t.Errorf("expected delete called with 'feature/auth', got %v", mock.deleteBranchCalls)
	}
}

func TestPostMergeCleanup_Execute_NoDelete(t *testing.T) {
	mock := &mockCleanupRepo{branchSHA: "abc1234"}
	cleanup := NewPostMergeCleanup(mock)

	result, err := cleanup.Execute("main", "feature/auth", true)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.CheckedOut {
		t.Error("expected CheckedOut to be true")
	}
	if !result.Pulled {
		t.Error("expected Pulled to be true")
	}
	if !result.RemotePruned {
		t.Error("expected RemotePruned to be true even when noDelete is true")
	}
	if result.BranchDeleted {
		t.Error("expected BranchDeleted to be false when noDelete is true")
	}
	if result.DeletedBranchSHA != "" {
		t.Errorf("expected empty DeletedBranchSHA, got %s", result.DeletedBranchSHA)
	}
	if mock.pruneCalls != 1 {
		t.Errorf("expected prune called once even with noDelete, got %d", mock.pruneCalls)
	}
	if len(mock.deleteBranchCalls) != 0 {
		t.Error("expected delete not to be called when noDelete is true")
	}
	if len(mock.branchSHACalls) != 0 {
		t.Error("expected GetBranchSHA not to be called when noDelete is true")
	}
}

func TestPostMergeCleanup_Execute_CheckoutFailure(t *testing.T) {
	mock := &mockCleanupRepo{
		checkoutErr: errors.New("checkout failed"),
		branchSHA:   "abc1234",
	}
	cleanup := NewPostMergeCleanup(mock)

	result, err := cleanup.Execute("main", "feature/auth", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CheckedOut {
		t.Error("expected CheckedOut to be false")
	}
	if result.Pulled {
		t.Error("expected Pulled to be false after checkout failure")
	}
	if result.BranchDeleted {
		t.Error("expected BranchDeleted to be false after checkout failure")
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}

	// Remaining steps should not run
	if len(mock.pullCalls) != 0 {
		t.Error("pull should not be called after checkout failure")
	}
	if mock.pruneCalls != 0 {
		t.Error("prune should not be called after checkout failure")
	}
	if len(mock.deleteBranchCalls) != 0 {
		t.Error("delete should not be called after checkout failure")
	}
}

func TestPostMergeCleanup_Execute_PullFailure(t *testing.T) {
	mock := &mockCleanupRepo{
		pullErr:   errors.New("pull failed"),
		branchSHA: "abc1234",
	}
	cleanup := NewPostMergeCleanup(mock)

	result, err := cleanup.Execute("main", "feature/auth", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.CheckedOut {
		t.Error("expected CheckedOut to be true")
	}
	if result.Pulled {
		t.Error("expected Pulled to be false")
	}
	if !result.BranchDeleted {
		t.Error("expected BranchDeleted to be true — pull failure should not block deletion")
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

func TestPostMergeCleanup_Execute_PruneFailure(t *testing.T) {
	mock := &mockCleanupRepo{
		pruneErr:  errors.New("network timeout"),
		branchSHA: "abc1234",
	}
	cleanup := NewPostMergeCleanup(mock)

	result, err := cleanup.Execute("main", "feature/auth", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.CheckedOut {
		t.Error("expected CheckedOut to be true")
	}
	if !result.Pulled {
		t.Error("expected Pulled to be true")
	}
	if result.RemotePruned {
		t.Error("expected RemotePruned to be false")
	}
	if !result.BranchDeleted {
		t.Error("expected BranchDeleted to be true — prune failure should not block deletion")
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

func TestPostMergeCleanup_Execute_DeleteFailure(t *testing.T) {
	mock := &mockCleanupRepo{
		branchSHA:       "abc1234",
		deleteBranchErr: errors.New("branch in use"),
	}
	cleanup := NewPostMergeCleanup(mock)

	result, err := cleanup.Execute("main", "feature/auth", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.CheckedOut {
		t.Error("expected CheckedOut to be true")
	}
	if !result.Pulled {
		t.Error("expected Pulled to be true")
	}
	if result.BranchDeleted {
		t.Error("expected BranchDeleted to be false")
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}
}

func TestPostMergeCleanup_Execute_GetBranchSHAFailure(t *testing.T) {
	mock := &mockCleanupRepo{
		branchSHAErr: errors.New("rev-parse failed"),
	}
	cleanup := NewPostMergeCleanup(mock)

	result, err := cleanup.Execute("main", "feature/auth", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BranchDeleted {
		t.Error("expected BranchDeleted to be false when SHA capture fails")
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}
	// Delete should not be called if we can't capture the SHA
	if len(mock.deleteBranchCalls) != 0 {
		t.Error("delete should not be called when SHA capture fails")
	}
}

func TestPostMergeCleanup_Execute_AllFailures(t *testing.T) {
	mock := &mockCleanupRepo{
		checkoutErr: errors.New("checkout failed"),
	}
	cleanup := NewPostMergeCleanup(mock)

	result, err := cleanup.Execute("main", "feature/auth", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CheckedOut || result.Pulled || result.RemotePruned || result.BranchDeleted {
		t.Error("nothing should succeed when checkout fails")
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning (checkout only, rest skipped), got %d", len(result.Warnings))
	}
}

func TestPostMergeCleanup_Execute_PullAndDeleteBothFail(t *testing.T) {
	mock := &mockCleanupRepo{
		pullErr:         errors.New("network error"),
		deleteBranchErr: errors.New("branch locked"),
		branchSHA:       "abc1234",
	}
	cleanup := NewPostMergeCleanup(mock)

	result, err := cleanup.Execute("main", "feature/auth", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.CheckedOut {
		t.Error("expected CheckedOut to be true")
	}
	if result.Pulled {
		t.Error("expected Pulled to be false")
	}
	if result.BranchDeleted {
		t.Error("expected BranchDeleted to be false")
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

func TestPostMergeCleanup_Execute_EmptyDefaultBranch(t *testing.T) {
	cleanup := NewPostMergeCleanup(&mockCleanupRepo{})

	_, err := cleanup.Execute("", "feature/auth", false)

	if err == nil {
		t.Fatal("expected error for empty defaultBranch")
	}
	if !errors.Is(err, ErrCleanupInvalidArgs) {
		t.Errorf("expected ErrCleanupInvalidArgs, got %v", err)
	}
}

func TestPostMergeCleanup_Execute_EmptyFeatureBranch(t *testing.T) {
	cleanup := NewPostMergeCleanup(&mockCleanupRepo{})

	_, err := cleanup.Execute("main", "", false)

	if err == nil {
		t.Fatal("expected error for empty featureBranch")
	}
	if !errors.Is(err, ErrCleanupInvalidArgs) {
		t.Errorf("expected ErrCleanupInvalidArgs, got %v", err)
	}
}

func TestPostMergeCleanup_Execute_SameBranches(t *testing.T) {
	cleanup := NewPostMergeCleanup(&mockCleanupRepo{})

	_, err := cleanup.Execute("main", "main", false)

	if err == nil {
		t.Fatal("expected error when defaultBranch == featureBranch")
	}
	if !errors.Is(err, ErrCleanupInvalidArgs) {
		t.Errorf("expected ErrCleanupInvalidArgs, got %v", err)
	}
}
