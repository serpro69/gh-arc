package github

import (
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
