package handler_test

// Security regression tests for broken access control (Vulnerability A2).
//
// These tests verify that write/execute routes are gated with the role
// middleware exactly as the server registers them: a freshly registered
// viewer (the role created by POST /v1/auth/register) must be denied with
// 403, while editor and admin pass through.
//
// The tests exercise the same middleware (apiauth.RequireRole) that
// api/rest/server.go wires onto the mux, applied to the real handlers.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiauth "github.com/CoscaAI/cosca/api/auth"
	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/internal/agents"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/orchestration"
)

// viewerClaims returns JWT claims for a freshly registered viewer user.
func viewerClaims() *internalauth.Claims {
	return &internalauth.Claims{
		Sub:      "viewer-1",
		Username: "viewer",
		Role:     "viewer",
		Type:     "access",
	}
}

// editorClaims returns JWT claims for an editor user.
func editorClaims() *internalauth.Claims {
	return &internalauth.Claims{
		Sub:      "editor-1",
		Username: "editor",
		Role:     "editor",
		Type:     "access",
	}
}

// adminClaims returns JWT claims for an admin user.
func adminClaims() *internalauth.Claims {
	return &internalauth.Claims{
		Sub:      "admin-1",
		Username: "admin",
		Role:     "admin",
		Type:     "access",
	}
}

// serveAs wraps the given handler with the role gate and serves the request
// with the given claims context. Returns the recorder.
func serveAs(t *testing.T, gate apiauth.Role, h http.HandlerFunc, claims *internalauth.Claims, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	protected := apiauth.RequireRole(gate)(h)
	req = req.WithContext(newClaimsContext(claims))
	w := httptest.NewRecorder()
	protected.ServeHTTP(w, req)
	return w
}

// =============================================================================
// POST /v1/run — execution burns LLM tokens, editor+ only
// =============================================================================

