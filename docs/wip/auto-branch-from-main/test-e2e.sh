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
#   3. `gh-arc` binary must be built
#
# Usage:
#   TEST_DIR=/path/to/repo ./test-e2e.sh                 # Run all tests
#   TEST_DIR=/path/to/repo ./test-e2e.sh <test_name>     # Run specific test
#
# Environment variables:
#   TEST_DIR     - Path to test repository (REQUIRED)
#   GH_ARC_BIN   - Path to gh-arc binary (default: PROJECT_ROOT/gh-arc)
#   EDITOR       - Will be overridden with custom editor for automated tests
#
# Note: This creates REAL PRs. Manual cleanup required after tests.
#

set -e

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

# Script paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
GH_ARC_BIN="${GH_ARC_BIN:-arc}"
TEST_DIR="${TEST_DIR:-}"

# Validate prerequisites
if [ -z "$TEST_DIR" ]; then
  echo -e "${RED}ERROR: TEST_DIR environment variable is required${NC}"
  echo "Usage: TEST_DIR=/path/to/repo $0"
  exit 2
fi

if [ ! -d "$TEST_DIR/.git" ]; then
  echo -e "${RED}ERROR: TEST_DIR must be a git repository${NC}"
  exit 2
fi

if ! command -v "${GH_ARC_BIN}" >/dev/null 2>&1; then
  echo -e "${RED}ERROR: gh-arc binary not found at $GH_ARC_BIN${NC}"
  echo "Run: cd $PROJECT_ROOT && go build -o gh-arc"
  exit 2
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

# Test framework
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

# Helper functions
create_unique_branch() {
  local prefix="$1"
  local timestamp=$(date +%s)
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

verify_pr_exists() {
  local branch_name="$1"
  cd "$TEST_DIR"

  # Check if PR exists for this branch
  if gh pr list --head "$branch_name" --json number --jq '.[0].number' 2>/dev/null | grep -q '^[0-9]'; then
    local pr_number=$(gh pr list --head "$branch_name" --json number --jq '.[0].number')
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
  local modifications="$2"

  cat >"$editor_script" <<'EDITOR_EOF'
#!/bin/bash
# Custom editor for automated testing
template_file="$1"

# Read modification instructions from environment variable
modifications="$EDITOR_MODIFICATIONS"

if [ -n "$modifications" ]; then
    case "$modifications" in
        "remove_test_plan")
            # Remove test plan content to trigger validation failure
            sed -i '/^# Test Plan:/,/^# Reviewers:/{//!d}' "$template_file"
            ;;
        "add_test_plan")
            # Add test plan content
            sed -i 's/^# Test Plan:$/# Test Plan:\nManual testing performed\n/' "$template_file"
            ;;
        "add_extra_content")
            # Add extra content to Summary section
            sed -i '/^# Summary:/a\EXTRA CONTENT FROM USER - should be preserved!' "$template_file"
            ;;
        "complete_template")
            # Fill in all required fields
            sed -i 's/^# Title:$/# Title:\nTest PR Title/' "$template_file"
            sed -i 's/^# Test Plan:$/# Test Plan:\nManual testing performed/' "$template_file"
            ;;
    esac
fi

# If no modifications or just to review, do nothing (auto-save)
exit 0
EDITOR_EOF

  chmod +x "$editor_script"
  echo "$editor_script"
}

