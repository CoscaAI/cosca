// Package compute — FASE 7 DO HARDWARE & PERFORMANCE BRAIN: CAPABILITY MODEL.
//
// O Professor (L319): o Cosca deixa de ter "CPU=Ryzen, GPU=RX 6700 XT" e passa
// a ter uma MATRIZ de capabilities onde CADA UMA carrega: status, source,
// evidence, confidence, environment, timestamp. É a estrutura central que liga
// as descobertas F1-F6 (FACT) ao que o runtime pode realmente usar (USABLE).
//
// A INVARIANTE (L320): uma etapa não fabrica evidência para a anterior.
// - O FACT (hardware declara) vem dos probes F1-F6.
// - O MEASURED (benchmark observa) vem do Benchmark Contract (F9).
// - O capability model NÃO inventa capability: deriva reported/visible/usable
//   a partir do que os probes observaram, com proveniência.
// - "LLM afirma X → X vira capability" é PROIBIDO.
//
// Cada capability tem uma CADEIA (L318/L319): reported → visible → usable →
// measured. NUNCA assumir que o hardware existe = o processo pode usar.
package compute

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CapabilityStatus é o status observado de uma capability (a cadeia do
// Professor, L318/L319 + a especificação completa do L324).
type CapabilityStatus string

const (
	// StatusUnknown: não sabemos (ausência de evidência ≠ evidência de
	// ausência — L324 invariante: Unknown ≠ False).
	StatusUnknown CapabilityStatus = "unknown"
	// StatusReported: o hardware/OS declarou a capability (FACT observado).
	StatusReported CapabilityStatus = "reported"
	// StatusInferred: derivado por regra de inferência EXPLÍCITA (ex: amd64 →
	// avx2 plausível). NUNCA tratado como verificado (L324 regra de ouro).
	StatusInferred CapabilityStatus = "inferred"
	// StatusVisible: o processo consegue VER a capability neste ambiente
	// (device nodes, sysfs, ferramentas presentes).
	StatusVisible CapabilityStatus = "visible"
	// StatusMeasured: o benchmark observou a capability (F9 contract).
	StatusMeasured CapabilityStatus = "measured"
	// StatusVerified: o runtime VERIFICOU por execução real (probe ativo).
	StatusVerified CapabilityStatus = "verified"
	// StatusUsable: a capability é utilizável pelo workload (verificada e
	// com o runtime necessário).
	StatusUsable CapabilityStatus = "usable"
	// StatusUnavailable: determinada como indisponível neste ambiente
	// (ex: GPU reportada mas sem device nodes no jail → não visível).
	StatusUnavailable CapabilityStatus = "unavailable"
	// StatusUnverified: não foi possível verificar (falta de evidência) —
	// NUNCA assumir; registrar e seguir (epistemologia L317).
	StatusUnverified CapabilityStatus = "unverified"
	// StatusInvalidated: sabíamos, mas a evidência não é mais válida
	// (ex: driver mudou — L324). Impede que memória antiga ressuscite estado.
	StatusInvalidated CapabilityStatus = "invalidated"
	// StatusStale: foi verdadeiro quando observado, mas a validade expirou
	// (L324 temporalidade — observed_at/expires_at).
	StatusStale CapabilityStatus = "stale"
)

// Capability é UMA linha da matriz de capacidades do Cosca (L319).
// Cada capability carrega a proveniência completa — é isso que impede o
// Capability Brain de virar catálogo de palpites.
type Capability struct {
	// Name identifica a capability (ex: "compute.cpu.avx2", "compute.gpu.rocm",
	// "memory.capacity", "storage.nvme").
	Name string `json:"name" yaml:"name"`

	// Status é o nível mais alto da cadeia alcançado com evidência.
	Status CapabilityStatus `json:"status" yaml:"status"`

	// Reported/Visible/Usable/Measured: a cadeia completa (L318). Cada campo
	// é independente — a GPU pode ser reported=true, visible=false (jail),
	// usable=false, measured=false.
	Reported bool `json:"reported" yaml:"reported"`
	Visible  bool `json:"visible" yaml:"visible"`
	Usable   bool `json:"usable" yaml:"usable"`
	Measured bool `json:"measured" yaml:"measured"`

	// Source: de onde veio a informação (ex: "cpuprobe", "gpuprobe",
	// "environmentprobe", "benchmark-contract"). NUNCA "llm".
	Source string `json:"source" yaml:"source"`

	// Evidence: o dado observado que sustenta o status (ex: "flags: avx2",
	// "device /dev/kfd presente", "rocm-smi indisponível no jail").
	Evidence string `json:"evidence,omitempty" yaml:"evidence,omitempty"`

	// Confidence: quão confiável é esta capability (L316).
	Confidence string `json:"confidence" yaml:"confidence"`

	// Environment: o contexto onde foi observada (ex: "jail", "bare-metal",
	// "container") — a mesma capability pode ter status diferente por ambiente.
	Environment string `json:"environment" yaml:"environment"`

	// Timestamp: quando foi observada.
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`

	// ObservedAt/ExpiresAt: temporalidade (L324). ExpiresAt zero = sem
	// expiração declarada; quando passado, o status deve ser tratado como
	// Stale (foi verdadeiro quando observado, sem evidência de que continua).
	ObservedAt time.Time `json:"observed_at,omitempty" yaml:"observed_at,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`

	// DerivedFrom/InferenceRule: herança EXPLÍCITA (L324 cuidado 12). Quando
	// status=inferred, registra de qual capability foi derivada e por qual
	// regra — ninguém confunde inferência com observação.
	DerivedFrom   string `json:"derived_from,omitempty" yaml:"derived_from,omitempty"`
	InferenceRule string `json:"inference_rule,omitempty" yaml:"inference_rule,omitempty"`

	// WhyUnknown: a razão do UNKNOWN (L325 princípio 10 — o Cosca deve provar
	// que não sabe, não apenas "não sei"). Preenchido quando Status é unknown
	// ou unverified. Transforma UNKNOWN em algo explicável, não um buraco.
	WhyUnknown string `json:"why_unknown,omitempty" yaml:"why_unknown,omitempty"`

	// FailedEvidence: falha TAMBÉM é evidência (L325 princípio 5). Registra a
	// última tentativa falhada (reason) — impede repetir experimentos que já
	// demonstraram não funcionar.
	FailedEvidence string `json:"failed_evidence,omitempty" yaml:"failed_evidence,omitempty"`

	// Details: campos extras específicos da capability (ex: VRAM para gpu,
	// bytes para memory) — opcional, JSON.
	Details map[string]string `json:"details,omitempty" yaml:"details,omitempty"`
}

// WhyUnknown reasons canônicos (L325 princípio 10).
const (
	// WhyNotObserved: nada observou esta capability.
	WhyNotObserved = "not_observed"
	// WhyInaccessible: existe, mas inacessível neste ambiente.
	WhyInaccessible = "inaccessible"
	// WhyProbeFailed: a sonda falhou.
	WhyProbeFailed = "probe_failed"
	// WhyStale: foi verdadeiro, mas a validade expirou.
	WhyStale = "stale"
	// WhyConflictingEvidence: evidências se contradizem.
	WhyConflictingEvidence = "conflicting_evidence"
	// WhyEnvironmentMismatch: observado em ambiente diferente do atual.
	WhyEnvironmentMismatch = "environment_mismatch"
	// WhyInsufficientAuthority: sem autoridade para verificar.
	WhyInsufficientAuthority = "insufficient_authority"
)

// IsStale reports se a capability expirou (L324 temporalidade).
func (c *Capability) IsStale(now time.Time) bool {
	return !c.ExpiresAt.IsZero() && now.After(c.ExpiresAt)
}

// Promote é a REGRA DE OURO do F7 (L324): nenhum estado epistemológico
// inferior pode promover a si próprio para um estado superior. Promover
// exige evidência NOVA — o chamador declara qual.
//
//	INFERRED  → só com evidência de medição → MEASURED
//	MEASURED  → só com verificação por execução → VERIFIED
//	VERIFIED  → só com confirmação de utilização → USABLE
//
// Retorna error se a promoção não é permitida (não muda o estado).
func (c *Capability) Promote(to CapabilityStatus, evidence string) error {
	allowed := map[CapabilityStatus]CapabilityStatus{
		StatusInferred: StatusMeasured,  // inferência precisa de medição
		StatusMeasured: StatusVerified,  // medição precisa de verificação
		StatusVerified: StatusUsable,    // verificação precisa de uso confirmado
	}
	expected, ok := allowed[c.Status]
	if !ok {
		return fmt.Errorf("promote: %q não pode promover a si próprio (regra de ouro L324)", c.Status)
	}
	if to != expected {
		return fmt.Errorf("promote: %q → %q não permitido (esperado %q)", c.Status, to, expected)
	}
	if evidence == "" {
		return fmt.Errorf("promote: evidência nova obrigatória para %q → %q (L324)", c.Status, to)
	}
	c.Status = to
	c.Evidence = evidence
	c.Timestamp = time.Now().UTC()
	return nil
}

// Invalidate marca a capability como INVALIDATED (L324): sabíamos, mas a
// evidência não é mais válida (ex: driver mudou).
func (c *Capability) Invalidate(reason string) {
	c.Status = StatusInvalidated
	c.Evidence = reason
	c.Timestamp = time.Now().UTC()
}

// RecordFailure registra uma tentativa FALHADA como evidência (L325 princípio
// 5: falha também é evidência). A capability fica unknown/unverified com
// WhyUnknown=probe_failed e o motivo preservado — para o sistema não repetir
// eternamente experimentos que já demonstraram não funcionar.
func (c *Capability) RecordFailure(reason string) {
	if c.Status == StatusUsable || c.Status == StatusVerified {
		c.Status = StatusUnverified // a falha derruba a certeza anterior
	}
	if c.Status == StatusUnknown || c.Status == StatusUnverified || c.Status == StatusUnavailable {
		c.Status = StatusUnverified
	}
	c.WhyUnknown = WhyProbeFailed
	c.FailedEvidence = reason
	c.Timestamp = time.Now().UTC()
}

// ImpactLevel classifica o impacto de uma capability (L325 princípio 7:
// evidência proporcional ao impacto).
type ImpactLevel string

const (
	// ImpactLow: ex. listar diretório — evidência simples basta.
	ImpactLow ImpactLevel = "low"
	// ImpactMedium: ex. ler arquivo, buscar conhecimento — evidência de
	// observação suficiente.
	ImpactMedium ImpactLevel = "medium"
	// ImpactHigh: ex. executar código, alterar sistema, publicar, rede,
	// alterar memória — exige múltiplas evidências + contexto + política.
	ImpactHigh ImpactLevel = "high"
)

// EvidenceSufficient reports se a evidência atual é suficiente para o nível de
// impacto (L325 princípio 7). High exige status verified/usable (verificação
// real), não apenas reported/visible.
func (c *Capability) EvidenceSufficient(impact ImpactLevel) bool {
	switch impact {
	case ImpactLow:
		return c.Status == StatusVisible || c.Status == StatusMeasured ||
			c.Status == StatusVerified || c.Status == StatusUsable
	case ImpactMedium:
		return c.Status == StatusMeasured || c.Status == StatusVerified || c.Status == StatusUsable
	case ImpactHigh:
		return c.Status == StatusVerified || c.Status == StatusUsable
	default:
		return false
	}
}

// CapabilityModel é a matriz de capacidades do Cosca (F7 — a estrutura
// central, L319). Consolida as descobertas F1-F6 em capabilities com
// proveniência, prontas para o Workload Model (F8) comparar requisitos.
type CapabilityModel struct {
	// Capabilities é a matriz indexada por nome.
	Capabilities map[string]Capability `json:"capabilities" yaml:"capabilities"`
	// Environment é o contexto onde o modelo foi construído (F4).
	Environment string `json:"environment" yaml:"environment"`
	// BuiltAt é quando o modelo foi construído.
	BuiltAt time.Time `json:"built_at" yaml:"built_at"`
	// HardwareFingerprint identifica o hardware (L315) para correlação.
	HardwareFingerprint string `json:"hardware_fingerprint,omitempty" yaml:"hardware_fingerprint,omitempty"`
	// Hash é o SHA-256 determinístico do modelo (sem hash/built_at) — para
	// detecção de mudança de capacidade (o CapabilityChangedWarning existe).
	Hash string `json:"hash" yaml:"hash"`
}

// NewCapabilityModel cria um modelo vazio.
func NewCapabilityModel(env string) *CapabilityModel {
	return &CapabilityModel{
		Capabilities: map[string]Capability{},
		Environment:  env,
		BuiltAt:      time.Now().UTC(),
	}
}

// Set adiciona/substitui uma capability na matriz.
func (m *CapabilityModel) Set(c Capability) {
	if c.Environment == "" {
		c.Environment = m.Environment
	}
	if c.Timestamp.IsZero() {
		c.Timestamp = time.Now().UTC()
	}
	m.Capabilities[c.Name] = c
}

// Get devolve uma capability pelo nome.
func (m *CapabilityModel) Get(name string) (Capability, bool) {
	c, ok := m.Capabilities[name]
	return c, ok
}

// Names devolve os nomes das capabilities em ordem estável (determinismo).
func (m *CapabilityModel) Names() []string {
	names := make([]string, 0, len(m.Capabilities))
	for n := range m.Capabilities {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Usable reports se uma capability está utilizável (nível máximo da cadeia).
func (m *CapabilityModel) Usable(name string) bool {
	c, ok := m.Capabilities[name]
	return ok && c.Usable
}

// Visible reports se uma capability está visível no ambiente atual.
func (m *CapabilityModel) Visible(name string) bool {
	c, ok := m.Capabilities[name]
	return ok && c.Visible
}

// =============================================================================
// BuildCapabilityModel — deriva a matriz a partir do snapshot (F1-F6)
// =============================================================================

// BuildCapabilityModel constrói a matriz de capabilities a partir do snapshot
// de hardware (F1-F6). A derivação é determinística (ordem fixa) para hash
// estável. Nenhuma capability é inventada: tudo deriva do que os probes
// observaram, com proveniência.
func BuildCapabilityModel(snap HardwareSnapshot) *CapabilityModel {
	env := string(snap.Environment.Type)
	if env == "" {
		env = "unknown"
	}
	m := NewCapabilityModel(env)
	m.HardwareFingerprint = fingerprintFromSnapshot(snap)
	m.addCPUCapabilities(snap)
	m.addMemoryCapabilities(snap)
	m.addGPUCapabilities(snap)
	m.addStorageCapabilities(snap)
	m.addEnvironmentCapabilities(snap)
	m.Hash = m.computeHash()
	return m
}

// addCPUCapabilities deriva as capabilities de CPU com base no CPUInfo (F1).
func (m *CapabilityModel) addCPUCapabilities(snap HardwareSnapshot) {
	cpu := snap.CPU
	env := m.Environment

	// compute.cpu.cores — FACT do probe F1.
	m.Set(Capability{
		Name: "compute.cpu.cores", Status: StatusVisible,
		Reported: cpu.LogicalCPUs > 0, Visible: cpu.LogicalCPUs > 0,
		Usable: cpu.Available && cpu.LogicalCPUs > 0,
		Source: "cpuprobe", Evidence: itoaI(cpu.LogicalCPUs) + " logical",
		Confidence: "high", Environment: env,
		Details: map[string]string{"physical": itoaI(cpu.PhysicalCores), "logical": itoaI(cpu.LogicalCPUs)},
	})

	// compute.cpu.simd — as features ISA observadas no /proc/cpuinfo (FACT).
	for _, feat := range []string{"avx2", "avx", "fma", "sse4_2", "bmi2", "aes"} {
		name := "compute.cpu." + feat
		has := cpu.Features[feat]
		status := StatusUnavailable
		if has && cpu.Available {
			status = StatusVisible
		} else if !has {
			status = StatusUnavailable
		}
		m.Set(Capability{
			Name: name, Status: status,
			Reported: has, Visible: has && cpu.Available,
			Usable: has && cpu.Available,
			Source: "cpuprobe",
			Evidence: map[bool]string{true: "flags: " + feat, false: "flag ausente"}[has],
			Confidence: "high", Environment: env,
		})
	}
}

// addMemoryCapabilities deriva as capabilities de memória (F2).
func (m *CapabilityModel) addMemoryCapabilities(snap HardwareSnapshot) {
	mem := snap.Memory
	env := m.Environment

	m.Set(Capability{
		Name: "memory.capacity", Status: StatusVisible,
		Reported: mem.PhysicalBytes > 0, Visible: mem.VisibleBytes > 0,
		Usable: mem.AvailableBytes > 0,
		Source: "memoryprobe",
		Evidence: formatBytes(uint64(mem.PhysicalBytes)) + " physical / " +
			formatBytes(uint64(mem.AvailableBytes)) + " available",
		Confidence: "high", Environment: env,
		Details: map[string]string{
			"physical":  itoaI64(mem.PhysicalBytes),
			"visible":   itoaI64(mem.VisibleBytes),
			"available": itoaI64(mem.AvailableBytes),
			"effective": itoaI64(mem.EffectiveBytes),
		},
	})
}

// addGPUCapabilities deriva as capabilities de GPU com as 3 camadas (F5/L318).
func (m *CapabilityModel) addGPUCapabilities(snap HardwareSnapshot) {
	gpu := snap.GPU
	env := m.Environment

	// GPU reportada? (o hardware existe — lspci/vulkaninfo)
	gpuReported := gpu.Vendor != GPUNone || gpu.Model != ""
	// GPU visível? (device nodes presentes neste ambiente — F5 visibility)
	gpuVisible := gpu.Visibility.Visible

	// compute.gpu.present — a cadeia completa.
	m.Set(Capability{
		Name: "compute.gpu.present",
		Status: capStatus(gpuReported, gpuVisible, gpuVisible),
		Reported: gpuReported, Visible: gpuVisible, Usable: gpuVisible,
		Source: "gpuprobe",
		Evidence: map[bool]string{
			true: "device nodes visíveis (" + strings.Join(gpu.Visibility.DeviceNodes, ", ") + ")",
			false: "GPU reportada mas device nodes ausentes neste ambiente",
		}[gpuVisible],
		Confidence: gpu.Provenance.Confidence, Environment: env,
		Details: map[string]string{"model": gpu.Model, "vendor": string(gpu.Vendor)},
	})

	// compute.gpu.rocm / compute.gpu.vulkan / compute.gpu.opencl — por backend.
	for _, b := range []struct {
		name string
		has  bool
	}{
		{"compute.gpu.rocm", gpu.HasROCm},
		{"compute.gpu.vulkan", gpu.HasVulkan},
		{"compute.gpu.opencl", gpu.HasOpenCL},
		{"compute.gpu.cuda", gpu.HasCUDA},
	} {
		// Backend reportado + GPU visível = usável; reportado sem visível =
		// não usável neste ambiente; não reportado = unavailable.
		usable := b.has && gpuVisible
		status := StatusUnavailable
		if b.has && !gpuVisible {
			status = StatusUnavailable // reportado mas não visível no jail
		} else if usable {
			status = StatusUsable
		}
		m.Set(Capability{
			Name: b.name, Status: status,
			Reported: b.has, Visible: gpuVisible, Usable: usable,
			Source: "gpuprobe",
			Evidence: map[bool]string{
				true: "runtime detectado", false: "runtime não detectado",
			}[b.has],
			Confidence: gpu.Provenance.Confidence, Environment: env,
		})
	}

	// compute.gpu.vram — quando observável.
	if gpu.VRAMGB > 0 {
		m.Set(Capability{
			Name: "compute.gpu.vram", Status: StatusVisible,
			Reported: true, Visible: gpuVisible, Usable: gpuVisible,
			Source: "gpuprobe", Evidence: itoaI(gpu.VRAMGB) + " GB",
			Confidence: gpu.Provenance.Confidence, Environment: env,
			Details: map[string]string{"vram_gb": itoaI(gpu.VRAMGB)},
		})
	}
}

// addStorageCapabilities deriva as capabilities de storage (F6).
func (m *CapabilityModel) addStorageCapabilities(snap HardwareSnapshot) {
	st := snap.Storage
	env := m.Environment

	// storage.available — existe filesystem utilizável?
	visible := st.Detected && len(st.Mounts) > 0
	m.Set(Capability{
		Name: "storage.available", Status: capStatus(st.Detected, visible, visible),
		Reported: st.Detected, Visible: visible, Usable: visible,
		Source: "storageprobe",
		Evidence: itoaI(len(st.Mounts)) + " mounts",
		Confidence: st.Provenance.Confidence, Environment: env,
	})
}

// addEnvironmentCapabilities deriva as capabilities de ambiente (F4).
func (m *CapabilityModel) addEnvironmentCapabilities(snap HardwareSnapshot) {
	envInfo := snap.Environment
	env := m.Environment

	// environment.jail — o processo está na jaula (afeta TODAS as capabilities).
	m.Set(Capability{
		Name: "environment.jail", Status: StatusVisible,
		Reported: envInfo.InJail, Visible: envInfo.InJail, Usable: envInfo.InJail,
		Source: "environmentprobe",
		Evidence: map[bool]string{true: "COSCA_JAILED=1", false: "fora da jaula"}[envInfo.InJail],
		Confidence: "high", Environment: env,
	})

	// environment.sysfs — sysfs de topologia montado? (afeta F3).
	m.Set(Capability{
		Name: "environment.sysfs", Status: capStatus(true, envInfo.SysfsMounted, envInfo.SysfsMounted),
		Reported: true, Visible: envInfo.SysfsMounted, Usable: envInfo.SysfsMounted,
		Source: "environmentprobe",
		Evidence: map[bool]string{
			true: "sysfs de topologia montado", false: "sysfs não exposto (topologia parcial)",
		}[envInfo.SysfsMounted],
		Confidence: "high", Environment: env,
	})
}

// =============================================================================
// Helpers
// =============================================================================

// capStatus deriva o status mais alto da cadeia a partir dos booleans.
func capStatus(reported, visible, usable bool) CapabilityStatus {
	switch {
	case usable:
		return StatusUsable
	case visible:
		return StatusVisible
	case reported:
		return StatusUnavailable // reportado mas não visível/usável neste ambiente
	default:
		return StatusUnavailable
	}
}

// fingerprintFromSnapshot deriva um fingerprint técnico do snapshot (L315) —
// sem dados pessoais, só perfil técnico.
func fingerprintFromSnapshot(snap HardwareSnapshot) string {
	parts := []string{
		snap.CPU.Vendor.String(),
		snap.CPU.ModelName,
		snap.CPU.Arch,
		itoaI(snap.CPU.LogicalCPUs),
		itoaI(snap.CPU.PhysicalCores),
	}
	return strings.Join(parts, "|")
}

// computeHash calcula o SHA-256 determinístico do modelo (sem hash/built_at).
func (m *CapabilityModel) computeHash() string {
	// Serializa as capabilities em ordem estável de nome.
	var sb strings.Builder
	for _, name := range m.Names() {
		c := m.Capabilities[name]
		sb.WriteString(name + "=" + string(c.Status) +
			";r=" + boolStr(c.Reported) +
			";v=" + boolStr(c.Visible) +
			";u=" + boolStr(c.Usable) +
			";m=" + boolStr(c.Measured) + "\n")
	}
	sb.WriteString("env=" + m.Environment + "\n")
	sb.WriteString("fp=" + m.HardwareFingerprint + "\n")
	return sha256Hex(sb.String())
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func itoaI(n int) string {
	return itoa(n)
}

func itoaI64(n int64) string {
	// O itoa do pacote aceita int; converte (valores de bytes cabem em int
	// nesta plataforma; para uint64 grande, usa strconv).
	if n >= 0 && n <= int64(^uint(0)>>1) {
		return itoa(int(n))
	}
	return strconv.FormatInt(n, 10)
}// sha256Hex calcula o SHA-256 hex de uma string (para o hash do modelo).
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// MarshalJSON serializa o modelo com as tags JSON das structs.
func (m *CapabilityModel) MarshalJSON() ([]byte, error) {
	type alias CapabilityModel
	return json.Marshal((*alias)(m))
}

// String devolve o nome legível do vendor de CPU.
func (v CPUVendor) String() string {
	return string(v)
}
