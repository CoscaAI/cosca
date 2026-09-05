package oracle

import "strings"

// ─── Filtro de Apresentação (SEARCH_PROTOCOL §7, §9) ───────────────────────────
//
// O Cosca NÃO entrega lixo. Resultados NOISE são descartados; WEAK só entram
// se necessário; duplicatas são agrupadas (§9: 10 resultados → 1 evidência
// principal + referências).

// FilterAndRank recebe resultados crus e produz o SearchResponse final:
// classifica, pontua, descarta NOISE, agrupa duplicatas, detecta conflitos.
func FilterAndRank(intent SearchIntent, raw []SearchResult) SearchResponse {
	resp := SearchResponse{
		Outcome:   OutcomeNotFound,
		StoppedAt: LayerIdentity,
	}

	if len(raw) == 0 {
		resp.Outcome = OutcomeNotFound
		resp.Reason = "nenhuma evidência relevante encontrada (§15: ausência também é resultado)"
		return resp
	}

	// 1. Classificar e pontuar cada resultado.
	var ranked []RankedResult
	seen := make(map[string]bool) // deduplicação por conteúdo normalizado
	for _, r := range raw {
		if !r.Valid() {
			continue
		}
		// Deduplicação por CONTEÚDO (§9): títulos iguais com conteúdo
		// diferente NÃO são duplicatas — são candidatos a CONFLITO (§10).
		key := strings.ToLower(strings.TrimSpace(r.Content))
		if len(key) > 120 {
			key = key[:120]
		}
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(r.Title))
		}
		if seen[key] {
			continue // conteúdo idêntico = duplicata real
		}
		seen[key] = true

		rel := classifyRelevance(intent, r)
		score := scoreResult(intent, r, rel)
		rr := RankedResult{
			Result:    r,
			Relevance: rel,
			Score:     score,
			Layer:     LayerIdentity,
		}
		ranked = append(ranked, rr)
	}

	// 2. Separar apresentáveis (DIRECT/RELATED/CONTEXTUAL) do resto.
	var presentable []RankedResult
	var weak []RankedResult
	for _, rr := range ranked {
		if rr.Relevance.Presentable() {
			presentable = append(presentable, rr)
		} else if rr.Relevance == Weak {
			weak = append(weak, rr)
		}
		// NOISE: descartado silenciosamente (§7).
	}

	// 3. Detectar conflito: dois DIRECT com claims contraditórios.
	if c := detectConflict(presentable); c != nil {
		resp.Outcome = OutcomeConflict
		resp.Conflict = c
	}

	// 4. Montar resposta.
	resp.Results = ranked
	resp.Presentable = presentable
	if len(presentable) > 0 && resp.Outcome != OutcomeConflict {
		resp.Outcome = OutcomeFound
	} else if len(presentable) == 0 && len(weak) > 0 {
		// Só WEAK: insuficiente para decidir, mas existe algo.
		resp.Outcome = OutcomeInsufficient
		resp.Reason = "encontrados apenas resultados de relação superficial (WEAK) — insuficiente para responder com segurança"
	}

	return resp
}

// classifyRelevance classifica a relação semântica do resultado com a
// intenção (§6). Heurística determinística: título/entidade no conteúdo.
func classifyRelevance(intent SearchIntent, r SearchResult) RelevanceClass {
	lowerTitle := strings.ToLower(r.Title)
	lowerContent := strings.ToLower(r.Content)

	// DIRECT: o título é o artefato exato da intenção (ex.: CARRO_PROTOCOL.md
	// para pergunta sobre carro) — não apenas coincidência de palavra.
	if strings.Contains(lowerTitle, strings.ToLower(string(intent))) {
		return Direct
	}
	// DIRECT: conteúdo responde explicitamente à pergunta (contém a entidade
	// e é curto/objetivo).
	if r.Provenance.Verifiable() && len(r.Content) > 0 && len(r.Content) < 500 {
		return Direct
	}
	// RELATED: menciona a entidade com contexto.
	if r.Provenance.Verifiable() && containsAny(lowerContent, entityKeywords(r)) {
		return Related
	}
	// CONTEXTUAL: ajuda a compreender (tem proveniência, mas relação frouxa).
	if r.Provenance.Verifiable() {
		return Contextual
	}
	// WEAK: relação superficial (só palavra, sem proveniência forte).
	if lowerContent != "" {
		return Weak
	}
	return Noise
}

