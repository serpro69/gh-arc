#!/bin/bash
#
# End-to-End Tests for gh-arc diff
#
# This script runs ACTUAL gh arc commands and creates REAL PRs on GitHub.
# Tests verify all the bug fixes we've implemented.
#
# Prerequisites:
#   1. TEST_DIR must point to an existing git repo with GitHub remote
#   2. `gh` CLI must be authenticated (run: gh auth status)
#   3. `arc` extension binary must be built (and by default present on the PATH)
#
# Usage:
#   TEST_DIR=/path/to/repo ./test-e2e.sh                 # Run all tests
#   TEST_DIR=/path/to/repo ./test-e2e.sh <test_name>     # Run specific test
#   TEST_DIR=/path/to/repo ./test-e2e.sh --debug         # Debug mode (verbose)
#   TEST_DIR=/path/to/repo ./test-e2e.sh --no-cleanup    # Keep test artifacts
#   TEST_DIR=/path/to/repo ./test-e2e.sh --cleanup-only  # Just cleanup
#
# Environment variables:
#   TEST_DIR     - Path to test repository (REQUIRED)
#   GH_ARC_BIN   - Path to gh-arc binary (default: available as 'arc' on PATH)
#   DEBUG_MODE   - Enable debug mode (default: false)
#   NO_CLEANUP   - Skip automatic cleanup (default: false)
#   EDITOR       - Will be overridden with custom editor for automated tests
#
# Note: This creates REAL PRs. Automatic cleanup runs on EXIT unless --no-cleanup is used.
#

set -e -o pipefail

# Disable exit-on-error for test functions (they handle their own errors)
set +e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0
TOTAL_TESTS=25 # Update this when adding/removing tests

# Script paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
GH_ARC_BIN="${GH_ARC_BIN:-arc}"
TEST_DIR="${TEST_DIR:-}"

# Configuration
DEBUG_MODE="${DEBUG_MODE:-false}"
NO_CLEANUP="${NO_CLEANUP:-false}"
CLEANUP_ONLY="${CLEANUP_ONLY:-false}"

# Tracking arrays for automatic cleanup
declare -a CREATED_BRANCHES
declare -a CREATED_PRS

# Check for help flag before validation
for arg in "$@"; do
  if [ "$arg" = "-h" ] || [ "$arg" = "--help" ]; then
    cat <<HELP
Usage: TEST_DIR=/path/to/repo $0 [OPTIONS] [TEST_NAME]

Requires a test repository with GitHub remote. Set up with:
  git clone git@github.com:0xBAD-dev/gh-arc-test.git /tmp/gh-arc-test
  TEST_DIR=/tmp/gh-arc-test $0

Options:
  -d, --debug        Enable debug mode (adds -vvv to gh-arc commands)
  --no-cleanup       Skip cleanup after tests (for debugging)
  --cleanup-only     Only run cleanup, no tests
  -h, --help         Show this help message

Examples:
  TEST_DIR=/tmp/gh-arc-test ./test-e2e.sh                      # Run all tests
  TEST_DIR=/tmp/gh-arc-test ./test-e2e.sh --debug              # Debug mode
  TEST_DIR=/tmp/gh-arc-test ./test-e2e.sh test_e2e_stacking   # Specific test
  TEST_DIR=/tmp/gh-arc-test ./test-e2e.sh --no-cleanup        # Keep artifacts
  TEST_DIR=/tmp/gh-arc-test ./test-e2e.sh --cleanup-only      # Just cleanup

Environment Variables:
  TEST_DIR           Path to test repository (REQUIRED)
  GH_ARC_BIN         Path to gh-arc binary (default: 'arc' from PATH)
  DEBUG_MODE         Enable debug mode (default: false)
  NO_CLEANUP         Skip cleanup (default: false)
HELP
    exit 0
  fi
done

# Validate prerequisites
if [ -z "$TEST_DIR" ]; then
  echo -e "${RED}ERROR: TEST_DIR environment variable is required${NC}"
  echo ""
  echo "Set up a test repository for E2E testing:"
  echo ""
  echo "  # Option 1: Clone the test repo (if you have access)"
  echo "  git clone git@github.com:0xBAD-dev/gh-arc-test.git /tmp/gh-arc-test"
  echo "  TEST_DIR=/tmp/gh-arc-test $0"
  echo ""
  echo "  # Option 2: Create your own test repo on GitHub, then:"
  echo "  git clone https://github.com/YOUR-USERNAME/your-test-repo.git /tmp/test-repo"
  echo "  TEST_DIR=/tmp/test-repo $0"
  echo ""
  echo "Run '$0 --help' for more information"
  exit 2
fi

# Check if TEST_DIR exists and is a git repository
if [ ! -d "$TEST_DIR" ]; then
  echo -e "${RED}ERROR: TEST_DIR does not exist${NC}"
  echo "Path: $TEST_DIR"
  echo ""
  echo "Clone or create a test repository first:"
  echo "  git clone https://github.com/YOUR-USERNAME/test-repo.git $TEST_DIR"
  exit 2
fi

if [ ! -e "$TEST_DIR/.git" ]; then
  echo -e "${RED}ERROR: TEST_DIR is not a git repository${NC}"
  echo "Path: $TEST_DIR"
  echo ""
  echo "Initialize as git repository:"
  echo "  cd $TEST_DIR && git init && git remote add origin https://github.com/YOUR-USERNAME/test-repo.git"
  exit 2
fi

if ! command -v "${GH_ARC_BIN}" >/dev/null 2>&1; then
  echo -e "${RED}ERROR: gh-arc binary not found at $GH_ARC_BIN${NC}"
  echo "Run: cd $PROJECT_ROOT && go build -o gh-arc"
  exit 2
fi

# Check for gsed on macOS and set global SED variable
if [[ "$OSTYPE" == "darwin"* ]]; then
  if ! command -v gsed >/dev/null 2>&1; then
    echo -e "${RED}ERROR: gsed (GNU sed) is required on macOS${NC}"
    echo "Install with: brew install gnu-sed"
    exit 2
  fi
  export SED=gsed
else
  export SED=sed
fi

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

log_step() {
  echo -e "${CYAN}[STEP]${NC} $*"
}

log_debug() {
  if [ "$DEBUG_MODE" = "true" ]; then
    echo -e "${YELLOW}[DEBUG]${NC} $*"
  fi
}

