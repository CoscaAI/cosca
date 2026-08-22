//go:build !windows

package bootstrap

import (
	"os"
	"os/exec"
	"syscall"
)

// startDetachedProcess starts name in the background, detached from the
// calling process (new session via Setsid when the platform supports it, so
// the child survives the bootstrap process exiting), with output discarded.
//
// REVERSIBLE: the started process is `ollama serve`; kill it with
// `kill <pid>` (the pid is surfaced in the report evidence) or, when started
// via systemd, `systemctl --user stop ollama`.
func startDetachedProcess(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devnull.Close()
	cmd.Stdin = devnull
	cmd.Stdout = devnull
	cmd.Stderr = devnull
	if err := cmd.Start(); err != nil {
		return err
	}
	// Release resources; the child keeps running detached.
	return cmd.Process.Release()
}
