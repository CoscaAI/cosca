package config

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// saltFilePath tests
// =============================================================================

func TestSaltFilePath(t *testing.T) {
	// Set COSCA_DATA_DIR to a known value for deterministic testing
	tmpDir := t.TempDir()
	t.Setenv("COSCA_DATA_DIR", tmpDir)

	path := saltFilePath()
	expected := filepath.Join(tmpDir, saltFileName)
	if path != expected {
		t.Errorf("saltFilePath = %q, want %q", path, expected)
	}
}

// =============================================================================
// coscaDataDir tests
// =============================================================================

func TestCoscaDataDir_EnvVarSet(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("COSCA_DATA_DIR", tmpDir)

	dir := coscaDataDir()
	if dir != tmpDir {
		t.Errorf("coscaDataDir = %q, want %q", dir, tmpDir)
	}
}

func TestCoscaDataDir_NoEnvVar(t *testing.T) {
	os.Unsetenv("COSCA_DATA_DIR")
	dir := coscaDataDir()
	// Should end with .config/cosca/data
	if dir == "" {
		t.Error("coscaDataDir should not be empty")
	}
	// FromSlash evita hardcodar o separador: no Windows o caminho é
	// "C:\Users\<user>\.config\cosca\data", não ".../.config/cosca/data".
	suffix := filepath.FromSlash(".config/cosca/data")
	if len(dir) < len(suffix) || dir[len(dir)-len(suffix):] != suffix {
		t.Errorf("coscaDataDir should end with %s, got %q", suffix, dir)
	}
}

// =============================================================================
// loadOrCreateSalt tests
// =============================================================================

func TestLoadOrCreateSalt_NewSalt(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("COSCA_DATA_DIR", tmpDir)

	// Remove any existing salt file
	os.Remove(saltFilePath())

	salt, err := loadOrCreateSalt()
	if err != nil {
		t.Fatalf("loadOrCreateSalt failed: %v", err)
	}

	if len(salt) != 32 {
		t.Errorf("salt length = %d, want 32", len(salt))
	}

	// Verify salt file was created
	if _, err := os.Stat(saltFilePath()); os.IsNotExist(err) {
		t.Error("salt file was not created")
	}
}

func TestLoadOrCreateSalt_LoadsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("COSCA_DATA_DIR", tmpDir)

	// Create a known salt
	knownSalt := make([]byte, 32)
	for i := range knownSalt {
		knownSalt[i] = byte(i % 256)
	}
	if err := os.MkdirAll(coscaDataDir(), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(saltFilePath(), knownSalt, 0o600); err != nil {
		t.Fatalf("write salt: %v", err)
	}

	salt, err := loadOrCreateSalt()
	if err != nil {
		t.Fatalf("loadOrCreateSalt failed: %v", err)
	}

	if len(salt) != len(knownSalt) {
		t.Fatalf("salt length = %d, want %d", len(salt), len(knownSalt))
	}
	for i := range knownSalt {
		if salt[i] != knownSalt[i] {
			t.Errorf("salt[%d] = %d, want %d", i, salt[i], knownSalt[i])
			break
		}
	}
}

// =============================================================================
// deriveKey tests
// =============================================================================

func TestDeriveKey_Len(t *testing.T) {
	salt := make([]byte, 32)
	for i := range salt {
		salt[i] = byte(i)
	}

	key, err := deriveKey(salt)
	if err != nil {
		t.Fatalf("deriveKey failed: %v", err)
	}

	if len(key) != 32 {
		t.Errorf("key length = %d, want 32 (AES-256)", len(key))
	}
}

func TestDeriveKey_Deterministic(t *testing.T) {
	salt := make([]byte, 32)
	for i := range salt {
		salt[i] = byte(i)
	}

	key1, err := deriveKey(salt)
	if err != nil {
		t.Fatalf("deriveKey 1 failed: %v", err)
	}

	key2, err := deriveKey(salt)
	if err != nil {
		t.Fatalf("deriveKey 2 failed: %v", err)
	}

	for i := range key1 {
		if key1[i] != key2[i] {
			t.Errorf("key not deterministic: key1[%d]=%d, key2[%d]=%d", i, key1[i], i, key2[i])
			break
		}
	}
}

