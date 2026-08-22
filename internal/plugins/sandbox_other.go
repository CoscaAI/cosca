//
// Non-Linux sandbox: rlimits not available; sandbox is minimal.

//go:build !linux

package plugins

import "os/exec"

// applySandboxToCmd applies resource limits. On non-Linux, rlimits are not
// available. External plugins on these platforms must rely on OS-level
// isolation mechanisms (e.g., macOS sandbox, Windows Job Objects).
func applySandboxToCmd(cmd *exec.Cmd, sandbox *SandboxConfig) error {
	return nil
}
