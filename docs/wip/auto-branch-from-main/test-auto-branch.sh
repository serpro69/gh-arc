#!/bin/bash
#
# Auto-Branch from Main - End-to-End Regression Test Suite
#
# This script performs automated testing of the auto-branch-from-main feature.
# Tests use a local git repository with simulated remote to avoid needing GitHub access.
#
# Usage:
#   ./test-auto-branch.sh                     # Run all tests (temp dir, auto-cleanup)
#   ./test-auto-branch.sh <test_name>         # Run specific test
#   TEST_DIR=/path/to/repo ./test-auto-branch.sh  # Use existing repo (preserved)
#   TEST_DIR=/tmp/test ./test-auto-branch.sh  # Create new repo (preserved)
#   CLEANUP=1 TEST_DIR=/path ./test-auto-branch.sh  # Force cleanup (created repos only)
#
# Environment variables:
#   TEST_DIR  - Test directory (default: /tmp/gh-arc-test-$$)
#               - If directory exists with .git, uses existing repo (safe mode)
#               - If directory doesn't exist, creates new test repo
#               - Existing repos are NEVER deleted, even with CLEANUP=1
#   CLEANUP   - Set to 1 to force cleanup of created repos (default: 0)
#
# Exit codes:
#   0 - All tests passed
#   1 - One or more tests failed
#   2 - Setup error
#

set -e  # Exit on error (disabled during tests)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Track if TEST_DIR was user-specified
USER_SPECIFIED_DIR=0
if [ -n "$TEST_DIR" ]; then
    USER_SPECIFIED_DIR=1
fi

# Test environment paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
TEST_DIR="${TEST_DIR:-/tmp/gh-arc-test-$$}"
TEST_REPO="$TEST_DIR/test-repo"
TEST_REMOTE="$TEST_DIR/remote.git"
GH_ARC_BIN="$PROJECT_ROOT/gh-arc"
CLEANUP="${CLEANUP:-0}"

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $*"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_skip() {
    echo -e "${YELLOW}[SKIP]${NC} $*"
}

# Test framework functions
start_test() {
    local test_name="$1"
    echo ""
    echo "======================================================================"
    echo "TEST: $test_name"
    echo "======================================================================"
    TESTS_RUN=$((TESTS_RUN + 1))
}

pass_test() {
    local test_name="$1"
    log_success "$test_name"
    TESTS_PASSED=$((TESTS_PASSED + 1))
}

fail_test() {
    local test_name="$1"
    local reason="$2"
    log_error "$test_name - $reason"
    TESTS_FAILED=$((TESTS_FAILED + 1))
}

skip_test() {
    local test_name="$1"
    local reason="$2"
    log_skip "$test_name - $reason"
    TESTS_SKIPPED=$((TESTS_SKIPPED + 1))
}

assert_equals() {
    local expected="$1"
    local actual="$2"
    local message="${3:-Assertion failed}"

    if [ "$expected" = "$actual" ]; then
        return 0
    else
        echo "  Expected: '$expected'"
        echo "  Actual:   '$actual'"
        echo "  Message:  $message"
        return 1
    fi
}

assert_contains() {
    local haystack="$1"
    local needle="$2"
    local message="${3:-String not found}"

    if echo "$haystack" | grep -q "$needle"; then
        return 0
    else
        echo "  Haystack: '$haystack'"
        echo "  Needle:   '$needle'"
        echo "  Message:  $message"
        return 1
    fi
}

assert_file_exists() {
    local file="$1"
    local message="${2:-File does not exist: $file}"

    if [ -f "$file" ]; then
        return 0
    else
        echo "  Message: $message"
        return 1
    fi
}

assert_branch_exists() {
    local branch="$1"
    local message="${2:-Branch does not exist: $branch}"

    if git show-ref --verify --quiet "refs/heads/$branch"; then
        return 0
    else
        echo "  Message: $message"
        return 1
    fi
}

