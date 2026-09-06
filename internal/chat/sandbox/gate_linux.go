//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/processutil"
	"golang.org/x/sys/unix"
)

// nativeSandboxAvailable reports whether the platform has a non-bwrap native
// sandbox backend. On Linux, bwrap IS the native backend — the bwrap path is
// selected by findBwrap(), so this reports false (never shadows bwrap).
func nativeSandboxAvailable() bool { return false }

// execNative satisfaz a referencia de tipo no gate.go:121 (Executar) para o
// build Linux. Nunca e chamado em runtime — findBwrap() ja cobre o caminho
// Linux e nativeSandboxAvailable() retorna false — mas o Go exige o metodo no
// tipo. Estubo que retorna erro para nunca esconder uma execucao nao-isolada.
func (g *Gate) execNative(ctx context.Context, _ chat.Command, _ chat.SandboxMode) (*chat.SandboxResult, error) {
	return nil, fmt.Errorf("sandbox: native sandbox backend not available on Linux")
}

// envSandboxMemoryMB controls the maximum virtual address space (RLIMIT_AS)
// and data segment (RLIMIT_DATA) granted to processes inside the bwrap jail,
// in megabytes. Default 6144 (6 GiB): enough for real Go builds/tests inside
// the jail (pico ~1.9 GB for a typical `go run`, ~7.4 GB for an abusive
// 2 GB-heap process), while capping runaway memory so the jail can never
// drain the host's swap. Override per-machine without recompiling.
const envSandboxMemoryMB = "COSCA_SANDBOX_MEMORY_MB"

// defaultSandboxMemoryMB is the default jail memory cap (6 GiB).
const defaultSandboxMemoryMB = 6144

// sandboxMemoryLimitMB resolves the jail memory cap from the environment.
func sandboxMemoryLimitMB() int64 {
	raw := os.Getenv(envSandboxMemoryMB)
	if raw == "" {
		return defaultSandboxMemoryMB
	}
	mb, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || mb <= 0 {
		return defaultSandboxMemoryMB
	}
	return mb
}

// applyJailMemoryLimit sets RLIMIT_AS and RLIMIT_DATA on the bwrap process so
// everything spawned inside the jail (tools, compilers, tests) is bounded to
// sandboxMemoryLimitMB of virtual memory. RLIMIT_AS caps the whole address
// space; RLIMIT_DATA caps the heap/data growth — together they stop a runaway
// process from paging the host into swap. The kernel enforces the limit by
// killing the process (SIGKILL/SIGSEGV) on breach; the sandbox boundary keeps
// the host unaffected.
func applyJailMemoryLimit(proc *exec.Cmd) {
	limit := sandboxMemoryLimitMB() * 1024 * 1024
	rl := unix.Rlimit{Cur: uint64(limit), Max: uint64(limit)}
	// The rlimit is set on the bwrap launcher itself; bwrap preserves it
	// across exec into the sandboxed command (bwrap uses exec, not fork).
	if err := unix.Setrlimit(unix.RLIMIT_AS, &rl); err != nil {
		// Failing to apply a memory cap must not silently weaken the jail.
		fmt.Fprintf(os.Stderr, "sandbox: warning: could not apply RLIMIT_AS (%d MB): %v\n", sandboxMemoryLimitMB(), err)
	}
	if err := unix.Setrlimit(unix.RLIMIT_DATA, &rl); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox: warning: could not apply RLIMIT_DATA (%d MB): %v\n", sandboxMemoryLimitMB(), err)
	}
}

// applyJailResourceLimits endurece o runtime (Runtime Sentinel) com limites
// de recursos que impedem exaustão de processos (fork bomb), disco e file
// descriptors. O agente nunca transforma proposta em execução sem passar por
// aqui — e aqui, recurso é limitado.
func applyJailResourceLimits(proc *exec.Cmd) {
	applyJailMemoryLimit(proc)

	set := func(what int, val uint64, name string) {
		rl := unix.Rlimit{Cur: val, Max: val}
		if err := unix.Setrlimit(what, &rl); err != nil {
			fmt.Fprintf(os.Stderr, "sandbox: warning: could not apply %s (%d): %v\n", name, val, err)
		}
	}
	// FSIZE: tamanho máximo de arquivo escrito — impede disk exhaustion.
	set(unix.RLIMIT_FSIZE, 4*1024*1024*1024, "RLIMIT_FSIZE")
	// NOFILE: file descriptors — impede FD exhaustion.
	set(unix.RLIMIT_NOFILE, 4096, "RLIMIT_NOFILE")
	// NOTA: RLIMIT_NPROC é por-UID (não por-processo) — setar aqui limitaria
	// TODOS os processos do usuário, não só a jaula. Fork bomb dentro da jaula
	// é contido pelo PID namespace do --unshare-all + cgroups (se presentes).
}

