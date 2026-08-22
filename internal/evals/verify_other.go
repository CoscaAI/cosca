//go:build !unix

package evals

import "os/exec"

// configureProcessGroup is a no-op on platforms without POSIX process
// groups; CommandContext still kills the direct process on timeout.
func configureProcessGroup(cmd *exec.Cmd) {}
