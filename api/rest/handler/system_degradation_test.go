package handler

// White-box tests for SystemHandler (system.go). Because the hardware probe
// seam (h.probe) is unexported, this file lives in package handler — the same
// pattern as auth_whitebox_test.go. Verifies the endpoint degrades gracefully
// (never 500s) when the probe panics or returns no readings.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CoscaAI/cosca/internal/compute"
)

// TestHardware_PanickingProbeDegrades verifies a probe panic never 500s: the
// handler recovers and returns a valid JSON shape with zero/empty readings and
// a note field.
func TestHardware_PanickingProbeDegrades(t *testing.T) {
	h := NewSystemHandler()
	h.probe = func() compute.HardwareSnapshot {
		panic("probe exploded")
	}

	req := httptest.NewRequest("GET", "/v1/system/hardware", nil)
	w := httptest.NewRecorder()
	h.Hardware(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (graceful), got %d: %s", w.Code, w.Body.String())
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if raw["note"] == nil || raw["note"] == "" {
		t.Errorf("expected a degradation note, got %v", raw["note"])
	}
	cpu := raw["cpu"].(map[string]interface{})
	if cpu["cores"].(float64) != 0 {
		t.Errorf("expected zero cores on degraded probe, got %v", cpu["cores"])
	}
	memory := raw["memory"].(map[string]interface{})
	if memory["total_gb"].(float64) != 0 {
		t.Errorf("expected zero total_gb on degraded probe, got %v", memory["total_gb"])
	}
	gpu := raw["gpu"].(map[string]interface{})
	if gpu["vendor"].(string) != string(compute.GPUNone) {
		t.Errorf("expected gpu vendor %q on degraded probe, got %q", compute.GPUNone, gpu["vendor"])
	}
}

// TestHardware_EmptyProbeNote verifies a probe that succeeds but returns no
// usable readings still 200s with zero values plus a note.
func TestHardware_EmptyProbeNote(t *testing.T) {
	h := NewSystemHandler()
	h.probe = func() compute.HardwareSnapshot {
		return compute.HardwareSnapshot{GPU: compute.GPUInfo{Vendor: compute.GPUNone}}
	}

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
	if raw["note"] == nil || raw["note"] == "" {
		t.Errorf("expected a note when the probe returns no readings, got %v", raw["note"])
	}
}

// TestHardware_WellFormedProbeHasNoNote verifies a fully populated snapshot
// yields a clean payload (no note).
func TestHardware_WellFormedProbeHasNoNote(t *testing.T) {
	h := NewSystemHandler()
	h.probe = func() compute.HardwareSnapshot {
		return compute.HardwareSnapshot{
			LogicalCores: 16,
			TotalRAM:     32 * (1 << 30),
			AvailableRAM: 20 * (1 << 30),
			MemoryUsage:  37.5,
			GPU: compute.GPUInfo{
				Vendor:  compute.GPUAmd,
				Model:   "AMD Radeon RX 6700 XT",
				VRAMGB:  12,
				HasROCm: true,
			},
		}
	}

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
		t.Errorf("unexpected note on a well-formed probe: %v", note)
	}
	memory := raw["memory"].(map[string]interface{})
	if memory["total_gb"].(float64) != 32.0 {
		t.Errorf("expected total_gb 32, got %v", memory["total_gb"])
	}
	cpu := raw["cpu"].(map[string]interface{})
	if cpu["cores"].(float64) != 16 {
		t.Errorf("expected 16 cores, got %v", cpu["cores"])
	}
}

// TestHardware_ProbedAtIncluded verifies the CPU section keeps load readings
// from the snapshot.
func TestHardware_LoadValuesFromSnapshot(t *testing.T) {
	h := NewSystemHandler()
	h.probe = func() compute.HardwareSnapshot {
		return compute.HardwareSnapshot{
			LogicalCores: 8,
			Load1:        1.25,
			Load5:        1.5,
			CPUUsage:     42.0,
		}
	}

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
	cpu := raw["cpu"].(map[string]interface{})
	if cpu["load1"].(float64) != 1.25 {
		t.Errorf("expected load1 1.25, got %v", cpu["load1"])
	}
	if cpu["usage"].(float64) != 42.0 {
		t.Errorf("expected usage 42, got %v", cpu["usage"])
	}
}
