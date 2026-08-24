package search

// TestVectorEvidence_FullScanVsRouted — EXECUÇÃO do benchmark aprovado no
// AUDIT v2.1 (docs/reports/vectoragg-benchmark-design-2026-08-24.md §9).
//
// PAPEL EXATO: colher EVIDÊNCIA (números crus). NÃO interpreta, NÃO dá
// veredicto. Instrumentação 100% test-only; leitura somente (mode=ro).
//
//   - FULL-SCAN baseline: Engine.Search(ctx, params) com Scope=nil e
//     CandidateIDs vazio (IRRESTRINGÍVEL — nunca passa pelo resolver).
//   - ROTEADO: ApplyScope(resolver, query) → Scope roteado + CandidateIDs
//     derivado do domínio (module → pathHasSegment → JOIN document→vector →
//     TODOS os vector.id elegíveis).
//   - Ambos: EnableGraph=false, CandidatePool=0, MESMO Limit (top-K igual).
//
// GROUND-TRUTH NÃO-CIRCULAR: para cada query o GT (document.id exato + domínio
// + rota esperada) foi PRÉ-registrado ANTES de qualquer busca. Full-scan e
// roteado são coletados DEPOIS.
//
// NENHUM caminho de escrita: o DB abre com mode=ro; o vector store é montado
// sobre essa conexão (createTable IF NOT EXISTS é no-op sobre tabelas que já
// existem → não escreve). O corpus nunca é indexado/reindexado/apagado.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/vector"

	_ "modernc.org/sqlite"
)

const (
	benchDBPath = `C:\Users\Henrique\Documents\cosca\.cosca\knowledge.db`
	ollamaURL   = "http://localhost:11434/api/embeddings"
	ollamaModel = "nomic-embed-text"
)

// ── Ground-truth pré-registrado (ANTES de qualquer busca — não-circular) ─────

type benchGT struct {
	id           string   // document.id exato no corpus (RECALL_DOCUMENT: document_id)
	chunkID      string   // chunk.id exato no corpus (== vector.id; RECALL_CHUNK: chunk_id)
	module       string   // domínio (segmento do path) onde a evidência mora
	expectedPath string   // referência de path legível (documentação)
}

type benchQuery struct {
	n                 int
	text              string
	expectedModules   []string // rota esperada (aprovada no design)
	gt                benchGT  // ground-truth pré-registrado
	note              string   // observação honesta (ex.: file não indexado)
}

