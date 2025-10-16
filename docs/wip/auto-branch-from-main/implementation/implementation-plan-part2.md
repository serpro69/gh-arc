# Implementation Plan: Auto-Branch from Main (Part 2)

## Phase 3: Detection and Auto-Branch Logic (continued)

### Task 9: Create Auto-Branch Module Structure

**Goal**: Create the auto-branch module with basic structure.

**Files to create**:
- `internal/diff/auto_branch.go`
- `internal/diff/auto_branch_test.go`

**Implementation steps**:

1. Create `internal/diff/auto_branch.go`:
   ```go
   package diff

   import (
       "context"
       "fmt"
       "strings"
       "time"

       "github.com/serpro69/gh-arc/internal/config"
       "github.com/serpro69/gh-arc/internal/git"
       "github.com/serpro69/gh-arc/internal/logger"
   )

   // AutoBranchDetector detects when user is on main with commits ahead.
   type AutoBranchDetector struct {
       repo   GitRepository
       config *config.DiffConfig
   }

   // NewAutoBranchDetector creates a new auto-branch detector.
   func NewAutoBranchDetector(repo GitRepository, cfg *config.DiffConfig) *AutoBranchDetector {
       return &AutoBranchDetector{
           repo:   repo,
           config: cfg,
       }
   }

   // DetectionResult contains the result of commit detection.
   type DetectionResult struct {
       OnMainBranch bool   // True if currently on default branch
       CommitsAhead int    // Number of commits ahead of origin
       DefaultBranch string // Name of default branch (main/master)
   }

   // DetectCommitsOnMain detects if user is on main with unpushed commits.
   func (d *AutoBranchDetector) DetectCommitsOnMain(ctx context.Context) (*DetectionResult, error) {
       // Get current branch
       currentBranch, err := d.repo.GetCurrentBranch()
       if err != nil {
           return nil, fmt.Errorf("failed to get current branch: %w", err)
       }

       // Get default branch
       defaultBranch, err := d.repo.GetDefaultBranch()
       if err != nil {
           return nil, fmt.Errorf("failed to get default branch: %w", err)
       }

       result := &DetectionResult{
           OnMainBranch:  currentBranch == defaultBranch,
           CommitsAhead:  0,
           DefaultBranch: defaultBranch,
       }

       // If not on main, no detection needed
       if !result.OnMainBranch {
           return result, nil
       }

       // Count commits ahead of origin/main
       originBranch := fmt.Sprintf("origin/%s", defaultBranch)
       count, err := d.repo.CountCommitsAhead(currentBranch, originBranch)
       if err != nil {
           return nil, fmt.Errorf("failed to count commits ahead: %w", err)
       }

       result.CommitsAhead = count

       logger.Debug().
           Str("currentBranch", currentBranch).
           Str("defaultBranch", defaultBranch).
           Int("commitsAhead", count).
           Msg("Auto-branch detection complete")

       return result, nil
   }

   // ShouldAutoBranch determines if auto-branch flow should activate.
   func (d *AutoBranchDetector) ShouldAutoBranch(result *DetectionResult) bool {
       return result.OnMainBranch && result.CommitsAhead > 0
   }
   ```

2. Create `internal/diff/auto_branch_test.go`:
   ```go
   package diff

   import (
       "context"
       "testing"

       "github.com/serpro69/gh-arc/internal/config"
   )

   func TestDetectCommitsOnMain(t *testing.T) {
       t.Run("on main with commits ahead", func(t *testing.T) {
           // TODO: Implement when git test helpers are ready
           t.Skip("Requires git test fixtures")
       })

       t.Run("on feature branch", func(t *testing.T) {
           // TODO: Implement when git test helpers are ready
           t.Skip("Requires git test fixtures")
       })

       t.Run("on main but up-to-date", func(t *testing.T) {
           // TODO: Implement when git test helpers are ready
           t.Skip("Requires git test fixtures")
       })
   }

   func TestShouldAutoBranch(t *testing.T) {
       detector := &AutoBranchDetector{
           config: &config.DiffConfig{
               AutoCreateBranchFromMain: true,
           },
       }

       tests := []struct {
           name     string
           result   *DetectionResult
           expected bool
       }{
           {
               name: "on main with commits",
               result: &DetectionResult{
                   OnMainBranch: true,
                   CommitsAhead: 2,
               },
               expected: true,
           },
           {
               name: "on feature branch",
               result: &DetectionResult{
                   OnMainBranch: false,
                   CommitsAhead: 2,
               },
               expected: false,
           },
           {
               name: "on main but no commits",
               result: &DetectionResult{
                   OnMainBranch: true,
                   CommitsAhead: 0,
               },
               expected: false,
           },
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               got := detector.ShouldAutoBranch(tt.result)
               if got != tt.expected {
                   t.Errorf("ShouldAutoBranch() = %v, expected %v", got, tt.expected)
               }
           })
       }
   }
   ```

**Testing**:
```bash
go test ./internal/diff -run TestShouldAutoBranch -v
go build ./internal/diff
```

**Commit message**:
```
feat(diff): create auto-branch detection module

Create AutoBranchDetector with basic detection logic:
- DetectCommitsOnMain: checks if on default branch with commits ahead
- ShouldAutoBranch: determines if auto-branch should activate

Includes basic tests (full integration tests deferred to later tasks).
```

---

### Task 10: Implement Branch Name Generation

**Goal**: Add branch name generation with pattern support.

**Files to modify**:
- `internal/diff/auto_branch.go`

**Implementation steps**:

