package codeowners

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test CODEOWNERS file parsing
func TestParseCodeowners(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	t.Run("parses valid CODEOWNERS file", func(t *testing.T) {
		content := `# Comment line
*.go @golang-team
*.js @frontend-team @user1
/docs/ @doc-team
README.md @everyone
`
		githubDir := filepath.Join(tmpDir, ".github")
		os.MkdirAll(githubDir, 0755)
		codeownersPath := filepath.Join(githubDir, "CODEOWNERS")
		os.WriteFile(codeownersPath, []byte(content), 0644)

		co, err := ParseCodeowners(tmpDir)
		if err != nil {
			t.Fatalf("ParseCodeowners failed: %v", err)
		}

		if len(co.Rules) != 4 {
			t.Errorf("Expected 4 rules, got %d", len(co.Rules))
		}

		// Check first rule
		if co.Rules[0].Pattern != "*.go" {
			t.Errorf("Rule 0 pattern = %q, want %q", co.Rules[0].Pattern, "*.go")
		}
		if len(co.Rules[0].Owners) != 1 || co.Rules[0].Owners[0].Name != "@golang-team" {
			t.Errorf("Rule 0 owners incorrect: %v", co.Rules[0].Owners)
		}

		// Check rule with multiple owners
		if co.Rules[1].Pattern != "*.js" {
			t.Errorf("Rule 1 pattern = %q, want %q", co.Rules[1].Pattern, "*.js")
		}
		if len(co.Rules[1].Owners) != 2 {
			t.Errorf("Rule 1 owners count = %d, want 2", len(co.Rules[1].Owners))
		}
	})

	t.Run("handles missing CODEOWNERS file", func(t *testing.T) {
		emptyDir := filepath.Join(tmpDir, "empty")
		os.MkdirAll(emptyDir, 0755)

		co, err := ParseCodeowners(emptyDir)
		if err != nil {
			t.Fatalf("ParseCodeowners failed: %v", err)
		}

		if len(co.Rules) != 0 {
			t.Errorf("Expected 0 rules for missing file, got %d", len(co.Rules))
		}
	})

	t.Run("skips invalid lines", func(t *testing.T) {
		content := `*.go @golang-team
invalid-line-no-owner
*.js @frontend-team
`
		githubDir := filepath.Join(tmpDir, "skip-invalid", ".github")
		os.MkdirAll(githubDir, 0755)
		codeownersPath := filepath.Join(githubDir, "CODEOWNERS")
		os.WriteFile(codeownersPath, []byte(content), 0644)

		co, err := ParseCodeowners(filepath.Join(tmpDir, "skip-invalid"))
		if err != nil {
			t.Fatalf("ParseCodeowners failed: %v", err)
		}

		if len(co.Rules) != 2 {
			t.Errorf("Expected 2 valid rules, got %d", len(co.Rules))
		}
	})

	t.Run("finds CODEOWNERS in different locations", func(t *testing.T) {
		// Test .github/CODEOWNERS (already tested above)

		// Test docs/CODEOWNERS
		docsDir := filepath.Join(tmpDir, "docs-location", "docs")
		os.MkdirAll(docsDir, 0755)
		os.WriteFile(filepath.Join(docsDir, "CODEOWNERS"), []byte("*.md @docs-team"), 0644)

		co, err := ParseCodeowners(filepath.Join(tmpDir, "docs-location"))
		if err != nil {
			t.Fatalf("ParseCodeowners failed: %v", err)
		}
		if len(co.Rules) != 1 {
			t.Errorf("Expected 1 rule from docs/CODEOWNERS, got %d", len(co.Rules))
		}

		// Test root CODEOWNERS
		rootDir := filepath.Join(tmpDir, "root-location")
		os.MkdirAll(rootDir, 0755)
		os.WriteFile(filepath.Join(rootDir, "CODEOWNERS"), []byte("*.txt @root-team"), 0644)

		co, err = ParseCodeowners(rootDir)
		if err != nil {
			t.Fatalf("ParseCodeowners failed: %v", err)
		}
		if len(co.Rules) != 1 {
			t.Errorf("Expected 1 rule from root CODEOWNERS, got %d", len(co.Rules))
		}
	})
}

// Test rule parsing
func TestParseRule(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantPattern string
		wantOwners  int
		wantError   bool
	}{
		{
			name:        "simple rule",
			line:        "*.go @golang-team",
			wantPattern: "*.go",
			wantOwners:  1,
			wantError:   false,
		},
		{
			name:        "multiple owners",
			line:        "*.js @user1 @user2 @org/team",
			wantPattern: "*.js",
			wantOwners:  3,
			wantError:   false,
		},
		{
			name:        "directory pattern",
			line:        "/docs/ @doc-team",
			wantPattern: "/docs/",
			wantOwners:  1,
			wantError:   false,
		},
		{
			name:      "no owners",
			line:      "*.md",
			wantError: true,
		},
		{
			name:      "invalid owner (no @)",
			line:      "*.py invalidowner",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := parseRule(tt.line, 1)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("parseRule failed: %v", err)
			}

			if rule.Pattern != tt.wantPattern {
				t.Errorf("Pattern = %q, want %q", rule.Pattern, tt.wantPattern)
			}

			if len(rule.Owners) != tt.wantOwners {
				t.Errorf("Owners count = %d, want %d", len(rule.Owners), tt.wantOwners)
			}
		})
	}
}

