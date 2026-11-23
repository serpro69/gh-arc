package cmd

import "testing"

func TestUnitCommand(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"unit"})
	if err != nil || found != unitCmd {
		t.Fatalf("unit command not registered: %v", err)
	}
	if unitCmd.RunE == nil || unitCmd.Args == nil {
		t.Fatal("unit must have a handler and argument validation")
	}
	if err := unitCmd.Args(unitCmd, []string{"unexpected"}); err == nil {
		t.Fatal("unit accepted positional arguments")
	}
	if err := unitCmd.Args(unitCmd, nil); err != nil {
		t.Fatalf("unit rejected an empty argument list: %v", err)
	}
}