1. Add to `internal/diff/auto_branch.go`:
   ```go
   import (
       "crypto/rand"
       "encoding/hex"
       // ... existing imports ...
   )

   // GenerateBranchName generates a branch name based on the configured pattern.
   // Returns the generated name and nil prompt if pattern is set.
   // Returns empty string and true for shouldPrompt if pattern is "null".
   func (d *AutoBranchDetector) GenerateBranchName() (string, bool, error) {
       pattern := d.config.AutoBranchNamePattern

       // If pattern is literally "null", prompt user
       if pattern == "null" {
           return "", true, nil
       }

       // If pattern is empty, use default
       if pattern == "" {
           timestamp := time.Now().Unix()
           return fmt.Sprintf("feature/auto-from-main-%d", timestamp), false, nil
       }

       // Apply pattern with placeholders
       name := pattern
       name = strings.ReplaceAll(name, "{timestamp}", fmt.Sprintf("%d", time.Now().Unix()))
       name = strings.ReplaceAll(name, "{date}", time.Now().Format("2006-01-02"))
       name = strings.ReplaceAll(name, "{datetime}", time.Now().Format("2006-01-02T150405"))

       // Get username from git config
       username, err := d.repo.GetGitConfig("user.name")
       if err != nil {
           username = "user"
       }
       // Sanitize username for branch name
       username = sanitizeBranchName(username)
       name = strings.ReplaceAll(name, "{username}", username)

       // Generate random string
       randomStr := generateRandomString(6)
       name = strings.ReplaceAll(name, "{random}", randomStr)

       return name, false, nil
   }

   // sanitizeBranchName removes characters invalid in git branch names.
   func sanitizeBranchName(name string) string {
       // Replace spaces and invalid chars with hyphens
       name = strings.ReplaceAll(name, " ", "-")
       name = strings.ReplaceAll(name, "..", "-")
       name = strings.ToLower(name)

       // Remove other invalid characters
       invalidChars := []string{"~", "^", ":", "?", "*", "[", "\\"}
       for _, char := range invalidChars {
           name = strings.ReplaceAll(name, char, "")
       }

       return name
   }

   // generateRandomString generates a random alphanumeric string of given length.
   func generateRandomString(length int) string {
       bytes := make([]byte, length/2+1)
       if _, err := rand.Read(bytes); err != nil {
           // Fallback to timestamp-based if crypto/rand fails
           return fmt.Sprintf("%d", time.Now().UnixNano()%1000000)[:length]
       }
       return hex.EncodeToString(bytes)[:length]
   }

   // EnsureUniqueBranchName ensures the branch name is unique by appending a counter if needed.
   func (d *AutoBranchDetector) EnsureUniqueBranchName(baseName string) (string, error) {
       name := baseName
       counter := 1

       for {
           exists, err := d.repo.BranchExists(name)
           if err != nil {
               return "", fmt.Errorf("failed to check branch existence: %w", err)
           }

           if !exists {
               return name, nil
           }

           // Append counter to make unique
           name = fmt.Sprintf("%s-%d", baseName, counter)
           counter++

           // Safety check to prevent infinite loop
           if counter > 100 {
               return "", fmt.Errorf("failed to generate unique branch name after 100 attempts")
           }
       }
   }
   ```

2. Add tests to `internal/diff/auto_branch_test.go`:
   ```go
   func TestGenerateBranchName(t *testing.T) {
       tests := []struct {
           name              string
           pattern           string
           expectPrompt      bool
           expectContains    string
       }{
           {
               name:           "empty pattern uses default",
               pattern:        "",
               expectPrompt:   false,
               expectContains: "feature/auto-from-main-",
           },
           {
               name:           "null pattern triggers prompt",
               pattern:        "null",
               expectPrompt:   true,
           },
           {
               name:           "pattern with timestamp",
               pattern:        "feature/{timestamp}",
               expectPrompt:   false,
               expectContains: "feature/",
           },
           {
               name:           "pattern with date",
               pattern:        "auto/{date}",
               expectPrompt:   false,
               expectContains: "auto/",
           },
           {
               name:           "pattern with datetime",
               pattern:        "fix/{datetime}",
               expectPrompt:   false,
               expectContains: "fix/",
           },
           {
               name:           "pattern with random",
               pattern:        "feature/temp-{random}",
               expectPrompt:   false,
               expectContains: "feature/temp-",
           },
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               detector := &AutoBranchDetector{
                   config: &config.DiffConfig{
                       AutoBranchNamePattern: tt.pattern,
                   },
                   repo: &mockGitRepo{
                       gitConfig: map[string]string{
                           "user.name": "Test User",
                       },
                   },
               }

               name, shouldPrompt, err := detector.GenerateBranchName()
               if err != nil {
                   t.Fatalf("GenerateBranchName() failed: %v", err)
               }

               if shouldPrompt != tt.expectPrompt {
                   t.Errorf("shouldPrompt = %v, expected %v", shouldPrompt, tt.expectPrompt)
               }

               if !tt.expectPrompt && tt.expectContains != "" {
                   if !strings.Contains(name, tt.expectContains) {
                       t.Errorf("name %q doesn't contain %q", name, tt.expectContains)
                   }
               }
           })
       }
   }

   func TestSanitizeBranchName(t *testing.T) {
       tests := []struct {
           input    string
           expected string
       }{
           {"John Doe", "john-doe"},
           {"test..name", "test--name"},
           {"test~name", "testname"},
           {"test:name", "testname"},
           {"Test Name!", "test-name!"},
       }

       for _, tt := range tests {
           t.Run(tt.input, func(t *testing.T) {
               got := sanitizeBranchName(tt.input)
               if got != tt.expected {
                   t.Errorf("sanitizeBranchName(%q) = %q, expected %q", tt.input, got, tt.expected)
               }
           })
       }
   }

   func TestEnsureUniqueBranchName(t *testing.T) {
       detector := &AutoBranchDetector{
           repo: &mockGitRepo{
               existingBranches: map[string]bool{
                   "feature/test":   true,
                   "feature/test-1": true,
               },
           },
       }

       // Should return feature/test-2 since test and test-1 exist
       name, err := detector.EnsureUniqueBranchName("feature/test")
       if err != nil {
           t.Fatalf("EnsureUniqueBranchName() failed: %v", err)
       }

       expected := "feature/test-2"
       if name != expected {
           t.Errorf("EnsureUniqueBranchName() = %q, expected %q", name, expected)
       }
   }

   // Mock git repository for testing
   type mockGitRepo struct {
       gitConfig        map[string]string
       existingBranches map[string]bool
   }

   func (m *mockGitRepo) GetGitConfig(key string) (string, error) {
       if val, ok := m.gitConfig[key]; ok {
           return val, nil
       }
       return "", fmt.Errorf("key not found")
   }

   func (m *mockGitRepo) BranchExists(name string) (bool, error) {
       return m.existingBranches[name], nil
   }

   // Implement other required interface methods as stubs...
   func (m *mockGitRepo) Path() string                             { return "" }
   func (m *mockGitRepo) GetDefaultBranch() (string, error)        { return "main", nil }
   func (m *mockGitRepo) ListBranches(bool) ([]git.BranchInfo, error) { return nil, nil }
   func (m *mockGitRepo) GetMergeBase(string, string) (string, error) { return "", nil }
   func (m *mockGitRepo) GetCommitRange(string, string) ([]git.CommitInfo, error) { return nil, nil }
   ```