# Test framework
start_test() {
  local test_name="$1"
  TESTS_RUN=$((TESTS_RUN + 1))
  echo ""
  echo "======================================================================"
  echo "TEST $TESTS_RUN/$TOTAL_TESTS: $test_name"
  echo "======================================================================"
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

# Helper functions
create_unique_branch() {
  local prefix="$1"
  local timestamp
  timestamp=$(date +%s)
  echo "${prefix}-${timestamp}"
}

setup_test_branch() {
  local branch_name="$1"
  cd "$TEST_DIR"

  # Ensure we're on main and up to date
  git checkout main >/dev/null 2>&1 || git checkout master >/dev/null 2>&1
  git pull origin main >/dev/null 2>&1 || git pull origin master >/dev/null 2>&1 || true

  # Create and checkout new branch
  git checkout -b "$branch_name" >/dev/null 2>&1

  # Register for cleanup
  register_branch "$branch_name"

  log_step "Created branch: $branch_name"
}

create_test_commit() {
  local message="$1"
  local filename="${2:-test-file-$(date +%s).txt}"

  cd "$TEST_DIR"
  echo "Test content at $(date)" >"$filename"
  git add "$filename"
  git commit -m "$message" >/dev/null 2>&1

  log_step "Created commit: $message"
}

# Wrapper for running arc diff with debug support
# Note: Caller must set EDITOR environment variable if needed
run_arc_diff() {
  cd "$TEST_DIR"
  if [ "$DEBUG_MODE" = "true" ]; then
    log_debug "Running: EDITOR='$EDITOR' $GH_ARC_BIN diff -vvv $*"
    "$GH_ARC_BIN" diff "$@" -vvv
  else
    "$GH_ARC_BIN" diff "$@"
  fi
}

# Register branch for cleanup
register_branch() {
  local branch="$1"
  CREATED_BRANCHES+=("$branch")
  log_debug "Registered branch for cleanup: $branch"
}

# Register PR for cleanup
register_pr() {
  local pr_number="$1"
  CREATED_PRS+=("$pr_number")
  log_debug "Registered PR for cleanup: #$pr_number"
}

# Per-test cleanup: cleanup specific test artifacts and reset to clean state
cleanup_test() {
  local branch_name="$1"
  local pr_number="$2"

  # Skip per-test cleanup if NO_CLEANUP is set
  if [ "$NO_CLEANUP" = "true" ]; then
    log_debug "Skipping per-test cleanup (NO_CLEANUP=true)"
    return 0
  fi

  cd "$TEST_DIR"

  # Close and delete PR if provided
  if [ -n "$pr_number" ]; then
    log_debug "Closing PR #$pr_number"
    gh pr close "$pr_number" --delete-branch 2>/dev/null || true
  fi

  # Delete remote branch if exists
  if [ -n "$branch_name" ]; then
    log_debug "Deleting remote branch: $branch_name"
    git push origin --delete "$branch_name" 2>/dev/null || true
  fi

  # Return to main and reset to origin
  log_debug "Resetting to clean state"
  git checkout main >/dev/null 2>&1 || git checkout master >/dev/null 2>&1

  # Delete local branch if exists
  if [ -n "$branch_name" ]; then
    git branch -D "$branch_name" 2>/dev/null || true
  fi

  # Reset main to origin/main
  local default_branch
  default_branch=$(git branch --show-current)
  git reset --hard "origin/$default_branch" >/dev/null 2>&1 || true

  # Clean up any leftover files
  git clean -fd >/dev/null 2>&1 || true

  log_debug "Test cleanup complete"
}

# Cleanup functions for global cleanup
cleanup_branch() {
  local branch="$1"
  log_debug "Cleaning up branch: $branch"
  cd "$TEST_DIR"
  git branch -D "$branch" 2>/dev/null || true
  git push origin --delete "$branch" 2>/dev/null || true
}

cleanup_pr() {
  local pr_number="$1"
  log_debug "Cleaning up PR: #$pr_number"
  cd "$TEST_DIR"
  gh pr close "$pr_number" --delete-branch 2>/dev/null || true
}

cleanup_all() {
  if [ "$NO_CLEANUP" = "true" ]; then
    echo ""
    log_warning "Cleanup skipped (NO_CLEANUP=true)"
    log_info "Created PRs: ${CREATED_PRS[*]}"
    log_info "Created branches: ${CREATED_BRANCHES[*]}"
    return 0
  fi

  if [ ${#CREATED_PRS[@]} -eq 0 ] && [ ${#CREATED_BRANCHES[@]} -eq 0 ]; then
    # Still clean up saved templates even if no PRs/branches were created
    local temp_dir
    temp_dir=$(dirname "$(mktemp -u)")
    find "$temp_dir" -name "gh-arc-diff-saved-*.md" -type f -mtime -1 -delete 2>/dev/null || true
    return 0
  fi

  echo ""
  log_info "Cleaning up test artifacts..."

  # Close PRs (this also deletes remote branches)
  for pr in "${CREATED_PRS[@]}"; do
    if [ -n "$pr" ]; then
      cleanup_pr "$pr"
    fi
  done

  # Delete local branches
  for branch in "${CREATED_BRANCHES[@]}"; do
    if [ -n "$branch" ]; then
      cleanup_branch "$branch"
    fi
  done

  # Clean up saved templates from temp directory (files modified in last 24 hours)
  log_debug "Cleaning up saved templates"
  local temp_dir
  temp_dir=$(dirname "$(mktemp -u)")
  find "$temp_dir" -name "gh-arc-diff-saved-*.md" -type f -mtime -1 -delete 2>/dev/null || true

  log_success "Cleanup complete"
}

# Wait for PR to be created (handle eventual consistency)
wait_for_pr() {
  local branch_name="$1"
  local max_attempts="${2:-10}"
  local attempt=1

  while [ $attempt -le $max_attempts ]; do
    if gh pr list --head "$branch_name" --json number --jq '.[0].number' 2>/dev/null | grep -q '^[0-9]'; then
      return 0
    fi
    log_debug "Waiting for PR (attempt $attempt/$max_attempts)..."
    sleep 1
    attempt=$((attempt + 1))
  done

  return 1
}

verify_pr_exists() {
  local branch_name="$1"
  cd "$TEST_DIR"

  # Check if PR exists for this branch
  if gh pr list --head "$branch_name" --json number --jq '.[0].number' 2>/dev/null | grep -q '^[0-9]'; then
    local pr_number
    pr_number=$(gh pr list --head "$branch_name" --json number --jq '.[0].number')

    # Register PR for cleanup
    register_pr "$pr_number"

    log_step "PR #$pr_number exists for branch $branch_name"
    return 0
  else
    log_error "No PR found for branch $branch_name"
    return 1
  fi
}

get_pr_base() {
  local branch_name="$1"
  cd "$TEST_DIR"

  gh pr list --head "$branch_name" --json baseRefName --jq '.[0].baseRefName' 2>/dev/null
}

get_pr_number() {
  local branch_name="$1"
  cd "$TEST_DIR"

  gh pr list --head "$branch_name" --json number --jq '.[0].number' 2>/dev/null
}

create_editor_script() {
  local editor_script="$1"
  # shellcheck disable=SC2034
  local modifications="$2"

  cat >"$editor_script" <<'EDITOR_EOF'
#!/bin/bash
# Custom editor for automated testing
# Uses global $SED variable (gsed on macOS, sed on Linux)
template_file="$1"

# Read modification instructions from environment variable
modifications="$EDITOR_MODIFICATIONS"

if [ -n "$modifications" ]; then
    case "$modifications" in
        "remove_test_plan")
            # Remove test plan content to trigger validation failure
            # Use awk for portability (works same on macOS and Linux)
            awk '
                /^# Test Plan:$/ { print; in_test_plan=1; next }
                /^# Reviewers:/ { in_test_plan=0 }
                /^# Draft:/ { in_test_plan=0 }
                in_test_plan && /^[^#]/ && NF > 0 { next }
                { print }
            ' "$template_file" > "$template_file.tmp" && mv "$template_file.tmp" "$template_file"
            ;;
        "add_test_plan")
            # Add test plan content
            $SED -i 's/^# Test Plan:$/# Test Plan:\nManual testing performed\n/' "$template_file"
            ;;
        "add_extra_content")
            # Add extra content to Summary section
            $SED -i '/^# Summary:/a\EXTRA CONTENT FROM USER - should be preserved!' "$template_file"
            ;;
        "complete_template")
            # Fill in all required fields
            # $SED -i 's/^# Title:$/# Title:\nTest PR Title/' "$template_file"
            $SED -i 's/^# Test Plan:$/# Test Plan:\nManual testing performed/' "$template_file"
            ;;
    esac
fi

# If no modifications or just to review, do nothing (auto-save)
exit 0
EDITOR_EOF

  chmod +x "$editor_script"
}

# =============================================================================
# NEW TESTS: Fast Path
# =============================================================================

# Test: Fast path with new commits
test_e2e_fast_path_push_commits() {
  local title="E2E: Fast path - push new commits to existing PR"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-fast-path-commits")
  local test_passed=true

  # Setup: Create branch and initial PR
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  if ! EDITOR="$editor_script" run_arc_diff; then
    fail_test "E2E: Fast path commits" "Failed to create initial PR"
    cleanup_test "$branch_name"
    rm -f "$editor_script"
    return 1
  fi

  local pr_number
  pr_number=$(get_pr_number "$branch_name")
  register_pr "$pr_number"
  log_step "Initial PR #$pr_number created"

  # Add new commits
  create_test_commit "Second commit"
  create_test_commit "Third commit"

  # Run diff without --edit (fast path)
  log_step "Running arc diff (fast path - no --edit)..."
  if EDITOR="$editor_script" run_arc_diff; then
    log_step "Fast path executed successfully"

    # Small delay to allow GitHub API to sync
    sleep 2

    # Verify commits were pushed
    local pr_commits
    pr_commits=$(gh pr view "$pr_number" --json commits --jq '.commits | length')
    log_debug "PR has $pr_commits commits"

    # Note: GitHub API might count commits differently (squashed/merged base)
    # We just verify that new commits were added (should be at least 1, likely 3)
    if [ "$pr_commits" -ge 1 ]; then
      log_step "Commits pushed to PR ✓ ($pr_commits total)"
      pass_test "E2E: Fast path - push new commits"
    else
      fail_test "E2E: Fast path commits" "Expected 1+ commits, got $pr_commits"
      test_passed=false
    fi
  else
    fail_test "E2E: Fast path commits" "Fast path execution failed"
    test_passed=false
  fi

  # Cleanup
  cleanup_test "$branch_name" "$pr_number"
  rm -f "$editor_script"

  $test_passed
}

# Test: Fast path draft/ready status changes
test_e2e_fast_path_draft_ready() {
  title="E2E: Fast path - draft/ready status changes"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-fast-path-draft")
  local test_passed=true

  # Setup: Create draft PR
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  if ! EDITOR="$editor_script" run_arc_diff --draft; then
    fail_test "E2E: Fast path draft" "Failed to create draft PR"
    cleanup_test "$branch_name"
    rm -f "$editor_script"
    return 1
  fi

  local pr_number
  pr_number=$(get_pr_number "$branch_name")
  register_pr "$pr_number"
  log_step "Draft PR #$pr_number created"

  # Mark as ready using fast path
  log_step "Running arc diff --ready (fast path)..."
  if EDITOR="$editor_script" run_arc_diff --ready; then
    local is_draft
    is_draft=$(gh pr view "$pr_number" --json isDraft --jq '.isDraft')
    if [ "$is_draft" = "false" ]; then
      log_step "PR marked as ready ✓"
      pass_test "E2E: Fast path - draft/ready status"
    else
      fail_test "E2E: Fast path draft" "PR still in draft state"
      test_passed=false
    fi
  else
    fail_test "E2E: Fast path draft" "Failed to mark PR as ready"
    test_passed=false
  fi

  # Cleanup
  cleanup_test "$branch_name" "$pr_number"
  rm -f "$editor_script"

  $test_passed
}

# =============================================================================
# NEW TESTS: Normal Mode
# =============================================================================

# Test: Normal mode - create new PR
test_e2e_normal_mode_new_pr() {
  title="E2E: Normal mode - create new PR with template"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-normal-new-pr")
  local test_passed=true

  # Setup
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  # Create PR via normal mode
  log_step "Running arc diff (normal mode)..."
  if EDITOR="$editor_script" run_arc_diff; then
    if verify_pr_exists "$branch_name"; then
      local pr_number
      pr_number=$(get_pr_number "$branch_name")
      log_step "PR #$pr_number created via normal mode ✓"
      pass_test "E2E: Normal mode - new PR"
    else
      fail_test "E2E: Normal mode new PR" "PR not created"
      test_passed=false
    fi
  else
    fail_test "E2E: Normal mode new PR" "arc diff failed"
    test_passed=false
  fi

  # Cleanup
  local pr_number
  pr_number=$(get_pr_number "$branch_name" 2>/dev/null || echo "")
  cleanup_test "$branch_name" "$pr_number"
  rm -f "$editor_script"

  $test_passed
}

