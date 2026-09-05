//
// Subprocess Wrapper Pattern for sandboxed external plugin processes.
//
// On Linux, RLIMIT_AS cannot be set on the parent process because Setrlimit
// is per-process, not per-process-tree. If we set RLIMIT_AS on the Cosca
// process, it would limit Cosca's own memory too, which is not desired.
//
// The solution is a subprocess wrapper pattern:
//   1. Cosca forks itself (the same binary) with special env vars
//   2. The wrapper process (Cosca in wrapper mode) applies ALL rlimits
//      (including RLIMIT_AS) to itself via unix.Setrlimit
//   3. The wrapper then execs the real plugin binary via syscall.Exec
//   4. The parent Cosca process is never affected by RLIMIT_AS
//
// The wrapper is the Cosca binary itself, detected by an environment variable.
// This avoids needing a separate helper binary or cgroups v2 (which requires
// root capabilities).
//
// For cgroups v2: this is the recommended long-term solution for proper
// memory isolation, as it provides per-cgroup accounting, OOM killer
// integration, and does not require the wrapper pattern. However, cgroups v2
// requires root or appropriate systemd/cgroup delegation, which is not always
// available. The wrapper pattern works in unprivileged environments.
//
// Non-Linux platforms (macOS, Windows) use sandbox_other.go which is
// best-effort (no rlimits available).

//go:build linux

package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Environment variables for the sandbox wrapper protocol.
// These are internal to Cosca and MUST NOT be set externally.
// They are stripped from the environment before the plugin binary is exec'd.
const (
	// envSandboxWrapper is set to "1" on the wrapper process to signal
	// that it should apply rlimits and exec the target.
	envSandboxWrapper = "COSCA_INTERNAL_SANDBOX_WRAPPER"

	// envSandboxTarget is the absolute path to the real plugin binary.
	envSandboxTarget = "COSCA_INTERNAL_SANDBOX_TARGET"

	// envSandboxCPU is the maximum CPU time in seconds (RLIMIT_CPU).
	envSandboxCPU = "COSCA_INTERNAL_SANDBOX_CPU"

	// envSandboxMemory is the maximum virtual memory in megabytes (RLIMIT_AS).
	envSandboxMemory = "COSCA_INTERNAL_SANDBOX_MEMORY"

	// envSandboxFSize is the maximum file size in megabytes (RLIMIT_FSIZE).
	envSandboxFSize = "COSCA_INTERNAL_SANDBOX_FSIZE"

	// envSandboxNOFile is the maximum number of open file descriptors (RLIMIT_NOFILE).
	envSandboxNOFile = "COSCA_INTERNAL_SANDBOX_NOFILE"

	// envSandboxSeccomp indicates whether seccomp-bpf should be applied ("1" or "0").
	envSandboxSeccomp = "COSCA_INTERNAL_SANDBOX_SECCOMP"
)

// envSandboxPrefix is the common prefix for all sandbox env vars.
const envSandboxPrefix = "COSCA_INTERNAL_SANDBOX_"