**Testing**:
```bash
go test ./internal/diff -run TestGenerateBranchName -v
go test ./internal/diff -run TestSanitizeBranchName -v
go test ./internal/diff -run TestEnsureUniqueBranchName -v
```

**Commit message**:
```
feat(diff): implement branch name generation with patterns

Add GenerateBranchName with support for:
- Default pattern: feature/auto-from-main-{timestamp}
- "null" triggers user prompt
- Custom patterns with placeholders: {timestamp}, {date}, {datetime}, {username}, {random}
- Branch name sanitization for git compatibility
- Unique name generation (appends counter if branch exists)

Includes comprehensive tests with mock git repository.
```

---

### Task 11: Implement User Prompts

**Goal**: Add interactive user prompts for confirmation.

**Files to modify**:
- `internal/diff/auto_branch.go`

**Implementation steps**:

1. Add prompt utilities to `internal/diff/auto_branch.go`:
   ```go
   import (
       "bufio"
       "os"
       // ... existing imports ...
   )

   // promptYesNo prompts user with a yes/no question.
   // Returns true for yes, false for no.
   func promptYesNo(message string, defaultYes bool) (bool, error) {
       defaultStr := "Y/n"
       if !defaultYes {
           defaultStr = "y/N"
       }

       fmt.Printf("%s (%s) ", message, defaultStr)

       reader := bufio.NewReader(os.Stdin)
       response, err := reader.ReadString('\n')
       if err != nil {
           return false, fmt.Errorf("failed to read input: %w", err)
       }

       response = strings.TrimSpace(strings.ToLower(response))

       // Empty response uses default
       if response == "" {
           return defaultYes, nil
       }

       switch response {
       case "y", "yes":
           return true, nil
       case "n", "no":
           return false, nil
       default:
           // Invalid response, ask again
           fmt.Println("Please answer 'y' or 'n'")
           return promptYesNo(message, defaultYes)
       }
   }

   // promptBranchName prompts user for a branch name.
   func promptBranchName() (string, error) {
       fmt.Print("Enter branch name (or press Enter for default): ")

       reader := bufio.NewReader(os.Stdin)
       response, err := reader.ReadString('\n')
       if err != nil {
           return "", fmt.Errorf("failed to read input: %w", err)
       }

       name := strings.TrimSpace(response)

       // Empty means use default
       if name == "" {
           return "", nil
       }

       // Validate branch name
       if strings.Contains(name, " ") {
           fmt.Println("Branch name cannot contain spaces")
           return promptBranchName()
       }

       return name, nil
   }

   // ShouldCreateBranch checks config or prompts user to create branch.
   func (d *AutoBranchDetector) ShouldCreateBranch() (bool, error) {
       if d.config.AutoCreateBranchFromMain {
           return true, nil
       }

       return promptYesNo("Create feature branch automatically?", true)
   }

   // ShouldStashChanges checks config or prompts user to stash changes.
   func (d *AutoBranchDetector) ShouldStashChanges() (bool, error) {
       if d.config.AutoStashUncommittedChanges {
           return true, nil
       }

       return promptYesNo("You have uncommitted changes. Stash them?", true)
   }

   // ShouldResetMain checks config or prompts user to reset main.
   func (d *AutoBranchDetector) ShouldResetMain() (bool, error) {
       if d.config.AutoResetMain {
           return true, nil
       }

       return promptYesNo("Reset main to origin/main?", true)
   }

   // GetBranchName gets branch name from generator or prompts user.
   func (d *AutoBranchDetector) GetBranchName() (string, error) {
       generatedName, shouldPrompt, err := d.GenerateBranchName()
       if err != nil {
           return "", err
       }

       if shouldPrompt {
           userInput, err := promptBranchName()
           if err != nil {
               return "", err
           }

           // Use default if user pressed Enter
           if userInput == "" {
               timestamp := time.Now().Unix()
               generatedName = fmt.Sprintf("feature/auto-from-main-%d", timestamp)
           } else {
               generatedName = userInput
           }
       }

       // Ensure uniqueness
       return d.EnsureUniqueBranchName(generatedName)
   }
   ```

**Testing**:

Since prompts require user interaction, we'll test the logic without actual I/O:

```go
func TestShouldCreateBranch(t *testing.T) {
    t.Run("auto-create enabled", func(t *testing.T) {
        detector := &AutoBranchDetector{
            config: &config.DiffConfig{
                AutoCreateBranchFromMain: true,
            },
        }

        should, err := detector.ShouldCreateBranch()
        if err != nil {
            t.Fatalf("ShouldCreateBranch() failed: %v", err)
        }
        if !should {
            t.Error("ShouldCreateBranch() = false, expected true when auto-create enabled")
        }
    })

    // Interactive prompt testing requires mock stdin - tested manually
}
```

**Testing**:
```bash
go test ./internal/diff -run TestShouldCreateBranch -v
go build ./internal/diff
```

**Commit message**:
```
feat(diff): add user prompts for auto-branch flow

Add interactive prompt utilities:
- promptYesNo: yes/no questions with default
- promptBranchName: get branch name from user
- ShouldCreateBranch: check config or prompt
- ShouldStashChanges: check config or prompt
- ShouldResetMain: check config or prompt
- GetBranchName: generate or prompt for branch name

Handles invalid input with re-prompting.
Includes tests for config-driven paths (prompts tested manually).
```

---

### Task 12: Implement Core Auto-Branch Flow

**Goal**: Implement the main auto-branch orchestration logic.

**Files to modify**:
- `internal/diff/auto_branch.go`

**Implementation steps**:

