//
// Tests for the Capability Profile (internal/compute/capability.go).
//
// Covers:
//   - BuildCapabilityProfile: capabilities derivadas corretamente
//   - Hash estável: mesmo perfil → mesmo hash; mudança de GPU → hash diferente
//   - MachineStore Save/Load round-trip; escrita atômica; Load quando ausente → nil
//   - DiffProfiles: igual → unchanged; troca de GPU → changed com resumo
//

package compute

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// baseSnapshot monta um HardwareSnapshot determinístico para os testes.
func baseSnapshot() HardwareSnapshot {
	return HardwareSnapshot{
		LogicalCores: 16,
		TotalRAM:     32 * (1 << 30),
		AvailableRAM: 20 * (1 << 30),
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
		ProbedAt: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC),
	}
}

// =============================================================================
// BuildCapabilityProfile — derivação de capabilities
// =============================================================================

func TestBuildCapabilityProfile_CapabilitiesDerived(t *testing.T) {
	p := BuildCapabilityProfile(baseSnapshot())

	for _, want := range []string{"gpu_compute", "rocm", "vulkan", "gpu_tts", "large_memory"} {
		if !contains(p.Capabilities, want) {
			t.Errorf("capabilities missing %q: %v", want, p.Capabilities)
		}
	}

	if p.GPU.Model != "AMD Radeon RX 6700 XT" {
		t.Errorf("gpu model = %q", p.GPU.Model)
	}
	if p.CPU.Threads != 16 {
		t.Errorf("threads = %d, want 16", p.CPU.Threads)
	}
	if p.CPU.Cores == 0 {
		t.Error("cores should be > 0")
	}
	if p.Memory.TotalGB != 32 {
		t.Errorf("total_gb = %.1f, want 32", p.Memory.TotalGB)
	}
	if p.Hash == "" {
		t.Error("hash should not be empty")
	}
}

func TestBuildCapabilityProfile_NoGPU_OmitsComputeCaps(t *testing.T) {
	snap := baseSnapshot()
	snap.GPU = GPUInfo{Vendor: GPUNone, HasROCm: false, HasVulkan: false, HasCUDA: false, VRAMGB: 0}
	p := BuildCapabilityProfile(snap)

	for _, want := range []string{"gpu_compute", "rocm", "vulkan", "cuda", "gpu_tts"} {
		if contains(p.Capabilities, want) {
			t.Errorf("capabilities should NOT contain %q: %v", want, p.Capabilities)
		}
	}
	// large_memory ainda deve estar presente (RAM >= 16GB).
	if !contains(p.Capabilities, "large_memory") {
		t.Errorf("expected large_memory: %v", p.Capabilities)
	}
}

func TestBuildCapabilityProfile_GPUttsHeuristic(t *testing.T) {
	snap := baseSnapshot()
	snap.GPU.VRAMGB = 3
	if contains(BuildCapabilityProfile(snap).Capabilities, "gpu_tts") {
		t.Error("gpu_tts should be absent when VRAM < 4 GB")
	}
	snap.GPU.VRAMGB = 4
	if !contains(BuildCapabilityProfile(snap).Capabilities, "gpu_tts") {
		t.Error("gpu_tts should be present when VRAM >= 4 GB")
	}
}

func TestBuildCapabilityProfile_SmallMemoryOmitsLargeMemory(t *testing.T) {
	snap := baseSnapshot()
	snap.TotalRAM = 8 * (1 << 30)
	if contains(BuildCapabilityProfile(snap).Capabilities, "large_memory") {
		t.Error("large_memory should be absent when RAM < 16 GB")
	}
}

// =============================================================================
// Hash — estável para a mesma capacidade, diferente quando a GPU muda
// =============================================================================

