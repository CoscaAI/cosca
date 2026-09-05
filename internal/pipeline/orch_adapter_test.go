package pipeline

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

// TestConvertStreamEvent_AllTypes verifica que convertStreamEvent (a camada
// engine→RunEvent do pipeline) mapeia TODOS os tipos do engine, incluindo o
// novo StreamEventCancelled → EventCancelled (ETAPA 4, aditivo). Antes o
// cancelled caía no default (EventContent), o que perdia a semântica de
// cancelamento como estado final distinto do erro.
func TestConvertStreamEvent_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		ev       orchestration.StreamEvent
		wantType RunEventType
		wantData string
	}{
		{
			name:     "chunk→content",
			ev:       orchestration.StreamEvent{Type: orchestration.StreamEventChunk, Content: "Hello"},
			wantType: EventContent,
			wantData: "Hello",
		},
		{
			name:     "error→error",
			ev:       orchestration.StreamEvent{Type: orchestration.StreamEventError, Content: "stream_read_failed"},
			wantType: EventError,
			wantData: "stream_read_failed",
		},
		{
			name:     "cancelled→cancelled (ADITIVO, estado final distinto de erro)",
			ev:       orchestration.StreamEvent{Type: orchestration.StreamEventCancelled, Content: "stream_cancelled"},
			wantType: EventCancelled,
			wantData: "stream_cancelled",
		},
		{
			name:     "progress→tool_start",
			ev:       orchestration.StreamEvent{Type: orchestration.StreamEventProgress, Content: "resolving agent"},
			wantType: EventToolStart,
			wantData: "resolving agent",
		},
		{
			name:     "stage_transition→tool_start",
			ev:       orchestration.StreamEvent{Type: orchestration.StreamEventStageTransition, Content: "Entering stage: report"},
			wantType: EventToolStart,
			wantData: "Entering stage: report",
		},
		{
			name:     "desconhecido→content (conservador)",
			ev:       orchestration.StreamEvent{Type: orchestration.StreamEventType("future_evt"), Content: "x"},
			wantType: EventContent,
			wantData: "x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertStreamEvent(tt.ev)
			if got.Type != tt.wantType {
				t.Errorf("convertStreamEvent type = %q, want %q", got.Type, tt.wantType)
			}
			if data, ok := got.Data.(string); !ok || data != tt.wantData {
				t.Errorf("convertStreamEvent data = %v (type %T), want %q", got.Data, got.Data, tt.wantData)
			}
		})
	}
}