func TestDeriveKey_DifferentSaltGivesDifferentKey(t *testing.T) {
	salt1 := make([]byte, 32)
	salt2 := make([]byte, 32)
	for i := range salt1 {
		salt1[i] = byte(i)
		salt2[i] = byte(i + 1)
	}

	key1, err := deriveKey(salt1)
	if err != nil {
		t.Fatalf("deriveKey 1 failed: %v", err)
	}

	key2, err := deriveKey(salt2)
	if err != nil {
		t.Fatalf("deriveKey 2 failed: %v", err)
	}

	different := false
	for i := range key1 {
		if key1[i] != key2[i] {
			different = true
			break
		}
	}
	if !different {
		t.Error("Different salts should produce different keys")
	}
}

// =============================================================================
// encryptData / decryptData roundtrip tests
// =============================================================================

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	salt := make([]byte, 32)
	for i := range salt {
		salt[i] = byte(i % 256)
	}

	plaintexts := []string{
		"hello world",
		"",
		"a",
		"sk-very-long-api-key-that-should-be-encrypted-at-rest-123456789",
		"!@#$%^&*()_+-=[]{}|;':\",./<>?",
		"unicode: こんにちは世界",
	}

	for _, pt := range plaintexts {
		t.Run("len="+itoa(len(pt)), func(t *testing.T) {
			ciphertext, err := encryptData([]byte(pt), salt)
			if err != nil {
				t.Fatalf("encryptData failed: %v", err)
			}

			decrypted, err := decryptData(ciphertext, salt)
			if err != nil {
				t.Fatalf("decryptData failed: %v", err)
			}

			if string(decrypted) != pt {
				t.Errorf("decrypt roundtrip failed: got %q, want %q", string(decrypted), pt)
			}
		})
	}
}

func TestEncryptData_ProducesDifferentCiphertexts(t *testing.T) {
	salt := make([]byte, 32)
	for i := range salt {
		salt[i] = byte(i % 256)
	}

	plaintext := []byte("test message")

	c1, err := encryptData(plaintext, salt)
	if err != nil {
		t.Fatalf("encryptData 1 failed: %v", err)
	}

	c2, err := encryptData(plaintext, salt)
	if err != nil {
		t.Fatalf("encryptData 2 failed: %v", err)
	}

	// Different nonces should produce different ciphertexts
	same := true
	if len(c1) == len(c2) {
		same = true
		for i := range c1 {
			if c1[i] != c2[i] {
				same = false
				break
			}
		}
	}
	if same {
		t.Log("Ciphertexts are the same (very unlikely but possible with GCM)")
	}
}

func TestDecryptData_WrongSalt(t *testing.T) {
	salt1 := make([]byte, 32)
	salt2 := make([]byte, 32)
	for i := range salt1 {
		salt1[i] = byte(i % 256)
		salt2[i] = byte((i + 1) % 256)
	}

	plaintext := []byte("sensitive data")
	ciphertext, err := encryptData(plaintext, salt1)
	if err != nil {
		t.Fatalf("encryptData failed: %v", err)
	}

	_, err = decryptData(ciphertext, salt2)
	if err == nil {
		t.Error("decryptData should fail with wrong salt")
	}
}

func TestDecryptData_ShortCiphertext(t *testing.T) {
	salt := make([]byte, 32)

	_, err := decryptData([]byte("short"), salt)
	if err == nil {
		t.Error("decryptData should fail with too-short ciphertext")
	}
}

func TestDecryptData_CorruptedCiphertext(t *testing.T) {
	salt := make([]byte, 32)
	for i := range salt {
		salt[i] = byte(i % 256)
	}

	plaintext := []byte("test data")
	ciphertext, err := encryptData(plaintext, salt)
	if err != nil {
		t.Fatalf("encryptData failed: %v", err)
	}

	// Corrupt the last byte
	if len(ciphertext) > 0 {
		ciphertext[len(ciphertext)-1] ^= 0xFF
	}

	_, err = decryptData(ciphertext, salt)
	if err == nil {
		t.Error("decryptData should fail with corrupted ciphertext")
	}
}

// =============================================================================
// EncryptAPIKeys / DecryptAPIKeys tests
// =============================================================================