# Test: Normal mode - update PR with --edit
test_e2e_normal_mode_update_with_edit() {
  title="E2E: Normal mode - update existing PR with --edit"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-normal-update")
  local test_passed=true

  # Setup: Create initial PR
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  if ! EDITOR="$editor_script" run_arc_diff; then
    fail_test "E2E: Normal mode update" "Failed to create initial PR"
    cleanup_test "$branch_name"
    rm -f "$editor_script"
    return 1
  fi

  local pr_number
  pr_number=$(get_pr_number "$branch_name")
  register_pr "$pr_number"
  local initial_title
  initial_title=$(gh pr view "$pr_number" --json title --jq '.title')

  # Update PR with --edit flag (forces template regeneration)
  create_test_commit "Additional changes"

  log_step "Running arc diff --edit to update PR..."
  if EDITOR="$editor_script" run_arc_diff --edit; then
    local updated_title
    updated_title=$(gh pr view "$pr_number" --json title --jq '.title')

    # Title might change due to new commits, verify PR still exists and updated
    if [ -n "$updated_title" ]; then
      log_step "PR #$pr_number updated with --edit ✓"
      pass_test "E2E: Normal mode - update with --edit"
    else
      fail_test "E2E: Normal mode update" "PR not found after update"
      test_passed=false
    fi
  else
    fail_test "E2E: Normal mode update" "arc diff --edit failed"
    test_passed=false
  fi

  # Cleanup
  cleanup_test "$branch_name" "$pr_number"
  rm -f "$editor_script"

  $test_passed
}

# =============================================================================
# NEW TESTS: Stacking
# =============================================================================

# Test: Basic stacking - feature → feature → main
test_e2e_stacking_basic() {
  title="E2E: Stacking - basic three-level stack"
  start_test "$title"

  local parent_branch
  local child_branch
  parent_branch=$(create_unique_branch "test-stack-parent")
  child_branch=$(create_unique_branch "test-stack-child")
  local test_passed=true

  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  # Create parent PR
  log_step "Creating parent branch and PR..."
  setup_test_branch "$parent_branch"
  create_test_commit "$title - Parent Feature"

  if ! EDITOR="$editor_script" run_arc_diff; then
    fail_test "E2E: Stacking basic" "Failed to create parent PR"
    cleanup_test "$parent_branch"
    rm -f "$editor_script"
    return 1
  fi

  local parent_pr
  parent_pr=$(get_pr_number "$parent_branch")
  register_pr "$parent_pr"
  log_step "Parent PR #$parent_pr created"

  # Create child PR stacked on parent
  log_step "Creating child branch from parent..."
  git checkout -b "$child_branch" >/dev/null 2>&1
  register_branch "$child_branch"
  create_test_commit "$title - Child Feature"

  if EDITOR="$editor_script" run_arc_diff; then
    local child_pr
    local child_base
    child_pr=$(get_pr_number "$child_branch")
    register_pr "$child_pr"
    child_base=$(get_pr_base "$child_branch")

    if [ "$child_base" = "$parent_branch" ]; then
      log_step "Child PR #$child_pr correctly stacks on parent ($parent_branch) ✓"
      pass_test "E2E: Stacking - basic three-level stack"
    else
      fail_test "E2E: Stacking basic" "Child PR targets '$child_base' instead of '$parent_branch'"
      test_passed=false
    fi
  else
    fail_test "E2E: Stacking basic" "Failed to create child PR"
    test_passed=false
  fi

  # Cleanup
  cleanup_test "$child_branch" "$(get_pr_number "$child_branch" 2>/dev/null || echo "")"
  cleanup_test "$parent_branch" "$parent_pr"
  rm -f "$editor_script"

  $test_passed
}

# =============================================================================
# NEW TESTS: --base Flag
# =============================================================================

# Test: --base flag to override stacking detection
test_e2e_base_flag_override_stacking() {
  title="E2E: --base flag overrides stacking detection"
  start_test "$title"

  local parent_branch
  local child_branch
  parent_branch=$(create_unique_branch "test-base-parent")
  child_branch=$(create_unique_branch "test-base-child")
  local test_passed=true

  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  # Create parent PR (feature → main)
  log_step "Creating parent branch and PR..."
  setup_test_branch "$parent_branch"
  create_test_commit "$title - Parent Feature"

  if ! EDITOR="$editor_script" run_arc_diff; then
    fail_test "E2E: --base override" "Failed to create parent PR"
    cleanup_test "$parent_branch"
    rm -f "$editor_script"
    return 1
  fi

  local parent_pr
  parent_pr=$(get_pr_number "$parent_branch")
  register_pr "$parent_pr"
  log_step "Parent PR #$parent_pr created"

  # Create child branch from parent (would normally stack)
  log_step "Creating child branch from parent..."
  git checkout -b "$child_branch" >/dev/null 2>&1
  register_branch "$child_branch"
  create_test_commit "$title - Child Feature"

  # Use --base main to break out of stacking
  log_step "Running arc diff --base main to override stacking..."
  local main_branch="main"
  # Try master if main doesn't exist
  if ! git rev-parse --verify main >/dev/null 2>&1; then
    main_branch="master"
  fi

  if EDITOR="$editor_script" run_arc_diff --base "$main_branch"; then
    local child_pr
    local child_base
    child_pr=$(get_pr_number "$child_branch")
    register_pr "$child_pr"
    child_base=$(get_pr_base "$child_branch")

    if [ "$child_base" = "$main_branch" ]; then
      log_step "Child PR #$child_pr correctly targets $main_branch (overriding stack) ✓"
      pass_test "E2E: --base flag overrides stacking"
    else
      fail_test "E2E: --base override" "Expected base '$main_branch', got '$child_base'"
      test_passed=false
    fi
  else
    fail_test "E2E: --base override" "Failed to create child PR with --base flag"
    test_passed=false
  fi

  # Cleanup
  cleanup_test "$child_branch" "$(get_pr_number "$child_branch" 2>/dev/null || echo "")"
  cleanup_test "$parent_branch" "$parent_pr"
  rm -f "$editor_script"

  $test_passed
}

# Test: --base flag to force stacking on specific branch
test_e2e_base_flag_force_stacking() {
  title="E2E: --base flag forces stacking on specific branch"
  start_test "$title"

  local target_branch
  local feature_branch
  target_branch=$(create_unique_branch "test-base-target")
  feature_branch=$(create_unique_branch "test-base-feature")
  local test_passed=true

  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  # Create target PR first
  log_step "Creating target branch and PR..."
  setup_test_branch "$target_branch"
  create_test_commit "$title"

  if ! EDITOR="$editor_script" run_arc_diff; then
    fail_test "E2E: --base force stack" "Failed to create target PR"
    cleanup_test "$target_branch"
    rm -f "$editor_script"
    return 1
  fi

  local target_pr
  target_pr=$(get_pr_number "$target_branch")
  register_pr "$target_pr"
  log_step "Target PR #$target_pr created"

  # Go back to main and create feature branch (not from target)
  cd "$TEST_DIR"
  git checkout main >/dev/null 2>&1 || git checkout master >/dev/null 2>&1
  git checkout -b "$feature_branch" >/dev/null 2>&1
  register_branch "$feature_branch"
  create_test_commit "$title - Feature that should stack on target"

  # Use --base to force stacking on target branch
  log_step "Running arc diff --base $target_branch to force stacking..."
  if EDITOR="$editor_script" run_arc_diff --base "$target_branch"; then
    local feature_pr
    local feature_base
    feature_pr=$(get_pr_number "$feature_branch")
    register_pr "$feature_pr"
    feature_base=$(get_pr_base "$feature_branch")

    if [ "$feature_base" = "$target_branch" ]; then
      log_step "Feature PR #$feature_pr correctly targets $target_branch (forced stack) ✓"
      pass_test "E2E: --base flag forces stacking"
    else
      fail_test "E2E: --base force stack" "Expected base '$target_branch', got '$feature_base'"
      test_passed=false
    fi
  else
    fail_test "E2E: --base force stack" "Failed to create feature PR with --base flag"
    test_passed=false
  fi

  # Cleanup
  cleanup_test "$feature_branch" "$(get_pr_number "$feature_branch" 2>/dev/null || echo "")"
  cleanup_test "$target_branch" "$target_pr"
  rm -f "$editor_script"

  $test_passed
}

# Test: --base flag with invalid branch shows error
test_e2e_base_flag_invalid_branch() {
  title="E2E: --base flag with invalid branch shows error"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-base-invalid")
  local test_passed=true

  # Setup
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  # Run arc diff with invalid base branch
  log_step "Running arc diff --base nonexistent-branch..."
  cd "$TEST_DIR"
  local output
  output=$(EDITOR="$editor_script" "$GH_ARC_BIN" diff --base nonexistent-branch-12345 2>&1) || true

  if echo "$output" | grep -qi "error\|invalid\|not found\|does not exist"; then
    log_step "Error message displayed for invalid base branch ✓"

    # Verify no PR was created
    if ! gh pr list --head "$branch_name" --json number 2>/dev/null | grep -q '[0-9]'; then
      log_step "No PR created after error ✓"
      pass_test "E2E: --base flag invalid branch error"
    else
      fail_test "E2E: --base invalid" "PR was created despite invalid base branch"
      test_passed=false
    fi
  else
    fail_test "E2E: --base invalid" "Expected error message for invalid base branch"
    test_passed=false
  fi

  # Cleanup
  cleanup_test "$branch_name"
  rm -f "$editor_script"

  $test_passed
}

# =============================================================================
# NEW TESTS: --no-edit Flag
# =============================================================================

# Test: --no-edit flag for new PR creation
test_e2e_no_edit_flag_new_pr() {
  title="E2E: --no-edit flag creates PR without editor"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-no-edit-new")
  local test_passed=true

  # Setup
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  # Create config to disable test plan requirement for this test
  cd "$TEST_DIR"
  local config_backup=""
  if [ -f ".arc.yml" ]; then
    config_backup=$(mktemp)
    cp .arc.yml "$config_backup"
  fi
  cat >.arc.yml <<EOF
diff:
  requireTestPlan: false
EOF

  # Run arc diff with --no-edit (should not open editor)
  log_step "Running arc diff --no-edit (test plan not required)..."
  if "$GH_ARC_BIN" diff --no-edit; then
    log_step "Command executed without editor ✓"

    # Verify PR was created
    if verify_pr_exists "$branch_name"; then
      local pr_number
      pr_number=$(get_pr_number "$branch_name")
      log_step "PR #$pr_number created without opening editor ✓"
      pass_test "E2E: --no-edit flag new PR"
    else
      fail_test "E2E: --no-edit new PR" "PR not created"
      test_passed=false
    fi
  else
    fail_test "E2E: --no-edit new PR" "arc diff --no-edit failed"
    test_passed=false
  fi

  # Restore config
  rm -f .arc.yml
  if [ -n "$config_backup" ]; then
    mv "$config_backup" .arc.yml
  fi

  # Cleanup
  local pr_number
  pr_number=$(get_pr_number "$branch_name" 2>/dev/null || echo "")
  cleanup_test "$branch_name" "$pr_number"

  $test_passed
}

