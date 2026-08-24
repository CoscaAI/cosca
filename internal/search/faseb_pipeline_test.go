// Package search — FASE B (PROVA / TDD RED).
//
// Este arquivo NÃO implementa a conexão `search ↔ vectoragg`. Ele escreve a
// PROVA que a Fase B deve satisfazer: o invariante da Fase A (não materializar
// os 28.888 embeddings do pipeline quando um `Scope` está roteado).
//
// ORDEM DO PROFESSOR: MAPEAR → PROVAR → IMPLEMENTAR. Estamos em **PROVAR**.
//
// O que estes 2 testes expressam (o contrato desejado da Fase B):
//
//	Teste 1 — TestSearchWithRoute_DoesNotMaterializeFullVectorIndex
//	  Quando um `Scope` é roteado (modlink) com módulos válidos E há
//	  `CandidateIDs` (a fonte de candidatos), o pipeline de busca NÃO pode
//	  full-scan / materializar o índice inteiro de vetores. A busca DEVE
//	  delegar o retrieval confinado ao `vectoragg.RetrieveCandidates`
//	  (que decodifica só os candidatos, provedores da TopK), como provado na
//	  Fase A (`internal/vectoragg/vectoragg_candidates_test.go`).
//
//	Teste 2 — TestSearchWithRoute_FallsBackToFullScan_WhenNoScope
//	  SEM scope roteado (NoRoute → Modules vazio), o full-scan é LEGÍTIMO.
//	  Confirma que o Teste 1 só exige confinamento quando há scope — nunca
//	  como regra universal.
//
// Estado HOJE (por que os 2 são RED/GREEN):
//   - `SearchWithRoute` (scope.go) resolve a rota e injeta o `Scope`, mas NÃO
//     gera `CandidateIDs` a partir do scope e NÃO delega ao retriever do
//     `vectoragg`. A fase vetorial (`searchVector`) só confina quando
//     `resolveChunkCandidates` devolve candidatos — e esse caminho exige um
//     cliente FTS para traduzir `chunks_fts_<rowid>`, e ainda assim é o
//     mecanismo interno legado, NÃO o `vectoragg`.
//   - Quando o `search` chama `SearchWithMetrics(..., nil, 0, ...)` (candidatos
//     nulos), faz FULL-SCAN — exatamente o "palheiro" que a Fase A eliminou.
//
//   => Teste 1 FALHA hoje (RED): com `Scope` roteado, a fase vetorial observada
//      recebe candidatos NIL (full-scan), quebrando o invariante.
//   => Teste 2 PASSA hoje (GREEN): sem scope roteado, o full-scan é o
//      comportamento de linha de base permitido.
//
// Não toca vectoragg/modlink/oracle/migrations/embed. Não implementa nada.
package search

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/vector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// metricsVectorStore é um vector.Store que também expõe a superfície
// instrumentada `MetricsSearcher` e REGISTRA o conjunto de candidatos que a
// fase vetorial recebeu. Isso permite ao teste PROVAR, de forma observável, se
// a busca vetorial full-scanou (candidatos nil) ou confinou aos candidatos.
//
// TotalVectors representa o tamanho do catálogo (matamos o "84,6 MB" / 28.888
// da auditoria; usamos um número representativo menor — o contrato é o mesmo).
type metricsVectorStore struct {
	called         bool
	candidateIDs   []string
	scannedVectors int
	totalVectors   int
}

func (m *metricsVectorStore) Store(_ int, _ []vector.VectorRecord) error { return nil }
func (m *metricsVectorStore) Delete(_ []string) error                    { return nil }
func (m *metricsVectorStore) DeleteByDocument(_ string) error            { return nil }
func (m *metricsVectorStore) DeleteByEntity(_ string) error              { return nil }
func (m *metricsVectorStore) Rebuild() error                             { return nil }
func (m *metricsVectorStore) Stats() (vector.VectorStats, error)         { return vector.VectorStats{}, nil }
func (m *metricsVectorStore) Dimension() int                             { return 128 }
func (m *metricsVectorStore) Count() (int, error)                        { return m.totalVectors, nil }
func (m *metricsVectorStore) Close() error                               { return nil }

func (m *metricsVectorStore) Search(_ []float64, _ int) ([]vector.SearchResult, error) {
	m.called = true
	m.candidateIDs = nil
	m.scannedVectors = m.totalVectors
	return []vector.SearchResult{{ID: "full-1", Score: 0.7, ChunkID: "chunk-1"}}, nil
}

func (m *metricsVectorStore) SearchWithFilter(_ []float64, _ int, _ map[string]string) ([]vector.SearchResult, error) {
	m.called = true
	m.candidateIDs = nil
	m.scannedVectors = m.totalVectors
	return []vector.SearchResult{{ID: "full-1", Score: 0.7, ChunkID: "chunk-1"}}, nil
}

