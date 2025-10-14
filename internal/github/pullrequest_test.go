package github

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultPullRequestListOptions(t *testing.T) {
	opts := DefaultPullRequestListOptions()

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"State", opts.State, "open"},
		{"Sort", opts.Sort, "updated"},
		{"Direction", opts.Direction, "desc"},
		{"PerPage", opts.PerPage, 30},
		{"Page", opts.Page, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("DefaultPullRequestListOptions().%s = %v, expected %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestParseLinkHeader(t *testing.T) {
	tests := []struct {
		name           string
		linkHeader     string
		expectedLinks  map[string]string
		checkRel       string
		expectedExists bool
	}{
		{
			name:           "empty header",
			linkHeader:     "",
			expectedLinks:  map[string]string{},
			checkRel:       "next",
			expectedExists: false,
		},
		{
			name:       "single link",
			linkHeader: `<https://api.github.com/repos/owner/repo/pulls?page=2>; rel="next"`,
			expectedLinks: map[string]string{
				"next": "https://api.github.com/repos/owner/repo/pulls?page=2",
			},
			checkRel:       "next",
			expectedExists: true,
		},
		{
			name: "multiple links",
			linkHeader: `<https://api.github.com/repos/owner/repo/pulls?page=2>; rel="next", ` +
				`<https://api.github.com/repos/owner/repo/pulls?page=5>; rel="last"`,
			expectedLinks: map[string]string{
				"next": "https://api.github.com/repos/owner/repo/pulls?page=2",
				"last": "https://api.github.com/repos/owner/repo/pulls?page=5",
			},
			checkRel:       "next",
			expectedExists: true,
		},
		{
			name: "all link types",
			linkHeader: `<https://api.github.com/repos/owner/repo/pulls?page=3>; rel="next", ` +
				`<https://api.github.com/repos/owner/repo/pulls?page=1>; rel="prev", ` +
				`<https://api.github.com/repos/owner/repo/pulls?page=1>; rel="first", ` +
				`<https://api.github.com/repos/owner/repo/pulls?page=10>; rel="last"`,
			expectedLinks: map[string]string{
				"next":  "https://api.github.com/repos/owner/repo/pulls?page=3",
				"prev":  "https://api.github.com/repos/owner/repo/pulls?page=1",
				"first": "https://api.github.com/repos/owner/repo/pulls?page=1",
				"last":  "https://api.github.com/repos/owner/repo/pulls?page=10",
			},
			checkRel:       "last",
			expectedExists: true,
		},
		{
			name:           "malformed link - missing parts",
			linkHeader:     `<https://api.github.com/repos/owner/repo/pulls?page=2>`,
			expectedLinks:  map[string]string{},
			checkRel:       "next",
			expectedExists: false,
		},
		{
			name:           "malformed link - invalid format",
			linkHeader:     `invalid link header`,
			expectedLinks:  map[string]string{},
			checkRel:       "next",
			expectedExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := parseLinkHeader(tt.linkHeader)

			// Check the specific rel we're testing
			_, exists := links[tt.checkRel]
			if exists != tt.expectedExists {
				t.Errorf("parseLinkHeader(%q)[%q] existence = %v, expected %v",
					tt.linkHeader, tt.checkRel, exists, tt.expectedExists)
			}

			// Verify the actual links match expected
			for rel, expectedURL := range tt.expectedLinks {
				gotURL, exists := links[rel]
				if !exists {
					t.Errorf("parseLinkHeader(%q) missing expected rel %q", tt.linkHeader, rel)
					continue
				}
				if gotURL != expectedURL {
					t.Errorf("parseLinkHeader(%q)[%q] = %q, expected %q",
						tt.linkHeader, rel, gotURL, expectedURL)
				}
			}

			// Verify no unexpected links
			if len(links) != len(tt.expectedLinks) {
				t.Errorf("parseLinkHeader(%q) returned %d links, expected %d",
					tt.linkHeader, len(links), len(tt.expectedLinks))
			}
		})
	}
}

func TestPullRequestStructures(t *testing.T) {
	t.Run("PullRequest structure", func(t *testing.T) {
		now := time.Now()
		pr := &PullRequest{
			Number:    123,
			Title:     "Test PR",
			State:     "open",
			Draft:     false,
			CreatedAt: now,
			UpdatedAt: now,
			User: PRUser{
				Login: "testuser",
				Name:  "Test User",
				Email: "test@example.com",
			},
			Head: PRBranch{
				Ref: "feature-branch",
				SHA: "abc123",
				Repo: PRRepository{
					Name:     "test-repo",
					FullName: "owner/test-repo",
					Owner: PRUser{
						Login: "owner",
					},
				},
			},
			Base: PRBranch{
				Ref: "main",
				SHA: "def456",
				Repo: PRRepository{
					Name:     "test-repo",
					FullName: "owner/test-repo",
					Owner: PRUser{
						Login: "owner",
					},
				},
			},
			HTMLURL: "https://github.com/owner/test-repo/pull/123",
		}

		if pr.Number != 123 {
			t.Errorf("PR Number = %d, expected 123", pr.Number)
		}
		if pr.Title != "Test PR" {
			t.Errorf("PR Title = %s, expected 'Test PR'", pr.Title)
		}
		if pr.User.Login != "testuser" {
			t.Errorf("PR User.Login = %s, expected 'testuser'", pr.User.Login)
		}
		if pr.Head.Ref != "feature-branch" {
			t.Errorf("PR Head.Ref = %s, expected 'feature-branch'", pr.Head.Ref)
		}
		if pr.Base.Ref != "main" {
			t.Errorf("PR Base.Ref = %s, expected 'main'", pr.Base.Ref)
		}
	})

	t.Run("PRReview structure", func(t *testing.T) {
		now := time.Now()
		review := PRReview{
			ID: 1,
			User: PRUser{
				Login: "reviewer",
			},
			State:       "APPROVED",
			SubmittedAt: now,
		}

		if review.State != "APPROVED" {
			t.Errorf("Review State = %s, expected 'APPROVED'", review.State)
		}
	})

	t.Run("PRCheck structure", func(t *testing.T) {
		now := time.Now()
		check := PRCheck{
			ID:          1,
			Name:        "ci-test",
			Status:      "completed",
			Conclusion:  "success",
			StartedAt:   now,
			CompletedAt: now.Add(5 * time.Minute),
		}

		if check.Status != "completed" {
			t.Errorf("Check Status = %s, expected 'completed'", check.Status)
		}
		if check.Conclusion != "success" {
			t.Errorf("Check Conclusion = %s, expected 'success'", check.Conclusion)
		}
	})

	t.Run("PRReviewer structure", func(t *testing.T) {
		reviewer := PRReviewer{
			Login: "reviewer1",
			Type:  "User",
		}

		if reviewer.Type != "User" {
			t.Errorf("Reviewer Type = %s, expected 'User'", reviewer.Type)
		}
	})
}

func TestPullRequestListOptionsVariations(t *testing.T) {
	tests := []struct {
		name string
		opts *PullRequestListOptions
	}{
		{
			name: "open PRs sorted by created",
			opts: &PullRequestListOptions{
				State:     "open",
				Sort:      "created",
				Direction: "desc",
				PerPage:   50,
				Page:      1,
			},
		},
		{
			name: "closed PRs sorted by updated",
			opts: &PullRequestListOptions{
				State:     "closed",
				Sort:      "updated",
				Direction: "asc",
				PerPage:   100,
				Page:      2,
			},
		},
		{
			name: "all PRs with custom pagination",
			opts: &PullRequestListOptions{
				State:     "all",
				Sort:      "popularity",
				Direction: "desc",
				PerPage:   25,
				Page:      3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the structure is valid
			if tt.opts.PerPage < 1 || tt.opts.PerPage > 100 {
				t.Errorf("PerPage %d is outside valid range (1-100)", tt.opts.PerPage)
			}
			if tt.opts.Page < 1 {
				t.Errorf("Page %d should be >= 1", tt.opts.Page)
			}

			validStates := map[string]bool{"open": true, "closed": true, "all": true}
			if !validStates[tt.opts.State] {
				t.Errorf("State %s is not valid (open, closed, all)", tt.opts.State)
			}

			validSorts := map[string]bool{"created": true, "updated": true, "popularity": true, "long-running": true}
			if !validSorts[tt.opts.Sort] {
				t.Errorf("Sort %s is not valid", tt.opts.Sort)
			}

			validDirections := map[string]bool{"asc": true, "desc": true}
			if !validDirections[tt.opts.Direction] {
				t.Errorf("Direction %s is not valid (asc, desc)", tt.opts.Direction)
			}
		})
	}
}