// TestRun_ViewerForbidden verifies a viewer cannot execute /v1/run.
func TestRun_ViewerForbidden(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	runH := handler.NewRunHandler(agents.NewManager(""), nil, nil)
	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(`{"prompt":"Hello"}`))
	req.Header.Set("Content-Type", "application/json")

	w := serveAs(t, apiauth.RoleEditor, runH.Execute, viewerClaims(), req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for viewer on /v1/run, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRun_ViewerForbiddenStream verifies a viewer cannot execute /v1/run/stream.
func TestRun_ViewerForbiddenStream(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	runH := handler.NewRunHandler(agents.NewManager(""), nil, nil)
	req := httptest.NewRequest("POST", "/v1/run/stream", strings.NewReader(`{"prompt":"Hello"}`))
	req.Header.Set("Content-Type", "application/json")

	w := serveAs(t, apiauth.RoleEditor, runH.Stream, viewerClaims(), req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for viewer on /v1/run/stream, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRun_EditorAllowed verifies an editor can execute /v1/run (positive
// control). Uses a mock provider so the handler completes with 200.
func TestRun_EditorAllowed(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	chat.GetRegistry().Register("test-mock", mockProviderFactory, "Test mock provider", 1)
	if err := chat.GetRegistry().Select(nil, chat.ChatRegistryConfig{Primary: "test-mock", AutoDetect: false}); err != nil {
		t.Fatalf("failed to select mock provider: %v", err)
	}

	runH := handler.NewRunHandler(agents.NewManager(""), chat.GetRegistry(), nil)
	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(`{"prompt":"Hello"}`))
	req.Header.Set("Content-Type", "application/json")

	w := serveAs(t, apiauth.RoleEditor, runH.Execute, editorClaims(), req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for editor on /v1/run, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRun_AdminAllowed verifies admin bypasses the editor gate (positive
// control for the admin > editor > viewer hierarchy).
func TestRun_AdminAllowed(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	chat.GetRegistry().Register("test-mock", mockProviderFactory, "Test mock provider", 1)
	if err := chat.GetRegistry().Select(nil, chat.ChatRegistryConfig{Primary: "test-mock", AutoDetect: false}); err != nil {
		t.Fatalf("failed to select mock provider: %v", err)
	}

	runH := handler.NewRunHandler(agents.NewManager(""), chat.GetRegistry(), nil)
	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(`{"prompt":"Hello"}`))
	req.Header.Set("Content-Type", "application/json")

	w := serveAs(t, apiauth.RoleEditor, runH.Execute, adminClaims(), req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for admin on /v1/run, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// DELETE /v1/memory/delete — destructive, editor+ only
// =============================================================================

// TestMemoryDelete_ViewerForbidden verifies a viewer cannot delete memory.
func TestMemoryDelete_ViewerForbidden(t *testing.T) {
	h := newTestMemoryHandler(t)
	stored := storeTestMemory(t, h, `{"type":"fact","layer":"session","content":"must not be deleted by viewer"}`)

	req := httptest.NewRequest("DELETE", "/v1/memory/delete?id="+stored.ID+"&layer=session", nil)

	w := serveAs(t, apiauth.RoleEditor, h.Delete, viewerClaims(), req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for viewer on DELETE /v1/memory/delete, got %d: %s", w.Code, w.Body.String())
	}
}

// TestMemoryDelete_EditorAllowed verifies an editor can delete memory
// (positive control).
func TestMemoryDelete_EditorAllowed(t *testing.T) {
	h := newTestMemoryHandler(t)
	stored := storeTestMemory(t, h, `{"type":"fact","layer":"session","content":"editable"}`)

	req := httptest.NewRequest("DELETE", "/v1/memory/delete?id="+stored.ID+"&layer=session", nil)

	w := serveAs(t, apiauth.RoleEditor, h.Delete, editorClaims(), req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for editor on DELETE /v1/memory/delete, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// GET /v1/executions — exposes LLM prompts, editor+ only
// =============================================================================

// TestExecutions_ViewerForbidden verifies a viewer cannot list executions.
func TestExecutions_ViewerForbidden(t *testing.T) {
	orchestration.ResetExecutionStore()
	execH := handler.NewExecutionsHandler(orchestration.GetExecutionStore())

	req := httptest.NewRequest("GET", "/v1/executions", nil)

	w := serveAs(t, apiauth.RoleEditor, execH.List, viewerClaims(), req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for viewer on GET /v1/executions, got %d: %s", w.Code, w.Body.String())
	}
}

// TestExecutions_ViewerForbiddenGet verifies a viewer cannot read a single
// execution detail either.
func TestExecutions_ViewerForbiddenGet(t *testing.T) {
	orchestration.ResetExecutionStore()
	execH := handler.NewExecutionsHandler(orchestration.GetExecutionStore())

	req := httptest.NewRequest("GET", "/v1/executions/some-id", nil)

	w := serveAs(t, apiauth.RoleEditor, execH.Get, viewerClaims(), req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for viewer on GET /v1/executions/{id}, got %d: %s", w.Code, w.Body.String())
	}
}

// TestExecutions_EditorAllowed verifies an editor can list executions
// (positive control).
func TestExecutions_EditorAllowed(t *testing.T) {
	orchestration.ResetExecutionStore()
	execH := handler.NewExecutionsHandler(orchestration.GetExecutionStore())

	req := httptest.NewRequest("GET", "/v1/executions", nil)

	w := serveAs(t, apiauth.RoleEditor, execH.List, editorClaims(), req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for editor on GET /v1/executions, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// GET /v1/analytics — aggregate queries across all users, admin only
// =============================================================================

// TestAnalytics_AdminOnly verifies analytics is denied for viewer AND editor
// and allowed for admin.
func TestAnalytics_AdminOnly(t *testing.T) {
	auditStore := newTestAuditStore(t)
	analyticsH := handler.NewAnalyticsHandler(auditStore)

	t.Run("viewer denied", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/analytics", nil)
		w := serveAs(t, apiauth.RoleAdmin, analyticsH.GetAnalytics, viewerClaims(), req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for viewer on GET /v1/analytics, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("editor denied", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/analytics", nil)
		w := serveAs(t, apiauth.RoleAdmin, analyticsH.GetAnalytics, editorClaims(), req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for editor on GET /v1/analytics, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("admin allowed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/analytics", nil)
		w := serveAs(t, apiauth.RoleAdmin, analyticsH.GetAnalytics, adminClaims(), req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for admin on GET /v1/analytics, got %d: %s", w.Code, w.Body.String())
		}
	})
}