// SearchWithMetrics (MetricsSearcher) registra o conjunto de candidatos e a
// quantidade efetivamente escaneada. Candidatos NIL ⇒ full-scan de todo o
// catálogo (ScannedVectors == TotalVectors); candidatos não-vazios ⇒ caminho
// híbrido/bounded (ScannedVectors << TotalVectors) — o alvo do L3/Fase B.
func (m *metricsVectorStore) SearchWithMetrics(_ []float64, _ int, candidateIDs []string, _ int, _ map[string]string) ([]vector.SearchResult, vector.SearchMetrics, error) {
	m.called = true
	m.candidateIDs = append([]string(nil), candidateIDs...)
	scanned := m.totalVectors
	if len(candidateIDs) > 0 {
		scanned = len(candidateIDs)
	}
	m.scannedVectors = scanned
	results := []vector.SearchResult{{ID: "vec-1", Score: 0.5, Content: "hit", ChunkID: "chunk-1"}}
	metrics := vector.SearchMetrics{
		TotalVectors:   m.totalVectors,
		ScannedVectors: scanned,
		ReturnedK:      len(results),
	}
	return results, metrics, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// TESTE 1 — PROVA do INVARIANTE (RED hoje)
// ─────────────────────────────────────────────────────────────────────────────

// TestSearchWithRoute_DoesNotMaterializeFullVectorIndex prova que, quando um
// `Scope` é roteado (modlink) com um módulo válido E há `CandidateIDs` (a fonte
// de candidatos, no vocabulário do `vectoragg`), a busca NÃO pode full-scan /
// materializar o índice inteiro de vetores. A fase vetorial DEVE receber o
// conjunto de candidatos confinado (delegado ao `vectoragg.RetrieveCandidates`
// na Fase B), e NÃO um candidato nulo.
//
// FALHA HOJE (RED): o `search` não está fiado ao `vectoragg`; `SearchWithRoute`
// injeta o `Scope` mas NÃO gera candidatos dele, e o mecanismo interno
// (`resolveChunkCandidates`) só traduz `chunks_fts_<rowid>` (exige FTS), nunca
// os IDs de vetor permitidos que o `vectoragg` aceita. Resultado: a fase
// vetorial é chamada com candidatos NIL (full-scan), quebrando o invariante.
func TestSearchWithRoute_DoesNotMaterializeFullVectorIndex(t *testing.T) {
	const catalogSize = 2000 // representativo do catálogo (a auditoria mediu 28.888)
	store := &metricsVectorStore{totalVectors: catalogSize}
	engine := NewEngine(nil, store, nil, nil, stubEmbedFunc)

	// Rota determinística válida → SearchScope.Modules = ["memory"].
	resolver := modlink.NewResolver([]modlink.Route{
		{Trigger: "memoria da familia", Module: "memory", Capability: "memory.search", Priority: 10},
	})

	// CandidateIDs são os IDs de vetor PERMITIDOS (a fonte de candidatos) — o
	// mesmo vocabulário de vectoragg.SearchRequest{Query, Scope, CandidateIDs, TopK}.
	// A query original é a que carrega "o quê"; o Scope é "onde".
	params := SearchParams{
		Query:        "memoria da familia",
		Limit:        20,
		EnableFTS:    false,
		EnableVector: true,
		EnableGraph:  false,
		CandidateIDs: []string{"vec-0001", "vec-0002"},
	}

	res, err := SearchWithRoute(context.Background(), engine, resolver, "memoria da familia", params)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, store.called, "a fase vetorial deve estar ativa")

	// INVARIANTE da Fase A: com Scope roteado + CandidateIDs, a fase vetorial
	// NÃO pode full-scan (candidatos NIL) nem materializar o índice inteiro.
	assert.NotEmpty(t, store.candidateIDs,
		"com um Scope roteado, a fase vetorial DEVE receber os candidatos confinados "+
			"(delegados ao vectoragg.RetrieveCandidates), NÃO um candidato NIL/full-scan")
	assert.Less(t, store.scannedVectors, store.totalVectors,
		"com um Scope roteado a busca NÃO pode materializar o índice inteiro "+
			"(full-scan); deve decodificar SÓ os candidatos")
}

// ─────────────────────────────────────────────────────────────────────────────
// TESTE 2 — CONTRAPESO de regressão (GREEN hoje)
// ─────────────────────────────────────────────────────────────────────────────

// TestSearchWithRoute_FallsBackToFullScan_WhenNoScope é o contrapeso do Teste 1:
// SEM scope roteado (NoRoute → Modules vazio) o full-scan é LEGÍTIMO. Confirma
// que o Teste 1 só exige confinamento quando há um scope, nunca como regra
// universal — até o `vectoragg`/Fase A "nunca materializar tudo" vale para o
// caminho confinado, não para a busca sem roteamento.
func TestSearchWithRoute_FallsBackToFullScan_WhenNoScope(t *testing.T) {
	const catalogSize = 2000
	store := &metricsVectorStore{totalVectors: catalogSize}
	engine := NewEngine(nil, store, nil, nil, stubEmbedFunc)

	// Resolver que NÃO tem a rota da query → NoRoute → SearchScope.Modules vazio
	// → busca ilimitada / retrocompatível (o invariante do professor).
	resolver := modlink.NewResolver([]modlink.Route{
		{Trigger: "memoria da familia", Module: "memory", Capability: "memory.search", Priority: 10},
	})

	params := SearchParams{
		Query:        "musica eletronica", // não casa com a rota conhecida
		Limit:        20,
		EnableFTS:    false,
		EnableVector: true,
		EnableGraph:  false,
		CandidateIDs: []string{"vec-0001", "vec-0002"},
	}

	res, err := SearchWithRoute(context.Background(), engine, resolver, "musica eletronica", params)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, store.called, "a fase vetorial deve estar ativa")

	// SEM scope roteado, o full-scan é a linha de base permitida — NÃO é o bug
	// que o Teste 1 combate (aquele é do caminho COM scope).
	assert.Equal(t, store.totalVectors, store.scannedVectors,
		"sem scope roteado o full-scan é o comportamento legítimo (baseline)")
}
