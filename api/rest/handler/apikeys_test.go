package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiauth "github.com/CoscaAI/cosca/api/auth"
	"github.com/CoscaAI/cosca/api/rest/handler"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// TestNewAPIKeysHandler verifies the constructor returns a non-nil handler.
func TestNewAPIKeysHandler(t *testing.T) {
	h := handler.NewAPIKeysHandler(nil, nil)
	if h == nil {
		t.Fatal("expected non-nil handler with nil store")
	}
}

// TestAPIKeysListEmpty verifies List returns an empty array when no keys exist.
func TestAPIKeysListEmpty(t *testing.T) {
	keyStore := internalauth.NewAPIKeyStore(internalauth.APIKeyStoreConfig{})
	h := handler.NewAPIKeysHandler(keyStore, nil)

	req := httptest.NewRequest("GET", "/v1/api-keys", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty api key list, got %d keys", len(resp))
	}
}

// TestAPIKeysListWithKeys verifies List returns stored keys without exposing
// the hash.
func TestAPIKeysListWithKeys(t *testing.T) {
	keyStore := internalauth.NewAPIKeyStore(internalauth.APIKeyStoreConfig{})
	key, _, err := keyStore.Generate("my-key", "admin", "admin", 0)
	if err != nil {
		t.Fatalf("failed to generate api key: %v", err)
	}

	h := handler.NewAPIKeysHandler(keyStore, nil)

	req := httptest.NewRequest("GET", "/v1/api-keys", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 api key, got %d", len(resp))
	}
	if resp[0]["id"] != key.ID {
		t.Errorf("expected id %q, got %v", key.ID, resp[0]["id"])
	}
	if resp[0]["name"] != "my-key" {
		t.Errorf("expected name 'my-key', got %v", resp[0]["name"])
	}
	// The hash must never be exposed via the REST API.
	if hash, ok := resp[0]["hash"].(string); ok && hash != "" {
		t.Error("api key hash must not be exposed in List response")
	}
}

