// Package installer — Event Stream (o contrato para a UI).
//
// O professor desenhou: "a UI visualiza o que o Provisioner REALMENTE faz".
// Para isso, o orquestrador EMITE eventos em tempo real — cada etapa publica
// o que está acontecendo (CHECK/ACTION/RESULT/EVIDENCE/STATE), e qualquer UI
// (exe com animação, terminal, webview) consome o stream e desenha por cima.
//
// A animação NUNCA é falsa: cada evento reflete o estado real do provisioner.
package installer

import (
	"encoding/json"
	"io"
	"time"
)

// EventType classifica um evento do stream.
type EventType string

// Tipos de evento emitidos pelo orquestrador.
const (
	// EventStateChanged: o Installation State avançou.
	EventStateChanged EventType = "state_changed"
	// EventStepStarted: um check começou (Detect).
	EventStepStarted EventType = "step_started"
	// EventStepComplete: um check terminou com veredito.
	EventStepComplete EventType = "step_complete"
	// EventProgress: progresso de uma fase longa (ex: index build).
	EventProgress EventType = "progress"
	// EventCertified: o provisionamento completou.
	EventCertified EventType = "certified"
	// EventError: uma fase bloqueou.
	EventError EventType = "error"
)

// Event é um evento do stream de provisionamento.
type Event struct {
	Type      EventType `json:"type"`
	Check     string    `json:"check,omitempty"`
	Action    string    `json:"action,omitempty"`
	Result    Result    `json:"result,omitempty"`
	Evidence  []string  `json:"evidence,omitempty"`
	State     State     `json:"state,omitempty"`
	FromState State     `json:"from_state,omitempty"`
	Phase     string    `json:"phase,omitempty"`
	Progress  int       `json:"progress,omitempty"` // 0..100
	Message   string    `json:"message,omitempty"`
	Time      time.Time `json:"time"`
}

// EmitFunc é o callback de eventos (a UI/stream consome).
type EmitFunc func(Event)

// JSONEmitter serializa eventos como JSON (uma linha por evento) — o formato
// que o `cosca setup --watch` escreve para a UI consumir.
func JSONEmitter(w io.Writer) EmitFunc {
	enc := json.NewEncoder(w)
	return func(e Event) {
		_ = enc.Encode(e)
	}
}

// NopEmitter descarta eventos (modo silencioso).
func NopEmitter() EmitFunc {
	return func(Event) {}
}

// emitStateChanged publica a transição de estado.
func (e *Event) emit(f EmitFunc) {
	if f != nil {
		f(*e)
	}
}