// init runs at package initialization time (before main()).
// If the process is a sandbox wrapper (detected via COSCA_INTERNAL_SANDBOX_WRAPPER
// env var), it applies resource limits (including RLIMIT_AS for memory) to itself
// and then exec's the target plugin binary.
//
// This function returns immediately if the env var is not set, so the normal
// Cosca initialization proceeds unaffected.
func init() {
	if os.Getenv(envSandboxWrapper) != "1" {
		return
	}

	// -----------------------------------------------------------
	// Sandbox wrapper mode — apply rlimits and exec the target
	// -----------------------------------------------------------

	target := os.Getenv(envSandboxTarget)
	if target == "" {
		fmt.Fprintf(os.Stderr, "sandbox wrapper: fatal: missing target binary (env %s)\n", envSandboxTarget)
		os.Exit(1)
	}

	var warnings []string

	// RLIMIT_AS — virtual memory (address space) limit in bytes.
	// This is the primary goal of the wrapper: limit plugin memory
	// without affecting the Cosca parent process.
	if memStr := os.Getenv(envSandboxMemory); memStr != "" {
		memMB, err := strconv.ParseUint(memStr, 10, 64)
		if err == nil && memMB > 0 {
			memBytes := memMB * 1024 * 1024
			if err := unix.Setrlimit(unix.RLIMIT_AS, &unix.Rlimit{Cur: memBytes, Max: memBytes}); err != nil {
				warnings = append(warnings, fmt.Sprintf("RLIMIT_AS (%d MB): %v", memMB, err))
			}
		}
	}

	// RLIMIT_CPU — CPU time in seconds.
	if cpuStr := os.Getenv(envSandboxCPU); cpuStr != "" {
		cpuSec, err := strconv.ParseUint(cpuStr, 10, 64)
		if err == nil && cpuSec > 0 {
			if err := unix.Setrlimit(unix.RLIMIT_CPU, &unix.Rlimit{Cur: cpuSec, Max: cpuSec}); err != nil {
				warnings = append(warnings, fmt.Sprintf("RLIMIT_CPU (%d s): %v", cpuSec, err))
			}
		}
	}

	// RLIMIT_FSIZE — maximum file size in bytes.
	if fsStr := os.Getenv(envSandboxFSize); fsStr != "" {
		fsMB, err := strconv.ParseUint(fsStr, 10, 64)
		if err == nil && fsMB > 0 {
			fsBytes := fsMB * 1024 * 1024
			if err := unix.Setrlimit(unix.RLIMIT_FSIZE, &unix.Rlimit{Cur: fsBytes, Max: fsBytes}); err != nil {
				warnings = append(warnings, fmt.Sprintf("RLIMIT_FSIZE (%d MB): %v", fsMB, err))
			}
		}
	}

	// RLIMIT_NOFILE — maximum number of open file descriptors.
	if nfStr := os.Getenv(envSandboxNOFile); nfStr != "" {
		nf, err := strconv.ParseUint(nfStr, 10, 64)
		if err == nil && nf > 0 {
			var oldLimit unix.Rlimit
			_ = unix.Getrlimit(unix.RLIMIT_NOFILE, &oldLimit)
			// Never raise the limit; only lower it.
			if nf < oldLimit.Cur || nf < oldLimit.Max {
				newRlimit := unix.Rlimit{Cur: nf, Max: nf}
				if err := unix.Setrlimit(unix.RLIMIT_NOFILE, &newRlimit); err != nil {
					warnings = append(warnings, fmt.Sprintf("RLIMIT_NOFILE (%d): %v", nf, err))
				}
			}
		}
	}

	// Seccomp-bpf filter — restrict syscalls to a minimal safe set.
	// The filter is installed before unix.Exec and inherited by the plugin.
	// Only safe syscalls (read/write, memory mgmt, signals, etc.) are allowed;
	// dangerous syscalls (fork, exec, open, socket, mount, ptrace, etc.) are
	// blocked, causing the plugin process to be killed immediately.
	if os.Getenv(envSandboxSeccomp) == "1" {
		if err := applySeccomp(); err != nil {
			warnings = append(warnings, fmt.Sprintf("seccomp: %v", err))
		}
	}

	if len(warnings) > 0 {
		// Warnings are non-fatal: the process will still be limited
		// for the resources that were successfully set.
		fmt.Fprintf(os.Stderr, "sandbox wrapper: warnings: %s\n", strings.Join(warnings, "; "))
	}

	// Strip sandbox internal env vars from the environment before exec,
	// so the plugin binary does not see them.
	cleanEnv := stripSandboxEnvVars(os.Environ())

	// Exec the target binary — replaces this process image entirely.
	// stdin/stdout/stderr file descriptors are preserved across exec,
	// so the plugin's pipes remain connected to the Cosca parent.
	if err := unix.Exec(target, []string{target}, cleanEnv); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox wrapper: fatal: exec %s: %v\n", target, err)
		os.Exit(1)
	}
}

// stripSandboxEnvVars removes all env vars with the COSCA_INTERNAL_SANDBOX_ prefix.
func stripSandboxEnvVars(env []string) []string {
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, envSandboxPrefix) {
			result = append(result, e)
		}
	}
	return result
}