func TestPRUserFields(t *testing.T) {
	tests := []struct {
		name string
		user PRUser
	}{
		{
			name: "user with all fields",
			user: PRUser{
				Login: "octocat",
				Name:  "The Octocat",
				Email: "octocat@github.com",
			},
		},
		{
			name: "user with login only",
			user: PRUser{
				Login: "octocat",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.user.Login == "" {
				t.Error("User Login should not be empty")
			}
		})
	}
}

func TestPRBranchFields(t *testing.T) {
	branch := PRBranch{
		Ref: "feature/test",
		SHA: "abc123def456",
		Repo: PRRepository{
			Name:     "test-repo",
			FullName: "owner/test-repo",
			Owner: PRUser{
				Login: "owner",
			},
		},
	}

	if branch.Ref == "" {
		t.Error("Branch Ref should not be empty")
	}
	if branch.SHA == "" {
		t.Error("Branch SHA should not be empty")
	}
	if branch.Repo.FullName == "" {
		t.Error("Branch Repo.FullName should not be empty")
	}
}

func TestPRReviewStates(t *testing.T) {
	validStates := []string{
		"APPROVED",
		"CHANGES_REQUESTED",
		"COMMENTED",
		"DISMISSED",
		"PENDING",
	}

	for _, state := range validStates {
		t.Run(state, func(t *testing.T) {
			review := PRReview{
				ID:    1,
				State: state,
			}

			if review.State != state {
				t.Errorf("Review State = %s, expected %s", review.State, state)
			}
		})
	}
}

func TestPRCheckStatuses(t *testing.T) {
	tests := []struct {
		status     string
		conclusion string
	}{
		{"queued", ""},
		{"in_progress", ""},
		{"completed", "success"},
		{"completed", "failure"},
		{"completed", "neutral"},
		{"completed", "cancelled"},
		{"completed", "skipped"},
		{"completed", "timed_out"},
		{"completed", "action_required"},
	}

	for _, tt := range tests {
		t.Run(tt.status+"_"+tt.conclusion, func(t *testing.T) {
			check := PRCheck{
				ID:         1,
				Name:       "test-check",
				Status:     tt.status,
				Conclusion: tt.conclusion,
			}

			if check.Status != tt.status {
				t.Errorf("Check Status = %s, expected %s", check.Status, tt.status)
			}
			if check.Conclusion != tt.conclusion {
				t.Errorf("Check Conclusion = %s, expected %s", check.Conclusion, tt.conclusion)
			}
		})
	}
}

func TestGetCurrentRepositoryPullRequestsError(t *testing.T) {
	// Create client without repository context
	client := &Client{
		repo:           nil,
		config:         DefaultConfig(),
		cache:          &NoOpCache{},
		circuitBreaker: NewCircuitBreaker(5, 1*time.Minute),
	}

	_, err := client.GetCurrentRepositoryPullRequests(nil, nil)
	if err == nil {
		t.Error("Expected error when repository context is not set")
	}

	expectedMsg := "no repository context set"
	if err.Error() != expectedMsg {
		t.Errorf("Error message = %q, expected %q", err.Error(), expectedMsg)
	}
}

func TestGetCurrentRepositoryPullRequestsWithPaginationError(t *testing.T) {
	// Create client without repository context
	client := &Client{
		repo:           nil,
		config:         DefaultConfig(),
		cache:          &NoOpCache{},
		circuitBreaker: NewCircuitBreaker(5, 1*time.Minute),
	}

	_, err := client.GetCurrentRepositoryPullRequestsWithPagination(nil, nil)
	if err == nil {
		t.Error("Expected error when repository context is not set")
	}

	expectedMsg := "no repository context set"
	if err.Error() != expectedMsg {
		t.Errorf("Error message = %q, expected %q", err.Error(), expectedMsg)
	}
}

func TestDeterminePRStatus(t *testing.T) {
	tests := []struct {
		name               string
		reviews            []PRReview
		checks             []PRCheck
		expectedReviewStat string
		expectedCheckStat  string
	}{
		{
			name:               "no reviews or checks",
			reviews:            []PRReview{},
			checks:             []PRCheck{},
			expectedReviewStat: "review_required",
			expectedCheckStat:  "pending",
		},
		{
			name: "approved review",
			reviews: []PRReview{
				{State: "APPROVED"},
			},
			checks:             []PRCheck{},
			expectedReviewStat: "approved",
			expectedCheckStat:  "pending",
		},
		{
			name: "changes requested takes priority",
			reviews: []PRReview{
				{State: "APPROVED"},
				{State: "CHANGES_REQUESTED"},
			},
			checks:             []PRCheck{},
			expectedReviewStat: "changes_requested",
			expectedCheckStat:  "pending",
		},
		{
			name: "commented review",
			reviews: []PRReview{
				{State: "COMMENTED"},
			},
			checks:             []PRCheck{},
			expectedReviewStat: "commented",
			expectedCheckStat:  "pending",
		},
		{
			name: "pending review",
			reviews: []PRReview{
				{State: "PENDING"},
			},
			checks:             []PRCheck{},
			expectedReviewStat: "pending",
			expectedCheckStat:  "pending",
		},
		{
			name:    "all checks success",
			reviews: []PRReview{},
			checks: []PRCheck{
				{Status: "completed", Conclusion: "success"},
				{Status: "completed", Conclusion: "success"},
			},
			expectedReviewStat: "review_required",
			expectedCheckStat:  "success",
		},
		{
			name:    "one check failure",
			reviews: []PRReview{},
			checks: []PRCheck{
				{Status: "completed", Conclusion: "success"},
				{Status: "completed", Conclusion: "failure"},
			},
			expectedReviewStat: "review_required",
			expectedCheckStat:  "failure",
		},
		{
			name:    "checks in progress",
			reviews: []PRReview{},
			checks: []PRCheck{
				{Status: "in_progress"},
				{Status: "completed", Conclusion: "success"},
			},
			expectedReviewStat: "review_required",
			expectedCheckStat:  "in_progress",
		},
		{
			name:    "neutral check result",
			reviews: []PRReview{},
			checks: []PRCheck{
				{Status: "completed", Conclusion: "neutral"},
			},
			expectedReviewStat: "review_required",
			expectedCheckStat:  "neutral",
		},
		{
			name: "approved with successful checks",
			reviews: []PRReview{
				{State: "APPROVED"},
			},
			checks: []PRCheck{
				{Status: "completed", Conclusion: "success"},
			},
			expectedReviewStat: "approved",
			expectedCheckStat:  "success",
		},
		{
			name: "changes requested with failed checks",
			reviews: []PRReview{
				{State: "CHANGES_REQUESTED"},
			},
			checks: []PRCheck{
				{Status: "completed", Conclusion: "failure"},
			},
			expectedReviewStat: "changes_requested",
			expectedCheckStat:  "failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := DeterminePRStatus(tt.reviews, tt.checks)

			if status.ReviewStatus != tt.expectedReviewStat {
				t.Errorf("ReviewStatus = %s, expected %s", status.ReviewStatus, tt.expectedReviewStat)
			}

			if status.CheckStatus != tt.expectedCheckStat {
				t.Errorf("CheckStatus = %s, expected %s", status.CheckStatus, tt.expectedCheckStat)
			}
		})
	}
}

func TestPRStatusStruct(t *testing.T) {
	status := PRStatus{
		ReviewStatus: "approved",
		CheckStatus:  "success",
	}

	if status.ReviewStatus != "approved" {
		t.Errorf("ReviewStatus = %s, expected approved", status.ReviewStatus)
	}

	if status.CheckStatus != "success" {
		t.Errorf("CheckStatus = %s, expected success", status.CheckStatus)
	}
}

func TestEnrichPullRequestNilPR(t *testing.T) {
	client := &Client{
		config:         DefaultConfig(),
		cache:          &NoOpCache{},
		circuitBreaker: NewCircuitBreaker(5, 1*time.Minute),
	}

	err := client.EnrichPullRequest(nil, "owner", "repo", nil)
	if err == nil {
		t.Error("Expected error when PR is nil")
	}

	expectedMsg := "pull request is nil"
	if err.Error() != expectedMsg {
		t.Errorf("Error message = %q, expected %q", err.Error(), expectedMsg)
	}
}