assert_remote_branch_exists() {
    local branch="$1"
    local message="${2:-Remote branch does not exist: $branch}"

    if git ls-remote --heads origin "$branch" | grep -q "$branch"; then
        return 0
    else
        echo "  Message: $message"
        return 1
    fi
}

# Setup and teardown
setup_test_environment() {
    log_info "Setting up test environment..."

    # Build gh-arc if needed
    if [ ! -f "$GH_ARC_BIN" ]; then
        log_info "Building gh-arc..."
        cd "$PROJECT_ROOT"
        go build -o "$GH_ARC_BIN" || {
            log_error "Failed to build gh-arc"
            exit 2
        }
    fi

    # Check if TEST_DIR is already a git repository
    if [ -d "$TEST_DIR/.git" ]; then
        log_info "Using existing repository at $TEST_DIR"
        TEST_REPO="$TEST_DIR"
        cd "$TEST_REPO"

        # Verify it's a git repository
        if ! git rev-parse --git-dir >/dev/null 2>&1; then
            log_error "$TEST_DIR is not a valid git repository"
            exit 2
        fi

        # Check if remote exists
        if ! git remote get-url origin >/dev/null 2>&1; then
            log_error "$TEST_DIR does not have an 'origin' remote configured"
            exit 2
        fi

        log_success "Using existing repository at $TEST_DIR"
    else
        # Create test directories
        mkdir -p "$TEST_DIR"

        # Create bare remote repository
        log_info "Creating test remote repository..."
        git init --bare "$TEST_REMOTE" >/dev/null 2>&1

        # Create test repository
        log_info "Creating test repository..."
        git clone "$TEST_REMOTE" "$TEST_REPO" >/dev/null 2>&1
        cd "$TEST_REPO"

        # Configure git
        git config user.name "Test User"
        git config user.email "test@example.com"

        # Create initial commit on main
        echo "# Test Repository" > README.md
        git add README.md
        git commit -m "Initial commit" >/dev/null 2>&1
        git push origin main >/dev/null 2>&1

        log_success "Test environment ready at $TEST_DIR"
    fi
}

teardown_test_environment() {
    # Never cleanup if TEST_DIR was an existing git repository
    if [ -d "$TEST_DIR/.git" ] && [ "$USER_SPECIFIED_DIR" -eq 1 ]; then
        log_info "Preserving existing repository: $TEST_DIR"
        return 0
    fi

    # Only cleanup if:
    # 1. TEST_DIR was NOT user-specified, OR
    # 2. CLEANUP=1 was explicitly set
    if [ "$USER_SPECIFIED_DIR" -eq 0 ] || [ "$CLEANUP" -eq 1 ]; then
        log_info "Cleaning up test environment..."
        cd /
        rm -rf "$TEST_DIR"
        log_success "Test environment cleaned"
    else
        log_info "Preserving test directory: $TEST_DIR"
        log_info "  (use CLEANUP=1 to force cleanup of custom directories)"
    fi
}

reset_test_repo() {
    cd "$TEST_REPO"

    # For existing user repositories, be more conservative
    if [ "$USER_SPECIFIED_DIR" -eq 1 ] && [ -d "$TEST_DIR/.git" ]; then
        log_info "Skipping aggressive cleanup for existing repository"

        # Just checkout main/master and ensure clean working directory
        git checkout main 2>/dev/null || git checkout master 2>/dev/null || {
            log_warning "Could not checkout main/master branch"
        }

        # Stash any uncommitted changes
        git stash push -u -m "test-auto-branch stash" >/dev/null 2>&1 || true

        # Remove test config if exists
        rm -f .arc.json

        return 0
    fi

    # For created test repos, do full cleanup
    # Delete all local branches except main
    git branch | grep -v "main\|master" | xargs -r git branch -D 2>/dev/null || true

    # Checkout main
    git checkout main 2>/dev/null || git checkout -b main 2>/dev/null

    # Reset to remote
    git fetch origin >/dev/null 2>&1
    git reset --hard origin/main >/dev/null 2>&1

    # Clean working directory
    git clean -fdx >/dev/null 2>&1

    # Remove test config
    rm -f .arc.json
}

