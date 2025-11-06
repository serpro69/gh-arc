package diff

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/serpro69/gh-arc/internal/github"
)

// Mock git repository for testing
type mockPRGitRepo struct {
	hasUnpushedCommits bool
	hasUnpushedErr     error
	pushErr            error
	pushedBranches     []string
}

func (m *mockPRGitRepo) HasUnpushedCommits(branch string) (bool, error) {
	if m.hasUnpushedErr != nil {
		return false, m.hasUnpushedErr
	}
	return m.hasUnpushedCommits, nil
}

func (m *mockPRGitRepo) Push(ctx context.Context, branch string) error {
	if m.pushErr != nil {
		return m.pushErr
	}
	m.pushedBranches = append(m.pushedBranches, branch)
	return nil
}

// Mock GitHub client for testing
type mockPRGitHubClient struct {
	createPRFunc       func(ctx context.Context, owner, name, title, head, base, body string, draft bool, parentPR *github.PullRequest) (*github.PullRequest, error)
	updatePRFunc       func(ctx context.Context, owner, name string, number int, title, body string, draft *bool, parentPR *github.PullRequest) (*github.PullRequest, error)
	markReadyFunc      func(ctx context.Context, owner, name string, pr *github.PullRequest) (*github.PullRequest, error)
	convertDraftFunc   func(ctx context.Context, owner, name string, pr *github.PullRequest) (*github.PullRequest, error)
	assignReviewersFunc func(ctx context.Context, owner, name string, number int, users, teams []string) error
	getCurrentUserFunc func(ctx context.Context) (string, error)
}

func (m *mockPRGitHubClient) CreatePullRequest(ctx context.Context, owner, name, title, head, base, body string, draft bool, parentPR *github.PullRequest) (*github.PullRequest, error) {
	if m.createPRFunc != nil {
		return m.createPRFunc(ctx, owner, name, title, head, base, body, draft, parentPR)
	}
	return &github.PullRequest{
		Number:  123,
		Title:   title,
		HTMLURL: "https://github.com/owner/repo/pull/123",
		Draft:   draft,
		Head:    github.PRBranch{Ref: head},
		Base:    github.PRBranch{Ref: base},
	}, nil
}

func (m *mockPRGitHubClient) UpdatePullRequest(ctx context.Context, owner, name string, number int, title, body string, draft *bool, parentPR *github.PullRequest) (*github.PullRequest, error) {
	if m.updatePRFunc != nil {
		return m.updatePRFunc(ctx, owner, name, number, title, body, draft, parentPR)
	}
	draftValue := false
	if draft != nil {
		draftValue = *draft
	}
	return &github.PullRequest{
		Number:  number,
		Title:   title,
		HTMLURL: fmt.Sprintf("https://github.com/owner/repo/pull/%d", number),
		Draft:   draftValue,
	}, nil
}

func (m *mockPRGitHubClient) MarkPRReadyForReview(ctx context.Context, owner, name string, pr *github.PullRequest) (*github.PullRequest, error) {
	if m.markReadyFunc != nil {
		return m.markReadyFunc(ctx, owner, name, pr)
	}
	pr.Draft = false
	return pr, nil
}

func (m *mockPRGitHubClient) ConvertPRToDraft(ctx context.Context, owner, name string, pr *github.PullRequest) (*github.PullRequest, error) {
	if m.convertDraftFunc != nil {
		return m.convertDraftFunc(ctx, owner, name, pr)
	}
	pr.Draft = true
	return pr, nil
}

func (m *mockPRGitHubClient) AssignReviewers(ctx context.Context, owner, name string, number int, users, teams []string) error {
	if m.assignReviewersFunc != nil {
		return m.assignReviewersFunc(ctx, owner, name, number, users, teams)
	}
	return nil
}

func (m *mockPRGitHubClient) GetCurrentUser(ctx context.Context) (string, error) {
	if m.getCurrentUserFunc != nil {
		return m.getCurrentUserFunc(ctx)
	}
	return "testuser", nil
}

