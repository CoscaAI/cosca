//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// ─── Jail memory limit (Linux-only: RLIMIT_AS) ───────────────────────────────

func TestSandboxMemoryLimitMBDefault(t *testing.T) {
	t.Setenv(envSandboxMemoryMB, "")
	require.Equal(t, int64(defaultSandboxMemoryMB), sandboxMemoryLimitMB())
}

func TestSandboxMemoryLimitMBFromEnv(t *testing.T) {
	t.Setenv(envSandboxMemoryMB, "4096")
	require.Equal(t, int64(4096), sandboxMemoryLimitMB())
}

func TestSandboxMemoryLimitMBRejectsInvalid(t *testing.T) {
	t.Setenv(envSandboxMemoryMB, "abc")
	require.Equal(t, int64(defaultSandboxMemoryMB), sandboxMemoryLimitMB())
	t.Setenv(envSandboxMemoryMB, "-5")
	require.Equal(t, int64(defaultSandboxMemoryMB), sandboxMemoryLimitMB())
	t.Setenv(envSandboxMemoryMB, "0")
	require.Equal(t, int64(defaultSandboxMemoryMB), sandboxMemoryLimitMB())
}

// TestApplyJailMemoryLimitSetsRlimit verifies the jail memory cap is applied as
// a real RLIMIT_AS/RLIMIT_DATA. The limit is applied in a SUBPROCESS so the
// test binary's own address space is never constrained — rlimits are
// process-scoped and leaking them into sibling tests would break them (e.g.
// spawning a command under a 512 MB cap). The subprocess reports the effective
// limits it observes after applyJailMemoryLimit.
//
// Under -race this assertion is skipped: the ThreadSanitizer needs a large
// virtual address space for shadow memory. The pure resolution logic is always
// covered by TestSandboxMemoryLimitMB*.
func TestApplyJailMemoryLimitSetsRlimit(t *testing.T) {
	if testingRace() {
		t.Skip("RLIMIT_AS under race detector blocks TSan shadow memory")
	}
	t.Setenv(envSandboxMemoryMB, "512")

	// Self-exec: the helper test binary runs the child mode which applies the
	// limit and prints the observed RLIMIT_AS/RLIMIT_DATA.
	if os.Getenv("JAIL_RLIMIT_CHILD") == "1" {
		applyJailMemoryLimit(newExecCmd("true"))
		var rl unix.Rlimit
		if err := unix.Getrlimit(unix.RLIMIT_AS, &rl); err != nil {
			fmt.Fprintf(os.Stderr, "getrlimit: %v\n", err)
			os.Exit(2)
		}
		fmt.Printf("%d %d\n", rl.Cur, rl.Max)
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestApplyJailMemoryLimitSetsRlimit")
	cmd.Env = append(os.Environ(), "JAIL_RLIMIT_CHILD=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "child failed: %s", string(out))

	var cur, max uint64
	_, serr := fmt.Sscanf(string(out), "%d %d", &cur, &max)
	require.NoError(t, serr)
	require.Equal(t, uint64(512*1024*1024), cur)
	require.Equal(t, uint64(512*1024*1024), max)
}

// newExecCmd builds a minimal exec.Cmd for limit tests.
func newExecCmd(name string) *exec.Cmd {
	return exec.Command(name)
}

// ─── bwrap-specific behavior (Linux-only) ────────────────────────────────────

func TestGate_RejectsAnyWorkspaceSymlink(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "workspace")
	require.NoError(t, os.Symlink(target, link))
	g := NewGate(link, chat.SandboxWorkspace)
	_, err := g.bwrapCommand(context.Background(), chat.Command{Args: []string{"true"}}, chat.SandboxWorkspace)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlinked workspace")
}

func TestGate_BwrapArgsDoNotBindHostRoot(t *testing.T) {
	g := &Gate{workspace: "/workspace", bwrapPath: "/usr/bin/bwrap"}
	proc, err := g.bwrapCommand(context.Background(), chat.Command{Args: []string{"true"}}, chat.SandboxWorkspace)
	require.NoError(t, err)
	for i, arg := range proc.Args {
		if arg == "--ro-bind" && i+2 < len(proc.Args) && proc.Args[i+1] == "/" && proc.Args[i+2] == "/" {
			t.Fatal("bwrap command must not bind the host root")
		}
	}
	assert.Contains(t, proc.Args, "/usr")
	assert.Contains(t, proc.Args, "--clearenv")
	for i, arg := range proc.Args {
		if arg == "/run" {
			t.Fatalf("sandbox must not expose host /run at arg %d", i)
		}
	}
}

func TestGate_RejectsWorkspaceResolvingToRoot(t *testing.T) {
	g := NewGate("/", chat.SandboxWorkspace)
	_, err := g.bwrapCommand(context.Background(), chat.Command{Args: []string{"true"}}, chat.SandboxWorkspace)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filesystem root")
}

func TestGate_EmbedReadOnlyInWorkspaceMode(t *testing.T) {
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "internal", "embed", "cosca"), 0o755))

	g := &Gate{workspace: ws, bwrapPath: "/usr/bin/bwrap"}
	proc, err := g.bwrapCommand(context.Background(), chat.Command{Args: []string{"true"}}, chat.SandboxWorkspace)
	require.NoError(t, err)

	embed := filepath.Join(ws, "internal", "embed", "cosca")
	bindIdx, roBindIdx := -1, -1
	for i, arg := range proc.Args {
		if arg == "--bind" && i+1 < len(proc.Args) && proc.Args[i+1] == ws {
			bindIdx = i
		}
		if arg == "--ro-bind" && i+1 < len(proc.Args) && proc.Args[i+1] == embed {
			roBindIdx = i
		}
	}
	require.Greater(t, bindIdx, -1, "workspace --bind deve existir no modo gravável")
	require.Greater(t, roBindIdx, -1, "embed --ro-bind deve existir no modo gravável")
	assert.Greater(t, roBindIdx, bindIdx, "o --ro-bind do embed deve vir DEPOIS do --bind do workspace (para sobrescrever)")
}

func TestGate_RejectsSymlinkWorkspaceResolvingToRoot(t *testing.T) {
	link := filepath.Join(t.TempDir(), "workspace")
	require.NoError(t, os.Symlink("/", link))
	g := NewGate(link, chat.SandboxWorkspace)
	_, err := g.bwrapCommand(context.Background(), chat.Command{Args: []string{"true"}}, chat.SandboxWorkspace)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filesystem root")
}