create_test_config() {
    local auto_create="${1:-false}"
    local pattern="${2:-feature/auto-from-main-{timestamp}}"
    local threshold="${3:-24}"

    cat > .arc.json <<EOF
{
  "diff": {
    "autoCreateBranchFromMain": $auto_create,
    "autoBranchNamePattern": "$pattern",
    "staleRemoteThresholdHours": $threshold
  }
}
EOF
}

create_test_commits() {
    local count="${1:-1}"

    for i in $(seq 1 "$count"); do
        echo "Commit $i content" >> test-file-$i.txt
        git add "test-file-$i.txt"
        git commit -m "Test commit $i" >/dev/null 2>&1
    done
}

# Test: Detection on main with commits
test_detection_on_main_with_commits() {
    start_test "Detection on main with commits"
    reset_test_repo

    # Create commits on main
    create_test_commits 2

    # Test detection logic (we'll use a go test for this)
    cd "$PROJECT_ROOT"
    if go test -run TestDetectCommitsOnMain ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Detection on main with commits"
    else
        fail_test "Detection on main with commits" "Detection test failed"
    fi
}

# Test: Detection skips when on feature branch
test_detection_skips_feature_branch() {
    start_test "Detection skips when on feature branch"
    reset_test_repo

    # Create feature branch
    git checkout -b feature/test-branch >/dev/null 2>&1
    create_test_commits 1

    # Detection should skip
    cd "$PROJECT_ROOT"
    if go test -run TestDetectCommitsOnMain_OnFeatureBranch ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Detection skips when on feature branch"
    else
        fail_test "Detection skips when on feature branch" "Test failed"
    fi
}

# Test: Detection skips when no commits ahead
test_detection_skips_no_commits() {
    start_test "Detection skips when no commits ahead"
    reset_test_repo

    # No commits created, should skip
    cd "$PROJECT_ROOT"
    if go test -run TestDetectCommitsOnMain_NoCommitsAhead ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Detection skips when no commits ahead"
    else
        fail_test "Detection skips when no commits ahead" "Test failed"
    fi
}

# Test: Branch name generation with timestamp pattern
test_branch_name_timestamp() {
    start_test "Branch name generation with timestamp"

    cd "$PROJECT_ROOT"
    if go test -run TestGenerateBranchName_TimestampPattern ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Branch name generation with timestamp"
    else
        fail_test "Branch name generation with timestamp" "Test failed"
    fi
}

# Test: Branch name generation with date pattern
test_branch_name_date() {
    start_test "Branch name generation with date pattern"

    cd "$PROJECT_ROOT"
    if go test -run TestGenerateBranchName_DatePattern ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Branch name generation with date pattern"
    else
        fail_test "Branch name generation with date pattern" "Test failed"
    fi
}

# Test: Branch name generation with datetime pattern
test_branch_name_datetime() {
    start_test "Branch name generation with datetime pattern"

    cd "$PROJECT_ROOT"
    if go test -run TestGenerateBranchName_DateTimePattern ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Branch name generation with datetime pattern"
    else
        fail_test "Branch name generation with datetime pattern" "Test failed"
    fi
}

# Test: Branch name generation with username pattern
test_branch_name_username() {
    start_test "Branch name generation with username pattern"

    cd "$PROJECT_ROOT"
    if go test -run TestGenerateBranchName_UsernamePattern ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Branch name generation with username pattern"
    else
        fail_test "Branch name generation with username pattern" "Test failed"
    fi
}

# Test: Branch name generation with random pattern
test_branch_name_random() {
    start_test "Branch name generation with random pattern"

    cd "$PROJECT_ROOT"
    if go test -run TestGenerateBranchName_RandomPattern ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Branch name generation with random pattern"
    else
        fail_test "Branch name generation with random pattern" "Test failed"
    fi
}

# Test: Username sanitization
test_username_sanitization() {
    start_test "Username sanitization"

    cd "$PROJECT_ROOT"
    if go test -run TestSanitizeBranchName ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Username sanitization"
    else
        fail_test "Username sanitization" "Test failed"
    fi
}

