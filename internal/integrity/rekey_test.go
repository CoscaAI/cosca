package integrity

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRekey_GeneratesNewKeypair(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// os.UserHomeDir reads USERPROFILE on Windows (HOME is ignored).
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}

	// Arm an old key so kernelKeyDir resolves to the temp HOME (not the
	// coscaRoot fallback), mirroring production where the key lives outside
	// the jail.
	oldKeys := filepath.Join(home, ".config", "cosca", "keys")
	require.NoError(t, os.MkdirAll(oldKeys, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(oldKeys, "kernel_private.key"), []byte("old-dummy"), 0o600))

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "embed", "cosca", "keys"), 0o755))

	require.NoError(t, Rekey(root))

	// The new private key is machine-bound (no passphrase): it unprotects on
	// this machine/user and yields a full Ed25519 identity.
	privPath := filepath.Join(oldKeys, "kernel_private.key")
	_, err := loadPrivateKey(privPath)
	require.NoError(t, err, "new key must unprotect machine-bound on this machine")

	// Versioned public key updated.
	pub, err := os.ReadFile(filepath.Join(root, "internal", "embed", "cosca", "keys", "kernel_public.key"))
	require.NoError(t, err)
	require.NotEmpty(t, pub)

	// Workspace copy (.cosca/keys/) espelha a versionada — é a que Check lê.
	ws, err := os.ReadFile(filepath.Join(root, ".cosca", "keys", "kernel_public.key"))
	require.NoError(t, err)
	require.Equal(t, pub, ws, "workspace public key must mirror the versioned key")
}

// TestRekey_ChangesKeypair proves Rekey actually mints a FRESH identity on each
// call (passphrase no longer exists — this is the machine-bound recovery path).
func TestRekey_ChangesKeypair(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "embed", "cosca", "keys"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".cosca", "keys"), 0o700))

	require.NoError(t, Rekey(root))
	first, err := os.ReadFile(filepath.Join(root, ".cosca", "keys", "kernel_public.key"))
	require.NoError(t, err)
	require.NotEmpty(t, first)

	require.NoError(t, Rekey(root))
	second, err := os.ReadFile(filepath.Join(root, ".cosca", "keys", "kernel_public.key"))
	require.NoError(t, err)
	require.NotEmpty(t, second)

	// A fresh identity must differ from the previous one.
	require.NotEqual(t, first, second, "rekeying must produce a fresh keypair")
}
