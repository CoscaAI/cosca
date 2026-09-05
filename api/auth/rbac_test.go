package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CoscaAI/cosca/api/auth"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// TestRequireRoleAdmin verifies that an admin user passes through
// RequireRole for an admin-only endpoint.
func TestRequireRoleAdmin(t *testing.T) {
	mw := auth.RequireRole(auth.RoleAdmin)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	claims := &internalauth.Claims{
		Sub:      "admin-1",
		Username: "adminuser",
		Role:     "admin",
	}
	ctx := context.WithValue(context.Background(), auth.ContextKeyClaims, claims)

	req := httptest.NewRequest("GET", "/v1/admin/endpoint", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called for admin user")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}
}

// TestRequireRoleEditorAccessingAdmin verifies that an editor user gets a 403
// when trying to access an admin-only endpoint.
func TestRequireRoleEditorAccessingAdmin(t *testing.T) {
	mw := auth.RequireRole(auth.RoleAdmin)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for editor accessing admin endpoint")
		w.WriteHeader(http.StatusOK)
	}))

	claims := &internalauth.Claims{
		Sub:      "editor-1",
		Username: "editoruser",
		Role:     "editor",
	}
	ctx := context.WithValue(context.Background(), auth.ContextKeyClaims, claims)

	req := httptest.NewRequest("GET", "/v1/admin/endpoint", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", w.Code)
	}
}

// TestRequireRoleViewerAccessingEditor verifies that a viewer user gets a 403
// when trying to access an editor-only endpoint.
func TestRequireRoleViewerAccessingEditor(t *testing.T) {
	mw := auth.RequireRole(auth.RoleEditor)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for viewer accessing editor endpoint")
		w.WriteHeader(http.StatusOK)
	}))

	claims := &internalauth.Claims{
		Sub:      "viewer-1",
		Username: "vieweruser",
		Role:     "viewer",
	}
	ctx := context.WithValue(context.Background(), auth.ContextKeyClaims, claims)

	req := httptest.NewRequest("PUT", "/v1/editor/endpoint", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", w.Code)
	}
}

// TestRequireRoleNoClaims verifies that a request without claims in context
// returns 401 Unauthorized.
func TestRequireRoleNoClaims(t *testing.T) {
	mw := auth.RequireRole(auth.RoleAdmin)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when no claims are present")
		w.WriteHeader(http.StatusOK)
	}))

	// Context without claims.
	req := httptest.NewRequest("GET", "/v1/admin/endpoint", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", w.Code)
	}
}

// TestRequireRoleAdminBypassesAll verifies that an admin user bypasses
// all role checks, even for explicitly requiring admin.
func TestRequireRoleAdminBypassesAll(t *testing.T) {
	mw := auth.RequireRole(auth.RoleEditor)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	claims := &internalauth.Claims{
		Sub:      "admin-2",
		Username: "adminuser2",
		Role:     "admin",
	}
	ctx := context.WithValue(context.Background(), auth.ContextKeyClaims, claims)

	req := httptest.NewRequest("GET", "/v1/editor/endpoint", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called — admin bypasses all role checks")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}
}

// TestRequireRoleEditorWithViewerLevel verifies that a viewer can access
// a viewer-level endpoint.
func TestRequireRoleViewerWithViewerLevel(t *testing.T) {
	mw := auth.RequireRole(auth.RoleViewer)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	claims := &internalauth.Claims{
		Sub:      "viewer-2",
		Username: "vieweruser2",
		Role:     "viewer",
	}
	ctx := context.WithValue(context.Background(), auth.ContextKeyClaims, claims)

	req := httptest.NewRequest("GET", "/v1/viewer/endpoint", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called for viewer accessing viewer endpoint")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}
}

// TestRequireRoleEditorWithEditorLevel verifies that an editor can access
// an editor-level endpoint.
func TestRequireRoleEditorWithEditorLevel(t *testing.T) {
	mw := auth.RequireRole(auth.RoleEditor)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	claims := &internalauth.Claims{
		Sub:      "editor-2",
		Username: "editoruser2",
		Role:     "editor",
	}
	ctx := context.WithValue(context.Background(), auth.ContextKeyClaims, claims)

	req := httptest.NewRequest("POST", "/v1/editor/endpoint", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called for editor accessing editor endpoint")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}
}

// TestRequireRoleWithNilClaims verifies that nil claims in context
// returns 401 Unauthorized.
func TestRequireRoleWithNilClaims(t *testing.T) {
	mw := auth.RequireRole(auth.RoleViewer)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for nil claims")
		w.WriteHeader(http.StatusOK)
	}))

	// Put a typed nil in the context.
	ctx := context.WithValue(context.Background(), auth.ContextKeyClaims, (*internalauth.Claims)(nil))

	req := httptest.NewRequest("GET", "/v1/endpoint", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", w.Code)
	}
}

// TestRequireRole_EditorHierarchy verifies the full editor gate hierarchy:
// a viewer is denied (403), an editor is allowed (200), and an admin
// bypasses to the editor level (200). This is the exact contract enforced
// on the write/execute REST routes.
func TestRequireRole_EditorHierarchy(t *testing.T) {
	mw := auth.RequireRole(auth.RoleEditor)

	tests := []struct {
		name     string
		role     string
		expectOK bool
	}{
		{name: "viewer denied editor access", role: "viewer", expectOK: false},
		{name: "editor allowed editor access", role: "editor", expectOK: true},
		{name: "admin bypasses to editor access", role: "admin", expectOK: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			claims := &internalauth.Claims{
				Sub:      "user-" + tc.role,
				Username: tc.role,
				Role:     tc.role,
				Type:     "access",
			}
			ctx := context.WithValue(context.Background(), auth.ContextKeyClaims, claims)

			req := httptest.NewRequest("POST", "/v1/editor/endpoint", nil).WithContext(ctx)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			if tc.expectOK {
				if !called {
					t.Error("handler should have been called")
				}
				if w.Code != http.StatusOK {
					t.Errorf("expected 200 OK, got %d", w.Code)
				}
			} else {
				if called {
					t.Error("handler should not have been called")
				}
				if w.Code != http.StatusForbidden {
					t.Errorf("expected 403 Forbidden, got %d", w.Code)
				}
			}
		})
	}
}
