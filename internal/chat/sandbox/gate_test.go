// Package sandbox provides tests for the Gate sandbox, covering construction,
// mode reporting, path validation, and command execution in direct mode.
package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── NewGate ─────────────────────────────────────────────────────────────────

func TestNewGate(t *testing.T) {
	t.Parallel()

	t.Run("creates gate with absolute workspace path", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// Semântica de path absoluto POSIX: "/tmp/../tmp/workspace" não é
			// um path válido no Windows (falta o drive) e filepath.Abs resolve
			// para C:\tmp\workspace em vez de /tmp/workspace.
			t.Skip("semântica de path absoluto POSIX — não aplicável no Windows")
		}
		g := NewGate("/tmp/../tmp/workspace", chat.SandboxWorkspace)
		assert.Equal(t, "/tmp/workspace", g.workspace)
		assert.Equal(t, chat.SandboxWorkspace, g.mode)
	})

	t.Run("resolves relative workspace to absolute", func(t *testing.T) {
		// Use a real directory so Abs resolves correctly.
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxReadOnly)
		assert.True(t, filepath.IsAbs(g.workspace))
		assert.Equal(t, chat.SandboxReadOnly, g.mode)
	})

	t.Run("creates gate with full mode", func(t *testing.T) {
		g := NewGate("/workspace", chat.SandboxFull)
		assert.Equal(t, chat.SandboxFull, g.mode)
	})

	t.Run("creates gate with default mode", func(t *testing.T) {
		g := NewGate("/workspace", chat.SandboxWorkspace)
		assert.Equal(t, chat.SandboxWorkspace, g.mode)
	})
}

// ─── Mode ────────────────────────────────────────────────────────────────────

func TestGate_Mode(t *testing.T) {
	t.Parallel()

	t.Run("returns the sandbox mode", func(t *testing.T) {
		g := NewGate("/ws", chat.SandboxReadOnly)
		assert.Equal(t, chat.SandboxReadOnly, g.Mode())
	})

	t.Run("returns workspace mode", func(t *testing.T) {
		g := NewGate("/ws", chat.SandboxWorkspace)
		assert.Equal(t, chat.SandboxWorkspace, g.Mode())
	})
}

// ─── ValidatePath ────────────────────────────────────────────────────────────

func TestGate_ValidatePath(t *testing.T) {
	t.Parallel()

	t.Run("valid path in workspace passes", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxWorkspace)
		err := g.ValidatePath(".")
		assert.NoError(t, err)
	})

	t.Run("path outside workspace fails", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// "/etc/passwd" não é um path absoluto no Windows (falta o drive),
			// então o Rails resolve contra o workspace e o escape não é
			// detectado — semântica de path absoluto POSIX.
			t.Skip("semântica de path absoluto POSIX — não aplicável no Windows")
		}
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxWorkspace)
		err := g.ValidatePath("/etc/passwd")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes workspace")
	})

	t.Run("path traversal outside workspace fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxWorkspace)
		err := g.ValidatePath("../../etc/passwd")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes workspace")
	})

	t.Run("blocked directory path fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755)
		g := NewGate(tmpDir, chat.SandboxWorkspace)
		err := g.ValidatePath(".git/HEAD")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "blocked directory")
	})
}

// ─── Execute (direct mode) ───────────────────────────────────────────────────

func TestGate_Execute_DirectMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Os subtests usam comandos POSIX (echo, pwd, sh -c) que não existem
		// como executáveis no Windows (echo/pwd são builtins do cmd.exe e o
		// shell POSIX /bin/sh não está disponível por padrão).
		t.Skip("comandos POSIX (echo/pwd/sh) inexistentes no Windows")
	}
	t.Setenv("COSCA_ALLOW_NO_ROOT", "1")

	t.Run("executes simple command", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxFull)

		result, err := g.Execute(context.Background(), chat.Command{
			Args: []string{"echo", "hello sandbox"},
		}, chat.SandboxFull)
		require.NoError(t, err)
		assert.Equal(t, "hello sandbox\n", result.Stdout)
		assert.Equal(t, 0, result.ExitCode)
		assert.Greater(t, result.Duration, time.Duration(0))
	})

	t.Run("executes command with working directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxFull)

		result, err := g.Execute(context.Background(), chat.Command{
			Args:    []string{"pwd"},
			WorkDir: tmpDir,
		}, chat.SandboxFull)
		require.NoError(t, err)
		assert.Contains(t, result.Stdout, tmpDir)
		assert.Equal(t, 0, result.ExitCode)
	})

	t.Run("captures stderr", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxFull)

		result, err := g.Execute(context.Background(), chat.Command{
			Args: []string{"sh", "-c", "echo error >&2"},
		}, chat.SandboxFull)
		require.NoError(t, err)
		assert.Contains(t, result.Stderr, "error")
	})

	t.Run("captures non-zero exit code", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxFull)

		result, err := g.Execute(context.Background(), chat.Command{
			Args: []string{"sh", "-c", "exit 42"},
		}, chat.SandboxFull)
		require.NoError(t, err)
		assert.Equal(t, 42, result.ExitCode)
	})
}

// ─── Execute: empty args ─────────────────────────────────────────────────────

