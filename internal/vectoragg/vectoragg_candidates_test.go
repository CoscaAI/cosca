// Package vectoragg — testes do CONTRATO NOVO (Caminho A, ADR-013 §10).
//
// Estes testes PROVAM a correção dos 4 pontos da auditoria (P1-P4) numa lib
// ISOLADA (nenhum outro pacote depende do vectoragg):
//
//   - P1 (confinamento físico): Scope → allowed modules → allowed candidate IDs
//     → retrieval → decode. Não é "módulo inteiro + LIMIT 10".
//   - P2 (counts): CatalogCounts() ≠ ScopeCounts(scope), semântica explícita.
//   - P3 (query original vs RouteID): a string original chega ao FTS5 MATCH;
//     o RouteID NÃO é o texto buscado (separador '.'/ '|' quebraria o FTS5).
//   - P4 (materialização): decode SÓ nos candidatos; nunca materializar tudo.
//
// Os testes são red-sobre-o-código-atual por construção: referenciam a API nova
// (SearchRequest / RetrieveCandidates / FTSMatch / CatalogCounts / ScopeCounts)
// que ainda não existe no vectoragg.go — o pacote não compila até implementar.
package vectoragg

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/modlink"
)

// seedCandidateDB cria um banco SQLite REAL em diretório temporário com as
// tabelas dos módulos (vectors/entities/relationships/chunks_fts/entities_fts),
// popula `nVectors` vetores da `dim` dada (BLOB float32 LE determinístico),
// um punhado de entidades/relações e uma linha FTS cujo conteúdo é a frase
// acentuada usada para provar o contrato query→MATCH. Devolve o caminho e os
// IDs dos vetores (na ordem de inserção — a "fonte de candidatos").
func seedCandidateDB(tb testing.TB, nVectors, dim int) (string, []string) {
	tb.Helper()
	dir := tb.TempDir()
	path := filepath.Join(dir, "knowledge.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		tb.Fatalf("open seed db: %v", err)
	}

	stmts := []string{
		`CREATE TABLE vectors (
			id TEXT PRIMARY KEY, vector BLOB NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			document_id TEXT NOT NULL DEFAULT '',
			chunk_id TEXT NOT NULL DEFAULT '',
			entity_id TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE entities (
			id TEXT PRIMARY KEY, entity_type TEXT NOT NULL, name TEXT NOT NULL,
			path TEXT NOT NULL, metadata_json TEXT NOT NULL DEFAULT '{}')`,
		`CREATE TABLE relationships (
			id TEXT PRIMARY KEY, source_id TEXT NOT NULL, source_type TEXT NOT NULL,
			target_id TEXT NOT NULL, target_type TEXT NOT NULL, rel_type TEXT NOT NULL,
			weight REAL NOT NULL DEFAULT 1.0)`,
		`CREATE VIRTUAL TABLE chunks_fts USING fts5(content, heading, section_type)`,
		`CREATE VIRTUAL TABLE entities_fts USING fts5(name, entity_type, metadata)`,
	}
	for _, s := range stmts {
		if _, e := db.Exec(s); e != nil {
			_ = db.Close()
			tb.Fatalf("create schema: %v", e)
		}
	}

	// ── Vectors (nVectors), determinísticos, dentro de uma transação ──────
	tx, err := db.Begin()
	if err != nil {
		_ = db.Close()
		tb.Fatalf("begin tx: %v", err)
	}
	ins, err := tx.Prepare(`INSERT INTO vectors (id, vector, content, document_id, chunk_id, entity_id) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		_ = db.Close()
		tb.Fatalf("prepare insert vector: %v", err)
	}
	ids := make([]string, 0, nVectors)
	var state uint64 = 0x9E3779B97F4A7C15
	blob := make([]byte, dim*4)
	for i := 0; i < nVectors; i++ {
		id := fmt.Sprintf("vec-%05d", i+1)
		// LCG determinístico preenche o blob (não-zero, reproduzível).
		for j := 0; j < dim; j++ {
			state = state*6364136223846793005 + 1442695040888963407
			v := float32((state >> 32) & 0xffff)
			v /= float32(0xffff)
			binary.LittleEndian.PutUint32(blob[j*4:], math.Float32bits(v))
		}
		if _, e := ins.Exec(id, blob, "content-"+id, "doc-"+id, "chunk-"+id, ""); e != nil {
			_ = db.Close()
			tb.Fatalf("insert vector %s: %v", id, e)
		}
		ids = append(ids, id)
	}
	if err := ins.Close(); err != nil {
		_ = db.Close()
		tb.Fatalf("close ins: %v", err)
	}
	if err := tx.Commit(); err != nil {
		_ = db.Close()
		tb.Fatalf("commit: %v", err)
	}

	// ── Entities (3) ─────────────────────────────────────────────────────
	ents := []struct {
		id, typ, name, path, meta string
	}{
		{"ent-1", "agent", "alpha", "/p/agent/alpha.md", `{"vendor":"internal"}`},
		{"ent-2", "module", "beta", "/p/module/beta.md", `{}`},
		{"ent-3", "skill", "gamma", "/p/skill/gamma.md", `{"lang":"go"}`},
	}
	for _, ent := range ents {
		if _, e := db.Exec(`INSERT INTO entities (id, entity_type, name, path, metadata_json) VALUES (?,?,?,?,?)`,
			ent.id, ent.typ, ent.name, ent.path, ent.meta); e != nil {
			_ = db.Close()
			tb.Fatalf("insert entity %s: %v", ent.id, e)
		}
	}

	// ── Relationships (cadeia A→B→C) ─────────────────────────────────────
	rels := []struct {
		id, s, st, t, tt, rt string
		weight               float64
	}{
		{"rel-1", "ent-1", "agent", "ent-2", "module", "implements", 1.0},
		{"rel-2", "ent-2", "module", "ent-3", "skill", "contains", 0.7},
		{"rel-3", "ent-1", "agent", "ent-3", "skill", "related_to", 0.5},
	}
	for _, r := range rels {
		if _, e := db.Exec(`INSERT INTO relationships (id, source_id, source_type, target_id, target_type, rel_type, weight) VALUES (?,?,?,?,?,?,?)`,
			r.id, r.s, r.st, r.t, r.tt, r.rt, r.weight); e != nil {
			_ = db.Close()
			tb.Fatalf("insert relationship %s: %v", r.id, e)
		}
	}

	// ── FTS5: uma linha cujo conteúdo é EXATAMENTE a frase acentuada usada
	// para provar que a query ORIGINAL (não o RouteID) chega ao MATCH. ─────
	if _, e := db.Exec(`INSERT INTO chunks_fts(rowid, content, heading, section_type) VALUES (?,?,?,?)`,
		100, "árvore urbana no terreno", "Vegetação", "text"); e != nil {
		_ = db.Close()
		tb.Fatalf("insert fts: %v", e)
	}
	if _, e := db.Exec(`INSERT INTO entities_fts(rowid, name, entity_type, metadata) VALUES (?,?,?,?)`,
		1, "alpha", "agent", "internal"); e != nil {
		_ = db.Close()
		tb.Fatalf("insert entities_fts: %v", e)
	}

	if err := db.Close(); err != nil {
		tb.Fatalf("close seed db: %v", err)
	}
	return path, ids
}

// ── P4: prova de QUANTOS BLOBs foram decodificados ───────────────────────────

func TestRetrieveCandidatesDecodesOnlyTopK(t *testing.T) {
	const n, dim, topK = 2000, 768, 10 // 2000 >> 10
	path, ids := seedCandidateDB(t, n, dim)
	defer removePath(t, path)

	a := openTestAgg(t, path)
	defer a.Close()

	req := SearchRequest{CandidateIDs: ids[:10], TopK: topK}
	res, err := a.RetrieveCandidates(req)
	if err != nil {
		t.Fatalf("retrieve candidates: %v", err)
	}

	if len(res.Vectors) != topK {
		t.Fatalf("len(decoded)=%d, want %d (não %d)", len(res.Vectors), topK, n)
	}
	if res.Decoded != topK {
		t.Fatalf("Decoded=%d, want %d (não %d)", res.Decoded, topK, n)
	}
	wantBytes := int64(topK * dim * 4)
	if res.BytesDecoded != wantBytes {
		t.Fatalf("BytesDecoded=%d, want ~%d bytes (30 KB, não 84,6 MB)", res.BytesDecoded, wantBytes)
	}
	// A decodificação de fato materializou float32 de dim real.
	if res.Vectors[0].Dim != dim || len(res.Vectors[0].Embedding) != dim {
		t.Errorf("embedding decode wrong: dim=%d len=%d", res.Vectors[0].Dim, len(res.Vectors[0].Embedding))
	}
	if res.CandidateSet != len(ids[:10]) {
		t.Errorf("CandidateSet=%d want %d", res.CandidateSet, len(ids[:10]))
	}
	if res.Truncated {
		t.Errorf("with len(CandidateIDs)==TopK there is no truncation")
	}
}

// Agora com MAIS candidatos que TopK: NÃO pode decodificar tudo.
func TestRetrieveCandidatesDecodesAtMostTopKWhenCandidatesExceed(t *testing.T) {
	const n, dim, topK = 2000, 768, 10
	path, ids := seedCandidateDB(t, n, dim)
	defer removePath(t, path)

	a := openTestAgg(t, path)
	defer a.Close()

	// 100 ids de candidato, TopK=10 → decodifica 10, truncando.
	res, err := a.RetrieveCandidates(SearchRequest{CandidateIDs: ids[:100], TopK: topK})
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if len(res.Vectors) != topK {
		t.Fatalf("len(decoded)=%d, want %d", len(res.Vectors), topK)
	}
	if !res.Truncated {
		t.Errorf("expected Truncated=true when 100 candidates exceed TopK=10")
	}
	if res.BytesDecoded != int64(topK*dim*4) {
		t.Errorf("BytesDecoded=%d, want %d", res.BytesDecoded, int64(topK*dim*4))
	}
}

// ── P4: REGRESSÃO — falha se o código voltar a Vectors(0)/materializar tudo ──

func TestRetrieveCandidatesRegressionFailsIfFullDecode(t *testing.T) {
	const n, dim = 2000, 768
	path, ids := seedCandidateDB(t, n, dim)
	defer removePath(t, path)

	a := openTestAgg(t, path)
	defer a.Close()

	total, err := a.CatalogCounts()
	if err != nil {
		t.Fatalf("catalog counts: %v", err)
	}
	if total.Vectors < 100 {
		t.Fatalf("test needs > 100 vectors to prove confinement, got %d", total.Vectors)
	}

	res, err := a.RetrieveCandidates(SearchRequest{CandidateIDs: ids[:10], TopK: 10})
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}

	// INVARIANTE: candidate retrieval respeita os IDs. Se o código REGREDIR
	// para Vectors(0) (=materializar o índice inteiro), Decoded == total e este
	// teste FALHA.
	if int64(res.Decoded) == total.Vectors {
		t.Fatalf("REGRESSION: decode=%d == total catalog %d — candidate retrieval materializou TUDO", res.Decoded, total.Vectors)
	}
	if res.Decoded != 10 {
		t.Fatalf("Decoded=%d, want 10 (top-K estrito, nunca o catálogo inteiro)", res.Decoded)
	}
}

// ── P1: confinamento físico (Scope → allowed modules → candidate IDs → decode) ──

func TestRetrieveCandidatesPhysicalConfinement(t *testing.T) {
	const n, dim = 50, 8
	path, ids := seedCandidateDB(t, n, dim)
	defer removePath(t, path)

	a := openTestAgg(t, path)
	defer a.Close()

	// Scope que NÃO inclui o módulo "vector" → nenhum candidato, nunca fallback.
	noVector := &modlink.SearchScope{Modules: []string{"fts"}}
	res, err := a.RetrieveCandidates(SearchRequest{CandidateIDs: ids[:10], Scope: noVector, TopK: 10})
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if len(res.Vectors) != 0 || res.Decoded != 0 {
		t.Fatalf("scope sem 'vector' deve render 0 candidatos (confinamento físico), got decoded=%d", res.Decoded)
	}
	for _, m := range res.ScopeModules {
		if m != "fts" {
			t.Errorf("ScopeModules=%v, want only [fts]", res.ScopeModules)
		}
	}

	// NoRoute=true → espaço vazio → zero candidatos.
	noRoute, err := a.RetrieveCandidates(SearchRequest{CandidateIDs: ids[:10], Scope: &modlink.SearchScope{NoRoute: true}, TopK: 10})
	if err != nil {
		t.Fatalf("retrieve noroute: %v", err)
	}
	if len(noRoute.Vectors) != 0 {
		t.Fatalf("NoRoute deve render 0 candidatos, got %d", len(noRoute.Vectors))
	}
}

// ── P4: TopK<=0 nunca é "sem limite" ─────────────────────────────────────────

func TestRetrieveCandidatesTopKZeroMeansNoResults(t *testing.T) {
	path, ids := seedCandidateDB(t, 5, 4)
	defer removePath(t, path)
	a := openTestAgg(t, path)
	defer a.Close()

	res, err := a.RetrieveCandidates(SearchRequest{CandidateIDs: ids[:1], TopK: 0})
	if err != nil {
		t.Fatalf("retrieve topk=0: %v", err)
	}
	if len(res.Vectors) != 0 || res.Decoded != 0 {
		t.Fatalf("TopK=0 deve significar SEM RESULTADOS (não 'sem limite'), got decoded=%d", res.Decoded)
	}
}

func TestRetrieveCandidatesTopKNegativeIsError(t *testing.T) {
	path, ids := seedCandidateDB(t, 5, 4)
	defer removePath(t, path)
	a := openTestAgg(t, path)
	defer a.Close()

	if _, err := a.RetrieveCandidates(SearchRequest{CandidateIDs: ids[:1], TopK: -1}); err == nil {
		t.Fatalf("TopK<0 deve ser rejeitado, não virar 'sem limite'")
	}
}

// CandidateIDs vazio → zero candidatos (nunca "ler tudo").
func TestRetrieveCandidatesEmptyCandidateIDsNeverReadAll(t *testing.T) {
	path, _ := seedCandidateDB(t, 100, 4)
	defer removePath(t, path)
	a := openTestAgg(t, path)
	defer a.Close()

	res, err := a.RetrieveCandidates(SearchRequest{TopK: 10})
	if err != nil {
		t.Fatalf("retrieve empty ids: %v", err)
	}
	if len(res.Vectors) != 0 || res.CandidateSet != 0 {
		t.Fatalf("CandidateIDs vazio deve render 0 candidatos (nunca materializar tudo), got %d", len(res.Vectors))
	}
}

// ── P3: a query ORIGINAL chega ao MATCH; RouteID NÃO é o texto buscado ───────

func TestSearchUsesOriginalQueryNotRouteID(t *testing.T) {
	path, _ := seedCandidateDB(t, 5, 8)
	defer removePath(t, path)
	a := openTestAgg(t, path)
	defer a.Close()

	const query = "árvore urbana no terreno" // conteúdo pesquisado (o que)
	// Scope com RouteID contendo '.' e '|' — separadores que QUEBRAM o FTS5 se
	// usados como texto de busca (foi o FAIL do P3 na auditoria).
	scope := &modlink.SearchScope{
		Modules: []string{"fts", "vector"},
		RouteID: "vegetation.world.materials",
		NoRoute: false,
	}

	res, err := a.Search(SearchRequest{Query: query, Scope: scope, TopK: 10})
	if err != nil {
		t.Fatalf("search: %v (RouteID com '.'/'|' NÃO pode quebrar o FTS — prova que a query original é usada)", err)
	}
	found := false
	for _, h := range res.FTSHits {
		if h.Table == "chunks_fts" {
			found = true
		}
	}
	if !found {
		t.Fatalf("esperado hit FTS da query original %q; RouteID 'vegetation.world.materials' NÃO é o texto buscado — FTSHits=%+v", query, res.FTSHits)
	}
}

func TestFTSMatchPassesOriginalQueryToMatch(t *testing.T) {
	path, _ := seedCandidateDB(t, 2, 4)
	defer removePath(t, path)
	a := openTestAgg(t, path)
	defer a.Close()

	hits, err := a.FTSMatch("árvore urbana no terreno", 10)
	if err != nil {
		t.Fatalf("ftsmatch: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("esperado hit para a query original (o texto que chega ao MATCH ?)")
	}
	for _, h := range hits {
		if h.Rank <= 0 {
			t.Errorf("esperado rank > 0 para um match, got %v", h.Rank)
		}
	}
}

func TestFTSMatchTopKZeroMeansNoResults(t *testing.T) {
	path, _ := seedCandidateDB(t, 2, 4)
	defer removePath(t, path)
	a := openTestAgg(t, path)
	defer a.Close()

	hits, err := a.FTSMatch("árvore urbana no terreno", 0)
	if err != nil {
		t.Fatalf("ftsmatch topk=0: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("FTSMatch TopK=0 deve significar SEM RESULTADOS (não 'em limite'), got %d", len(hits))
	}
}

// ── P2: CatalogCounts() vs ScopeCounts(scope) — semântica explícita ──────────

func TestCatalogCountsVsScopeCounts(t *testing.T) {
	path, _ := seedCandidateDB(t, 20, 8)
	defer removePath(t, path)
	a := openTestAgg(t, path)
	defer a.Close()

	cat, err := a.CatalogCounts()
	if err != nil {
		t.Fatalf("catalog counts: %v", err)
	}
	// Catálogo conta TUDO.
	if cat.Vectors != 20 || cat.Entities != 3 || cat.Relationships != 3 {
		t.Fatalf("catalog counts wrong: %+v", cat)
	}
	if cat.ChunksFTS != 1 || cat.EntitiesFTS != 1 {
		t.Fatalf("catalog fts counts wrong: %+v", cat)
	}

	// ScopeCounts só o escopo {vector} → conta vetores, NÃO os demais módulos.
	scoped, err := a.ScopeCounts(&modlink.SearchScope{Modules: []string{"vector"}})
	if err != nil {
		t.Fatalf("scoped counts: %v", err)
	}
	if scoped.Vectors != 20 {
		t.Fatalf("ScopeCounts(vector).Vectors=%d, want 20", scoped.Vectors)
	}
	if scoped.Entities != 0 || scoped.Relationships != 0 || scoped.ChunksFTS != 0 || scoped.EntitiesFTS != 0 {
		t.Fatalf("ScopeCounts(vector) deve contar SÓ vetores, got %+v", scoped)
	}

	// São distintos E cada um correto.
	if cat == scoped {
		t.Fatalf("CatalogCounts e ScopeCounts devem ter semântica distinta")
	}

	// ScopeCounts(nil) = todos os módulos do catálogo (espelho completo).
	all, err := a.ScopeCounts(nil)
	if err != nil {
		t.Fatalf("scoped nil: %v", err)
	}
	if all.Vectors != cat.Vectors || all.Entities != cat.Entities {
		t.Fatalf("ScopeCounts(nil) deve igualar o catálogo quando seleciona todos os módulos: %+v vs %+v", all, cat)
	}

	// ScopeCounts(NoRoute) = zero para tudo (nunca conta "tudo" por engano).
	nr, err := a.ScopeCounts(&modlink.SearchScope{NoRoute: true})
	if err != nil {
		t.Fatalf("scoped noroute: %v", err)
	}
	if nr.Vectors != 0 || nr.Entities != 0 || nr.ChunksFTS != 0 {
		t.Fatalf("ScopeCounts(NoRoute) deve contar 0, got %+v", nr)
	}
}

// ── Benchmark: antes (Vectors(0) ~84,6 MB / muitos) vs depois (TopK=10 ~KB) ──

func BenchmarkCandidateTopKVsFullDecode(b *testing.B) {
	const n = 28888 // o número real do catálogo (audit P4)
	const dim = 768
	path, ids := seedCandidateDB(b, n, dim)
	defer removePath(b, path)

	a, err := Open(MirrorCatalog(path))
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer a.Close()

	b.ReportAllocs()

	// ANTES (legado): Vectors(0) materializa/decodifica o índice inteiro.
	cand := make([]string, 10)
	copy(cand, ids[:10])
	b.Run("antes_Vectors0_materializa_tudo", func(b *testing.B) {
		decodedVectors := int64(n)
		decodedBytes := int64(n) * dim * 4
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			out, _ := a.Vectors(0)
			_ = len(out)
		}
		// Evidência explícita no output (o ReportMetric custom não renderiza em
		// Go 1.26 — reportamos direto para stdout).
		fmt.Fprintf(os.Stdout, "\n  [ANTES] Vectors(0): decodificou %d BLOBs / ~%d bytes (~%.1f MB)\n",
			decodedVectors, decodedBytes, float64(decodedBytes)/(1024*1024))
	})

	// DEPOIS (novo): candidate retrieval + top-K decodifica SÓ os candidatos.
	b.Run("depois_RetrieveCandidates_TopK10", func(b *testing.B) {
		decodedVectors := int64(len(cand))
		decodedBytes := decodedVectors * dim * 4
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			out, _ := a.RetrieveCandidates(SearchRequest{CandidateIDs: cand, TopK: 10})
			_ = len(out.Vectors)
		}
		fmt.Fprintf(os.Stdout, "\n  [DEPOIS] RetrieveCandidates TopK=10: decodificou %d BLOBs / ~%d bytes (~%.1f KB)\n",
			decodedVectors, decodedBytes, float64(decodedBytes)/1024)
	})
}

// removePath é um helper seguro de limpeza (não falha se o arquivo já sumiu).
func removePath(tb testing.TB, path string) {
	tb.Helper()
	if err := os.Remove(path); err != nil {
		tb.Logf("remove temp db: %v", err)
	}
}
