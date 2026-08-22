//go:build unix

package evals

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup puts the command in its own process group and makes
// context cancellation SIGKILL the whole group. Killing only the direct
// process (sh) would leave children (e.g. sleep) holding the output pipe
// open, which would block CombinedOutput past the deadline.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
}