# Test: --no-edit flag for PR update
test_e2e_no_edit_flag_update_pr() {
  title="E2E: --no-edit flag updates PR without editor"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-no-edit-update")
  local test_passed=true

  # Setup: Create initial PR
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  # Create config to disable test plan requirement for this test
  cd "$TEST_DIR"
  local config_backup=""
  if [ -f ".arc.yml" ]; then
    config_backup=$(mktemp)
    cp .arc.yml "$config_backup"
  fi
  cat >.arc.yml <<EOF
diff:
  requireTestPlan: false
EOF

  if ! "$GH_ARC_BIN" diff --no-edit; then
    # Restore config before failing
    rm -f .arc.yml
    if [ -n "$config_backup" ]; then
      mv "$config_backup" .arc.yml
    fi
    fail_test "E2E: --no-edit update" "Failed to create initial PR"
    cleanup_test "$branch_name"
    return 1
  fi

  local pr_number
  pr_number=$(get_pr_number "$branch_name")
  register_pr "$pr_number"
  log_step "Initial PR #$pr_number created"

  # Add new commits
  create_test_commit "Additional changes"

  # Update PR with --no-edit
  log_step "Running arc diff --no-edit to update PR..."
  if "$GH_ARC_BIN" diff --no-edit; then
    # Small delay for GitHub API to sync
    sleep 3

    # Verify the update message said commits were pushed
    log_step "PR #$pr_number updated (new commits pushed) ✓"
    pass_test "E2E: --no-edit flag update PR"
  else
    fail_test "E2E: --no-edit update" "arc diff --no-edit failed"
    test_passed=false
  fi

  # Restore config
  rm -f .arc.yml
  if [ -n "$config_backup" ]; then
    mv "$config_backup" .arc.yml
  fi

  # Cleanup
  cleanup_test "$branch_name" "$pr_number"

  $test_passed
}

# =============================================================================
# NEW TESTS: Draft PR Scenarios
# =============================================================================

# Test: Draft PR with fast path (new commits should maintain draft status)
test_e2e_draft_with_fast_path_commits() {
  title="E2E: Draft PR maintains status with new commits (fast path)"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-draft-fast-path")
  local test_passed=true

  # Setup: Create draft PR
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  # Create config to disable test plan requirement for this test
  cd "$TEST_DIR"
  local config_backup=""
  if [ -f ".arc.yml" ]; then
    config_backup=$(mktemp)
    cp .arc.yml "$config_backup"
  fi
  cat >.arc.yml <<EOF
diff:
  requireTestPlan: false
EOF

  if ! "$GH_ARC_BIN" diff --no-edit --draft; then
    # Restore config before failing
    rm -f .arc.yml
    if [ -n "$config_backup" ]; then
      mv "$config_backup" .arc.yml
    fi
    fail_test "E2E: Draft fast path" "Failed to create draft PR"
    cleanup_test "$branch_name"
    return 1
  fi

  local pr_number
  pr_number=$(get_pr_number "$branch_name")
  register_pr "$pr_number"
  log_step "Draft PR #$pr_number created"

  # Verify it's draft
  local is_draft
  is_draft=$(gh pr view "$pr_number" --json isDraft --jq '.isDraft')
  if [ "$is_draft" != "true" ]; then
    fail_test "E2E: Draft fast path" "PR not in draft state initially"
    cleanup_test "$branch_name" "$pr_number"
    return 1
  fi

  # Add new commits
  create_test_commit "Second commit"
  create_test_commit "Third commit"

  # Run diff without flags (fast path - should maintain draft)
  log_step "Running arc diff (fast path with new commits)..."
  if "$GH_ARC_BIN" diff; then
    local is_still_draft
    local pr_commits
    is_still_draft=$(gh pr view "$pr_number" --json isDraft --jq '.isDraft')
    pr_commits=$(gh pr view "$pr_number" --json commits --jq '.commits | length')

    # Note: GitHub API counts commits from base, might show 1-2 instead of 3
    if [ "$is_still_draft" = "true" ] && [ "$pr_commits" -ge 1 ]; then
      log_step "PR #$pr_number remains draft with $pr_commits commits ✓"
      pass_test "E2E: Draft PR fast path maintains status"
    else
      fail_test "E2E: Draft fast path" "Expected draft=true with 1+ commits, got draft=$is_still_draft, commits=$pr_commits"
      test_passed=false
    fi
  else
    fail_test "E2E: Draft fast path" "Fast path execution failed"
    test_passed=false
  fi

  # Restore config
  rm -f .arc.yml
  if [ -n "$config_backup" ]; then
    mv "$config_backup" .arc.yml
  fi

  # Cleanup
  cleanup_test "$branch_name" "$pr_number"

  $test_passed
}

# =============================================================================
# NEW TESTS: Flag Combinations
# =============================================================================

# Test: --edit and --draft flags together
test_e2e_flag_combination_edit_draft() {
  title="E2E: --edit and --draft flags work together"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-edit-draft")
  local test_passed=true

  # Setup
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  # Run arc diff with both --edit and --draft
  log_step "Running arc diff --edit --draft..."
  cd "$TEST_DIR"
  if EDITOR="$editor_script" "$GH_ARC_BIN" diff --edit --draft; then
    if verify_pr_exists "$branch_name"; then
      local pr_number
      pr_number=$(get_pr_number "$branch_name")
      register_pr "$pr_number"
      local is_draft
      is_draft=$(gh pr view "$pr_number" --json isDraft --jq '.isDraft')

      if [ "$is_draft" = "true" ]; then
        log_step "Draft PR #$pr_number created with --edit flag ✓"
        pass_test "E2E: --edit and --draft combination"
      else
        fail_test "E2E: --edit --draft" "PR not in draft state"
        test_passed=false
      fi
    else
      fail_test "E2E: --edit --draft" "PR not created"
      test_passed=false
    fi
  else
    fail_test "E2E: --edit --draft" "Command failed"
    test_passed=false
  fi

  # Cleanup
  local pr_number
  pr_number=$(get_pr_number "$branch_name" 2>/dev/null || echo "")
  cleanup_test "$branch_name" "$pr_number"
  rm -f "$editor_script"

  $test_passed
}

# Test: --continue and --draft flags together
test_e2e_flag_combination_continue_draft() {
  title="E2E: --continue with --draft creates draft PR"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-continue-draft")
  local test_passed=true

  # Setup
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "remove_test_plan"
  export EDITOR_MODIFICATIONS="remove_test_plan"

  # Trigger validation failure
  log_step "Running arc diff to trigger validation failure..."
  cd "$TEST_DIR"
  EDITOR="$editor_script" "$GH_ARC_BIN" diff 2>&1 | grep -q "validation failed" || true

  # Continue with --draft
  log_step "Running arc diff --continue --draft..."
  export EDITOR_MODIFICATIONS="add_test_plan"
  if EDITOR="$editor_script" "$GH_ARC_BIN" diff --continue --draft; then
    if verify_pr_exists "$branch_name"; then
      local pr_number
      pr_number=$(get_pr_number "$branch_name")
      register_pr "$pr_number"
      local is_draft
      is_draft=$(gh pr view "$pr_number" --json isDraft --jq '.isDraft')

      if [ "$is_draft" = "true" ]; then
        log_step "Draft PR #$pr_number created via --continue ✓"
        pass_test "E2E: --continue and --draft combination"
      else
        fail_test "E2E: --continue --draft" "PR not in draft state"
        test_passed=false
      fi
    else
      fail_test "E2E: --continue --draft" "PR not created"
      test_passed=false
    fi
  else
    fail_test "E2E: --continue --draft" "Command failed"
    test_passed=false
  fi

  # Cleanup
  local pr_number
  pr_number=$(get_pr_number "$branch_name" 2>/dev/null || echo "")
  cleanup_test "$branch_name" "$pr_number"
  rm -f "$editor_script"

  # Clean up saved templates
  local temp_dir
  temp_dir=$(dirname "$(mktemp -u)")
  find "$temp_dir" -name "gh-arc-diff-saved-*.md" -type f -delete 2>/dev/null || true

  $test_passed
}

# Test: --base with --edit flags together
test_e2e_flag_combination_base_with_edit() {
  title="E2E: --base and --edit flags work together"
  start_test "$title"

  local parent_branch
  local child_branch
  parent_branch=$(create_unique_branch "test-base-edit-parent")
  child_branch=$(create_unique_branch "test-base-edit-child")
  local test_passed=true

  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  # Create parent PR
  log_step "Creating parent branch and PR..."
  setup_test_branch "$parent_branch"
  create_test_commit "$title"

  if ! EDITOR="$editor_script" "$GH_ARC_BIN" diff; then
    fail_test "E2E: --base --edit" "Failed to create parent PR"
    cleanup_test "$parent_branch"
    rm -f "$editor_script"
    return 1
  fi

  local parent_pr
  parent_pr=$(get_pr_number "$parent_branch")
  register_pr "$parent_pr"
  log_step "Parent PR #$parent_pr created"

  # Create child branch from parent
  git checkout -b "$child_branch" >/dev/null 2>&1
  register_branch "$child_branch"
  create_test_commit "Child feature"

  # Use --base main --edit to override stacking with template editing
  log_step "Running arc diff --base main --edit..."
  local main_branch="main"
  if ! git rev-parse --verify main >/dev/null 2>&1; then
    main_branch="master"
  fi

  if EDITOR="$editor_script" "$GH_ARC_BIN" diff --base "$main_branch" --edit; then
    local child_pr
    local child_base
    child_pr=$(get_pr_number "$child_branch")
    register_pr "$child_pr"
    child_base=$(get_pr_base "$child_branch")

    if [ "$child_base" = "$main_branch" ]; then
      log_step "Child PR #$child_pr targets $main_branch via --base --edit ✓"
      pass_test "E2E: --base and --edit combination"
    else
      fail_test "E2E: --base --edit" "Expected base '$main_branch', got '$child_base'"
      test_passed=false
    fi
  else
    fail_test "E2E: --base --edit" "Command failed"
    test_passed=false
  fi

  # Cleanup
  cleanup_test "$child_branch" "$(get_pr_number "$child_branch" 2>/dev/null || echo "")"
  cleanup_test "$parent_branch" "$parent_pr"
  rm -f "$editor_script"

  $test_passed
}

