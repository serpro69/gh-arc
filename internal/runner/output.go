package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

func PrintBanner(w io.Writer, name string) {
	fmt.Fprintf(w, "▶ Running %s...\n", name)
}

func PrintResult(w io.Writer, result RunResult) {
	switch result.Status {
	case StatusPassed:
		fmt.Fprintf(w, "\n✓ %s: passed [%s]\n", result.Name, formatDuration(result.Duration))
	case StatusFailed:
		fmt.Fprintf(w, "\n✗ %s: failed (exit code %d) [%s]\n", result.Name, result.ExitCode, formatDuration(result.Duration))
	case StatusError:
		if result.Err != nil {
			fmt.Fprintf(w, "\n✗ %s: %s\n", result.Name, result.Err)
		} else {
			fmt.Fprintf(w, "\n✗ %s: error\n", result.Name)
		}
	case StatusSkipped:
		fmt.Fprintf(w, "\n⚠ %s: skipped\n", result.Name)
	}
}

func PrintSummary(w io.Writer, result ExecutionResult) {
	total := len(result.Runners)
	if total == 0 {
		return
	}

	failed := result.FailedCount()
	noun := "runners"
	if total == 1 {
		noun = "runner"
	}

	fmt.Fprintf(w, "\n━━━\n")
	if failed == 0 {
		fmt.Fprintf(w, "✓ %d %s passed\n", total, noun)
	} else if total == 1 {
		fmt.Fprintf(w, "✗ 1 %s failed\n", noun)
	} else {
		fmt.Fprintf(w, "✗ %d of %d %s failed\n", failed, total, noun)
	}
}

type jsonOutput struct {
	Command string       `json:"command"`
	Success bool         `json:"success"`
	Runners []jsonRunner `json:"runners"`
	Skipped string       `json:"skipped,omitempty"`
}

type jsonRunner struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

func FormatJSON(result ExecutionResult, command string) ([]byte, error) {
	runners := make([]jsonRunner, len(result.Runners))
	for i, r := range result.Runners {
		var errStr string
		if r.Err != nil {
			errStr = r.Err.Error()
		}
		runners[i] = jsonRunner{
			Name:       r.Name,
			Status:     string(r.Status),
			ExitCode:   r.ExitCode,
			DurationMs: r.Duration.Milliseconds(),
			Error:      errStr,
		}
	}

	out := jsonOutput{
		Command: command,
		Success: result.Success,
		Runners: runners,
		Skipped: result.SkipReason,
	}

	return json.MarshalIndent(out, "", "  ")
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return d.Truncate(time.Second).String()
}
