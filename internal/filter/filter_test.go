package filter

import (
	"testing"
	"time"

	"github.com/serpro69/gh-arc/internal/github"
	"github.com/stretchr/testify/assert"
)

func TestFilterPullRequests_NoFilter(t *testing.T) {
	prs := []*github.PullRequest{
		{Number: 1, User: github.PRUser{Login: "alice"}},
		{Number: 2, User: github.PRUser{Login: "bob"}},
	}

	filtered := FilterPullRequests(prs, nil)
	assert.Equal(t, prs, filtered)
}

func TestFilterPullRequests_ByAuthor(t *testing.T) {
	prs := []*github.PullRequest{
		{Number: 1, User: github.PRUser{Login: "alice"}},
		{Number: 2, User: github.PRUser{Login: "bob"}},
		{Number: 3, User: github.PRUser{Login: "alice"}},
	}

	filter := &PRFilter{Author: "alice"}
	filtered := FilterPullRequests(prs, filter)

	assert.Len(t, filtered, 2)
	assert.Equal(t, 1, filtered[0].Number)
	assert.Equal(t, 3, filtered[1].Number)
}

func TestFilterPullRequests_ByAuthorMe(t *testing.T) {
	prs := []*github.PullRequest{
		{Number: 1, User: github.PRUser{Login: "alice"}},
		{Number: 2, User: github.PRUser{Login: "bob"}},
		{Number: 3, User: github.PRUser{Login: "alice"}},
	}

	filter := &PRFilter{Author: "me", CurrentUser: "alice"}
	filtered := FilterPullRequests(prs, filter)

	assert.Len(t, filtered, 2)
	assert.Equal(t, 1, filtered[0].Number)
	assert.Equal(t, 3, filtered[1].Number)
}

func TestFilterPullRequests_ByStatus(t *testing.T) {
	prs := []*github.PullRequest{
		{
			Number: 1,
			Reviews: []github.PRReview{
				{State: "APPROVED"},
			},
		},
		{
			Number: 2,
			Reviews: []github.PRReview{
				{State: "CHANGES_REQUESTED"},
			},
		},
		{
			Number:  3,
			Reviews: []github.PRReview{},
		},
	}

	filter := &PRFilter{Status: "approved"}
	filtered := FilterPullRequests(prs, filter)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 1, filtered[0].Number)
}

func TestFilterPullRequests_ByBranch(t *testing.T) {
	prs := []*github.PullRequest{
		{Number: 1, Head: github.PRBranch{Ref: "feature/auth"}},
		{Number: 2, Head: github.PRBranch{Ref: "bugfix/login"}},
		{Number: 3, Head: github.PRBranch{Ref: "feature/api"}},
	}

	filter := &PRFilter{Branch: "feature/*"}
	filtered := FilterPullRequests(prs, filter)

	assert.Len(t, filtered, 2)
	assert.Equal(t, 1, filtered[0].Number)
	assert.Equal(t, 3, filtered[1].Number)
}

func TestFilterPullRequests_MultipleCriteria(t *testing.T) {
	prs := []*github.PullRequest{
		{
			Number: 1,
			User:   github.PRUser{Login: "alice"},
			Head:   github.PRBranch{Ref: "feature/auth"},
			Reviews: []github.PRReview{
				{State: "APPROVED"},
			},
		},
		{
			Number: 2,
			User:   github.PRUser{Login: "alice"},
			Head:   github.PRBranch{Ref: "bugfix/login"},
			Reviews: []github.PRReview{
				{State: "APPROVED"},
			},
		},
		{
			Number: 3,
			User:   github.PRUser{Login: "bob"},
			Head:   github.PRBranch{Ref: "feature/api"},
			Reviews: []github.PRReview{
				{State: "APPROVED"},
			},
		},
	}

	filter := &PRFilter{
		Author: "alice",
		Branch: "feature/*",
		Status: "approved",
	}
	filtered := FilterPullRequests(prs, filter)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 1, filtered[0].Number)
}

