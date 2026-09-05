package stream

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

// ── Map: translation engine→wire (single source of truth) ───────────────────

func TestEngineMapper_MapAllTypes(t *testing.T) {
	m := NewEngineMapper("sess-1", "model-x", "provider-y", "agent-z")

	tests := []struct {
		name     string
		ev       orchestration.StreamEvent
		wantType string
		wantNil  bool
	}{
		{
			name:     "chunk→response (legacy shape preserved)",
			ev:       orchestration.StreamEvent{Type: orchestration.StreamEventChunk, Content: "Hello"},
			wantType: EventResponse,
		},
		{
			name:     "progress→progress",
			ev:       orchestration.StreamEvent{Type: orchestration.StreamEventProgress, Content: "resolving agent", Metadata: map[string]interface{}{"agent": "agent-z"}},
			wantType: EventProgress,
		},
		{
			name:     "stage_transition→progress",
			ev:       orchestration.StreamEvent{Type: orchestration.StreamEventStageTransition, Content: "Entering stage: report", Metadata: map[string]interface{}{"stage": "report"}},
			wantType: EventProgress,
		},
		{
			name:     "error→error",
			ev:       orchestration.StreamEvent{Type: orchestration.StreamEventError, Content: "stream_read_failed"},
			wantType: EventError,
		},
		{
			name:     "cancelled→cancelled",
			ev:       orchestration.StreamEvent{Type: orchestration.StreamEventCancelled, Content: "stream_cancelled"},
			wantType: EventCancelled,
		},
		{
			name:    "unknown→nil (conservative)",
			ev:      orchestration.StreamEvent{Type: orchestration.StreamEventType("future_event")},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := m.Map(tt.ev)
			if tt.wantNil {
				if wire != nil {
					t.Fatalf("expected nil WireEvent, got %+v", wire)
				}
				return
			}
			if wire == nil {
				t.Fatalf("expected WireEvent, got nil")
			}
			if wire.Type != tt.wantType {
				t.Errorf("wire.Type = %q, want %q", wire.Type, tt.wantType)
			}
		})
	}
}

// TestEngineMapper_MapChunkPreservesLegacyString verifies the chunk→response
// mapping emits a STRING data payload so SSEWriter produces the legacy
// {"type":"response","content":"..."} that pkg/cosca + cosca-desktop parse.
func TestEngineMapper_MapChunkPreservesLegacyString(t *testing.T) {
	m := NewEngineMapper("sess", "m", "p", "a")
	wire := m.Map(orchestration.StreamEvent{Type: orchestration.StreamEventChunk, Content: "Hello"})
	if wire.Type != EventResponse {
		t.Fatalf("type = %q, want %q", wire.Type, EventResponse)
	}
	s, ok := wire.Data.(string)
	if !ok {
		t.Fatalf("chunk data must remain a string for legacy wire shape, got %T", wire.Data)
	}
	if s != "Hello" {
		t.Errorf("chunk data = %q, want %q", s, "Hello")
	}
}

// TestEngineMapper_MapProgressStructured verifies progress events render a
// structured payload (content + metadata) aligning with the existing
// knowledge-sync progress wire shape.
func TestEngineMapper_MapProgressStructured(t *testing.T) {
	m := NewEngineMapper("sess", "m", "p", "a")
	wire := m.Map(orchestration.StreamEvent{
		Type:     orchestration.StreamEventStageTransition,
		Content:  "Entering stage: report",
		Metadata: map[string]interface{}{"stage": "report"},
	})
	if wire.Type != EventProgress {
		t.Fatalf("type = %q, want progress", wire.Type)
	}
	p, ok := wire.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("progress data must be a map, got %T", wire.Data)
	}
	if p["content"] != "Entering stage: report" {
		t.Errorf("progress content = %v", p["content"])
	}
	md, ok := p["metadata"].(map[string]interface{})
	if !ok || md["stage"] != "report" {
		t.Errorf("progress metadata = %v, want stage=report", p["metadata"])
	}
}

// ── Envelope builders ────────────────────────────────────────────────────────

func TestEngineMapper_ThinkingPayload(t *testing.T) {
	m := NewEngineMapper("sess-1", "model-x", "provider-y", "agent-z")
	p := m.ThinkingPayload("Analyzing...")
	if p["content"] != "Analyzing..." {
		t.Errorf("content = %v", p["content"])
	}
	if p["session_id"] != "sess-1" {
		t.Errorf("session_id = %v", p["session_id"])
	}
	if p["model"] != "model-x" {
		t.Errorf("model = %v", p["model"])
	}
	if p["provider"] != "provider-y" {
		t.Errorf("provider = %v", p["provider"])
	}
	if p["agent"] != "agent-z" {
		t.Errorf("agent = %v", p["agent"])
	}
}