func TestCapabilityProfileHash_StableAndSensitiveToGPU(t *testing.T) {
	snap := baseSnapshot()

	first := BuildCapabilityProfile(snap)
	second := BuildCapabilityProfile(snap)
	if first.Hash != second.Hash {
		t.Errorf("hash should be stable: %s != %s", first.Hash, second.Hash)
	}
	if len(first.Hash) != 64 {
		t.Errorf("hash should be a 64-char SHA-256 hex, got %d", len(first.Hash))
	}

	// Trocar a GPU (mesmo timestamp de probe) deve mudar o hash.
	snap2 := baseSnapshot()
	snap2.GPU = GPUInfo{Vendor: GPUNvidia, Model: "NVIDIA GeForce RTX 3060", VRAMGB: 12, HasCUDA: true}
	changed := BuildCapabilityProfile(snap2)
	if changed.Hash == first.Hash {
		t.Error("hash should change when the GPU changes")
	}

	// ProbedAt diferente (mas mesma capacidade) NÃO deve mudar o hash.
	snap3 := baseSnapshot()
	snap3.ProbedAt = snap3.ProbedAt.Add(24 * time.Hour)
	if h := BuildCapabilityProfile(snap3).Hash; h != first.Hash {
		t.Errorf("hash should ignore probed_at: %s != %s", h, first.Hash)
	}
}

func TestCapabilityProfileHash_IgnoresAvailableRAM(t *testing.T) {
	// Memória livre flutua a cada probe — NÃO pode afetar o hash de capacidade
	// (senão o hash mudaria a cada execução e não detectaria troca de GPU).
	base := BuildCapabilityProfile(baseSnapshot())

	snap := baseSnapshot()
	snap.AvailableRAM = 15 * (1 << 30) // 20GB → 15GB livres
	other := BuildCapabilityProfile(snap)

	if base.Hash != other.Hash {
		t.Errorf("hash should ignore available_gb: %s != %s", base.Hash, other.Hash)
	}
}

func TestCapabilityProfileHash_DiffersWhenRAMOrCoresChange(t *testing.T) {
	base := BuildCapabilityProfile(baseSnapshot())

	// RAM total diferente → hash diferente.
	snapRAM := baseSnapshot()
	snapRAM.TotalRAM = 64 * (1 << 30)
	if h := BuildCapabilityProfile(snapRAM).Hash; h == base.Hash {
		t.Error("hash should change when total RAM changes")
	}

	// Cores diferentes → hash diferente.
	snapCores := baseSnapshot()
	snapCores.LogicalCores = 8
	if h := BuildCapabilityProfile(snapCores).Hash; h == base.Hash {
		t.Error("hash should change when core count changes")
	}
}

// =============================================================================
// MachineStore — round-trip, atomicidade, Load ausente
// =============================================================================

func TestMachineStore_SaveLoadRoundTrip(t *testing.T) {
	store := NewMachineStore(t.TempDir())
	p := BuildCapabilityProfile(baseSnapshot())

	if store.Exists() {
		t.Fatal("store should not exist before Save")
	}
	if err := store.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !store.Exists() {
		t.Fatal("store should exist after Save")
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil {
		t.Fatal("Load returned nil after Save")
	}
	if got.Hash != p.Hash {
		t.Errorf("round-trip hash = %s, want %s", got.Hash, p.Hash)
	}
	if got.GPU.Model != p.GPU.Model {
		t.Errorf("round-trip gpu model = %q, want %q", got.GPU.Model, p.GPU.Model)
	}
	if len(got.Capabilities) != len(p.Capabilities) {
		t.Errorf("round-trip capabilities = %v, want %v", got.Capabilities, p.Capabilities)
	}
}

func TestMachineStore_AtomicWriteLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewMachineStore(dir)
	p := BuildCapabilityProfile(baseSnapshot())

	if err := store.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// A escrita atômica (temp + rename) não pode deixar arquivos temporários.
	entries, err := os.ReadDir(filepath.Join(dir, "machine"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("atomic Save left a temp file behind: %s", e.Name())
		}
	}

	// Sobrescrever também é atômico e válido.
	p2 := p
	p2.GPU = GPUInfo{Vendor: GPUNvidia, Model: "NVIDIA GeForce RTX 3060"}
	p2.Hash = p2.computeHash()
	if err := store.Save(p2); err != nil {
		t.Fatalf("overwrite Save: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load after overwrite: %v", err)
	}
	if got.GPU.Model != "NVIDIA GeForce RTX 3060" {
		t.Errorf("overwrite not persisted: %q", got.GPU.Model)
	}
}