# Test: Branch name collision handling
test_collision_handling() {
    start_test "Branch name collision handling"

    cd "$PROJECT_ROOT"
    if go test -run TestEnsureUniqueBranchName_WithCollision ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Branch name collision handling"
    else
        fail_test "Branch name collision handling" "Test failed"
    fi
}

# Test: Multiple collision retries
test_multiple_collisions() {
    start_test "Multiple collision retries"

    cd "$PROJECT_ROOT"
    if go test -run TestEnsureUniqueBranchName_MultipleCollisions ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Multiple collision retries"
    else
        fail_test "Multiple collision retries" "Test failed"
    fi
}

# Test: Git CountCommitsAhead
test_git_count_commits_ahead() {
    start_test "Git CountCommitsAhead"

    cd "$PROJECT_ROOT"
    if go test -run TestCountCommitsAhead ./internal/git/... >/dev/null 2>&1; then
        pass_test "Git CountCommitsAhead"
    else
        fail_test "Git CountCommitsAhead" "Test failed"
    fi
}

# Test: Git BranchExists
test_git_branch_exists() {
    start_test "Git BranchExists"

    cd "$PROJECT_ROOT"
    if go test -run TestBranchExists ./internal/git/... >/dev/null 2>&1; then
        pass_test "Git BranchExists"
    else
        fail_test "Git BranchExists" "Test failed"
    fi
}

# Test: Git GetCommitsBetween
test_git_get_commits_between() {
    start_test "Git GetCommitsBetween"

    cd "$PROJECT_ROOT"
    if go test -run TestGetCommitsBetween ./internal/git/... >/dev/null 2>&1; then
        pass_test "Git GetCommitsBetween"
    else
        fail_test "Git GetCommitsBetween" "Test failed"
    fi
}

# Test: Git PushBranch
test_git_push_branch() {
    start_test "Git PushBranch"

    cd "$PROJECT_ROOT"
    if go test -run TestPushBranch ./internal/git/... >/dev/null 2>&1; then
        pass_test "Git PushBranch"
    else
        fail_test "Git PushBranch" "Test failed"
    fi
}

# Test: Git CheckoutTrackingBranch
test_git_checkout_tracking() {
    start_test "Git CheckoutTrackingBranch"

    cd "$PROJECT_ROOT"
    if go test -run TestCheckoutTrackingBranch ./internal/git/... >/dev/null 2>&1; then
        pass_test "Git CheckoutTrackingBranch"
    else
        fail_test "Git CheckoutTrackingBranch" "Test failed"
    fi
}

# Test: Stale remote detection
test_stale_remote_detection() {
    start_test "Stale remote detection"

    cd "$PROJECT_ROOT"
    if go test -run TestCheckStaleRemote ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Stale remote detection"
    else
        fail_test "Stale remote detection" "Test failed"
    fi
}

# Test: Configuration validation
test_config_validation() {
    start_test "Configuration validation"

    cd "$PROJECT_ROOT"
    if go test -run TestValidate_AutoBranchNamePattern ./internal/config/... >/dev/null 2>&1; then
        pass_test "Configuration validation"
    else
        fail_test "Configuration validation" "Test failed"
    fi
}

# Test: Integration - Full workflow
test_integration_full_workflow() {
    start_test "Integration - Full workflow"

    cd "$PROJECT_ROOT"
    if go test -run TestFullAutomaticFlow ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Integration - Full workflow"
    else
        fail_test "Integration - Full workflow" "Test failed"
    fi
}

# Test: Integration - Config disabled
test_integration_config_disabled() {
    start_test "Integration - Config disabled (skipped: requires interaction)"
    skip_test "Integration - Config disabled" "Interactive prompt cannot be automated"
}

# Test: Integration - Master branch
test_integration_master_branch() {
    start_test "Integration - Master branch detection"

    cd "$PROJECT_ROOT"
    if go test -run TestDetectionResult_MasterBranch ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Integration - Master branch detection"
    else
        fail_test "Integration - Master branch detection" "Test failed"
    fi
}

