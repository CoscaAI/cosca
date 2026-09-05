package integrity

import (
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// machineSignSetup builds a signing-ready workspace and a REAL machine-bound
// kernel key in the isolated HOME key dir. It returns the workspace root and the
// kernel key dir. COSCA_ROOT is pinned to the workspace so Ed25519 signature
// verification (verifyBlockFromChain) resolves the chain file.
func machineSignSetup(t *testing.T) (root, keysDir string) {
	t.Helper()
	home := isolateUserHome(t)
	keysDir = filepath.Join(home, ".config", "cosca", "keys")
	_, _, err := GenerateKeyPair(keysDir)
	require.NoError(t, err)

	root = t.TempDir()
	embed := filepath.Join(root, "internal", "embed", "cosca", "memory")
	require.NoError(t, os.MkdirAll(embed, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(embed, "test.md"), []byte("hello machine\n"), 0o644))

	// Versioned public key (as in production) — integrity.Check is fail-closed.
	pubData, err := os.ReadFile(filepath.Join(keysDir, "kernel_public.key"))
	require.NoError(t, err)
	embKey := filepath.Join(root, "internal", "embed", "cosca", "keys", "kernel_public.key")
	require.NoError(t, os.MkdirAll(filepath.Dir(embKey), 0o755))
	require.NoError(t, os.WriteFile(embKey, pubData, 0o644))

	// Active workspace key integrity.Check reads.
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".cosca", "keys"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".cosca", "keys", "kernel_public.key"), pubData, 0o600))

	t.Setenv("COSCA_ROOT", root)
	return root, keysDir
}

func TestSignMachineBound(t *testing.T) {
	root, _ := machineSignSetup(t)

	res, err := Sign(root)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, 1, res.BlockNumber)
	// Ed25519-signed block (NOT git-anchored).
	require.False(t, res.Anchored)
	require.NotEmpty(t, res.BlockHash)
	require.NotEmpty(t, res.PrevHash)
	require.Greater(t, res.FilesSigned, 0)

	info, err := Check(root)
	require.NoError(t, err)
	require.True(t, info.Valid, "machine-bound signed chain must verify: %v", info.Errors)
}

func TestSignAfterLearningMachineBound(t *testing.T) {
	root, _ := machineSignSetup(t)

	res := SignAfterLearning(root)
	require.NotNil(t, res)
	// With a machine-bound key present, the Ed25519 path wins (no git fallback).
	require.False(t, res.Anchored, "machine-bound key must produce an Ed25519 block")
	require.Greater(t, res.BlockNumber, 0)

	info, err := Check(root)
	require.NoError(t, err)
	require.True(t, info.Valid, "machine-bound SignAfterLearning chain must verify: %v", info.Errors)
}

func TestSign_CorruptBlobRefused(t *testing.T) {
	root, keysDir := machineSignSetup(t)

	// Corrupt the machine-bound blob — unprotect MUST fail and refuse to read.
	require.NoError(t, os.WriteFile(filepath.Join(keysDir, "kernel_private.key"), []byte("corrupted-blob"), 0o600))

	_, err := Sign(root)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unprotect", "error must name the machine-bound unprotect failure")
}

func TestMachineKeyRoundTrip(t *testing.T) {
	isolateUserHome(t) // hermetic: don't write a fallback machine-id into the real config dir
	secret := []byte("0123456789abcdef0123456789abcdef")
	require.Len(t, secret, 32)

	blob, err := protectMachineKey(secret, machineKeyEntropy)
	require.NoError(t, err)
	require.NotEmpty(t, blob)

	// Same machine + user must recover the exact bytes.
	recovered, err := unprotectMachineKey(blob, machineKeyEntropy)
	require.NoError(t, err)
	require.Equal(t, secret, recovered)
}

func TestMachineKeyWrongEntropyFails(t *testing.T) {
	isolateUserHome(t)
	blob, err := protectMachineKey([]byte("secret"), []byte("entropy-a"))
	require.NoError(t, err)

	// Wrong entropy (domain separator) must fail to unprotect.
	_, err = unprotectMachineKey(blob, []byte("entropy-b"))
	require.Error(t, err)
}

func TestMachineKeyEmptyInput(t *testing.T) {
	isolateUserHome(t)
	_, err := protectMachineKey(nil, machineKeyEntropy)
	require.Error(t, err)
	_, err = unprotectMachineKey(nil, machineKeyEntropy)
	require.Error(t, err)
}

// TestSignErrorDoesNotLeakKeyMaterial is an M6 regression guard: even when a
// signing error is produced AFTER the private key is loaded into memory, the
// error must never echo the key material (raw, hex or base64).
func TestSignErrorDoesNotLeakKeyMaterial(t *testing.T) {
	root, keysDir := machineSignSetup(t)

	priv, err := loadPrivateKey(filepath.Join(keysDir, "kernel_private.key"))
	require.NoError(t, err)
	blob, err := os.ReadFile(filepath.Join(keysDir, "kernel_private.key"))
	require.NoError(t, err)

	// Every representation of the key material that must never appear in an
	// error string.
	leaks := []string{}
	for _, raw := range [][]byte{priv, priv.Seed(), blob} {
		leaks = append(leaks, string(raw))
		leaks = append(leaks, base64.StdEncoding.EncodeToString(raw))
		leaks = append(leaks, hex.EncodeToString(raw))
	}

	// Force a failure AFTER the key is loaded: remove the embed dir so
	// scanEmbed errors while the key is in memory.
	embed := filepath.Join(root, "internal", "embed", "cosca")
	require.NoError(t, os.RemoveAll(embed))

	_, err = Sign(root)
	require.Error(t, err)
	errStr := err.Error()
	for _, leak := range leaks {
		require.NotContains(t, errStr, leak, "sign error must not leak key material")
	}
	require.Contains(t, errStr, "scan embed", "expected the pipeline failure, not a key echo")
}
