package diff

import (
	"testing"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/git"
	"github.com/serpro69/gh-arc/internal/github"
)

func TestNewContinueModeExecutor(t *testing.T) {
	repo := &git.Repository{}
	client := &github.Client{}
	cfg := &config.DiffConfig{
		AutoCreateBranchFromMain: true,
	}
	owner := "testowner"
	name := "testrepo"

	executor := NewContinueModeExecutor(repo, client, cfg, owner, name)

	if executor == nil {
		t.Fatal("NewContinueModeExecutor() returned nil")
	}

	if executor.repo != repo {
		t.Error("repo not set correctly")
	}

	if executor.client != client {
		t.Error("client not set correctly")
	}

	if executor.config != cfg {
		t.Error("config not set correctly")
	}

	if executor.owner != owner {
		t.Errorf("owner = %s, expected %s", executor.owner, owner)
	}

	if executor.name != name {
		t.Errorf("name = %s, expected %s", executor.name, name)
	}

	if executor.autoBranchDetector == nil {
		t.Error("autoBranchDetector not initialized")
	}

	// Verify autoBranchDetector is properly configured
	if executor.autoBranchDetector.repo != repo {
		t.Error("autoBranchDetector.repo not set correctly")
	}

	if executor.autoBranchDetector.config != cfg {
		t.Error("autoBranchDetector.config not set correctly")
	}
}

// Note: Integration tests for handleAutoBranch and Execute with auto-branch scenarios
// are covered by E2E tests in test/test-e2e.sh:
// - test_e2e_auto_branch_with_continue: Tests auto-branch + continue mode flow
// - test_e2e_auto_branch_creation: Tests basic auto-branch detection
//
// These E2E tests provide comprehensive coverage including:
// - Auto-branch detection when user is on main with commits
// - Template validation failure before auto-branch execution
// - Auto-branch execution during continue mode
// - PR creation with correct head/base branches
// - Checkout to auto-created branch
//
// Unit testing this functionality requires complex mocking of:
// - git.Repository (10+ methods)
// - github.Client (5+ methods)
// - template.FindSavedTemplates (file I/O)
// - os.Remove (file operations)
//
// The E2E tests provide better coverage and confidence for this cross-cutting feature.