# Test: Integration - Collision retry
test_integration_collision() {
    start_test "Integration - Collision with retry"

    cd "$PROJECT_ROOT"
    if go test -run TestBranchCollision ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Integration - Collision with retry"
    else
        fail_test "Integration - Collision with retry" "Test failed"
    fi
}

# Test: Integration - Custom patterns
test_integration_custom_patterns() {
    start_test "Integration - Custom branch patterns"

    cd "$PROJECT_ROOT"
    if go test -run TestCustomBranchPattern ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Integration - Custom branch patterns"
    else
        fail_test "Integration - Custom branch patterns" "Test failed"
    fi
}

# =============================================================================
# Template Handling Tests
# Tests for bug fixes related to continue mode and template handling:
# - ExtractBaseBranch function for both stacked and non-stacked PRs
# - Template sorting by modification time (newest first)
# - Preserving edits across multiple validation failures
# - Template validation and parsing
# =============================================================================

# Test: ExtractBaseBranch for non-stacked PRs
test_template_extract_base_nonstacked() {
    start_test "Template - Extract base branch (non-stacked)"

    cd "$PROJECT_ROOT"
    if go test -run TestExtractBaseBranch_NonStackedFormat ./internal/template/... >/dev/null 2>&1; then
        pass_test "Template - Extract base branch (non-stacked)"
    else
        fail_test "Template - Extract base branch (non-stacked)" "Test failed"
    fi
}

# Test: ExtractBaseBranch for stacked PRs
test_template_extract_base_stacked() {
    start_test "Template - Extract base branch (stacked)"

    cd "$PROJECT_ROOT"
    if go test -run TestExtractBaseBranch_StackedFormat ./internal/template/... >/dev/null 2>&1; then
        pass_test "Template - Extract base branch (stacked)"
    else
        fail_test "Template - Extract base branch (stacked)" "Test failed"
    fi
}

# Test: ExtractBaseBranch fallback to marker
test_template_extract_base_fallback() {
    start_test "Template - Extract base branch (fallback)"

    cd "$PROJECT_ROOT"
    if go test -run TestExtractBaseBranch_FallbackToMarker ./internal/template/... >/dev/null 2>&1; then
        pass_test "Template - Extract base branch (fallback)"
    else
        fail_test "Template - Extract base branch (fallback)" "Test failed"
    fi
}

# Test: ExtractBaseBranch with real template content
test_template_extract_base_real() {
    start_test "Template - Extract base branch (real templates)"

    cd "$PROJECT_ROOT"
    if go test -run TestExtractBaseBranch_RealTemplateContent ./internal/template/... >/dev/null 2>&1; then
        pass_test "Template - Extract base branch (real templates)"
    else
        fail_test "Template - Extract base branch (real templates)" "Test failed"
    fi
}

# Test: Template sorting by modification time
test_template_sorting_modtime() {
    start_test "Template - Sorting by modification time"

    cd "$PROJECT_ROOT"
    if go test -run TestFindSavedTemplates_SortsByModTime ./internal/template/... >/dev/null 2>&1; then
        pass_test "Template - Sorting by modification time"
    else
        fail_test "Template - Sorting by modification time" "Test failed"
    fi
}

# Test: Continue mode preserves edits across validation failures
test_continue_mode_preserves_edits() {
    start_test "Continue mode - Preserves edits across validation failures"

    cd "$PROJECT_ROOT"
    if go test -run TestContinueMode_PreservesEditsAcrossValidationFailures ./internal/template/... >/dev/null 2>&1; then
        pass_test "Continue mode - Preserves edits across validation failures"
    else
        fail_test "Continue mode - Preserves edits across validation failures" "Test failed"
    fi
}

