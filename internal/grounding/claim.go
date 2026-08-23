// Package grounding implements the RAG fidelity gates (ADR-011, fatia G1).
//
// It is the "RAG grounded" layer, deliberately separated from the knowledge
// engine: it does NOT re-implement retrieval. It consumes the retrieved
// chunks (as SourceChunk) and produces a fidelity verdict — how much of an
// answer is actually supported by the retrieved evidence — plus a per-claim
// attribution (claim → source chunk + matching terms).
//
// Everything here is deterministic and zero-LLM: no model is called by the
// gates themselves. The only LLM touch-point is BuildAnswer, which only runs
// the generative function when chunks ARE present ("sem evidência não gera").
//
// Design notes (honest, from the ADR §2/§6):
//   - C4/self-RAG heurístico first (token-overlap + n-grama + número) — ~1ms,
//     zero-LLM. No local HHEM NLI in this fatia (fase 2, ErrGateNotWired).
//   - C3/fail-open como invariante: faithfulness == -1 (scorer indisponível)
//     NUNCA bloqueia; "sem evidência não gera" degrada para extrativo.
package grounding

// SourceChunk é um chunk recuperado do motor de busca, já reduzido ao que o
// gate de fidelidade precisa. Deriva de search.SearchResult (conversão feita
// pelo chamador) — o grounding não importa internal/search para não acoplar
// os dois conceitos.
type SourceChunk struct {
	// ChunkID é o identificador estável do chunk (o que o qrels usa).
	ChunkID string
	// DocumentID é o id do documento-pai do chunk.
	DocumentID string
	// DocumentPath é o caminho do documento-pai (trilha de auditoria).
	DocumentPath string
	// Content é o texto do chunk. Necessário para o token-overlap do
	// VerifyClaim — sem o conteúdo textual não há como verificar se um claim
	// está efetivamente sustentado. (Aditivo: não está no esboço do ADR, mas
	// a verificação heurística não existe sem ele.)
	Content string
	// MatchingTerms são os termos da query que casaram com este chunk
	// (populados pelo chamador a partir do ranking, quando disponível).
	MatchingTerms []string
	// Score é a pontuação de relevância do chunk no ranking (BM25/cosíno/etc).
	Score float64
}

// Claim é uma afirmação extraída de uma resposta para ser verificada contra
// os chunks recuperados.
type Claim struct {
	// Text é o texto da afirmação (normalmente uma sentença da resposta).
	Text string
	// HasCitation indica se a sentença veio com um marcador de citação
	// ([RAG:...], [Chunk N], [K-xxxx]).
	HasCitation bool
	// CitationMarker é o marcador de citação bruto (ex.: "[Chunk 7]").
	CitationMarker string
	// Substantiated é o resultado da verificação: true quando o claim está
	// sustentado pelo melhor chunk (score >= threshold). Preenchido pelo
	// Verify*; para um claim recém-extraído é false (verificação pendente).
	Substantiated bool
}

// ClaimVerdict é o resultado da verificação de UM claim contra os chunks:
// o melhor chunk que o sustenta, os termos que casaram e a pontuação.
type ClaimVerdict struct {
	// Claim é o claim verificado (com Substantiated preenchido).
	Claim Claim
	// BestChunkIdx é o índice (em chunks) do chunk mais relacionado ao claim,
	// ou -1 quando nenhum chunk o sustenta (Score 0).
	BestChunkIdx int
	// MatchingTerms são os termos do claim que casaram com o best chunk.
	MatchingTerms []string
	// Score é a pontuação de suporte do melhor chunk para este claim.
	Score float64
	// Supported é true quando Score >= threshold de verificação.
	Supported bool
}