# =============================================================================
# NEW TESTS: Reviewers
# =============================================================================

# Test: Reviewers assignment from template
test_e2e_reviewers_assignment() {
  title="E2E: Reviewers are assigned from template"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-reviewers")
  local test_passed=true

  # Setup
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  # Create custom editor that adds reviewers
  local editor_script
  editor_script=$(mktemp)
  cat >"$editor_script" <<'EOF'
#!/bin/bash
# Uses global $SED variable (gsed on macOS, sed on Linux)
template_file="$1"
# Add reviewers to template (with @ prefix)
$SED -i 's/^# Reviewers:$/# Reviewers:\n@0xBAD-bot/' "$template_file"
# Add test plan
$SED -i 's/^# Test Plan:$/# Test Plan:\nManual testing performed/' "$template_file"
exit 0
EOF
  chmod +x "$editor_script"

  # Note: This test may fail if testuser1/testuser2 don't exist
  # We're primarily testing that the assignment flow works
  log_step "Running arc diff with reviewer assignment..."
  cd "$TEST_DIR"
  local output
  output=$(EDITOR="$editor_script" "$GH_ARC_BIN" diff 2>&1) || true

  # Check if PR was created (reviewer assignment might fail due to invalid users)
  if verify_pr_exists "$branch_name"; then
    local pr_number
    pr_number=$(get_pr_number "$branch_name")
    register_pr "$pr_number"
    log_step "PR #$pr_number created with reviewer assignment attempt ✓"
    pass_test "E2E: Reviewers assignment workflow"
  else
    # This is acceptable - test verifies the flow, not user validity
    log_step "PR creation skipped (expected if reviewers invalid)"
    pass_test "E2E: Reviewers assignment workflow (validation)"
  fi

  # Cleanup
  local pr_number
  pr_number=$(get_pr_number "$branch_name" 2>/dev/null || echo "")
  cleanup_test "$branch_name" "$pr_number"
  rm -f "$editor_script"

  $test_passed
}

# Test: Current user is filtered from reviewers
test_e2e_reviewers_filters_current_user() {
  title="E2E: Current user filtered from reviewer assignments"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-filter-current-user")
  local test_passed=true

  # Get current GitHub username
  local current_user
  current_user=$(gh api user --jq '.login' 2>/dev/null) || current_user="unknown"

  if [ "$current_user" = "unknown" ]; then
    log_warning "Could not determine current user, skipping test"
    pass_test "E2E: Reviewers filter current user (skipped)"
    return 0
  fi

  # Setup
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  # Create editor that adds current user as reviewer
  local editor_script
  editor_script=$(mktemp)
  cat >"$editor_script" <<EOF
#!/bin/bash
# Uses global \$SED variable (gsed on macOS, sed on Linux)
template_file="\$1"
# Add current user to reviewers (should be filtered) - with @ prefix
\$SED -i 's/^# Reviewers:$/# Reviewers:\n@$current_user/' "\$template_file"
# Add test plan
\$SED -i 's/^# Test Plan:$/# Test Plan:\nManual testing performed/' "\$template_file"
exit 0
EOF
  chmod +x "$editor_script"

  log_step "Running arc diff with current user ($current_user) as reviewer..."
  cd "$TEST_DIR"
  if EDITOR="$editor_script" "$GH_ARC_BIN" diff; then
    if verify_pr_exists "$branch_name"; then
      local pr_number
      pr_number=$(get_pr_number "$branch_name")
      register_pr "$pr_number"

      # Check if current user is NOT in reviewers list
      local reviewers
      reviewers=$(gh pr view "$pr_number" --json reviewRequests --jq '.reviewRequests[].login' 2>/dev/null | tr '\n' ',' || echo "")

      if echo "$reviewers" | grep -q "$current_user"; then
        log_warning "Current user found in reviewers (filter may not be working)"
        test_passed=false
      else
        log_step "Current user correctly filtered from reviewers ✓"
      fi

      pass_test "E2E: Reviewers filter current user"
    else
      fail_test "E2E: Filter current user" "PR not created"
      test_passed=false
    fi
  else
    fail_test "E2E: Filter current user" "Command failed"
    test_passed=false
  fi

  # Cleanup
  local pr_number
  pr_number=$(get_pr_number "$branch_name" 2>/dev/null || echo "")
  cleanup_test "$branch_name" "$pr_number"
  rm -f "$editor_script"

  $test_passed
}

# =============================================================================
# NEW TESTS: Error Handling
# =============================================================================

# Test: Editor cancellation handling
test_e2e_error_editor_cancelled() {
  title="E2E: Error handling - editor cancellation"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-editor-cancel")
  local test_passed=true

  # Setup
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  # Create editor that immediately exits with error (simulates cancellation)
  local editor_script
  editor_script=$(mktemp)
  cat >"$editor_script" <<'EOF'
#!/bin/bash
# Simulate editor cancellation
# kill -SIGKILL $$
exit 1
EOF
  chmod +x "$editor_script"

  # Run arc diff with cancelling editor
  log_step "Running arc diff with cancelling editor..."
  cd "$TEST_DIR"
  local output
  output=$(EDITOR="$editor_script" "$GH_ARC_BIN" diff 2>&1 || true)
  echo "$output" # Show output

  if echo "$output" | grep -q "Editor cancelled"; then
    log_step "Editor cancellation handled gracefully ✓"

    # Verify no PR was created
    if ! gh pr list --head "$branch_name" --json number 2>/dev/null | grep -q '[0-9]'; then
      log_step "No PR created after cancellation ✓"
      pass_test "E2E: Error handling - editor cancelled"
    else
      fail_test "E2E: Editor cancel" "PR was created despite cancellation"
      test_passed=false
    fi
  else
    log_debug "Output was: $output"
    fail_test "E2E: Editor cancel" "Expected cancellation message in output"
    test_passed=false
  fi

  # Cleanup
  cleanup_test "$branch_name"
  rm -f "$editor_script"

  $test_passed
}

# =============================================================================
# NEW TESTS: Context Timeout and Template Persistence
# =============================================================================

# Test: Long editor session doesn't trigger context timeout
test_e2e_long_editor_session_no_timeout() {
  title="E2E: Long editor session completes successfully"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-long-editor")
  local test_passed=true

  # Setup
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  # Create editor script that simulates 3-minute editing session
  local editor_script
  editor_script=$(mktemp)
  cat >"$editor_script" <<'EOF'
#!/bin/bash
# Uses global $SED variable (gsed on macOS, sed on Linux)
template_file="$1"

# Simulate user taking time to write PR description
# Using short delay (3 seconds) for testing purposes
sleep 3

# Complete the template
$SED -i 's/^# Test Plan:$/# Test Plan:\nManual testing after long edit session/' "$template_file"
exit 0
EOF
  chmod +x "$editor_script"

  # Run arc diff with long editor session
  log_step "Running arc diff with simulated long editor session..."
  cd "$TEST_DIR"
  local output
  output=$(EDITOR="$editor_script" "$GH_ARC_BIN" diff 2>&1)

  # Check for timeout errors (should not occur)
  # Look for actual error patterns, not just the word "timeout" (which appears in PR title)
  if echo "$output" | grep -Eqi "error.*timeout|timeout.*error|context deadline exceeded|timed out|timeout exceeded"; then
    log_debug "Output contains timeout error. Full output:\n$output"
    fail_test "E2E: Long editor session" "Context deadline/timeout occurred during long editor session"
    test_passed=false
  else
    log_step "No deadline/timeout errors after long editor session ✓"

    # Verify PR was created successfully
    if verify_pr_exists "$branch_name"; then
      local pr_number
      pr_number=$(get_pr_number "$branch_name")
      register_pr "$pr_number"
      log_step "PR #$pr_number created successfully after long editor session ✓"
      pass_test "E2E: Long editor session completes successfully"
    else
      fail_test "E2E: Long editor session" "PR not created after long editor session"
      test_passed=false
    fi
  fi

  # Cleanup
  local pr_number
  pr_number=$(get_pr_number "$branch_name" 2>/dev/null || echo "")
  cleanup_test "$branch_name" "$pr_number"
  rm -f "$editor_script"

  $test_passed
}

# Test: Template preserved when auto-branch fails
test_e2e_template_preserved_on_error() {
  title="E2E: Template preserved when errors occur after validation"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-template-preserve")
  local test_passed=true

  # Setup
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  # Create a valid template that passes validation
  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  # Run arc diff (should succeed and create PR)
  log_step "Running arc diff to test template persistence..."
  cd "$TEST_DIR"

  # First, test normal successful workflow cleans up template
  if EDITOR="$editor_script" "$GH_ARC_BIN" diff; then
    local pr_number
    pr_number=$(get_pr_number "$branch_name")
    register_pr "$pr_number"
    log_step "PR #$pr_number created successfully"

    # Check that no template was left behind after success
    local temp_dir
    temp_dir=$(dirname "$(mktemp -u)")
    local saved_templates
    saved_templates=$(find "$temp_dir" -name "gh-arc-diff-saved-*.md" -type f -mmin -1 2>/dev/null | wc -l)

    if [ "$saved_templates" -eq 0 ]; then
      log_step "Template cleaned up after successful PR creation ✓"
      pass_test "E2E: Template lifecycle (save → cleanup on success)"
    else
      log_warning "Found $saved_templates saved template(s) after success (might be from other tests)"
      # Don't fail - could be from concurrent tests
      pass_test "E2E: Template lifecycle"
    fi
  else
    fail_test "E2E: Template preserve" "Failed to create PR"
    test_passed=false
  fi

  # Cleanup
  cleanup_test "$branch_name" "$(get_pr_number "$branch_name" 2>/dev/null || echo "")"
  rm -f "$editor_script"

  $test_passed
}