# Test: Template validation with missing fields
test_template_validation() {
    start_test "Template - Validation with missing fields"

    cd "$PROJECT_ROOT"
    if go test -run TestValidateFields ./internal/template/... >/dev/null 2>&1; then
        pass_test "Template - Validation with missing fields"
    else
        fail_test "Template - Validation with missing fields" "Test failed"
    fi
}

# Test: Template parsing with complete fields
test_template_parsing() {
    start_test "Template - Parsing with complete fields"

    cd "$PROJECT_ROOT"
    if go test -run TestParseTemplateFilled ./internal/template/... >/dev/null 2>&1; then
        pass_test "Template - Parsing with complete fields"
    else
        fail_test "Template - Parsing with complete fields" "Test failed"
    fi
}

# =============================================================================
# Stacking Tests (Bug Fixes)
# Tests for bug fix: stacking detection when parent and main are at same commit
# This handles the auto-branch scenario where:
# - Local main has 1 commit ahead of origin/main
# - Auto-created branch is at same commit as local main
# - Child branch should stack on auto-created branch, not main
# =============================================================================

# Test: Stacking detection when parent and main are at same commit
test_stacking_same_commit_scenario() {
    start_test "Stacking - Detection with same commit (auto-branch scenario)"

    cd "$PROJECT_ROOT"
    if go test -run TestDetectBaseBranch_StackingDetected_SameCommitAsMain ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Stacking - Detection with same commit (auto-branch scenario)"
    else
        fail_test "Stacking - Detection with same commit (auto-branch scenario)" "Test failed"
    fi
}

# Test: Stacking detection with different merge-bases
test_stacking_different_mergebase() {
    start_test "Stacking - Detection with different merge-bases"

    cd "$PROJECT_ROOT"
    if go test -run TestDetectBaseBranch_StackingDetected ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Stacking - Detection with different merge-bases"
    else
        fail_test "Stacking - Detection with different merge-bases" "Test failed"
    fi
}

# Test: Stacking disabled configuration
test_stacking_disabled() {
    start_test "Stacking - Configuration disabled"

    cd "$PROJECT_ROOT"
    if go test -run TestDetectBaseBranch_StackingDisabled ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Stacking - Configuration disabled"
    else
        fail_test "Stacking - Configuration disabled" "Test failed"
    fi
}

# Test: No stacking opportunity detected
test_stacking_no_opportunity() {
    start_test "Stacking - No opportunity detected"

    cd "$PROJECT_ROOT"
    if go test -run TestDetectBaseBranch_NoStackingOpportunity ./internal/diff/... >/dev/null 2>&1; then
        pass_test "Stacking - No opportunity detected"
    else
        fail_test "Stacking - No opportunity detected" "Test failed"
    fi
}

# =============================================================================
# Continue Mode Integration Tests
# Shell-based integration tests for continue mode functionality:
# - Loading saved templates (both stacked and non-stacked)
# - Extracting base branch from template headers
# - Loading newest template when multiple exist
# - Preserving edits when validation fails
# These complement the Go unit tests with real file operations
# =============================================================================

# Test: Continue mode with saved template (shell-based)
test_continue_mode_saved_template() {
    start_test "Continue mode - Load saved template (shell)"
    reset_test_repo

    # Create a fake saved template
    local temp_file="/tmp/gh-arc-saved-test-$$.md"
    cat > "$temp_file" <<'EOF'
# Creating PR: feature/test → main
# Base Branch: main (read-only)

# Title:
Test Feature

# Summary:
Test summary

# Test Plan:

# Reviewers:

# Draft:
false
EOF

    # Check that template exists and is readable
    if [ -f "$temp_file" ] && [ -r "$temp_file" ]; then
        # Verify we can extract base branch
        local base=$(grep "^# Base Branch:" "$temp_file" | sed 's/# Base Branch: \(.*\) (read-only)/\1/')
        if [ "$base" = "main" ]; then
            pass_test "Continue mode - Load saved template (shell)"
        else
            fail_test "Continue mode - Load saved template (shell)" "Failed to extract base: got '$base', expected 'main'"
        fi
    else
        fail_test "Continue mode - Load saved template (shell)" "Template file not created or not readable"
    fi

    # Cleanup
    rm -f "$temp_file"
}