// findBwrap looks for the bwrap binary on the system.
// Returns the full path to bwrap, or empty string if not found.
func findBwrap() string {
	path, err := exec.LookPath("bwrap")
	if err != nil {
		return ""
	}
	path, err = filepath.Abs(path)
	if err != nil || !filepath.IsAbs(path) {
		return ""
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(path) {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 || info.Mode()&0022 != 0 {
		return ""
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || (st.Uid != uint32(os.Geteuid()) && st.Uid != 0) {
		return ""
	}
	return path
}

// execBwrap runs a command inside a bubblewrap sandbox.
// The mode determines workspace mount behaviour:
//   - SandboxReadOnly:  workspace is mounted --ro-bind (read-only)
//   - SandboxWorkspace: workspace is mounted --bind (writable)
//
// Full namespace isolation (PID, UTS, network, mount) is applied.
// The host filesystem is mounted read-only except for the workspace.
func (g *Gate) execBwrap(ctx context.Context, cmd chat.Command, mode chat.SandboxMode) (*chat.SandboxResult, error) {
	if len(cmd.Args) == 0 {
		return nil, fmt.Errorf("sandbox: empty command arguments")
	}

	execCmd, err := g.bwrapCommand(ctx, cmd, mode)
	if err != nil {
		return nil, err
	}

	// bwrap needs access to /dev/null for its own internal setup; ensure
	// stdin is wired up properly.
	execCmd.Stdin = os.Stdin

	// Run through the shared executor (streaming capture, idle detection,
	// hard runtime cap, whole-tree termination).
	res, err := processutil.Run(ctx, execCmd, processutil.Config{
		IdleTimeout: cmd.IdleTimeout,
		MaxRuntime:  cmd.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("sandbox: bwrap execution failed: %w", err)
	}

	// Preserve the sandbox-establishment diagnostic: a bwrap exit before/while
	// establishing namespaces is not a normal command result. Returning it as a
	// result would silently turn a failed jail into an unsandboxed invocation.
	// An interruption (idle/hard timeout or cancellation) is reported as such.
	switch res.Status {
	case processutil.StatusSuccess:
		// clean exit — fall through to result construction
	case processutil.StatusIdleTimeout, processutil.StatusHardTimeout, processutil.StatusCancelled:
		return nil, fmt.Errorf("sandbox: bwrap execution interrupted (%s)", string(res.Status))
	default:
		return nil, fmt.Errorf("sandbox: bwrap failed to establish sandbox (exit %d): %s", res.ExitCode, sanitizeSandboxDiagnostic(res.Stderr, cmd.Env))
	}

	return &chat.SandboxResult{
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
		ExitCode: res.ExitCode,
		Duration: res.Duration,
		Status:   string(res.Status),
		IdleFor:  res.IdleFor,
	}, nil
}

func (g *Gate) bwrapCommand(ctx context.Context, cmd chat.Command, mode chat.SandboxMode) (*exec.Cmd, error) {
	if err := validateEnv(cmd.Env); err != nil {
		return nil, err
	}
	if err := g.validateWorkspace(); err != nil {
		return nil, err
	}
	// clearenv prevents ambient credentials (including API keys) entering the jail.
	args := []string{g.bwrapPath, "--unshare-all", "--die-with-parent", "--clearenv", "--ro-bind", "/usr", "/usr", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64", "--ro-bind", "/etc", "/etc", "--tmpfs", "/tmp", "--dev", "/dev", "--proc", "/proc"}
	if mode == chat.SandboxReadOnly {
		args = append(args, "--ro-bind", g.workspace, g.workspace)
	} else {
		args = append(args, "--bind", g.workspace, g.workspace)
		// A memória do kernel (embed) é SEMPRE read-only, mesmo no modo
		// workspace gravável: agente preso não escreve no cérebro do Don.
		// O --ro-bind posterior sobrescreve o --bind gravável no mesmo path.
		embedDir := filepath.Join(g.workspace, "internal", "embed", "cosca")
		if _, err := os.Stat(embedDir); err == nil {
			args = append(args, "--ro-bind", embedDir, embedDir)
		}
	}
	workDir := cmd.WorkDir
	if workDir == "" {
		workDir = g.workspace
	}
	args = append(args, "--chdir", workDir)
	args = append(args, "--setenv", "PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "--setenv", "HOME", g.workspace)
	for k, v := range cmd.Env {
		args = append(args, "--setenv", k, v)
	}
	args = append(args, cmd.Args...)
	proc := exec.CommandContext(ctx, args[0], args[1:]...)
	// bwrap uses --clearenv and receives only explicit --setenv values. Keep
	// Cmd.Env non-nil as an additional defense against ambient inheritance.
	proc.Env = safeEnv(nil)
	proc.Stdin = os.Stdin
	// Reserve the jail budget: memory + processos + disco + FDs (Runtime
	// Sentinel) para nada dentro da jaula exaurir o host.
	applyJailResourceLimits(proc)
	return proc, nil
}

func sanitizeSandboxDiagnostic(s string, env map[string]string) string {
	for _, value := range env {
		if len(value) >= 4 {
			s = strings.ReplaceAll(s, value, "[REDACTED]")
		}
	}
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || (r >= 0x20 && r != 0x7f) {
			return r
		}
		return ' '
	}, s)
	s = strings.TrimSpace(s)
	if len(s) > 1024 {
		s = s[:1024] + "..."
	}
	return s
}
