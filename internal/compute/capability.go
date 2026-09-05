//
// Capability Profile versionado — o "Machine Profile" da Cosca.
//
// O perfil de capacidade congelas as capacidades de inferência da máquina em
// .cosca/machine/profile.json (cpu / memória / gpu / capabilities) com um hash
// SHA-256 estável. Se o hardware mudar (ex.: trocar a GPU), o DiffProfiles
// detecta "OLD PROFILE → hardware diff → NEW PROFILE" e avisa que o backend de
// inferência precisa ser reavaliado — evitando reinstalar tudo.
//
// Regra do Don: nada de sobrescrever às cegas. O `cosca machine profile` só
// grava quando o diff de capacidade detecta mudança real (ou quando não existe
// perfil salvo).
//

package compute

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// =============================================================================
// Capability Profile
// =============================================================================

// CapabilityChangedWarning é a mensagem emitida quando a capacidade da máquina
// muda — o backend de inferência precisa ser reavaliado (regra do Don).
const CapabilityChangedWarning = "Capacidade mudou — backend de inferência precisa ser reavaliado."

// CapabilityProfile é a fotografia das capacidades de inferência da máquina.
// O campo Hash é o SHA-256 do JSON do perfil SEM os campos hash/probed_at —
// determinístico e estável para a mesma capacidade de hardware.
type CapabilityProfile struct {
	CPU struct {
		Cores   int `json:"cores"`
		Threads int `json:"threads"`
	} `json:"cpu"`
	Memory struct {
		TotalGB     float64 `json:"total_gb"`
		AvailableGB float64 `json:"available_gb"`
	} `json:"memory"`
	GPU          GPUInfo   `json:"gpu"`
	Capabilities []string  `json:"capabilities"` // e.g. ["avx2","gpu_compute","rocm","vulkan"]
	Hash         string    `json:"hash"`         // SHA-256 do perfil sem o campo hash
	ProbedAt     time.Time `json:"probed_at"`
}

// BuildCapabilityProfile deriva o perfil de capacidade a partir de um snapshot
// de hardware. A derivação de capabilities é determinística (ordem fixa), o
// que garante um hash estável.
func BuildCapabilityProfile(snap HardwareSnapshot) CapabilityProfile {
	p := CapabilityProfile{}

	cores := physicalCores()
	if cores == 0 {
		cores = snap.LogicalCores
	}
	p.CPU.Cores = cores
	p.CPU.Threads = snap.LogicalCores

	p.Memory.TotalGB = roundToOneDecimal(float64(snap.TotalRAM) / (1 << 30))
	p.Memory.AvailableGB = roundToOneDecimal(float64(snap.AvailableRAM) / (1 << 30))

	p.GPU = snap.GPU
	p.Capabilities = deriveCapabilities(snap)
	p.ProbedAt = snap.ProbedAt
	p.Hash = p.computeHash()
	return p
}

// deriveCapabilities deriva a lista de capabilities de inferência a partir do
// snapshot de hardware.
//
//	| Condição                          | Capability     |
//	|-----------------------------------|----------------|
//	| GOARCH == amd64                   | avx2           |
//	| GPU.Vendor != GPUNone             | gpu_compute    |
//	| GPU.Vendor != GPUNone             | gpu_exec       |
//	| GPU.HasROCm                       | rocm           |
//	| GPU.HasVulkan                     | vulkan         |
//	| GPU.HasCUDA                       | cuda           |
//	| GPU.VRAMGB >= 4                   | gpu_tts        |
//	| RAM total >= 16 GB                | large_memory   |
//
// gpu_tts é uma heurística: VRAM >= 4 GB é suficiente para executar um backend
// de TTS local (ex.: Piper/torch base). Ela é documentada aqui e validada
// apenas pelo diff de capacidade — nunca por presença de binários.
//
// gpu_exec indica que há um backend de inferência GPU utilizável (o executor
// Ollama com backend ROCm). Ela é derivada apenas da presença de GPU — a
// disponibilidade real do servidor é checada em runtime pelo executor.
func deriveCapabilities(snap HardwareSnapshot) []string {
	var caps []string
	if runtime.GOARCH == "amd64" {
		caps = append(caps, "avx2") // best-effort: a maioria dos x86-64 modernos suporta AVX2
	}
	if snap.GPU.Vendor != GPUNone {
		caps = append(caps, "gpu_compute")
		caps = append(caps, "gpu_exec") // backend de inferência GPU disponível (Ollama ROCm)
	}
	if snap.GPU.HasROCm {
		caps = append(caps, "rocm")
	}
	if snap.GPU.HasVulkan {
		caps = append(caps, "vulkan")
	}
	if snap.GPU.HasCUDA {
		caps = append(caps, "cuda")
	}
	if snap.GPU.VRAMGB >= 4 {
		caps = append(caps, "gpu_tts") // heurística: VRAM >= 4 GB para TTS local
	}
	if float64(snap.TotalRAM) >= 16*(1<<30) {
		caps = append(caps, "large_memory")
	}
	return caps
}

