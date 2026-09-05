package compute

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Fase 5 — GPU como AMBIENTE COMPUTACIONAL (6 dimensões, L318)
// =============================================================================

// TestProbeGPU_F5_AMD_DeviceNodesVisible: GPU AMD reportada E com device nodes
// visíveis (fake) → visibility true, capabilities reported=true e
// actually_visible=true, provenance high (cadeia epistemológica verificada).
func TestProbeGPU_F5_AMD_DeviceNodesVisible(t *testing.T) {
	s := fakeSourcesAllEmpty()
	s.readVendor = func() GPUVendor { return GPUAmd }
	s.hasCommand = func(name string) bool {
		return name == "rocm-smi" || name == "rocminfo"
	}
	s.stat = func(path string) bool {
		switch path {
		case "/dev/kfd", "/dev/dri/renderD128":
			return true
		}
		return false
	}
	s.listDir = func(path string) []string {
		switch path {
		case "/dev/dri":
			return []string{"renderD128", "card0", "renderD129"}
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
	s.environment = func() string { return "jail" }

	info := probeGPU(s)

	if info.Vendor != GPUAmd {
		t.Fatalf("vendor = %q, want amd", info.Vendor)
	}

	// VISIBILITY
	if !info.Visibility.Visible {
		t.Error("Visibility.Visible should be true (device nodes presentes)")
	}
	if info.Visibility.Environment != "jail" {
		t.Errorf("Visibility.Environment = %q, want jail", info.Visibility.Environment)
	}
	wantNodes := []string{"/dev/dri/renderD128", "/dev/kfd"}
	if !slices.Equal(info.Visibility.DeviceNodes, wantNodes) {
		t.Errorf("Visibility.DeviceNodes = %v, want %v", info.Visibility.DeviceNodes, wantNodes)
	}
	if len(info.Visibility.Refs) == 0 {
		t.Error("Visibility.Refs should explain the classification")
	}

	// CAPABILITIES (3 camadas)
	if !info.Capabilities.Reported {
		t.Error("Capabilities.Reported should be true (GPU reportada pelo hardware/OS)")
	}
	if !info.Capabilities.ActuallyVisible {
		t.Error("Capabilities.ActuallyVisible should be true (device nodes visíveis)")
	}
	if info.Capabilities.ExperimentallyVerified {
		t.Error("Capabilities.ExperimentallyVerified must be false (sem experimento nesta fase)")
	}
	if info.Capabilities.WhyNot == "" {
		t.Error("Capabilities.WhyNot should explain why no experimental verification")
	}

	// PROVENANCE
	if !strings.Contains(info.Provenance.Source, "rocminfo/rocm-smi") {
		t.Errorf("Provenance.Source = %q, want rocminfo/rocm-smi", info.Provenance.Source)
	}
	if !strings.Contains(info.Provenance.Source, "device-nodes(stat)") {
		t.Errorf("Provenance.Source = %q, want device-nodes(stat)", info.Provenance.Source)
	}
	if info.Provenance.Timestamp.IsZero() {
		t.Error("Provenance.Timestamp should be set")
	}
	if info.Provenance.Confidence != "high" {
		t.Errorf("Provenance.Confidence = %q, want high (runtime confirmado + device nodes visíveis)", info.Provenance.Confidence)
	}
	if info.Provenance.Evidence == "" {
		t.Error("Provenance.Evidence should carry the raw rocm-smi evidence")
	}
}

// TestProbeGPU_F5_AMD_NoDeviceNodes: GPU reportada MAS device nodes ausentes
// (stat falha) → visibility false, actually_visible=false, limitations
// registradas, confiança reduzida (detected ≠ available, L317).
func TestProbeGPU_F5_AMD_NoDeviceNodes(t *testing.T) {
	s := fakeSourcesAllEmpty()
	s.readVendor = func() GPUVendor { return GPUAmd }
	s.hasCommand = func(name string) bool {
		return name == "rocm-smi" || name == "rocminfo"
	}
	// stat falha para tudo (default de fakeSourcesAllEmpty) — sem device nodes.
	s.listDir = func(path string) []string {
		switch path {
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
	s.environment = func() string { return "container" }

	info := probeGPU(s)

	if info.Vendor != GPUAmd {
		t.Fatalf("vendor = %q, want amd", info.Vendor)
	}
	if info.Visibility.Visible {
		t.Error("Visibility.Visible should be false (nenhum device node presente)")
	}
	if len(info.Visibility.DeviceNodes) != 0 {
		t.Errorf("Visibility.DeviceNodes = %v, want empty", info.Visibility.DeviceNodes)
	}
	if info.Visibility.Environment != "container" {
		t.Errorf("Visibility.Environment = %q, want container", info.Visibility.Environment)
	}
	if !info.Capabilities.Reported {
		t.Error("Capabilities.Reported should be true (hardware/OS ainda reporta)")
	}
	if info.Capabilities.ActuallyVisible {
		t.Error("Capabilities.ActuallyVisible should be false (sem device nodes)")
	}
	if info.Provenance.Confidence != "medium" {
		t.Errorf("Provenance.Confidence = %q, want medium (reportada mas não verificável no ambiente)", info.Provenance.Confidence)
	}
	if len(info.Limitations) == 0 {
		t.Error("expected Limitations (bandwidth não observável em user space)")
	}
}

// TestProbeGPU_F5_NoGPU: nenhuma GPU → GPUNone, todas as dimensões false,
// provenance registra a ausência, confiança low, sem crash.
func TestProbeGPU_F5_NoGPU(t *testing.T) {
	info := probeGPU(fakeSourcesAllEmpty())

	if info.Vendor != GPUNone {
		t.Fatalf("vendor = %q, want none", info.Vendor)
	}
	if info.Visibility.Visible {
		t.Error("Visibility.Visible should be false without GPU")
	}
	if info.Capabilities.Reported || info.Capabilities.ActuallyVisible || info.Capabilities.ExperimentallyVerified {
		t.Error("all capabilities layers must be false without GPU")
	}
	if info.Provenance.Source == "" {
		t.Error("Provenance.Source should record the absence of a GPU")
	}
	if !strings.Contains(info.Provenance.Source, "nenhuma fonte observada") {
		t.Errorf("Provenance.Source = %q, want absence recorded", info.Provenance.Source)
	}
	if info.Provenance.Confidence != "low" {
		t.Errorf("Provenance.Confidence = %q, want low (ausência pode ser efeito do ambiente)", info.Provenance.Confidence)
	}
	if info.Provenance.Timestamp.IsZero() {
		t.Error("Provenance.Timestamp should be set even without GPU")
	}
	if info.Capabilities.WhyNot == "" {
		t.Error("Capabilities.WhyNot should explain the GPUNone state")
	}
}

// TestProbeGPU_F5_ProvenanceVulkan: provenance preenchido para uma GPU
// descoberta apenas via Vulkan (vulkaninfo) — fonte, timestamp e versão.
func TestProbeGPU_F5_ProvenanceVulkan(t *testing.T) {
	s := fakeSourcesAllEmpty()
	s.hasCommand = func(name string) bool { return name == "vulkaninfo" }
	s.runCommand = func(name string, args ...string) string {
		if name == "vulkaninfo" {
			return "Vulkan Instance Version: 1.3.275\n"
		}
		return ""
	}

	info := probeGPU(s)

	if !info.HasVulkan {
		t.Error("HasVulkan should be true")
	}
	if info.VulkanVersion != "1.3.275" {
		t.Errorf("VulkanVersion = %q, want 1.3.275", info.VulkanVersion)
	}
	if !strings.Contains(info.Provenance.Source, "vulkaninfo") {
		t.Errorf("Provenance.Source = %q, want vulkaninfo", info.Provenance.Source)
	}
	if info.Provenance.Timestamp.IsZero() {
		t.Error("Provenance.Timestamp should be set")
	}
	if info.Provenance.Confidence == "" {
		t.Error("Provenance.Confidence should be set (low expected)")
	}
	if info.Provenance.Confidence != "low" {
		t.Errorf("Provenance.Confidence = %q, want low (vulkan sem vendor identificado)", info.Provenance.Confidence)
	}
}

// TestGPUInfo_F5_JSONSerialization: os campos novos (resources/runtime/
// visibility/capabilities/provenance/limitations) serializam e fazem
// round-trip sem quebrar os campos existentes.
func TestGPUInfo_F5_JSONSerialization(t *testing.T) {
	info := GPUInfo{
		Vendor:         GPUAmd,
		Model:          "AMD Radeon RX 6700 XT",
		VRAMGB:         12,
		HasROCm:        true,
		Compute:        "gfx1031",
		ROCmVersion:    "7.2.4",
		BandwidthGBs:   512,
		VulkanVersion:  "1.3.275",
		OpenCLVersion:  "OpenCL 3.0",
		Visibility: GPUVisibility{
			Visible:     true,
			DeviceNodes: []string{"/dev/kfd", "/dev/dri/renderD128"},
			Environment: "jail",
		},
		Capabilities: GPUCapabilities{
			Reported:        true,
			ActuallyVisible: true,
			WhyNot:          "verificação experimental é fase posterior",
		},
		Provenance: GPUProvenance{
			Source:     "rocminfo/rocm-smi,device-nodes(stat)",
			Timestamp:  time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC),
			Evidence:   "GPU[0] : GFX Version: gfx1031",
			Confidence: "high",
		},
		Limitations: []string{"bandwidth não observável em user space"},
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)

	// Campos existentes (não quebrados).
	for _, want := range []string{
		`"vendor":"amd"`, `"vram_gb":12`, `"has_rocm":true`, `"rocm_version":"7.2.4"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("marshaled GPUInfo missing %q: %s", want, s)
		}
	}
	// Campos novos (Fase 5).
	for _, want := range []string{
		`"bandwidth_gbs":512`,
		`"vulkan_version":"1.3.275"`,
		`"opencl_version":"OpenCL 3.0"`,
		`"visibility":{"visible":true,"device_nodes":["/dev/kfd","/dev/dri/renderD128"],"environment":"jail"}`,
		`"capabilities":{"reported":true,"actually_visible":true,"experimentally_verified":false`,
		`"provenance":{"source":"rocminfo/rocm-smi,device-nodes(stat)","timestamp":"2026-08-16T12:00:00Z","evidence":"GPU[0] : GFX Version: gfx1031","confidence":"high"}`,
		`"limitations":["bandwidth não observável em user space"]`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("marshaled GPUInfo missing %q: %s", want, s)
		}
	}

	// Round-trip.
	var back GPUInfo
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !back.Visibility.Visible || back.Visibility.Environment != "jail" {
		t.Errorf("round-trip visibility = %+v", back.Visibility)
	}
	if !back.Capabilities.ActuallyVisible || back.Capabilities.ExperimentallyVerified {
		t.Errorf("round-trip capabilities = %+v", back.Capabilities)
	}
	if back.Provenance.Confidence != "high" || back.Provenance.Source == "" {
		t.Errorf("round-trip provenance = %+v", back.Provenance)
	}
	if back.BandwidthGBs != 512 {
		t.Errorf("round-trip bandwidth = %d, want 512", back.BandwidthGBs)
	}
}

// TestProbeGPU_F5_ReportedViaLspci_VendorUnresolved espelha o ambiente real da
// jaula: lspci DECLARA a GPU (model), mas o vendor não é resolvível (sysfs DRM
// e ferramentas ROCm/CUDA ausentes) e não há device nodes. reported=true,
// actually_visible=false, confidence medium, limitações registradas.
func TestProbeGPU_F5_ReportedViaLspci_VendorUnresolved(t *testing.T) {
	s := fakeSourcesAllEmpty()
	s.runCommand = func(name string, args ...string) string {
		if name == "lspci" {
			return `01:00.0 "VGA compatible controller" "Advanced Micro Devices, Inc. [AMD]" "Device 73bf" "PowerColor" "Device 4701" "Red Devil RX 6700 XT"`
		}
		return ""
	}

	info := probeGPU(s)

	if info.Vendor != GPUNone {
		t.Fatalf("vendor = %q, want none (unresolvable in this environment)", info.Vendor)
	}
	if info.Model == "" {
		t.Fatal("model should be filled from lspci")
	}
	if !info.Capabilities.Reported {
		t.Error("Capabilities.Reported should be true (GPU declarada por lspci)")
	}
	if info.Capabilities.ActuallyVisible {
		t.Error("Capabilities.ActuallyVisible should be false (sem device nodes)")
	}
	if info.Provenance.Confidence != "medium" {
		t.Errorf("Provenance.Confidence = %q, want medium (reportada mas não verificável no ambiente)", info.Provenance.Confidence)
	}
	if len(info.Limitations) == 0 {
		t.Error("expected Limitations (vendor não resolvível neste ambiente)")
	}
	if !strings.Contains(info.Provenance.Evidence, "VGA") {
		t.Errorf("evidence should be the lspci VGA line, got %q", info.Provenance.Evidence)
	}
}

// TestGPUInfo_F5_ZeroValue_Omitempty: um GPUInfo zero continua serializando sem
// os campos novos (additividade / backward compatibility com JSON).
func TestGPUInfo_F5_ZeroValue_Omitempty(t *testing.T) {
	data, err := json.Marshal(GPUInfo{Vendor: GPUNone})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, absent := range []string{
		"bandwidth_gbs", "vulkan_version", "opencl_version",
		"device_nodes", "environment", "evidence", "source", "confidence",
	} {
		if strings.Contains(s, absent) {
			t.Errorf("zero GPUInfo should omit %q: %s", absent, s)
		}
	}
	if !strings.Contains(s, `"vendor":"none"`) {
		t.Errorf("vendor none missing: %s", s)
	}
}
