package task

import "time"

// Esta arquivo carrega o envelope do Evento de task — a linguagem neutra entre
// o runtime (data plane) e o TaskOrchestrator (control plane).
//
// O cosca-trader modelava o envelope genérico em `internal/event` (o bus do
// domínio de trading). O Root NÃO tem um pacote `internal/event` equivalente, e
// criar um bus genérico aqui seria escopo demais para a F1. A decisão (ADR-015)
// é: a primitiva carrega o próprio envelope MÍNIMO (`Event`), stdlib-only, que
// o ADAPTER de domínio mapeia para o transporte de eventos real do Root (ex.:
// `durable.Event`, `pipeline.DurableEvent`) quando composto. Assim a primitiva
// permanece pura e auto-contida.

// EventType identifica o tipo de evento de task do Task Continuation Loop.
type EventType string

const (
	// EventTaskStarted — a task foi iniciada (primeiro evento do ciclo).
	EventTaskStarted EventType = "task.started"
	// EventPositionChanged — o snapshot do data plane mudou (Artifacts).
	EventPositionChanged EventType = "task.position_changed"
	// EventIntentGenerated — o runtime gerou uma intenção (pending action).
	EventIntentGenerated EventType = "task.intent_generated"
	// EventRiskCheck — o data plane reporta o veredito de risco (watchdog fail-closed).
	EventRiskCheck EventType = "task.risk_check"
	// EventExecution — um passo de execução foi observado.
	EventExecution EventType = "task.execution"
	// EventFill — um resultado parcial/fill foi aplicado (checkpoint idempotente).
	EventFill EventType = "task.fill"
	// EventStateChanged — o status do ciclo de vida mudou.
	EventStateChanged EventType = "task.state_changed"
	// EventTaskContinue — o Control Plane sinaliza a continuação.
	EventTaskContinue EventType = "task.continue"
)

// Severidade do evento (paridade com `event.Severity*` do cosca-trader).
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

// Event é a unidade da comunicação entre os planes (Task Continuation Loop).
// `Source` identifica o emissor (ex.: "worker:code", "provider:deepseek",
// "oms", "risk"); `CorrelationID` agrupa os eventos de uma mesma task (equivale
// ao TaskID); `Payload` é um dos payloads tipados deste pacote (ou um
// map[string]any contendo "task_id").
type Event struct {
	ID            string    `json:"id"`
	Type          EventType `json:"type"`
	Timestamp     time.Time `json:"timestamp"`
	Source        string    `json:"source"`
	Payload       any       `json:"payload,omitempty"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	CausationID   string    `json:"causation_id,omitempty"`
	Severity      string    `json:"severity"`
}

// ── Payloads dos eventos de task ─────────────────────────────────────────────
// São o contrato entre o Runtime (data plane) e o TaskOrchestrator (control
// plane). Tipados para não circular JSON solto e para o CorrelationID bater com
// o TaskID.

// StartedPayload é o payload de EventTaskStarted.
type StartedPayload struct {
	Task TaskState `json:"task"`
}

// PositionChangedPayload é o payload de EventPositionChanged. O snapshot do
// data plane entra como `Artifacts` genérico (chave/valor) — o adapter converteu
// o domínio para essas chaves.
type PositionChangedPayload struct {
	TaskID    TaskID            `json:"task_id"`
	Artifacts map[string]string `json:"artifacts"`
}

// IntentGeneratedPayload é o payload de EventIntentGenerated.
type IntentGeneratedPayload struct {
	TaskID TaskID `json:"task_id"`
	Intent string `json:"intent"`
}

// RiskCheckPayload é o payload de EventRiskCheck (o data plane reporta o
// veredito de risco — um bloqueio marca o watchdog fail-closed).
type RiskCheckPayload struct {
	TaskID  TaskID `json:"task_id"`
	Blocked bool   `json:"blocked"`
	Reason  string `json:"reason,omitempty"`
}

// ExecutionPayload é o payload de EventExecution.
type ExecutionPayload struct {
	TaskID TaskID `json:"task_id"`
	Detail string `json:"detail,omitempty"`
}

// FillPayload é o payload de EventFill. O snapshot do data plane entra como
// `Artifacts` genérico (chave/valor) — o adapter converteu o domínio para essas
// chaves. O orquestrador registra um checkpoint idempotente ("fill") quando um
// fill é aplicado.
type FillPayload struct {
	TaskID    TaskID            `json:"task_id"`
	Artifacts map[string]string `json:"artifacts"`
}

// StateChangedPayload é o payload de EventStateChanged.
type StateChangedPayload struct {
	TaskID TaskID     `json:"task_id"`
	Status TaskStatus `json:"status"`
}

// ContinuePayload é o payload de EventTaskContinue (o Control Plane sinaliza a
// continuação; o Runtime consome para re-injetar o prompt de continuação).
type ContinuePayload struct {
	TaskID TaskID             `json:"task_id"`
	Reason ContinuationReason `json:"reason,omitempty"`
}
