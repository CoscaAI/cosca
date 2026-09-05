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
	// A REAL machine-bound key (not a dummy): the unprotect must succeed and the
	// identity be confirmed on this machine/user.
	_, _, err := GenerateKeyPair(keyDir)
	require.NoError(t, err)

	require.NoError(t, VerifyKernelIdentity(t.TempDir()))
}

func TestVerifyKernelIdentity_KeyMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	// Nenhuma chave em HOME nem no fallback → nega (agente preso na jaula).
	require.Error(t, VerifyKernelIdentity(t.TempDir()))
}

func TestVerifyKernelIdentity_InvalidKeyBlob(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	keyDir := filepath.Join(home, ".config", "cosca", "keys")
	require.NoError(t, os.MkdirAll(keyDir, 0o700))
	// A file that is neither a valid machine-bound blob nor a legacy passphrase
	// key → unprotect/parse must fail → identity denied. Passphrase doesn't
	// exist anymore; the denial factor is a corrupted/invalid key blob.
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "kernel_private.key"), []byte("corrupted-blob-not-a-key"), 0o600))

	require.Error(t, VerifyKernelIdentity(t.TempDir()))
}
