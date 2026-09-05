package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/internal/secrets"
)

// newTestSecretsHandler creates a SecretsHandler backed by a real encrypted
// SQLite vault in a temp directory.
func newTestSecretsHandler(t *testing.T) *handler.SecretsHandler {
	t.Helper()
	vault, err := secrets.New(filepath.Join(t.TempDir(), "secrets.db"), testJWTSecret)
	if err != nil {
		t.Fatalf("failed to create secrets vault: %v", err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	return handler.NewSecretsHandler(vault, nil)
}

// createSecret stores a secret through the handler and asserts success.
func createSecret(t *testing.T, h *handler.SecretsHandler, body string) {
	t.Helper()
	req := httptest.NewRequest("POST", "/v1/secrets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateOrUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSecretsListEmpty verifies List returns an empty keys array with count 0.
func TestSecretsListEmpty(t *testing.T) {
	h := newTestSecretsHandler(t)

	req := httptest.NewRequest("GET", "/v1/secrets", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Secrets []secrets.StoredKey `json:"secrets"`
		Count   int                 `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Secrets == nil {
		t.Error("expected non-nil secrets array")
	}
	if resp.Count != 0 {
		t.Errorf("expected count 0, got %d", resp.Count)
	}
}

// TestSecretsListWithItems verifies List returns stored secret keys.
func TestSecretsListWithItems(t *testing.T) {
	h := newTestSecretsHandler(t)

	createSecret(t, h, `{"key":"api-key","value":"sk-1234","type":"api"}`)
	createSecret(t, h, `{"key":"db-pass","value":"hunter2","type":"password"}`)

	req := httptest.NewRequest("GET", "/v1/secrets", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Secrets []secrets.StoredKey `json:"secrets"`
		Count   int                 `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Count != 2 {
		t.Errorf("expected count 2, got %d", resp.Count)
	}
	// Values must never be exposed through List.
	for _, s := range resp.Secrets {
		if s.Key != "api-key" && s.Key != "db-pass" {
			t.Errorf("unexpected key %q in list", s.Key)
		}
	}
}

// TestSecretsCreate verifies CreateOrUpdate stores a new secret.
func TestSecretsCreate(t *testing.T) {
	h := newTestSecretsHandler(t)

	body := `{"key":"token","value":"secret-value","type":"api"}`
	req := httptest.NewRequest("POST", "/v1/secrets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateOrUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp["status"] != "stored" {
		t.Errorf("expected status 'stored', got %q", resp["status"])
	}
	if resp["key"] != "token" {
		t.Errorf("expected key 'token', got %q", resp["key"])
	}
}

// TestSecretsUpdate verifies CreateOrUpdate overwrites an existing secret
// value and preserves the key.
func TestSecretsUpdate(t *testing.T) {
	h := newTestSecretsHandler(t)

	createSecret(t, h, `{"key":"token","value":"old-value","type":"api"}`)
	createSecret(t, h, `{"key":"token","value":"new-value","type":"api"}`)

	req := httptest.NewRequest("GET", "/v1/secrets/token", nil)
	req.SetPathValue("key", "token")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Value != "new-value" {
		t.Errorf("expected updated value 'new-value', got %q", resp.Value)
	}
}

// TestSecretsCreateMissingKey verifies 400 when key is empty.
func TestSecretsCreateMissingKey(t *testing.T) {
	h := newTestSecretsHandler(t)

	body := `{"key":"","value":"some-value","type":"api"}`
	req := httptest.NewRequest("POST", "/v1/secrets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateOrUpdate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSecretsCreateMissingValue verifies 400 when value is empty.
func TestSecretsCreateMissingValue(t *testing.T) {
	h := newTestSecretsHandler(t)

	body := `{"key":"token","value":"","type":"api"}`
	req := httptest.NewRequest("POST", "/v1/secrets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateOrUpdate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSecretsCreateKeyTooLong verifies 400 when the key exceeds the max
// allowed length.
func TestSecretsCreateKeyTooLong(t *testing.T) {
	h := newTestSecretsHandler(t)

	longKey := strings.Repeat("k", 101)
	body := `{"key":"` + longKey + `","value":"v","type":"api"}`
	req := httptest.NewRequest("POST", "/v1/secrets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateOrUpdate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSecretsCreateInvalidJSON verifies 400 on a malformed body.
func TestSecretsCreateInvalidJSON(t *testing.T) {
	h := newTestSecretsHandler(t)

	req := httptest.NewRequest("POST", "/v1/secrets", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateOrUpdate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSecretsGetFound verifies Get returns the decrypted value.
func TestSecretsGetFound(t *testing.T) {
	h := newTestSecretsHandler(t)

	createSecret(t, h, `{"key":"creds","value":"super-secret","type":"password","created_by":"tester"}`)

	req := httptest.NewRequest("GET", "/v1/secrets/creds", nil)
	req.SetPathValue("key", "creds")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Key       string `json:"key"`
		Value     string `json:"value"`
		Type      string `json:"type"`
		CreatedBy string `json:"created_by"`
		CreatedAt int64  `json:"created_at"`
		UpdatedAt int64  `json:"updated_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Key != "creds" {
		t.Errorf("expected key 'creds', got %q", resp.Key)
	}
	if resp.Value != "super-secret" {
		t.Errorf("expected decrypted value 'super-secret', got %q", resp.Value)
	}
	if resp.Type != "password" {
		t.Errorf("expected type 'password', got %q", resp.Type)
	}
	if resp.CreatedBy == "" {
		t.Error("expected non-empty created_by")
	}
	if resp.CreatedAt == 0 || resp.UpdatedAt == 0 {
		t.Error("expected non-zero timestamps")
	}
}

// TestSecretsGetNotFound verifies Get returns 404 for an unknown key.
func TestSecretsGetNotFound(t *testing.T) {
	h := newTestSecretsHandler(t)

	req := httptest.NewRequest("GET", "/v1/secrets/unknown", nil)
	req.SetPathValue("key", "unknown")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSecretsDeleteSuccess verifies Delete removes a secret when the
// confirm_key matches.
func TestSecretsDeleteSuccess(t *testing.T) {
	h := newTestSecretsHandler(t)

	createSecret(t, h, `{"key":"doomed","value":"value","type":"api"}`)

	body := `{"confirm_key":"doomed"}`
	req := httptest.NewRequest("DELETE", "/v1/secrets/doomed", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "doomed")
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp["status"] != "deleted" {
		t.Errorf("expected status 'deleted', got %q", resp["status"])
	}

	// The secret must be gone.
	getReq := httptest.NewRequest("GET", "/v1/secrets/doomed", nil)
	getReq.SetPathValue("key", "doomed")
	w2 := httptest.NewRecorder()
	h.Get(w2, getReq)
	if w2.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w2.Code)
	}
}

// TestSecretsDeleteNotFound verifies Delete returns 404 for an unknown key.
func TestSecretsDeleteNotFound(t *testing.T) {
	h := newTestSecretsHandler(t)

	body := `{"confirm_key":"nope"}`
	req := httptest.NewRequest("DELETE", "/v1/secrets/nope", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "nope")
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSecretsDeleteConfirmMismatch verifies Delete returns 400 when
// confirm_key does not match the path key.
func TestSecretsDeleteConfirmMismatch(t *testing.T) {
	h := newTestSecretsHandler(t)

	createSecret(t, h, `{"key":"keepme","value":"value","type":"api"}`)

	body := `{"confirm_key":"wrong-key"}`
	req := httptest.NewRequest("DELETE", "/v1/secrets/keepme", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "keepme")
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSecretsDeleteMalformedJSON verifies Delete returns 400 on a malformed
// body.
func TestSecretsDeleteMalformedJSON(t *testing.T) {
	h := newTestSecretsHandler(t)

	req := httptest.NewRequest("DELETE", "/v1/secrets/keepme", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "keepme")
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSecretsListNilVault verifies List returns 503 when the vault is nil.
// (Also covered by handler_coverage_test.go; kept here for completeness of
// the success/error matrix in this file.)
func TestSecretsListNilVaultHandler(t *testing.T) {
	h := handler.NewSecretsHandler(nil, nil)

	req := httptest.NewRequest("GET", "/v1/secrets", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}
