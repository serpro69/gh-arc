# Test Coverage for gh arc diff

This document provides a detailed breakdown of E2E test coverage for the `gh arc diff` command.

## Overview

- **Total E2E Tests:** 22
- **Test Categories:** 10
- **Flags Tested:** 5 (--draft, --ready, --edit, --no-edit, --continue, --base)
- **Workflows Tested:** 7
- **Edge Cases Tested:** 6

## Coverage by Flag

### --draft / --ready Flags (3 tests)

| Test | Coverage | Status |
|------|----------|--------|
| `test_e2e_fast_path_draft_ready` | Draft → ready transition via fast path | ✅ Covered |
| `test_e2e_draft_with_fast_path_commits` | Draft PR maintains status with new commits | ✅ Covered |
| `test_e2e_flag_combination_edit_draft` | --edit and --draft together | ✅ Covered |

**Scenarios Covered:**
- Create draft PR with `--draft`
- Mark draft PR as ready with `--ready`
- Draft PR fast path (new commits maintain draft status)
- Draft PR creation with template editing (`--edit --draft`)

**Coverage:** 100% - All draft/ready scenarios tested

### --edit Flag (3 tests)

| Test | Coverage | Status |
|------|----------|--------|
| `test_e2e_normal_mode_update_with_edit` | Force template regeneration on PR update | ✅ Covered |
| `test_e2e_flag_combination_edit_draft` | --edit with --draft | ✅ Covered |
| `test_e2e_flag_combination_base_with_edit` | --edit with --base | ✅ Covered |

**Scenarios Covered:**
- Force template regeneration for existing PR
- Edit flag with draft creation
- Edit flag with base override

**Coverage:** 100% - All --edit scenarios tested

### --no-edit Flag (2 tests)

| Test | Coverage | Status |
|------|----------|--------|
| `test_e2e_no_edit_flag_new_pr` | Create PR without editor | ✅ Covered |
| `test_e2e_no_edit_flag_update_pr` | Update PR without editor | ✅ Covered |

**Scenarios Covered:**
- New PR creation without opening editor
- Existing PR update without opening editor

**Coverage:** 100% - All --no-edit scenarios tested

### --continue Flag (3 tests)

| Test | Coverage | Status |
|------|----------|--------|
| `test_e2e_continue_validation_failure` | Multiple validation failures, edit preservation | ✅ Covered |
| `test_e2e_continue_stacked_pr` | Continue mode with stacked PR format | ✅ Covered |
| `test_e2e_flag_combination_continue_draft` | --continue with --draft | ✅ Covered |

**Scenarios Covered:**
- Template validation failure → continue with fixes
- Multiple validation failures (edit preservation)
- Continue mode with stacked PR template format
- Continue mode creating draft PR

**Coverage:** 100% - All --continue scenarios tested

### --base Flag (3 tests)

| Test | Coverage | Status |
|------|----------|--------|
| `test_e2e_base_flag_override_stacking` | Override stacking to target main | ✅ Covered |
| `test_e2e_base_flag_force_stacking` | Force stacking on specific branch | ✅ Covered |
| `test_e2e_base_flag_invalid_branch` | Error handling for invalid base | ✅ Covered |

**Scenarios Covered:**
- Break out of stacking with `--base main`
- Force stacking on specific branch
- Error handling for nonexistent base branch

**Coverage:** 100% - All --base scenarios tested

## Coverage by Workflow

### Fast Path (2 tests) - 100% Coverage

| Workflow | Test | Status |
|----------|------|--------|
| Push new commits | `test_e2e_fast_path_push_commits` | ✅ |
| Draft/ready transitions | `test_e2e_fast_path_draft_ready` | ✅ |

**What's Tested:**
- Existing PR with new commits (no editor)
- Draft status changes without reopening template
- No template regeneration path

### Normal Mode (2 tests) - 100% Coverage

| Workflow | Test | Status |
|----------|------|--------|
| New PR creation | `test_e2e_normal_mode_new_pr` | ✅ |
| PR update with --edit | `test_e2e_normal_mode_update_with_edit` | ✅ |

**What's Tested:**
- Template generation and editing
- PR creation with metadata
- Forced template regeneration

### Continue Mode (2 tests) - 100% Coverage

| Workflow | Test | Status |
|----------|------|--------|
| Validation retry | `test_e2e_continue_validation_failure` | ✅ |
| Stacked PR continue | `test_e2e_continue_stacked_pr` | ✅ |

**What's Tested:**
- Template preservation across validation failures
- Loading newest saved template
- Stacked PR template format handling
- Branch information extraction from templates

### PR Stacking (2 tests) - 100% Coverage

