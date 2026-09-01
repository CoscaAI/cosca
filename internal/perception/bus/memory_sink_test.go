package bus

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// ──────────────────────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────────────────────

// mockEpisodicWriter captura os EpisodicRecords gravados (satisfaz EpisodicWriter).
type mockEpisodicWriter struct {
	mu  sync.Mutex
	got []memory.EpisodicRecord
}

func (m *mockEpisodicWriter) StoreEpisodic(_ context.Context, rec memory.EpisodicRecord) (*memory.EpisodicRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.got = append(m.got, rec)
	return &rec, nil
}

func (m *mockEpisodicWriter) snapshot() []memory.EpisodicRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]memory.EpisodicRecord, len(m.got))
	copy(out, m.got)
	return out
}

// buildTestWorldState monta um WorldState com uma observação de visão, uma de
// áudio e uma MultiRel ligando-as (o "o que estava vendo quando ouvi X").
func buildTestWorldState() *WorldState {
	entities := []worldmodel.WorldEntity{
		{ID: "e1", Type: worldmodel.EntityObject, Label: "editor aberto", Confidence: 0.9},
		{ID: "e2", Type: worldmodel.EntityObject, Label: "monitor", Confidence: 0.8},
	}
	visionObs := Observation{
		Timestamp:  1000,
		Modality:   ModalityVision,
		Duration:   500 * time.Millisecond,
		Sequence:   1,
		Confidence: 0.9,
		Payload:    Payload{Vision: &vision.Observation{Entities: entities, Latency: 10 * time.Millisecond}},
	}
	audioObs := Observation{
		Timestamp:  1100,
		Modality:   ModalityAudio,
		Duration:   300 * time.Millisecond,
		Sequence:   2,
		Confidence: 0.95,
		Payload: Payload{Audio: &AudioPayload{
			Text:      "abrir o editor",
			SegmentID: 7,
			IsFinal:   true,
			Tokens:    []Token{{Word: "abrir", Offset: 0, Confidence: 0.95}, {Word: "o", Offset: 100000000, Confidence: 0.9}},
		}},
	}
	rel := MultiRel{
		AudioSeg:   7,
		TextHint:   "abrir o editor",
		AudioConf:  0.9,
		Confidence: 0.95,
		Overlap: []WindowRef{
			{Sequence: 1, Timestamp: 1000, Entities: entities},
		},
		Window: [2]time.Duration{300 * time.Millisecond, 300 * time.Millisecond},
	}
	return &WorldState{
		Timestamp: 1000,
		Window:    []Observation{visionObs, audioObs},
		Relations: []MultiRel{rel},
	}
}

// ──────────────────────────────────────────────────────────────
// Testes dos builders de registros
// ──────────────────────────────────────────────────────────────

func TestSinkRecordForMultimodal(t *testing.T) {
	ws := buildTestWorldState()
	rel := ws.Relations[0]
	rec := sinkRecordForMultimodal(rel, ws)
	if rec == nil {
		t.Fatal("sinkRecordForMultimodal returned nil")
	}
	if rec.Modality != "multimodal" {
		t.Errorf("Modality = %q, want multimodal", rec.Modality)
	}
	if rec.AudioText != "abrir o editor" {
		t.Errorf("AudioText = %q, want %q", rec.AudioText, "abrir o editor")
	}
	if len(rec.MultiRels) != 1 {
		t.Fatalf("expected 1 MultiRel, got %d", len(rec.MultiRels))
	}
	if rec.MultiRels[0].AudioSeg != 7 {
		t.Errorf("MultiRel.AudioSeg = %d, want 7", rec.MultiRels[0].AudioSeg)
	}
	if len(rec.MultiRels[0].Overlap) != 1 {
		t.Errorf("expected 1 overlap in MultiRel, got %d", len(rec.MultiRels[0].Overlap))
	}
	if len(rec.Entities) != 2 {
		t.Errorf("expected 2 entities, got %d", len(rec.Entities))
	}
}