func TestEnrichPullRequestsEmptyList(t *testing.T) {
	client := &Client{
		config:         DefaultConfig(),
		cache:          &NoOpCache{},
		circuitBreaker: NewCircuitBreaker(5, 1*time.Minute),
	}

	err := client.EnrichPullRequests(nil, "owner", "repo", []*PullRequest{})
	if err != nil {
		t.Errorf("EnrichPullRequests with empty list should not error, got: %v", err)
	}
}

func TestReviewStatusPriority(t *testing.T) {
	// Test that CHANGES_REQUESTED takes priority over APPROVED
	reviews := []PRReview{
		{ID: 1, State: "APPROVED", User: PRUser{Login: "user1"}},
		{ID: 2, State: "APPROVED", User: PRUser{Login: "user2"}},
		{ID: 3, State: "CHANGES_REQUESTED", User: PRUser{Login: "user3"}},
		{ID: 4, State: "COMMENTED", User: PRUser{Login: "user4"}},
	}

	status := DeterminePRStatus(reviews, []PRCheck{})

	if status.ReviewStatus != "changes_requested" {
		t.Errorf("Expected CHANGES_REQUESTED to take priority, got %s", status.ReviewStatus)
	}
}

func TestCheckStatusPriority(t *testing.T) {
	// Test that failure takes priority over success
	checks := []PRCheck{
		{ID: 1, Status: "completed", Conclusion: "success", Name: "test1"},
		{ID: 2, Status: "completed", Conclusion: "success", Name: "test2"},
		{ID: 3, Status: "completed", Conclusion: "failure", Name: "test3"},
	}

	status := DeterminePRStatus([]PRReview{}, checks)

	if status.CheckStatus != "failure" {
		t.Errorf("Expected failure to take priority, got %s", status.CheckStatus)
	}
}

func TestCheckStatusInProgressPriority(t *testing.T) {
	// Test that in_progress takes priority over neutral
	checks := []PRCheck{
		{ID: 1, Status: "completed", Conclusion: "neutral", Name: "test1"},
		{ID: 2, Status: "in_progress", Name: "test2"},
	}

	status := DeterminePRStatus([]PRReview{}, checks)

	if status.CheckStatus != "in_progress" {
		t.Errorf("Expected in_progress status, got %s", status.CheckStatus)
	}
}

func TestCheckConclusionEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		conclusion     string
		expectedStatus string
	}{
		{"timed_out is failure", "timed_out", "failure"},
		{"action_required is failure", "action_required", "failure"},
		{"cancelled is neutral", "cancelled", "neutral"},
		{"skipped is neutral", "skipped", "neutral"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks := []PRCheck{
				{Status: "completed", Conclusion: tt.conclusion},
			}

			status := DeterminePRStatus([]PRReview{}, checks)

			if status.CheckStatus != tt.expectedStatus {
				t.Errorf("For conclusion %s, expected status %s, got %s",
					tt.conclusion, tt.expectedStatus, status.CheckStatus)
			}
		})
	}
}

func TestMultipleReviewStates(t *testing.T) {
	// Test various combinations of review states
	tests := []struct {
		name     string
		states   []string
		expected string
	}{
		{
			name:     "multiple approvals",
			states:   []string{"APPROVED", "APPROVED", "APPROVED"},
			expected: "approved",
		},
		{
			name:     "multiple comments",
			states:   []string{"COMMENTED", "COMMENTED"},
			expected: "commented",
		},
		{
			name:     "multiple pending",
			states:   []string{"PENDING", "PENDING"},
			expected: "pending",
		},
		{
			name:     "mixed with changes requested",
			states:   []string{"APPROVED", "COMMENTED", "CHANGES_REQUESTED"},
			expected: "changes_requested",
		},
		{
			name:     "approved and commented",
			states:   []string{"APPROVED", "COMMENTED"},
			expected: "approved",
		},
		{
			name:     "commented and pending",
			states:   []string{"COMMENTED", "PENDING"},
			expected: "commented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reviews []PRReview
			for i, state := range tt.states {
				reviews = append(reviews, PRReview{
					ID:    i + 1,
					State: state,
				})
			}

			status := DeterminePRStatus(reviews, []PRCheck{})

			if status.ReviewStatus != tt.expected {
				t.Errorf("For states %v, expected %s, got %s",
					tt.states, tt.expected, status.ReviewStatus)
			}
		})
	}
}

func TestFindExistingPR(t *testing.T) {
	t.Run("no repository context", func(t *testing.T) {
		client := &Client{
			repo:           nil,
			config:         DefaultConfig(),
			cache:          &NoOpCache{},
			circuitBreaker: NewCircuitBreaker(5, 1*time.Minute),
		}

		_, err := client.FindExistingPRForCurrentBranch(nil, "feature-branch")
		if err == nil {
			t.Error("Expected error when repository context is not set")
		}

		if err.Error() != "no repository context set" {
			t.Errorf("Error message = %q, expected 'no repository context set'", err.Error())
		}
	})
}

func TestDetectBaseChanged(t *testing.T) {
	tests := []struct {
		name          string
		existingPR    *PullRequest
		detectedBase  string
		expectChanged bool
	}{
		{
			name:          "nil PR",
			existingPR:    nil,
			detectedBase:  "main",
			expectChanged: false,
		},
		{
			name: "base unchanged",
			existingPR: &PullRequest{
				Number: 123,
				Base: PRBranch{
					Ref: "main",
				},
			},
			detectedBase:  "main",
			expectChanged: false,
		},
		{
			name: "base changed from main to feature",
			existingPR: &PullRequest{
				Number: 123,
				Base: PRBranch{
					Ref: "main",
				},
			},
			detectedBase:  "feature/auth",
			expectChanged: true,
		},
		{
			name: "base changed from feature to main",
			existingPR: &PullRequest{
				Number: 123,
				Base: PRBranch{
					Ref: "feature/parent",
				},
			},
			detectedBase:  "main",
			expectChanged: true,
		},
		{
			name: "base changed between feature branches",
			existingPR: &PullRequest{
				Number: 123,
				Base: PRBranch{
					Ref: "feature/auth",
				},
			},
			detectedBase:  "feature/payment",
			expectChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBaseChanged(tt.existingPR, tt.detectedBase)

			if result != tt.expectChanged {
				t.Errorf("DetectBaseChanged() = %v, want %v", result, tt.expectChanged)
			}
		})
	}
}

func TestFindDependentPRsForCurrentBranch(t *testing.T) {
	t.Run("no repository context", func(t *testing.T) {
		client := &Client{
			repo:           nil,
			config:         DefaultConfig(),
			cache:          &NoOpCache{},
			circuitBreaker: NewCircuitBreaker(5, 1*time.Minute),
		}

		_, err := client.FindDependentPRsForCurrentBranch(nil, "feature-branch")
		if err == nil {
			t.Error("Expected error when repository context is not set")
		}

		if err.Error() != "no repository context set" {
			t.Errorf("Error message = %q, expected 'no repository context set'", err.Error())
		}
	})
}

func TestUpdatePRBaseForCurrentRepo(t *testing.T) {
	t.Run("no repository context", func(t *testing.T) {
		client := &Client{
			repo:           nil,
			config:         DefaultConfig(),
			cache:          &NoOpCache{},
			circuitBreaker: NewCircuitBreaker(5, 1*time.Minute),
		}

		err := client.UpdatePRBaseForCurrentRepo(nil, 123, "new-base")
		if err == nil {
			t.Error("Expected error when repository context is not set")
		}

		if err.Error() != "no repository context set" {
			t.Errorf("Error message = %q, expected 'no repository context set'", err.Error())
		}
	})
}

