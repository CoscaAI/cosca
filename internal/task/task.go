// Package task é o CONTRATO do Task Continuation Loop (ADR-015): o TaskState
// estruturado — a fonte de verdade do ciclo de vida de uma task — e o modelo de
// dados compartilhado entre o Cognitive Control Plane (Kernel, Task Orchestrator,
// Agents, Memory/OWM, Policy/Risk) e o Operational Data Plane (executores,
// workers, provedores, strategists).
//
// A interface entre os dois planes é ESTE contrato + os eventos de task; NUNCA
// uma chamada direta Kernel→executor. O pacote `task` é a linguagem neutra de
// módulos: quem decide lê TaskState; quem executa publica eventos que atualizam
// o TaskState.
//
// Herança provada no handoff do cosca-trader (internal/autopilot/handoff.go):
//   - dinheiro em decimal.Decimal (nunca float no caminho de corretude);
//   - estado terminal (COMPLETE/ABORTED) limpa o watchdog (dead-man switch);
//   - idempotência por checkpoint (etapa já concluída não é re-executada);
//   - transição ilegal → ErrIllegalTransition (HTTP 409).
//
// IMPORTANTE (isolamento de módulos): este pacote é NEUTRO de domínio. Ele NÃO
// importa `internal/workers`, `internal/editors`, `internal/desktop`,
// `internal/providers` nem `internal/pipeline` — essas são fronteiras do
// ADAPTER (a borda de domínio injeta a implementação de `TaskRepository` e o
// mapeamento de payloads de domínio). O snapshot do data plane entra aqui como
// um mapa genérico chave/valor (`Artifacts`); quem sabe o significado das chaves
// (ex.: "side"="buy", "open"="true") é o adapter, que converte o objeto de
// domínio nessas chaves.
package task

import (
	"errors"
	"fmt"
	"strings"
)

// Chaves do snapshot genérico do data plane (Artifacts). São a linguagem neutra
// entre o runtime (produz) e o orquestrador (lê): o ADAPTER é quem converte um
// objeto de domínio (ex.: domain.Position) nestas chaves. O pacote `task`
// continua agnóstico — não importa nem sabe o que é uma "posição de trading",
// apenas o contrato de chaves.
const (
	ArtifactSymbol = "symbol" // qual instrumento o snapshot descreve
	ArtifactSide   = "side"   // sentido ("buy" | "sell" | ...)
	ArtifactOpen   = "open"   // "true" | "false" — o snapshot está "em curso"?
	ArtifactQty    = "qty"    // quantidade/largura (string decimal)
	ArtifactPrice  = "price"  // preço de referência (string decimal)
)

// TaskID identifica uma task — a mesma identidade que amarra os eventos da task
// no bus via CorrelationID (ADR-015: "reuso do CorrelationID/CausationID").
type TaskID string

// TaskStatus é o estado do ciclo de vida de uma task.
type TaskStatus string

const (
	StatusActive   TaskStatus = "ACTIVE"   // rodando/avançando
	StatusWaiting  TaskStatus = "WAITING"  // aguardando insumo/evento externo
	StatusPaused   TaskStatus = "PAUSED"   // pausado por risco/política
	StatusComplete TaskStatus = "COMPLETE" // objetivo satisfeito (terminal)
	StatusAborted  TaskStatus = "ABORTED"  // abortada (terminal)
)

// ContinuationReason é o MOTIVO COGNITIVO pelo qual o Control Plane decide
// CONTINUE — nunca por "passo a passo"; só há retomada quando um destes
// gatilhos aparece (ADR-015, seção "Task Continuation Loop").
type ContinuationReason string

const (
	ContIncompleteObjective ContinuationReason = "INCOMPLETE_OBJECTIVE" // objetivo ainda não satisfeito
	ContStepLimit           ContinuationReason = "STEP_LIMIT"           // limite de continuações atingido → escala p/ o Don
	ContInputRequired       ContinuationReason = "INPUT_REQUIRED"       // a task aguarda insumo do Kernel/dono
	ContWatchdog            ContinuationReason = "WATCHDOG"             // dead-man switch marcado (exceção/erro/risco)
)

// TaskObjective é a descrição do objetivo. `Direction` é a direção
// ("buy" | "sell"); `Deadline` é o limite do watchdog (string ISO RFC3339 —
// em F2 o orchestrator passa a interpretá-lo; em F1 o watchdog é disparado por
// evento de risco/erro ou por CheckWatchdog).
type TaskObjective struct {
	ID        string `json:"id"`                 // idempotency key / correlation do executor
	Objective string `json:"objective"`          // instrução de alto nível (ex.: "aumentar exposição em BTCUSDT")
	Symbol    string `json:"symbol"`             // ex.: "BTCUSDT"
	Direction string `json:"direction"`          // "buy" | "sell"
	Deadline  string `json:"deadline,omitempty"` // time.Time; deadline de watchdog (RFC3339)
}

