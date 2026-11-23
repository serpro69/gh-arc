package cmd

import (
	"testing"
)

func TestLintCommand(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"lint"})
	if err != nil || found != lintCmd {
		t.Fatalf("lint command not registered: %v", err)
	}
	if lintCmd.RunE == nil || lintCmd.Args == nil {
		t.Fatal("lint must have a handler and argument validation")
	}
	if err := lintCmd.Args(lintCmd, []string{"unexpected"}); err == nil {
		t.Fatal("lint accepted positional arguments")
	}
	if err := lintCmd.Args(lintCmd, nil); err != nil {
		t.Fatalf("lint rejected an empty argument list: %v", err)
	}
}

func TestLintFlags(t *testing.T) {
	for _, tt := range []struct {
		name     string
		args     []string
		fix, all bool
	}{
		{name: "defaults"},
		{name: "fix", args: []string{"--fix"}, fix: true},
		{name: "all", args: []string{"--all"}, all: true},
		{name: "combined", args: []string{"--fix", "--all"}, fix: true, all: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			originalFix, originalAll := lintFix, lintAll
			fixFlag, allFlag := lintCmd.Flags().Lookup("fix"), lintCmd.Flags().Lookup("all")
			fixChanged, allChanged := fixFlag.Changed, allFlag.Changed
			t.Cleanup(func() {
				lintFix, lintAll = originalFix, originalAll
				fixFlag.Changed, allFlag.Changed = fixChanged, allChanged
			})
			lintFix, lintAll = false, false
			if err := lintCmd.ParseFlags(tt.args); err != nil {
				t.Fatal(err)
			}
			if lintFix != tt.fix || lintAll != tt.all {
				t.Fatalf("fix/all = %v/%v, want %v/%v", lintFix, lintAll, tt.fix, tt.all)
			}
		})
	}
}