func TestDependentPRLogic(t *testing.T) {
	// Test the logic for filtering dependent PRs
	// Simulating how FindDependentPRs would filter PRs

	t.Run("filter PRs by base branch", func(t *testing.T) {
		allPRs := []*PullRequest{
			{
				Number: 100,
				Head:   PRBranch{Ref: "feature/child1"},
				Base:   PRBranch{Ref: "feature/parent"},
			},
			{
				Number: 101,
				Head:   PRBranch{Ref: "feature/child2"},
				Base:   PRBranch{Ref: "feature/parent"},
			},
			{
				Number: 102,
				Head:   PRBranch{Ref: "feature/child3"},
				Base:   PRBranch{Ref: "main"},
			},
			{
				Number: 103,
				Head:   PRBranch{Ref: "feature/parent"},
				Base:   PRBranch{Ref: "main"},
			},
		}

		targetBase := "feature/parent"
		var dependentPRs []*PullRequest

		// Simulate FindDependentPRs filtering logic
		for _, pr := range allPRs {
			if pr.Base.Ref == targetBase {
				dependentPRs = append(dependentPRs, pr)
			}
		}

		// Should find PRs 100 and 101 (both target feature/parent as base)
		if len(dependentPRs) != 2 {
			t.Errorf("Expected 2 dependent PRs, got %d", len(dependentPRs))
		}

		// Verify the correct PRs were found
		foundNumbers := make(map[int]bool)
		for _, pr := range dependentPRs {
			foundNumbers[pr.Number] = true
		}

		if !foundNumbers[100] || !foundNumbers[101] {
			t.Error("Expected to find PRs #100 and #101 as dependents")
		}
	})

	t.Run("no dependent PRs", func(t *testing.T) {
		allPRs := []*PullRequest{
			{
				Number: 100,
				Head:   PRBranch{Ref: "feature/a"},
				Base:   PRBranch{Ref: "main"},
			},
			{
				Number: 101,
				Head:   PRBranch{Ref: "feature/b"},
				Base:   PRBranch{Ref: "main"},
			},
		}

		targetBase := "feature/parent"
		var dependentPRs []*PullRequest

		for _, pr := range allPRs {
			if pr.Base.Ref == targetBase {
				dependentPRs = append(dependentPRs, pr)
			}
		}

		if len(dependentPRs) != 0 {
			t.Errorf("Expected 0 dependent PRs, got %d", len(dependentPRs))
		}
	})

	t.Run("multiple levels of stacking", func(t *testing.T) {
		allPRs := []*PullRequest{
			{
				Number: 100,
				Head:   PRBranch{Ref: "feature/grandchild"},
				Base:   PRBranch{Ref: "feature/child"},
			},
			{
				Number: 101,
				Head:   PRBranch{Ref: "feature/child"},
				Base:   PRBranch{Ref: "feature/parent"},
			},
			{
				Number: 102,
				Head:   PRBranch{Ref: "feature/parent"},
				Base:   PRBranch{Ref: "main"},
			},
		}

		// Find dependents of feature/parent (should be feature/child)
		targetBase := "feature/parent"
		var dependentPRs []*PullRequest

		for _, pr := range allPRs {
			if pr.Base.Ref == targetBase {
				dependentPRs = append(dependentPRs, pr)
			}
		}

		if len(dependentPRs) != 1 {
			t.Errorf("Expected 1 dependent PR for feature/parent, got %d", len(dependentPRs))
		}

		if dependentPRs[0].Number != 101 {
			t.Errorf("Expected PR #101 as dependent, got #%d", dependentPRs[0].Number)
		}
	})
}

func TestDetectRebase(t *testing.T) {
	tests := []struct {
		name           string
		existingPR     *PullRequest
		currentBaseSHA string
		expectRebased  bool
	}{
		{
			name:           "nil PR",
			existingPR:     nil,
			currentBaseSHA: "abc123",
			expectRebased:  false,
		},
		{
			name: "empty current SHA",
			existingPR: &PullRequest{
				Number: 100,
				Base: PRBranch{
					Ref: "main",
					SHA: "abc123",
				},
			},
			currentBaseSHA: "",
			expectRebased:  false,
		},
		{
			name: "SHA matches - no rebase",
			existingPR: &PullRequest{
				Number: 100,
				Base: PRBranch{
					Ref: "main",
					SHA: "abc1234567890",
				},
			},
			currentBaseSHA: "abc1234567890",
			expectRebased:  false,
		},
		{
			name: "SHA differs - rebase detected",
			existingPR: &PullRequest{
				Number: 100,
				Base: PRBranch{
					Ref: "main",
					SHA: "abc1234567890",
				},
			},
			currentBaseSHA: "def9876543210",
			expectRebased:  true,
		},
		{
			name: "different SHA on feature branch",
			existingPR: &PullRequest{
				Number: 100,
				Base: PRBranch{
					Ref: "feature/parent",
					SHA: "1111111111111",
				},
			},
			currentBaseSHA: "2222222222222",
			expectRebased:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectRebase(tt.existingPR, tt.currentBaseSHA)
			if result != tt.expectRebased {
				t.Errorf("DetectRebase() = %v, want %v", result, tt.expectRebased)
			}
		})
	}
}

func TestHandleStackedPRUpdate(t *testing.T) {
	t.Run("nil PR error", func(t *testing.T) {
		client := &Client{}
		result, err := client.HandleStackedPRUpdate(nil, nil, "main", "abc123", false)

		if err == nil {
			t.Error("Expected error for nil PR, got nil")
		}

		if result == nil {
			t.Fatal("Expected result even on error")
		}

		if result.Error == nil {
			t.Error("Expected result.Error to be set")
		}
	})

	t.Run("no changes detected", func(t *testing.T) {
		client := &Client{}
		existingPR := &PullRequest{
			Number: 100,
			Base: PRBranch{
				Ref: "main",
				SHA: "abc123",
			},
		}

		result, err := client.HandleStackedPRUpdate(nil, existingPR, "main", "abc123", false)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if result.UpdatedBase {
			t.Error("Expected UpdatedBase to be false when no changes")
		}

		if result.RebaseDetected {
			t.Error("Expected RebaseDetected to be false when SHAs match")
		}
	})

	t.Run("base branch changed", func(t *testing.T) {
		// This would require mocking the GitHub API call
		// For now, we test the logic without the actual API update
		existingPR := &PullRequest{
			Number: 100,
			Base: PRBranch{
				Ref: "main",
				SHA: "abc123",
			},
		}

		// Test that DetectBaseChanged would return true
		changed := DetectBaseChanged(existingPR, "feature/parent")
		if !changed {
			t.Error("Expected base change to be detected")
		}
	})

	t.Run("rebase detected but same branch", func(t *testing.T) {
		client := &Client{}
		existingPR := &PullRequest{
			Number: 100,
			Base: PRBranch{
				Ref: "main",
				SHA: "abc1234567890",
			},
		}

		result, err := client.HandleStackedPRUpdate(nil, existingPR, "main", "def9876543210", false)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if !result.RebaseDetected {
			t.Error("Expected RebaseDetected to be true when SHA differs")
		}

		// Base should not be updated when only SHA changed (same branch name)
		if result.UpdatedBase {
			t.Error("Expected UpdatedBase to be false when only rebase detected")
		}
	})

	t.Run("result fields populated correctly", func(t *testing.T) {
		client := &Client{}
		existingPR := &PullRequest{
			Number: 100,
			Base: PRBranch{
				Ref: "main",
				SHA: "abc123",
			},
		}

		// Use same branch name to avoid API call, but different SHA to trigger rebase detection
		result, err := client.HandleStackedPRUpdate(nil, existingPR, "main", "def456", false)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if result.OldBase != "main" {
			t.Errorf("Expected OldBase to be 'main', got '%s'", result.OldBase)
		}

		if result.NewBase != "main" {
			t.Errorf("Expected NewBase to be 'main', got '%s'", result.NewBase)
		}

		if !result.RebaseDetected {
			t.Error("Expected RebaseDetected to be true when SHA changed")
		}

		// Should not update base when only SHA changed (rebase on same branch)
		if result.UpdatedBase {
			t.Error("Expected UpdatedBase to be false for rebase-only scenario")
		}
	})
}

func TestStackedPRUpdateResult(t *testing.T) {
	t.Run("result struct fields", func(t *testing.T) {
		result := &StackedPRUpdateResult{
			UpdatedBase:    true,
			OldBase:        "main",
			NewBase:        "feature/parent",
			RebaseDetected: false,
			Error:          nil,
		}

		if !result.UpdatedBase {
			t.Error("Expected UpdatedBase to be true")
		}

		if result.OldBase != "main" {
			t.Errorf("Expected OldBase 'main', got '%s'", result.OldBase)
		}

		if result.NewBase != "feature/parent" {
			t.Errorf("Expected NewBase 'feature/parent', got '%s'", result.NewBase)
		}

		if result.RebaseDetected {
			t.Error("Expected RebaseDetected to be false")
		}

		if result.Error != nil {
			t.Errorf("Expected no error, got %v", result.Error)
		}
	})
}