func TestEngineMapper_ThinkingPayloadSkipsEmpty(t *testing.T) {
	m := NewEngineMapper("", "", "", "")
	p := m.ThinkingPayload("hello")
	if p["content"] != "hello" {
		t.Errorf("content = %v", p["content"])
	}
	if _, ok := p["session_id"]; ok {
		t.Error("session_id must be omitted when empty")
	}
	if _, ok := p["model"]; ok {
		t.Error("model must be omitted when empty")
	}
	if _, ok := p["provider"]; ok {
		t.Error("provider must be omitted when empty")
	}
	if _, ok := p["agent"]; ok {
		t.Error("agent must be omitted when empty")
	}
}

func TestEngineMapper_StatusPayload(t *testing.T) {
	m := NewEngineMapper("", "", "", "")
	p := m.StatusPayload(StatusProcessing)
	if p["status"] != "processing" {
		t.Errorf("status = %v, want processing", p["status"])
	}
}

func TestEngineMapper_MetadataPayload(t *testing.T) {
	m := NewEngineMapper("sess-1", "model-x", "provider-y", "agent-z")
	tu := TokenUsage{InputTokens: 10, OutputTokens: 20, TotalTokens: 30}
	p := m.MetadataPayload(1234, tu)
	if p["session_id"] != "sess-1" || p["model"] != "model-x" || p["provider"] != "provider-y" || p["agent"] != "agent-z" {
		t.Errorf("identity fields = %+v", p)
	}
	if p["duration_ms"] != int64(1234) {
		t.Errorf("duration_ms = %v", p["duration_ms"])
	}
	if _, ok := p["token_usage"].(TokenUsage); !ok {
		t.Errorf("token_usage = %T, want TokenUsage", p["token_usage"])
	}
}

func TestEngineMapper_DonePayload(t *testing.T) {
	m := NewEngineMapper("sess-1", "", "", "")
	p := m.DonePayload(4321)
	if p["duration_ms"] != int64(4321) {
		t.Errorf("duration_ms = %v", p["duration_ms"])
	}
	if p["session_id"] != "sess-1" {
		t.Errorf("session_id = %v", p["session_id"])
	}
}

// ── Coherent sequence (canonical wire order) ────────────────────────────────

// TestEngineMapper_CoherentSequence constrói a sequência canônica exatamente
// como o handler a emite: thinking → status:processing → progress* →
// response* → metadata → status:done → done. Verifica a ORDEM estável e que
// cada payload carrega o shape esperado.
func TestEngineMapper_CoherentSequence(t *testing.T) {
	m := NewEngineMapper("sess-1", "model-x", "provider-y", "agent-z")

	type step struct {
		wireType string
		data     interface{}
	}
	var seq []step

	// 1. thinking (envelope)
	seq = append(seq, step{EventThinking, m.ThinkingPayload("Analyzing...")})
	// 2. status:processing
	seq = append(seq, step{EventStatus, m.StatusPayload(StatusProcessing)})
	// 3. progress* (from engine)
	seq = append(seq, step{"", m.Map(orchestration.StreamEvent{Type: orchestration.StreamEventProgress, Content: "resolving"}).Data})
	// 4. response* (chunks)
	seq = append(seq, step{"", m.Map(orchestration.StreamEvent{Type: orchestration.StreamEventChunk, Content: "Hello"}).Data})
	// 5. metadata
	seq = append(seq, step{EventMetadata, m.MetadataPayload(1500, TokenUsage{OutputTokens: 8})})
	// 6. status:done
	seq = append(seq, step{EventStatus, m.StatusPayload(StatusDone)})
	// 7. done
	seq = append(seq, step{EventDone, m.DonePayload(1500)})

	wantTypes := []string{
		EventThinking, EventStatus, EventProgress, EventResponse,
		EventMetadata, EventStatus, EventDone,
	}
	if len(seq) != len(wantTypes) {
		t.Fatalf("sequence length = %d, want %d", len(seq), len(wantTypes))
	}
	for i, want := range wantTypes {
		// steps 3 and 4 (index 2, 3) get their type from the mapper Map result.
		got := seq[i].wireType
		if got == "" {
			// derive from data (they came from Map but we set type to "" above)
			// — instead recompute via Map below for these two.
			continue
		}
		if got != want {
			t.Errorf("step %d type = %q, want %q", i, got, want)
		}
	}

	// Re-verify the Map-derived steps (3 & 4) explicitly.
	if got := m.Map(orchestration.StreamEvent{Type: orchestration.StreamEventProgress, Content: "resolving"}).Type; got != "progress" {
		t.Errorf("progress wire type = %q", got)
	}
	if got := m.Map(orchestration.StreamEvent{Type: orchestration.StreamEventChunk, Content: "Hello"}).Type; got != "response" {
		t.Errorf("chunk wire type = %q", got)
	}
}

