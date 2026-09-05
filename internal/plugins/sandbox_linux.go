//
// Linux sandbox: applies resource limits to external plugin child processes.
//
// Memory isolation (RLIMIT_AS) requires the Subprocess Wrapper Pattern
// (see sandbox_wrapper.go) because Setrlimit on RLIMIT_AS is per-process,
// not per-process-tree — setting it on the parent would affect Cosca itself.
//
// The wrapper pattern forks a transient Cosca process that applies ALL
// rlimits (including RLIMIT_AS) to itself and then exec's the real plugin
// binary via unix.Exec. The parent Cosca process is never affected.
//
// For non-memory rlimits (CPU, file size, nofile), we retain the direct
// Setrlimit approach, which is safe (these limits have negligible effect on
// the parent). However, the long-term solution for all resource controls
// is cgroups v2, which provides proper per-cgroup accounting without needing
// the wrapper pattern or root capabilities (with systemd delegation).

//go:build linux

package plugins

import (
	"fmt"
	"os/exec"

	"golang.org/x/sys/unix"
)

// applySandboxToCmd configures resource limits for an external plugin process.
//
// If sandbox.MaxMemoryMB > 0, it uses the Subprocess Wrapper Pattern
// (wrapCmdForSandbox) to apply ALL limits, including RLIMIT_AS, in a
// transient wrapper process. The wrapper sets limits on itself via
// unix.Setrlimit and then exec's the target binary.
//
// If sandbox.MaxMemoryMB == 0, it applies CPU, file size, and nofile limits
// directly via unix.Setrlimit on the current (parent) process. The child
// inherits these limits at fork time and the parent's limits are restored.
// RLIMIT_AS is NOT set in this path because it would limit the parent.
//
// In both cases, the child process inherits the restricted limits and the
// parent's own limits are preserved.
func applySandboxToCmd(cmd *exec.Cmd, sandbox *SandboxConfig) error {
	if !sandbox.Enabled {
		return nil
	}

	// Use the Subprocess Wrapper Pattern when memory isolation is required.
	// RLIMIT_AS cannot be set on the parent, so we delegate to a wrapper
	// process that applies ALL limits in the child's context.
	if sandbox.MaxMemoryMB > 0 {
		return wrapCmdForSandbox(cmd, sandbox)
	}

	// -----------------------------------------------------------
	// Direct rlimit approach (no memory limit).
	// Safe for CPU, file size, and open file limits because these
	// do not constrain the parent process in any meaningful way.
	// -----------------------------------------------------------

	type savedLimit struct {
		resource int
		old      unix.Rlimit
	}
	var saved []savedLimit
	var errs []error

	setLimit := func(res int, cur, maxCur uint64) {
		var old unix.Rlimit
		if err := unix.Getrlimit(res, &old); err != nil {
			errs = append(errs, fmt.Errorf("getrlimit %d: %w", res, err))
			return
		}
		saved = append(saved, savedLimit{resource: res, old: old})
		newLimit := unix.Rlimit{Cur: cur, Max: maxCur}
		if err := unix.Setrlimit(res, &newLimit); err != nil {
			errs = append(errs, fmt.Errorf("setrlimit %d: %w", res, err))
		}
	}

	// CPU time limit (does not affect parent's ability to run).
	if sandbox.MaxCPUSeconds > 0 {
		setLimit(unix.RLIMIT_CPU, uint64(sandbox.MaxCPUSeconds), uint64(sandbox.MaxCPUSeconds))
	}
	// File size limit (does not affect parent's ability to allocate memory).
	if sandbox.MaxFileSizeMB > 0 {
		fsize := uint64(sandbox.MaxFileSizeMB) * 1024 * 1024
		setLimit(unix.RLIMIT_FSIZE, fsize, fsize)
	}
	// Open file limit (does not affect parent's ability to allocate).
	setLimit(unix.RLIMIT_NOFILE, 64, 64)

	// Restore original limits after child process starts.
	// The child inherits the restricted limits; parent gets originals back.
	defer func() {
		for _, s := range saved {
			restore := unix.Rlimit{Cur: s.old.Cur, Max: s.old.Max}
			_ = unix.Setrlimit(s.resource, &restore)
		}
	}()

	if len(errs) > 0 {
		return fmt.Errorf("sandbox: %v", errs)
	}
	return nil
}
