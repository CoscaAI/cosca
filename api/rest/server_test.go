package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/CoscaAI/cosca/internal/workflows"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected loopback host, got %s", cfg.Host)
	}
	if cfg.Port == 0 {
		t.Error("port should not be zero")
	}
	if cfg.ReadTimeout == 0 {
		t.Error("ReadTimeout should have a default")
	}
	if cfg.WriteTimeout == 0 {
		t.Error("WriteTimeout should have a default")
	}
	if cfg.RegistrationEnabled {
		t.Error("Registration should be disabled by default")
	}
	if cfg.RuntimeClient != nil {
		t.Error("RuntimeClient should be nil by default")
	}
}

func TestConfig_FieldValues(t *testing.T) {
	cfg := Config{
		Host:           "0.0.0.0",
		Port:           8080,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
		CORSOrigins:    "*",
	}

	if cfg.Host != "0.0.0.0" {
		t.Errorf("host mismatch")
	}
	if cfg.Port != 8080 {
		t.Errorf("port mismatch")
	}
	if cfg.MaxHeaderBytes != 1<<20 {
		t.Errorf("max header bytes mismatch")
	}
}

func TestNewServer_Minimal(t *testing.T) {
	agentsMgr := agents.NewManager("test")
	skillsMgr := skills.NewManager("test")
	providersMgr := providers.NewManager()
	workflowsMgr := workflows.NewManager("test")

	cfg := DefaultConfig()
	cfg.Port = 9999

	s := New(
		nil,                                   // knowledge
		nil,                                   // memory
		nil,                                   // runtime
		agentsMgr,                             // agents
		skillsMgr,                             // skills
		providersMgr,                          // providers
		workflowsMgr,                          // workflows
		nil,                                   // authStore
		nil,                                   // apiKeyStore
		[]byte("test-secret-32-bytes-long!!"), // jwtSecret
		cfg,
		nil,   // tokenStore
		nil,   // auditStore
		nil,   // secretsVault
		nil,   // emergencyMgr
		nil,   // pluginsMgr
		false, // enableWebsocket
		nil,   // wsAllowedOrigins
		nil,   // traceStore
		nil,   // deptStore
		nil,   // pipelineServices
	)

	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.mux == nil {
		t.Fatal("mux should not be nil")
	}
	if s.emergencyManager == nil {
		t.Error("emergencyManager should be auto-created when nil is passed")
	}
	if s.config.Port != 9999 {
		t.Errorf("port mismatch: %d", s.config.Port)
	}
}

func TestServer_RouteRegistration(t *testing.T) {
	agentsMgr := agents.NewManager("test")
	skillsMgr := skills.NewManager("test")
	providersMgr := providers.NewManager()
	workflowsMgr := workflows.NewManager("test")

	cfg := DefaultConfig()
	cfg.Port = 9998

	s := New(
		nil, nil, nil,
		agentsMgr, skillsMgr, providersMgr, workflowsMgr,
		nil, nil,
		[]byte("test-secret-32-bytes-long!!"),
		cfg,
		nil, nil, nil, nil, nil,
		false, nil, nil, nil, nil,
	)

	// Test that critical routes exist by hitting them.
	// /health always returns 200. /ready may return 503 when no runtime.
	routes := []struct {
		method  string
		path    string
		minCode int // minimum acceptable HTTP status
		maxCode int
	}{
		{"GET", "/health", 200, 200},
		{"GET", "/ready", 200, 503},
	}

	for _, rt := range routes {
		req := httptest.NewRequest(rt.method, rt.path, nil)
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)

		if w.Code < rt.minCode || w.Code > rt.maxCode {
			t.Errorf("%s %s: code %d outside [%d,%d]", rt.method, rt.path, w.Code, rt.minCode, rt.maxCode)
		}
	}
}

func TestServer_BrainWebRoutes(t *testing.T) {
	agentsMgr := agents.NewManager("test")
	skillsMgr := skills.NewManager("test")
	providersMgr := providers.NewManager()
	workflowsMgr := workflows.NewManager("test")

	cfg := DefaultConfig()
	cfg.Port = 9996

	s := New(
		nil, nil, nil,
		agentsMgr, skillsMgr, providersMgr, workflowsMgr,
		nil, nil,
		[]byte("test-secret-32-bytes-long!!"),
		cfg, nil, nil, nil, nil, nil,
		false, nil, nil, nil, nil,
	)

	// /brain/graph deve servir JSON 200 mesmo sem token (rota pública,
	// dados sanitizados). A rota atravessa auth fall-closed.
	req := httptest.NewRequest(http.MethodGet, "/brain/graph", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("/brain/graph: expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("/brain/graph: content-type=%q, expected application/json", ct)
	}

	// /brain deve servir o index.html 200 sem token.
	req2 := httptest.NewRequest(http.MethodGet, "/brain", nil)
	w2 := httptest.NewRecorder()
	s.mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("/brain: expected 200, got %d", w2.Code)
	}
	if ct := w2.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("/brain: content-type=%q, expected text/html", ct)
	}

	// /brain/activity deve servir JSON 200 sem token (read-only).
	req3 := httptest.NewRequest(http.MethodGet, "/brain/activity", nil)
	w3 := httptest.NewRecorder()
	s.mux.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("/brain/activity: expected 200, got %d", w3.Code)
	}
	if ct := w3.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("/brain/activity: content-type=%q, expected application/json", ct)
	}

	// /brain/observatory deve servir JSON 200 sem token (read-only).
	req4 := httptest.NewRequest(http.MethodGet, "/brain/observatory", nil)
	w4 := httptest.NewRecorder()
	s.mux.ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("/brain/observatory: expected 200, got %d", w4.Code)
	}
	if ct := w4.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("/brain/observatory: content-type=%q, expected application/json", ct)
	}
}

func TestServer_HealthEndpoint(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Port = 9997

	s := New(
		nil, nil, nil,
		agents.NewManager("test"),
		skills.NewManager("test"),
		providers.NewManager(),
		workflows.NewManager("test"),
		nil, nil,
		[]byte("test-secret-32-bytes-long!!"),
		cfg, nil, nil, nil, nil, nil,
		false, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health endpoint: expected 200, got %d", w.Code)
	}
}

func TestServer_ReadyEndpoint(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Port = 9996

	s := New(
		nil, nil, nil,
		agents.NewManager("test"),
		skills.NewManager("test"),
		providers.NewManager(),
		workflows.NewManager("test"),
		nil, nil,
		[]byte("test-secret-32-bytes-long!!"),
		cfg, nil, nil, nil, nil, nil,
		false, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	// /ready returns 503 when runtime is nil — that's expected.
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("ready endpoint: expected 200 or 503, got %d", w.Code)
	}
}

func TestServer_MiddlewareApplication(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Port = 9995

	s := New(
		nil, nil, nil,
		agents.NewManager("test"),
		skills.NewManager("test"),
		providers.NewManager(),
		workflows.NewManager("test"),
		nil, nil,
		[]byte("test-secret-32-bytes-long!!"),
		cfg, nil, nil, nil, nil, nil,
		false, nil, nil, nil, nil,
	)

	// Verify we can add middleware without panicking.
	s.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "middleware")
			next.ServeHTTP(w, r)
		})
	})

	// Verify middleware is registered (not panic).
	if len(s.middlewares) != 1 {
		t.Errorf("expected 1 middleware, got %d", len(s.middlewares))
	}
}
