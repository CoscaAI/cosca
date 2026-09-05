package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestShouldRunDeterministic verifies the deterministic-mode decision
// function (serve.go). No server is booted.
//
// Cases:
//
//	""+""        → false (current behavior — flag unset, env unset)
//	"none"+""    → true  (flag none)
//	""+"none"    → true  (env COSCA_PROVIDER=none)
//	"ollama"+"none" → false (flag wins for a real provider)
func TestShouldRunDeterministic(t *testing.T) {
	cases := []struct {
		name      string
		flagValue string
		envValue  string
		want      bool
	}{
		{"no flag no env — current behavior", "", "", false},
		{"flag none", "none", "", true},
		{"env none", "", "none", true},
		{"flag wins for real provider", "ollama", "none", false},
		{"flag wins for real provider, empty env", "ollama", "", false},
		{"flag case-insensitive", "NONE", "", true},
		{"env case-insensitive", "", "None", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRunDeterministic(tc.flagValue, tc.envValue); got != tc.want {
				t.Errorf("shouldRunDeterministic(%q, %q) = %v, want %v", tc.flagValue, tc.envValue, got, tc.want)
			}
		})
	}
}

// TestServeCommand_ProviderFlag verifies the --provider flag wiring without
// booting the server.
func TestServeCommand_ProviderFlag(t *testing.T) {
	cmd := NewServeCommand()
	flag := cmd.Flags().Lookup("provider")
	if flag == nil {
		t.Fatal("missing --provider flag on serve command")
	}
	if flag.DefValue != "" {
		t.Errorf("--provider default = %q, want \"\"", flag.DefValue)
	}
	if err := cmd.Flags().Set("provider", "none"); err != nil {
		t.Fatalf("--provider none should parse: %v", err)
	}
	got, err := cmd.Flags().GetString("provider")
	if err != nil {
		t.Fatalf("GetString(provider): %v", err)
	}
	if got != "none" {
		t.Errorf("provider = %q, want \"none\"", got)
	}
}

// TestServeCommand_ProviderFlagEnvOnly verifies the flag default stays empty
// so COSCA_PROVIDER remains the source of truth when --provider is unset.
func TestServeCommand_ProviderFlagEnvOnly(t *testing.T) {
	cmd := NewServeCommand()
	got, err := cmd.Flags().GetString("provider")
	if err != nil {
		t.Fatalf("GetString(provider): %v", err)
	}
	if got != "" {
		t.Errorf("provider = %q, want empty default", got)
	}
}

// TestIsLLMRestPath verifies which REST routes the deterministic-mode
// middleware blocks (503) versus passes through.
func TestIsLLMRestPath(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{"run", http.MethodPost, "/v1/run", true},
		{"run stream", http.MethodPost, "/v1/run/stream", true},
		{"workflow run", http.MethodPost, "/v1/workflows/deploy/run", true},
		{"workflow run stream", http.MethodPost, "/v1/workflows/deploy/run/stream", true},
		{"workflow list passes", http.MethodGet, "/v1/workflows", false},
		{"workflow get passes", http.MethodGet, "/v1/workflows/deploy", false},
		{"knowledge search passes", http.MethodPost, "/v1/knowledge/search", false},
		{"status passes", http.MethodGet, "/v1/status", false},
		{"health passes", http.MethodGet, "/health", false},
		{"memory passes", http.MethodGet, "/v1/memory/search", false},
		{"run GET is not an LLM path", http.MethodGet, "/v1/run", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLLMRestPath(tc.method, tc.path); got != tc.want {
				t.Errorf("isLLMRestPath(%q, %q) = %v, want %v", tc.method, tc.path, got, tc.want)
			}
		})
	}
}

// TestDeterministicModeMiddleware_LLMReturns503 verifies the middleware
// blocks LLM-dependent routes with 503 and the deterministic message, and
// passes deterministic routes through.
func TestDeterministicModeMiddleware_LLMReturns503(t *testing.T) {
	mw := deterministicModeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot) // marker: request passed through
	}))

	t.Run("blocked run", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/run", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "capacidade cognitiva indisponível") {
			t.Errorf("body = %q, want deterministic message", rec.Body.String())
		}
	})

	t.Run("pass-through status", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/status", nil))
		if rec.Code != http.StatusTeapot {
			t.Errorf("status = %d, want pass-through (418)", rec.Code)
		}
	})
}
