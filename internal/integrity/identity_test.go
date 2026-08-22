package integrity

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifyKernelIdentity_KeyReadable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// os.UserHomeDir reads USERPROFILE on Windows (HOME is ignored).
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	keyDir := filepath.Join(home, ".config", "cosca", "keys")
	require.NoError(t, os.MkdirAll(keyDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "kernel_private.key"), []byte("dummy-key-material"), 0o600))

	// Chave legível (sem passphrase) → identidade confirmada.
	require.NoError(t, VerifyKernelIdentity(t.TempDir(), ""))
}

func TestVerifyKernelIdentity_KeyMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Nenhuma chave em HOME nem no fallback → nega (agente preso na jaula).
	require.Error(t, VerifyKernelIdentity(t.TempDir(), ""))
}

func TestVerifyKernelIdentity_PassphraseInvalid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	keyDir := filepath.Join(home, ".config", "cosca", "keys")
	require.NoError(t, os.MkdirAll(keyDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "kernel_private.key"), []byte("dummy-key-material"), 0o600))

	// Chave inválida não decripta com passphrase nenhuma → nega.
	require.Error(t, VerifyKernelIdentity(t.TempDir(), "passphrase-errada"))
}