// Query set v3 — PRÉ-REGISTRADO (v3 §10.3/§10.4). Cada query expressa o
// CONTEÚDO real do chunk (linguagem do documento), e o ground-truth
// (document_id + chunk_id) foi fixado ANTES de qualquer busca. Queries foram
// redigidas lendo o texto real do chunk (read-only), NUNCA escolhidas depois de
// ver o resultado. Rota esperada = o módulo do path do documento onde a
// evidência mora; gate exact exige que Resolve(query).Modules == exatamente isso.
var benchmarkQueries = []benchQuery{
	{
		n: 1,
		text: "hot reload atualiza arquivos markdown sem reiniciar o runtime",
		expectedModules: []string{"runtime"},
		gt: benchGT{
			id:           "2f27dc0a-15f9-4086-8b65-cf015cc35399",
			chunkID:      "47659d61-351f-4d91-9889-d8fc5f31e643",
			module:       "runtime",
			expectedPath: `internal\embed\cosca\runtime\HOT_RELOAD.md`,
		},
		note: "Chunk real: 'Hot Reload enables live-updating Markdown files without restarting the Runtime'. Query em pt-br espelhando esse conteúdo.",
	},
	{
		n: 2,
		text: "continuidade da memoria do kernel entre execucoes",
		expectedModules: []string{"memory"},
		gt: benchGT{
			id:           "efd5b2c8-fd54-4243-aa92-07d74f97db41",
			chunkID:      "e84c5a50-0c7a-47ff-b54a-9bb92c760657",
			module:       "memory",
			expectedPath: `internal\embed\cosca\memory\agent\cosca-kernel\archive\L001-L214.md`,
		},
		note: "Chunk real (L34): 'Requisito do Don — Continuidade de Memória entre Sessões'. Palavra 'sessões' evitada na query porque o trigger dela roteia p/ runtime (gate exact exige um único módulo).",
	},
	{
		n: 3,
		text: "como funciona o motor de busca hibrida com fts5 e busca vetorial",
		expectedModules: []string{"knowledge"},
		gt: benchGT{
			id:           "aff80ddc-f855-4dd8-a0e0-7ac39f2a03bf",
			chunkID:      "abc97c26-456f-405f-add7-1e6fd159e519",
			module:       "knowledge",
			expectedPath: `docs\knowledge\overview.md`,
		},
		note: "Chunk real (diagrama): 'Query → HYBRID SEARCH ENGINE → Phase 1: FTS5 Full-Text ...'. Expressa o conteúdo real do doc (busca híbrida FTS5 + vetorial).",
	},
	{
		n: 4,
		text: "como o compute fabric gerencia os worker pools por load",
		expectedModules: []string{"architecture"},
		gt: benchGT{
			id:           "6a03e0f7-43be-43eb-b3fe-1093b4b6c48a",
			chunkID:      "93758a21-1ca2-45b5-a000-4da525c961c5",
			module:       "architecture",
			expectedPath: `docs\architecture\fabric-tuning.md`,
		},
		note: "Chunk real: 'O Compute Fabric ... gerencia os worker pools do Cosca ... com escala adaptativa por load'. Módulo architecture (docs/architecture).",
	},
	{
		n: 5,
		text: "como rodar benchmark de performance no cosca",
		expectedModules: []string{"cli"},
		gt: benchGT{
			id:           "98877065-b3cd-43a1-9ce0-c383bcc2c42a",
			chunkID:      "6a5911f3-f055-4d12-9d7d-439aa97a9c0f",
			module:       "cli",
			expectedPath: `docs\cli\commands.md`,
		},
		note: "Chunk real (comando `cosca benchmark`): 'Run performance benchmarks.' Palavra 'benchmark' roteia para cli (trigger adicionado na instrumentação).",
	},
	{
		n: 6,
		text: "maquina de estados de seguranca valida autoriza executa verifica audita",
		expectedModules: []string{"security"},
		gt: benchGT{
			id:           "2e89bfb6-a751-4bd7-8653-ba3828b2fbb1",
			chunkID:      "06d4d3f4-e0fb-4af4-be50-4a05d29719fd",
			module:       "security",
			expectedPath: `internal\embed\cosca\knowledge\security\README.md`,
		},
		note: "Chunk real: 'VALIDATE → AUTHORIZE → EXECUTE → VERIFY → AUDIT — nunca execute-e-verifique-depois'. Doc vive em knowledge/security (segmento security + knowledge); rota para security inclui o doc. Módulo gt registrado=security.",
	},
}

// ── Rotas test-only (pertencem ao benchmark; INSTRUMENTAÇÃO, não produção) ───

func benchmarkResolver() *modlink.Resolver {
	routes := []modlink.Route{
		{Trigger: "família", Module: "memory", Capability: "memory.search", Priority: 10},
		{Trigger: "memoria", Module: "memory", Capability: "memory.search", Priority: 10},
		{Trigger: "velocidade", Module: "memory", Capability: "memory.search", Priority: 10},
		{Trigger: "lei", Module: "memory", Capability: "memory.search", Priority: 10},
		{Trigger: "serve", Module: "runtime", Capability: "runtime.search", Priority: 20},
		{Trigger: "systemd", Module: "runtime", Capability: "runtime.search", Priority: 20},
		{Trigger: "serviço", Module: "runtime", Capability: "runtime.search", Priority: 20},
		{Trigger: "configurar", Module: "runtime", Capability: "runtime.search", Priority: 20},
		{Trigger: "sessões", Module: "runtime", Capability: "runtime.search", Priority: 20},
		{Trigger: "runtime", Module: "runtime", Capability: "runtime.search", Priority: 20},
		{Trigger: "gate", Module: "knowledge", Capability: "knowledge.search", Priority: 30},
		{Trigger: "integridade", Module: "knowledge", Capability: "knowledge.search", Priority: 30},
		{Trigger: "chain", Module: "knowledge", Capability: "knowledge.search", Priority: 30},
		{Trigger: "cosseno", Module: "knowledge", Capability: "knowledge.search", Priority: 30},
		{Trigger: "similaridade", Module: "knowledge", Capability: "knowledge.search", Priority: 30},
		{Trigger: "cálculo", Module: "knowledge", Capability: "knowledge.search", Priority: 30},
		{Trigger: "funciona", Module: "knowledge", Capability: "knowledge.search", Priority: 30},
		// v3 — rotas adicionais (instrumentação do benchmark p/ o novo query set):
		// architecture / cli / security (módulos com conteúdo no corpus).
		{Trigger: "fabric", Module: "architecture", Capability: "architecture.search", Priority: 40},
		{Trigger: "worker", Module: "architecture", Capability: "architecture.search", Priority: 40},
		{Trigger: "benchmark", Module: "cli", Capability: "cli.search", Priority: 50},
		{Trigger: "seguranca", Module: "security", Capability: "security.search", Priority: 60},
	}
	return modlink.NewResolver(routes)
}