func TestMatchesStatus(t *testing.T) {
	tests := []struct {
		name         string
		status       github.PRStatus
		filterStatus string
		expected     bool
	}{
		{
			name:         "approved matches",
			status:       github.PRStatus{ReviewStatus: "approved"},
			filterStatus: "approved",
			expected:     true,
		},
		{
			name:         "changes_requested matches",
			status:       github.PRStatus{ReviewStatus: "changes_requested"},
			filterStatus: "changes_requested",
			expected:     true,
		},
		{
			name:         "review_required matches",
			status:       github.PRStatus{ReviewStatus: "review_required"},
			filterStatus: "review_required",
			expected:     true,
		},
		{
			name:         "pending matches",
			status:       github.PRStatus{ReviewStatus: "pending"},
			filterStatus: "pending",
			expected:     true,
		},
		{
			name:         "commented matches",
			status:       github.PRStatus{ReviewStatus: "commented"},
			filterStatus: "commented",
			expected:     true,
		},
		{
			name:         "status mismatch",
			status:       github.PRStatus{ReviewStatus: "approved"},
			filterStatus: "changes_requested",
			expected:     false,
		},
		{
			name:         "invalid filter status",
			status:       github.PRStatus{ReviewStatus: "approved"},
			filterStatus: "invalid",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesStatus(tt.status, tt.filterStatus)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesBranch(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		pattern  string
		expected bool
	}{
		{
			name:     "exact match",
			branch:   "feature/auth",
			pattern:  "feature/auth",
			expected: true,
		},
		{
			name:     "wildcard match all",
			branch:   "feature/auth",
			pattern:  "feature/*",
			expected: true,
		},
		{
			name:     "wildcard match prefix",
			branch:   "feature-123",
			pattern:  "feature*",
			expected: true,
		},
		{
			name:     "wildcard no match",
			branch:   "bugfix/login",
			pattern:  "feature/*",
			expected: false,
		},
		{
			name:     "question mark wildcard",
			branch:   "feature-1",
			pattern:  "feature-?",
			expected: true,
		},
		{
			name:     "case insensitive exact",
			branch:   "Feature/Auth",
			pattern:  "feature/auth",
			expected: false, // filepath.Match is case-sensitive
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesBranch(tt.branch, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesDraft(t *testing.T) {
	tests := []struct {
		name         string
		pr           *github.PullRequest
		filterStatus string
		expected     bool
	}{
		{
			name:         "draft PR with draft filter",
			pr:           &github.PullRequest{Draft: true},
			filterStatus: "draft",
			expected:     true,
		},
		{
			name:         "non-draft PR with draft filter",
			pr:           &github.PullRequest{Draft: false},
			filterStatus: "draft",
			expected:     false,
		},
		{
			name:         "draft PR without draft filter",
			pr:           &github.PullRequest{Draft: true},
			filterStatus: "approved",
			expected:     true,
		},
		{
			name:         "non-draft PR without draft filter",
			pr:           &github.PullRequest{Draft: false},
			filterStatus: "approved",
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesDraft(tt.pr, tt.filterStatus)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterPullRequests_EmptyResult(t *testing.T) {
	prs := []*github.PullRequest{
		{Number: 1, User: github.PRUser{Login: "alice"}},
		{Number: 2, User: github.PRUser{Login: "bob"}},
	}

	filter := &PRFilter{Author: "charlie"}
	filtered := FilterPullRequests(prs, filter)

	assert.Empty(t, filtered)
}

func TestFilterPullRequests_CaseInsensitiveAuthor(t *testing.T) {
	prs := []*github.PullRequest{
		{Number: 1, User: github.PRUser{Login: "Alice"}},
		{Number: 2, User: github.PRUser{Login: "bob"}},
	}

	filter := &PRFilter{Author: "alice"}
	filtered := FilterPullRequests(prs, filter)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 1, filtered[0].Number)
}

func createTestPR(number int, author string, branch string, reviewState string, draft bool) *github.PullRequest {
	pr := &github.PullRequest{
		Number:    number,
		Title:     "Test PR",
		State:     "open",
		Draft:     draft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		User:      github.PRUser{Login: author},
		Head:      github.PRBranch{Ref: branch},
		Base:      github.PRBranch{Ref: "main"},
	}

	if reviewState != "" {
		pr.Reviews = []github.PRReview{
			{State: reviewState},
		}
	}

	return pr
}

func TestFilterPullRequests_ComplexScenario(t *testing.T) {
	prs := []*github.PullRequest{
		createTestPR(1, "alice", "feature/auth", "APPROVED", false),
		createTestPR(2, "alice", "feature/api", "CHANGES_REQUESTED", false),
		createTestPR(3, "bob", "feature/ui", "APPROVED", false),
		createTestPR(4, "alice", "bugfix/login", "APPROVED", false),
		createTestPR(5, "charlie", "feature/db", "", true),
	}

	tests := []struct {
		name     string
		filter   *PRFilter
		expected []int // PR numbers
	}{
		{
			name:     "alice's feature branches",
			filter:   &PRFilter{Author: "alice", Branch: "feature/*"},
			expected: []int{1, 2},
		},
		{
			name:     "approved PRs",
			filter:   &PRFilter{Status: "approved"},
			expected: []int{1, 3, 4},
		},
		{
			name:     "alice's approved PRs",
			filter:   &PRFilter{Author: "alice", Status: "approved"},
			expected: []int{1, 4},
		},
		{
			name:     "feature branches",
			filter:   &PRFilter{Branch: "feature/*"},
			expected: []int{1, 2, 3, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterPullRequests(prs, tt.filter)
			assert.Len(t, filtered, len(tt.expected))

			numbers := make([]int, len(filtered))
			for i, pr := range filtered {
				numbers[i] = pr.Number
			}
			assert.Equal(t, tt.expected, numbers)
		})
	}
}
