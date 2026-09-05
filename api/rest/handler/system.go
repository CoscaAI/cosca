package handler

import (
	"math"
	"net/http"
	"time"

	"github.com/CoscaAI/cosca/internal/compute"
)

// SystemHandler serves read-only system telemetry for the Command Center: a
// point-in-time hardware probe (CPU / RAM / GPU) plus a combined status
// summary. It is stateless — every request probes on demand — so it needs no
// store dependencies.
type SystemHandler struct {
	started time.Time
	probe   func() compute.HardwareSnapshot
}

// NewSystemHandler creates a new SystemHandler. Hardware probing is stateless
// and best-effort, so no dependencies are required.
func NewSystemHandler() *SystemHandler {
	return &SystemHandler{
		started: time.Now(),
		probe:   compute.ProbeHardware,
	}
}

// --- Response types ---

type cpuHardware struct {
	Cores int     `json:"cores"`
	Load1 float64 `json:"load1"`
	Load5 float64 `json:"load5"`
	Usage float64 `json:"usage"`
}

type memoryHardware struct {
	TotalGB     float64 `json:"total_gb"`
	AvailableGB float64 `json:"available_gb"`
	Usage       float64 `json:"usage"`
}

type gpuHardware struct {
	Vendor      string `json:"vendor"`
	Model       string `json:"model"`
	VRAMGB      int    `json:"vram_gb"`
	Driver      string `json:"driver"`
	HasROCm     bool   `json:"has_rocm"`
	HasVulkan   bool   `json:"has_vulkan"`
	HasCUDA     bool   `json:"has_cuda"`
	Compute     string `json:"compute"`
	ROCmVersion string `json:"rocm_version"`
}

// hardwareResponse is the payload of GET /v1/system/hardware. Usage fields are
// 0-100 percentages; RAM is reported in GB rounded to 1 decimal. The note
// field is only present when the best-effort probe returned no readings.
type hardwareResponse struct {
	CPU    cpuHardware    `json:"cpu"`
	Memory memoryHardware `json:"memory"`
	GPU    gpuHardware    `json:"gpu"`
	Note   string         `json:"note,omitempty"`
}

// statusResponse is the payload of GET /v1/system/status.
type statusResponse struct {
	Hardware      hardwareResponse `json:"hardware"`
	UptimeSeconds int64            `json:"uptime_seconds"`
	Healthy       bool             `json:"healthy"`
}

// --- Handlers ---

// Hardware handles GET /v1/system/hardware — a point-in-time CPU, RAM and GPU
// telemetry snapshot. The probe is best-effort and never 500s: on failure the
// response carries zero/empty readings plus a note field.
func (h *SystemHandler) Hardware(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.snapshot())
}

// Status handles GET /v1/system/status — the hardware snapshot plus basic
// runtime state: uptime since this handler started and a liveness flag.
func (h *SystemHandler) Status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{
		Hardware:      h.snapshot(),
		UptimeSeconds: int64(time.Since(h.started).Seconds()),
		Healthy:       true,
	})
}

// snapshot probes the system and returns a ready-to-serialize payload. A
// deferred recover guarantees the endpoint never panics or 500s even if the
// probe misbehaves — the caller always receives a valid JSON shape.
func (h *SystemHandler) snapshot() (hw hardwareResponse) {
	defer func() {
		if recover() != nil {
			hw = hardwareResponse{
				GPU:  gpuHardware{Vendor: string(compute.GPUNone)},
				Note: "hardware probe failed — zero/empty readings returned",
			}
		}
	}()

	hw = buildHardwareResponse(h.probe())
	if hw.CPU.Cores == 0 && hw.Memory.TotalGB == 0 && hw.GPU.Vendor == string(compute.GPUNone) {
		hw.Note = "hardware probe returned no readings — zero/empty values"
	}
	return hw
}

// buildHardwareResponse maps a HardwareSnapshot onto the Command Center wire
// shape. RAM is converted to GB (1 decimal); usage values pass through as
// 0-100 percentages.
func buildHardwareResponse(snap compute.HardwareSnapshot) hardwareResponse {
	return hardwareResponse{
		CPU: cpuHardware{
			Cores: snap.LogicalCores,
			Load1: snap.Load1,
			Load5: snap.Load5,
			Usage: snap.CPUUsage,
		},
		Memory: memoryHardware{
			TotalGB:     roundDecimal1(float64(snap.TotalRAM) / (1 << 30)),
			AvailableGB: roundDecimal1(float64(snap.AvailableRAM) / (1 << 30)),
			Usage:       snap.MemoryUsage,
		},
		GPU: gpuHardware{
			Vendor:      string(snap.GPU.Vendor),
			Model:       snap.GPU.Model,
			VRAMGB:      snap.GPU.VRAMGB,
			Driver:      snap.GPU.Driver,
			HasROCm:     snap.GPU.HasROCm,
			HasVulkan:   snap.GPU.HasVulkan,
			HasCUDA:     snap.GPU.HasCUDA,
			Compute:     snap.GPU.Compute,
			ROCmVersion: snap.GPU.ROCmVersion,
		},
	}
}

// roundDecimal1 rounds a value to 1 decimal place.
func roundDecimal1(v float64) float64 {
	return math.Round(v*10) / 10
}
