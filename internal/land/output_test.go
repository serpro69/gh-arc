package land

import (
	"bytes"
	"strings"
	"testing"

	"github.com/serpro69/gh-arc/internal/github"
)

func newTestStyle() (*OutputStyle, *bytes.Buffer) {
	style := NewOutputStyle(false)
	buf := &bytes.Buffer{}
	style.writer = buf
	return style, buf
}

func TestNewOutputStyle(t *testing.T) {
	t.Run("with color enabled", func(t *testing.T) {
		style := NewOutputStyle(true)
		if style == nil {
			t.Fatal("NewOutputStyle returned nil")
		}
		if !style.useColor {
			t.Error("useColor should be true")
		}
	})

	t.Run("with color disabled", func(t *testing.T) {
		style := NewOutputStyle(false)
		if style == nil {
			t.Fatal("NewOutputStyle returned nil")
		}
		if style.useColor {
			t.Error("useColor should be false")
		}
	})
}

func TestPrintStep(t *testing.T) {
	tests := []struct {
		name     string
		icon     string
		message  string
		expected string
	}{
		{
			name:     "success icon",
			icon:     "✓",
			message:  "operation completed",
			expected: "✓ operation completed\n",
		},
		{
			name:     "error icon",
			icon:     "✗",
			message:  "operation failed",
			expected: "✗ operation failed\n",
		},
		{
			name:     "warning icon",
			icon:     "⚠",
			message:  "something to note",
			expected: "⚠ something to note\n",
		},
		{
			name:     "unknown icon",
			icon:     "?",
			message:  "unknown step",
			expected: "? unknown step\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style, buf := newTestStyle()
			style.PrintStep(tt.icon, tt.message)
			if buf.String() != tt.expected {
				t.Errorf("PrintStep() = %q, want %q", buf.String(), tt.expected)
			}
		})
	}
}

func TestPrintDetail(t *testing.T) {
	style, buf := newTestStyle()
	style.PrintDetail("Request a review or use --force to bypass")
	expected := "  Request a review or use --force to bypass\n"
	if buf.String() != expected {
		t.Errorf("PrintDetail() = %q, want %q", buf.String(), expected)
	}
}

func TestPrintPRFound(t *testing.T) {
	style, buf := newTestStyle()
	pr := &github.PullRequest{
		Number: 42,
		Title:  "Add auth middleware",
		Head:   github.PRBranch{Ref: "feature/auth"},
		Base:   github.PRBranch{Ref: "main"},
	}
	style.PrintPRFound(pr)

	output := buf.String()
	expectedParts := []string{
		"✓",
		"Found PR #42",
		`"Add auth middleware"`,
		"feature/auth → main",
	}
	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("PrintPRFound() missing %q in output: %q", part, output)
		}
	}
}

func TestPrintApprovalStatus(t *testing.T) {
	t.Run("passed", func(t *testing.T) {
		style, buf := newTestStyle()
		style.PrintApprovalStatus(true, "Approved by @alice, @bob")
		if !strings.Contains(buf.String(), "✓ Approved by @alice, @bob") {
			t.Errorf("PrintApprovalStatus(true) = %q, want success icon", buf.String())
		}
	})

	t.Run("failed", func(t *testing.T) {
		style, buf := newTestStyle()
		style.PrintApprovalStatus(false, "PR needs approval — no reviews yet")
		if !strings.Contains(buf.String(), "✗ PR needs approval") {
			t.Errorf("PrintApprovalStatus(false) = %q, want error icon", buf.String())
		}
	})
}

func TestPrintCIStatus(t *testing.T) {
	t.Run("passed", func(t *testing.T) {
		style, buf := newTestStyle()
		style.PrintCIStatus(true, "All CI checks passed (3/3)")
		if !strings.Contains(buf.String(), "✓ All CI checks passed (3/3)") {
			t.Errorf("PrintCIStatus(true) = %q, want success icon", buf.String())
		}
	})

	t.Run("failed", func(t *testing.T) {
		style, buf := newTestStyle()
		style.PrintCIStatus(false, "CI check 'tests' failed (1/3 passed)")
		if !strings.Contains(buf.String(), "✗ CI check 'tests' failed") {
			t.Errorf("PrintCIStatus(false) = %q, want error icon", buf.String())
		}
	})
}