// Test owner type detection
func TestOwnerType(t *testing.T) {
	tests := []struct {
		owner    string
		wantType string
	}{
		{"@user1", "user"},
		{"@org/team", "team"},
		{"@myorg/security-team", "team"},
	}

	for _, tt := range tests {
		t.Run(tt.owner, func(t *testing.T) {
			rule, err := parseRule("*.go "+tt.owner, 1)
			if err != nil {
				t.Fatalf("parseRule failed: %v", err)
			}

			if len(rule.Owners) == 0 {
				t.Fatal("No owners parsed")
			}

			if rule.Owners[0].Type != tt.wantType {
				t.Errorf("Owner type = %q, want %q", rule.Owners[0].Type, tt.wantType)
			}
		})
	}
}

// Test pattern matching
func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		filePath string
		want     bool
	}{
		// Exact matches
		{
			name:     "exact match",
			pattern:  "README.md",
			filePath: "README.md",
			want:     true,
		},
		{
			name:     "exact match nested",
			pattern:  "README.md",
			filePath: "docs/README.md",
			want:     true,
		},
		// Wildcard patterns
		{
			name:     "wildcard extension",
			pattern:  "*.go",
			filePath: "main.go",
			want:     true,
		},
		{
			name:     "wildcard extension nested",
			pattern:  "*.go",
			filePath: "pkg/util/helpers.go",
			want:     true,
		},
		{
			name:     "wildcard no match",
			pattern:  "*.go",
			filePath: "main.js",
			want:     false,
		},
		// Directory patterns
		{
			name:     "directory match",
			pattern:  "docs/",
			filePath: "docs/README.md",
			want:     true,
		},
		{
			name:     "directory no match",
			pattern:  "docs/",
			filePath: "src/main.go",
			want:     false,
		},
		// Root-relative patterns
		{
			name:     "root relative match",
			pattern:  "/config.yml",
			filePath: "config.yml",
			want:     true,
		},
		{
			name:     "root relative no match nested",
			pattern:  "/config.yml",
			filePath: "subdir/config.yml",
			want:     false,
		},
		// Double-star patterns
		{
			name:     "double star match",
			pattern:  "src/**/*.js",
			filePath: "src/components/Button.js",
			want:     true,
		},
		{
			name:     "double star nested match",
			pattern:  "src/**/*.js",
			filePath: "src/pages/home/index.js",
			want:     true,
		},
		{
			name:     "double star no match",
			pattern:  "src/**/*.js",
			filePath: "lib/util.js",
			want:     false,
		},
		// Complex patterns
		{
			name:     "pattern with path",
			pattern:  "src/*.go",
			filePath: "src/main.go",
			want:     true,
		},
		{
			name:     "pattern with path nested",
			pattern:  "src/*.go",
			filePath: "src/pkg/main.go",
			want:     false, // * doesn't match /, so this should not match nested files
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.pattern, tt.filePath)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.filePath, got, tt.want)
			}
		})
	}
}

// Test GetReviewers
func TestGetReviewers(t *testing.T) {
	co := &CodeOwners{
		Rules: []Rule{
			{Pattern: "*.go", Owners: []Owner{{Name: "@golang-team", Type: "team"}}},
			{Pattern: "*.js", Owners: []Owner{{Name: "@frontend-team", Type: "team"}}},
			{Pattern: "docs/", Owners: []Owner{{Name: "@doc-team", Type: "team"}}},
			{Pattern: "*.md", Owners: []Owner{{Name: "@user1", Type: "user"}, {Name: "@user2", Type: "user"}}},
		},
	}

	t.Run("single file match", func(t *testing.T) {
		changedFiles := []string{"main.go"}
		reviewers := co.GetReviewers(changedFiles, "")

		if len(reviewers) != 1 {
			t.Errorf("Expected 1 reviewer, got %d", len(reviewers))
		}
		if reviewers[0] != "@golang-team" {
			t.Errorf("Expected @golang-team, got %s", reviewers[0])
		}
	})

	t.Run("multiple files different patterns", func(t *testing.T) {
		changedFiles := []string{"main.go", "app.js"}
		reviewers := co.GetReviewers(changedFiles, "")

		if len(reviewers) != 2 {
			t.Errorf("Expected 2 reviewers, got %d", len(reviewers))
		}
	})

	t.Run("overlapping patterns", func(t *testing.T) {
		changedFiles := []string{"README.md", "docs/guide.md"}
		reviewers := co.GetReviewers(changedFiles, "")

		// Should have @doc-team (from docs/) and @user1, @user2 (from *.md)
		// But since later rules override, docs/guide.md should only match docs/
		if len(reviewers) < 1 {
			t.Errorf("Expected at least 1 reviewer, got %d", len(reviewers))
		}
	})

	t.Run("filters current user", func(t *testing.T) {
		changedFiles := []string{"README.md"}
		reviewers := co.GetReviewers(changedFiles, "@user1")

		// Should not include @user1
		for _, r := range reviewers {
			if strings.EqualFold(r, "@user1") {
				t.Error("Current user should be filtered out")
			}
		}
	})

	t.Run("no matching files", func(t *testing.T) {
		changedFiles := []string{"random.xyz"}
		reviewers := co.GetReviewers(changedFiles, "")

		if len(reviewers) != 0 {
			t.Errorf("Expected 0 reviewers, got %d", len(reviewers))
		}
	})
}