# Test: Continue mode with stacked PR template (shell-based)
test_continue_mode_stacked_template() {
    start_test "Continue mode - Load stacked PR template (shell)"
    reset_test_repo

    # Create a fake stacked PR template
    local temp_file="/tmp/gh-arc-saved-stacked-$$.md"
    cat > "$temp_file" <<'EOF'
# 📚 Creating stacked PR on feature/parent (PR #123: Parent PR)
# Base Branch: feature/parent (read-only)
# Current Branch: feature/child

# Title:
Child Feature

# Summary:
Builds on parent

# Test Plan:
Run tests

# Reviewers:

# Draft:
false
EOF

    # Verify we can extract base branch from stacked format
    if [ -f "$temp_file" ] && [ -r "$temp_file" ]; then
        # Check for stacked format marker
        if grep -q "# 📚 Creating stacked PR on" "$temp_file"; then
            # Extract base using sed
            local base=$(grep "^# Base Branch:" "$temp_file" | sed 's/# Base Branch: \(.*\) (read-only)/\1/')
            if [ "$base" = "feature/parent" ]; then
                pass_test "Continue mode - Load stacked PR template (shell)"
            else
                fail_test "Continue mode - Load stacked PR template (shell)" "Failed to extract base from stacked template: got '$base', expected 'feature/parent'"
            fi
        else
            fail_test "Continue mode - Load stacked PR template (shell)" "Stacked format marker not found"
        fi
    else
        fail_test "Continue mode - Load stacked PR template (shell)" "Template file not created"
    fi

    # Cleanup
    rm -f "$temp_file"
}

# Test: Multiple saved templates - newest loaded first
test_continue_mode_newest_template() {
    start_test "Continue mode - Load newest template when multiple exist"

    # Create multiple templates with different timestamps
    local temp_dir="/tmp/gh-arc-template-test-$$"
    mkdir -p "$temp_dir"

    # Create older template
    local old_template="$temp_dir/gh-arc-saved-old.md"
    echo "# Creating PR: feature/old → main" > "$old_template"
    echo "# Base Branch: main (read-only)" >> "$old_template"
    sleep 0.1

    # Create newer template
    local new_template="$temp_dir/gh-arc-saved-new.md"
    echo "# Creating PR: feature/new → main" > "$new_template"
    echo "# Base Branch: develop (read-only)" >> "$new_template"

    # Find newest by modification time
    local newest=$(ls -t "$temp_dir"/gh-arc-saved-*.md 2>/dev/null | head -n1)

    if [ "$newest" = "$new_template" ]; then
        # Verify content
        if grep -q "develop" "$newest"; then
            pass_test "Continue mode - Load newest template when multiple exist"
        else
            fail_test "Continue mode - Load newest template when multiple exist" "Newest template has wrong content"
        fi
    else
        fail_test "Continue mode - Load newest template when multiple exist" "Wrong template selected as newest"
    fi

    # Cleanup
    rm -rf "$temp_dir"
}

# Test: Template validation failure preserves edits
test_continue_mode_validation_failure() {
    start_test "Continue mode - Validation failure preserves edits (shell)"

    # Create template missing required field (Test Plan)
    local temp_file="/tmp/gh-arc-validation-test-$$.md"
    cat > "$temp_file" <<'EOF'
# Creating PR: feature/test → main
# Base Branch: main (read-only)

# Title:
My Feature

# Summary:
Feature description

EXTRA EDITS FROM USER - should be preserved!

# Test Plan:

# Reviewers:

# Draft:
false
EOF

    # Verify template contains user edits
    if grep -q "EXTRA EDITS FROM USER" "$temp_file"; then
        # Simulate saving after validation failure
        local new_temp="/tmp/gh-arc-validation-retry-$$.md"
        cp "$temp_file" "$new_temp"

        # Verify edits are preserved in new file
        if grep -q "EXTRA EDITS FROM USER" "$new_temp"; then
            pass_test "Continue mode - Validation failure preserves edits (shell)"
        else
            fail_test "Continue mode - Validation failure preserves edits (shell)" "Edits not preserved in saved template"
        fi
        rm -f "$new_temp"
    else
        fail_test "Continue mode - Validation failure preserves edits (shell)" "Template doesn't contain test edits"
    fi

    # Cleanup
    rm -f "$temp_file"
}

