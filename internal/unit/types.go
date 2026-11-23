package unit

import "github.com/serpro69/gh-arc/internal/runner"

// UnitOptions holds unit command output flags. The caller also configures the
// executor's JSONMode and Verbose options when constructing the runner engine.
type UnitOptions struct {
	JSONMode bool
	Verbose  bool
}

// UnitResult contains the outcomes of the configured test runners.
type UnitResult struct {
	runner.ExecutionResult
}
