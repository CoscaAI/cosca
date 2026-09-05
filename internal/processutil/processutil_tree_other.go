//go:build !unix && !windows

package processutil

import "os/exec"

// treeKiller is the best-effort fallback for platforms with neither POSIX
// process groups nor the Windows Job Object API: it only kills the direct
// process. On such platforms the idle/hard monitors plus a WaitDelay-esque
// callers' own timeout still bound execution.
type treeKiller struct {
	cmd *exec.Cmd
}

func newTreeKiller(cmd *exec.Cmd) *treeKiller { return &treeKiller{cmd: cmd} }

func (tk *treeKiller) attach() {}

func (tk *treeKiller) kill() error {
	if tk.cmd.Process != nil {
		return tk.cmd.Process.Kill()
	}
	return nil
}

func (tk *treeKiller) release() {}