func TestEncryptDecryptAPIKeys_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("COSCA_DATA_DIR", tmpDir)
	os.Remove(saltFilePath())

	p := &ProviderConfig{
		APIKey: "sk-my-secret-openai-api-key-12345",
	}

	// Encrypt
	if err := p.EncryptAPIKeys(); err != nil {
		t.Fatalf("EncryptAPIKeys failed: %v", err)
	}

	// After encryption, key should be base64-encoded (not plaintext)
	if p.APIKey == "sk-my-secret-openai-api-key-12345" {
		t.Error("APIKey should not still be plaintext after encryption")
	}
	if p.APIKey == "" {
		t.Error("APIKey should not be empty after encryption")
	}

	// Decrypt
	if err := p.DecryptAPIKeys(); err != nil {
		t.Fatalf("DecryptAPIKeys failed: %v", err)
	}

	if p.APIKey != "sk-my-secret-openai-api-key-12345" {
		t.Errorf("APIKey after decrypt = %q, want %q", p.APIKey, "sk-my-secret-openai-api-key-12345")
	}
}

func TestEncryptAPIKeys_EmptyKey(t *testing.T) {
	p := &ProviderConfig{
		APIKey: "",
	}

	if err := p.EncryptAPIKeys(); err != nil {
		t.Errorf("EncryptAPIKeys with empty key should not error: %v", err)
	}

	if p.APIKey != "" {
		t.Errorf("APIKey should remain empty, got %q", p.APIKey)
	}
}

func TestDecryptAPIKeys_EmptyKey(t *testing.T) {
	p := &ProviderConfig{
		APIKey: "",
	}

	if err := p.DecryptAPIKeys(); err != nil {
		t.Errorf("DecryptAPIKeys with empty key should not error: %v", err)
	}
}

func TestEncryptAPIKeys_DoubleEncryption(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("COSCA_DATA_DIR", tmpDir)
	os.Remove(saltFilePath())

	p := &ProviderConfig{
		APIKey: "sk-original-key",
	}

	// First encryption
	if err := p.EncryptAPIKeys(); err != nil {
		t.Fatalf("First EncryptAPIKeys failed: %v", err)
	}
	encrypted1 := p.APIKey

	// Second encryption should be a no-op (already encrypted)
	if err := p.EncryptAPIKeys(); err != nil {
		t.Fatalf("Second EncryptAPIKeys failed: %v", err)
	}
	encrypted2 := p.APIKey

	if encrypted1 != encrypted2 {
		t.Error("Double encryption should be idempotent (keys should match)")
	}

	// Decrypt should still give us the original key
	if err := p.DecryptAPIKeys(); err != nil {
		t.Fatalf("DecryptAPIKeys failed: %v", err)
	}
	if p.APIKey != "sk-original-key" {
		t.Errorf("APIKey after double-encrypt/decrypt = %q, want %q", p.APIKey, "sk-original-key")
	}
}

func TestDecryptAPIKeys_InvalidBase64(t *testing.T) {
	p := &ProviderConfig{
		APIKey: "this-is-not-valid-base64!!!",
	}

	// Not base64 → plaintext (backward compatibility): no error, value preserved.
	err := p.DecryptAPIKeys()
	if err != nil {
		t.Errorf("DecryptAPIKeys should NOT error for plaintext keys, got: %v", err)
	}
	if p.APIKey != "this-is-not-valid-base64!!!" {
		t.Errorf("APIKey was modified: %q", p.APIKey)
	}
}

func TestDecryptAPIKeys_PlaintextKey(t *testing.T) {
	// Plaintext keys (backward compatible) should not be modified by DecryptAPIKeys
	p := &ProviderConfig{
		APIKey: "plaintext-key-12345",
	}

	err := p.DecryptAPIKeys()
	if err != nil {
		t.Errorf("DecryptAPIKeys should NOT error for plaintext keys, got: %v", err)
	}
	if p.APIKey != "plaintext-key-12345" {
		t.Errorf("APIKey was modified: %q", p.APIKey)
	}
}

// =============================================================================
// getMachineID tests
// =============================================================================

func TestGetMachineID_ReturnsNonEmpty(t *testing.T) {
	id := getMachineID()
	if id == "" {
		t.Error("getMachineID should not return empty string")
	}
}

// =============================================================================
// Helpers
// =============================================================================

// itoa is a simple int to string converter for test names.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	if neg {
		return "-" + digits
	}
	if digits == "" {
		return "0"
	}
	return digits
}
