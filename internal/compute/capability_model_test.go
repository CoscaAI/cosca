package compute

import (
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Capability Model — F7 (a estrutura central, L319)
// =============================================================================

// fakeSnapshotJailGPU constrói um snapshot realista da jaula: GPU reportada
// (RX 6700 XT via lspci/vulkan) mas NÃO visível (sem device nodes) — o caso
// epistemológico crítico do L318 (reported ≠ visible ≠ usable).
func fakeSnapshotJailGPU() HardwareSnapshot {
	return HardwareSnapshot{
		LogicalCores: 16,
		TotalRAM:     33554432 * 1024, // 32 GB
		AvailableRAM: 25165824 * 1024,
		ProbedAt:     time.Now(),
		CPU: CPUInfo{
			Vendor: CPUAMD, ModelName: "AMD Ryzen 7 5700X3D", Arch: "amd64",
			LogicalCPUs: 16, PhysicalCores: 8, Packages: 1,
			Features: map[string]bool{
				"avx2": true, "avx": true, "fma": true, "sse4_2": true,
				"bmi2": true, "aes": true, "avx512f": false,
			},
			Detected: true, Available: true, Usable: true,
		},
		Memory: MemoryInfo{
			PhysicalBytes: 33554432 * 1024, VisibleBytes: 33554432 * 1024,
			AvailableBytes: 25165824 * 1024, EffectiveBytes: 33554432 * 1024,
			PageSize: 4096, Detected: true, Available: true, Usable: true,
		},
		Environment: EnvironmentInfo{
			Type: EnvJail, Types: []EnvironmentType{EnvJail},
			InJail: true, SysfsMounted: false, GOOS: "linux", GOARCH: "amd64",
			Detected: true, Available: true, Usable: true,
		},
		GPU: GPUInfo{
			Vendor: GPUAmd, Model: "Red Devil RX 6700 XT",
			Driver: "vulkan", HasVulkan: true, HasVAAPI: true,
			VRAMGB: 12, Compute: "gfx1030",
			Visibility: GPUVisibility{
				Visible:      false, // a jaula NÃO expõe device nodes
				DeviceNodes:  []string{},
				Environment:  "jail",
			},
			Capabilities: GPUCapabilities{
				Reported: true, ActuallyVisible: false, ExperimentallyVerified: false,
			},
			Provenance: GPUProvenance{
				Source: "vulkaninfo,vainfo,lspci", Confidence: "medium",
			},
		},
		Storage: StorageInfo{
			Detected: true, Available: true, Usable: true,
			Provenance: StorageProvenance{Confidence: "medium"},
		},
	}
}

func TestBuildCapabilityModel_HappyPath(t *testing.T) {
	m := BuildCapabilityModel(fakeSnapshotJailGPU())

	if m.Environment != "jail" {
		t.Errorf("environment = %q, want jail", m.Environment)
	}
	if m.Hash == "" {
		t.Error("hash should be set")
	}
	if len(m.Capabilities) == 0 {
		t.Fatal("no capabilities derived")
	}

	// CPU cores: reportado e usável.
	c, ok := m.Get("compute.cpu.cores")
	if !ok {
		t.Fatal("compute.cpu.cores missing")
	}
	if !c.Reported || !c.Usable {
		t.Errorf("cpu.cores = reported:%v usable:%v, want true/true", c.Reported, c.Usable)
	}
	if c.Source != "cpuprobe" {
		t.Errorf("source = %q, want cpuprobe", c.Source)
	}
	if c.Confidence != "high" {
		t.Errorf("confidence = %q, want high", c.Confidence)
	}

	// SIMD: avx2 presente, avx512f ausente.
	if avx2, _ := m.Get("compute.cpu.avx2"); !avx2.Reported {
		t.Error("avx2 should be reported")
	}
	if avx512, _ := m.Get("compute.cpu.avx512f"); avx512.Reported {
		t.Error("avx512f should NOT be reported (Zen 3 não tem)")
	}
}

func TestBuildCapabilityModel_GPUReportedNotVisible(t *testing.T) {
	// O CASO CRÍTICO (L318): GPU existe no host, mas a jaula não expõe device
	// nodes. reported=true, visible=false, usable=false.
	m := BuildCapabilityModel(fakeSnapshotJailGPU())

	present, ok := m.Get("compute.gpu.present")
	if !ok {
		t.Fatal("compute.gpu.present missing")
	}
	if !present.Reported {
		t.Error("GPU should be reported (lspci/vulkan viu)")
	}
	if present.Visible {
		t.Error("GPU should NOT be visible in the jail (no device nodes)")
	}
	if present.Usable {
		t.Error("GPU should NOT be usable in the jail")
	}
	if m.Usable("compute.gpu.present") {
		t.Error("Usable() should be false for GPU in jail")
	}

	// Vulkan: reportado mas não usável na jaula.
	vk, _ := m.Get("compute.gpu.vulkan")
	if !vk.Reported {
		t.Error("vulkan should be reported (runtime presente no host)")
	}
	if vk.Usable {
		t.Error("vulkan should NOT be usable in jail (sem device nodes)")
	}

	// ROcm: não reportado no jail.
	rocm, _ := m.Get("compute.gpu.rocm")
	if rocm.Reported {
		t.Error("rocm should not be reported in jail (rocm-smi indisponível)")
	}
}

func TestBuildCapabilityModel_DeterministicHash(t *testing.T) {
	a := BuildCapabilityModel(fakeSnapshotJailGPU())
	b := BuildCapabilityModel(fakeSnapshotJailGPU())
	if a.Hash != b.Hash {
		t.Errorf("hash não determinístico: %q vs %q", a.Hash, b.Hash)
	}
}

func TestCapabilityModel_NamesSorted(t *testing.T) {
	m := BuildCapabilityModel(fakeSnapshotJailGPU())
	names := m.Names()
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Errorf("names not sorted: %v", names)
		}
	}
}