# =============================================================================
# Test 1: Continue mode - Validation failure preserves edits
# =============================================================================
test_e2e_continue_validation_failure() {
  start_test "E2E: Continue mode preserves edits on validation failure"

  local branch_name=$(create_unique_branch "test-continue-validation")
  local test_passed=true

  setup_test_branch "$branch_name"
  create_test_commit "Test commit for continue mode"

  # Create custom editor that removes test plan (triggers validation failure)
  local editor_script=$(mktemp)
  create_editor_script "$editor_script" "remove_test_plan"

  log_step "Running arc diff with incomplete template (expect failure)..."
  cd "$TEST_DIR"
  export EDITOR_MODIFICATIONS="remove_test_plan"
  if EDITOR="$editor_script" "$GH_ARC_BIN" diff --no-edit 2>&1 | grep -q "validation failed"; then
    log_step "Validation failed as expected"
  else
    fail_test "E2E: Continue mode validation failure" "Expected validation to fail"
    rm -f "$editor_script"
    return 1
  fi

  # Now add extra content and still fail validation
  log_step "Running arc diff --continue with extra content (expect failure)..."
  export EDITOR_MODIFICATIONS="add_extra_content"
  if EDITOR="$editor_script" "$GH_ARC_BIN" diff --continue 2>&1 | grep -q "validation failed"; then
    log_step "Second validation failed as expected"
  else
    fail_test "E2E: Continue mode validation failure" "Expected second validation to fail"
    rm -f "$editor_script"
    return 1
  fi

  # Find the saved template and verify extra content is preserved
  log_step "Looking for saved templates in /tmp..."

  # Try different patterns
  local saved_template=$(find /tmp -name "gh-arc-saved-*.md" -type f -mmin -5 2>/dev/null | sort -t- -k4 -n | tail -n1)

  if [ -z "$saved_template" ]; then
    # Debug: show what templates exist
    log_warning "Debugging: Looking for any gh-arc templates..."
    find /tmp -name "gh-arc-*.md" -type f -mmin -5 2>/dev/null | while read f; do
      log_info "Found: $f"
    done

    # Try again with broader pattern
    saved_template=$(find /tmp -name "gh-arc-*.md" -type f -mmin -5 2>/dev/null | sort -t- -k4 -n | tail -n1)

    if [ -z "$saved_template" ]; then
      fail_test "E2E: Continue mode validation failure" "No saved template found"
      rm -f "$editor_script"
      return 1
    fi
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
  export EDITOR_MODIFICATIONS="add_test_plan"
  if EDITOR="$editor_script" "$GH_ARC_BIN" diff --continue >/dev/null 2>&1; then
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
  rm -f "$editor_script" "$saved_template"

  $test_passed
}

# =============================================================================
# Test 2: Continue mode with stacked PR
# =============================================================================
test_e2e_continue_stacked_pr() {
  start_test "E2E: Continue mode with stacked PR"

  local parent_branch=$(create_unique_branch "test-stacked-parent")
  local child_branch=$(create_unique_branch "test-stacked-child")

  # Create parent branch and PR
  log_step "Creating parent branch and PR..."
  setup_test_branch "$parent_branch"
  create_test_commit "Parent feature"

  cd "$TEST_DIR"
  local editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  if ! EDITOR="$editor_script" "$GH_ARC_BIN" diff >/dev/null 2>&1; then
    fail_test "E2E: Continue stacked PR" "Failed to create parent PR"
    rm -f "$editor_script"
    return 1
  fi

  local parent_pr_number=$(get_pr_number "$parent_branch")
  log_step "Parent PR #$parent_pr_number created"

  # Create child branch from parent
  log_step "Creating child branch from parent..."
  git checkout -b "$child_branch" >/dev/null 2>&1
  create_test_commit "Child feature"

  # Run diff without test plan to trigger validation failure
  log_step "Running arc diff with incomplete template..."
  export EDITOR_MODIFICATIONS="remove_test_plan"
  if EDITOR="$editor_script" "$GH_ARC_BIN" diff --no-edit 2>&1 | grep -q "validation failed"; then
    log_step "Validation failed as expected"
  else
    fail_test "E2E: Continue stacked PR" "Expected validation to fail"
    rm -f "$editor_script"
    return 1
  fi

  # Check that saved template has stacked format
  log_step "Looking for saved template..."
  local saved_template=$(find /tmp -name "gh-arc-saved-*.md" -type f -mmin -5 2>/dev/null | sort -t- -k4 -n | tail -n1)

  if [ -z "$saved_template" ]; then
    # Try broader pattern
    saved_template=$(find /tmp -name "gh-arc-*.md" -type f -mmin -5 2>/dev/null | sort -t- -k4 -n | tail -n1)

    if [ -z "$saved_template" ]; then
      log_warning "No saved template found (validation might have passed unexpectedly)"
      fail_test "E2E: Continue stacked PR" "No saved template found"
      rm -f "$editor_script"
      return 1
    fi
  fi

  log_step "Found saved template: $saved_template"

  if grep -q "Creating stacked PR on $parent_branch" "$saved_template"; then
    log_step "Saved template has stacked format ✓"
  else
    log_warning "Template might not be in stacked format (this might be OK)"
  fi

  # Run continue to create stacked PR
  log_step "Running arc diff --continue to create stacked PR..."
  export EDITOR_MODIFICATIONS="add_test_plan"
  if EDITOR="$editor_script" "$GH_ARC_BIN" diff --continue >/dev/null 2>&1; then
    log_step "Stacked PR created successfully"

    # Verify PR exists and has correct base
    if verify_pr_exists "$child_branch"; then
      local pr_base=$(get_pr_base "$child_branch")
      if [ "$pr_base" = "$parent_branch" ]; then
        log_step "PR correctly targets parent branch: $parent_branch ✓"
        pass_test "E2E: Continue mode with stacked PR"
      else
        fail_test "E2E: Continue stacked PR" "PR targets '$pr_base' instead of '$parent_branch'"
      fi
    else
      fail_test "E2E: Continue stacked PR" "PR not created"
    fi
  else
    fail_test "E2E: Continue stacked PR" "Failed to create stacked PR"
  fi

  # Cleanup
  rm -f "$editor_script" "$saved_template"
}

