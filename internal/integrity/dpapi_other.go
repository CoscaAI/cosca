//go:build !windows

package integrity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// This file is a BEST-EFFORT, NON-PRODUCTION fallback for platforms that lack
// the Windows Data Protection API (Linux/Darwin). It lets the package compile
// and run on those platforms, but the "machine-bound" guarantee is weaker:
//
//   - The encryption key is derived from a best-effort machine identifier
//     (/etc/machine-id on Linux, or a persistent per-user 0600 file, or the
//     hostname as a last resort) — this binds the key to the same "machine"
//     but is NOT cryptographically anchored to the TPM/hardware the way DPAPI
//     is on Windows. A copy of the key material + the machine-id file can be
//     replayed on another box.
//   - Do NOT use this path for secrets that must survive an attacker with
//     filesystem access on the target host. On Windows the real DPAPI path
//     (dpapi_windows.go) is used instead.
//
// The on-disk format is self-describing (MPROT-V1:<base64>), so a blob can be
// distinguished from the legacy passphrase armored format by loadPrivateKey.

const machineKeyFallbackPrefix = "MPROT-V1:"

// machineID returns a stable, best-effort machine identifier used to derive the
// fallback key. It prefers a real machine-id file where one exists (Linux),
// otherwise creates a persistent 0600 file in the user config dir, otherwise
// falls back to the hostname. The same identifier is required to unprotect a
// blob, so it must be stable across the lifetime of the key.
func machineID() string {
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id", "/etc/hostname"} {
		if b, err := os.ReadFile(p); err == nil {
			if id := strings.TrimSpace(string(b)); id != "" {
				return id
			}
		}
	}

	if cfg, err := os.UserConfigDir(); err == nil && cfg != "" {
		p := filepath.Join(cfg, "cosca", "machine-id")
		if b, err := os.ReadFile(p); err == nil {
			if id := strings.TrimSpace(string(b)); id != "" {
				return id
			}
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err == nil {
			id := "cosca-" + randomHex(16)
			if err := os.WriteFile(p, []byte(id), 0o600); err == nil {
				return id
			}
		}
	}

	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "unknown-machine"
}

// fallbackKey derives a 32-byte AES-256 key from the machine identifier and the
// application entropy, following the same iterated-SHA-256 shape as the legacy
// deriveKey so the two remain conceptually consistent.
func fallbackKey(machineID string, entropy []byte) []byte {
	material := append([]byte("cosca-machine-bound-key-v1:"+machineID), entropy...)
	key := sha256.Sum256(material)
	for i := 0; i < 1000; i++ {
		key = sha256.Sum256(append(key[:], material...))
	}
	return key[:]
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand is not expected to fail; fall back to a fixed pattern.
		for i := range b {
			b[i] = byte(i * 7)
		}
	}
	return hex.EncodeToString(b)
}

// protectMachineKey encrypts data with AES-256-GCM keyed by the machine id and
// returns a self-describing blob. See the package note above re: platform
// strength.
func protectMachineKey(data []byte, entropy []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("protectMachineKey: empty input")
	}

	block, err := aes.NewCipher(fallbackKey(machineID(), entropy))
	if err != nil {
		return nil, fmt.Errorf("protectMachineKey: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("protectMachineKey: create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("protectMachineKey: generate nonce: %w", err)
	}

	payload := append(nonce, gcm.Seal(nil, nonce, data, nil)...)
	out := make([]byte, 0, len(machineKeyFallbackPrefix)+base64.StdEncoding.EncodedLen(len(payload)))
	out = append(out, machineKeyFallbackPrefix...)
	out = base64.StdEncoding.AppendEncode(out, payload)
	return out, nil
}

// unprotectMachineKey reverses protectMachineKey, returning an error if the
// blob was created on a different "machine" (different machine-id) or if it was
// tampered with (AES-GCM authenticates).
func unprotectMachineKey(blob []byte, entropy []byte) ([]byte, error) {
	s := string(blob)
	if !strings.HasPrefix(s, machineKeyFallbackPrefix) {
		return nil, fmt.Errorf("unprotectMachineKey: not a machine-protected blob (legacy or foreign format)")
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, machineKeyFallbackPrefix))
	if err != nil {
		return nil, fmt.Errorf("unprotectMachineKey: decode blob: %w", err)
	}

	block, err := aes.NewCipher(fallbackKey(machineID(), entropy))
	if err != nil {
		return nil, fmt.Errorf("unprotectMachineKey: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("unprotectMachineKey: create gcm: %w", err)
	}

	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("unprotectMachineKey: blob too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("unprotectMachineKey: decrypt failed (different machine or tampered): %w", err)
	}
	return plaintext, nil
}
