package cmd

import "errors"

// ErrSilentExit requests exit code 1 when the command has already reported
// its failure, so Execute must not print another error message.
var ErrSilentExit = errors.New("command failed")