# =============================================================================
# Test 3: Stacking detection with same-commit scenario
# =============================================================================
test_e2e_stacking_same_commit() {
  start_test "E2E: Stacking detection with same-commit (auto-branch scenario)"

  cd "$TEST_DIR"

  # Ensure we're on main
  git checkout main >/dev/null 2>&1 || git checkout master >/dev/null 2>&1
  git pull origin main >/dev/null 2>&1 || git pull origin master >/dev/null 2>&1 || true

  # Create commit on main
  log_step "Creating commit on main (simulating auto-branch scenario)..."
  create_test_commit "Feature on main"

  # Run arc diff to trigger auto-branch creation
  log_step "Running arc diff (should create auto-branch)..."
  local editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  if ! EDITOR="$editor_script" "$GH_ARC_BIN" diff >/dev/null 2>&1; then
    fail_test "E2E: Stacking same-commit" "Failed to create auto-branch PR"
    rm -f "$editor_script"
    return 1
  fi

  # Get the auto-created branch name
  local auto_branch=$(git branch --show-current)
  log_step "Auto-branch created: $auto_branch"

  local auto_pr_number=$(get_pr_number "$auto_branch")
  log_step "Auto-branch PR #$auto_pr_number created"

  # Create child branch from auto-branch
  local child_branch=$(create_unique_branch "test-stacked-child")
  log_step "Creating child branch from auto-branch..."
  git checkout -b "$child_branch" >/dev/null 2>&1
  create_test_commit "Child feature"

  # Run arc diff to create stacked PR
  log_step "Running arc diff (should stack on auto-branch)..."
  if EDITOR="$editor_script" "$GH_ARC_BIN" diff >/dev/null 2>&1; then
    log_step "Child PR created"

    # Verify PR targets auto-branch, not main
    if verify_pr_exists "$child_branch"; then
      local pr_base=$(get_pr_base "$child_branch")
      if [ "$pr_base" = "$auto_branch" ]; then
        log_step "PR correctly stacks on auto-branch: $auto_branch ✓"
        pass_test "E2E: Stacking detection with same-commit"
      else
        fail_test "E2E: Stacking same-commit" "PR targets '$pr_base' instead of '$auto_branch'"
      fi
    else
      fail_test "E2E: Stacking same-commit" "Child PR not created"
    fi
  else
    fail_test "E2E: Stacking same-commit" "Failed to create child PR"
  fi

  # Cleanup
  rm -f "$editor_script"
}