// Test ValidateOwner
func TestValidateOwner(t *testing.T) {
	tests := []struct {
		owner string
		want  bool
	}{
		{"@user1", true},
		{"@org/team", true},
		{"@my-org/security-team", true},
		{"user1", false}, // Missing @
		{"@", false},     // Empty name
		{"", false},      // Empty string
		{"@org/", false}, // Empty team name
		{"@/team", false}, // Empty org name
		{"@org/team/extra", false}, // Too many slashes
	}

	for _, tt := range tests {
		t.Run(tt.owner, func(t *testing.T) {
			got := ValidateOwner(tt.owner)
			if got != tt.want {
				t.Errorf("ValidateOwner(%q) = %v, want %v", tt.owner, got, tt.want)
			}
		})
	}
}

// Test DeduplicateReviewers
func TestDeduplicateReviewers(t *testing.T) {
	tests := []struct {
		name      string
		reviewers []string
		want      int
	}{
		{
			name:      "no duplicates",
			reviewers: []string{"@user1", "@user2", "@user3"},
			want:      3,
		},
		{
			name:      "with duplicates",
			reviewers: []string{"@user1", "@user2", "@user1", "@user3", "@user2"},
			want:      3,
		},
		{
			name:      "all duplicates",
			reviewers: []string{"@user1", "@user1", "@user1"},
			want:      1,
		},
		{
			name:      "empty list",
			reviewers: []string{},
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeduplicateReviewers(tt.reviewers)
			if len(result) != tt.want {
				t.Errorf("DeduplicateReviewers() length = %d, want %d", len(result), tt.want)
			}

			// Check that all elements are unique
			seen := make(map[string]bool)
			for _, r := range result {
				if seen[r] {
					t.Errorf("Duplicate reviewer in result: %s", r)
				}
				seen[r] = true
			}
		})
	}
}

// Test FilterCurrentUser
func TestFilterCurrentUser(t *testing.T) {
	tests := []struct {
		name        string
		reviewers   []string
		currentUser string
		want        int
		shouldHave  []string
	}{
		{
			name:        "filters exact match",
			reviewers:   []string{"@user1", "@user2", "@user3"},
			currentUser: "@user1",
			want:        2,
			shouldHave:  []string{"@user2", "@user3"},
		},
		{
			name:        "filters with auto-added @",
			reviewers:   []string{"@user1", "@user2", "@user3"},
			currentUser: "user1",
			want:        2,
			shouldHave:  []string{"@user2", "@user3"},
		},
		{
			name:        "case insensitive",
			reviewers:   []string{"@User1", "@user2"},
			currentUser: "@user1",
			want:        1,
			shouldHave:  []string{"@user2"},
		},
		{
			name:        "no match",
			reviewers:   []string{"@user1", "@user2"},
			currentUser: "@user3",
			want:        2,
			shouldHave:  []string{"@user1", "@user2"},
		},
		{
			name:        "empty current user",
			reviewers:   []string{"@user1", "@user2"},
			currentUser: "",
			want:        2,
			shouldHave:  []string{"@user1", "@user2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterCurrentUser(tt.reviewers, tt.currentUser)
			if len(result) != tt.want {
				t.Errorf("FilterCurrentUser() length = %d, want %d", len(result), tt.want)
			}

			for _, expected := range tt.shouldHave {
				found := false
				for _, r := range result {
					if strings.EqualFold(r, expected) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected reviewer %s not found in result", expected)
				}
			}
		})
	}
}

