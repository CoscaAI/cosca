package integrity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateKeyPair creates a new Ed25519 key pair and saves them as PEM files.
//
// MACHINE-BOUND (no passphrase): the private key is serialized as PKCS#8 and
// protected with protectMachineKey (DPAPI on Windows), so it is bound to the
// current user on the current machine. There is nothing to remember.
//
// Security controls:
//   - M2: on Windows a restrictive DACL (owner-only) is applied to the private
//     key file. On POSIX the 0600 mode bits are the enforcement.
//   - M6: the key material is never logged, formatted, or echoed.
//   - M1: the key lives in keysDir (production: ~/.config/cosca/keys, outside
//     the jail), never inside the workspace.
//
// Private key file: 0600 permissions (only kernel can re-sign).
// Public key file: 0644 permissions.
// Returns the raw keys for immediate use (e.g., signing genesis block).
func GenerateKeyPair(keysDir string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ed25519: %w", err)
	}

	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return nil, nil, fmt.Errorf("create keys dir: %w", err)
	}

	// Serialize private key to PKCS#8 (raw DER), then protect machine-bound.
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal private key: %w", err)
	}

	protected, err := protectPrivateKey(privBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("protect private key (machine-bound): %w", err)
	}

	privPath := filepath.Join(keysDir, "kernel_private.key")
	if err := os.WriteFile(privPath, protected, 0600); err != nil {
		return nil, nil, fmt.Errorf("write private key: %w", err)
	}

	// M2: enforce a real owner-only DACL on Windows (best-effort 0600 on POSIX).
	if err := applyPrivateKeyACL(privPath); err != nil {
		return nil, nil, fmt.Errorf("apply restrictive acl to private key: %w", err)
	}

	// Save public key (SPKI PEM) — always unencrypted, needed for verification
	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal public key: %w", err)
	}
	pubPath := filepath.Join(keysDir, "kernel_public.key")
	if err := os.WriteFile(pubPath, pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}), 0644); err != nil {
		return nil, nil, fmt.Errorf("write public key: %w", err)
	}

	return pub, priv, nil
}
