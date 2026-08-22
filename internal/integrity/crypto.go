package integrity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	encryptedKeyPreamble = "-----BEGIN COSCA ENCRYPTED PRIVATE KEY-----"
	encryptedKeyEpilogue = "-----END COSCA ENCRYPTED PRIVATE KEY-----"
	saltLen              = 16
	nonceLen             = 12
)

// deriveKey derives a 32-byte AES-256 key from a passphrase and salt.
func deriveKey(passphrase string, salt []byte) []byte {
	// Iterated SHA-256: not a password KDF, just key derivation.
	// Security comes from passphrase secrecy, not KDF cost.
	material := append([]byte(passphrase), salt...)
	key := sha256.Sum256(material)
	for i := 0; i < 1000; i++ {
		key = sha256.Sum256(append(key[:], material...))
	}
	return key[:]
}

// encryptPrivateKey encrypts raw key bytes with AES-256-GCM using a passphrase.
// Returns the armored format: salt + nonce + ciphertext, base64-encoded.
func encryptPrivateKey(plaintext []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	key := deriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt: salt || nonce || ciphertext
	payload := append(salt, nonce...)
	payload = append(payload, aesgcm.Seal(nil, nonce, plaintext, nil)...)

	encoded := base64.StdEncoding.EncodeToString(payload)

	armored := fmt.Sprintf("%s\n%s\n%s\n", encryptedKeyPreamble, wrapBase64(encoded, 64), encryptedKeyEpilogue)
	return []byte(armored), nil
}

// decryptPrivateKey decrypts an armored encrypted private key using the passphrase.
// Returns the raw key bytes.
func decryptPrivateKey(armored []byte, passphrase string) ([]byte, error) {
	content := string(armored)
	content = strings.TrimPrefix(content, encryptedKeyPreamble+"\n")
	content = strings.TrimSuffix(content, "\n"+encryptedKeyEpilogue+"\n")
	content = strings.TrimSuffix(content, "\n"+encryptedKeyEpilogue)
	content = strings.TrimSpace(content)
	content = strings.ReplaceAll(content, "\n", "")

	payload, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("decode armored key: %w", err)
	}

	if len(payload) < saltLen+nonceLen {
		return nil, fmt.Errorf("encrypted key too short")
	}

	salt := payload[:saltLen]
	nonce := payload[saltLen : saltLen+nonceLen]
	ciphertext := payload[saltLen+nonceLen:]

	key := deriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed — wrong passphrase or tampered key: %w", err)
	}

	return plaintext, nil
}

// wrapBase64 wraps a base64 string at the given line width.
func wrapBase64(s string, width int) string {
	var b strings.Builder
	for i := 0; i < len(s); i += width {
		end := i + width
		if end > len(s) {
			end = len(s)
		}
		b.WriteString(s[i:end])
		if end < len(s) {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// isEncryptedKey checks if the key data is in the encrypted format.
func isEncryptedKey(data []byte) bool {
	return strings.HasPrefix(string(data), encryptedKeyPreamble)
}