// computeHash retorna o SHA-256 do perfil de CAPACIDADE estável. São excluídos
// os campos transitórios/de controle:
//
//   - hash      — nunca pode ser parte do próprio hash;
//   - probed_at — timestamp do probe (não é capacidade);
//   - available_gb — memória LIVRE flutua a cada probe (cache/buffers). Se
//     entrasse no hash, o hash mudaria a cada execução — contrariando o objetivo
//     do Don: identificar troca de hardware (ex.: GPU) de forma estável.
//
// Campos flutuantes restantes (total_gb) são arredondados na montagem
// (BuildCapabilityProfile), mantendo o hash determinístico.
func (p *CapabilityProfile) computeHash() string {
	type cpuHash struct {
		Cores   int `json:"cores"`
		Threads int `json:"threads"`
	}
	type memHash struct {
		TotalGB float64 `json:"total_gb"`
	}
	payload := struct {
		CPU          cpuHash  `json:"cpu"`
		Memory       memHash  `json:"memory"`
		GPU          GPUInfo  `json:"gpu"`
		Capabilities []string `json:"capabilities"`
	}{
		CPU:          cpuHash{Cores: p.CPU.Cores, Threads: p.CPU.Threads},
		Memory:       memHash{TotalGB: p.Memory.TotalGB},
		GPU:          p.GPU,
		Capabilities: p.Capabilities,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// physicalCores conta os núcleos físicos a partir do /proc/cpuinfo (par
// "physical id"+"core id"). Retorna 0 quando indisponível (não-Linux).
func physicalCores() int {
	lines := readFileLines("/proc/cpuinfo")
	seen := make(map[string]struct{})
	phys := ""
	for _, line := range lines {
		fields := strings.SplitN(line, ":", 2)
		if len(fields) != 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		val := strings.TrimSpace(fields[1])
		switch key {
		case "physical id":
			phys = val
		case "core id":
			seen[phys+"|"+val] = struct{}{}
		}
	}
	return len(seen)
}

// roundToOneDecimal arredonda um valor para uma casa decimal, mantendo o JSON
// (e portanto o hash) determinístico diante de flutuações pequenas.
func roundToOneDecimal(v float64) float64 {
	return math.Round(v*10) / 10
}

// =============================================================================
// MachineStore — .cosca/machine/profile.json
// =============================================================================

// MachineStore persiste o CapabilityProfile em .cosca/machine/profile.json com
// escrita atômica (temp + rename) — nunca deixa um arquivo corrompido para trás.
type MachineStore struct {
	dir  string
	path string
}

// NewMachineStore cria um MachineStore sob o diretório .cosca da aplicação.
func NewMachineStore(coscaDir string) *MachineStore {
	return &MachineStore{
		dir:  filepath.Join(coscaDir, "machine"),
		path: filepath.Join(coscaDir, "machine", "profile.json"),
	}
}

// Exists reporta se o profile.json já existe no disco.
func (s *MachineStore) Exists() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

// Save grava o perfil de forma atômica (arquivo temporário + rename).
func (s *MachineStore) Save(p CapabilityProfile) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("criar diretório do perfil: %w", err)
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar perfil: %w", err)
	}

	tmp, err := os.CreateTemp(s.dir, "profile-*.tmp")
	if err != nil {
		return fmt.Errorf("criar arquivo temporário: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op após o rename bem-sucedido

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("escrever perfil: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync do perfil: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fechar perfil: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("renomear perfil: %w", err)
	}
	return nil
}

// Load lê o profile.json. Retorna (nil, nil) quando o arquivo não existe.
func (s *MachineStore) Load() (*CapabilityProfile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var p CapabilityProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse do perfil: %w", err)
	}
	return &p, nil
}

// =============================================================================
// DiffProfiles — hardware diff
// =============================================================================

// DiffProfiles compara a capacidade efetiva de dois perfis (GPU vendor/model/
// VRAM/ROCm/CUDA, RAM total e núcleos). Retorna se mudou e as linhas do resumo
// em pt-BR. A última linha (quando mudou) é o aviso CapabilityChangedWarning.
func DiffProfiles(old, new CapabilityProfile) (changed bool, summary []string) {
	if old.GPU.Vendor != new.GPU.Vendor || old.GPU.Model != new.GPU.Model {
		changed = true
		summary = append(summary, fmt.Sprintf("GPU: %s → %s", gpuLabel(old.GPU), gpuLabel(new.GPU)))
	}
	if old.GPU.VRAMGB != new.GPU.VRAMGB {
		changed = true
		summary = append(summary, fmt.Sprintf("VRAM: %d GB → %d GB", old.GPU.VRAMGB, new.GPU.VRAMGB))
	}
	if old.GPU.HasROCm != new.GPU.HasROCm {
		changed = true
		summary = append(summary, fmt.Sprintf("ROCm: %s → %s", enabledLabel(old.GPU.HasROCm), enabledLabel(new.GPU.HasROCm)))
	}
	if old.GPU.HasCUDA != new.GPU.HasCUDA {
		changed = true
		summary = append(summary, fmt.Sprintf("CUDA: %s → %s", enabledLabel(old.GPU.HasCUDA), enabledLabel(new.GPU.HasCUDA)))
	}
	if old.Memory.TotalGB != new.Memory.TotalGB {
		changed = true
		summary = append(summary, fmt.Sprintf("RAM: %.1f GB → %.1f GB", old.Memory.TotalGB, new.Memory.TotalGB))
	}
	if old.CPU.Cores != new.CPU.Cores {
		changed = true
		summary = append(summary, fmt.Sprintf("CPU: %d núcleos → %d núcleos", old.CPU.Cores, new.CPU.Cores))
	}
	if changed {
		summary = append(summary, CapabilityChangedWarning)
	}
	return changed, summary
}

// gpuLabel produz o rótulo legível de uma GPU para o diff (modelo, ou vendor).
func gpuLabel(g GPUInfo) string {
	if g.Model != "" {
		return g.Model
	}
	return string(g.Vendor)
}

func enabledLabel(b bool) string {
	if b {
		return "habilitado"
	}
	return "desabilitado"
}