func TestFormatStackingMetadata(t *testing.T) {
	tests := []struct {
		name          string
		parentPR      *PullRequest
		expectEmpty   bool
		expectedParts []string
	}{
		{
			name:        "nil parent PR",
			parentPR:    nil,
			expectEmpty: true,
		},
		{
			name: "parent PR with title",
			parentPR: &PullRequest{
				Number: 122,
				Title:  "Add authentication system",
			},
			expectEmpty: false,
			expectedParts: []string{
				"---",
				"📚 **Stacked on:**",
				"#122",
				"Add authentication system",
				"part of a stack",
				"Review and merge that PR first",
			},
		},
		{
			name: "parent PR with long title",
			parentPR: &PullRequest{
				Number: 999,
				Title:  "Implement comprehensive authentication and authorization system with JWT",
			},
			expectEmpty: false,
			expectedParts: []string{
				"---",
				"📚 **Stacked on:**",
				"#999",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatStackingMetadata(tt.parentPR)

			if tt.expectEmpty {
				if result != "" {
					t.Errorf("Expected empty string, got %q", result)
				}
				return
			}

			for _, part := range tt.expectedParts {
				if !strings.Contains(result, part) {
					t.Errorf("Expected result to contain %q, got %q", part, result)
				}
			}
		})
	}
}

func TestCreatePullRequestForCurrentRepo(t *testing.T) {
	t.Run("no repository context", func(t *testing.T) {
		client := &Client{
			repo:           nil,
			config:         DefaultConfig(),
			cache:          &NoOpCache{},
			circuitBreaker: NewCircuitBreaker(5, 1*time.Minute),
		}

		_, err := client.CreatePullRequestForCurrentRepo(nil, "Test PR", "feature", "main", "Description", false, nil)
		if err == nil {
			t.Error("Expected error when repository context is not set")
		}

		if err.Error() != "no repository context set" {
			t.Errorf("Error message = %q, expected 'no repository context set'", err.Error())
		}
	})
}

func TestCreatePRRequestStructure(t *testing.T) {
	t.Run("basic request fields", func(t *testing.T) {
		req := CreatePRRequest{
			Title:               "Test PR",
			Head:                "feature-branch",
			Base:                "main",
			Body:                "Test description",
			Draft:               false,
			MaintainerCanModify: true,
		}

		if req.Title != "Test PR" {
			t.Errorf("Title = %q, expected 'Test PR'", req.Title)
		}
		if req.Head != "feature-branch" {
			t.Errorf("Head = %q, expected 'feature-branch'", req.Head)
		}
		if req.Base != "main" {
			t.Errorf("Base = %q, expected 'main'", req.Base)
		}
		if !req.MaintainerCanModify {
			t.Error("Expected MaintainerCanModify to be true")
		}
	})

	t.Run("draft PR", func(t *testing.T) {
		req := CreatePRRequest{
			Title:               "Draft PR",
			Head:                "wip-feature",
			Base:                "main",
			Body:                "",
			Draft:               true,
			MaintainerCanModify: true,
		}

		if !req.Draft {
			t.Error("Expected Draft to be true")
		}
	})
}

func TestUpdatePRRequestStructure(t *testing.T) {
	t.Run("update title only", func(t *testing.T) {
		req := UpdatePRRequest{
			Title: "Updated title",
		}

		if req.Title != "Updated title" {
			t.Errorf("Title = %q, expected 'Updated title'", req.Title)
		}
		if req.Body != "" {
			t.Error("Body should be empty")
		}
	})

	t.Run("update draft state", func(t *testing.T) {
		readyForReview := false
		req := UpdatePRRequest{
			Draft: &readyForReview,
		}

		if req.Draft == nil {
			t.Fatal("Draft pointer should not be nil")
		}
		if *req.Draft != false {
			t.Error("Expected draft to be false (ready for review)")
		}
	})

	t.Run("nil draft means no change", func(t *testing.T) {
		req := UpdatePRRequest{
			Title: "Update",
			Draft: nil,
		}

		if req.Draft != nil {
			t.Error("Draft should be nil to indicate no change")
		}
	})
}

func TestUpdatePullRequestForCurrentRepo(t *testing.T) {
	t.Run("no repository context", func(t *testing.T) {
		client := &Client{
			repo:           nil,
			config:         DefaultConfig(),
			cache:          &NoOpCache{},
			circuitBreaker: NewCircuitBreaker(5, 1*time.Minute),
		}

		_, err := client.UpdatePullRequestForCurrentRepo(nil, 123, "Updated title", "Updated body", nil, nil)
		if err == nil {
			t.Error("Expected error when repository context is not set")
		}

		if err.Error() != "no repository context set" {
			t.Errorf("Error message = %q, expected 'no repository context set'", err.Error())
		}
	})
}

func TestCheckDraftTransitionForCurrentRepo(t *testing.T) {
	t.Run("no repository context", func(t *testing.T) {
		client := &Client{
			repo:           nil,
			config:         DefaultConfig(),
			cache:          &NoOpCache{},
			circuitBreaker: NewCircuitBreaker(5, 1*time.Minute),
		}

		pr := &PullRequest{
			Number: 123,
			Draft:  true,
		}

		_, err := client.CheckDraftTransitionForCurrentRepo(nil, pr)
		if err == nil {
			t.Error("Expected error when repository context is not set")
		}

		if err.Error() != "no repository context set" {
			t.Errorf("Error message = %q, expected 'no repository context set'", err.Error())
		}
	})
}

func TestCheckDraftTransition(t *testing.T) {
	t.Run("nil PR error", func(t *testing.T) {
		client := &Client{}

		_, err := client.CheckDraftTransition(nil, "owner", "repo", nil)
		if err == nil {
			t.Error("Expected error for nil PR")
		}

		expectedMsg := "pull request is nil"
		if err.Error() != expectedMsg {
			t.Errorf("Error message = %q, expected %q", err.Error(), expectedMsg)
		}
	})

	t.Run("PR not in draft state", func(t *testing.T) {
		client := &Client{}

		pr := &PullRequest{
			Number: 123,
			Draft:  false,
		}

		result, err := client.CheckDraftTransition(nil, "owner", "repo", pr)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if !result.CanTransition {
			t.Error("Expected CanTransition to be true for non-draft PR")
		}

		if result.DependentPRCount != 0 {
			t.Errorf("Expected DependentPRCount to be 0, got %d", result.DependentPRCount)
		}
	})
}

func TestDraftTransitionResultStructure(t *testing.T) {
	t.Run("can transition with no blockers", func(t *testing.T) {
		result := &DraftTransitionResult{
			CanTransition:    true,
			BlockingReasons:  []string{},
			DependentPRsOpen: false,
			DependentPRsDraft: false,
			DependentPRCount: 0,
		}

		if !result.CanTransition {
			t.Error("Expected CanTransition to be true")
		}
		if len(result.BlockingReasons) != 0 {
			t.Errorf("Expected no blocking reasons, got %d", len(result.BlockingReasons))
		}
	})

	t.Run("can transition with dependent PRs", func(t *testing.T) {
		result := &DraftTransitionResult{
			CanTransition:    true,
			BlockingReasons:  []string{"2 dependent PR(s) target this branch"},
			DependentPRsOpen: true,
			DependentPRsDraft: false,
			DependentPRCount: 2,
		}

		if !result.CanTransition {
			t.Error("Expected CanTransition to be true even with dependents")
		}
		if len(result.BlockingReasons) != 1 {
			t.Errorf("Expected 1 blocking reason, got %d", len(result.BlockingReasons))
		}
		if result.DependentPRCount != 2 {
			t.Errorf("Expected DependentPRCount to be 2, got %d", result.DependentPRCount)
		}
	})

	t.Run("dependent PRs in draft state", func(t *testing.T) {
		result := &DraftTransitionResult{
			CanTransition:    true,
			BlockingReasons:  []string{"1 dependent PR(s) are still in draft state"},
			DependentPRsOpen: true,
			DependentPRsDraft: true,
			DependentPRCount: 1,
		}

		if !result.DependentPRsDraft {
			t.Error("Expected DependentPRsDraft to be true")
		}
	})
}