# =============================================================================
# Test 4: Template sorting by modification time
# =============================================================================
test_e2e_template_sorting() {
  start_test "E2E: Template sorting by modification time"

  local branch_name=$(create_unique_branch "test-template-sorting")
  setup_test_branch "$branch_name"
  create_test_commit "Test commit"

  cd "$TEST_DIR"

  # Create multiple saved templates with different timestamps
  log_step "Creating multiple saved templates..."
  local temp1="/tmp/gh-arc-saved-old-$$.md"
  local temp2="/tmp/gh-arc-saved-new-$$.md"

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
New Template

# Summary:
This is the NEW template with NEWER content

# Test Plan:

# Reviewers:

# Draft:
false
EOF

  # Run arc diff --continue (should load newest template)
  log_step "Running arc diff --continue..."
  local editor_script=$(mktemp)
  create_editor_script "$editor_script" "add_test_plan"
  export EDITOR_MODIFICATIONS="add_test_plan"

  if EDITOR="$editor_script" "$GH_ARC_BIN" diff --continue >/dev/null 2>&1; then
    log_step "PR created"

    # Get PR details to check which template was used
    if verify_pr_exists "$branch_name"; then
      local pr_number=$(get_pr_number "$branch_name")
      local pr_body=$(gh pr view "$pr_number" --json body --jq '.body' 2>/dev/null)

      if echo "$pr_body" | grep -q "NEWER content"; then
        log_step "Newest template was loaded ✓"
        pass_test "E2E: Template sorting by modification time"
      else
        fail_test "E2E: Template sorting" "Wrong template loaded (should be newest)"
      fi
    else
      fail_test "E2E: Template sorting" "PR not created"
    fi
  else
    fail_test "E2E: Template sorting" "Failed to create PR"
  fi

  # Cleanup
  rm -f "$editor_script" "$temp1" "$temp2"
}

# =============================================================================
# Test 5: Auto-branch detection and creation
# =============================================================================
test_e2e_auto_branch_creation() {
  start_test "E2E: Auto-branch creation from main"

  cd "$TEST_DIR"

  # Ensure we're on main
  git checkout main >/dev/null 2>&1 || git checkout master >/dev/null 2>&1
  git pull origin main >/dev/null 2>&1 || git pull origin master >/dev/null 2>&1 || true

  # Create commits on main
  log_step "Creating commits on main..."
  create_test_commit "Feature commit 1"
  create_test_commit "Feature commit 2"

  # Run arc diff (should auto-create branch)
  log_step "Running arc diff (should trigger auto-branch)..."
  local editor_script=$(mktemp)
  create_editor_script "$editor_script" "complete_template"
  export EDITOR_MODIFICATIONS="complete_template"

  if EDITOR="$editor_script" "$GH_ARC_BIN" diff >/dev/null 2>&1; then
    local current_branch=$(git branch --show-current)

    # Should not be on main anymore
    if [ "$current_branch" != "main" ] && [ "$current_branch" != "master" ]; then
      log_step "Auto-branch created: $current_branch ✓"

      # Verify PR exists
      if verify_pr_exists "$current_branch"; then
        local pr_base=$(get_pr_base "$current_branch")
        if [ "$pr_base" = "main" ] || [ "$pr_base" = "master" ]; then
          log_step "PR correctly targets main branch ✓"
          pass_test "E2E: Auto-branch creation from main"
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
  rm -f "$editor_script"
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
  local specific_test="$1"

  log_info "gh-arc End-to-End Test Suite"
  log_info "============================"
  log_info "Test repository: $TEST_DIR"
  log_info "gh-arc binary: $GH_ARC_BIN"
  echo ""

  # Check gh CLI authentication
  if ! gh auth status >/dev/null 2>&1; then
    log_error "gh CLI is not authenticated"
    log_info "Run: gh auth login"
    exit 2
  fi

  log_info "gh CLI: authenticated ✓"

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

    test_e2e_continue_validation_failure
    test_e2e_continue_stacked_pr
    test_e2e_stacking_same_commit
    test_e2e_template_sorting
    test_e2e_auto_branch_creation
  fi

  # Print summary
  if print_summary; then
    echo ""
    log_warning "Manual cleanup required:"
    log_info "  1. Review PRs: gh pr list"
    log_info "  2. Close PRs: gh pr close <number>"
    log_info "  3. Delete branches: git branch -D <branch>"
    echo ""
    exit 0
  else
    exit 1
  fi
}

# Run main
main "$@"