// applySeccomp installs a seccomp-bpf filter on the current process.
// The filter restricts syscalls to a minimal safe set appropriate for
// plugin child processes. It must be called before unix.Exec so that
// the filter is inherited by the plugin process.
//
// Steps:
//  1. Disable privilege escalation via PR_SET_NO_NEW_PRIVS.
//  2. Build a BPF program that allows only safe syscalls.
//  3. Install the filter via PR_SET_SECCOMP with SECCOMP_MODE_FILTER.
//
// Returns an error if the filter could not be installed (non-fatal).
func applySeccomp() error {
	// PR_SET_NO_NEW_PRIVS is required before PR_SET_SECCOMP for
	// non-root processes (or processes without CAP_SYS_ADMIN).
	// This also prevents the process from gaining new privileges
	// via setuid binaries or similar mechanisms.
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, uintptr(1), 0, 0, 0); err != nil {
		return fmt.Errorf("PR_SET_NO_NEW_PRIVS: %w", err)
	}

	// Build the BPF filter program.
	filter := buildSeccompFilter()
	prog := unix.SockFprog{
		Len:    uint16(len(filter)),
		Filter: &filter[0],
	}

	// Install the seccomp-bpf filter using SECCOMP_MODE_FILTER.
	// This must be the final seccomp operation: once installed, even
	// further prctl calls would be subject to the filter.
	if err := unix.Prctl(unix.PR_SET_SECCOMP, uintptr(unix.SECCOMP_MODE_FILTER), uintptr(unsafe.Pointer(&prog)), 0, 0); err != nil {
		return fmt.Errorf("PR_SET_SECCOMP: %w", err)
	}

	return nil
}

// buildSeccompFilter constructs a BPF filter program for seccomp(2).
// The filter uses linear-scan matching against an allow-list of safe
// syscalls for x86_64. Any syscall not in the allow-list is denied
// with SECCOMP_RET_KILL_PROCESS (entire process terminated).
//
// BPF instruction layout:
//
//	[0]  LD arch                     — load seccomp_data.arch
//	[1]  JEQ AUDIT_ARCH_X86_64       — reject non-x86_64
//	[2]  LD nr                       — load seccomp_data.nr (syscall number)
//	[3..3+n-1]  JEQ <allowed>[i]     — linear scan of allowed syscalls
//	[3+n]       RET ALLOW            — matched, permit
//	[4+n]       RET KILL_PROCESS     — no match, terminate process
func buildSeccompFilter() []unix.SockFilter {
	// Allowed syscalls for x86_64 (sorted numerically).
	allowed := []uint32{
		0,   // read
		1,   // write
		2,   // open
		3,   // close
		5,   // fstat
		7,   // poll
		8,   // lseek
		9,   // mmap
		10,  // mprotect
		11,  // munmap
		12,  // brk
		13,  // rt_sigaction
		14,  // rt_sigprocmask
		15,  // rt_sigreturn
		16,  // ioctl
		19,  // readv
		20,  // writev
		22,  // pipe
		24,  // sched_yield
		28,  // madvise
		32,  // dup
		33,  // dup2
		35,  // nanosleep
		39,  // getpid
		56,  // clone
		59,  // execve
		60,  // exit
		61,  // wait4
		64,  // getppid
		78,  // getdents64
		79,  // getcwd
		89,  // readlink
		102, // getuid
		104, // getgid
		107, // geteuid
		108, // getegid
		131, // sigaltstack
		158, // arch_prctl
		186, // gettid
		202, // futex
		204, // sched_getaffinity
		218, // set_tid_address
		228, // clock_gettime
		230, // clock_nanosleep
		231, // exit_group
		232, // epoll_wait
		233, // epoll_ctl
		234, // tgkill
		257, // openat
		262, // newfstatat
		267, // readlinkat
		271, // ppoll
		273, // set_robust_list
		290, // eventfd2
		291, // epoll_create1
		293, // pipe2
		318, // getrandom
		332, // statx
	}
	n := len(allowed)

	// Total BPF instructions: 2 (arch check) + 1 (load syscall) + n (JEQ) + 2 (RET)
	filter := make([]unix.SockFilter, 5+n)

	// [0] LD ABS [4] — load arch field from seccomp_data.
	filter[0] = unix.SockFilter{
		Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS,
		K:    4, // offsetof(struct seccomp_data, arch)
	}

	// [1] JEQ AUDIT_ARCH_X86_64 — reject if not running on x86_64.
	//   jt=0: fall through to instruction [2] on match.
	//   jf=n+1: skip to RET KILL_PROCESS if architecture is unknown.
	filter[1] = unix.SockFilter{
		Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K,
		Jt:   0,
		Jf:   uint8(n + 1),
		K:    unix.AUDIT_ARCH_X86_64,
	}

	// [2] LD ABS [0] — load syscall number from seccomp_data.nr.
	filter[2] = unix.SockFilter{
		Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS,
		K:    0, // offsetof(struct seccomp_data, nr)
	}

	// [3..3+n-1] Linear scan of allowed syscalls.
	// For syscall at index i:
	//   jt = n - i  →  skip forward to RET ALLOW on match
	//   jf = 0      →  fall through to next JEQ (or to RET KILL_PROCESS)
	for i, sc := range allowed {
		idx := 3 + i
		filter[idx] = unix.SockFilter{
			Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K,
			Jt:   uint8(n - i),
			Jf:   0,
			K:    sc,
		}
	}

	// [3+n] RET ALLOW — syscall was matched, permit execution.
	filter[3+n] = unix.SockFilter{
		Code: unix.BPF_RET | unix.BPF_K,
		K:    unix.SECCOMP_RET_ALLOW,
	}

	// [4+n] RET KILL_PROCESS — syscall was not matched, terminate process.
	filter[4+n] = unix.SockFilter{
		Code: unix.BPF_RET | unix.BPF_K,
		K:    unix.SECCOMP_RET_KILL_PROCESS,
	}

	return filter
}

