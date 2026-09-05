package memory

import (
	"context"
	"testing"
	"time"
)

// newEpisodicEngine cria um engine sobre um dir temporário, com a camada
// episódica garantida. Reutiliza o padrão dos demais testes de memory.
func newEpisodicEngine(t *testing.T) *MemoryEngine {
	t.Helper()
	engine, err := NewEngine(WithConfig(EngineConfig{DataDir: t.TempDir(), AutoPrune: false}))
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if err := engine.EnsureEpisodicLayer(); err != nil {
		t.Fatalf("EnsureEpisodicLayer failed: %v", err)
	}
	return engine
}

// TestEpisodicRoundTrip_StoreAndQuery faz o round-trip da memória episódica:
// grava um EpisodicRecord sincronizado (visão+áudio via MultiRel) e o recupera
// por janela temporal, por query de texto e por modality.
func TestEpisodicRoundTrip_StoreAndQuery(t *testing.T) {
	engine := newEpisodicEngine(t)
	ctx := context.Background()

	base := time.Date(2026, 8, 31, 10, 32, 0, 0, time.UTC)
	rec := EpisodicRecord{
		ID:            "epi-roundtrip-1",
		Timestamp:     base,
		Monotonic:     1234567890,
		Modality:      "multimodal",
		Sequence:      42,
		Confidence:    0.91,
		AudioText:     "abrir o editor",
		VisionSummary: "Vision observation: 2 entidade(s). editor aberto, monitor",
		Tokens: []EpisodicToken{
			{Word: "abrir", Offset: 0, Confidence: 0.95},
			{Word: "o", Offset: 200_000_000, Confidence: 0.9},
			{Word: "editor", Offset: 400_000_000, Confidence: 0.9},
		},
		MultiRels: []EpisodicMultiRel{
			{
				AudioSeg:   7,
				TextHint:   "abrir o editor",
				AudioConf:  0.9,
				Confidence: 0.95,
				Overlap: []EpisodicWindowRef{
					{Sequence: 10, Timestamp: 1200000000,
						Entities: []EpisodicEntity{
							{ID: "e1", Label: "editor aberto", Type: "object", Confidence: 0.9},
							{ID: "e2", Label: "monitor", Type: "object", Confidence: 0.8},
						}},
				},
			},
		},
		Entities: []EpisodicEntity{
			{ID: "e1", Label: "editor aberto", Type: "object", Confidence: 0.9},
			{ID: "e2", Label: "monitor", Type: "object", Confidence: 0.8},
		},
		Context: "test",
	}

	saved, err := engine.StoreEpisodic(ctx, rec)
	if err != nil {
		t.Fatalf("StoreEpisodic failed: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("StoreEpisodic returned empty ID")
	}

	// 1. Recuperar por janela temporal (Since/Until).
	got, err := engine.QueryEpisodic(ctx, EpisodicQuery{
		Since: base.Add(-time.Hour),
		Until: base.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("QueryEpisodic (time window) failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 record in time window, got %d", len(got))
	}
	if got[0].ID != "epi-roundtrip-1" {
		t.Errorf("record ID = %q, want epi-roundtrip-1", got[0].ID)
	}
	if got[0].AudioText != "abrir o editor" {
		t.Errorf("AudioText = %q, want %q", got[0].AudioText, "abrir o editor")
	}
	if len(got[0].MultiRels) != 1 || len(got[0].MultiRels[0].Overlap) != 1 {
		t.Errorf("expected 1 MultiRel with 1 overlap, got %+v", got[0].MultiRels)
	}
	if len(got[0].Entities) != 2 {
		t.Errorf("expected 2 entities, got %d", len(got[0].Entities))
	}

	// 2. Recuperar por query de texto (AND por palavra).
	gotQ, err := engine.QueryEpisodic(ctx, EpisodicQuery{Query: "editor abrir"})
	if err != nil {
		t.Fatalf("QueryEpisodic (query) failed: %v", err)
	}
	if len(gotQ) != 1 {
		t.Fatalf("expected 1 record for query, got %d", len(gotQ))
	}

	// 3. Recuperar por modality (multimodal deve bater; audio não).
	gotM, err := engine.QueryEpisodic(ctx, EpisodicQuery{Modality: "multimodal"})
	if err != nil {
		t.Fatalf("QueryEpisodic (modality) failed: %v", err)
	}
	if len(gotM) != 1 {
		t.Fatalf("expected 1 multimodal record, got %d", len(gotM))
	}
	gotA, err := engine.QueryEpisodic(ctx, EpisodicQuery{Modality: "audio"})
	if err != nil {
		t.Fatalf("QueryEpisodic (audio modality) failed: %v", err)
	}
	if len(gotA) != 0 {
		t.Fatalf("expected 0 audio records, got %d", len(gotA))
	}

	// 4. Janela temporal estrita: fora da janela não retorna.
	gotOut, err := engine.QueryEpisodic(ctx, EpisodicQuery{
		Since: base.Add(2 * time.Hour),
		Until: base.Add(3 * time.Hour),
	})
	if err != nil {
		t.Fatalf("QueryEpisodic (out-of-window) failed: %v", err)
	}
	if len(gotOut) != 0 {
		t.Fatalf("expected 0 out-of-window records, got %d", len(gotOut))
	}
}

// TestEpisodic_QueryFiltersModality verifica a filtragem por modality entre
// registros de áudio isolado e multimodal.
func TestEpisodic_QueryFiltersModality(t *testing.T) {
	engine := newEpisodicEngine(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 31, 9, 0, 0, 0, time.UTC)

	audio := EpisodicRecord{ID: "epi-audio-1", Timestamp: base, Modality: "audio", AudioText: "eita, esqueci de salvar", Confidence: 0.8}
	vis := EpisodicRecord{ID: "epi-vis-1", Timestamp: base.Add(time.Minute), Modality: "vision", VisionSummary: "monitor", Confidence: 0.7}
	multi := EpisodicRecord{ID: "epi-multi-1", Timestamp: base.Add(2 * time.Minute), Modality: "multimodal", AudioText: "salvar agora", VisionSummary: "editor", Confidence: 0.9}

	for _, r := range []EpisodicRecord{audio, vis, multi} {
		if _, err := engine.StoreEpisodic(ctx, r); err != nil {
			t.Fatalf("StoreEpisodic(%s) failed: %v", r.ID, err)
		}
	}

	// Buscar por modality.
	audios, _ := engine.QueryEpisodic(ctx, EpisodicQuery{Modality: "audio"})
	if len(audios) != 1 || audios[0].ID != "epi-audio-1" {
		t.Errorf("audio filter: got %d records, want 1 (epi-audio-1)", len(audios))
	}
	// Buscar por texto "salvar" — bate em audio ("esqueci de salvar" id? não —
	// "salvar" está em multi "salvar agora"; audio tem "esqueci de salvar").
	got, _ := engine.QueryEpisodic(ctx, EpisodicQuery{Query: "salvar"})
	if len(got) != 2 {
		t.Errorf("query 'salvar': got %d records, want 2 (audio+multi)", len(got))
	}
	// Query que só bate em uma entidade.
	gotVis, _ := engine.QueryEpisodic(ctx, EpisodicQuery{Query: "monitor"})
	if len(gotVis) != 1 || gotVis[0].ID != "epi-vis-1" {
		t.Errorf("query 'monitor': got %d records, want 1 (epi-vis-1)", len(gotVis))
	}
}

// TestEpisodicLayer_OnDemand verifica que a camada é registrada on-demand e é
// idempotente (chamar duas vezes não cria store duplicado nem erro).
func TestEpisodicLayer_OnDemand(t *testing.T) {
	engine := newEpisodicEngine(t)
	if !engine.EpisodicEnabled() {
		t.Fatal("expected EpisodicEnabled true after EnsureEpisodicLayer")
	}
	// Idempotente.
	if err := engine.EnsureEpisodicLayer(); err != nil {
		t.Fatalf("second EnsureEpisodicLayer failed: %v", err)
	}
	if !engine.EpisodicEnabled() {
		t.Fatal("expected EpisodicEnabled true after second ensure")
	}
}

// TestEpisodic_PruneLimit verifica o cap de registros via PruneEpisodic.
func TestEpisodic_PruneLimit(t *testing.T) {
	engine := newEpisodicEngine(t)
	ctx := context.Background()
	base := time.Now()
	for i := 0; i < 5; i++ {
		_, err := engine.StoreEpisodic(ctx, EpisodicRecord{
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			Modality:  "vision",
			VisionSummary: "frame",
			Sequence:  uint64(i + 1),
		})
		if err != nil {
			t.Fatalf("StoreEpisodic#%d failed: %v", i, err)
		}
	}
	removed, err := engine.PruneEpisodic(ctx, 3)
	if err != nil {
		t.Fatalf("PruneEpisodic failed: %v", err)
	}
	if removed != 2 {
		t.Fatalf("PruneEpisodic removed = %d, want 2", removed)
	}
	got, _ := engine.QueryEpisodic(ctx, EpisodicQuery{})
	if len(got) != 3 {
		t.Fatalf("after prune, got %d records, want 3", len(got))
	}
}