func TestSinkRecordForAudio(t *testing.T) {
	ws := buildTestWorldState()
	var audio *Observation
	for i := range ws.Window {
		if ws.Window[i].Modality == ModalityAudio {
			audio = &ws.Window[i]
			break
		}
	}
	if audio == nil {
		t.Fatal("no audio observation in test world state")
	}
	rec := sinkRecordForAudio(audio)
	if rec == nil {
		t.Fatal("sinkRecordForAudio returned nil")
	}
	if rec.Modality != "audio" {
		t.Errorf("Modality = %q, want audio", rec.Modality)
	}
	if rec.AudioText != "abrir o editor" {
		t.Errorf("AudioText = %q, want %q", rec.AudioText, "abrir o editor")
	}
	if len(rec.Tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(rec.Tokens))
	}
}

func TestSinkRecordForVision(t *testing.T) {
	ws := buildTestWorldState()
	var vis *Observation
	for i := range ws.Window {
		if ws.Window[i].Modality == ModalityVision {
			vis = &ws.Window[i]
			break
		}
	}
	if vis == nil {
		t.Fatal("no vision observation in test world state")
	}
	rec := sinkRecordForVision(vis)
	if rec == nil {
		t.Fatal("sinkRecordForVision returned nil")
	}
	if rec.Modality != "vision" {
		t.Errorf("Modality = %q, want vision", rec.Modality)
	}
	if len(rec.Entities) != 2 {
		t.Errorf("expected 2 entities, got %d", len(rec.Entities))
	}
	if rec.VisionSummary == "" {
		t.Error("expected non-empty VisionSummary")
	}
}

// ──────────────────────────────────────────────────────────────
// Teste de dedup + enqueue (sem goroutines — lê o buffer diretamente)
// ──────────────────────────────────────────────────────────────

func TestMemorySink_BuildRecordsEnqueuesAndDedups(t *testing.T) {
	w := &mockEpisodicWriter{}
	sink := NewMemorySink(nil, w, MemorySinkConfig{Enabled: true, MaxBuffered: 64, MaxSeen: 64}, zerolog.Nop())
	if !sink.Enabled() {
		t.Fatal("expected sink to be enabled")
	}

	ws := buildTestWorldState()
	// Primeira passada: enfileira o registro multimodal (áudio e visão estão
	// cobertos pela MultiRel → somente o multimodal é gerado).
	sink.buildRecords(ws)

	var got []memory.EpisodicRecord
	drain := func() {
		for {
			select {
			case r := <-sink.ch:
				got = append(got, r)
			case <-time.After(30 * time.Millisecond):
				return
			}
		}
	}
	drain()
	if len(got) != 1 {
		t.Fatalf("expected 1 enqueued record (multimodal), got %d", len(got))
	}
	if got[0].Modality != "multimodal" {
		t.Errorf("got modality %q, want multimodal", got[0].Modality)
	}

	// Segunda passada com o mesmo WorldState: dedup por Sequence → nada novo.
	sink.buildRecords(ws)
	var got2 []memory.EpisodicRecord
	drain = func() {
		for {
			select {
			case r := <-sink.ch:
				got2 = append(got2, r)
			case <-time.After(20 * time.Millisecond):
				return
			}
		}
	}
	drain()
	if len(got2) != 0 {
		t.Fatalf("expected 0 re-enqueued records after dedup, got %d", len(got2))
	}
}

// ──────────────────────────────────────────────────────────────
// Teste end-to-end via worker (mock writer) com poll por deadline
// ──────────────────────────────────────────────────────────────

func TestMemorySink_WorkerWritesToEngine(t *testing.T) {
	w := &mockEpisodicWriter{}
	sink := NewMemorySink(nil, w, MemorySinkConfig{Enabled: true, MaxBuffered: 32, MaxSeen: 32}, zerolog.Nop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.Start(ctx)
	defer sink.Stop()

	ws := buildTestWorldState()
	sink.buildRecords(ws)
	sink.buildRecords(ws) // dedup — não deve duplicar

	// Poll por deadline até o worker gravar.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := w.snapshot(); len(got) >= 1 {
			if got[0].Modality != "multimodal" {
				t.Errorf("worker wrote modality %q, want multimodal", got[0].Modality)
			}
			if len(w.snapshot()) > 1 {
				t.Errorf("expected exactly 1 record written (dedup), got %d", len(w.snapshot()))
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the worker to write an EpisodicRecord")
}
