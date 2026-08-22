//
// Package config provides AES-256-GCM encryption for sensitive config fields
// (API keys) at rest. The encryption key is derived from a machine-specific
// identifier (hostname + machine-id) combined with a persistent random salt
// stored in the Cosca data directory. This defends against exfiltration of the
// plaintext config file without requiring an external keyring dependency.

package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

// MarshalYAML prevents accidental plaintext YAML serialization by callers
// outside Config.Save. External references omit the credential; legacy
// in-memory credentials are encrypted in the serialized copy.
func (p ProviderConfig) MarshalYAML() (interface{}, error) {
	wire := p
	if wire.APIKeyEnv != "" {
		wire.APIKey = ""
	} else if err := wire.EncryptAPIKeys(); err != nil {
		return nil, fmt.Errorf("protect provider credential: %w", err)
	}
	type providerConfigWire ProviderConfig
	return providerConfigWire(wire), nil
}

// saltFileName is the name of the salt file stored in the Cosca data directory.
const saltFileName = ".cosca_salt"

// coscaDataDir returns the Cosca data directory path.
// It respects COSCA_DATA_DIR environment variable, falling back to
// ~/.config/cosca/data.
func coscaDataDir() string {
	if env := os.Getenv("COSCA_DATA_DIR"); env != "" {
		if expanded, err := homedir.Expand(env); err == nil {
			return expanded
		}
	}
	return filepath.Join(UserHomeDir(), ".config", "cosca", "data")
}

// saltFilePath returns the full path to the salt file.
func saltFilePath() string {
	return filepath.Join(coscaDataDir(), saltFileName)
}

// loadOrCreateSalt loads the persistent salt from disk, creating a new
// random salt if one does not exist. The salt file is created with 0o600
// permissions. The salt does not need to be secret — it is combined with
// the hostname and machine-id only to anchor the derived key to this machine.
func loadOrCreateSalt() ([]byte, error) {
	path := saltFilePath()

	// Try to read existing salt
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		return data, nil
	}

	// Create a new random salt (32 bytes)
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(coscaDataDir(), 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	// Write salt with restrictive permissions
	if err := os.WriteFile(path, salt, 0o600); err != nil {
		return nil, fmt.Errorf("write salt file: %w", err)
	}

	return salt, nil
}

// deriveKey creates a 32-byte AES-256 key from machine-specific data
// combined with the given salt. The machine data is hostname+machine-id,
// providing a semi-stable, machine-bound key.
func deriveKey(salt []byte) ([]byte, error) {
	// Usa COSCA_REAL_HOSTNAME se disponivel (setado pelo auto-jail antes
	// de entrar na jaula). Isso garante que a chave derivada seja a mesma
	// dentro e fora da jaula, mesmo com --hostname cosca-jail no bwrap.
	hostname := os.Getenv("COSCA_REAL_HOSTNAME")
	if hostname == "" {
		var err error
		hostname, err = os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
	}
	machineID := getMachineID()
	data := fmt.Sprintf("%s:%s", hostname, machineID)
	hash := sha256.Sum256(append([]byte(data), salt...))
	return hash[:], nil
}

// getMachineID tries to read a machine-specific identifier.
// On Linux it reads /etc/machine-id; falls back to hostname.
func getMachineID() string {
	paths := []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil && len(data) >= 32 {
			return string(data[:32])
		}
	}
	// Fallback: use hostname
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return "unknown"
}

// encryptData encrypts plaintext with AES-256-GCM using a key derived from
// the given salt. Returns nonce+ciphertext (the nonce is prepended).
func encryptData(plaintext []byte, salt []byte) ([]byte, error) {
	key, err := deriveKey(salt)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Prepend nonce to ciphertext for storage: nonce + encrypted_data
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decryptData decrypts data that was encrypted with encryptData.
// Expects ciphertext in the format: nonce + encrypted_data.
func decryptData(ciphertext []byte, salt []byte) ([]byte, error) {
	key, err := deriveKey(salt)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, encryptedData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// =============================================================================
// ProviderConfig encryption/decryption helpers
// =============================================================================

// EncryptAPIKeys encrypts the APIKey field of the ProviderConfig.
// If the key is already encrypted (detected by successful base64 decode),
// it is left as-is to avoid double-encryption. If the key is empty, nothing
// is done. The encrypted value is stored as base64-encoded ciphertext.
func (p *ProviderConfig) EncryptAPIKeys() error {
	if p.APIKey == "" {
		return nil
	}

	// Check if already encrypted (base64-encoded ciphertext)
	if _, err := base64.StdEncoding.DecodeString(p.APIKey); err == nil {
		// Try to decrypt it to check — if decrypt succeeds, it's already encrypted
		salt, saltErr := loadOrCreateSalt()
		if saltErr != nil {
			return saltErr
		}
		decoded, _ := base64.StdEncoding.DecodeString(p.APIKey)
		if _, decErr := decryptData(decoded, salt); decErr == nil {
			// Already encrypted, skip
			return nil
		}
	}

	salt, err := loadOrCreateSalt()
	if err != nil {
		return fmt.Errorf("load salt: %w", err)
	}

	encrypted, err := encryptData([]byte(p.APIKey), salt)
	if err != nil {
		return fmt.Errorf("encrypt API key: %w", err)
	}

	p.APIKey = base64.StdEncoding.EncodeToString(encrypted)
	return nil
}

// DecryptAPIKeys decrypts the APIKey field of the ProviderConfig if it is
// encrypted (base64-encoded ciphertext). If the key is plaintext (backward
// compatibility) or empty, it is left as-is.
func (p *ProviderConfig) DecryptAPIKeys() error {
	if p.APIKey == "" {
		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(p.APIKey)
	if err != nil {
		// Not valid base64 ciphertext → plaintext key (backward
		// compatibility). Leave it as-is, no error, no warning.
		return nil
	}

	salt, err := loadOrCreateSalt()
	if err != nil {
		return fmt.Errorf("load salt: %w", err)
	}

	decrypted, decErr := decryptData(decoded, salt)
	if decErr != nil {
		return decErr
	}

	p.APIKey = string(decrypted)
	return nil
}

// MaskAPIKey returns a masked version of the API key suitable for logging
// and display. It shows the first 4 and last 4 characters separated by
// "...", preserving the ability to identify which key is in use without
// exposing the full secret. Keys shorter than 12 characters are fully masked.
func MaskAPIKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
