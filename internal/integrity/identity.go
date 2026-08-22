package integrity

import (
	"fmt"
	"os"
	"path/filepath"
)

// VerifyKernelIdentity confirms the caller is the kernel — i.e. it can read
// the kernel's private key, which lives OUTSIDE the jail (~/.config/cosca/keys,
// mode 0600). Sandboxed agents cannot reach that path, so this cleanly
// separates "kernel (trusted)" from "agent (untrusted)".
//
// If passphrase is non-empty, the check is stronger: it must also decrypt the
// private key (full Ed25519 identity proof — requires the Don's passphrase).
func VerifyKernelIdentity(coscaRoot, passphrase string) error {
	keysDir := kernelKeyDir(coscaRoot)
	privKeyPath := filepath.Join(keysDir, "kernel_private.key")

	if _, err := os.ReadFile(privKeyPath); err != nil {
		return fmt.Errorf("kernel identity unavailable — private key not readable (inside a jail?): %w", err)
	}

	if passphrase != "" {
		if _, err := loadPrivateKey(privKeyPath, passphrase); err != nil {
			return fmt.Errorf("kernel passphrase invalid: %w", err)
		}
	}
	return nil
}
