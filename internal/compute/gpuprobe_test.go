package compute

import (
	"encoding/json"
	"strings"
	"testing"
)

// =============================================================================
// GPU Probe — real machine
// =============================================================================

func TestProbeGPU_RealMachine(t *testing.T) {
	info := ProbeGPU()
	if info.Vendor == GPUNone {
		t.Skip("no GPU detected in this environment; fallback covered by TestProbeGPU_Fallback_None")
	}
	if info.Vendor != GPUAmd {
		t.Skipf("machine has a non-AMD GPU (%s); AMD-specific assertions skipped", info.Vendor)
	}
	if !info.HasROCm {
		t.Error("AMD machine: expected HasROCm (rocminfo/rocm-smi or /opt/rocm)")
	}
	if info.Model == "" || (!strings.Contains(info.Model, "RX 6700") && !strings.Contains(info.Model, "Navi")) {
		t.Errorf("AMD machine: expected model containing 'RX 6700' or 'Navi', got %q", info.Model)
	}
	if info.VRAMGB <= 0 {
		t.Errorf("AMD machine: expected VRAMGB > 0, got %d", info.VRAMGB)
	}
	if info.Compute == "" {
		t.Error("AMD machine: expected compute target (e.g. gfx1031)")
	}
	if info.Driver == "" {
		t.Error("AMD machine: expected a driver")
	}
}

// =============================================================================
// GPU Probe — fake environments
// =============================================================================

func fakeSourcesAllEmpty() gpuSources {
	return gpuSources{
		runCommand: func(string, ...string) string { return "" },
		hasCommand: func(string) bool { return false },
		readFile:   func(string) string { return "" },
		stat:       func(string) bool { return false },
		listDir:    func(string) []string { return nil },
		readVendor: func() GPUVendor { return "" },
	}
}

func TestProbeGPU_Fallback_None(t *testing.T) {
	info := probeGPU(fakeSourcesAllEmpty())
	if info.Vendor != GPUNone {
		t.Errorf("expected GPUNone in empty environment, got %q", info.Vendor)
	}
}

func TestProbeGPU_FakeAMD(t *testing.T) {
	s := fakeSourcesAllEmpty()
	s.readVendor = func() GPUVendor { return GPUAmd }
	s.hasCommand = func(name string) bool {
		return name == "rocm-smi" || name == "rocminfo"
	}
	s.stat = func(path string) bool { return path == "/dev/kfd" }
	s.readFile = func(path string) string {
		if path == "/opt/rocm/.info/version" {
			return "7.2.4"
		}
		return ""
	}
	s.listDir = func(path string) []string {
		switch path {
		case "/opt":
			return []string{"rocm-7.2.4"}
		case "/usr/share/vulkan/icd.d":
			return []string{"radeon_icd.json"}
		}
		return nil
	}
	s.runCommand = func(name string, args ...string) string {
		switch {
		case name == "rocminfo":
			return "  Name:                    gfx1031\n  Marketing Name:          AMD Radeon RX 6700 XT\n"
		case name == "rocm-smi" && len(args) == 2:
			return "GPU[0]\t\t: VRAM Total Memory (B): 12868124672\n"
		case name == "rocm-smi" && len(args) == 1:
			return "GPU[0]\t\t: GFX Version: \t\tgfx1031\n"
		}
		return ""
	}

	info := probeGPU(s)
	if info.Vendor != GPUAmd {
		t.Errorf("vendor = %q, want amd", info.Vendor)
	}
	if !info.HasROCm {
		t.Error("HasROCm should be true")
	}
	if info.Model != "AMD Radeon RX 6700 XT" {
		t.Errorf("model = %q", info.Model)
	}
	if info.VRAMGB != 12 {
		t.Errorf("VRAMGB = %d, want 12", info.VRAMGB)
	}
	if info.Compute != "gfx1031" {
		t.Errorf("compute = %q, want gfx1031", info.Compute)
	}
	if info.ROCmVersion != "7.2.4" {
		t.Errorf("rocm_version = %q, want 7.2.4", info.ROCmVersion)
	}
	if !info.HasVulkan {
		t.Error("HasVulkan should be true")
	}
	if info.Driver != "rocm-smi" {
		t.Errorf("driver = %q, want rocm-smi", info.Driver)
	}
}

func TestProbeGPU_FakeNVIDIA(t *testing.T) {
	s := fakeSourcesAllEmpty()
	s.readVendor = func() GPUVendor { return GPUNvidia }
	s.hasCommand = func(name string) bool { return name == "nvidia-smi" }
	s.runCommand = func(name string, args ...string) string {
		switch {
		case strings.Contains(strings.Join(args, " "), "name,memory.total"):
			return "NVIDIA GeForce RTX 4090, 24564 MiB\n"
		case strings.Contains(strings.Join(args, " "), "compute_cap"):
			return "8.9\n"
		}
		return ""
	}

	info := probeGPU(s)
	if info.Vendor != GPUNvidia {
		t.Errorf("vendor = %q, want nvidia", info.Vendor)
	}
	if !info.HasCUDA {
		t.Error("HasCUDA should be true")
	}
	if info.Model != "NVIDIA GeForce RTX 4090" {
		t.Errorf("model = %q", info.Model)
	}
	if info.VRAMGB != 24 {
		t.Errorf("VRAMGB = %d, want 24", info.VRAMGB)
	}
	if info.Compute != "sm_89" {
		t.Errorf("compute = %q, want sm_89", info.Compute)
	}
}