func TestMachineStore_LoadWhenMissingReturnsNil(t *testing.T) {
	store := NewMachineStore(t.TempDir())
	if store.Exists() {
		t.Fatal("store should not exist")
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load when missing: %v", err)
	}
	if got != nil {
		t.Fatalf("Load when missing should return nil, got %+v", got)
	}
}

func TestMachineStore_CorruptFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	store := NewMachineStore(dir)
	if err := os.MkdirAll(filepath.Join(dir, "machine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Error("Load on corrupt file should return an error")
	}
}

// =============================================================================
// DiffProfiles
// =============================================================================

func TestDiffProfiles_Unchanged(t *testing.T) {
	old := BuildCapabilityProfile(baseSnapshot())
	new := BuildCapabilityProfile(baseSnapshot())

	changed, summary := DiffProfiles(old, new)
	if changed {
		t.Errorf("expected unchanged, got changed with summary %v", summary)
	}
	if len(summary) != 0 {
		t.Errorf("expected empty summary, got %v", summary)
	}
}

func TestDiffProfiles_GPUChange(t *testing.T) {
	old := BuildCapabilityProfile(baseSnapshot())
	snap := baseSnapshot()
	snap.GPU = GPUInfo{Vendor: GPUNvidia, Model: "NVIDIA GeForce RTX 3060", VRAMGB: 12, HasCUDA: true}
	new := BuildCapabilityProfile(snap)

	changed, summary := DiffProfiles(old, new)
	if !changed {
		t.Fatal("expected changed=true on GPU swap")
	}
	joined := strings.Join(summary, "\n")
	if !strings.Contains(joined, "AMD Radeon RX 6700 XT") || !strings.Contains(joined, "NVIDIA GeForce RTX 3060") {
		t.Errorf("summary should mention both GPUs: %v", summary)
	}
	if !strings.Contains(joined, CapabilityChangedWarning) {
		t.Errorf("summary missing capability-changed warning: %v", summary)
	}
	if summary[len(summary)-1] != CapabilityChangedWarning {
		t.Errorf("warning should be the last summary line: %v", summary)
	}
}

func TestDiffProfiles_RAMAndCoresChange(t *testing.T) {
	// Construído manualmente: o DiffProfiles compara CPU.Cores (núcleos físicos)
	// e Memory.TotalGB — independentemente da derivação via BuildCapabilityProfile.
	old := BuildCapabilityProfile(baseSnapshot())

	new := old
	new.CPU.Cores = old.CPU.Cores + 4
	new.Memory.TotalGB = old.Memory.TotalGB + 32
	new.Hash = new.computeHash()

	changed, summary := DiffProfiles(old, new)
	if !changed {
		t.Fatal("expected changed=true")
	}
	joined := strings.Join(summary, "\n")
	if !strings.Contains(joined, "RAM:") {
		t.Errorf("summary missing RAM line: %v", summary)
	}
	if !strings.Contains(joined, "CPU:") {
		t.Errorf("summary missing CPU line: %v", summary)
	}
}

func TestDiffProfiles_ROCmEnabledChange(t *testing.T) {
	old := BuildCapabilityProfile(baseSnapshot())

	snap := baseSnapshot()
	snap.GPU.HasROCm = false
	new := BuildCapabilityProfile(snap)

	changed, summary := DiffProfiles(old, new)
	if !changed {
		t.Fatal("expected changed=true when ROCm availability changes")
	}
	if !strings.Contains(strings.Join(summary, "\n"), "ROCm: habilitado → desabilitado") {
		t.Errorf("summary missing ROCm line: %v", summary)
	}
}

// =============================================================================
// Helpers
// =============================================================================

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
