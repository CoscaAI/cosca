package evals

import (
	"os/exec"

	"github.com/CoscaAI/cosca/internal/processutil"
)

// configureProcessGroup wires cmd so the whole process tree can be killed on
// cancellation. The platform implementation is shared in processutil (Unix:
// Setpgid + SIGKILL -pid; Windows: a Job Object with taskkill fallback) so the
// verify runner does not leak orphaned grandchildren past a timeout.
//
// It returns a release function that frees OS resources (e.g. the Windows Job
// Object handle); call it after the command finishes running.
func configureProcessGroup(cmd *exec.Cmd) func() {
	kill, release := processutil.ConfigureProcessGroup(cmd)
	cmd.Cancel = kill
	return release
}