// TaskState é o CONTRATO entre os planes — a fonte de verdade do ciclo de vida.
//
// É genérico/neutro de domínio: em vez de carregar um tipo de posição de
// trading, carrega um `Artifacts` (mapa chave/valor) — o snapshot do data plane.
// O ADAPTER converte o domínio nesse mapa. A consistência é eventual: o
// TaskState pode estar uma leitura atrás da realidade do data plane (trade-off
// documentado na ADR-015), o que o "snapshot" documenta.
type TaskState struct {
	TaskID             TaskID             `json:"task_id"`
	Objective          TaskObjective      `json:"objective"`
	Status             TaskStatus         `json:"status"`
	CurrentStep        int                `json:"current_step"`
	Agent              string             `json:"agent"`
	PolicyVersion      string             `json:"policy_version"`
	Checkpoints        []string           `json:"checkpoints"`     // etapas concluídas (idempotência da retomada)
	PendingActions     []string           `json:"pending_actions"` // ações aguardando execução
	Observations       []string           `json:"observations"`    // leituras/evidências do runtime
	Errors             []string           `json:"errors,omitempty"`
	ContinuationReason ContinuationReason `json:"continuation_reason,omitempty"`
	// Artifacts é o snapshot genérico do data plane (chave/valor). O ADAPTER
	// traduz objetos de domínio para estas chaves. As chaves semânticas vivem
	// nas constantes Artifact* deste pacote.
	Artifacts map[string]string `json:"artifacts,omitempty"`
}

// Clone devolve uma cópia independente (slices e mapa de artifacts copiados) —
// leituras seguras sem o lock do orquestrador.
func (s TaskState) Clone() TaskState {
	c := s
	c.Checkpoints = append([]string(nil), s.Checkpoints...)
	c.PendingActions = append([]string(nil), s.PendingActions...)
	c.Observations = append([]string(nil), s.Observations...)
	c.Errors = append([]string(nil), s.Errors...)
	c.Artifacts = cloneArtifacts(s.Artifacts)
	return c
}

// ArtifactValue devolve o valor de uma chave do snapshot ("", se ausente) —
// leitura segura de um campo possivelmente nil.
func (s TaskState) ArtifactValue(key string) string {
	return s.Artifacts[key]
}

func cloneArtifacts(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Erros do contrato.
var (
	// ErrIllegalTransition é uma transição não permitida (HTTP 409).
	ErrIllegalTransition = errors.New("transição ilegal de estado (409)")
	// ErrInvalidObjective é um objetivo malformado.
	ErrInvalidObjective = errors.New("objetivo de task inválido")
)

// legalTransitions define o grafo do ciclo de vida. Estados terminais não saem
// para lugar nenhum. Uma task PAUSADA pode voltar a ACTIVE (retomada); que estava
// WAITING volta quando o insumo chegar.
var legalTransitions = map[TaskStatus][]TaskStatus{
	StatusActive:   {StatusWaiting, StatusPaused, StatusComplete, StatusAborted},
	StatusWaiting:  {StatusActive, StatusPaused, StatusComplete, StatusAborted},
	StatusPaused:   {StatusActive, StatusComplete, StatusAborted},
	StatusComplete: {},
	StatusAborted:  {},
}

// IsTerminal devolve se o status é um estado de saída (COMPLETE/ABORTED). Estado
// terminal limpa o watchdog — espelho do isTerminal do handoff.
func IsTerminal(s TaskStatus) bool {
	return s == StatusComplete || s == StatusAborted
}

// LegalTransition valida uma transição `from → to`. Mesmo estado é no-op
// idempotente (re-envio do mesmo passo não duplica — como o handoff). Transição
// ilegal (skip, ida para trás, ou sair de um estado terminal) → ErrIllegalTransition.
func LegalTransition(from, to TaskStatus) error {
	if from == to {
		return nil // idempotente
	}
	if IsTerminal(from) {
		return errors.Join(ErrIllegalTransition, fmt.Errorf("%q → %q: estado terminal não sai para lugar nenhum", from, to))
	}
	for _, n := range legalTransitions[from] {
		if n == to {
			return nil
		}
	}
	return errors.Join(ErrIllegalTransition, fmt.Errorf("%q → %q é uma transição ilegal", from, to))
}

// ValidateObjective valida o contrato tipado do objetivo (espelha ValidateSpec do
// handoff): ID, Symbol e Direction obrigatórios; Direction ∈ buy|sell.
func ValidateObjective(obj TaskObjective) error {
	if strings.TrimSpace(obj.ID) == "" {
		return errors.Join(ErrInvalidObjective, errors.New("ID obrigatório (idempotency key / correlation)"))
	}
	if strings.TrimSpace(obj.Symbol) == "" {
		return errors.Join(ErrInvalidObjective, errors.New("symbol obrigatório"))
	}
	switch obj.Direction {
	case "buy", "sell":
	default:
		return errors.Join(ErrInvalidObjective, fmt.Errorf("direction deve ser buy|sell, veio %q", obj.Direction))
	}
	return nil
}

// Valid devolve se a ContinuationReason é conhecida.
func (r ContinuationReason) Valid() bool {
	switch r {
	case ContIncompleteObjective, ContStepLimit, ContInputRequired, ContWatchdog:
		return true
	}
	return false
}