// ── Abertura read-only + vector store ───────────────────────────────────────

func benchOpenRO(t interface{ Fatalf(string, ...interface{}) }) *sql.DB {
	dsn := "file:" + filepath.ToSlash(benchDBPath) + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open ro: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("ping ro: %v", err)
	}
	return db
}

// ── Embedding via Ollama (nomic-embed-text, 768-dim — o mesmo do corpus) ─────

func ollamaEmbed(prompt string) ([]float64, error) {
	body, _ := json.Marshal(map[string]any{"model": ollamaModel, "prompt": prompt})
	req, err := http.NewRequest(http.MethodPost, ollamaURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Embedding, nil
}

// ── Candidatos do domínio (exaustivos) via JOIN document→vector ──────────────

// benchCandidateIDs deriva o conjunto EXAUSTIVO de vector.id do(s) módulo(s)
// roteado(s): module → pathHasSegment(document.path) → JOIN → vector.id.
// replicando a exata lógica de pathHasSegment (search/scope.go).
func benchCandidateIDs(db *sql.DB, modules []string) ([]string, error) {
	rows, err := db.Query(`SELECT v.id, d.path FROM vectors v JOIN documents d ON v.document_id=d.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modSet := map[string]bool{}
	for _, m := range modules {
		modSet[strings.ToLower(m)] = true
	}
	out := make([]string, 0)
	seen := map[string]bool{}
	for rows.Next() {
		var id, path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		for seg := range segmentSet(path) {
			if modSet[seg] {
				if !seen[id] {
					seen[id] = true
					out = append(out, id)
				}
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// segmentSet devolve o conjunto de segmentos do path (case-insensitive).
func segmentSet(path string) map[string]bool {
	out := map[string]bool{}
	norm := filepath.ToSlash(path)
	for _, seg := range strings.Split(norm, "/") {
		if seg != "" {
			out[strings.ToLower(seg)] = true
		}
	}
	return out
}

// benchExhaustiveCount é a PROVA via SQL: COUNT(DISTINCT vector.id via JOIN)
// por módulo path-segment — deve igualar len(CandidateIDs).
func benchExhaustiveCount(db *sql.DB, module string) (int64, error) {
	var n int64
	q := `SELECT COUNT(DISTINCT v.id) FROM vectors v JOIN documents d ON v.document_id=d.id
		WHERE '/'||REPLACE(d.path,'\','/')||'/' LIKE '%/'||?||'/%'`
	if err := db.QueryRow(q, module).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// benchDocModule resolves o módulo real de um path (por path-segment).
func benchDocModule(path string) string {
	for seg := range segmentSet(path) {
		switch seg {
		case "memory", "knowledge", "architecture", "runtime", "cli", "security":
			return seg
		}
	}
	return "(none)"
}

// ── Coleta de métricas ───────────────────────────────────────────────────────

type benchMetrics struct {
	total     int
	scanned   int
	metadataC int
	k         int
	int8      bool
	float32   bool
	vecLat    time.Duration
}

type benchOutcome struct {
	valid            string
	motivoInvalid    string
	totalVectors     int
	candidateCount   int
	exhaustive       bool
	fullScanned      int
	routedScanned    int
	fullBLOBs        int
	routedBLOBs      int
	fullVecLat       time.Duration
	routedVecLat     time.Duration
	fullTotalLat     time.Duration
	routedTotalLat   time.Duration
	topK             int
	recallFull1      int
	recallRouted1    int
	recallFull5      int
	recallRouted5    int
	recallFull10     int
	recallRouted10   int
	recallChunkFull1    int
	recallChunkRouted1  int
	recallChunkFull5    int
	recallChunkRouted5  int
	recallChunkFull10   int
	recallChunkRouted10 int
	scopeModules     []string
	noRoute          bool
	scopeRouted      bool
}

// recallAt reports whether the GT document appears in the first `k` results.
// Nível 1 — RECALL_DOCUMENT@K (a métrica PRINCIPAL do v3): compara
// DocumentID == gt.document_id.
func recallAt(results []SearchResult, gtID string, k int) int {
	if k > len(results) {
		k = len(results)
	}
	for i := 0; i < k; i++ {
		if results[i].DocumentID == gtID {
			return 1
		}
	}
	return 0
}

// recallChunkAt reports whether the GT chunk appears in the first `k` results.
// Nível 2 — RECALL_CHUNK@K (a métrica SECUNDÁRIA do v3): exige o CHUNK
// específico pré-registrado. O `search.SearchResult` não expõe um campo
// ChunkID; aqui o chunk é identificado por `results[i].ID`, que é o id do
// vector store (`vectors.id`), e no corpus `vectors.id == chunk_id` (1:1 por
// chunk — verificado na diagnose read-only). Esse ID é o mesmo que o Nível 1
// usa via DocumentID, mas a comparação é por chunk exato.
func recallChunkAt(results []SearchResult, gtChunkID string, k int) int {
	if k > len(results) {
		k = len(results)
	}
	for i := 0; i < k; i++ {
		if results[i].ID == gtChunkID {
			return 1
		}
	}
	return 0
}

func TestVectorEvidence_FullScanVsRouted(t *testing.T) {
	db := benchOpenRO(t)
	defer db.Close()

	// Vector store sobre a conexão RO (createTable é no-op em tabelas existentes).
	vecStore, err := vector.NewSQLiteVec(vector.SQLiteVecConfig{DB: db, Dimension: 768})
	if err != nil {
		t.Fatalf("new sqlite vec: %v", err)
	}
	defer vecStore.Close()

	resolver := benchmarkResolver()

	// Embedding: chamado por query (uma vez).
	runQuery := func(q benchQuery) benchOutcome {
		out := benchOutcome{}
		ctx := context.Background()

		// 0. Gate de pré-execução: Resolve(query).Modules == esperado?
		scope := resolver.Resolve(q.text)
		out.scopeModules = append([]string(nil), scope.Modules...)
		out.noRoute = scope.NoRoute
		out.scopeRouted = vecScopeRouted(scope)
		gateOK := modulesEqual(scope.Modules, q.expectedModules)
		if !gateOK {
			out.valid = "INVALID"
			out.motivoInvalid = fmt.Sprintf("gate: Resolve(query).Modules=%v != esperado %v", scope.Modules, q.expectedModules)
			return out
		}

		// 1. Embutir a query (uma vez — mesmo vetor para os dois braços).
		vec, err := ollamaEmbed(q.text)
		if err != nil {
			out.valid = "INVALID"
			out.motivoInvalid = "embed falhou: " + err.Error()
			return out
		}
		_ = vec

		// ── FULL-SCAN (baseline IRRESTRINGÍVEL) ──
		fullParams := SearchParams{
			Query:        q.text,
			Limit:        10,
			EnableFTS:    false,
			EnableVector: true,
			EnableGraph:  false,
			CandidatePool: 0,
			// Scope NIL + CandidateIDs vazio = full-scan verdadeiro.
		}
		var fullM benchMetrics
		eng := NewEngine(nil, vecStore, nil, nil, func(ctx context.Context, text string) (*EmbeddingRequest, error) {
			return &EmbeddingRequest{Vector: vec}, nil
		})
		eng.MetricsSink = func(m vector.SearchMetrics) {
			fullM = benchMetrics{total: m.TotalVectors, scanned: m.ScannedVectors, metadataC: m.MetadataCandidates, k: m.ReturnedK, int8: m.Int8Enabled, float32: m.UsedFloat32, vecLat: m.Latency}
		}
		fullRes, ferr := eng.Search(ctx, fullParams)
		if ferr != nil {
			out.valid = "INVALID"
			out.motivoInvalid = "full-scan erro: " + ferr.Error()
			return out
		}
		out.fullScanned = fullM.scanned
		out.fullBLOBs = 0 // caminho default in-memory int8/index → não decodifica BLOB
		out.fullVecLat = fullM.vecLat
		out.fullTotalLat = fullRes.Duration

		// ── ROTEADO (ApplyScope + CandidateIDs derivado do domínio) ──
		routedParams, _ := ApplyScope(resolver, q.text, SearchParams{
			Query:        q.text,
			Limit:        10,
			EnableFTS:    false,
			EnableVector: true,
			EnableGraph:  false,
			CandidatePool: 0,
		})
		candidates, cerr := benchCandidateIDs(db, routedParams.Scope.Modules)
		if cerr != nil {
			out.valid = "INVALID"
			out.motivoInvalid = "candidate derive erro: " + cerr.Error()
			return out
		}
		out.candidateCount = len(candidates)
		out.topK = 10

		// Exaustividade (prova COUNT(JOIN) == len(CandidateIDs)) — para o scope.
		for _, m := range routedParams.Scope.Modules {
			cnt, e := benchExhaustiveCount(db, m)
			if e != nil {
				out.valid = "INVALID"
				out.motivoInvalid = "exhaustive count erro: " + e.Error()
				return out
			}
			_ = cnt // registrado por módulo no loop; interseção verificada no agregado abaixo
		}
		// Agregado: total exaustivo = soma por módulo; comparamos com len(set).
		var modTotal int64
		for _, m := range routedParams.Scope.Modules {
			cnt, _ := benchExhaustiveCount(db, m)
			modTotal += cnt
		}
		out.exhaustive = (len(candidates) > 0) && (int64(len(candidates)) == modTotal)

		if len(candidates) == 0 {
			out.valid = "INVALID"
			out.motivoInvalid = "len(CandidateIDs)==0 (anti '0 trabalho')"
			return out
		}

		routedParams.CandidateIDs = candidates
		var routedM benchMetrics
		eng2 := NewEngine(nil, vecStore, nil, nil, func(ctx context.Context, text string) (*EmbeddingRequest, error) {
			return &EmbeddingRequest{Vector: vec}, nil
		})
		eng2.MetricsSink = func(m vector.SearchMetrics) {
			routedM = benchMetrics{total: m.TotalVectors, scanned: m.ScannedVectors, metadataC: m.MetadataCandidates, k: m.ReturnedK, int8: m.Int8Enabled, float32: m.UsedFloat32, vecLat: m.Latency}
		}
		routedRes, rerr := eng2.Search(ctx, routedParams)
		if rerr != nil {
			out.valid = "INVALID"
			out.motivoInvalid = "roteado erro: " + rerr.Error()
			return out
		}
		out.routedScanned = routedM.scanned
		out.routedBLOBs = routedM.scanned // caminho float32 bound: decodifica candidato
		out.routedVecLat = routedM.vecLat
		out.routedTotalLat = routedRes.Duration

		out.totalVectors = fullM.total

		// Regra de validade §2.1.
		if !out.scopeRouted {
			out.valid = "INVALID"
			out.motivoInvalid = "scopeRouted==false"
			return out
		}
		if out.noRoute {
			out.valid = "INVALID"
			out.motivoInvalid = "NoRoute==true"
			return out
		}
		if len(candidates) == 0 {
			out.valid = "INVALID"
			out.motivoInvalid = "len(CandidateIDs)==0"
			return out
		}
		if !out.exhaustive {
			out.valid = "INVALID"
			out.motivoInvalid = "CandidateIDs NÃO exaustivo (COUNT(JOIN) != len)"
			return out
		}
		if out.routedScanned >= out.totalVectors {
			out.valid = "INVALID"
			out.motivoInvalid = fmt.Sprintf("ScannedVectors(%d) >= TotalVectors(%d) — sem trabalho reduzido", out.routedScanned, out.totalVectors)
			return out
		}
		out.valid = "VALID"

		// Recall@K (coletado DEPOIS, contra o GT pré-registrado).
		// Nível 1 — RECALL_DOCUMENT@K (document_id).
		// Nível 2 — RECALL_CHUNK@K (chunk_id específico pré-registrado).
		out.recallFull1 = recallAt(fullRes.Results, q.gt.id, 1)
		out.recallRouted1 = recallAt(routedRes.Results, q.gt.id, 1)
		out.recallFull5 = recallAt(fullRes.Results, q.gt.id, 5)
		out.recallRouted5 = recallAt(routedRes.Results, q.gt.id, 5)
		out.recallFull10 = recallAt(fullRes.Results, q.gt.id, 10)
		out.recallRouted10 = recallAt(routedRes.Results, q.gt.id, 10)

		out.recallChunkFull1 = recallChunkAt(fullRes.Results, q.gt.chunkID, 1)
		out.recallChunkRouted1 = recallChunkAt(routedRes.Results, q.gt.chunkID, 1)
		out.recallChunkFull5 = recallChunkAt(fullRes.Results, q.gt.chunkID, 5)
		out.recallChunkRouted5 = recallChunkAt(routedRes.Results, q.gt.chunkID, 5)
		out.recallChunkFull10 = recallChunkAt(fullRes.Results, q.gt.chunkID, 10)
		out.recallChunkRouted10 = recallChunkAt(routedRes.Results, q.gt.chunkID, 10)
		return out
	}

	fmt.Printf("\n===== BENCHMARK FULL-SCAN vs ROTEADO (números crus) =====\n")
	for _, q := range benchmarkQueries {
		res := runQuery(q)
		printBenchResult(t, q, res)
	}
}

func printBenchResult(t *testing.T, q benchQuery, r benchOutcome) {
	fmt.Printf("\n────────────────────────────────────────────────────────────\n")
	fmt.Printf("query: %s\n", q.text)
	fmt.Printf("ground_truth_document_id: %s   (pre-registrado)\n", q.gt.id)
	fmt.Printf("ground_truth_chunk_id: %s      (pre-registrado)\n", q.gt.chunkID)
	fmt.Printf("rota_esperada: %v   rota_resolvida: %v  (noRoute=%v scopeRouted=%v)\n", q.expectedModules, r.scopeModules, r.noRoute, r.scopeRouted)
	if q.note != "" {
		fmt.Printf("nota_gt: %s\n", q.note)
	}
	fmt.Printf("status: %s\n", r.valid)
	if r.valid != "VALID" {
		fmt.Printf("motivo: %s\n", r.motivoInvalid)
		fmt.Printf("  (demais campos = '-'); Top-K: %d; TotalVectors: %d; CandidateIDs: %d\n", r.topK, r.totalVectors, r.candidateCount)
		return
	}
	fmt.Printf("  TotalVectors: %d\n", r.totalVectors)
	fmt.Printf("  CandidateIDs (count): %d   / exaustividade COUNT(JOIN)==len: %v\n", r.candidateCount, r.exhaustive)
	fmt.Printf("  ScannedVectors: full=%d routed=%d\n", r.fullScanned, r.routedScanned)
	fmt.Printf("  latencia (vetorial): full=%s routed=%s\n", r.fullVecLat.Round(time.Microsecond), r.routedVecLat.Round(time.Microsecond))
	fmt.Printf("  latencia (search total): full=%s routed=%s\n", r.fullTotalLat.Round(time.Microsecond), r.routedTotalLat.Round(time.Microsecond))
	fmt.Printf("  Top-K: %d\n", r.topK)
	fmt.Printf("  RECALL_DOCUMENT@1/@5/@10: full=[%d %d %d]  routed=[%d %d %d]\n",
		r.recallFull1, r.recallFull5, r.recallFull10, r.recallRouted1, r.recallRouted5, r.recallRouted10)
	fmt.Printf("  RECALL_CHUNK@1/@5/@10:   full=[%d %d %d]  routed=[%d %d %d]\n",
		r.recallChunkFull1, r.recallChunkFull5, r.recallChunkFull10, r.recallChunkRouted1, r.recallChunkRouted5, r.recallChunkRouted10)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func vecScopeRouted(scope *modlink.SearchScope) bool {
	return scope != nil && !scope.NoRoute && len(scope.Modules) > 0
}

func modulesEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