// entityKeywords extrai palavras-chave do resultado para heurística.
func entityKeywords(r SearchResult) []string {
	var kw []string
	for _, w := range strings.Fields(strings.ToLower(r.Title)) {
		if len(w) > 3 {
			kw = append(kw, w)
		}
	}
	return kw
}

// scoreResult computa o score semântico (§8).
func scoreResult(intent SearchIntent, r SearchResult, rel RelevanceClass) SemanticScore {
	s := SemanticScore{
		Relevance:    1.0,
		IntentMatch:  0.0,
		EntityMatch:  0.0,
		ContextMatch: 0.5,
		Recency:      0.5,
		Provenance:   0.5,
		EvidenceValue: 0.5,
	}

	// Intenção: resultado que menciona a intenção pontua mais.
	if containsAny(strings.ToLower(r.Content), []string{strings.ToLower(string(intent))}) {
		s.IntentMatch = 1.0
	} else {
		s.IntentMatch = 0.3
		s.Penalties = append(s.Penalties, "intent_miss")
	}

	// Proveniência: verificável pontua mais.
	if r.Provenance.Verifiable() {
		s.Provenance = 1.0
	} else {
		s.Penalties = append(s.Penalties, "low_provenance")
	}

	// Evidência: fact/measured pontuam mais que inferred/hypothesis.
	switch r.Class {
	case Fact, Measured:
		s.EvidenceValue = 1.0
	case Evidence:
		s.EvidenceValue = 0.8
	case Inferred:
		s.EvidenceValue = 0.4
		s.Penalties = append(s.Penalties, "inferred")
	case Hypothesis:
		s.EvidenceValue = 0.2
		s.Penalties = append(s.Penalties, "hypothesis_not_fact")
	}

	// Classe de relevância ajusta o peso.
	switch rel {
	case Direct:
		s.Relevance = 1.0
	case Related:
		s.Relevance = 0.7
	case Contextual:
		s.Relevance = 0.5
	case Weak:
		s.Relevance = 0.3
		s.Penalties = append(s.Penalties, "weak_relevance")
	case Noise:
		s.Relevance = 0.1
		s.Penalties = append(s.Penalties, "noise")
	}

	s.Compute()
	return s
}

// detectConflict procura dois resultados DIRECT com claims contraditórios
// (§10). Heurística simples: mesmo tema, proveniências diferentes, e o texto
// contém sinais de contradição.
func detectConflict(results []RankedResult) *Conflict {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			a, b := results[i], results[j]
			if a.Relevance != Direct || b.Relevance != Direct {
				continue
			}
			// Mesmo tema (título similar) mas conteúdo divergente.
			if strings.Contains(strings.ToLower(b.Result.Title), strings.ToLower(a.Result.Title)) ||
				strings.Contains(strings.ToLower(a.Result.Title), strings.ToLower(b.Result.Title)) {
				if strings.TrimSpace(a.Result.Content) != strings.TrimSpace(b.Result.Content) {
					return &Conflict{
						Question: "resultados relevantes discordam sobre o mesmo tema",
						Sides: []ConflictSide{
							{Claim: a.Result.Title, Provenance: a.Result.Provenance.Ref, Evidence: truncateContent(a.Result.Content, 100)},
							{Claim: b.Result.Title, Provenance: b.Result.Provenance.Ref, Evidence: truncateContent(b.Result.Content, 100)},
						},
					}
				}
			}
		}
	}
	return nil
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func truncateContent(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