1. Add main flow method to `internal/diff/auto_branch.go`:
   ```go
   // AutoBranchResult contains the result of auto-branch operation.
   type AutoBranchResult struct {
       BranchCreated   string // Name of created branch
       CommitsMoved    int    // Number of commits moved to branch
       StashCreated    bool   // Whether changes were stashed
       ResetPending    bool   // Whether main reset is pending (happens after PR)
   }

   // ExecuteAutoBranch performs the auto-branch flow.
   // This is the main orchestration method.
   func (d *AutoBranchDetector) ExecuteAutoBranch(
       ctx context.Context,
       detection *DetectionResult,
   ) (*AutoBranchResult, error) {
       logger.Info().
           Str("defaultBranch", detection.DefaultBranch).
           Int("commitsAhead", detection.CommitsAhead).
           Msg("Starting auto-branch flow")

       result := &AutoBranchResult{
           CommitsMoved: detection.CommitsAhead,
       }

       // Step 1: Check if user wants to create branch
       shouldCreate, err := d.ShouldCreateBranch()
       if err != nil {
           return nil, fmt.Errorf("failed to get user confirmation: %w", err)
       }
       if !shouldCreate {
           return nil, fmt.Errorf("auto-branch cancelled by user")
       }

       // Step 2: Handle uncommitted changes
       status, err := d.repo.GetWorkingDirectoryStatus()
       if err != nil {
           return nil, fmt.Errorf("failed to check working directory status: %w", err)
       }

       if !status.IsClean {
           shouldStash, err := d.ShouldStashChanges()
           if err != nil {
               return nil, fmt.Errorf("failed to get stash confirmation: %w", err)
           }
           if !shouldStash {
               return nil, fmt.Errorf("cannot proceed with uncommitted changes")
           }

           // Stash changes
           fmt.Println("✓ Stashing uncommitted changes...")
           _, err = d.repo.Stash("gh-arc auto-branch")
           if err != nil {
               return nil, fmt.Errorf("failed to stash changes: %w", err)
           }
           result.StashCreated = true
       }

       // Step 3: Get branch name
       branchName, err := d.GetBranchName()
       if err != nil {
           return nil, fmt.Errorf("failed to get branch name: %w", err)
       }

       // Step 4: Create branch from current HEAD
       fmt.Printf("✓ Creating feature branch: %s\n", branchName)
       err = d.repo.CreateBranch(branchName, "")
       if err != nil {
           return nil, fmt.Errorf("failed to create branch: %w", err)
       }
       result.BranchCreated = branchName

       // Step 5: Checkout the new branch
       err = d.repo.CheckoutBranch(branchName)
       if err != nil {
           return nil, fmt.Errorf("failed to checkout branch: %w", err)
       }

       // Step 6: Pop stash if we stashed
       if result.StashCreated {
           err = d.repo.StashPop()
           if err != nil {
               logger.Warn().Err(err).Msg("Failed to pop stash, but branch created successfully")
               fmt.Printf("⚠️  Warning: Failed to restore stashed changes: %v\n", err)
               fmt.Println("   You can manually restore with: git stash pop")
           }
       }

       // Step 7: Mark that reset is pending (will happen after PR creation)
       result.ResetPending = true

       fmt.Printf("✓ Created feature branch '%s' with your %d commits\n",
           branchName, result.CommitsMoved)

       return result, nil
   }

   // ResetMainBranch resets the default branch to origin after successful PR creation.
   // Call this after the PR is successfully created.
   func (d *AutoBranchDetector) ResetMainBranch(
       ctx context.Context,
       defaultBranch string,
       currentBranch string,
   ) error {
       // Check if user wants to reset
       shouldReset, err := d.ShouldResetMain()
       if err != nil {
           return fmt.Errorf("failed to get reset confirmation: %w", err)
       }
       if !shouldReset {
           logger.Info().Msg("User declined to reset main, skipping")
           return nil
       }

       fmt.Println("\n✓ Resetting main to origin/main...")

       // Checkout main
       err = d.repo.CheckoutBranch(defaultBranch)
       if err != nil {
           return fmt.Errorf("failed to checkout %s: %w", defaultBranch, err)
       }

       // Reset to origin/main
       originRef := fmt.Sprintf("origin/%s", defaultBranch)
       err = d.repo.ResetHard(originRef)
       if err != nil {
           return fmt.Errorf("failed to reset to %s: %w", originRef, err)
       }

       // Checkout back to feature branch
       err = d.repo.CheckoutBranch(currentBranch)
       if err != nil {
           return fmt.Errorf("failed to checkout %s: %w", currentBranch, err)
       }

       fmt.Println("  ✓ Reset main to origin/main")

       return nil
   }
   ```

**Testing**:

Add integration-style test (requires full git setup):

```go
func TestExecuteAutoBranch(t *testing.T) {
    // This is an integration test that requires full git setup
    // Mark as integration test
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    t.Run("full auto-branch flow", func(t *testing.T) {
        // TODO: Implement with full git test fixtures
        t.Skip("Requires complete git test environment")
    })
}
```

**Testing**:
```bash
go test ./internal/diff -v
go build ./internal/diff
```

**Commit message**:
```
feat(diff): implement core auto-branch orchestration

Add ExecuteAutoBranch method for full auto-branch flow:
1. Confirm branch creation with user
2. Handle uncommitted changes (stash if needed)
3. Get branch name (generate or prompt)
4. Create and checkout feature branch
5. Restore stashed changes
6. Display success message

Add ResetMainBranch for post-PR reset:
- Checkout main
- Reset to origin/main
- Checkout back to feature branch

Integration tests deferred to Phase 6.
```

---

## Phase 4: Integration with diff Command

### Task 13: Integrate Detection into diff Command

**Goal**: Add auto-branch detection to the start of `runDiff`.

**Files to modify**:
- `cmd/diff.go`

**Implementation steps**:

1. Open `cmd/diff.go`
2. Add import for auto-branch:
   ```go
   import (
       // ... existing imports ...
       "github.com/serpro69/gh-arc/internal/diff"
   )
   ```