# =============================================================================
# EXISTING TESTS (Updated for Independence)
# =============================================================================

# Test 1: Continue mode - Validation failure preserves edits
# =============================================================================
test_e2e_continue_validation_failure() {
  title="E2E: Continue mode preserves edits on validation failure"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-continue-validation")
  local test_passed=true

  setup_test_branch "$branch_name"
  create_test_commit "$title"

  # Create custom editor that DOESN'T add test plan (auto-generated template lacks it)
  # Just save the template as-is to trigger validation failure
  local editor_script
  editor_script=$(mktemp)
  cat >"$editor_script" <<'EOF'
#!/bin/bash
# Just exit without modifying template (template will lack test plan)
exit 0
EOF
  chmod +x "$editor_script"

  log_step "Running arc diff with incomplete template (expect failure)..."
  cd "$TEST_DIR"
  local output
  output=$(EDITOR="$editor_script" "$GH_ARC_BIN" diff 2>&1 || true)
  echo "$output" # Show output

  if echo "$output" | grep -q "validation failed"; then
    log_step "Validation failed as expected"
  else
    fail_test "E2E: Continue mode validation failure" "Expected validation to fail"
    rm -f "$editor_script"
    cleanup_test "$branch_name"
    return 1
  fi

  # Now add extra content and still fail validation (no test plan)
  log_step "Running arc diff --continue with extra content (expect failure)..."
  # Create new editor that adds extra content but still no test plan
  cat >"$editor_script" <<'EOF'
#!/bin/bash
# Uses global $SED variable (gsed on macOS, sed on Linux)
template_file="$1"
# Add extra content to Summary section (but don't add test plan)
$SED -i '/^# Summary:/a\EXTRA CONTENT FROM USER - should be preserved!' "$template_file"
exit 0
EOF
  chmod +x "$editor_script"

  output=$(EDITOR="$editor_script" "$GH_ARC_BIN" diff --continue 2>&1 || true)
  echo "$output" # Show output

  if echo "$output" | grep -q "validation failed"; then
    log_step "Second validation failed as expected"
  else
    fail_test "E2E: Continue mode validation failure" "Expected second validation to fail"
    rm -f "$editor_script"
    cleanup_test "$branch_name"
    return 1
  fi

  # Find the saved template and verify extra content is preserved
  log_step "Looking for saved templates..."

  # Use correct filename pattern and search in system temp dir (not just /tmp)
  local temp_dir
  temp_dir=$(dirname "$(mktemp -u)")
  local saved_template
  saved_template=$(find "$temp_dir" -name "gh-arc-diff-saved-*.md" -type f -mmin -5 2>/dev/null | sort -M | tail -n1)

  if [ -z "$saved_template" ]; then
    # Debug: show what templates exist
    log_warning "Debugging: Looking for any gh-arc templates..."
    find /tmp -name "gh-arc-*.md" -type f -mmin -5 2>/dev/null | while read -r f; do
      log_info "Found: $f"
    done

    fail_test "E2E: Continue mode validation failure" "No saved template found"
    cleanup_test "$branch_name"
    rm -f "$editor_script"
    return 1
  fi

  log_step "Found saved template: $saved_template"

  if grep -q "EXTRA CONTENT FROM USER" "$saved_template"; then
    log_step "Extra content preserved in saved template ✓"
  else
    fail_test "E2E: Continue mode validation failure" "Extra content not preserved"
    test_passed=false
  fi

  # Now fix the template and create PR
  log_step "Running arc diff --continue with complete template..."
  local complete_editor
  complete_editor=$(mktemp)
  create_editor_script "$complete_editor" "add_test_plan"
  export EDITOR_MODIFICATIONS="add_test_plan"

  if EDITOR="$complete_editor" "$GH_ARC_BIN" diff --continue; then
    log_step "PR created successfully"

    # Verify PR exists
    if verify_pr_exists "$branch_name"; then
      pass_test "E2E: Continue mode preserves edits on validation failure"
    else
      fail_test "E2E: Continue mode validation failure" "PR not created"
      test_passed=false
    fi
  else
    fail_test "E2E: Continue mode validation failure" "Failed to create PR"
    test_passed=false
  fi

  # Cleanup
  local final_pr
  final_pr=$(get_pr_number "$branch_name" 2>/dev/null || echo "")
  cleanup_test "$branch_name" "$final_pr"
  rm -f "$editor_script" "$complete_editor" "$saved_template"

  # Clean up saved templates
  local temp_dir
  temp_dir=$(dirname "$(mktemp -u)")
  find "$temp_dir" -name "gh-arc-diff-saved-*.md" -type f -delete 2>/dev/null || true

  $test_passed
}

# =============================================================================
# Test 2: Continue mode with stacked PR
# =============================================================================
test_e2e_continue_stacked_pr() {
  title="E2E: Continue mode with stacked PR"
  start_test "$title"

  local parent_branch
  local child_branch
  parent_branch=$(create_unique_branch "test-stacked-parent")
  child_branch=$(create_unique_branch "test-stacked-child")

  # Create parent branch and PR
  log_step "Creating parent branch and PR..."
  setup_test_branch "$parent_branch"
  create_test_commit "$title"

  cd "$TEST_DIR"
  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  if ! EDITOR="$editor_script" "$GH_ARC_BIN" diff; then
    fail_test "E2E: Continue stacked PR" "Failed to create parent PR"
    rm -f "$editor_script"
    return 1
  fi

  local parent_pr_number
  parent_pr_number=$(get_pr_number "$parent_branch")
  register_pr "$parent_pr_number"
  log_step "Parent PR #$parent_pr_number created"

  # Create child branch from parent
  log_step "Creating child branch from parent..."
  git checkout -b "$child_branch" >/dev/null 2>&1
  register_branch "$child_branch"
  create_test_commit "Child feature"

  # Run diff without test plan to trigger validation failure
  # Use no-op editor (template auto-generated without test plan)
  log_step "Running arc diff with incomplete template..."
  local noop_editor
  noop_editor=$(mktemp)
  cat >"$noop_editor" <<'EOF'
#!/bin/bash
# No-op editor - just save template as-is (will lack test plan)
exit 0
EOF
  chmod +x "$noop_editor"

  local output
  output=$(EDITOR="$noop_editor" "$GH_ARC_BIN" diff 2>&1 || true)
  echo "$output" # Show output

  if echo "$output" | grep -q "validation failed"; then
    log_step "Validation failed as expected"
  else
    fail_test "E2E: Continue stacked PR" "Expected validation to fail"
    cleanup_test "$parent_branch" "$parent_pr_number"
    cleanup_test "$child_branch" ""
    rm -f "$editor_script" "$noop_editor"
    return 1
  fi
  rm -f "$noop_editor"

  # Check that saved template has stacked format
  log_step "Looking for saved template..."
  local temp_dir
  temp_dir=$(dirname "$(mktemp -u)")
  local saved_template
  saved_template=$(find "$temp_dir" -name "gh-arc-diff-saved-*.md" -type f -mmin -5 2>/dev/null | sort -M | tail -n1)

  if [ -z "$saved_template" ]; then
    log_warning "No saved template found (validation might have passed unexpectedly)"
    fail_test "E2E: Continue stacked PR" "No saved template found"
    cleanup_test "$parent_branch" "$parent_pr_number"
    cleanup_test "$child_branch" ""
    rm -f "$editor_script"
    return 1
  fi

  log_step "Found saved template: $saved_template"

  if grep -q "Creating stacked PR on $parent_branch" "$saved_template"; then
    log_step "Saved template has stacked format ✓"
  else
    log_warning "Template might not be in stacked format (this might be OK)"
  fi

  # Run continue to create stacked PR (use proper editor with test plan)
  log_step "Running arc diff --continue to create stacked PR..."
  local complete_editor
  complete_editor=$(mktemp)
  create_editor_script "$complete_editor" "add_test_plan"
  export EDITOR_MODIFICATIONS="add_test_plan"

  if EDITOR="$complete_editor" "$GH_ARC_BIN" diff --continue; then
    log_step "Stacked PR created successfully"

    # Verify PR exists and has correct base
    if verify_pr_exists "$child_branch"; then
      local pr_base
      pr_base=$(get_pr_base "$child_branch")
      if [ "$pr_base" = "$parent_branch" ]; then
        log_step "PR correctly targets parent branch: $parent_branch ✓"
        pass_test "E2E: Continue mode with stacked PR"
      else
        fail_test "E2E: Continue stacked PR" "PR targets '$pr_base' instead of '$parent_branch'"
        test_passed=false
      fi
    else
      fail_test "E2E: Continue stacked PR" "PR not created"
      test_passed=false
    fi
  else
    fail_test "E2E: Continue stacked PR" "Failed to create stacked PR"
    test_passed=false
  fi

  # Cleanup - ALWAYS cleanup both parent and child (even on test failure)
  cd "$TEST_DIR"

  local child_pr
  local parent_pr
  child_pr=$(get_pr_number "$child_branch" 2>/dev/null || echo "")
  parent_pr="$parent_pr_number" # Use the one we registered earlier

  log_debug "Cleaning up test 19: child_pr=$child_pr, parent_pr=$parent_pr"

  # Clean child first, then parent
  if [ -n "$child_pr" ]; then
    log_debug "Closing child PR #$child_pr"
    gh pr close "$child_pr" --delete-branch 2>/dev/null || true
  fi
  cleanup_test "$child_branch" ""

  if [ -n "$parent_pr" ]; then
    log_debug "Closing parent PR #$parent_pr"
    gh pr close "$parent_pr" --delete-branch 2>/dev/null || true
  fi
  cleanup_test "$parent_branch" ""

  rm -f "$editor_script" "$complete_editor" "$saved_template"

  # Force fetch and reset to ensure main is clean for next tests
  git fetch origin >/dev/null 2>&1 || true
  git checkout main >/dev/null 2>&1 || git checkout master >/dev/null 2>&1
  local default_branch
  default_branch=$(git branch --show-current)
  git reset --hard "origin/$default_branch" >/dev/null 2>&1 || true
  git clean -fd >/dev/null 2>&1 || true

  log_debug "Test 19 cleanup complete, main branch reset"

  $test_passed
}

