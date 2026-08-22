package integrity

import (
	"fmt"
	"os"
	"path/filepath"
)

// VerifyKernelIdentity confirms the caller is the kernel — i.e. it can read AND
// decrypt the kernel's Ed25519 private key, which lives OUTSIDE the jail
// (~/.config/cosca/keys) and is protected machine-bound via DPAPI (no
// passphrase). Sandboxed agents cannot reach that path, and a different machine
// or user cannot unprotect the blob, so this cleanly separates "kernel on this
// machine (trusted)" from "agent (untrusted)".
//
// There is no passphrase factor anymore: the proof is "same machine + same user
// profile" (the DPAPI binding), which is exactly what makes the key
// machine-bound. An unreadable file or a blob that fails to unprotect (wrong
// machine/user, tampered, or a legacy passphrase-encrypted key) returns an
// error and denies identity.
func VerifyKernelIdentity(coscaRoot string) error {
	keysDir := kernelKeyDir(coscaRoot)
	if keysDir == "" {
		return fmt.Errorf("kernel identity unavailable — cannot resolve user config directory")
	}
	privKeyPath := filepath.Join(keysDir, "kernel_private.key")

	if _, err := os.ReadFile(privKeyPath); err != nil {
		return fmt.Errorf("kernel identity unavailable — private key not readable (inside a jail?): %w", err)
	}

	if _, err := loadPrivateKey(privKeyPath); err != nil {
		return fmt.Errorf("kernel identity unavailable — machine-bound key could not be unprotect (wrong machine or user?): %w", err)
	}
	return nil
}
