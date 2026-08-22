package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreAPIKey_EncryptsAtRest(t *testing.T) {
	// Isola num dir temporário.
	home, _ := os.UserHomeDir()
	orig := os.Getenv("COSCA_GLOBAL_CONFIG")
	os.Setenv("COSCA_GLOBAL_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	defer func() {
		if orig != "" {
			os.Setenv("COSCA_GLOBAL_CONFIG", orig)
		} else {
			os.Unsetenv("COSCA_GLOBAL_CONFIG")
		}
		_ = home
	}()

	if err := StoreAPIKey("openai", "sk-super-secret-12345"); err != nil {
		t.Fatalf("StoreAPIKey: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(os.Getenv("COSCA_GLOBAL_CONFIG")))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "sk-super-secret-12345") {
		t.Error("API key must be encrypted at rest (plaintext found)")
	}
	if !strings.Contains(string(data), "openai") {
		t.Error("provider name should be persisted")
	}
}

func TestStoreAPIKey_EmptyKey_Errors(t *testing.T) {
	if err := StoreAPIKey("openai", ""); err == nil {
		t.Error("empty key should error")
	}
}