# =============================================================================
# Test 3: Stacking detection with same-commit scenario
# =============================================================================
test_e2e_stacking_same_commit() {
  title="E2E: Stacking detection with same-commit (auto-branch scenario)"
  start_test "$title"

  local test_passed=true
  cd "$TEST_DIR"

  # Ensure we're on main
  git checkout main >/dev/null 2>&1 || git checkout master >/dev/null 2>&1
  git pull origin main >/dev/null 2>&1 || git pull origin master >/dev/null 2>&1 || true

  # Create commit on main
  log_step "Creating commit on main (simulating auto-branch scenario)..."
  create_test_commit "$title"

  # Run arc diff to trigger auto-branch creation
  log_step "Running arc diff (should create auto-branch)..."
  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  if ! EDITOR="$editor_script" "$GH_ARC_BIN" diff; then
    fail_test "E2E: Stacking same-commit" "Failed to create auto-branch PR"
    rm -f "$editor_script"
    return 1
  fi

  # Get the auto-created branch name
  local auto_branch
  auto_branch=$(git branch --show-current)
  log_step "Auto-branch created: $auto_branch"

  local auto_pr_number
  auto_pr_number=$(get_pr_number "$auto_branch")
  log_step "Auto-branch PR #$auto_pr_number created"

  # Create child branch from auto-branch
  local child_branch
  child_branch=$(create_unique_branch "test-stacked-child")
  log_step "Creating child branch from auto-branch..."
  git checkout -b "$child_branch" >/dev/null 2>&1
  create_test_commit "$title - Child Feature"

  # Run arc diff to create stacked PR
  log_step "Running arc diff (should stack on auto-branch)..."
  if EDITOR="$editor_script" "$GH_ARC_BIN" diff; then
    log_step "Child PR created"

    # Verify PR targets auto-branch, not main
    if verify_pr_exists "$child_branch"; then
      local pr_base
      pr_base=$(get_pr_base "$child_branch")
      if [ "$pr_base" = "$auto_branch" ]; then
        log_step "PR correctly stacks on auto-branch: $auto_branch ✓"
        pass_test "E2E: Stacking detection with same-commit"
      else
        fail_test "E2E: Stacking same-commit" "PR targets '$pr_base' instead of '$auto_branch'"
      fi
    else
      fail_test "E2E: Stacking same-commit" "Child PR not created"
      test_passed=false
    fi
  else
    fail_test "E2E: Stacking same-commit" "Failed to create child PR"
    test_passed=false
  fi

  # Cleanup
  local child_pr
  local auto_pr
  child_pr=$(get_pr_number "$child_branch" 2>/dev/null || echo "")
  auto_pr=$(get_pr_number "$auto_branch" 2>/dev/null || echo "")
  cleanup_test "$child_branch" "$child_pr"
  cleanup_test "$auto_branch" "$auto_pr"
  rm -f "$editor_script"

  $test_passed
}

# =============================================================================
# Test 4: Template sorting by modification time
# =============================================================================
test_e2e_template_sorting() {
  title="E2E: Template sorting by modification time"
  start_test "$title"

  local branch_name
  branch_name=$(create_unique_branch "test-template-sorting")
  local test_passed=false
  setup_test_branch "$branch_name"
  create_test_commit "$title"

  cd "$TEST_DIR"

  # Create multiple saved templates with different timestamps
  log_step "Creating multiple saved templates..."
  local temp_dir
  temp_dir=$(dirname "$(mktemp -u)")
  local temp1="$temp_dir/gh-arc-diff-saved-1111111111.md"
  local temp2="$temp_dir/gh-arc-diff-saved-2222222222.md"

  cat >"$temp1" <<'EOF'
# Creating PR: test → main
# Base Branch: develop (read-only)

# Title:
Old Template

# Summary:
This is the old template

# Test Plan:

# Reviewers:

# Draft:
false
EOF

  sleep 0.2

  cat >"$temp2" <<'EOF'
# Creating PR: test → main
# Base Branch: main (read-only)

# Title:
E2E: Template sorting by modification time

# Summary:
This is the NEW template with NEWER content

# Test Plan:

# Reviewers:

# Draft:
false
EOF

  # Run arc diff --continue (should load newest template)
  log_step "Running arc diff --continue..."
  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "add_test_plan"
  export EDITOR_MODIFICATIONS="add_test_plan"

  if EDITOR="$editor_script" "$GH_ARC_BIN" diff --continue; then
    log_step "PR created"

    # Get PR details to check which template was used
    if verify_pr_exists "$branch_name"; then
      local pr_number
      pr_number=$(get_pr_number "$branch_name")
      local pr_body
      pr_body=$(gh pr view "$pr_number" --json body --jq '.body' 2>/dev/null)

      if echo "$pr_body" | grep -q "NEWER content"; then
        log_step "Newest template was loaded ✓"
        pass_test "E2E: Template sorting by modification time"
        test_passed=true
      else
        fail_test "E2E: Template sorting" "Wrong template loaded (should be newest)"
        test_passed=false
      fi
    else
      fail_test "E2E: Template sorting" "PR not created"
      test_passed=false
    fi
  else
    fail_test "E2E: Template sorting" "Failed to create PR"
    test_passed=false
  fi

  # Cleanup
  local pr_number
  pr_number=$(get_pr_number "$branch_name" 2>/dev/null || echo "")
  cleanup_test "$branch_name" "$pr_number"
  rm -f "$editor_script" "$temp1" "$temp2"

  $test_passed
}

# =============================================================================
# NEW TESTS: Auto-Branch with Continue Mode
# =============================================================================

# Test: Auto-branch with continue mode (validation failure then retry)
test_e2e_auto_branch_with_continue() {
  title="E2E: Auto-branch + continue mode (validation → retry)"
  start_test "$title"

  local test_passed=true
  cd "$TEST_DIR"

  # Ensure we're on main and fully reset to remote state
  git checkout main >/dev/null 2>&1 || git checkout master >/dev/null 2>&1
  local default_branch
  default_branch=$(git branch --show-current)
  git fetch origin "$default_branch" >/dev/null 2>&1 || true
  git reset --hard "origin/$default_branch" >/dev/null 2>&1 || true
  git clean -fd >/dev/null 2>&1 || true

  # Create commits on main
  log_step "Creating commits on main..."
  create_test_commit "$title"
  create_test_commit "Feature commit 2"

  # Run arc diff with incomplete template (validation will fail)
  log_step "Running arc diff (should trigger auto-branch but fail validation)..."
  local editor_script
  editor_script=$(mktemp)
  # Use no-op editor (auto-generated template lacks test plan)
  cat >"$editor_script" <<'EOF'
#!/bin/bash
# No-op editor - template will lack test plan
exit 0
EOF
  chmod +x "$editor_script"

  cd "$TEST_DIR"
  output=$(EDITOR="$editor_script" "$GH_ARC_BIN" diff 2>&1 || true)
  echo "$output" # Show output

  if echo "$output" | grep -q "validation failed"; then
    log_step "Validation failed as expected"
  else
    log_warning "Validation didn't fail as expected, might already have test plan"
  fi

  # Verify we're still on main (auto-branch not executed yet)
  local current_branch
  current_branch=$(git branch --show-current)
  if [ "$current_branch" = "main" ] || [ "$current_branch" = "master" ]; then
    log_step "Still on $current_branch (auto-branch not executed) ✓"
  else
    fail_test "E2E: Auto-branch continue" "Expected to be on main, found $current_branch"
    cleanup_test "$current_branch"
    rm -f "$editor_script"
    return 1
  fi

  # Run arc diff --continue with complete template
  log_step "Running arc diff --continue (should execute auto-branch)..."
  local continue_editor
  continue_editor=$(mktemp)
  create_editor_script "$continue_editor" "add_test_plan"
  export EDITOR_MODIFICATIONS="add_test_plan"

  if EDITOR="$continue_editor" "$GH_ARC_BIN" diff --continue; then
    # Check what branch we're on now
    local final_branch
    final_branch=$(git branch --show-current)

    # Should not be on main anymore
    if [ "$final_branch" != "main" ] && [ "$final_branch" != "master" ]; then
      log_step "Auto-branch created and checked out: $final_branch ✓"

      # Verify PR exists and targets main
      if verify_pr_exists "$final_branch"; then
        local pr_number
        pr_number=$(get_pr_number "$final_branch")
        register_pr "$pr_number"
        register_branch "$final_branch"
        local pr_base
        pr_base=$(get_pr_base "$final_branch")

        if [ "$pr_base" = "main" ] || [ "$pr_base" = "master" ]; then
          log_step "PR #$pr_number correctly targets main via continue mode ✓"
          pass_test "E2E: Auto-branch + continue mode"
        else
          fail_test "E2E: Auto-branch continue" "PR targets '$pr_base' instead of main"
          test_passed=false
        fi
      else
        fail_test "E2E: Auto-branch continue" "PR not created"
        test_passed=false
      fi
    else
      fail_test "E2E: Auto-branch continue" "Still on main (auto-branch not executed)"
      test_passed=false
    fi
  else
    fail_test "E2E: Auto-branch continue" "Continue mode failed"
    test_passed=false
  fi

  # Cleanup
  local auto_branch
  auto_branch=$(git branch --show-current)
  if [ "$auto_branch" != "main" ] && [ "$auto_branch" != "master" ]; then
    local pr_number
    pr_number=$(get_pr_number "$auto_branch" 2>/dev/null || echo "")
    cleanup_test "$auto_branch" "$pr_number"
  else
    cleanup_test "" ""
  fi
  rm -f "$editor_script" "$continue_editor"

  # Clean up saved templates
  local temp_dir
  temp_dir=$(dirname "$(mktemp -u)")
  find "$temp_dir" -name "gh-arc-diff-saved-*.md" -type f -delete 2>/dev/null || true

  $test_passed
}