func TestProbeGPU_FakeVulkanOnly(t *testing.T) {
	s := fakeSourcesAllEmpty()
	s.readVendor = func() GPUVendor { return "" }
	s.hasCommand = func(name string) bool { return name == "vulkaninfo" }
	s.runCommand = func(name string, args ...string) string {
		if name == "vulkaninfo" {
			return "GPU0:\napiVersion         = 1.3.275\n"
		}
		return ""
	}

	info := probeGPU(s)
	if info.Vendor != GPUNone {
		t.Errorf("vendor = %q, want none (no sysfs/tool vendor)", info.Vendor)
	}
	if !info.HasVulkan {
		t.Error("HasVulkan should be true")
	}
	if info.Driver != "vulkan" {
		t.Errorf("driver = %q, want vulkan", info.Driver)
	}
}

// =============================================================================
// Parsers
// =============================================================================

func TestParseRocminfo_SkipsCPUAgent(t *testing.T) {
	out := `  Name:                    AMD Ryzen 7 5700X3D 8-Core Processor
  Marketing Name:          AMD Ryzen 7 5700X3D 8-Core Processor
  Vendor Name:             CPU
  Name:                    gfx1031
  Marketing Name:          AMD Radeon RX 6700 XT
  Vendor Name:             AMD`
	model, gfx := parseRocminfo(out)
	if model != "AMD Radeon RX 6700 XT" {
		t.Errorf("model = %q, want AMD Radeon RX 6700 XT", model)
	}
	if gfx != "gfx1031" {
		t.Errorf("gfx = %q, want gfx1031", gfx)
	}
}

func TestVramBytesFromRocmSMI(t *testing.T) {
	out := "GPU[0]\t\t: VRAM Total Memory (B): 12868124672\n"
	if got := vramBytesFromRocmSMI(out); got != 12868124672 {
		t.Errorf("vram bytes = %d, want 12868124672", got)
	}
	if got := vramBytesFromRocmSMI("no vram here"); got != 0 {
		t.Errorf("expected 0 on missing key, got %d", got)
	}
}

func TestGfxFromRocmSMI(t *testing.T) {
	out := "GPU[0]\t\t: GFX Version: \t\tgfx1031\n"
	if got := gfxFromRocmSMI(out); got != "gfx1031" {
		t.Errorf("gfx = %q, want gfx1031", got)
	}
}

func TestParseNvidiaVRAM(t *testing.T) {
	if got := parseNvidiaVRAM("24564 MiB"); got != 24 {
		t.Errorf("24564 MiB = %d, want 24", got)
	}
	if got := parseNvidiaVRAM("12 GiB"); got != 12 {
		t.Errorf("12 GiB = %d, want 12", got)
	}
	if got := parseNvidiaVRAM("junk"); got != 0 {
		t.Errorf("junk = %d, want 0", got)
	}
}

func TestNvidiaComputeCap(t *testing.T) {
	if got := nvidiaComputeCap("8.0"); got != "sm_80" {
		t.Errorf("8.0 = %q, want sm_80", got)
	}
	if got := nvidiaComputeCap("8.9"); got != "sm_89" {
		t.Errorf("8.9 = %q, want sm_89", got)
	}
}

func TestBytesToGB(t *testing.T) {
	if got := bytesToGB(12868124672); got != 12 {
		t.Errorf("12868124672 = %d, want 12", got)
	}
	if got := bytesToGB(1073741824); got != 1 {
		t.Errorf("1GiB = %d, want 1", got)
	}
}

// =============================================================================
// Hardware snapshot integration
// =============================================================================

func TestProbeHardware_Combined(t *testing.T) {
	snap := ProbeHardware()
	if snap.LogicalCores == 0 {
		t.Error("LogicalCores should be > 0")
	}
	if snap.ProbedAt.IsZero() {
		t.Error("ProbedAt should not be zero")
	}
	// GPU must be present (may legitimately be GPUNone in minimal environments).
	if snap.GPU.Vendor == "" {
		t.Error("GPU vendor should never be empty (expect GPUNone at minimum)")
	}
}

func TestHardwareSnapshot_MarshalWithGPU(t *testing.T) {
	snap := HardwareSnapshot{
		LogicalCores: 8,
		Load1:        1.5,
		TotalRAM:     1024,
		AvailableRAM: 512,
		GPU: GPUInfo{
			Vendor:      GPUAmd,
			Model:       "AMD Radeon RX 6700 XT",
			VRAMGB:      12,
			Driver:      "rocm-smi",
			HasROCm:     true,
			HasVulkan:   true,
			Compute:     "gfx1031",
			ROCmVersion: "7.2.4",
		},
	}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	// Existing fields unchanged (still serialized with default names).
	for _, want := range []string{`"LogicalCores":8`, `"Load1":1.5`, `"TotalRAM":1024`, `"AvailableRAM":512`} {
		if !strings.Contains(s, want) {
			t.Errorf("marshaled snapshot missing %q: %s", want, s)
		}
	}
	// New GPU field with its own JSON tags.
	for _, want := range []string{
		`"gpu"`, `"vendor":"amd"`, `"vram_gb":12`, `"has_rocm":true`,
		`"has_vulkan":true`, `"compute":"gfx1031"`, `"rocm_version":"7.2.4"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("marshaled snapshot missing %q: %s", want, s)
		}
	}
}

func TestHardwareProbe_SnapshotIncludesGPU(t *testing.T) {
	hp := NewHardwareProbe(0)
	hp.snapshot = hp.probe()
	snap := hp.Snapshot()
	if snap.GPU.Vendor == "" {
		t.Error("probe() should populate GPU (GPUNone at minimum)")
	}
}
