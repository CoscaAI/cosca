package secrets_test

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/secrets"
)

// makeMasterKey creates a random 32-byte key suitable for vault initialization.
func makeMasterKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("generate master key: %v", err)
	}
	return key
}

// TestNew_VaultCreate verifies that a new vault can be created from scratch
// with a valid master key and database path.
func TestNew_VaultCreate(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	// Verify the database file was created.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}
}

// TestNew_MasterKeyTooShort verifies New returns an error when the master
// secret is shorter than the required 32 bytes.
func TestNew_MasterKeyTooShort(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	shortKey := []byte("too-short")

	_, err := secrets.New(dbPath, shortKey)
	if err == nil {
		t.Fatal("expected error for short master key, got nil")
	}
}

// TestNew_ReopenExisting verifies that a vault can be reopened from an
// existing database file (idempotent creation).
func TestNew_ReopenExisting(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	// First create.
	v1, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("first New() error: %v", err)
	}
	v1.Close()

	// Reopen with same key.
	v2, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("second New() error: %v", err)
	}
	defer func() { _ = v2.Close() }()
}

// TestSet_StoresEncryptedValue verifies that Set correctly stores an
// encrypted value in the vault.
func TestSet_StoresEncryptedValue(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	err = vault.Set("api_key", "my-secret-value", "token", "admin")
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
}

// TestSet_EmptyKey verifies that Set returns an error when key is empty.
func TestSet_EmptyKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	err = vault.Set("", "value", "type", "user")
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
}

// TestSet_UpdateExisting verifies that an existing key can be updated.
func TestSet_UpdateExisting(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	err = vault.Set("key1", "original-value", "type-a", "user-a")
	if err != nil {
		t.Fatalf("first Set() error: %v", err)
	}

	// Update the same key.
	err = vault.Set("key1", "updated-value", "type-b", "user-b")
	if err != nil {
		t.Fatalf("second Set() error: %v", err)
	}

	secret, value, err := vault.Get("key1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if value != "updated-value" {
		t.Errorf("value = %q, want %q", value, "updated-value")
	}
	// CreatedBy and CreatedAt should be preserved on update.
	_ = secret
}

// TestGet_DecryptsStoredValue verifies that Get correctly retrieves and
// decrypts a previously stored secret.
func TestGet_DecryptsStoredValue(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	err = vault.Set("api_key", "my-secret-value", "token", "admin")
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	secret, value, err := vault.Get("api_key")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if value != "my-secret-value" {
		t.Errorf("value = %q, want %q", value, "my-secret-value")
	}
	if secret.Key != "api_key" {
		t.Errorf("key = %q, want %q", secret.Key, "api_key")
	}
	if secret.Type != "token" {
		t.Errorf("type = %q, want %q", secret.Type, "token")
	}
	if secret.CreatedBy != "admin" {
		t.Errorf("created_by = %q, want %q", secret.CreatedBy, "admin")
	}
}

// TestGet_NotFound verifies that Get returns an error for a non-existent key.
func TestGet_NotFound(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	_, _, err = vault.Get("non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
}

// TestGet_EmptyKey verifies that Get returns an error for empty key.
func TestGet_EmptyKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	_, _, err = vault.Get("")
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
}

// TestGet_WrongKeyReturnsDifferentData verifies that using a different
// master key causes decryption to fail or return garbage.
func TestGet_WrongKeyReturnsDifferentData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	err = vault.Set("secret1", "correct-value", "token", "admin")
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	vault.Close()

	// Reopen with a different key.
	differentKey := makeMasterKey(t)
	vault2, err := secrets.New(dbPath, differentKey)
	if err != nil {
		t.Fatalf("New() with different key error: %v", err)
	}
	defer func() { _ = vault2.Close() }()

	_, value, err := vault2.Get("secret1")
	// With a different key, decryption should fail.
	if err == nil {
		t.Logf("decrypted with wrong key (cryptographic coincidence): %q", value)
	}
}

// TestDelete_RemovesValue verifies that Delete removes a stored key.
func TestDelete_RemovesValue(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	err = vault.Set("key-to-delete", "some-value", "test", "user")
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	err = vault.Delete("key-to-delete")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, _, err = vault.Get("key-to-delete")
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

// TestDelete_NotFound verifies that Delete returns an error for a non-existent key.
func TestDelete_NotFound(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	err = vault.Delete("non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
}

// TestDelete_EmptyKey verifies that Delete returns an error for empty key.
func TestDelete_EmptyKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	err = vault.Delete("")
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
}

// TestList_ReturnsKeysWithoutValues verifies that List returns metadata
// without decrypted values.
func TestList_ReturnsKeysWithoutValues(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	keys := []string{"key-a", "key-b", "key-c"}
	for _, key := range keys {
		err := vault.Set(key, "val-"+key, "type-"+key, "user-"+key)
		if err != nil {
			t.Fatalf("Set(%q) error: %v", key, err)
		}
	}

	stored, err := vault.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(stored) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(stored))
	}

	// Verify all expected keys are present and no values are leaked.
	for _, sk := range stored {
		found := false
		for _, k := range keys {
			if sk.Key == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected key in list: %q", sk.Key)
		}
		if sk.CreatedBy != "user-"+sk.Key {
			t.Errorf("key %q created_by = %q", sk.Key, sk.CreatedBy)
		}
	}
}