func TestPRExecutor_CreatePR(t *testing.T) {
	tests := []struct {
		name       string
		request    *PRRequest
		pushErr    error
		createErr  error
		wantErr    bool
		wantPushed bool
	}{
		{
			name: "create PR successfully",
			request: &PRRequest{
				Title:       "Test PR",
				HeadBranch:  "feature/test",
				BaseBranch:  "main",
				Body:        "Test body",
				Draft:       false,
				Reviewers:   []string{},
				ExistingPR:  nil,
				CurrentUser: "testuser",
			},
			wantPushed: true,
		},
		{
			name: "create draft PR",
			request: &PRRequest{
				Title:       "Draft PR",
				HeadBranch:  "feature/draft",
				BaseBranch:  "main",
				Body:        "Draft body",
				Draft:       true,
				Reviewers:   []string{},
				ExistingPR:  nil,
				CurrentUser: "testuser",
			},
			wantPushed: true,
		},
		{
			name: "push failure",
			request: &PRRequest{
				Title:       "Test PR",
				HeadBranch:  "feature/test",
				BaseBranch:  "main",
				Body:        "Test body",
				Draft:       false,
				Reviewers:   []string{},
				ExistingPR:  nil,
				CurrentUser: "testuser",
			},
			pushErr:    errors.New("push failed"),
			wantErr:    true,
			wantPushed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockPRGitRepo{pushErr: tt.pushErr}
			mockClient := &mockPRGitHubClient{}

			executor := NewPRExecutor(mockClient, mockRepo, "owner", "repo")
			result, err := executor.CreateOrUpdatePR(context.Background(), tt.request)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateOrUpdatePR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if result.WasCreated != true {
				t.Errorf("Expected WasCreated = true for new PR, got %v", result.WasCreated)
			}

			if tt.wantPushed && len(mockRepo.pushedBranches) == 0 {
				t.Error("Expected branch to be pushed, but no pushes recorded")
			}
		})
	}
}

func TestPRExecutor_UpdatePR_DraftTransitions(t *testing.T) {
	tests := []struct {
		name             string
		existingDraft    bool
		requestDraft     bool
		hasUnpushed      bool
		wantDraftChanged bool
		wantPushed       bool
		wantMarkReady    bool
		wantConvertDraft bool
	}{
		{
			name:             "draft to ready transition",
			existingDraft:    true,
			requestDraft:     false,
			hasUnpushed:      false,
			wantDraftChanged: true,
			wantPushed:       false,
			wantMarkReady:    true,
			wantConvertDraft: false,
		},
		{
			name:             "ready to draft transition",
			existingDraft:    false,
			requestDraft:     true,
			hasUnpushed:      false,
			wantDraftChanged: true,
			wantPushed:       false,
			wantMarkReady:    false,
			wantConvertDraft: true,
		},
		{
			name:             "no draft status change",
			existingDraft:    false,
			requestDraft:     false,
			hasUnpushed:      false,
			wantDraftChanged: false,
			wantPushed:       false,
			wantMarkReady:    false,
			wantConvertDraft: false,
		},
		{
			name:             "update with unpushed commits",
			existingDraft:    false,
			requestDraft:     false,
			hasUnpushed:      true,
			wantDraftChanged: false,
			wantPushed:       true,
			wantMarkReady:    false,
			wantConvertDraft: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockPRGitRepo{
				hasUnpushedCommits: tt.hasUnpushed,
			}

			markReadyCalled := false
			convertDraftCalled := false

			mockClient := &mockPRGitHubClient{
				markReadyFunc: func(ctx context.Context, owner, name string, pr *github.PullRequest) (*github.PullRequest, error) {
					markReadyCalled = true
					pr.Draft = false
					return pr, nil
				},
				convertDraftFunc: func(ctx context.Context, owner, name string, pr *github.PullRequest) (*github.PullRequest, error) {
					convertDraftCalled = true
					pr.Draft = true
					return pr, nil
				},
			}

			existingPR := &github.PullRequest{
				Number: 42,
				Title:  "Existing PR",
				Draft:  tt.existingDraft,
			}

			request := &PRRequest{
				Title:       "Updated Title",
				HeadBranch:  "feature/test",
				BaseBranch:  "main",
				Body:        "Updated body",
				Draft:       tt.requestDraft,
				Reviewers:   []string{},
				ExistingPR:  existingPR,
				CurrentUser: "testuser",
			}

			executor := NewPRExecutor(mockClient, mockRepo, "owner", "repo")
			result, err := executor.CreateOrUpdatePR(context.Background(), request)

			if err != nil {
				t.Fatalf("CreateOrUpdatePR() unexpected error = %v", err)
			}

			if result.WasCreated {
				t.Error("Expected WasCreated = false for update, got true")
			}

			if result.DraftChanged != tt.wantDraftChanged {
				t.Errorf("DraftChanged = %v, want %v", result.DraftChanged, tt.wantDraftChanged)
			}

			if result.Pushed != tt.wantPushed {
				t.Errorf("Pushed = %v, want %v", result.Pushed, tt.wantPushed)
			}

			if markReadyCalled != tt.wantMarkReady {
				t.Errorf("MarkReady called = %v, want %v", markReadyCalled, tt.wantMarkReady)
			}

			if convertDraftCalled != tt.wantConvertDraft {
				t.Errorf("ConvertDraft called = %v, want %v", convertDraftCalled, tt.wantConvertDraft)
			}

			if tt.wantPushed && len(mockRepo.pushedBranches) == 0 {
				t.Error("Expected branch to be pushed, but no pushes recorded")
			}
		})
	}
}

