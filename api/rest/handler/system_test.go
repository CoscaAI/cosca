package handler_test

// Tests for SystemHandler (system.go) — hardware telemetry + status for the
// Command Center. Covers the JSON wire shape (types/keys present, values are
// environment-dependent), the status summary, and route registration through
// the real mux.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CoscaAI/cosca/api/rest"
	"github.com/CoscaAI/cosca/api/rest/handler"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// newSystemRouteServer builds a real REST server (rest.New) and returns its
// mux, so the registered system routes can be exercised end-to-end.
func newSystemRouteServer(t *testing.T) *http.ServeMux {
	t.Helper()
	userStore := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	srv := rest.New(nil, nil, nil, nil, nil, nil, nil,
		userStore, nil, nil, rest.DefaultConfig(), nil, nil, nil, nil, nil,
		false, nil, nil, nil, nil)
	return srv.Mux()
}

// =============================================================================
// Hardware
// =============================================================================

func TestHardware_ReturnsJSONShape(t *testing.T) {
	h := handler.NewSystemHandler()

	req := httptest.NewRequest("GET", "/v1/system/hardware", nil)
	w := httptest.NewRecorder()
	h.Hardware(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Top-level sections present.
	for _, key := range []string{"cpu", "memory", "gpu"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected key %q in response", key)
		}
	}

	cpu, ok := raw["cpu"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'cpu' to be an object")
	}
	for _, key := range []string{"cores", "load1", "load5", "usage"} {
		if _, ok := cpu[key]; !ok {
			t.Errorf("expected key %q in cpu", key)
		}
	}
	if _, ok := cpu["cores"].(float64); !ok {
		t.Errorf("cores must be numeric, got %T", cpu["cores"])
	}
	if _, ok := cpu["usage"].(float64); !ok {
		t.Errorf("usage must be numeric, got %T", cpu["usage"])
	}

	memory, ok := raw["memory"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'memory' to be an object")
	}
	for _, key := range []string{"total_gb", "available_gb", "usage"} {
		if _, ok := memory[key]; !ok {
			t.Errorf("expected key %q in memory", key)
		}
	}
	if _, ok := memory["total_gb"].(float64); !ok {
		t.Errorf("total_gb must be numeric, got %T", memory["total_gb"])
	}

	gpu, ok := raw["gpu"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'gpu' to be an object")
	}
	for _, key := range []string{"vendor", "model", "vram_gb", "driver", "has_rocm", "has_vulkan", "has_cuda", "compute", "rocm_version"} {
		if _, ok := gpu[key]; !ok {
			t.Errorf("expected key %q in gpu", key)
		}
	}
	if _, ok := gpu["vram_gb"].(float64); !ok {
		t.Errorf("vram_gb must be numeric, got %T", gpu["vram_gb"])
	}
	if _, ok := gpu["has_rocm"].(bool); !ok {
		t.Errorf("has_rocm must be a bool, got %T", gpu["has_rocm"])
	}
}

// TestHardware_NoNoteOnHealthyProbe verifies the note field is omitted when
// the probe returns real readings (the Command Center wire shape has no note).
func TestHardware_NoNoteOnHealthyProbe(t *testing.T) {
	h := handler.NewSystemHandler()

	req := httptest.NewRequest("GET", "/v1/system/hardware", nil)
	w := httptest.NewRecorder()
	h.Hardware(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if note, ok := raw["note"]; ok && note != "" {
		t.Errorf("unexpected note on a healthy probe: %v", note)
	}
}

// =============================================================================
// Status
// =============================================================================

func TestStatus_ReturnsHardwareUptimeHealthy(t *testing.T) {
	h := handler.NewSystemHandler()

	req := httptest.NewRequest("GET", "/v1/system/status", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := raw["hardware"].(map[string]interface{}); !ok {
		t.Errorf("expected 'hardware' object in status, got %T", raw["hardware"])
	}
	uptime, ok := raw["uptime_seconds"].(float64)
	if !ok {
		t.Fatalf("uptime_seconds must be numeric, got %T", raw["uptime_seconds"])
	}
	if uptime < 0 {
		t.Errorf("uptime_seconds = %v, want >= 0", uptime)
	}
	healthy, ok := raw["healthy"].(bool)
	if !ok || !healthy {
		t.Errorf("healthy must be true, got %v (%T)", raw["healthy"], raw["healthy"])
	}
}

// =============================================================================
// Route registration (server.go wiring)
// =============================================================================

func TestSystemRoutes_Registered(t *testing.T) {
	mux := newSystemRouteServer(t)

	cases := []struct {
		name   string
		path   string
		status int
	}{
		{"hardware", "/v1/system/hardware", http.StatusOK},
		{"status", "/v1/system/status", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("GET %s = %d, want %d: %s", tc.path, w.Code, tc.status, w.Body.String())
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("GET %s Content-Type = %q, want application/json", tc.path, ct)
			}
		})
	}
}