func TestPrintDependentPRs(t *testing.T) {
	t.Run("zero dependents prints nothing", func(t *testing.T) {
		style, buf := newTestStyle()
		style.PrintDependentPRs(0)
		if buf.Len() != 0 {
			t.Errorf("PrintDependentPRs(0) should produce no output, got %q", buf.String())
		}
	})

	t.Run("single dependent", func(t *testing.T) {
		style, buf := newTestStyle()
		style.PrintDependentPRs(1)
		output := buf.String()
		if !strings.Contains(output, "⚠ 1 dependent PR targets this branch") {
			t.Errorf("PrintDependentPRs(1) = %q, want singular form", output)
		}
	})

	t.Run("multiple dependents", func(t *testing.T) {
		style, buf := newTestStyle()
		style.PrintDependentPRs(3)
		output := buf.String()
		if !strings.Contains(output, "⚠ 3 dependent PRs target this branch") {
			t.Errorf("PrintDependentPRs(3) = %q, want plural form", output)
		}
	})
}

func TestPrintMerged(t *testing.T) {
	t.Run("squash merge", func(t *testing.T) {
		style, buf := newTestStyle()
		style.PrintMerged("squash", "main", "abc1234def5678")
		output := buf.String()
		if !strings.Contains(output, "✓ Squash-merged into main (abc1234)") {
			t.Errorf("PrintMerged(squash) = %q", output)
		}
	})

	t.Run("rebase merge", func(t *testing.T) {
		style, buf := newTestStyle()
		style.PrintMerged("rebase", "main", "abc1234def5678")
		output := buf.String()
		if !strings.Contains(output, "✓ Rebased into main (abc1234)") {
			t.Errorf("PrintMerged(rebase) = %q", output)
		}
	})

	t.Run("short SHA unchanged", func(t *testing.T) {
		style, buf := newTestStyle()
		style.PrintMerged("squash", "main", "abc12")
		output := buf.String()
		if !strings.Contains(output, "(abc12)") {
			t.Errorf("PrintMerged() should keep short SHA as-is, got %q", output)
		}
	})
}

func TestPrintCheckout(t *testing.T) {
	style, buf := newTestStyle()
	style.PrintCheckout("main")
	if !strings.Contains(buf.String(), "✓ Switched to main, pulled latest") {
		t.Errorf("PrintCheckout() = %q", buf.String())
	}
}

func TestPrintBranchDeleted(t *testing.T) {
	style, buf := newTestStyle()
	style.PrintBranchDeleted("feature/auth", "a1b2c3d4e5f6")
	output := buf.String()

	expectedParts := []string{
		"✓ Deleted local branch feature/auth",
		"git checkout -b feature/auth a1b2c3d",
	}
	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("PrintBranchDeleted() missing %q in %q", part, output)
		}
	}
}

func TestPrintCleanupWarning(t *testing.T) {
	style, buf := newTestStyle()
	style.PrintCleanupWarning("Failed to delete local branch")
	if !strings.Contains(buf.String(), "⚠ Failed to delete local branch") {
		t.Errorf("PrintCleanupWarning() = %q", buf.String())
	}
}

