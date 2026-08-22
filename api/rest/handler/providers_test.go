package handler_test

// Tests for ProvidersHandler (providers.go) with a real providers.Manager
// (static catalogue — no network or credentials required).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/internal/providers"
)

// newProvidersHandler returns a handler backed by a fresh static manager.
func newProvidersHandler() *handler.ProvidersHandler {
	return handler.NewProvidersHandler(providers.NewManager())
}

// TestProvidersListWithManager verifies List returns 200 with the static
// catalogue (non-empty, includes openai).
func TestProvidersListWithManager(t *testing.T) {
	h := newProvidersHandler()

	req := httptest.NewRequest("GET", "/v1/providers", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var resp []providers.ProviderInfo
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(resp) == 0 {
		t.Fatal("expected non-empty providers list")
	}
	found := false
	for _, p := range resp {
		if p.Name == "openai" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'openai' in providers list")
	}
}

// TestProvidersGetFound verifies Get returns 200 for a known provider.
func TestProvidersGetFound(t *testing.T) {
	h := newProvidersHandler()

	req := httptest.NewRequest("GET", "/v1/providers/openai", nil)
	req.SetPathValue("name", "openai")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var resp providers.ProviderInfo
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Name != "openai" {
		t.Errorf("expected provider name 'openai', got %q", resp.Name)
	}
}

// TestProvidersGetNotFound verifies Get returns 404 for an unknown provider.
func TestProvidersGetNotFound(t *testing.T) {
	h := newProvidersHandler()

	req := httptest.NewRequest("GET", "/v1/providers/nonexistent", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
	}
}

// TestProvidersTestLocal verifies Test returns 200 for the 'local' provider.
// 'local' is always configured and never touches the network (its BaseURL is
// not a valid HTTP URL, so testHTTP reports 'configured' immediately).
func TestProvidersTestLocal(t *testing.T) {
	h := newProvidersHandler()

	req := httptest.NewRequest("POST", "/v1/providers/local/test", nil)
	req.SetPathValue("name", "local")
	w := httptest.NewRecorder()
	h.Test(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var resp providers.TestResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Status == "" {
		t.Error("expected a status in test result")
	}
	if resp.Model != "tf-idf" {
		t.Errorf("expected model 'tf-idf', got %q", resp.Model)
	}
}

// TestProvidersTestUnknown verifies Test returns 500 for an unknown provider.
func TestProvidersTestUnknown(t *testing.T) {
	h := newProvidersHandler()

	req := httptest.NewRequest("POST", "/v1/providers/nonexistent/test", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()
	h.Test(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 Internal Server Error, got %d: %s", w.Code, w.Body.String())
	}
}

// TestProvidersSetActiveSuccess verifies SetActive returns 200 with the
// updated active provider in the status payload.
func TestProvidersSetActiveSuccess(t *testing.T) {
	h := newProvidersHandler()

	body := `{"provider":"openai","model":"gpt-4o"}`
	req := httptest.NewRequest("PUT", "/v1/providers/active", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.SetActive(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var resp providers.ProviderStatus
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Active != "openai" {
		t.Errorf("expected active provider 'openai', got %q", resp.Active)
	}
}

// TestProvidersSetActiveUnknown verifies SetActive returns 400 when the
// provider is not in the catalogue.
func TestProvidersSetActiveUnknown(t *testing.T) {
	h := newProvidersHandler()

	body := `{"provider":"nonexistent"}`
	req := httptest.NewRequest("PUT", "/v1/providers/active", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.SetActive(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestProvidersNilManagerSearch verifies Search/Get paths against a nil
// manager. Get must return 503 (the only nil-mgr path not covered by
// handler_coverage_test.go for providers).
func TestProvidersNilManagerGet(t *testing.T) {
	h := handler.NewProvidersHandler(nil)

	req := httptest.NewRequest("GET", "/v1/providers/openai", nil)
	req.SetPathValue("name", "openai")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}