// TestAPIKeysListNilStore verifies List returns 503 when the store is nil.
func TestAPIKeysListNilStore(t *testing.T) {
	h := handler.NewAPIKeysHandler(nil, nil)

	req := httptest.NewRequest("GET", "/v1/api-keys", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAPIKeysCreateSuccess verifies Create returns 201 with the full key.
func TestAPIKeysCreateSuccess(t *testing.T) {
	keyStore := internalauth.NewAPIKeyStore(internalauth.APIKeyStoreConfig{})
	auditStore := newTestAuditStore(t)
	h := handler.NewAPIKeysHandler(keyStore, auditStore)

	body := `{"name":"deploy-key"}`
	req := httptest.NewRequest("POST", "/v1/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if id, _ := resp["id"].(string); id == "" {
		t.Error("expected non-empty id in response")
	}
	if resp["name"] != "deploy-key" {
		t.Errorf("expected name 'deploy-key', got %v", resp["name"])
	}
	// The full plaintext key is returned exactly once at creation time.
	if key, _ := resp["key"].(string); !strings.HasPrefix(key, "cosca_sk_") {
		t.Errorf("expected full key with 'cosca_sk_' prefix, got %q", key)
	}
	if resp["status"] != "active" {
		t.Errorf("expected status 'active', got %v", resp["status"])
	}
	if resp["created_by"] != "system" {
		t.Errorf("expected created_by 'system' without claims, got %v", resp["created_by"])
	}
}

// TestAPIKeysCreateWithExpiry verifies expires_in_days sets an expiration date.
func TestAPIKeysCreateWithExpiry(t *testing.T) {
	keyStore := internalauth.NewAPIKeyStore(internalauth.APIKeyStoreConfig{})
	h := handler.NewAPIKeysHandler(keyStore, nil)

	body := `{"name":"short-lived","expires_in_days":30}`
	req := httptest.NewRequest("POST", "/v1/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if expiresAt, _ := resp["expires_at"].(string); expiresAt == "" {
		t.Error("expected expires_at to be set when expires_in_days > 0")
	}
}

// TestAPIKeysCreateWithClaims verifies Create uses JWT claims for created_by
// and role (the "scopes" carried by the authenticated identity).
func TestAPIKeysCreateWithClaims(t *testing.T) {
	keyStore := internalauth.NewAPIKeyStore(internalauth.APIKeyStoreConfig{})
	auditStore := newTestAuditStore(t)
	h := handler.NewAPIKeysHandler(keyStore, auditStore)

	claims := &internalauth.Claims{
		Sub:      "user-abc",
		Username: "apikeyadmin",
		Role:     "admin",
		Type:     "access",
	}
	ctx := context.WithValue(context.Background(), apiauth.ContextKeyClaims, claims)

	body := `{"name":"claimed-key"}`
	req := httptest.NewRequest("POST", "/v1/api-keys", strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["created_by"] != "apikeyadmin" {
		t.Errorf("expected created_by 'apikeyadmin' from claims, got %v", resp["created_by"])
	}
}

// TestAPIKeysCreateDuplicateNamesAllowed documents that the handler does not
// enforce unique API key names: two keys may share the same name. Both
// creations succeed with distinct IDs.
func TestAPIKeysCreateDuplicateNamesAllowed(t *testing.T) {
	keyStore := internalauth.NewAPIKeyStore(internalauth.APIKeyStoreConfig{})
	h := handler.NewAPIKeysHandler(keyStore, nil)

	create := func() string {
		body := `{"name":"dup-name"}`
		req := httptest.NewRequest("POST", "/v1/api-keys", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		id, _ := resp["id"].(string)
		return id
	}

	first := create()
	second := create()
	if first == second {
		t.Error("expected distinct IDs for duplicate-named keys")
	}
}

// TestAPIKeysCreateInvalidJSON verifies Create returns 400 for a bad body.
func TestAPIKeysCreateInvalidJSON(t *testing.T) {
	keyStore := internalauth.NewAPIKeyStore(internalauth.APIKeyStoreConfig{})
	h := handler.NewAPIKeysHandler(keyStore, nil)

	req := httptest.NewRequest("POST", "/v1/api-keys", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAPIKeysCreateMissingName verifies Create returns 400 when name is empty.
func TestAPIKeysCreateMissingName(t *testing.T) {
	keyStore := internalauth.NewAPIKeyStore(internalauth.APIKeyStoreConfig{})
	h := handler.NewAPIKeysHandler(keyStore, nil)

	body := `{"name":""}`
	req := httptest.NewRequest("POST", "/v1/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAPIKeysCreateNameTooLong verifies Create returns 400 for a long name.
func TestAPIKeysCreateNameTooLong(t *testing.T) {
	keyStore := internalauth.NewAPIKeyStore(internalauth.APIKeyStoreConfig{})
	h := handler.NewAPIKeysHandler(keyStore, nil)

	longName := strings.Repeat("a", 101)
	body := `{"name":"` + longName + `"}`
	req := httptest.NewRequest("POST", "/v1/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for name over 100 chars, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAPIKeysCreateNilStore verifies Create returns 503 when the store is nil.
func TestAPIKeysCreateNilStore(t *testing.T) {
	h := handler.NewAPIKeysHandler(nil, nil)

	body := `{"name":"x"}`
	req := httptest.NewRequest("POST", "/v1/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAPIKeysRevokeSuccess verifies Revoke returns 200 and marks the key revoked.
func TestAPIKeysRevokeSuccess(t *testing.T) {
	keyStore := internalauth.NewAPIKeyStore(internalauth.APIKeyStoreConfig{})
	key, _, err := keyStore.Generate("revoke-me", "admin", "admin", 0)
	if err != nil {
		t.Fatalf("failed to generate api key: %v", err)
	}

	auditStore := newTestAuditStore(t)
	h := handler.NewAPIKeysHandler(keyStore, auditStore)

	req := httptest.NewRequest("DELETE", "/v1/api-keys/"+key.ID, nil)
	req.SetPathValue("id", key.ID)
	w := httptest.NewRecorder()

	h.Revoke(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["status"] != "revoked" {
		t.Errorf("expected status 'revoked', got %v", resp["status"])
	}

	// The key should now fail validation.
	loaded, err := keyStore.GetByID(key.ID)
	if err != nil {
		t.Fatalf("failed to load key: %v", err)
	}
	if loaded.Status != "revoked" {
		t.Errorf("expected stored key status 'revoked', got %q", loaded.Status)
	}
}

// TestAPIKeysRevokeNotFound verifies Revoke returns 404 for a missing key.
func TestAPIKeysRevokeNotFound(t *testing.T) {
	keyStore := internalauth.NewAPIKeyStore(internalauth.APIKeyStoreConfig{})
	h := handler.NewAPIKeysHandler(keyStore, nil)

	req := httptest.NewRequest("DELETE", "/v1/api-keys/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.Revoke(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAPIKeysRevokeMissingID verifies Revoke returns 400 when the id is empty.
func TestAPIKeysRevokeMissingID(t *testing.T) {
	keyStore := internalauth.NewAPIKeyStore(internalauth.APIKeyStoreConfig{})
	h := handler.NewAPIKeysHandler(keyStore, nil)

	req := httptest.NewRequest("DELETE", "/v1/api-keys/", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	h.Revoke(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAPIKeysRevokeNilStore verifies Revoke returns 503 when the store is nil.
func TestAPIKeysRevokeNilStore(t *testing.T) {
	h := handler.NewAPIKeysHandler(nil, nil)

	req := httptest.NewRequest("DELETE", "/v1/api-keys/whatever", nil)
	req.SetPathValue("id", "whatever")
	w := httptest.NewRecorder()

	h.Revoke(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}