func TestFormatLandResult(t *testing.T) {
	style := NewOutputStyle(false)

	t.Run("full result", func(t *testing.T) {
		result := &LandResult{
			PR: &github.PullRequest{
				Number: 42,
				Title:  "Add auth middleware",
			},
			MergeMethod:      "squash",
			MergeCommitSHA:   "abc1234def5678",
			DefaultBranch:    "main",
			DeletedBranch:    "feature/auth",
			DeletedBranchSHA: "a1b2c3d4e5f6",
			DependentPRCount: 2,
		}

		output := FormatLandResult(result, style)

		expectedParts := []string{
			"✓ PR #42 squash-merged into main (abc1234)",
			"✓ Cleaned up branch feature/auth",
			"git checkout -b feature/auth a1b2c3d",
			"⚠ 2 dependent PRs may be retargeted",
		}
		for _, part := range expectedParts {
			if !strings.Contains(output, part) {
				t.Errorf("FormatLandResult() missing %q in:\n%s", part, output)
			}
		}
	})

	t.Run("rebase merge", func(t *testing.T) {
		result := &LandResult{
			PR:             &github.PullRequest{Number: 10},
			MergeMethod:    "rebase",
			MergeCommitSHA: "deadbeef1234",
			DefaultBranch:  "main",
		}

		output := FormatLandResult(result, style)
		if !strings.Contains(output, "rebased into main") {
			t.Errorf("FormatLandResult() should say 'rebased' for rebase method, got:\n%s", output)
		}
	})

	t.Run("single dependent PR", func(t *testing.T) {
		result := &LandResult{
			PR:               &github.PullRequest{Number: 5},
			MergeMethod:      "squash",
			MergeCommitSHA:   "abc1234",
			DefaultBranch:    "main",
			DependentPRCount: 1,
		}

		output := FormatLandResult(result, style)
		if !strings.Contains(output, "1 dependent PR may be retargeted") {
			t.Errorf("FormatLandResult() should use singular form, got:\n%s", output)
		}
	})

	t.Run("with cleanup warnings", func(t *testing.T) {
		result := &LandResult{
			PR:             &github.PullRequest{Number: 7},
			MergeMethod:    "squash",
			MergeCommitSHA: "abc1234",
			DefaultBranch:  "main",
			CleanupWarnings: []string{
				"Failed to pull latest",
				"Failed to delete local branch",
			},
		}

		output := FormatLandResult(result, style)
		if !strings.Contains(output, "⚠ Failed to pull latest") {
			t.Errorf("FormatLandResult() missing cleanup warning, got:\n%s", output)
		}
		if !strings.Contains(output, "⚠ Failed to delete local branch") {
			t.Errorf("FormatLandResult() missing cleanup warning, got:\n%s", output)
		}
	})

	t.Run("no branch deleted", func(t *testing.T) {
		result := &LandResult{
			PR:             &github.PullRequest{Number: 8},
			MergeMethod:    "squash",
			MergeCommitSHA: "abc1234",
			DefaultBranch:  "main",
		}

		output := FormatLandResult(result, style)
		if strings.Contains(output, "Cleaned up branch") {
			t.Errorf("FormatLandResult() should not mention branch cleanup when no branch deleted, got:\n%s", output)
		}
	})
}

func TestFormatWithIconColor(t *testing.T) {
	style := NewOutputStyle(true)
	if !style.useColor {
		t.Error("useColor should be true")
	}

	// Verify methods work without panicking (color output depends on terminal)
	_ = style.formatWithIcon("✓", "test")
	_ = style.formatWithIcon("✗", "test")
	_ = style.formatWithIcon("⚠", "test")
}

func TestTruncateSHA(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abc1234def5678", "abc1234"},
		{"abc1234", "abc1234"},
		{"abc12", "abc12"},
		{"", ""},
	}

	for _, tt := range tests {
		result := truncateSHA(tt.input)
		if result != tt.expected {
			t.Errorf("truncateSHA(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestStepAndDetailTogether(t *testing.T) {
	style, buf := newTestStyle()

	style.PrintApprovalStatus(false, "PR needs approval — no reviews yet")
	style.PrintDetail("Request a review or use --force to bypass")

	output := buf.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), output)
	}
	if !strings.HasPrefix(lines[0], "✗") {
		t.Errorf("first line should start with ✗, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  ") {
		t.Errorf("second line should be indented, got %q", lines[1])
	}
}
