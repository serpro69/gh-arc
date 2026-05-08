package land

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/github"
)

// WorkflowRepo defines git operations needed by the land workflow.
type WorkflowRepo interface {
	CheckerRepo
	CleanupRepo
	GetCurrentBranch() (string, error)
	GetDefaultBranch() (string, error)
}

// WorkflowClient defines GitHub operations needed by the land workflow.
type WorkflowClient interface {
	CheckerClient
	MergerClient
	EnrichPullRequest(ctx context.Context, owner, repo string, pr *github.PullRequest) error
}

// LandOptions holds the flags and options for the land command.
type LandOptions struct {
	Squash   bool
	Rebase   bool
	Force    bool
	Edit     bool
	NoDelete bool
}

// LandWorkflow orchestrates the entire land command sequence.
type LandWorkflow struct {
	repo       WorkflowRepo
	client     WorkflowClient
	config     *config.Config
	owner      string
	name       string
	checker    *PreMergeChecker
	merger     *MergeExecutor
	cleanup    *PostMergeCleanup
	output     *OutputStyle
	stdin      io.Reader
	isTerminal func() bool
}

// NewLandWorkflow creates a new LandWorkflow with all sub-components.
func NewLandWorkflow(repo WorkflowRepo, client WorkflowClient, cfg *config.Config, owner, name string) *LandWorkflow {
	return &LandWorkflow{
		repo:       repo,
		client:     client,
		config:     cfg,
		owner:      owner,
		name:       name,
		checker:    NewPreMergeChecker(repo, client, &cfg.Land),
		merger:     NewMergeExecutor(client),
		cleanup:    NewPostMergeCleanup(repo),
		output:     NewOutputStyle(cfg.Output.Color),
		stdin:      os.Stdin,
		isTerminal: func() bool { return term.IsTerminal(int(os.Stdin.Fd())) },
	}
}

// Execute runs the full land workflow: pre-merge checks, merge, and cleanup.
func (w *LandWorkflow) Execute(ctx context.Context, opts *LandOptions) (*LandResult, error) {
	if opts == nil {
		opts = &LandOptions{}
	}

	if err := w.checker.CheckCleanWorkingDir(); err != nil {
		w.output.PrintStep("✗", "Working directory has uncommitted changes")
		w.output.PrintDetail("Commit or stash your changes before landing")
		return nil, err
	}

	currentBranch, err := w.repo.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	defaultBranch, err := w.repo.GetDefaultBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get default branch: %w", err)
	}

	if err := w.checker.CheckNotOnTrunk(currentBranch, defaultBranch); err != nil {
		w.output.PrintStep("✗", fmt.Sprintf("Cannot land from %s — already on default branch", currentBranch))
		return nil, err
	}

	pr, err := w.checker.CheckPRExists(ctx, currentBranch)
	if err != nil {
		w.output.PrintStep("✗", "No open pull request found for current branch")
		w.output.PrintDetail("Run 'gh arc diff' to create one")
		return nil, err
	}
	w.output.PrintPRFound(pr)

	if err := w.checker.CheckLocalHeadMatchesPR(pr); err != nil {
		w.output.PrintStep("✗", "Local HEAD does not match PR head")
		w.output.PrintDetail("Push your changes with 'gh arc diff' or 'git push' before landing")
		return nil, err
	}

	if err := w.client.EnrichPullRequest(ctx, w.owner, w.name, pr); err != nil {
		return nil, fmt.Errorf("failed to enrich pull request: %w", err)
	}

	approvalResult, err := w.checker.CheckApproval(ctx, pr, opts.Force)
	if err != nil {
		return nil, fmt.Errorf("failed to check approval: %w", err)
	}
	for _, msg := range approvalResult.Messages {
		w.output.PrintApprovalStatus(approvalResult.Passed, msg)
	}
	if !approvalResult.Passed {
		if approvalResult.NeedsConfirmation {
			confirmed, promptErr := w.promptConfirmation()
			if promptErr != nil {
				return nil, promptErr
			}
			if !confirmed {
				w.output.PrintDetail("Merge cancelled")
				return nil, ErrMergeDeclined
			}
		} else {
			w.output.PrintDetail("Request a review or use --force to bypass")
			return nil, ErrApprovalFailed
		}
	}

	ciResult, err := w.checker.CheckCI(ctx, pr, opts.Force)
	if err != nil {
		return nil, fmt.Errorf("failed to check CI: %w", err)
	}
	for _, msg := range ciResult.Messages {
		w.output.PrintCIStatus(ciResult.Passed, msg)
	}
	if !ciResult.Passed {
		w.output.PrintDetail("Wait for checks to complete or use --force to bypass")
		return nil, ErrCIFailed
	}

	dependentPRs, err := w.checker.CheckDependentPRs(ctx, currentBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to check dependent PRs: %w", err)
	}
	w.output.PrintDependentPRs(len(dependentPRs))

	mergeMethod := w.resolveMergeMethod(opts)

	mergeResult, err := w.merger.Execute(ctx, &MergeRequest{
		PR:     pr,
		Method: mergeMethod,
		Edit:   opts.Edit,
	})
	if err != nil {
		w.output.PrintStep("✗", fmt.Sprintf("Merge failed: %v", err))
		return nil, err
	}
	w.output.PrintMerged(mergeMethod, pr.Base.Ref, mergeResult.SHA)

	noDelete := opts.NoDelete || !w.config.Land.DeleteLocalBranch
	cleanupResult, err := w.cleanup.Execute(defaultBranch, currentBranch, noDelete)
	if err != nil {
		return nil, fmt.Errorf("cleanup failed: %w", err)
	}
	w.printCleanupResult(cleanupResult, defaultBranch, currentBranch)

	return &LandResult{
		PR:               pr,
		MergeMethod:      mergeMethod,
		MergeCommitSHA:   mergeResult.SHA,
		DefaultBranch:    defaultBranch,
		DeletedBranch:    deletedBranchName(cleanupResult, currentBranch),
		DeletedBranchSHA: cleanupResult.DeletedBranchSHA,
		DependentPRCount: len(dependentPRs),
		CleanupWarnings:  cleanupResult.Warnings,
	}, nil
}

func (w *LandWorkflow) resolveMergeMethod(opts *LandOptions) string {
	if opts.Squash {
		return config.MergeMethodSquash
	}
	if opts.Rebase {
		return config.MergeMethodRebase
	}
	return w.config.Land.DefaultMergeMethod
}

func (w *LandWorkflow) promptConfirmation() (bool, error) {
	if !w.isTerminal() {
		w.output.PrintDetail("Non-interactive environment — use --force to bypass approval check")
		return false, ErrNonInteractive
	}

	fmt.Fprint(w.output.writer, "  Proceed with merge? [y/N] ")
	reader := bufio.NewReader(w.stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(strings.ToLower(line)) == "y", nil
}

func (w *LandWorkflow) printCleanupResult(result *CleanupResult, defaultBranch, featureBranch string) {
	if result.CheckedOut && result.Pulled {
		w.output.PrintCheckout(defaultBranch)
	}
	if result.BranchDeleted {
		w.output.PrintBranchDeleted(featureBranch, result.DeletedBranchSHA)
	}
	for _, warning := range result.Warnings {
		w.output.PrintCleanupWarning(warning)
	}
}

func deletedBranchName(result *CleanupResult, featureBranch string) string {
	if result.BranchDeleted {
		return featureBranch
	}
	return ""
}