| Workflow | Test | Status |
|----------|------|--------|
| Basic stacking | `test_e2e_stacking_basic` | ✅ |
| Same-commit stacking | `test_e2e_stacking_same_commit` | ✅ |

**What's Tested:**
- Feature → feature → main hierarchy
- Stacking detection with auto-branch scenario
- Dependent PR detection
- Base branch calculation

### Auto-Branch (1 test) - 100% Coverage

| Workflow | Test | Status |
|----------|------|--------|
| Auto-branch from main | `test_e2e_auto_branch_creation` | ✅ |

**What's Tested:**
- Commits on main detection
- Automatic branch creation
- PR creation targeting main
- Branch checkout after creation

### Reviewer Assignment (2 tests) - 100% Coverage

| Workflow | Test | Status |
|----------|------|--------|
| Reviewer assignment | `test_e2e_reviewers_assignment` | ✅ |
| Current user filtering | `test_e2e_reviewers_filters_current_user` | ✅ |

**What's Tested:**
- Reviewer parsing from template
- Reviewer assignment via GitHub API
- Current user filtering from reviewer list

### Error Handling (1 test) - 100% Coverage

| Workflow | Test | Status |
|----------|------|--------|
| Editor cancellation | `test_e2e_error_editor_cancelled` | ✅ |

**What's Tested:**
- Editor exits with error code
- Graceful error message
- No PR created on cancellation

## Coverage by Feature

### Template System (3 tests)

| Feature | Test | Status |
|---------|------|--------|
| Template generation | `test_e2e_normal_mode_new_pr` | ✅ |
| Template sorting | `test_e2e_template_sorting` | ✅ |
| Template preservation | `test_e2e_continue_validation_failure` | ✅ |

**Coverage:**
- ✅ Generate template from commits
- ✅ Load newest template by mtime
- ✅ Preserve edits across validation failures
- ✅ Parse template fields (title, summary, test plan, reviewers, draft, ref)
- ✅ Validate required fields
- ✅ Handle stacked vs non-stacked template formats

### Base Branch Detection (5 tests)

| Feature | Test | Status |
|---------|------|--------|
| Stacking detection | `test_e2e_stacking_basic` | ✅ |
| Same-commit detection | `test_e2e_stacking_same_commit` | ✅ |
| Base override | `test_e2e_base_flag_override_stacking` | ✅ |
| Force stacking | `test_e2e_base_flag_force_stacking` | ✅ |
| Invalid base error | `test_e2e_base_flag_invalid_branch` | ✅ |

**Coverage:**
- ✅ Detect parent branch with open PR
- ✅ Handle same-commit scenario
- ✅ Override with --base flag
- ✅ Force stacking on specific branch
- ✅ Error handling for invalid base

### Draft Status Management (3 tests)

| Feature | Test | Status |
|---------|------|--------|
| Draft creation | `test_e2e_flag_combination_edit_draft` | ✅ |
| Draft → ready | `test_e2e_fast_path_draft_ready` | ✅ |
| Draft maintenance | `test_e2e_draft_with_fast_path_commits` | ✅ |

**Coverage:**
- ✅ Create draft PR with --draft
- ✅ Mark ready with --ready
- ✅ Maintain draft status in fast path
- ✅ Draft status with continue mode

### PR Update Logic (4 tests)

| Feature | Test | Status |
|---------|------|--------|
| Fast path push | `test_e2e_fast_path_push_commits` | ✅ |
| Force edit update | `test_e2e_normal_mode_update_with_edit` | ✅ |
| No-edit update | `test_e2e_no_edit_flag_update_pr` | ✅ |
| Draft fast path | `test_e2e_draft_with_fast_path_commits` | ✅ |

**Coverage:**
- ✅ Push new commits without editor
- ✅ Update with forced template regeneration
- ✅ Update without editor
- ✅ Fast path maintains PR metadata (draft status)

## Edge Cases Tested

### Invalid Input (1 test)

| Edge Case | Test | Status |
|-----------|------|--------|
| Invalid base branch | `test_e2e_base_flag_invalid_branch` | ✅ |

**Coverage:**
- ✅ Error message shown
- ✅ No PR created
- ✅ Clean error exit

### User Interaction (1 test)

| Edge Case | Test | Status |
|-----------|------|--------|
| Editor cancellation | `test_e2e_error_editor_cancelled` | ✅ |

**Coverage:**
- ✅ Editor exits with error
- ✅ Graceful cancellation message
- ✅ No artifacts created

### Template Edge Cases (2 tests)

| Edge Case | Test | Status |
|-----------|------|--------|
| Multiple validation failures | `test_e2e_continue_validation_failure` | ✅ |
| Stacked template format | `test_e2e_continue_stacked_pr` | ✅ |

