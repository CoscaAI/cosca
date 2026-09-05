//go:build linux || darwin

// Package hardening hardens the Cosca process at startup.
package hardening

import "golang.org/x/sys/unix"

// applyPlatformHardening applies Linux/macOS process hardening. Every call is
// best-effort: syscalls that require privileges the process does not have are
// ignored rather than fatal.
func applyPlatformHardening() {
	// 1. Disable core dumps via RLIMIT_CORE = 0. Setting both the soft and
	// hard limit to zero makes the reduction irreversible for this process.
	var rl unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_CORE, &rl); err == nil {
		rl.Cur = 0
		rl.Max = 0
		_ = unix.Setrlimit(unix.RLIMIT_CORE, &rl)
	}

	// 2. Disable ptrace attach (and, as a side effect, core dumps) via
	// PR_SET_DUMPABLE = 0: no other process may ptrace this one or read its
	// memory, and the kernel will not produce a core dump for it.
	_ = unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)

	// 3. Prevent privilege escalation via exec. PR_SET_NO_NEW_PRIVS blocks
	// setuid/setgid and file-capability escalation for this process and every
	// child it spawns. This is inherited across fork/exec and cannot be undone.
	_ = unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
}