func TestPRExecutor_AssignReviewers(t *testing.T) {
	tests := []struct {
		name           string
		reviewers      []string
		currentUser    string
		wantUsers      []string
		wantTeams      []string
		assignErr      error
		wantErr        bool
	}{
		{
			name:        "filter out current user",
			reviewers:   []string{"alice", "bob", "testuser", "charlie"},
			currentUser: "testuser",
			wantUsers:   []string{"alice", "bob", "charlie"},
			wantTeams:   []string{},
		},
		{
			name:        "teams and users",
			reviewers:   []string{"alice", "org/team1", "bob"},
			currentUser: "testuser",
			wantUsers:   []string{"alice", "bob"},
			wantTeams:   []string{"org/team1"},
		},
		{
			name:        "empty reviewers list",
			reviewers:   []string{},
			currentUser: "testuser",
			wantUsers:   nil,
			wantTeams:   nil,
		},
		{
			name:        "assignment failure",
			reviewers:   []string{"alice"},
			currentUser: "testuser",
			wantUsers:   []string{"alice"},
			assignErr:   errors.New("assignment failed"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var assignedUsers, assignedTeams []string

			mockClient := &mockPRGitHubClient{
				assignReviewersFunc: func(ctx context.Context, owner, name string, number int, users, teams []string) error {
					if tt.assignErr != nil {
						return tt.assignErr
					}
					assignedUsers = users
					assignedTeams = teams
					return nil
				},
			}

			executor := NewPRExecutor(mockClient, &mockPRGitRepo{}, "owner", "repo")

			pr := &github.PullRequest{Number: 123}
			reviewers, err := executor.assignReviewers(context.Background(), pr, tt.reviewers, tt.currentUser)

			if (err != nil) != tt.wantErr {
				t.Errorf("assignReviewers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if len(tt.reviewers) > 0 && len(reviewers) == 0 {
				// Skip empty reviewer check - function returns nil for empty results
				if len(tt.wantUsers) > 0 || len(tt.wantTeams) > 0 {
					t.Error("Expected reviewers to be returned, got empty slice")
				}
			}

			// Verify correct users and teams were assigned
			if len(assignedUsers) != len(tt.wantUsers) {
				t.Errorf("Assigned %d users, want %d", len(assignedUsers), len(tt.wantUsers))
			}

			if len(assignedTeams) != len(tt.wantTeams) {
				t.Errorf("Assigned %d teams, want %d", len(assignedTeams), len(tt.wantTeams))
			}
		})
	}
}

func TestPRExecutor_UpdateDraftStatus(t *testing.T) {
	tests := []struct {
		name          string
		currentDraft  bool
		wantDraft     bool
		wantMarkReady bool
		wantConvert   bool
		mockErr       error
		wantErr       bool
	}{
		{
			name:          "draft to ready",
			currentDraft:  true,
			wantDraft:     false,
			wantMarkReady: true,
			wantConvert:   false,
		},
		{
			name:          "ready to draft",
			currentDraft:  false,
			wantDraft:     true,
			wantMarkReady: false,
			wantConvert:   true,
		},
		{
			name:          "no change needed",
			currentDraft:  false,
			wantDraft:     false,
			wantMarkReady: false,
			wantConvert:   false,
		},
		{
			name:          "mark ready fails",
			currentDraft:  true,
			wantDraft:     false,
			wantMarkReady: true,
			mockErr:       errors.New("mark ready failed"),
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markReadyCalled := false
			convertDraftCalled := false

			mockClient := &mockPRGitHubClient{
				markReadyFunc: func(ctx context.Context, owner, name string, pr *github.PullRequest) (*github.PullRequest, error) {
					markReadyCalled = true
					if tt.mockErr != nil {
						return nil, tt.mockErr
					}
					pr.Draft = false
					return pr, nil
				},
				convertDraftFunc: func(ctx context.Context, owner, name string, pr *github.PullRequest) (*github.PullRequest, error) {
					convertDraftCalled = true
					if tt.mockErr != nil {
						return nil, tt.mockErr
					}
					pr.Draft = true
					return pr, nil
				},
			}

			pr := &github.PullRequest{
				Number: 42,
				Draft:  tt.currentDraft,
			}

			executor := NewPRExecutor(mockClient, &mockPRGitRepo{}, "owner", "repo")
			result, err := executor.UpdateDraftStatus(context.Background(), pr, tt.wantDraft)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateDraftStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if markReadyCalled != tt.wantMarkReady {
				t.Errorf("MarkReady called = %v, want %v", markReadyCalled, tt.wantMarkReady)
			}

			if convertDraftCalled != tt.wantConvert {
				t.Errorf("ConvertDraft called = %v, want %v", convertDraftCalled, tt.wantConvert)
			}

			if result.Draft != tt.wantDraft {
				t.Errorf("PR draft status = %v, want %v", result.Draft, tt.wantDraft)
			}
		})
	}
}
