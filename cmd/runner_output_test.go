package cmd

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/serpro69/gh-arc/internal/runner"
	"github.com/spf13/cobra"
)

func TestRunnerJSONWriteFailure(t *testing.T) {
	originalJSON := jsonOut
	t.Cleanup(func() { jsonOut = originalJSON })
	jsonOut = true
	writer := &partialJSONWriter{}
	command := &cobra.Command{Use: "unit"}
	command.SetOut(writer)
	err := finishRunnerCommand(command, runner.ExecutionResult{Success: true}, "unit")
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("lost write error: %v", err)
	}
	err = reportRunnerError(command, err)
	if !errors.Is(err, io.ErrClosedPipe) || writer.calls != 1 {
		t.Fatalf("error reporter retried after partial JSON: error=%v writes=%d", err, writer.calls)
	}
}

type partialJSONWriter struct {
	calls int
}

func (w *partialJSONWriter) Write(p []byte) (int, error) {
	w.calls++
	return len(p) / 2, io.ErrClosedPipe
}

func TestRunnerSilentExitDoesNotEmitTwice(t *testing.T) {
	originalJSON := jsonOut
	t.Cleanup(func() { jsonOut = originalJSON })
	jsonOut = true
	var output bytes.Buffer
	command := &cobra.Command{Use: "lint"}
	command.SetOut(&output)
	err := finishRunnerCommand(command, runner.ExecutionResult{Success: false}, "lint")
	before := output.String()
	if !errors.Is(reportRunnerError(command, err), ErrSilentExit) || output.String() != before {
		t.Fatal("error reporter duplicated an execution result")
	}
}