// Test GetOwnersForFile
func TestGetOwnersForFile(t *testing.T) {
	co := &CodeOwners{
		Rules: []Rule{
			{Pattern: "*.go", Owners: []Owner{{Name: "@golang-team", Type: "team"}}},
			{Pattern: "*.js", Owners: []Owner{{Name: "@frontend-team", Type: "team"}}},
			{Pattern: "src/**/*.go", Owners: []Owner{{Name: "@src-team", Type: "team"}}}, // More specific, later rule
		},
	}

	t.Run("basic match", func(t *testing.T) {
		owners := co.GetOwnersForFile("main.go")
		if len(owners) != 1 || owners[0].Name != "@golang-team" {
			t.Errorf("Expected @golang-team, got %v", owners)
		}
	})

	t.Run("later rule takes precedence", func(t *testing.T) {
		owners := co.GetOwnersForFile("src/pkg/util.go")
		// Should match the more specific src/**/*.go rule, not *.go
		if len(owners) != 1 || owners[0].Name != "@src-team" {
			t.Errorf("Expected @src-team (later rule), got %v", owners)
		}
	})

	t.Run("no match", func(t *testing.T) {
		owners := co.GetOwnersForFile("README.md")
		if len(owners) != 0 {
			t.Errorf("Expected no owners, got %v", owners)
		}
	})
}

// Test FindCodeownersFile
func TestFindCodeownersFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("finds in .github", func(t *testing.T) {
		githubDir := filepath.Join(tmpDir, "test1", ".github")
		os.MkdirAll(githubDir, 0755)
		codeownersPath := filepath.Join(githubDir, "CODEOWNERS")
		os.WriteFile(codeownersPath, []byte("*.go @team"), 0644)

		found, err := FindCodeownersFile(filepath.Join(tmpDir, "test1"))
		if err != nil {
			t.Fatalf("FindCodeownersFile failed: %v", err)
		}
		if found != codeownersPath {
			t.Errorf("Found path = %s, want %s", found, codeownersPath)
		}
	})

	t.Run("finds in docs", func(t *testing.T) {
		docsDir := filepath.Join(tmpDir, "test2", "docs")
		os.MkdirAll(docsDir, 0755)
		codeownersPath := filepath.Join(docsDir, "CODEOWNERS")
		os.WriteFile(codeownersPath, []byte("*.md @team"), 0644)

		found, err := FindCodeownersFile(filepath.Join(tmpDir, "test2"))
		if err != nil {
			t.Fatalf("FindCodeownersFile failed: %v", err)
		}
		if found != codeownersPath {
			t.Errorf("Found path = %s, want %s", found, codeownersPath)
		}
	})

	t.Run("finds in root", func(t *testing.T) {
		rootDir := filepath.Join(tmpDir, "test3")
		os.MkdirAll(rootDir, 0755)
		codeownersPath := filepath.Join(rootDir, "CODEOWNERS")
		os.WriteFile(codeownersPath, []byte("* @team"), 0644)

		found, err := FindCodeownersFile(rootDir)
		if err != nil {
			t.Fatalf("FindCodeownersFile failed: %v", err)
		}
		if found != codeownersPath {
			t.Errorf("Found path = %s, want %s", found, codeownersPath)
		}
	})

	t.Run("not found", func(t *testing.T) {
		emptyDir := filepath.Join(tmpDir, "test4")
		os.MkdirAll(emptyDir, 0755)

		_, err := FindCodeownersFile(emptyDir)
		if err == nil {
			t.Error("Expected error for missing file, got nil")
		}
	})
}

// Test HasCodeowners
func TestHasCodeowners(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("has CODEOWNERS", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "with-codeowners")
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "CODEOWNERS"), []byte("* @team"), 0644)

		if !HasCodeowners(dir) {
			t.Error("HasCodeowners() = false, want true")
		}
	})

	t.Run("no CODEOWNERS", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "without-codeowners")
		os.MkdirAll(dir, 0755)

		if HasCodeowners(dir) {
			t.Error("HasCodeowners() = true, want false")
		}
	})
}

// Benchmark tests
func BenchmarkMatchPattern(b *testing.B) {
	pattern := "src/**/*.go"
	filePath := "src/internal/pkg/util/helpers.go"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchPattern(pattern, filePath)
	}
}

func BenchmarkGetReviewers(b *testing.B) {
	co := &CodeOwners{
		Rules: []Rule{
			{Pattern: "*.go", Owners: []Owner{{Name: "@golang-team", Type: "team"}}},
			{Pattern: "*.js", Owners: []Owner{{Name: "@frontend-team", Type: "team"}}},
			{Pattern: "docs/", Owners: []Owner{{Name: "@doc-team", Type: "team"}}},
			{Pattern: "*.md", Owners: []Owner{{Name: "@user1", Type: "user"}, {Name: "@user2", Type: "user"}}},
		},
	}
	changedFiles := []string{"main.go", "app.js", "README.md", "docs/guide.md"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		co.GetReviewers(changedFiles, "")
	}
}