func TestGate_Execute_EmptyArgs(t *testing.T) {
	t.Parallel()

	t.Run("returns error for empty args in direct mode", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxFull)

		_, err := g.Execute(context.Background(), chat.Command{}, chat.SandboxFull)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty command arguments")
	})

	t.Run("returns error for empty args in read-only mode", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// No Windows o bwrap nunca está disponível, então sem
			// COSCA_ALLOW_NO_ROOT=1 o Gate falha-fechado com "bubblewrap is
			// required" ANTES da validação de args vazios (que só acontece no
			// caminho de execução direta). O subteste de modo direto acima
			// cobre a validação de args vazios nesta plataforma.
			t.Skip("sem bwrap no Windows o erro fail-closed precede a validação de args vazios")
		}
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxReadOnly)

		_, err := g.Execute(context.Background(), chat.Command{}, chat.SandboxReadOnly)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty command arguments")
	})

	t.Run("returns error for empty args in workspace mode (direct fallback)", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// Mesma justificativa do subteste read-only: sem bwrap o modo
			// workspace falha-fechado antes da validação de args vazios.
			t.Skip("sem bwrap no Windows o erro fail-closed precede a validação de args vazios")
		}
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxWorkspace)

		_, err := g.Execute(context.Background(), chat.Command{}, chat.SandboxWorkspace)
		// If bwrap is available, it would hit the bwrap path and also return an error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty command arguments")
	})
}

// ─── Execute: unknown command ────────────────────────────────────────────────

func TestGate_Execute_UnknownCommand(t *testing.T) {
	t.Setenv("COSCA_ALLOW_NO_ROOT", "1")

	t.Run("returns error for non-existent command", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxFull)

		_, err := g.Execute(context.Background(), chat.Command{
			Args: []string{"nonexistent_command_xyz123"},
		}, chat.SandboxFull)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed")
	})
}

// ─── Execute: environment variables ──────────────────────────────────────────

func TestGate_Execute_WithEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Usa "sh -c" para ecoar a variável — shell POSIX inexistente no Windows.
		t.Skip("comando POSIX (sh -c) inexistente no Windows")
	}
	t.Setenv("COSCA_ALLOW_NO_ROOT", "1")

	t.Run("passes environment variables", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxFull)

		result, err := g.Execute(context.Background(), chat.Command{
			Args: []string{"sh", "-c", "echo $MCP_TEST_VAR"},
			Env:  map[string]string{"MCP_TEST_VAR": "hello_from_test"},
		}, chat.SandboxFull)
		require.NoError(t, err)
		assert.Contains(t, result.Stdout, "hello_from_test")
	})
}

// ─── Execute: working directory defaults to workspace ────────────────────────

func TestGate_Execute_DefaultWorkDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Usa "pwd" — comando POSIX inexistente como executável no Windows.
		t.Skip("comando POSIX (pwd) inexistente no Windows")
	}
	t.Setenv("COSCA_ALLOW_NO_ROOT", "1")

	t.Run("defaults to workspace root", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxFull)

		result, err := g.Execute(context.Background(), chat.Command{
			Args: []string{"pwd"},
		}, chat.SandboxFull)
		require.NoError(t, err)
		assert.Contains(t, result.Stdout, tmpDir)
	})
}

// ─── Execute: unsupported mode ───────────────────────────────────────────────

func TestGate_Execute_UnknownMode(t *testing.T) {
	t.Parallel()

	t.Run("returns error for invalid sandbox mode", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := NewGate(tmpDir, chat.SandboxFull)

		_, err := g.Execute(context.Background(), chat.Command{
			Args: []string{"echo", "hi"},
		}, chat.SandboxMode(99))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown sandbox mode")
	})
}

func TestGate_FailClosedWhenBwrapUnavailable(t *testing.T) {
	// No Windows o sandbox nativo (Job Object) substitui o bwrap por design
	// (ADR-034) — este teste cobre o comportamento de "sem bwrap E sem backend
	// nativo", que é o caso de plataformas sem nenhum dos dois.
	if nativeSandboxAvailable() {
		t.Skip("native sandbox available — the bwrap-required path is not the Windows backend")
	}
	g := NewGate(t.TempDir(), chat.SandboxWorkspace)
	g.bwrapPath = ""
	t.Setenv("COSCA_ALLOW_NO_ROOT", "")
	_, err := g.Execute(context.Background(), chat.Command{Args: []string{"echo", "must-not-run"}}, chat.SandboxWorkspace)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bubblewrap is required")
}

func TestGate_RejectsDangerousEnvironment(t *testing.T) {
	for _, key := range []string{"LD_PRELOAD", "LD_LIBRARY_PATH", "BASH_ENV", "NODE_OPTIONS"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv("COSCA_ALLOW_NO_ROOT", "1")
			g := NewGate(t.TempDir(), chat.SandboxFull)
			_, err := g.Execute(context.Background(), chat.Command{Args: []string{"true"}, Env: map[string]string{key: "unsafe"}}, chat.SandboxFull)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not allowed")
		})
	}
}

func TestGate_CommandAlwaysUsesCleanEnvironment(t *testing.T) {
	t.Setenv("COSCA_ALLOW_NO_ROOT", "1")
	t.Setenv("MCP_TEST_TOKEN", "ambient-secret-token")
	g := NewGate(t.TempDir(), chat.SandboxFull)

	proc, err := g.Command(context.Background(), chat.Command{Args: []string{"true"}}, chat.SandboxFull)
	require.NoError(t, err)
	require.NotNil(t, proc.Env)
	assert.NotContains(t, proc.Env, "MCP_TEST_TOKEN=ambient-secret-token")

	_, err = g.Command(context.Background(), chat.Command{
		Args: []string{"true"},
		Env:  map[string]string{"LD_PRELOAD": "ambient-secret-token"},
	}, chat.SandboxFull)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LD_PRELOAD")
}

// ─── Sandbox interface conformance ───────────────────────────────────────────

func TestGate_ImplementsSandbox(t *testing.T) {
	t.Parallel()
	var _ chat.Sandbox = (*Gate)(nil)
}
