//go:build unix

package processutil

import (
	"os/exec"
	"sync"
	"syscall"
)

// treeKiller places the command in its own POSIX process group (Setpgid) so a
// SIGKILL on -pid terminates the whole group — the same pattern used by the
// evals verify helper, moved here so it is shared.
type treeKiller struct {
	mu  sync.Mutex
	cmd *exec.Cmd
}

func newTreeKiller(cmd *exec.Cmd) *treeKiller {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &treeKiller{cmd: cmd}
}

// attach is a no-op on Unix: the process group is established via SysProcAttr
// before the process is created, so every child is already in the group.
func (tk *treeKiller) attach() {}

func (tk *treeKiller) kill() error {
	tk.mu.Lock()
	defer tk.mu.Unlock()
	if tk.cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-tk.cmd.Process.Pid, syscall.SIGKILL)
	if err == syscall.ESRCH {
		return nil
	}
	return err
}

func (tk *treeKiller) release() {}
