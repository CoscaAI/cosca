package cosca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// ContextSDK Edge Cases
// =============================================================================

func TestContextSDK_Get_NotFound(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := c.Context.Get(context.Background(), "missing-key")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "get context") {
		t.Errorf("expected 'get context' in error, got %q", err.Error())
	}
}

func TestContextSDK_Get_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{bad`))
	})

	_, err := c.Context.Get(context.Background(), "key")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode context") {
		t.Errorf("expected 'decode context', got %q", err.Error())
	}
}

func TestContextSDK_Get_EmptyKey(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/context/")
		writeJSON(t, w, http.StatusOK, ContextEntry{Key: "", Value: "empty-key-value"})
	})

	entry, err := c.Context.Get(context.Background(), "")
	require.NoError(t, err)
	if entry.Value != "empty-key-value" {
		t.Errorf("expected Value %q, got %q", "empty-key-value", entry.Value)
	}
}

func TestContextSDK_Set_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	err := c.Context.Set(context.Background(), "bad-key", "bad-value", ContextScopeSession)
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !strings.Contains(err.Error(), "set context") {
		t.Errorf("expected 'set context', got %q", err.Error())
	}
}

func TestContextSDK_Set_AllScopes(t *testing.T) {
	t.Parallel()

	scopes := []ContextScope{ContextScopeGlobal, ContextScopeProject, ContextScopeSession}

	for _, scope := range scopes {
		scope := scope
		t.Run(string(scope), func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			})
			err := c.Context.Set(context.Background(), "k", "v", scope)
			if err != nil {
				t.Errorf("expected no error for scope %q, got %v", scope, err)
			}
		})
	}
}

func TestContextSDK_List_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := c.Context.List(context.Background(), ContextScopeSession)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "list context") {
		t.Errorf("expected 'list context', got %q", err.Error())
	}
}

func TestContextSDK_List_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[not valid`))
	})

	_, err := c.Context.List(context.Background(), ContextScopeGlobal)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode context") {
		t.Errorf("expected 'decode context', got %q", err.Error())
	}
}

func TestContextSDK_Delete_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	err := c.Context.Delete(context.Background(), "key")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "delete context") {
		t.Errorf("expected 'delete context', got %q", err.Error())
	}
}

func TestContextSDK_Build_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	err := c.Context.Build(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "build context") {
		t.Errorf("expected 'build context', got %q", err.Error())
	}
}

func TestContextSDK_GetCurrent_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := c.Context.GetCurrent(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "get current context") {
		t.Errorf("expected 'get current context', got %q", err.Error())
	}
}

func TestContextSDK_GetCurrent_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{bad json`))
	})

	_, err := c.Context.GetCurrent(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode context") {
		t.Errorf("expected 'decode context', got %q", err.Error())
	}
}

func TestContextSDK_Clear_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	err := c.Context.Clear(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "clear context") {
		t.Errorf("expected 'clear context', got %q", err.Error())
	}
}

// =============================================================================
// ContextScope Constants
// =============================================================================

func TestContextScopeConstants(t *testing.T) {
	t.Parallel()

	if ContextScopeGlobal != "global" {
		t.Errorf("expected ContextScopeGlobal 'global', got %q", ContextScopeGlobal)
	}
	if ContextScopeProject != "project" {
		t.Errorf("expected ContextScopeProject 'project', got %q", ContextScopeProject)
	}
	if ContextScopeSession != "session" {
		t.Errorf("expected ContextScopeSession 'session', got %q", ContextScopeSession)
	}
}

// =============================================================================
// Client Connect (HTTP-based)
// =============================================================================

func TestClient_Connect_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	client, err := NewClient(ClientConfig{
		RuntimeAddr:       addr,
		MaxRetries:        0,
		HeartbeatInterval: 0, // disable heartbeat
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("expected Connect to succeed, got %v", err)
	}

	if !client.IsConnected() {
		t.Error("expected client to be connected")
	}
}

func TestClient_Connect_HealthCheckFails(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	client, err := NewClient(ClientConfig{
		RuntimeAddr:       addr,
		MaxRetries:        0,
		HeartbeatInterval: 0,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.Connect(context.Background())
	if err == nil {
		t.Fatal("expected Connect to fail on non-200 health check")
	}
	if !strings.Contains(err.Error(), "runtime health check failed") {
		t.Errorf("expected 'runtime health check failed', got %q", err.Error())
	}
}

func TestClient_Close(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{
		RuntimeAddr: "localhost:9090",
		MaxRetries:  0,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Fatalf("expected Close to succeed, got %v", err)
	}

	if client.ConnectionState() != "closed" {
		t.Errorf("expected closed state, got %q", client.ConnectionState())
	}
}

func TestClient_Reconnect(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	client, err := NewClient(ClientConfig{
		RuntimeAddr:       addr,
		MaxRetries:        0,
		HeartbeatInterval: 0,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("expected initial Connect to succeed, got %v", err)
	}

	err = client.Reconnect(ctx)
	if err != nil {
		t.Fatalf("expected Reconnect to succeed, got %v", err)
	}
}

func TestClient_BaseURL(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{
		RuntimeAddr: "example.com:9090",
		MaxRetries:  0,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client.BaseURL() != "http://example.com:9090" {
		t.Errorf("expected base URL %q, got %q", "http://example.com:9090", client.BaseURL())
	}
}