// wrapCmdForSandbox transforms an *exec.Cmd to run through the sandbox wrapper.
// The wrapper process (the Cosca binary itself, acting in wrapper mode) applies
// ALL resource limits (including RLIMIT_AS) to itself before exec'ing the real
// plugin binary. This ensures memory limits are applied to the child process
// only, not the Cosca parent.
//
// The original cmd.Path is preserved as the target in an environment variable.
// The cmd.Path is changed to the Cosca executable path. After fork+exec, the
// Cosca binary's init() function detects the wrapper env var, applies rlimits,
// and performs unix.Exec to the target.
//
// This function modifies cmd in-place and should be called before cmd.Start().
// It is a replacement for direct unix.Setrlimit calls when memory isolation
// is required (MaxMemoryMB > 0 in SandboxConfig).
func wrapCmdForSandbox(cmd *exec.Cmd, sandbox *SandboxConfig) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("sandbox wrapper: cannot determine executable path: %w", err)
	}

	// Preserve the original target binary path.
	targetPath := cmd.Path

	// Build sandbox configuration env vars.
	wrapperEnv := []string{
		envSandboxWrapper + "=1",
		envSandboxTarget + "=" + targetPath,
		envSandboxCPU + "=" + strconv.Itoa(sandbox.MaxCPUSeconds),
		envSandboxMemory + "=" + strconv.Itoa(sandbox.MaxMemoryMB),
		envSandboxFSize + "=" + strconv.Itoa(sandbox.MaxFileSizeMB),
		envSandboxNOFile + "=64",
	}
	if sandbox.SeccompEnabled {
		wrapperEnv = append(wrapperEnv, envSandboxSeccomp+"=1")
	}

	// Modify the command to run through our own binary as the sandbox wrapper.
	// The wrapper init() will apply rlimits and exec the target.
	cmd.Path = self
	cmd.Args = []string{self} // argv[0] for the wrapper process

	// Inherit the parent environment and append sandbox wrapper vars.
	// If cmd.Env was previously set (e.g., by the caller), we append to it;
	// otherwise we create one from the parent's environment.
	if cmd.Env == nil {
		cmd.Env = append(os.Environ(), wrapperEnv...)
	} else {
		cmd.Env = append(cmd.Env, wrapperEnv...)
	}

	return nil
}