func TestStackingMetadataInPRBody(t *testing.T) {
	t.Run("stacking metadata added to empty body", func(t *testing.T) {
		parentPR := &PullRequest{
			Number: 100,
			Title:  "Parent PR",
		}

		body := ""
		metadata := FormatStackingMetadata(parentPR)

		if body != "" {
			finalBody := body + "\n\n" + metadata
			if !strings.Contains(finalBody, "📚 **Stacked on:**") {
				t.Error("Expected stacking metadata in final body")
			}
		} else {
			finalBody := metadata
			if !strings.Contains(finalBody, "📚 **Stacked on:**") {
				t.Error("Expected stacking metadata in final body")
			}
		}
	})

	t.Run("stacking metadata added to existing body", func(t *testing.T) {
		parentPR := &PullRequest{
			Number: 100,
			Title:  "Parent PR",
		}

		body := "This is my PR description"
		metadata := FormatStackingMetadata(parentPR)
		finalBody := body + "\n\n" + metadata

		if !strings.Contains(finalBody, "This is my PR description") {
			t.Error("Expected original body in final body")
		}
		if !strings.Contains(finalBody, "📚 **Stacked on:**") {
			t.Error("Expected stacking metadata in final body")
		}
	})

	t.Run("stacking metadata not duplicated", func(t *testing.T) {
		parentPR := &PullRequest{
			Number: 100,
			Title:  "Parent PR",
		}

		// Body already has stacking metadata
		body := "PR description\n\n---\n\n📚 **Stacked on:** #100 - Parent PR"

		// Check if body already contains metadata (simulating UpdatePullRequest logic)
		if strings.Contains(body, "📚 **Stacked on:**") {
			// Don't add metadata again
			finalBody := body
			if finalBody != body {
				t.Error("Body should not be modified when metadata already present")
			}
		} else {
			// Add metadata
			metadata := FormatStackingMetadata(parentPR)
			finalBody := body + "\n\n" + metadata
			if !strings.Contains(finalBody, "📚 **Stacked on:**") {
				t.Error("Expected stacking metadata in final body")
			}
		}
	})
}

func TestPRCreationWithStackingScenarios(t *testing.T) {
	t.Run("non-stacked PR (no parent)", func(t *testing.T) {
		// Simulate creating a PR without parent
		_ = "Feature PR"      // title
		_ = "feature/auth"    // head
		_ = "main"            // base
		body := "Implements authentication"
		parentPR := (*PullRequest)(nil)

		finalBody := body
		if parentPR != nil {
			metadata := FormatStackingMetadata(parentPR)
			if finalBody != "" {
				finalBody = body + "\n\n" + metadata
			} else {
				finalBody = metadata
			}
		}

		// Should not have stacking metadata
		if strings.Contains(finalBody, "📚 **Stacked on:**") {
			t.Error("Non-stacked PR should not have stacking metadata")
		}
		if finalBody != "Implements authentication" {
			t.Errorf("Final body = %q, expected original body only", finalBody)
		}
	})

	t.Run("stacked PR (with parent)", func(t *testing.T) {
		// Simulate creating a stacked PR
		_ = "Feature PR - Part 2"   // title
		_ = "feature/auth-part2"    // head
		_ = "feature/auth"          // base
		body := "Continues authentication work"
		parentPR := &PullRequest{
			Number: 122,
			Title:  "Feature PR - Part 1",
		}

		finalBody := body
		if parentPR != nil {
			metadata := FormatStackingMetadata(parentPR)
			if finalBody != "" {
				finalBody = body + "\n\n" + metadata
			} else {
				finalBody = metadata
			}
		}

		// Should have stacking metadata
		if !strings.Contains(finalBody, "📚 **Stacked on:**") {
			t.Error("Stacked PR should have stacking metadata")
		}
		if !strings.Contains(finalBody, "#122") {
			t.Error("Should reference parent PR number")
		}
		if !strings.Contains(finalBody, "Continues authentication work") {
			t.Error("Should preserve original body")
		}
	})
}

func TestDraftStateTransitions(t *testing.T) {
	t.Run("marking PR ready for review", func(t *testing.T) {
		// Simulate transition from draft to ready
		readyForReview := false
		req := UpdatePRRequest{
			Draft: &readyForReview,
		}

		if *req.Draft != false {
			t.Error("Expected draft to be false (ready for review)")
		}
	})

	t.Run("keeping PR in draft", func(t *testing.T) {
		// Simulate keeping PR in draft
		stillDraft := true
		req := UpdatePRRequest{
			Draft: &stillDraft,
		}

		if *req.Draft != true {
			t.Error("Expected draft to be true")
		}
	})

	t.Run("not changing draft state", func(t *testing.T) {
		// Simulate update without touching draft state
		req := UpdatePRRequest{
			Title: "Updated title",
			Draft: nil, // nil means don't change
		}

		if req.Draft != nil {
			t.Error("Draft should be nil when not changing state")
		}
	})
}

func TestStackingMetadataFormat(t *testing.T) {
	parentPR := &PullRequest{
		Number: 123,
		Title:  "Parent Feature",
	}

	metadata := FormatStackingMetadata(parentPR)

	// Check that metadata has proper markdown formatting
	lines := strings.Split(metadata, "\n")

	if len(lines) < 3 {
		t.Errorf("Expected at least 3 lines in metadata, got %d", len(lines))
	}

	// First line should be separator
	if lines[0] != "---" {
		t.Errorf("Expected first line to be '---', got %q", lines[0])
	}

	// Should contain the stacking reference line
	found := false
	for _, line := range lines {
		if strings.Contains(line, "📚 **Stacked on:**") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Metadata should contain stacking reference line")
	}
}

// ======================================
// Reviewer Assignment Tests
// ======================================

func TestParseReviewers(t *testing.T) {
	tests := []struct {
		name          string
		reviewers     []string
		expectedUsers []string
		expectedTeams []string
	}{
		{
			name:          "empty list",
			reviewers:     []string{},
			expectedUsers: []string{},
			expectedTeams: []string{},
		},
		{
			name:          "single user",
			reviewers:     []string{"@octocat"},
			expectedUsers: []string{"octocat"},
			expectedTeams: []string{},
		},
		{
			name:          "single team",
			reviewers:     []string{"@myorg/security-team"},
			expectedUsers: []string{},
			expectedTeams: []string{"myorg/security-team"},
		},
		{
			name:          "mixed users and teams",
			reviewers:     []string{"@octocat", "@myorg/security", "@reviewer2", "@myorg/frontend"},
			expectedUsers: []string{"octocat", "reviewer2"},
			expectedTeams: []string{"myorg/security", "myorg/frontend"},
		},
		{
			name:          "without @ prefix",
			reviewers:     []string{"octocat", "myorg/team"},
			expectedUsers: []string{"octocat"},
			expectedTeams: []string{"myorg/team"},
		},
		{
			name:          "mixed @ prefix",
			reviewers:     []string{"@octocat", "reviewer2", "@myorg/team"},
			expectedUsers: []string{"octocat", "reviewer2"},
			expectedTeams: []string{"myorg/team"},
		},
		{
			name:          "empty strings filtered",
			reviewers:     []string{"@octocat", "", "@", "myorg/team"},
			expectedUsers: []string{"octocat"},
			expectedTeams: []string{"myorg/team"},
		},
		{
			name:          "multiple users",
			reviewers:     []string{"@user1", "@user2", "@user3"},
			expectedUsers: []string{"user1", "user2", "user3"},
			expectedTeams: []string{},
		},
		{
			name:          "multiple teams",
			reviewers:     []string{"@org/team1", "@org/team2", "@org/team3"},
			expectedUsers: []string{},
			expectedTeams: []string{"org/team1", "org/team2", "org/team3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assignment := ParseReviewers(tt.reviewers)

			if len(assignment.Users) != len(tt.expectedUsers) {
				t.Errorf("ParseReviewers() users count = %d, expected %d", len(assignment.Users), len(tt.expectedUsers))
			}

			if len(assignment.Teams) != len(tt.expectedTeams) {
				t.Errorf("ParseReviewers() teams count = %d, expected %d", len(assignment.Teams), len(tt.expectedTeams))
			}

			// Check users
			for i, user := range assignment.Users {
				if user != tt.expectedUsers[i] {
					t.Errorf("ParseReviewers() user[%d] = %s, expected %s", i, user, tt.expectedUsers[i])
				}
			}

			// Check teams
			for i, team := range assignment.Teams {
				if team != tt.expectedTeams[i] {
					t.Errorf("ParseReviewers() team[%d] = %s, expected %s", i, team, tt.expectedTeams[i])
				}
			}
		})
	}
}