3. Locate `runDiff` function (line 118)
4. Add detection after opening git repository (around line 154, after getting current branch):
   ```go
   logger.Info().
       Str("branch", currentBranch).
       Msg("Current branch")

   // NEW: Detect commits on main and handle auto-branch flow
   autoBranchDetector := diff.NewAutoBranchDetector(gitRepo, &cfg.Diff)
   detection, err := autoBranchDetector.DetectCommitsOnMain(ctx)
   if err != nil {
       return fmt.Errorf("failed to detect commits on main: %w", err)
   }

   // NEW: Variable to store auto-branch result for later reset
   var autoBranchResult *diff.AutoBranchResult

   // NEW: If we detected commits on main, execute auto-branch flow
   if autoBranchDetector.ShouldAutoBranch(detection) {
       fmt.Printf("\n⚠️  Warning: You have %d commits on %s\n",
           detection.CommitsAhead, detection.DefaultBranch)

       autoBranchResult, err = autoBranchDetector.ExecuteAutoBranch(ctx, detection)
       if err != nil {
           return fmt.Errorf("auto-branch flow failed: %w", err)
       }

       // Update currentBranch to the newly created branch
       currentBranch = autoBranchResult.BranchCreated

       logger.Info().
           Str("newBranch", currentBranch).
           Int("commitsMoved", autoBranchResult.CommitsMoved).
           Msg("Auto-branch flow completed, continuing with diff")
   }

   // Create GitHub client
   client, err := github.NewClient()
   // ... rest of function continues normally ...
   ```

**Testing**:
```bash
go build ./cmd/...
go build ./...
./gh-arc --help
```

**Commit message**:
```
feat(cmd/diff): integrate auto-branch detection

Add auto-branch detection at start of runDiff:
- Detect commits on default branch
- Execute auto-branch flow if detected
- Update currentBranch to new feature branch
- Continue with normal diff flow

Flow activates before GitHub client creation, ensuring
we're on a feature branch before any PR operations.
```

---

### Task 14: Add Reset After Successful PR Creation

**Goal**: Reset main after successful PR creation.

**Files to modify**:
- `cmd/diff.go`

**Implementation steps**:

1. Open `cmd/diff.go`
2. Locate the end of `runDiff` (around line 644, after success message)
3. Add reset logic before final return:
   ```go
   // Display final success message
   fmt.Println("\n✓ Success!")
   if isDraft {
       fmt.Println("  PR created as draft")
   }
   if baseResult.IsStacking {
       fmt.Printf("  Stacked on: %s\n", baseResult.Base)
   }

   // NEW: Reset main if auto-branch was used
   if autoBranchResult != nil && autoBranchResult.ResetPending {
       err = autoBranchDetector.ResetMainBranch(
           ctx,
           detection.DefaultBranch,
           currentBranch,
       )
       if err != nil {
           // Log warning but don't fail the command
           logger.Warn().
               Err(err).
               Msg("Failed to reset main branch")
           fmt.Printf("\n⚠️  Warning: Failed to reset main: %v\n", err)
           fmt.Printf("   You can manually reset with:\n")
           fmt.Printf("     git checkout %s\n", detection.DefaultBranch)
           fmt.Printf("     git reset --hard origin/%s\n", detection.DefaultBranch)
           fmt.Printf("     git checkout %s\n", currentBranch)
       }
   }

   return nil
   ```

**Testing**:
```bash
go build ./cmd/...
./gh-arc diff --help
```

**Commit message**:
```
feat(cmd/diff): add main reset after successful PR

Reset main to origin/main after successful PR creation when
auto-branch flow was used. Reset is optional based on config.

On error, show warning and manual reset instructions without
failing the command (PR creation already succeeded).
```

---

### Task 15: Add User-Friendly Error Messages

**Goal**: Improve error messages for auto-branch scenarios.

**Files to modify**:
- `cmd/diff.go`

**Implementation steps**:

1. Open `cmd/diff.go`
2. Enhance error handling in auto-branch section:
   ```go
   if autoBranchDetector.ShouldAutoBranch(detection) {
       fmt.Printf("\n⚠️  Warning: You have %d commits on %s\n",
           detection.CommitsAhead, detection.DefaultBranch)

       autoBranchResult, err = autoBranchDetector.ExecuteAutoBranch(ctx, detection)
       if err != nil {
           // Check if user cancelled
           if strings.Contains(err.Error(), "cancelled by user") {
               fmt.Println("\n✗ Cannot create PR from main to main.")
               fmt.Println("  Please create a feature branch manually:")
               fmt.Printf("    git checkout -b feature/my-branch\n")
               fmt.Printf("    git checkout %s\n", detection.DefaultBranch)
               fmt.Printf("    git reset --hard origin/%s\n", detection.DefaultBranch)
               fmt.Printf("    git checkout feature/my-branch\n")
               fmt.Printf("    gh arc diff\n")
               return fmt.Errorf("operation cancelled")
           }

           // Other errors
           return fmt.Errorf("auto-branch flow failed: %w", err)
       }

       // ... rest of flow ...
   }
   ```

**Testing**:
```bash
go build ./cmd/...
```

**Commit message**:
```
feat(cmd/diff): improve auto-branch error messages

Add user-friendly error messages for auto-branch flow:
- Clear explanation when user cancels
- Step-by-step manual instructions as fallback
- Distinguish between user cancellation and system errors
```

---

## Phase 5: User Interaction and Polish

### Task 16: Add Progress Indicators

**Goal**: Add visual feedback during auto-branch operations.

**Files to modify**:
- `internal/diff/auto_branch.go`

**Implementation steps**:

1. Enhance ExecuteAutoBranch with better progress messages:
   ```go
   func (d *AutoBranchDetector) ExecuteAutoBranch(...) (*AutoBranchResult, error) {
       logger.Info().
           Str("defaultBranch", detection.DefaultBranch).
           Int("commitsAhead", detection.CommitsAhead).
           Msg("Starting auto-branch flow")

       result := &AutoBranchResult{
           CommitsMoved: detection.CommitsAhead,
       }

       // Step 1: Check if user wants to create branch
       shouldCreate, err := d.ShouldCreateBranch()
       if err != nil {
           return nil, fmt.Errorf("failed to get user confirmation: %w", err)
       }
       if !shouldCreate {
           return nil, fmt.Errorf("auto-branch cancelled by user")
       }

       // Add blank line for visual separation
       fmt.Println()

       // ... rest of method with existing messages ...
   }
   ```

**Testing**:
```bash
go build ./internal/diff
```