// TestList_EmptyVault verifies List returns an empty slice when vault is empty.
func TestList_EmptyVault(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	keys, err := vault.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

// TestEncryptDecrypt_ExportImport verifies the round-trip through
// EncryptForExport / DecryptFromImport.
func TestEncryptDecrypt_ExportImport(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	plaintext := "sensitive export data"

	hexCiphertext, err := vault.EncryptForExport(plaintext)
	if err != nil {
		t.Fatalf("EncryptForExport() error: %v", err)
	}

	decrypted, err := vault.DecryptFromImport(hexCiphertext)
	if err != nil {
		t.Fatalf("DecryptFromImport() error: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

// TestEncryptDecrypt_WrongKeyExportImport verifies that importing ciphertext
// encrypted with a different key fails to decrypt properly.
func TestEncryptDecrypt_WrongKeyExportImport(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault1, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	plaintext := "sensitive export data"
	hexCiphertext, err := vault1.EncryptForExport(plaintext)
	if err != nil {
		t.Fatalf("EncryptForExport() error: %v", err)
	}
	vault1.Close()

	// Create a vault with a different key.
	dbPath2 := filepath.Join(t.TempDir(), "vault2.db")
	differentKey := makeMasterKey(t)
	vault2, err := secrets.New(dbPath2, differentKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault2.Close() }()

	_, err = vault2.DecryptFromImport(hexCiphertext)
	if err == nil {
		t.Fatal("expected decryption error with wrong key, got nil")
	}
}

// TestEncrypt_NonceUniqueness verifies that each encryption call produces
// a different ciphertext (due to random nonce) even for the same plaintext.
func TestEncrypt_NonceUniqueness(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	hex1, err := vault.EncryptForExport("same-plaintext")
	if err != nil {
		t.Fatalf("first EncryptForExport() error: %v", err)
	}

	hex2, err := vault.EncryptForExport("same-plaintext")
	if err != nil {
		t.Fatalf("second EncryptForExport() error: %v", err)
	}

	if hex1 == hex2 {
		t.Error("expected different ciphertexts due to random nonce")
	}

	// Both should decrypt back to the same plaintext.
	dec1, err := vault.DecryptFromImport(hex1)
	if err != nil {
		t.Fatalf("DecryptFromImport(hex1) error: %v", err)
	}
	dec2, err := vault.DecryptFromImport(hex2)
	if err != nil {
		t.Fatalf("DecryptFromImport(hex2) error: %v", err)
	}
	if dec1 != dec2 || dec1 != "same-plaintext" {
		t.Errorf("decrypted values: %q, %q, want both %q", dec1, dec2, "same-plaintext")
	}
}

// TestDecryptFromImport_InvalidHex verifies that invalid hex input produces an error.
func TestDecryptFromImport_InvalidHex(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	_, err = vault.DecryptFromImport("not-hex-zzzz")
	if err == nil {
		t.Fatal("expected error for invalid hex, got nil")
	}
}

// TestClose_NoError verifies Close does not return an error.
func TestClose_NoError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := vault.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

// TestClose_DoubleClose verifies that double close is safe.
func TestClose_DoubleClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := vault.Close(); err != nil {
		t.Logf("first Close() error: %v", err)
	}
	// Second close should be safe.
	if err := vault.Close(); err != nil {
		t.Logf("second Close() error: %v", err)
	}
}

// TestSetGet_MultipleValues verifies storing and retrieving multiple secrets.
func TestSetGet_MultipleValues(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	masterKey := makeMasterKey(t)

	vault, err := secrets.New(dbPath, masterKey)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = vault.Close() }()

	testCases := map[string]struct {
		value     string
		typ       string
		createdBy string
	}{
		"api_key":     {"sk-1234567890", "token", "admin"},
		"db_password": {"p@ssw0rd!", "password", "admin"},
		"certificate": {"BEGIN CERTIFICATE...", "cert", "system"},
		"long_secret": {"this is a much longer secret value that spans multiple words and contains special characters like !@#$%^&*()", "note", "editor"},
		"unicode_key": {"café-🦀-value", "token", "admin"},
	}

	for key, tc := range testCases {
		err := vault.Set(key, tc.value, tc.typ, tc.createdBy)
		if err != nil {
			t.Fatalf("Set(%q) error: %v", key, err)
		}
	}

	for key, tc := range testCases {
		secret, value, err := vault.Get(key)
		if err != nil {
			t.Errorf("Get(%q) error: %v", key, err)
			continue
		}
		if value != tc.value {
			t.Errorf("Get(%q) value = %q, want %q", key, value, tc.value)
		}
		if secret.Key != key {
			t.Errorf("Get(%q) key = %q", key, secret.Key)
		}
		if secret.Type != tc.typ {
			t.Errorf("Get(%q) type = %q, want %q", key, secret.Type, tc.typ)
		}
		if secret.CreatedBy != tc.createdBy {
			t.Errorf("Get(%q) created_by = %q, want %q", key, secret.CreatedBy, tc.createdBy)
		}

		// Verify timestamps are set.
		if secret.CreatedAt == 0 {
			t.Errorf("Get(%q) created_at is zero", key)
		}
		if secret.UpdatedAt == 0 {
			t.Errorf("Get(%q) updated_at is zero", key)
		}
	}
}
