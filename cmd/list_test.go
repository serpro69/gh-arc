package cmd

import (
	"testing"
)

func TestListCommand(t *testing.T) {
	t.Run("command initialization", func(t *testing.T) {
		if listCmd.Use != "list" {
			t.Errorf("Expected Use to be 'list', got '%s'", listCmd.Use)
		}

		if listCmd.Short == "" {
			t.Error("Expected Short description to be set")
		}

		if listCmd.Long == "" {
			t.Error("Expected Long description to be set")
		}

		if listCmd.RunE == nil {
			t.Error("Expected RunE to be set")
		}
	})

	t.Run("list command is registered", func(t *testing.T) {
		// Check if list command is added to root
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "list" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected list command to be registered with root command")
		}
	})

	t.Run("flags are defined", func(t *testing.T) {
		flags := []string{"author", "status", "branch", "no-cache"}

		for _, flagName := range flags {
			flag := listCmd.Flags().Lookup(flagName)
			if flag == nil {
				t.Errorf("Expected flag --%s to be defined", flagName)
			}
		}
	})

	t.Run("flag shortcuts are defined", func(t *testing.T) {
		shortcuts := map[string]string{
			"a": "author",
			"s": "status",
			"b": "branch",
		}

		for shortcut, fullName := range shortcuts {
			flag := listCmd.Flags().ShorthandLookup(shortcut)
			if flag == nil {
				t.Errorf("Expected shortcut -%s to be defined for --%s", shortcut, fullName)
			}
			if flag != nil && flag.Name != fullName {
				t.Errorf("Expected shortcut -%s to map to --%s, got --%s", shortcut, fullName, flag.Name)
			}
		}
	})
}

func TestListFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantAuthor string
		wantStatus string
		wantBranch string
		wantCache  bool
		wantErr    bool
	}{
		{
			name:       "no flags",
			args:       []string{},
			wantAuthor: "",
			wantStatus: "",
			wantBranch: "",
			wantCache:  true,
			wantErr:    false,
		},
		{
			name:       "author flag",
			args:       []string{"--author", "octocat"},
			wantAuthor: "octocat",
			wantStatus: "",
			wantBranch: "",
			wantCache:  true,
			wantErr:    false,
		},
		{
			name:       "author flag shorthand",
			args:       []string{"-a", "octocat"},
			wantAuthor: "octocat",
			wantStatus: "",
			wantBranch: "",
			wantCache:  true,
			wantErr:    false,
		},
		{
			name:       "status flag",
			args:       []string{"--status", "approved"},
			wantAuthor: "",
			wantStatus: "approved",
			wantBranch: "",
			wantCache:  true,
			wantErr:    false,
		},
		{
			name:       "branch flag",
			args:       []string{"--branch", "feature/*"},
			wantAuthor: "",
			wantStatus: "",
			wantBranch: "feature/*",
			wantCache:  true,
			wantErr:    false,
		},
		{
			name:       "no-cache flag",
			args:       []string{"--no-cache"},
			wantAuthor: "",
			wantStatus: "",
			wantBranch: "",
			wantCache:  false,
			wantErr:    false,
		},
		{
			name:       "all flags combined",
			args:       []string{"--author", "octocat", "--status", "approved", "--branch", "main", "--no-cache"},
			wantAuthor: "octocat",
			wantStatus: "approved",
			wantBranch: "main",
			wantCache:  false,
			wantErr:    false,
		},
		{
			name:       "all flags combined with shortcuts",
			args:       []string{"-a", "octocat", "-s", "approved", "-b", "main", "--no-cache"},
			wantAuthor: "octocat",
			wantStatus: "approved",
			wantBranch: "main",
			wantCache:  false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags to default values
			listAuthor = ""
			listStatus = ""
			listBranch = ""
			listNoCache = false

			// Reset command flags
			listCmd.Flags().Parse(tt.args)

			// Check parsed values
			if listAuthor != tt.wantAuthor {
				t.Errorf("Expected author '%s', got '%s'", tt.wantAuthor, listAuthor)
			}

			if listStatus != tt.wantStatus {
				t.Errorf("Expected status '%s', got '%s'", tt.wantStatus, listStatus)
			}

			if listBranch != tt.wantBranch {
				t.Errorf("Expected branch '%s', got '%s'", tt.wantBranch, listBranch)
			}

			wantNoCache := !tt.wantCache
			if listNoCache != wantNoCache {
				t.Errorf("Expected no-cache %t, got %t", wantNoCache, listNoCache)
			}
		})
	}
}

func TestGetAuthorFilter(t *testing.T) {
	tests := []struct {
		name       string
		author     string
		wantResult string
	}{
		{
			name:       "empty author returns all",
			author:     "",
			wantResult: "all",
		},
		{
			name:       "specific author",
			author:     "octocat",
			wantResult: "octocat",
		},
		{
			name:       "me keyword",
			author:     "me",
			wantResult: "me",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listAuthor = tt.author
			result := getAuthorFilter()
			if result != tt.wantResult {
				t.Errorf("Expected '%s', got '%s'", tt.wantResult, result)
			}
		})
	}
}

func TestGetStatusFilter(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		wantResult string
	}{
		{
			name:       "empty status returns all",
			status:     "",
			wantResult: "all",
		},
		{
			name:       "draft status",
			status:     "draft",
			wantResult: "draft",
		},
		{
			name:       "approved status",
			status:     "approved",
			wantResult: "approved",
		},
		{
			name:       "changes_requested status",
			status:     "changes_requested",
			wantResult: "changes_requested",
		},
		{
			name:       "review_required status",
			status:     "review_required",
			wantResult: "review_required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listStatus = tt.status
			result := getStatusFilter()
			if result != tt.wantResult {
				t.Errorf("Expected '%s', got '%s'", tt.wantResult, result)
			}
		})
	}
}

func TestGetBranchFilter(t *testing.T) {
	tests := []struct {
		name       string
		branch     string
		wantResult string
	}{
		{
			name:       "empty branch returns all",
			branch:     "",
			wantResult: "all",
		},
		{
			name:       "specific branch",
			branch:     "main",
			wantResult: "main",
		},
		{
			name:       "branch with wildcard",
			branch:     "feature/*",
			wantResult: "feature/*",
		},
		{
			name:       "branch with multiple wildcards",
			branch:     "*/feature-*",
			wantResult: "*/feature-*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listBranch = tt.branch
			result := getBranchFilter()
			if result != tt.wantResult {
				t.Errorf("Expected '%s', got '%s'", tt.wantResult, result)
			}
		})
	}
}