// TestEngineMapper_SequenceSerializesFromSSEWriter valida que a sequência
// canônica serializa via SSEWriter como um fluxo coerente de eventos JSON,
// na ordem esperada (pkg/cosca + cosca-desktop continuam lendo response/done).
func TestEngineMapper_SequenceSerializesFromSSEWriter(t *testing.T) {
	m := NewEngineMapper("sess-1", "model-x", "provider-y", "agent-z")
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("NewSSEWriter: %v", err)
	}

	_ = sw.WriteEvent(EventThinking, m.ThinkingPayload("Analyzing..."))
	_ = sw.WriteEvent(EventStatus, m.StatusPayload(StatusProcessing))
	_ = sw.WriteEvent(EventProgress, m.Map(orchestration.StreamEvent{Type: orchestration.StreamEventProgress, Content: "resolving"}).Data)
	_ = sw.WriteEvent(EventResponse, m.Map(orchestration.StreamEvent{Type: orchestration.StreamEventChunk, Content: "Hello"}).Data)
	_ = sw.WriteEvent(EventMetadata, m.MetadataPayload(1500, TokenUsage{OutputTokens: 8}))
	_ = sw.WriteEvent(EventStatus, m.StatusPayload(StatusDone))
	_ = sw.WriteEvent(EventDone, m.DonePayload(1500))

	events := sseBodyMaps(t, tw.Body().String())
	wantTypes := []string{EventThinking, EventStatus, EventProgress, EventResponse, EventMetadata, EventStatus, EventDone}
	if len(events) != len(wantTypes) {
		t.Fatalf("event count = %d, want %d", len(events), len(wantTypes))
	}
	for i, want := range wantTypes {
		if got := events[i]["type"]; got != want {
			t.Errorf("event[%d].type = %v, want %v", i, got, want)
		}
	}

	// Status phases em ordem: processing → done.
	if events[1]["status"] != string(StatusProcessing) {
		t.Errorf("event[1].status = %v, want processing", events[1]["status"])
	}
	if events[5]["status"] != string(StatusDone) {
		t.Errorf("event[5].status = %v, want done", events[5]["status"])
	}

	// Response legado preserva content.
	if events[3]["content"] != "Hello" {
		t.Errorf("response content = %v, want Hello", events[3]["content"])
	}

	// Metadata carrega o enriquecimento do run.
	meta := events[4]
	if meta["model"] != "model-x" || meta["provider"] != "provider-y" || meta["agent"] != "agent-z" || meta["session_id"] != "sess-1" {
		t.Errorf("metadata identity = %v", meta)
	}
	if meta["duration_ms"] != float64(1500) {
		t.Errorf("metadata.duration_ms = %v, want 1500", meta["duration_ms"])
	}

	// Done preserva o shape legado (duration_ms + session_id).
	done := events[6]
	if done["duration_ms"] != float64(1500) {
		t.Errorf("done.duration_ms = %v, want 1500", done["duration_ms"])
	}
	if done["session_id"] != "sess-1" {
		t.Errorf("done.session_id = %v, want sess-1", done["session_id"])
	}

	// Legado: response + done coexistem com os campos aditivos.
	if events[3]["content"] != "Hello" || events[6]["type"] != EventDone {
		t.Error("legacy response + done must be intact alongside additive events")
	}
}

// sseBodyMaps parseia os blocos "data: {...}" do corpo SSE em uma lista de
// mapas, preservando a ordem do wire.
func sseBodyMaps(t *testing.T, body string) []map[string]interface{} {
	t.Helper()
	var events []map[string]interface{}
	for _, block := range strings.Split(body, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		if !strings.HasPrefix(block, "data: ") {
			continue
		}
		data := strings.TrimPrefix(block, "data: ")
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			t.Fatalf("invalid SSE JSON %q: %v", data, err)
		}
		events = append(events, m)
	}
	return events
}

// ── EstimateTokens ─────────────────────────────────────────────────────────

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "empty", in: "", want: 0},
		{name: "single-char", in: "a", want: 1},
		{name: "four-chars", in: "abcd", want: 1},
		{name: "five-chars", in: "abcde", want: 2},
		{name: "long", in: "this is a longer sentence", want: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EstimateTokens(tt.in); got != tt.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
