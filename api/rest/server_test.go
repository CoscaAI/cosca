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

func TestServer_CognitiveContractRoutes(t *testing.T) {
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

	// Contratos de leitura do cérebro — a UI do visualizador foi movida para o
	// dashboard "Casa Visível" (app separada). Estes endpoints mantêm os data
	// contracts (Graph/Observatory/Activity/Perception) e são read-only.
	// O teste usa s.mux (sem a cadeia de middleware auth), servindo o handler
	// direto — o foco é a REGISTRAÇÃO da rota e o JSON sanitizado.
	cases := []struct {
		method string
		path   string
	}{
		{method: "GET", path: "/v1/organization/graph"},
		{method: "GET", path: "/v1/cognitive/observatory"},
		{method: "GET", path: "/v1/cognitive/activity"},
		{method: "GET", path: "/v1/cognitive/perception"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s %s: expected 200, got %d (body=%s)", tc.method, tc.path, w.Code, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("%s %s: content-type=%q, expected application/json", tc.method, tc.path, ct)
		}
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
