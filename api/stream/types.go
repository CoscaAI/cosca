// Package stream provides shared streaming infrastructure (SSE + WebSocket)
// for the Cosca REST API.
package stream

// StreamEvent represents a generic streaming event with a typed name and
// associated data payload.
type StreamEvent struct {
	Event string
	Data  interface{}
}

// SSE event type constants used across all streaming handlers.
//
// EventResponse carries LLM token content (legacy name "response" preserved
// for backward compatibility). EventToken is an alias.
//
// ── CONTRATO CANÔNICO DE EVENTOS (ETAPA 4, FASE 2) ────────────────────────────
// O wire de /v1/run/stream expõe uma sequência COERENTE de eventos para o
// Desktop e demais consumidores:
//
//	thinking   → status:processing → progress* → response* (chunks)
//	           → metadata → status:done → done          (sucesso)
//	           → error                                    (falha do provider)
//	           → cancelled                                (cliente/cancel)
//
// Tipos ADITIVOS (não quebram consumidores legados pkg/cosca + cosca-desktop,
// que só conhecem thinking/response/done/error):
//   - progress  : sub-etapa/andamento (engine progress/stage_transition).
//   - metadata  : enriquecimento do run (session_id, model, provider, agent,
//     duration_ms, token_usage) — permite header consistente.
//   - status    : sinal de fase ("processing"|"done"|"cancelled"|"error").
//
// Os campos novos são IGNORADOS por parsers existentes (JSON lax): o shape
// legado {"type":"response","content":"..."} e {"type":"done","duration_ms":N}
// permanece intacto — nenhum campo novo é injetado nesses eventos.
const (
	EventThinking  = "thinking"  // Agent is thinking/analyzing before responding
	EventResponse  = "response"  // LLM token response content (streaming tokens)
	EventToken     = "response"  // Alias for EventResponse
	EventDone      = "done"      // Stream completed successfully
	EventError     = "error"     // Stream error (in-band, does not change HTTP status)
	EventProgress  = "progress"  // Progress update (sub-stage/advancement) — additive
	EventCancelled = "cancelled" // Stream cancelled (client disconnect or explicit cancel) — FINAL state; additive
	EventMetadata  = "metadata"  // Run-level enrichment (session/model/provider/agent/duration/tokens) — additive
	EventStatus    = "status"    // Status phase signal — additive
)

// StatusPhase enumera as fases de status expostas no wire (additivo).
type StatusPhase string

const (
	// StatusProcessing sinaliza que o engine está processando a request
	// (envelope de abertura após "thinking").
	StatusProcessing StatusPhase = "processing"
	// StatusDone sinaliza conclusão bem-sucedida (antes do "done").
	StatusDone StatusPhase = "done"
	// StatusCancelled sinaliza cancelamento (estado final, antes do terminated).
	StatusCancelled StatusPhase = "cancelled"
	// StatusError sinaliza falha do provider (estado final).
	StatusError StatusPhase = "error"
)

// TokenUsage carrega uma contabilização de tokens BEST-EFFORT para o evento
// "metadata". No caminho de streaming o provider normalmente não expõe o uso
// real, então os campos são ESTIMADOS a partir de contagens de caracteres
// (≈ 1 token por 4 chars). Aditivo e não-fatal: zero quando não disponível.
type TokenUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

// MetadataEventData é o corpo canônico do evento "metadata" (aditivo). Enriquece
// o run com identidade de sessão/modelo/provider/agente e com duração/contagem
// de tokens, permitindo ao Desktop renderizar um header consistente. Consumidores
// legados que só conhecem thinking/response/done/error o ignoram (JSON lax).
type MetadataEventData struct {
	SessionID  string     `json:"session_id,omitempty"`
	Model      string     `json:"model,omitempty"`
	Provider   string     `json:"provider,omitempty"`
	Agent      string     `json:"agent,omitempty"`
	DurationMs int64      `json:"duration_ms,omitempty"`
	TokenUsage TokenUsage `json:"token_usage,omitempty"`
}

// StatusEventData é o corpo canônico do evento "status" (aditivo): apenas a
// fase corrente. Ex.: {"type":"status","status":"processing"}.
type StatusEventData struct {
	Status StatusPhase `json:"status"`
}