**Commit message**:
```
feat(diff): add progress indicators to auto-branch

Add visual feedback and progress messages:
- Blank lines for visual separation
- Clear step-by-step progress indicators
- Consistent formatting with existing diff command output
```

---

### Task 17: Add Logging for Debugging

**Goal**: Add comprehensive debug logging.

**Files to modify**:
- `internal/diff/auto_branch.go`

**Implementation steps**:

1. Add debug logging throughout auto-branch flow:
   ```go
   func (d *AutoBranchDetector) ExecuteAutoBranch(...) (*AutoBranchResult, error) {
       logger.Info().
           Str("defaultBranch", detection.DefaultBranch).
           Int("commitsAhead", detection.CommitsAhead).
           Msg("Starting auto-branch flow")

       // ... existing code ...

       if !status.IsClean {
           logger.Debug().
               Int("stagedFiles", len(status.StagedFiles)).
               Int("unstagedFiles", len(status.UnstagedFiles)).
               Int("untrackedFiles", len(status.UntrackedFiles)).
               Msg("Working directory has uncommitted changes")

           shouldStash, err := d.ShouldStashChanges()
           // ... existing code ...

           if shouldStash {
               logger.Debug().Msg("Stashing uncommitted changes")
               // ... existing stash code ...
               logger.Debug().
                   Bool("stashCreated", true).
                   Msg("Successfully stashed changes")
           }
       } else {
           logger.Debug().Msg("Working directory is clean, no stash needed")
       }

       // ... rest of method with added logging ...

       logger.Info().
           Str("branchName", branchName).
           Int("commitsMoved", result.CommitsMoved).
           Bool("stashCreated", result.StashCreated).
           Msg("Auto-branch flow completed successfully")

       return result, nil
   }
   ```

**Testing**:
```bash
go build ./internal/diff
./gh-arc diff -v  # Test with verbose flag
```

**Commit message**:
```
feat(diff): add comprehensive debug logging to auto-branch

Add structured logging throughout auto-branch flow:
- Detection results
- User decisions (config vs prompt)
- Working directory status
- Branch operations
- Stash operations
- Final result summary

Enables debugging with -v flag: gh arc diff -v
```

---

### Task 18: Add Configuration Examples

**Goal**: Create example configuration files.

**Files to create**:
- `docs/wip/auto-branch-from-main/examples/config-auto.yaml`
- `docs/wip/auto-branch-from-main/examples/config-manual.yaml`
- `docs/wip/auto-branch-from-main/examples/config-custom-pattern.yaml`

**Implementation steps**:

1. Create fully automatic config:
   ```yaml
   # config-auto.yaml
   # Fully automatic auto-branch configuration
   # No prompts, everything happens automatically

   diff:
     # Enable stacking for trunk-based development
     enableStacking: true

     # Auto-branch from main settings (all enabled)
     autoCreateBranchFromMain: true
     autoStashUncommittedChanges: true
     autoResetMain: true
     autoBranchNamePattern: ""  # Use default: feature/auto-from-main-{timestamp}

     # Other diff settings
     createAsDraft: false
     requireTestPlan: true
   ```

2. Create manual/prompt config:
   ```yaml
   # config-manual.yaml
   # Manual/prompt-based auto-branch configuration
   # User is prompted for each decision

   diff:
     enableStacking: true

     # Auto-branch from main settings (all prompt)
     autoCreateBranchFromMain: false  # Prompt: "Create feature branch automatically?"
     autoStashUncommittedChanges: false  # Prompt: "Stash uncommitted changes?"
     autoResetMain: false  # Prompt: "Reset main to origin/main?"
     autoBranchNamePattern: "null"  # Prompt: "Enter branch name:"

     createAsDraft: false
     requireTestPlan: true
   ```

3. Create custom pattern config:
   ```yaml
   # config-custom-pattern.yaml
   # Custom branch naming pattern examples

   diff:
     enableStacking: true

     # Auto-branch with custom naming
     autoCreateBranchFromMain: true
     autoStashUncommittedChanges: true
     autoResetMain: true

     # Pattern options:
     # Option 1: Username and date
     autoBranchNamePattern: "feature/{username}-{date}"
     # Generates: feature/john-2025-10-16

     # Option 2: Datetime for uniqueness
     # autoBranchNamePattern: "auto/{datetime}"
     # Generates: auto/2025-10-16T143022

     # Option 3: Random suffix
     # autoBranchNamePattern: "fix/emergency-{random}"
     # Generates: fix/emergency-a7k3m9

     # Option 4: Timestamp (numeric)
     # autoBranchNamePattern: "wip/{timestamp}"
     # Generates: wip/1697654321

     createAsDraft: false
     requireTestPlan: true
   ```

**Testing**:
```bash
# Verify YAML is valid
cat docs/wip/auto-branch-from-main/examples/config-auto.yaml
```

**Commit message**:
```
docs: add auto-branch configuration examples

Add three example configurations:
- config-auto.yaml: Fully automatic (no prompts)
- config-manual.yaml: All prompts (user control)
- config-custom-pattern.yaml: Custom branch naming patterns

Includes comments explaining each option and example outputs.
```

---

## Phase 6: Documentation and Testing

### Task 19: Write Feature Documentation

**Goal**: Document the feature for users.

**Files to modify**:
- `README.md`

**Implementation steps**:

1. Open `README.md`
2. Find the configuration section
3. Add auto-branch configuration documentation:
   ```markdown
   ### Auto-Branch from Main

   If you accidentally commit to `main`/`master`, `gh-arc diff` can automatically create a feature branch and clean up main:

   ```bash
   # You're on main with 2 commits ahead
   $ gh arc diff

   ⚠️  Warning: You have 2 commits on main
   ✓ Creating feature branch: feature/auto-from-main-1697654321
   ✓ Created feature branch 'feature/auto-from-main-1697654321' with your 2 commits

   # ... normal PR creation flow ...

   ✓ Success!
     PR #42: https://github.com/user/repo/pull/42
   ✓ Reset main to origin/main
   ```

   #### Configuration

   Control auto-branch behavior in `.arc.json`:

   ```json
   {
     "diff": {
       "autoCreateBranchFromMain": true,        // Auto-create branch (false = prompt)
       "autoStashUncommittedChanges": true,     // Auto-stash changes (false = prompt)
       "autoResetMain": true,                   // Auto-reset main (false = prompt)
       "autoBranchNamePattern": ""              // Branch naming (see below)
     }
   }
   ```

   #### Branch Naming

   Customize branch names with `autoBranchNamePattern`:

   ```json
   // Default: feature/auto-from-main-{timestamp}
   "autoBranchNamePattern": ""

   // Prompt user for name
   "autoBranchNamePattern": "null"

   // Custom patterns with placeholders:
   "autoBranchNamePattern": "feature/{username}-{date}"        // feature/john-2025-10-16
   "autoBranchNamePattern": "auto/{datetime}"                  // auto/2025-10-16T143022
   "autoBranchNamePattern": "fix/emergency-{random}"           // fix/emergency-a7k3m9
   "autoBranchNamePattern": "wip/{timestamp}"                  // wip/1697654321
   ```

   **Placeholders:**
   - `{timestamp}` - Unix timestamp
   - `{date}` - ISO date (YYYY-MM-DD)
   - `{datetime}` - ISO datetime
   - `{username}` - Git user.name
   - `{random}` - 6-character random string

   #### Disabling Auto-Branch

   Set all to `false` to require manual prompts:

   ```json
   {
     "diff": {
       "autoCreateBranchFromMain": false,
       "autoStashUncommittedChanges": false,
       "autoResetMain": false
     }
   }
   ```

   Or decline when prompted:

   ```bash
   $ gh arc diff
   ⚠️  Warning: You have 2 commits on main
   ? Create feature branch automatically? (Y/n) n
   ✗ Cannot create PR from main to main.
   ```
   ```

**Testing**:
```bash
# Verify markdown renders correctly
cat README.md | grep -A 50 "Auto-Branch from Main"
```

**Commit message**:
```
docs: document auto-branch from main feature

Add comprehensive documentation to README:
- Feature overview with example output
- Configuration options
- Branch naming patterns with examples
- How to disable or customize behavior
- Placeholder reference

Includes code examples in bash and JSON.
```

---

### Task 20: Write Integration Tests

**Goal**: Add integration tests for the complete flow.

**Files to create**:
- `internal/diff/auto_branch_integration_test.go`

**Implementation steps**:

1. Create comprehensive integration test:
   ```go
   // +build integration

   package diff_test

   import (
       "context"
       "os"
       "path/filepath"
       "testing"
       "time"

       "github.com/go-git/go-git/v5"
       "github.com/go-git/go-git/v5/plumbing/object"

       "github.com/serpro69/gh-arc/internal/config"
       "github.com/serpro69/gh-arc/internal/diff"
       gitpkg "github.com/serpro69/gh-arc/internal/git"
   )

   func TestAutoBranch_Integration(t *testing.T) {
       if testing.Short() {
           t.Skip("Skipping integration test in short mode")
       }

       t.Run("full auto-branch flow with automatic settings", func(t *testing.T) {
           // Setup: Create test repository
           tmpDir := t.TempDir()
           gitRepo, err := git.PlainInit(tmpDir, false)
           if err != nil {
               t.Fatalf("Failed to init repository: %v", err)
           }

           // Create initial commit on main
           worktree, _ := gitRepo.Worktree()
           initialFile := filepath.Join(tmpDir, "README.md")
           os.WriteFile(initialFile, []byte("# Test"), 0644)
           worktree.Add("README.md")
           worktree.Commit("Initial commit", &git.CommitOptions{
               Author: &object.Signature{
                   Name:  "Test User",
                   Email: "test@example.com",
                   When:  time.Now(),
               },
           })

           // Simulate origin/main by creating a tag
           head, _ := gitRepo.Head()
           gitRepo.CreateTag("refs/tags/origin-main-sim", head.Hash(), &git.CreateTagOptions{
               Message: "Simulated origin/main",
           })

           // Add 2 more commits on main (simulating the mistake)
           for i := 1; i <= 2; i++ {
               testFile := filepath.Join(tmpDir, "file"+string(rune(i))+".txt")
               os.WriteFile(testFile, []byte("content"), 0644)
               worktree.Add("file" + string(rune(i)) + ".txt")
               worktree.Commit("Commit "+string(rune(i)), &git.CommitOptions{
                   Author: &object.Signature{
                       Name:  "Test User",
                       Email: "test@example.com",
                       When:  time.Now(),
                   },
               })
           }

           // Open with our wrapper
           repo, err := gitpkg.OpenRepository(tmpDir)
           if err != nil {
               t.Fatalf("Failed to open repository: %v", err)
           }

           // Configure auto-branch (all automatic)
           cfg := &config.DiffConfig{
               AutoCreateBranchFromMain:    true,
               AutoStashUncommittedChanges: true,
               AutoResetMain:               true,
               AutoBranchNamePattern:       "", // Use default
           }

           detector := diff.NewAutoBranchDetector(repo, cfg)

           // Test detection
           ctx := context.Background()
           detection, err := detector.DetectCommitsOnMain(ctx)
           if err != nil {
               t.Fatalf("DetectCommitsOnMain() failed: %v", err)
           }

           if !detection.OnMainBranch {
               t.Error("Should detect being on main branch")
           }
           if detection.CommitsAhead != 2 {
               t.Errorf("CommitsAhead = %d, expected 2", detection.CommitsAhead)
           }

           // Execute auto-branch
           result, err := detector.ExecuteAutoBranch(ctx, detection)
           if err != nil {
               t.Fatalf("ExecuteAutoBranch() failed: %v", err)
           }

           // Verify result
           if result.BranchCreated == "" {
               t.Error("Branch should be created")
           }
           if result.CommitsMoved != 2 {
               t.Errorf("CommitsMoved = %d, expected 2", result.CommitsMoved)
           }

           // Verify we're on new branch
           currentBranch, err := repo.GetCurrentBranch()
           if err != nil {
               t.Fatalf("GetCurrentBranch() failed: %v", err)
           }
           if currentBranch != result.BranchCreated {
               t.Errorf("CurrentBranch = %s, expected %s", currentBranch, result.BranchCreated)
           }

           // Verify commits are on new branch
           commits, err := repo.GetCommitRange("main", currentBranch)
           if err != nil {
               t.Fatalf("GetCommitRange() failed: %v", err)
           }
           if len(commits) != 2 {
               t.Errorf("New branch has %d commits, expected 2", len(commits))
           }

           t.Log("✓ Integration test passed: auto-branch flow works end-to-end")
       })

       // Add more integration tests for other scenarios...
   }
   ```

