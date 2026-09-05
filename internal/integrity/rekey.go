package integrity

import (
	"fmt"
	"os"
	"path/filepath"
)

// Rekey mints a NEW Ed25519 keypair protected machine-bound (DPAPI), replacing
// the old private key. This is the recovery path when the Don forgets the
// passphrase — but with machine-binding there is no passphrase to forget, so
// rekey is instead used to (a) migrate a legacy passphrase-encrypted key to the
// machine-bound format, or (b) mint a fresh identity when the current key is
// compromised or tied to a different machine/user.
//
// The versioned public key (internal/embed/cosca/keys/kernel_public.key) is
// updated so integrity.Check continues to pass. The git-anchored chain does NOT
// depend on the Ed25519 key (blocks carry SIGNATURE: GIT-ANCHORED), so rekeying
// does not invalidate existing chain blocks — only the caller must re-sign
// (cosca-check --sign-auto) because the public key file changed.
func Rekey(coscaRoot string) error {
	keysDir := kernelKeyDir(coscaRoot)
	if keysDir == "" {
		return fmt.Errorf("cannot resolve user config directory for kernel key")
	}

	if _, _, err := GenerateKeyPair(keysDir); err != nil {
		return fmt.Errorf("generate new keypair: %w", err)
	}

	pubData, err := os.ReadFile(filepath.Join(keysDir, "kernel_public.key"))
	if err != nil {
		return fmt.Errorf("read new public key: %w", err)
	}

	embeddedPub := filepath.Join(coscaRoot, "internal", "embed", "cosca", "keys", "kernel_public.key")
	if err := os.MkdirAll(filepath.Dir(embeddedPub), 0o755); err != nil {
		return fmt.Errorf("create embed keys dir: %w", err)
	}
	if err := os.WriteFile(embeddedPub, pubData, 0o644); err != nil {
		return fmt.Errorf("write versioned public key: %w", err)
	}

	// A cópia ativa que integrity.Check lê vive em .cosca/keys/ — precisa
	// espelhar a versionada, senão o Check reporta "PUBLIC KEY MISMATCH".
	wsKeysDir := filepath.Join(coscaRoot, ".cosca", "keys")
	if err := os.MkdirAll(wsKeysDir, 0o700); err != nil {
		return fmt.Errorf("create workspace keys dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(wsKeysDir, "kernel_public.key"), pubData, 0o600); err != nil {
		return fmt.Errorf("write workspace public key: %w", err)
	}
	return nil
}