func TestCapabilityModel_ProvenanceFields(t *testing.T) {
	m := BuildCapabilityModel(fakeSnapshotJailGPU())
	for _, name := range m.Names() {
		c := m.Capabilities[name]
		if c.Source == "" {
			t.Errorf("capability %q: source vazio", name)
		}
		if c.Source == "llm" {
			t.Errorf("capability %q: source NUNCA pode ser llm (L320)", name)
		}
		if c.Environment == "" {
			t.Errorf("capability %q: environment vazio", name)
		}
		if c.Timestamp.IsZero() {
			t.Errorf("capability %q: timestamp zero", name)
		}
	}
}

func TestCapabilityModel_SetOverrides(t *testing.T) {
	m := NewCapabilityModel("bare-metal")
	m.Set(Capability{Name: "x", Status: StatusVisible, Source: "test", Confidence: "high"})
	c, ok := m.Get("x")
	if !ok {
		t.Fatal("x missing")
	}
	if c.Environment != "bare-metal" {
		t.Errorf("environment = %q, want bare-metal (default do modelo)", c.Environment)
	}
	if c.Timestamp.IsZero() {
		t.Error("timestamp should be set by Set()")
	}

	// Sobrescrever mantém ordem estável.
	m.Set(Capability{Name: "x", Status: StatusUsable, Source: "test2", Confidence: "high"})
	c2, _ := m.Get("x")
	if c2.Status != StatusUsable {
		t.Errorf("status = %q, want usable (sobrescrito)", c2.Status)
	}
}

func TestCapabilityModel_JSONRoundtrip(t *testing.T) {
	m := BuildCapabilityModel(fakeSnapshotJailGPU())
	data, err := m.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		`"capabilities"`, `"compute.cpu.cores"`, `"compute.gpu.present"`,
		`"environment":"jail"`, `"hash"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing %q", want)
		}
	}
}
