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
// The private key is encrypted with the passphrase (AES-256-GCM) before saving.
// Private key file: 0600 permissions (only kernel can re-sign).
// Public key file: 0644 permissions.
// Returns the raw keys for immediate use (e.g., signing genesis block).
func GenerateKeyPair(keysDir string, passphrase string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ed25519: %w", err)
	}

	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return nil, nil, fmt.Errorf("create keys dir: %w", err)
	}

	// Serialize private key to PKCS#8
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal private key: %w", err)
	}

	// Encrypt with passphrase before saving
	encrypted, err := encryptPrivateKey(privBytes, passphrase)
	if err != nil {
		return nil, nil, fmt.Errorf("encrypt private key: %w", err)
	}

	privPath := filepath.Join(keysDir, "kernel_private.key")
	if err := os.WriteFile(privPath, encrypted, 0600); err != nil {
		return nil, nil, fmt.Errorf("write private key: %w", err)
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
