package stream

import (
	"github.com/CoscaAI/cosca/internal/orchestration"
)

// WireEvent é o resultado canônico do mapper engine→wire: o nome do evento SSE
// no wire e o data payload a ser escrito via SSEWriter.WriteEvent.
type WireEvent struct {
	Type string
	Data interface{}
}

// EngineMapper é a ÚNICA camada centralizada de tradução entre o
// orchestration.StreamEvent do engine e o contrato SSE canônico do wire
// (thinking/response/done/error/cancelled + progress/metadata/status aditivos).
//
// Ele carrega os campos request-scoped (session, model, provider, agent) que
// enriquecem os eventos envelope (thinking/metadata/done). Nenhuma lógica de
// mapeamento engine→wire deve viver no handler — tudo passa por aqui
// (ETAPA 4, FASE 2).
type EngineMapper struct {
	SessionID string
	Model     string
	Provider  string
	Agent     string
}

// NewEngineMapper constrói um EngineMapper vinculado aos campos request-scoped.
func NewEngineMapper(sessionID, model, provider, agent string) *EngineMapper {
	return &EngineMapper{SessionID: sessionID, Model: model, Provider: provider, Agent: agent}
}

// Map traduz um StreamEvent do engine para o evento canônico do wire.
//
// É a fonte única de verdade do mapeamento engine→wire:
//
//	chunk            → response        (nome legado preservado p/ consumidores)
//	progress         → progress        (aditivo; antes era engolido)
//	stage_transition → progress        (aditivo; antes era engolido)
//	error            → error           (terminal — o handler aplica side effects)
//	cancelled        → cancelled       (terminal — o handler aplica side effects)
//	<desconhecido>   → nil             (conservador: nunca quebra o stream)
//
// Retorna nil quando o tipo do engine não tem representação no wire.
func (m *EngineMapper) Map(ev orchestration.StreamEvent) *WireEvent {
	switch ev.Type {
	case orchestration.StreamEventChunk:
		// String data → WriteEvent(EventResponse, string) emite o shape legado
		// {"type":"response","content":"..."} — preservado.
		return &WireEvent{Type: EventResponse, Data: ev.Content}

	case orchestration.StreamEventProgress, orchestration.StreamEventStageTransition:
		return &WireEvent{Type: EventProgress, Data: m.progressPayload(ev)}

	case orchestration.StreamEventError:
		// Terminal: o handler decide como escrever (WriteError) + side effects.
		return &WireEvent{Type: EventError, Data: ev.Content}

	case orchestration.StreamEventCancelled:
		// Terminal: o handler escreve o estado final (WriteCancelled/WriteError
		// via writeStreamTermination). Data nil sinaliza "não escreva via
		// WriteEvent" (formatSSEEvent com nil não é seguro).
		return &WireEvent{Type: EventCancelled, Data: nil}

	default:
		return nil
	}
}

// progressPayload renderiza um evento progress ESTRUTURADO (alinha ao shape que
// o knowledge sync handler já emite para "progress"): um content legível + o
// metadata do engine (stage, agent, chunk_count, finish_reason) agrupado.
func (m *EngineMapper) progressPayload(ev orchestration.StreamEvent) map[string]interface{} {
	p := map[string]interface{}{"content": ev.Content}
	if len(ev.Metadata) > 0 {
		p["metadata"] = ev.Metadata
	}
	return p
}

// ThinkingPayload constrói o evento envelope "thinking". Preserva os campos
// legados {"content","session_id"} (lidos por pkg/cosca + cosca-desktop) e
// adiciona model/provider/agent para o header do Desktop (aditivo).
func (m *EngineMapper) ThinkingPayload(content string) map[string]interface{} {
	p := map[string]interface{}{"content": content}
	m.fillEnvelope(p)
	return p
}

// StatusPayload constrói o evento "status" (aditivo): {"status":"<phase>"}.
func (m *EngineMapper) StatusPayload(phase StatusPhase) map[string]interface{} {
	return map[string]interface{}{"status": string(phase)}
}

// MetadataPayload constrói o evento "metadata" (aditivo) com o enriquecimento
// do run (session/model/provider/agent + duração + token usage).
func (m *EngineMapper) MetadataPayload(durationMs int64, tokenUsage TokenUsage) map[string]interface{} {
	return map[string]interface{}{
		"session_id":  m.SessionID,
		"model":       m.Model,
		"provider":    m.Provider,
		"agent":       m.Agent,
		"duration_ms": durationMs,
		"token_usage": tokenUsage,
	}
}

// DonePayload constrói o evento final "done". Preserva o shape legado
// {"duration_ms","session_id"} lido por pkg/cosca + cosca-desktop.
func (m *EngineMapper) DonePayload(durationMs int64) map[string]interface{} {
	p := map[string]interface{}{"duration_ms": durationMs}
	if m.SessionID != "" {
		p["session_id"] = m.SessionID
	}
	return p
}

// fillEnvelope adiciona os campos request-scoped (session/model/provider/agent)
// a um payload envelope, apenas quando não-vazios.
func (m *EngineMapper) fillEnvelope(p map[string]interface{}) {
	if m.SessionID != "" {
		p["session_id"] = m.SessionID
	}
	if m.Model != "" {
		p["model"] = m.Model
	}
	if m.Provider != "" {
		p["provider"] = m.Provider
	}
	if m.Agent != "" {
		p["agent"] = m.Agent
	}
}

// EstimateTokens é um estimador determinístico e leve usado para popular o
// token_usage do evento "metadata" quando o provider não expõe o uso real via
// streaming. Aproxima ≈ 1 token por 4 caracteres.
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	n := len([]rune(s))
	if n <= 0 {
		return 0
	}
	return (n + 3) / 4
}
