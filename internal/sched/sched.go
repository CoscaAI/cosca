// Package sched implementa o Scheduler de Execução (§28 do manifesto
// Creative/Scientific/Media) — Fase 1, etapa 1.5.
//
// O scheduler cruza a classe de hardware de uma TASK (aitask.HardwareClass)
// com o probe real da máquina (compute.GPUInfo) e decide ONDE a tarefa roda:
// local CPU, local GPU (ROCm/Vulkan/OpenCL/VAAPI) ou remoto (API).
//
// Decisão por: performance · custo · disponibilidade · qualidade (§28).
// Sempre determinístico e testável: os cenários são puros.
package sched

import (
	"fmt"

	"github.com/CoscaAI/cosca/internal/aitask"
	"github.com/CoscaAI/cosca/internal/compute"
)

// Target é o destino de execução decidido pelo scheduler.
type Target string

// Destinos possíveis.
const (
	// TargetLocalCPU — roda na CPU da máquina.
	TargetLocalCPU Target = "local-cpu"
	// TargetLocalGPU — roda na GPU local (ROCm/Vulkan/OpenCL/VAAPI).
	TargetLocalGPU Target = "local-gpu"
	// TargetRemote — executa via API/provider remoto.
	TargetRemote Target = "remote"
)

// Reason explica por que o scheduler escolheu o destino.
type Reason string

// Razões comuns (fonte de verdade para logs/observabilidade §27).
const (
	ReasonGPUPresent      Reason = "gpu present and task wants gpu"
	ReasonNoGPU           Reason = "no gpu available — falling back to cpu"
	ReasonTaskPrefersCPU  Reason = "task prefers cpu"
	ReasonTaskWantsRemote Reason = "task wants remote execution"
	ReasonGPULowVRAM      Reason = "gpu vram below task requirement"
	ReasonNoModel         Reason = "no local model registered — remote"
)

// Decision é o resultado do scheduler.
type Decision struct {
	Target Target `json:"target" yaml:"target"`
	Reason Reason `json:"reason" yaml:"reason"`
	// GPU reflete o hardware considerado (para observabilidade).
	GPU compute.GPUInfo `json:"gpu" yaml:"gpu"`
	// MinVRAMBytes é o requisito mínimo de VRAM considerado (0 = nenhum).
	MinVRAMBytes int64 `json:"min_vram_bytes,omitempty" yaml:"min_vram_bytes,omitempty"`
}

// String devolve a representação legível da decisão.
func (d Decision) String() string {
	return fmt.Sprintf("%s (%s)", d.Target, d.Reason)
}

// Scheduler decide onde uma task roda.
type Scheduler struct {
	// gpuProvider devolve o estado da GPU (injetável para testes).
	gpuProvider func() compute.GPUInfo
	// hasLocalModel reports se existe modelo LOCAL registrado para a task.
	hasLocalModel func(task aitask.Type) bool
}

// Option configura o scheduler.
type Option func(*Scheduler)

// WithGPUProvider injeta a fonte de dados da GPU (padrão: compute.ProbeGPU).
func WithGPUProvider(fn func() compute.GPUInfo) Option {
	return func(s *Scheduler) { s.gpuProvider = fn }
}

// WithLocalModelCheck injeta a checagem de modelo local (padrão: sempre true).
func WithLocalModelCheck(fn func(aitask.Type) bool) Option {
	return func(s *Scheduler) { s.hasLocalModel = fn }
}

// New cria um scheduler com defaults.
func New(opts ...Option) *Scheduler {
	s := &Scheduler{
		gpuProvider:  compute.ProbeGPU,
		hasLocalModel: func(aitask.Type) bool { return true },
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Decide escolhe o destino de execução para uma task (§28).
func (s *Scheduler) Decide(task aitask.Type, minVRAMBytes int64) Decision {
	t, ok := aitask.Get(task)
	gpu := s.gpuProvider()
	if !ok {
		// Tarefa desconhecida → remoto (defensivo).
		return Decision{Target: TargetRemote, Reason: ReasonTaskWantsRemote, GPU: gpu}
	}

	switch t.Hardware {
	case aitask.HWRemote:
		// Tarefas que o manifesto classifica como remotas (LLM/VLM) só vão
		// para a GPU local se houver modelo local registrado E GPU disponível.
		if s.hasLocalModel(task) && gpu.Vendor != compute.GPUNone {
			return Decision{Target: TargetLocalGPU, Reason: ReasonGPUPresent, GPU: gpu, MinVRAMBytes: minVRAMBytes}
		}
		return Decision{Target: TargetRemote, Reason: ReasonTaskWantsRemote, GPU: gpu, MinVRAMBytes: minVRAMBytes}

	case aitask.HWGPU:
		if gpu.Vendor == compute.GPUNone {
			return Decision{Target: TargetLocalCPU, Reason: ReasonNoGPU, GPU: gpu, MinVRAMBytes: minVRAMBytes}
		}
		if minVRAMBytes > 0 && gpu.VRAMGB > 0 && int64(gpu.VRAMGB)<<30 < minVRAMBytes {
			return Decision{Target: TargetLocalCPU, Reason: ReasonGPULowVRAM, GPU: gpu, MinVRAMBytes: minVRAMBytes}
		}
		return Decision{Target: TargetLocalGPU, Reason: ReasonGPUPresent, GPU: gpu, MinVRAMBytes: minVRAMBytes}

	case aitask.HWCpu:
		// Tarefas leves preferem CPU — GPU só se já estiver ativa e o modelo
		// local existir (custo/performance: não acordar GPU para tarefa leve).
		if s.hasLocalModel(task) && gpu.Vendor != compute.GPUNone {
			return Decision{Target: TargetLocalGPU, Reason: ReasonGPUPresent, GPU: gpu, MinVRAMBytes: minVRAMBytes}
		}
		return Decision{Target: TargetLocalCPU, Reason: ReasonTaskPrefersCPU, GPU: gpu, MinVRAMBytes: minVRAMBytes}

	default: // HWAny
		if gpu.Vendor != compute.GPUNone && s.hasLocalModel(task) {
			return Decision{Target: TargetLocalGPU, Reason: ReasonGPUPresent, GPU: gpu, MinVRAMBytes: minVRAMBytes}
		}
		return Decision{Target: TargetLocalCPU, Reason: ReasonNoGPU, GPU: gpu, MinVRAMBytes: minVRAMBytes}
	}
}

// DecideModel escolhe o destino para uma task específica de MODELO (com VRAM
// conhecida do Model Registry). minVRAM vem do registro do modelo.
func (s *Scheduler) DecideModel(task aitask.Type, modelVRAMBytes int64) Decision {
	return s.Decide(task, modelVRAMBytes)
}
