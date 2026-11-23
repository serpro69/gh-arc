package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/serpro69/gh-arc/internal/config"
	"github.com/serpro69/gh-arc/internal/runner"
	"github.com/spf13/cobra"
)

func isRunnerCommand(cmd *cobra.Command) bool {
	return cmd != nil && (cmd.Name() == "lint" || cmd.Name() == "unit")
}

// Lint/unit accept only boolean/count flags, so --json cannot be another flag's
// string value. Stop at -- so positional text cannot change the output mode.
func runnerJSONFlag(args []string) (value, found bool) {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--json" {
			value, found = true, true
		} else if raw, ok := strings.CutPrefix(arg, "--json="); ok {
			if parsed, err := strconv.ParseBool(raw); err == nil {
				value, found = parsed, true
			}
		}
	}
	return value, found
}

func loadRunnerConfig(cmd *cobra.Command) (*config.Config, error) {
	if cmd.Root() == rootCmd && configInitialized {
		return cfg, configErr
	}
	// Direct handler callers do not run Cobra's configuration initializer.
	loaded, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if err := loaded.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return loaded, nil
}

// runnerOutputError prevents appending a second JSON object after a partial write.
type runnerOutputError struct{ err error }

func (e *runnerOutputError) Error() string { return e.err.Error() }
func (e *runnerOutputError) Unwrap() error { return e.err }

func writeRunnerJSON(cmd *cobra.Command, output []byte) error {
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(output)); err != nil {
		return &runnerOutputError{err: fmt.Errorf("failed to write runner results: %w", err)}
	}
	return nil
}

func finishRunnerCommand(cmd *cobra.Command, result runner.ExecutionResult, command string) error {
	if GetJSON() {
		output, err := runner.FormatJSON(result, command)
		if err != nil {
			return fmt.Errorf("failed to format %s results: %w", command, err)
		}
		if err := writeRunnerJSON(cmd, output); err != nil {
			return err
		}
	}
	if !result.Success {
		return ErrSilentExit
	}
	return nil
}

func reportRunnerError(cmd *cobra.Command, err error) error {
	var outputErr *runnerOutputError
	if !GetJSON() || errors.Is(err, ErrSilentExit) || errors.As(err, &outputErr) {
		return err
	}
	output, marshalErr := json.MarshalIndent(struct {
		Command string            `json:"command"`
		Success bool              `json:"success"`
		Runners []json.RawMessage `json:"runners"`
		Error   string            `json:"error"`
	}{Command: cmd.Name(), Runners: []json.RawMessage{}, Error: err.Error()}, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("failed to format command error: %w", marshalErr)
	}
	if writeErr := writeRunnerJSON(cmd, output); writeErr != nil {
		return writeErr
	}
	return ErrSilentExit
}
