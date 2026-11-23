package lint

import "github.com/serpro69/gh-arc/internal/runner"

// LintOptions holds lint-specific flags. The caller also configures the
// executor's JSONMode and Verbose options when constructing the runner engine.
type LintOptions struct {
	Fix      bool
	All      bool
	JSONMode bool
	Verbose  bool
}

// LintResult includes runner outcomes and the scope selected for this run.
// Runners with no changes inside their working directory are omitted.
type LintResult struct {
	runner.ExecutionResult
	ChangedFileCount int
	AllMode          bool
}