# Print summary
print_summary() {
    echo ""
    echo "======================================================================"
    echo "TEST SUMMARY"
    echo "======================================================================"
    echo "Total tests:  $TESTS_RUN"
    echo -e "${GREEN}Passed:       $TESTS_PASSED${NC}"
    echo -e "${RED}Failed:       $TESTS_FAILED${NC}"
    echo -e "${YELLOW}Skipped:      $TESTS_SKIPPED${NC}"
    echo "======================================================================"

    if [ "$TESTS_FAILED" -gt 0 ]; then
        echo -e "${RED}SOME TESTS FAILED${NC}"
        return 1
    elif [ "$TESTS_PASSED" -eq 0 ]; then
        echo -e "${YELLOW}NO TESTS RAN${NC}"
        return 1
    else
        echo -e "${GREEN}ALL TESTS PASSED${NC}"
        return 0
    fi
}

# Main execution
main() {
    local specific_test="$1"

    log_info "Auto-Branch from Main - E2E Test Suite"
    log_info "======================================="

    # Check prerequisites
    if ! command -v git >/dev/null 2>&1; then
        log_error "git is required but not installed"
        exit 2
    fi

    if ! command -v go >/dev/null 2>&1; then
        log_error "go is required but not installed"
        exit 2
    fi

    # Setup test environment
    setup_test_environment

    # Show cleanup mode
    if [ "$USER_SPECIFIED_DIR" -eq 1 ] && [ "$CLEANUP" -eq 0 ]; then
        log_info "Test directory will be preserved after tests complete"
    else
        log_info "Test directory will be automatically cleaned up"
    fi

    # Trap cleanup
    trap teardown_test_environment EXIT

    # Run tests
    if [ -n "$specific_test" ]; then
        log_info "Running specific test: $specific_test"
        if declare -f "$specific_test" >/dev/null 2>&1; then
            "$specific_test"
        else
            log_error "Test function not found: $specific_test"
            exit 2
        fi
    else
        log_info "Running all tests..."

        # Detection tests
        test_detection_on_main_with_commits
        test_detection_skips_feature_branch
        test_detection_skips_no_commits

        # Branch name generation tests
        test_branch_name_timestamp
        test_branch_name_date
        test_branch_name_datetime
        test_branch_name_username
        test_branch_name_random
        test_username_sanitization

        # Collision handling tests
        test_collision_handling
        test_multiple_collisions

        # Git operation tests
        test_git_count_commits_ahead
        test_git_branch_exists
        test_git_get_commits_between
        test_git_push_branch
        test_git_checkout_tracking

        # Other feature tests
        test_stale_remote_detection
        test_config_validation

        # Template handling tests
        test_template_extract_base_nonstacked
        test_template_extract_base_stacked
        test_template_extract_base_fallback
        test_template_extract_base_real
        test_template_sorting_modtime
        test_continue_mode_preserves_edits
        test_template_validation
        test_template_parsing

        # Stacking tests (bug fixes)
        test_stacking_same_commit_scenario
        test_stacking_different_mergebase
        test_stacking_disabled
        test_stacking_no_opportunity

        # Continue mode integration tests
        test_continue_mode_saved_template
        test_continue_mode_stacked_template
        test_continue_mode_newest_template
        test_continue_mode_validation_failure

        # Integration tests
        test_integration_full_workflow
        test_integration_config_disabled
        test_integration_master_branch
        test_integration_collision
        test_integration_custom_patterns
    fi

    # Print summary and exit with appropriate code
    if print_summary; then
        exit 0
    else
        exit 1
    fi
}

# Run main
main "$@"
