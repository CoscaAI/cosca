package oracle

import "testing"

// ─── Testes do protocolo de busca ─────────────────────────────────────────────

func TestSearchRequest_Valid_RejectsGarbage(t *testing.T) {
	cases := []struct {
		name string
		req  SearchRequest
		ok   bool
	}{
		{"válida", SearchRequest{Intent: IntentDiagnose, Query: "por que lento", Sources: []Source{SourceMemory}}, true},
		{"sem intenção", SearchRequest{Query: "lixo", Sources: []Source{SourceMemory}}, false},
		{"intenção desconhecida", SearchRequest{Intent: "hackear", Query: "x", Sources: []Source{SourceMemory}}, false},
		{"query vazia", SearchRequest{Intent: IntentDiagnose, Sources: []Source{SourceMemory}}, false},
		{"sem fonte", SearchRequest{Intent: IntentDiagnose, Query: "x"}, false},
		{"fonte inválida", SearchRequest{Intent: IntentDiagnose, Query: "x", Sources: []Source{"darkweb"}}, false},
	}
	for _, c := range cases {
		ok, _ := c.req.Valid()
		if ok != c.ok {
			t.Errorf("%s: esperado ok=%v, got %v", c.name, c.ok, ok)
		}
	}
}

func TestSearchResult_Valid_RequiresProvenanceForFact(t *testing.T) {
	// Fact sem proveniência → inválido (não pode ser promovido).
	r := SearchResult{Title: "X", Content: "Y", Class: Fact}
	if r.Valid() {
		t.Error("Fact sem proveniência deveria ser inválido")
	}
	// Fact com proveniência → válido.
	r = SearchResult{Title: "X", Content: "Y", Class: Fact, Provenance: Provenance{Ref: "abc123"}}
	if !r.Valid() {
		t.Error("Fact com proveniência deveria ser válido")
	}
	// Hypothesis sem proveniência → válido (é só hipótese, não promovida).
	r = SearchResult{Title: "X", Content: "Y", Class: Hypothesis}
	if !r.Valid() {
		t.Error("Hypothesis sem proveniência deveria ser válida (não é promovida a fato)")
	}
}

// ─── Testes do bloqueio (Gate) ────────────────────────────────────────────────

func TestGate_RejectsEmptyPackage(t *testing.T) {
	g := NewGate()
	v := g.Evaluate(SemanticPackage{})
	if v.Decision != Reject {
		t.Errorf("esperado REJECT para pacote vazio, got %s", v.Decision)
	}
}

func TestGate_RejectsNoIntent(t *testing.T) {
	g := NewGate()
	v := g.Evaluate(SemanticPackage{Result: "achei X"})
	if v.Decision != Reject {
		t.Errorf("esperado REJECT sem intenção, got %s", v.Decision)
	}
}

func TestGate_RejectsHighConfidenceWithoutEvidence(t *testing.T) {
	g := NewGate()
	v := g.Evaluate(SemanticPackage{
		Intent:     "diagnosticar lentidão",
		Result:     "StepRunner causa a lentidão",
		Confidence: ConfHigh,
	})
	if v.Decision != Reject {
		t.Errorf("esperado REJECT para HIGH sem evidência, got %s", v.Decision)
	}
	if len(v.Required) == 0 {
		t.Error("REJECT deveria listar o que falta (Required)")
	}
}

func TestGate_RejectsHighConfidenceWithoutProvenance(t *testing.T) {
	g := NewGate()
	v := g.Evaluate(SemanticPackage{
		Intent:     "diagnosticar lentidão",
		Result:     "StepRunner causa a lentidão",
		Evidence:   "benchmark: 29s → 5.4s",
		Confidence: ConfHigh,
		// sem Provenance
	})
	if v.Decision != Reject {
		t.Errorf("esperado REJECT para HIGH sem proveniência, got %s", v.Decision)
	}
}

func TestGate_AcceptsWithEvidence(t *testing.T) {
	g := NewGate()
	v := g.Evaluate(SemanticPackage{
		Intent:     "diagnosticar lentidão",
		Result:     "StepRunner executava build/test por request",
		Source:     "codebase",
		Provenance: "commit d8f5c7c",
		Evidence:   "benchmark: 29s → 5.4s após desativar",
		Context:    "produção",
		Confidence: ConfHigh,
	})
	if v.Decision != Accept {
		t.Errorf("esperado ACCEPT com evidência completa, got %s", v.Decision)
	}
}