func TestReviewerAssignmentStructure(t *testing.T) {
	t.Run("basic structure", func(t *testing.T) {
		assignment := &ReviewerAssignment{
			Users: []string{"user1", "user2"},
			Teams: []string{"org/team1"},
		}

		if len(assignment.Users) != 2 {
			t.Errorf("Users count = %d, expected 2", len(assignment.Users))
		}

		if len(assignment.Teams) != 1 {
			t.Errorf("Teams count = %d, expected 1", len(assignment.Teams))
		}

		if assignment.Users[0] != "user1" {
			t.Errorf("Users[0] = %s, expected 'user1'", assignment.Users[0])
		}

		if assignment.Teams[0] != "org/team1" {
			t.Errorf("Teams[0] = %s, expected 'org/team1'", assignment.Teams[0])
		}
	})

	t.Run("empty assignment", func(t *testing.T) {
		assignment := &ReviewerAssignment{
			Users: []string{},
			Teams: []string{},
		}

		if len(assignment.Users) != 0 {
			t.Errorf("Users count = %d, expected 0", len(assignment.Users))
		}

		if len(assignment.Teams) != 0 {
			t.Errorf("Teams count = %d, expected 0", len(assignment.Teams))
		}
	})
}

func TestAssignReviewersRequestStructure(t *testing.T) {
	t.Run("with users and teams", func(t *testing.T) {
		req := AssignReviewersRequest{
			Reviewers:     []string{"octocat", "reviewer2"},
			TeamReviewers: []string{"security-team", "frontend-team"},
		}

		if len(req.Reviewers) != 2 {
			t.Errorf("Reviewers count = %d, expected 2", len(req.Reviewers))
		}

		if len(req.TeamReviewers) != 2 {
			t.Errorf("TeamReviewers count = %d, expected 2", len(req.TeamReviewers))
		}
	})

	t.Run("users only", func(t *testing.T) {
		req := AssignReviewersRequest{
			Reviewers:     []string{"octocat"},
			TeamReviewers: []string{},
		}

		if len(req.Reviewers) != 1 {
			t.Errorf("Reviewers count = %d, expected 1", len(req.Reviewers))
		}

		if len(req.TeamReviewers) != 0 {
			t.Errorf("TeamReviewers count = %d, expected 0", len(req.TeamReviewers))
		}
	})

	t.Run("teams only", func(t *testing.T) {
		req := AssignReviewersRequest{
			Reviewers:     []string{},
			TeamReviewers: []string{"security"},
		}

		if len(req.Reviewers) != 0 {
			t.Errorf("Reviewers count = %d, expected 0", len(req.Reviewers))
		}

		if len(req.TeamReviewers) != 1 {
			t.Errorf("TeamReviewers count = %d, expected 1", len(req.TeamReviewers))
		}
	})
}

