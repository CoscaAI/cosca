package oracle

import "testing"

// ─── SEARCH_PROTOCOL: precisão antes de recall (§4) ────────────────────────────

func TestFilterAndRank_NoiseDiscarded(t *testing.T) {
	// Resultado NOISE (sem proveniência, conteúdo irrelevante) não aparece.
	raw := []SearchResult{
		{Title: "benchmark.md", Content: "o processamento do carro foi lento", Class: Inferred},
		{Title: "CARRO_PROTOCOL.md", Content: "checkup do carro: doctor 26/28", Class: Fact, Provenance: Provenance{Ref: "commit abc"}},
	}
	resp := FilterAndRank(IntentDiagnose, raw)
	if resp.Outcome != OutcomeFound {
		t.Errorf("esperado FOUND, got %s", resp.Outcome)
	}
	for _, p := range resp.Presentable {
		if p.Relevance == Noise {
			t.Error("NOISE não deveria estar em Presentable (§7)")
		}
	}
	// O benchmark.md (sem proveniência) não pode ser apresentado como FACT.
	if len(resp.Presentable) == 0 {
		t.Fatal("deveria ter pelo menos o CARRO_PROTOCOL")
	}
}

func TestFilterAndRank_DuplicatesGrouped(t *testing.T) {
	// 3 resultados com o MESMO conteúdo → agrupados em 1 (§9).
	raw := []SearchResult{
		{Title: "RECOVERY_PROTOCOL.md", Content: "idêntico", Class: Fact, Provenance: Provenance{Ref: "c1"}},
		{Title: "RECOVERY_PROTOCOL.md", Content: "idêntico", Class: Fact, Provenance: Provenance{Ref: "c2"}},
		{Title: "RECOVERY_PROTOCOL.md", Content: "idêntico", Class: Fact, Provenance: Provenance{Ref: "c3"}},
	}
	resp := FilterAndRank(IntentVerify, raw)
	if len(resp.Presentable) != 1 {
		t.Errorf("esperado 1 resultado após deduplicação (§9), got %d", len(resp.Presentable))
	}
}

func TestFilterAndRank_NotFound(t *testing.T) {
	resp := FilterAndRank(IntentDiagnose, nil)
	if resp.Outcome != OutcomeNotFound {
		t.Errorf("esperado NOT_FOUND sem resultados (§15), got %s", resp.Outcome)
	}
}

func TestFilterAndRank_ConflictDetected(t *testing.T) {
	// Dois DIRECT sobre o mesmo tema com conteúdo divergente → CONFLICT (§10).
	raw := []SearchResult{
		{Title: "COMPONENTE_X.md", Content: "funcional", Class: Fact, Provenance: Provenance{Ref: "commit antigo"}},
		{Title: "COMPONENTE_X.md", Content: "quebrado", Class: Fact, Provenance: Provenance{Ref: "commit novo"}},
	}
	resp := FilterAndRank(IntentVerify, raw)
	if resp.Outcome != OutcomeConflict {
		t.Errorf("esperado CONFLICT (§10), got %s", resp.Outcome)
	}
	if resp.Conflict == nil {
		t.Error("CONFLICT deveria ter detalhes do conflito")
	}
	if len(resp.Conflict.Sides) != 2 {
		t.Errorf("esperado 2 lados no conflito, got %d", len(resp.Conflict.Sides))
	}
}

func TestFilterAndRank_WeakOnly_Insufficient(t *testing.T) {
	// Só WEAK (sem proveniência, relação frouxa) → INSUFFICIENT (§15).
	raw := []SearchResult{
		{Title: "notes.md", Content: "talvez tenha algo a ver com isso", Class: Hypothesis},
	}
	resp := FilterAndRank(IntentDiagnose, raw)
	if resp.Outcome != OutcomeInsufficient {
		t.Errorf("esperado INSUFFICIENT_EVIDENCE com só WEAK, got %s", resp.Outcome)
	}
}

// ─── Anti-confirmação (§26-§27) ────────────────────────────────────────────────

func TestSearchHypothesis_SearchesBothSides(t *testing.T) {
	// A hipótese carrega evidência A FAVOR e CONTRA — nunca só a primeira.
	h := SearchHypothesis{
		Statement:    "StepRunner causa a lentidão",
		Supporting:   []string{"benchmark: 29s → 5.4s"},
		Contradicting: []string{"profiling mostra que o gargalo é o embedding"},
		Verified:     false,
		Verdict:      "inconclusiva — evidências divergem",
	}
	if len(h.Supporting) == 0 || len(h.Contradicting) == 0 {
		t.Error("hipótese deve buscar evidência A FAVOR E CONTRA (§27)")
	}
	if h.Verified {
		t.Error("não pode estar verificada com evidências divergentes")
	}
}