func TestGate_AcceptsWithCaveatWhenMemoryContradicts(t *testing.T) {
	g := NewGate()
	g.MemoryCheck = func(pkg SemanticPackage) (string, bool) {
		return "memória diz COMPONENTE X = funcional, pacote diz quebrado", true
	}
	v := g.Evaluate(SemanticPackage{
		Intent:     "verificar componente X",
		Result:     "COMPONENTE X = quebrado",
		Source:     "codebase",
		Provenance: "commit abc",
		Evidence:   "teste falha",
		Confidence: ConfMedium,
	})
	if v.Decision != AcceptWithCaveat {
		t.Errorf("esperado ACCEPT_WITH_CAVEAT com contradição, got %s", v.Decision)
	}
	if len(v.Questions) == 0 {
		t.Error("contradição deveria gerar perguntas (§16)")
	}
}

func TestGate_AcceptWithCaveatLowConfidence(t *testing.T) {
	g := NewGate()
	v := g.Evaluate(SemanticPackage{
		Intent:     "explorar domínio",
		Result:     "encontrei padrões X, Y, Z",
		Confidence: ConfLow,
	})
	if v.Decision != AcceptWithCaveat {
		t.Errorf("esperado ACCEPT_WITH_CAVEAT para exploração low-confidence, got %s", v.Decision)
	}
}

// ─── Inconclusive: o oráculo nunca inventa conclusão ──────────────────────────

func TestGate_NeverInvents(t *testing.T) {
	// O oráculo não tem caminho que produza INCONCLUSIVE com invenção:
	// sem evidência suficiente, a resposta é REJECT (falta contrato) ou
	// ACCEPT_WITH_CAVEAT (exploração) — nunca um FACT inventado.
	g := NewGate()
	v := g.Evaluate(SemanticPackage{
		Intent: "diagnosticar",
		Result: "talvez seja X, ou Y, não sei",
	})
	// Result existe mas sem evidência/proveniência/confiança → deve ser
	// caveat (não ACCEPT cego, não FACT).
	if v.Decision == Accept {
		t.Error("nunca ACCEPT sem evidência suficiente")
	}
}

// ─── INCONCLUSIVE (§14, §28): nunca inventar conclusão ─────────────────────────

func TestGate_InconclusiveWhenNoEvidenceAtAll(t *testing.T) {
	g := NewGate()
	v := g.Evaluate(SemanticPackage{
		Intent: "diagnosticar lentidão",
		Result: "achei algo no pipeline",
		// sem evidence, sem provenance, sem confidence
	})
	if v.Decision != Inconclusive {
		t.Errorf("esperado INCONCLUSIVE sem nenhuma evidência, got %s", v.Decision)
	}
	if len(v.Questions) == 0 {
		t.Error("INCONCLUSIVE deveria fazer perguntas (§19)")
	}
}

// ─── §10: resultado ≠ verdade (correlação ≠ causa) ────────────────────────────

func TestGate_CausalClaimWithoutCausalEvidence(t *testing.T) {
	g := NewGate()
	v := g.Evaluate(SemanticPackage{
		Intent:     "diagnosticar lentidão",
		Result:     "StepRunner causou a lentidão",
		Source:     "codebase",
		Provenance: "commit abc",
		Evidence:   "observei o código e vi o StepRunner",
		Confidence: ConfHigh,
	})
	if v.Decision != AcceptWithCaveat {
		t.Errorf("esperado ACCEPT_WITH_CAVEAT para causa sem evidência causal, got %s", v.Decision)
	}
	found := false
	for _, g := range v.SemanticGaps {
		if contains(g, "correlação") || contains(g, "causa") {
			found = true
		}
	}
	if !found {
		t.Errorf("SemanticGaps deveria mencionar correlação≠causa, got %v", v.SemanticGaps)
	}
}

func TestGate_CausalClaimWithCausalEvidence_Accepted(t *testing.T) {
	g := NewGate()
	v := g.Evaluate(SemanticPackage{
		Intent:     "diagnosticar lentidão",
		Result:     "StepRunner causou a lentidão",
		Source:     "codebase",
		Provenance: "commit abc",
		Evidence:   "benchmark antes/depois: 29s → 5.4s ao desativar build/test por request",
		Confidence: ConfHigh,
	})
	if v.Decision != Accept {
		t.Errorf("esperado ACCEPT com evidência causal (benchmark antes/depois), got %s", v.Decision)
	}
}

func TestGate_NoCausalClaim_NotFlagged(t *testing.T) {
	g := NewGate()
	v := g.Evaluate(SemanticPackage{
		Intent:     "mapear domínio",
		Result:     "encontrei os arquivos X, Y, Z",
		Source:     "codebase",
		Provenance: "commit abc",
		Evidence:   "grep nos arquivos",
		Confidence: ConfMedium,
	})
	if v.Decision != Accept {
		t.Errorf("esperado ACCEPT sem afirmação causal, got %s", v.Decision)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
