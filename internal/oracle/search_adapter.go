package oracle

import "strings"

// ─── Adapter: motor de busca → classificação semântica ────────────────────────
//
// Conecta o SEARCH_PROTOCOL (classificação DIRECT/RELATED/CONTEXTUAL/WEAK/
// NOISE) ao motor de busca real (internal/search). O motor retorna por score
// BM25/vetor; o adapter classifica por RELEVÂNCIA SEMÂNTICA com a intenção —
// "text match ≠ semantic relevance" (SEARCH_PROTOCOL §5).

// RawHit é a projeção mínima de um resultado do motor, independente do tipo
// concreto (search.SearchResult, knowledge.Result, etc.).
type RawHit struct {
	// Rank é a posição original do motor.
	Rank int
	// Title do resultado.
	Title string
	// Type do resultado (document, chunk, entity...).
	Type string
	// Score do motor (BM25/vetor).
	Score float64
	// Snippet do resultado.
	Snippet string
	// Path do documento de origem.
	Path string
}

// RankedHit é um resultado do motor com classificação semântica aplicada.
type RankedHit struct {
	// Rank é a posição original do motor.
	Rank int
	// Title do resultado.
	Title string
	// Type do resultado.
	Type string
	// Score do motor (BM25/vetor).
	Score float64
	// Snippet do resultado.
	Snippet string
	// Path do documento de origem.
	Path string
	// Relevance é a classificação semântica (§6).
	Relevance RelevanceClass
	// Presentable reporta se deve ser mostrado (§7: NOISE fora).
	Presentable bool
}

// ClassifyHits aplica a classificação semântica a resultados crus do motor.
// Intent é a intenção da busca (SEARCH_PROTOCOL §1). Quando a intenção é
// genérica (explore), Query carrega o termo real — o classificador usa o
// termo da query como sinal de DIRECT (título = artefato da pergunta).
func ClassifyHits(intent SearchIntent, query string, raw []RawHit) []RankedHit {
	hits := make([]RankedHit, 0, len(raw))
	// Sinal de relevância: a query quando a intenção é genérica, senão a
	// própria intenção.
	signal := string(intent)
	if intent == IntentExplore && query != "" {
		signal = query
	}
	for _, r := range raw {
		rel := classifyRawRelevance(signal, r.Title, r.Snippet, r.Score)
		hits = append(hits, RankedHit{
			Rank:        r.Rank,
			Title:       r.Title,
			Type:        r.Type,
			Score:       r.Score,
			Snippet:     r.Snippet,
			Path:        r.Path,
			Relevance:   rel,
			Presentable: rel.Presentable(),
		})
	}
	return hits
}

// classifyRawRelevance classifica um resultado cru — heurística determinística
// sobre título + snippet + score. TEXT MATCH ≠ SEMANTIC RELEVANCE (§5).
// signal é o termo da intenção (ou a query quando a intenção é exploratória).
// O score do motor pode ser negativo (BM25) — o TÍTULO é o sinal mais forte.
func classifyRawRelevance(signal, title, snippet string, score float64) RelevanceClass {
	lowerTitle := strings.ToLower(title)
	lowerSnippet := strings.ToLower(snippet)
	sig := strings.ToLower(strings.TrimSpace(signal))

	// DIRECT: o título É o artefato da pergunta (ex.: CARRO_PROTOCOL.md para
	// "carro") — o sinal mais forte, independente do score BM25.
	if sig != "" && (strings.Contains(lowerTitle, sig) || strings.Contains(lowerTitle, strings.ReplaceAll(sig, " ", "_"))) {
		return Direct
	}

	// DIRECT: snippet contém o termo e o score é positivo (motor confirmou).
	if sig != "" && strings.Contains(lowerSnippet, sig) && score >= 0 {
		return Direct
	}

	// RELATED: score positivo — o motor já ranqueou por similaridade.
	if score >= 0 {
		return Related
	}

	// CONTEXTUAL: score negativo (BM25) mas com conteúdo — pode ajudar.
	if title != "" || snippet != "" {
		return Contextual
	}

	// Sem informação → WEAK (não inventar DIRECT).
	return Weak
}