**Coverage:**
- ✅ Edits preserved across multiple failures
- ✅ Extra content accumulates
- ✅ Stacked template format (no head branch in header)
- ✅ Base branch extraction from stacked format

### Stacking Edge Cases (1 test)

| Edge Case | Test | Status |
|-----------|------|--------|
| Same commit detection | `test_e2e_stacking_same_commit` | ✅ |

**Coverage:**
- ✅ Auto-branch scenario (main and parent at same commit)
- ✅ Correct stacking despite identical commits
- ✅ Child PR targets auto-branch (not main)

### Reviewer Edge Cases (1 test)

| Edge Case | Test | Status |
|-----------|------|--------|
| Current user filtering | `test_e2e_reviewers_filters_current_user` | ✅ |

**Coverage:**
- ✅ Current user detected from GitHub API
- ✅ Filtered from reviewer assignments
- ✅ No self-assignment of reviews

## Gap Analysis

### Covered ✅
- All flags (--draft, --ready, --edit, --no-edit, --continue, --base)
- All primary workflows
- Flag combinations (--edit --draft, --continue --draft, --base --edit)
- Error handling for invalid input
- Edge cases (same-commit, validation failures, template formats)

### Not Covered ❌
- None - All identified scenarios are tested

### Out of Scope
- Network failures / GitHub API errors (transient, not testable in E2E)
- Authentication failures (prerequisite check)
- Git operation failures (mocked in unit tests)
- Rate limiting (transient, CI/CD handles)

## Test Execution Requirements

### Prerequisites
1. **Test Repository:** Clone a test repo with GitHub remote
   ```bash
   git clone git@github.com:0xBAD-dev/gh-arc-test.git /tmp/gh-arc-test
   ```
2. **GitHub CLI:** Authenticated with required scopes
   ```bash
   gh auth refresh -s read:user
   ```
3. **gh-arc Binary:** Built (`go build -o gh-arc`)

### Run All Tests
```bash
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh
```

### Run Specific Test
```bash
TEST_DIR=/tmp/gh-arc-test ./test/test-e2e.sh test_e2e_base_flag_override_stacking
```

### Expected Results
- **Duration:** ~2-4 minutes for full suite
- **Pass Rate:** 100% (22/22 tests)
- **Artifacts:** Auto-cleaned (use --no-cleanup to keep)

## Coverage Metrics

| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| Total Tests | 22 | 20+ | ✅ Exceeded |
| Flag Coverage | 5/5 | 100% | ✅ Complete |
| Workflow Coverage | 7/7 | 100% | ✅ Complete |
| Edge Case Coverage | 6/6 | 100% | ✅ Complete |
| Flag Combinations | 3/3 | 100% | ✅ Complete |

## Coverage by Priority

### P0 - Critical (11 tests) - 100% Coverage
- Fast path execution (2)
- Normal mode workflows (2)
- --base flag (3)
- --no-edit flag (2)
- Basic stacking (1)
- Auto-branch (1)

### P1 - High (7 tests) - 100% Coverage
- Draft PR scenarios (1)
- Flag combinations (3)
- Continue mode (2)
- Stacking edge case (1)

### P2 - Medium (4 tests) - 100% Coverage
- Reviewer assignment (2)
- Template sorting (1)
- Error handling (1)

## Regression Protection

All tests protect against known issues:
- ✅ Template edits lost on validation failure
- ✅ Wrong template loaded (sorting by name not mtime)
- ✅ Stacking detection failed with same-commit scenario
- ✅ Continue mode failed with stacked PR format
- ✅ Branch not pushed before PR creation
- ✅ Current user assigned as reviewer

## Continuous Integration

Tests are CI-ready:
- ✅ Exit code 0 on success, 1 on failure
- ✅ Automatic cleanup (no manual intervention)
- ✅ Parallel-safe (unique timestamped names)
- ✅ Idempotent (can rerun without side effects)
- ✅ Independent (each test standalone)

## Maintenance

### Adding New Tests
1. Add test function following naming convention: `test_e2e_<category>_<scenario>`
2. Add to appropriate category in main()
3. Update TOTAL_TESTS counter in test-e2e.sh
4. Update this document

### Updating Coverage
- Update flag coverage tables
- Update workflow coverage tables
- Update metrics
- Run full suite to verify

## Summary

The `gh arc diff` E2E test suite provides comprehensive coverage of all flags, workflows, and edge cases. With 22 tests organized into 10 categories, the suite ensures the diff command works correctly across all scenarios. All tests are idempotent, independent, and automatically clean up after themselves, making the suite ideal for continuous integration and regression testing.

**Coverage Level: 100%** - All identified scenarios are tested.