**Testing**:
```bash
# Run integration tests
go test -tags=integration ./internal/diff -run TestAutoBranch_Integration -v

# Run all tests
go test ./...
```

**Commit message**:
```
test(diff): add integration tests for auto-branch flow

Add comprehensive integration test covering:
- Repository setup with initial commit
- Simulating commits on main
- Auto-branch detection
- Full auto-branch execution
- Verification of branch creation and commit movement

Marked with integration build tag for separation from unit tests.
Run with: go test -tags=integration ./internal/diff
```

---

### Task 21: Update Architecture Documentation

**Goal**: Document the new auto-branch module in architecture docs.

**Files to modify**:
- `docs/contributing/ARCHITECTURE.md`

**Implementation steps**:

1. Open `docs/contributing/ARCHITECTURE.md`
2. Find the `internal/diff/` package section
3. Add auto-branch documentation:
   ```markdown
   #### `internal/diff/` - Diff Workflow

   **Purpose:** Implements diff command workflow logic.

   **Key Components:**
   - `base.go` - Base branch detection for stacking
   - `stacking.go` - Stacking logic and PR relationships
   - `commit_analysis.go` - Commit message parsing for templates
   - `dependent.go` - Dependent PR detection
   - `output.go` - Diff output formatting
   - `auto_branch.go` - **NEW:** Auto-branch from main detection and execution

   **Auto-Branch Module (`auto_branch.go`):**

   Handles the scenario where a user accidentally commits directly to main/master:

   ```
   Detection → User Confirmation → Stash → Create Branch → Checkout → Unstash
   ```

   **Key Types:**
   - `AutoBranchDetector` - Main detector and orchestrator
   - `DetectionResult` - Result of commit detection
   - `AutoBranchResult` - Result of auto-branch operation

   **Key Methods:**
   - `DetectCommitsOnMain()` - Detects commits ahead of origin/main
   - `ShouldAutoBranch()` - Determines if flow should activate
   - `ExecuteAutoBranch()` - Orchestrates the full auto-branch flow
   - `GenerateBranchName()` - Generates branch name from pattern
   - `ResetMainBranch()` - Resets main to origin after successful PR

   **Configuration:**
   - `autoCreateBranchFromMain` - Enable/disable auto-branch
   - `autoStashUncommittedChanges` - Auto-stash or prompt
   - `autoResetMain` - Auto-reset main or prompt
   - `autoBranchNamePattern` - Branch naming pattern

   **Integration Point:**

   Called at the start of `cmd/diff.go:runDiff()`:
   1. Detect commits on main
   2. Execute auto-branch if detected
   3. Update currentBranch variable
   4. Continue normal diff flow
   5. Reset main after successful PR (if configured)

   **Thread Safety:**
   Not thread-safe. Designed for single-user CLI usage.
   ```

**Testing**:
```bash
# Verify markdown
cat docs/contributing/ARCHITECTURE.md | grep -A 50 "auto_branch.go"
```

**Commit message**:
```
docs(architecture): document auto-branch module

Add auto-branch module documentation to ARCHITECTURE.md:
- Module purpose and workflow
- Key types and methods
- Configuration options
- Integration points with diff command
- Thread safety notes

Provides complete reference for developers.
```

---

## Final Steps

### Task 22: Create ADR for Auto-Branch Feature

**Goal**: Document the architectural decision.

**Files to create**:
- `docs/adr/0002-auto-branch-from-main.md`

**Implementation steps**:

1. Create ADR following the template from ADR-0001:
   ```markdown
   # ADR 0002: Auto-Branch from Main

   ## Status

   Accepted

   ## Date

   2025-10-16

   ## Context

   [Content from design document explaining the problem and alternatives]

   ## Decision

   We will implement automatic feature branch creation when commits are detected on main/master.

   [Details from design document]

   ## Consequences

   [Positive, negative, and neutral consequences]

   ## References

   - Design Document: `docs/wip/auto-branch-from-main/design/feature-design.md`
   - Implementation Plan: `docs/wip/auto-branch-from-main/implementation/implementation-plan.md`
   ```

**Testing**:
```bash
cat docs/adr/0002-auto-branch-from-main.md
```

**Commit message**:
```
docs(adr): add ADR for auto-branch from main feature

Document architectural decision for auto-branch feature:
- Problem context
- Decision rationale
- Implementation approach
- Consequences and trade-offs

References design and implementation documentation.
```

---

## Summary

This implementation plan provides 22 detailed, bite-sized tasks organized into 6 phases:

1. **Phase 1 (Tasks 1-3):** Configuration infrastructure
2. **Phase 2 (Tasks 4-8):** Git operations
3. **Phase 3 (Tasks 9-12):** Detection and auto-branch logic
4. **Phase 4 (Tasks 13-15):** Integration with diff command
5. **Phase 5 (Tasks 16-18):** User interaction and polish
6. **Phase 6 (Tasks 19-21):** Documentation and testing
7. **Final (Task 22):** ADR creation

Each task:
- Has clear goals and file locations
- Includes step-by-step implementation instructions
- Provides complete code examples
- Includes testing procedures
- Has a commit message template
- Can be implemented independently

Follow TDD principles by implementing tests alongside code in each task.

## Testing Strategy

- **Unit tests:** Write alongside each implementation task
- **Integration tests:** Task 20 (requires full git environment)
- **Manual testing:** After each phase completion
- **End-to-end testing:** After Phase 4 completion

## Commit Frequency

Commit after completing each task (22 commits minimum). This ensures:
- Incremental progress tracking
- Easy rollback if needed
- Clear commit history
- Reviewable changes

## Questions or Issues?

Refer to:
- `docs/wip/auto-branch-from-main/design/feature-design.md` for feature design
- `docs/contributing/ARCHITECTURE.md` for codebase architecture
- `docs/contributing/TESTING.md` for testing patterns
- `docs/contributing/WORKFLOWS.md` for common workflows