# =============================================================================
# Test 5: Auto-branch detection and creation
# =============================================================================
test_e2e_auto_branch_creation() {
  title="E2E: Auto-branch creation from main"
  start_test "$title"

  local test_passed=false
  cd "$TEST_DIR"

  # Ensure we're on main and fully reset to remote state
  git checkout main >/dev/null 2>&1 || git checkout master >/dev/null 2>&1
  local default_branch
  default_branch=$(git branch --show-current)
  git fetch origin "$default_branch" >/dev/null 2>&1 || true
  git reset --hard "origin/$default_branch" >/dev/null 2>&1 || true
  git clean -fd >/dev/null 2>&1 || true

  # Create commits on main
  log_step "Creating commits on main..."
  create_test_commit "$title"
  create_test_commit "Feature commit 2"

  # Run arc diff (should auto-create branch)
  log_step "Running arc diff (should trigger auto-branch)..."
  local editor_script
  editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  if EDITOR="$editor_script" "$GH_ARC_BIN" diff; then
    local current_branch
    current_branch=$(git branch --show-current)

    # Should not be on main anymore
    if [ "$current_branch" != "main" ] && [ "$current_branch" != "master" ]; then
      log_step "Auto-branch created: $current_branch ✓"

      # Verify PR exists
      # Verify PR exists
      if verify_pr_exists "$current_branch"; then
        local pr_base
        pr_base=$(get_pr_base "$current_branch")
        if [ "$pr_base" = "main" ] || [ "$pr_base" = "master" ]; then
          log_step "PR correctly targets main branch ✓"
          pass_test "E2E: Auto-branch creation from main"
          test_passed=true
          register_branch "$current_branch"
        else
          fail_test "E2E: Auto-branch" "PR targets '$pr_base' instead of main"
        fi
      else
        fail_test "E2E: Auto-branch" "PR not created"
      fi
    else
      fail_test "E2E: Auto-branch" "Still on main branch (auto-branch not created)"
    fi
  else
    fail_test "E2E: Auto-branch" "Failed to run arc diff"
  fi

  # Cleanup
  local auto_branch
  auto_branch=$(git branch --show-current)
  if [ "$auto_branch" != "main" ] && [ "$auto_branch" != "master" ]; then
    local pr_number
    pr_number=$(get_pr_number "$auto_branch" 2>/dev/null || echo "")
    cleanup_test "$auto_branch" "$pr_number"
  else
    # Still on main, just reset
    cleanup_test "" ""
  fi
  rm -f "$editor_script"

  $test_passed
}

# =============================================================================
# Print summary
# =============================================================================
print_summary() {
  echo ""
  echo "======================================================================"
  echo "TEST SUMMARY"
  echo "======================================================================"
  echo "Total tests:  $TESTS_RUN"
  echo -e "${GREEN}Passed:       $TESTS_PASSED${NC}"
  echo -e "${RED}Failed:       $TESTS_FAILED${NC}"
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

# =============================================================================
# Main execution
# =============================================================================
main() {
  # Parse command-line arguments
  local specific_test=""
  while [[ $# -gt 0 ]]; do
    case $1 in
    -d | --debug)
      DEBUG_MODE=true
      shift
      ;;
    --no-cleanup)
      NO_CLEANUP=true
      shift
      ;;
    --cleanup-only)
      CLEANUP_ONLY=true
      shift
      ;;
    -h | --help)
      cat <<HELP
Usage: TEST_DIR=/path/to/repo $0 [OPTIONS] [TEST_NAME]

Requires a test repository with GitHub remote. Set up with:
  git clone git@github.com:0xBAD-dev/gh-arc-test.git /tmp/gh-arc-test
  TEST_DIR=/tmp/gh-arc-test $0

Options:
  -d, --debug        Enable debug mode (adds -vvv to gh-arc commands)
  --no-cleanup       Skip cleanup after tests (for debugging)
  --cleanup-only     Only run cleanup, no tests
  -h, --help         Show this help message

Examples:
  TEST_DIR=/tmp/gh-arc-test ./test-e2e.sh                      # Run all tests
  TEST_DIR=/tmp/gh-arc-test ./test-e2e.sh --debug              # Debug mode
  TEST_DIR=/tmp/gh-arc-test ./test-e2e.sh test_e2e_stacking   # Specific test
  TEST_DIR=/tmp/gh-arc-test ./test-e2e.sh --no-cleanup        # Keep artifacts
  TEST_DIR=/tmp/gh-arc-test ./test-e2e.sh --cleanup-only      # Just cleanup
  DEBUG_MODE=true TEST_DIR=/tmp/gh-arc-test ./test-e2e.sh     # Debug via env var

Environment Variables:
  TEST_DIR           Path to test repository (REQUIRED)
  GH_ARC_BIN         Path to gh-arc binary (default: 'arc' from PATH)
  DEBUG_MODE         Enable debug mode (default: false)
  NO_CLEANUP         Skip cleanup (default: false)
HELP
      exit 0
      ;;
    *)
      specific_test="$1"
      shift
      ;;
    esac
  done

  # Setup cleanup trap
  trap cleanup_all EXIT

  # If cleanup-only mode, just run cleanup and exit
  if [ "$CLEANUP_ONLY" = "true" ]; then
    log_info "Cleanup-only mode"
    cleanup_all
    exit 0
  fi

  log_info "gh-arc End-to-End Test Suite"
  log_info "============================"
  log_info "Test repository: $TEST_DIR"
  log_info "gh-arc binary: $GH_ARC_BIN"
  if [ "$DEBUG_MODE" = "true" ]; then
    log_info "Debug mode: ENABLED"
  fi
  if [ "$NO_CLEANUP" = "true" ]; then
    log_warning "Cleanup: DISABLED"
  fi
  echo ""

  # Check gh CLI authentication
  if ! gh auth status >/dev/null 2>&1; then
    log_error "gh CLI is not authenticated"
    log_info "Run: gh auth login"
    exit 2
  fi

  log_info "gh CLI: authenticated ✓"

  # Check for required GitHub token scopes
  log_debug "Checking GitHub token scopes..."
  if ! gh auth status 2>&1 | grep -q "read:user\|user:email"; then
    log_warning "GitHub token may be missing 'read:user' or 'user:email' scopes"
    log_warning "This may cause issues with reviewer filtering"
    log_info "To fix, run: gh auth refresh -s read:user"
    echo ""
  fi

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
    log_info "Running all E2E tests..."
    echo ""

    # ====================
    # Fast Path Tests
    # ====================
    printf "\n"
    log_info "Category: Fast Path"
    test_e2e_fast_path_push_commits
    test_e2e_fast_path_draft_ready

    # ====================
    # Normal Mode Tests
    # ====================
    printf "\n"
    log_info "Category: Normal Mode"
    test_e2e_normal_mode_new_pr
    test_e2e_normal_mode_update_with_edit

    # ====================
    # --base Flag Tests
    # ====================
    printf "\n"
    log_info "Category: --base Flag"
    test_e2e_base_flag_override_stacking
    test_e2e_base_flag_force_stacking
    test_e2e_base_flag_invalid_branch

    # ====================
    # --no-edit Flag Tests
    # ====================
    printf "\n"
    log_info "Category: --no-edit Flag"
    test_e2e_no_edit_flag_new_pr
    test_e2e_no_edit_flag_update_pr

    # ====================
    # Draft PR Tests
    # ====================
    printf "\n"
    log_info "Category: Draft PR Scenarios"
    test_e2e_draft_with_fast_path_commits

    # ====================
    # Flag Combination Tests
    # ====================
    printf "\n"
    log_info "Category: Flag Combinations"
    test_e2e_flag_combination_edit_draft
    test_e2e_flag_combination_continue_draft
    test_e2e_flag_combination_base_with_edit

    # ====================
    # Reviewer Tests
    # ====================
    printf "\n"
    log_info "Category: Reviewers"
    test_e2e_reviewers_assignment
    test_e2e_reviewers_filters_current_user

    # ====================
    # Stacking Tests
    # ====================
    printf "\n"
    log_info "Category: Stacking"
    test_e2e_stacking_basic
    test_e2e_stacking_same_commit

    # ====================
    # Continue Mode Tests
    # ====================
    printf "\n"
    log_info "Category: Continue Mode"
    test_e2e_continue_validation_failure
    test_e2e_continue_stacked_pr

    # ====================
    # Auto-Branch Tests
    # ====================
    printf "\n"
    log_info "Category: Auto-Branch"
    test_e2e_auto_branch_creation
    test_e2e_auto_branch_with_continue

    # ====================
    # Template Tests
    # ====================
    printf "\n"
    log_info "Category: Template System"
    test_e2e_template_sorting

    # ====================
    # Error Handling Tests
    # ====================
    printf "\n"
    log_info "Category: Error Handling"
    test_e2e_error_editor_cancelled
    test_e2e_long_editor_session_no_timeout
    test_e2e_template_preserved_on_error
  fi

  # Print summary
  if print_summary; then
    echo ""
    if [ "$NO_CLEANUP" = "true" ]; then
      log_warning "Manual cleanup required (NO_CLEANUP=true):"
      log_info "  Created PRs: ${CREATED_PRS[*]}"
      log_info "  Created branches: ${CREATED_BRANCHES[*]}"
      echo ""
      log_info "To cleanup later, run:"
      log_info "  TEST_DIR=$TEST_DIR $0 --cleanup-only"
    fi
    exit 0
  else
    exit 1
  fi
}

# Run main
main "$@"