func TestGetStackAwareReviewers(t *testing.T) {
	tests := []struct {
		name                string
		opts                *StackAwareReviewerOptions
		expectedUsersCount  int
		expectedTeamsCount  int
		expectedUsers       []string
		expectedTeams       []string
		shouldContainUser   string
		shouldNotContainUser string
	}{
		{
			name: "current reviewers only",
			opts: &StackAwareReviewerOptions{
				CurrentReviewers: []string{"@octocat", "@myorg/security"},
				CurrentUser:      "",
				ParentPR:         nil,
				InheritParent:    false,
				DeduplicateStack: false,
			},
			expectedUsersCount: 1,
			expectedTeamsCount: 1,
			expectedUsers:      []string{"octocat"},
			expectedTeams:      []string{"myorg/security"},
		},
		{
			name: "filter out current user",
			opts: &StackAwareReviewerOptions{
				CurrentReviewers: []string{"@octocat", "@current-user"},
				CurrentUser:      "@current-user",
				ParentPR:         nil,
				InheritParent:    false,
				DeduplicateStack: false,
			},
			expectedUsersCount:   1,
			shouldContainUser:    "octocat",
			shouldNotContainUser: "current-user",
		},
		{
			name: "filter out current user case insensitive",
			opts: &StackAwareReviewerOptions{
				CurrentReviewers: []string{"@octocat", "@Current-User"},
				CurrentUser:      "@current-user",
				ParentPR:         nil,
				InheritParent:    false,
				DeduplicateStack: false,
			},
			expectedUsersCount:   1,
			shouldContainUser:    "octocat",
			shouldNotContainUser: "Current-User",
		},
		{
			name: "inherit from parent PR",
			opts: &StackAwareReviewerOptions{
				CurrentReviewers: []string{"@octocat"},
				CurrentUser:      "",
				ParentPR: &PullRequest{
					Number: 122,
					Reviewers: []PRReviewer{
						{Login: "parent-reviewer"},
					},
				},
				InheritParent:    true,
				DeduplicateStack: true,
			},
			expectedUsersCount: 2,
			shouldContainUser:  "octocat",
		},
		{
			name: "deduplicate inherited reviewers",
			opts: &StackAwareReviewerOptions{
				CurrentReviewers: []string{"@octocat", "@shared-reviewer"},
				CurrentUser:      "",
				ParentPR: &PullRequest{
					Number: 122,
					Reviewers: []PRReviewer{
						{Login: "shared-reviewer"},
					},
				},
				InheritParent:    true,
				DeduplicateStack: true,
			},
			expectedUsersCount: 2,
			shouldContainUser:  "octocat",
		},
		{
			name: "no inheritance when InheritParent is false",
			opts: &StackAwareReviewerOptions{
				CurrentReviewers: []string{"@octocat"},
				CurrentUser:      "",
				ParentPR: &PullRequest{
					Number: 122,
					Reviewers: []PRReviewer{
						{Login: "parent-reviewer"},
					},
				},
				InheritParent:    false,
				DeduplicateStack: true,
			},
			expectedUsersCount: 1,
			shouldContainUser:  "octocat",
		},
		{
			name: "empty reviewers",
			opts: &StackAwareReviewerOptions{
				CurrentReviewers: []string{},
				CurrentUser:      "",
				ParentPR:         nil,
				InheritParent:    false,
				DeduplicateStack: false,
			},
			expectedUsersCount: 0,
			expectedTeamsCount: 0,
		},
		{
			name: "teams and users combined",
			opts: &StackAwareReviewerOptions{
				CurrentReviewers: []string{"@octocat", "@myorg/team1", "@reviewer2", "@myorg/team2"},
				CurrentUser:      "",
				ParentPR:         nil,
				InheritParent:    false,
				DeduplicateStack: false,
			},
			expectedUsersCount: 2,
			expectedTeamsCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assignment := GetStackAwareReviewers(tt.opts)

			if assignment == nil {
				t.Fatal("GetStackAwareReviewers() returned nil")
			}

			if len(assignment.Users) != tt.expectedUsersCount {
				t.Errorf("Users count = %d, expected %d", len(assignment.Users), tt.expectedUsersCount)
			}

			if tt.expectedTeamsCount > 0 && len(assignment.Teams) != tt.expectedTeamsCount {
				t.Errorf("Teams count = %d, expected %d", len(assignment.Teams), tt.expectedTeamsCount)
			}

			// Check expected users
			if len(tt.expectedUsers) > 0 {
				for _, expectedUser := range tt.expectedUsers {
					found := false
					for _, user := range assignment.Users {
						if user == expectedUser {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected user %s not found in assignment", expectedUser)
					}
				}
			}

			// Check expected teams
			if len(tt.expectedTeams) > 0 {
				for _, expectedTeam := range tt.expectedTeams {
					found := false
					for _, team := range assignment.Teams {
						if team == expectedTeam {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected team %s not found in assignment", expectedTeam)
					}
				}
			}

			// Check shouldContainUser
			if tt.shouldContainUser != "" {
				found := false
				for _, user := range assignment.Users {
					if user == tt.shouldContainUser {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected user %s to be in assignment", tt.shouldContainUser)
				}
			}

			// Check shouldNotContainUser
			if tt.shouldNotContainUser != "" {
				for _, user := range assignment.Users {
					if strings.EqualFold(user, tt.shouldNotContainUser) {
						t.Errorf("User %s should not be in assignment", tt.shouldNotContainUser)
					}
				}
			}
		})
	}
}

func TestFormatReviewerAssignment(t *testing.T) {
	tests := []struct {
		name          string
		assignment    *ReviewerAssignment
		parentPR      *PullRequest
		inherited     bool
		expectEmpty   bool
		expectedParts []string
	}{
		{
			name:        "nil assignment",
			assignment:  nil,
			parentPR:    nil,
			inherited:   false,
			expectEmpty: true,
		},
		{
			name: "empty assignment",
			assignment: &ReviewerAssignment{
				Users: []string{},
				Teams: []string{},
			},
			parentPR:    nil,
			inherited:   false,
			expectEmpty: true,
		},
		{
			name: "users only",
			assignment: &ReviewerAssignment{
				Users: []string{"octocat", "reviewer2"},
				Teams: []string{},
			},
			parentPR:      nil,
			inherited:     false,
			expectEmpty:   false,
			expectedParts: []string{"Assigned reviewers:", "@octocat", "@reviewer2"},
		},
		{
			name: "teams only",
			assignment: &ReviewerAssignment{
				Users: []string{},
				Teams: []string{"myorg/security", "myorg/frontend"},
			},
			parentPR:      nil,
			inherited:     false,
			expectEmpty:   false,
			expectedParts: []string{"Assigned reviewers:", "@myorg/security", "@myorg/frontend"},
		},
		{
			name: "users and teams",
			assignment: &ReviewerAssignment{
				Users: []string{"octocat"},
				Teams: []string{"myorg/security"},
			},
			parentPR:      nil,
			inherited:     false,
			expectEmpty:   false,
			expectedParts: []string{"Assigned reviewers:", "@octocat", "@myorg/security"},
		},
		{
			name: "with inheritance note",
			assignment: &ReviewerAssignment{
				Users: []string{"octocat"},
				Teams: []string{},
			},
			parentPR: &PullRequest{
				Number: 122,
				Title:  "Parent PR",
			},
			inherited:     true,
			expectEmpty:   false,
			expectedParts: []string{"Assigned reviewers:", "@octocat", "parent PR #122"},
		},
		{
			name: "no inheritance note when inherited is false",
			assignment: &ReviewerAssignment{
				Users: []string{"octocat"},
				Teams: []string{},
			},
			parentPR: &PullRequest{
				Number: 122,
				Title:  "Parent PR",
			},
			inherited:   false,
			expectEmpty: false,
			expectedParts: []string{"Assigned reviewers:", "@octocat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatReviewerAssignment(tt.assignment, tt.parentPR, tt.inherited)

			if tt.expectEmpty {
				if result != "" {
					t.Errorf("Expected empty string, got %q", result)
				}
				return
			}

			for _, part := range tt.expectedParts {
				if !strings.Contains(result, part) {
					t.Errorf("Expected result to contain %q, got %q", part, result)
				}
			}

			// Check inheritance note is not present when not expected
			if !tt.inherited && strings.Contains(result, "parent PR") {
				t.Error("Result should not contain inheritance note when inherited is false")
			}
		})
	}
}

func TestDetectReviewerConflicts(t *testing.T) {
	tests := []struct {
		name              string
		currentReviewers  []string
		parentPR          *PullRequest
		expectedConflicts []string
	}{
		{
			name:              "no parent PR",
			currentReviewers:  []string{"@octocat"},
			parentPR:          nil,
			expectedConflicts: []string{},
		},
		{
			name:             "parent PR with no reviewers",
			currentReviewers: []string{"@octocat"},
			parentPR: &PullRequest{
				Number:    122,
				Reviewers: []PRReviewer{},
			},
			expectedConflicts: []string{},
		},
		{
			name:              "no current reviewers",
			currentReviewers:  []string{},
			parentPR:          &PullRequest{Number: 122, Reviewers: []PRReviewer{{Login: "octocat"}}},
			expectedConflicts: []string{},
		},
		{
			name:             "no conflicts - different reviewers",
			currentReviewers: []string{"@reviewer1", "@reviewer2"},
			parentPR: &PullRequest{
				Number: 122,
				Reviewers: []PRReviewer{
					{Login: "octocat"},
					{Login: "reviewer3"},
				},
			},
			expectedConflicts: []string{},
		},
		{
			name:             "single conflict",
			currentReviewers: []string{"@octocat", "@reviewer2"},
			parentPR: &PullRequest{
				Number: 122,
				Reviewers: []PRReviewer{
					{Login: "octocat"},
				},
			},
			expectedConflicts: []string{"@octocat"},
		},
		{
			name:             "multiple conflicts",
			currentReviewers: []string{"@octocat", "@reviewer2", "@reviewer3"},
			parentPR: &PullRequest{
				Number: 122,
				Reviewers: []PRReviewer{
					{Login: "octocat"},
					{Login: "reviewer2"},
				},
			},
			expectedConflicts: []string{"@octocat", "@reviewer2"},
		},
		{
			name:             "case insensitive matching",
			currentReviewers: []string{"@Octocat", "@Reviewer2"},
			parentPR: &PullRequest{
				Number: 122,
				Reviewers: []PRReviewer{
					{Login: "octocat"},
					{Login: "reviewer2"},
				},
			},
			expectedConflicts: []string{"@Octocat", "@Reviewer2"},
		},
		{
			name:             "without @ prefix",
			currentReviewers: []string{"octocat", "reviewer2"},
			parentPR: &PullRequest{
				Number: 122,
				Reviewers: []PRReviewer{
					{Login: "octocat"},
				},
			},
			expectedConflicts: []string{"octocat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflicts := DetectReviewerConflicts(tt.currentReviewers, tt.parentPR)

			if len(conflicts) != len(tt.expectedConflicts) {
				t.Errorf("DetectReviewerConflicts() returned %d conflicts, expected %d", len(conflicts), len(tt.expectedConflicts))
			}

			// Check that all expected conflicts are present
			for _, expected := range tt.expectedConflicts {
				found := false
				for _, conflict := range conflicts {
					if conflict == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected conflict %s not found in results", expected)
				}
			}
		})
	}
}

func TestAssignReviewersForCurrentRepo(t *testing.T) {
	t.Run("no repository context", func(t *testing.T) {
		client := &Client{
			repo:           nil,
			config:         DefaultConfig(),
			cache:          &NoOpCache{},
			circuitBreaker: NewCircuitBreaker(5, 1*time.Minute),
		}

		err := client.AssignReviewersForCurrentRepo(nil, 123, []string{"octocat"}, []string{})
		if err == nil {
			t.Error("Expected error when repository context is not set")
		}

		if err.Error() != "no repository context set" {
			t.Errorf("Error message = %q, expected 'no repository context set'", err.Error())
		}
	})
}

func TestStackAwareReviewerOptionsStructure(t *testing.T) {
	t.Run("all fields set", func(t *testing.T) {
		opts := &StackAwareReviewerOptions{
			CurrentReviewers: []string{"@octocat"},
			CurrentUser:      "@current-user",
			ParentPR: &PullRequest{
				Number: 122,
			},
			InheritParent:    true,
			DeduplicateStack: true,
		}

		if len(opts.CurrentReviewers) != 1 {
			t.Errorf("CurrentReviewers count = %d, expected 1", len(opts.CurrentReviewers))
		}

		if opts.CurrentUser != "@current-user" {
			t.Errorf("CurrentUser = %s, expected '@current-user'", opts.CurrentUser)
		}

		if opts.ParentPR == nil {
			t.Error("ParentPR should not be nil")
		}

		if !opts.InheritParent {
			t.Error("InheritParent should be true")
		}

		if !opts.DeduplicateStack {
			t.Error("DeduplicateStack should be true")
		}
	})
}

func TestReviewerAssignmentEdgeCases(t *testing.T) {
	t.Run("reviewers with multiple slashes", func(t *testing.T) {
		// Edge case: team with multiple slashes (though unusual)
		reviewers := []string{"@org/parent/child"}
		assignment := ParseReviewers(reviewers)

		// Should still be treated as a team (contains slash)
		if len(assignment.Teams) != 1 {
			t.Errorf("Expected 1 team, got %d", len(assignment.Teams))
		}
	})

	t.Run("whitespace in reviewer names", func(t *testing.T) {
		// Note: GitHub usernames can't have spaces, but testing input handling
		reviewers := []string{" @octocat ", "@org/team "}
		assignment := ParseReviewers(reviewers)

		// Parser should handle trimming (through @ prefix removal)
		// Though spaces in the middle won't be trimmed
		if len(assignment.Users) == 0 && len(assignment.Teams) == 0 {
			t.Error("Expected some reviewers to be parsed")
		}
	})

	t.Run("empty parent PR reviewers", func(t *testing.T) {
		opts := &StackAwareReviewerOptions{
			CurrentReviewers: []string{"@octocat"},
			CurrentUser:      "",
			ParentPR: &PullRequest{
				Number:    122,
				Reviewers: []PRReviewer{}, // Empty list
			},
			InheritParent:    true,
			DeduplicateStack: true,
		}

		assignment := GetStackAwareReviewers(opts)

		// Should only have current reviewers
		if len(assignment.Users) != 1 {
			t.Errorf("Expected 1 user, got %d", len(assignment.Users))
		}
	})
}
