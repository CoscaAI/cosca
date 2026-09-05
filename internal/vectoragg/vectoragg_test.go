// Package vectoragg — testes do agregador READ-ONLY (espelho, ADR-013 §6).
//
// Estes testes PROVAM o contrato de leitura em três frentes:
//
//  1. LEITURA REAL — um banco SQLite é criado num diretório temporário com as
//     tabelas de módulos (`vectors`, `entities`, `relationships`, `chunks_fts`,
//     `entities_fts`), populado, e o agregador lê e devolve as projeções
//     tipadas corretas (contagens e conteúdo).
//
//  2. ATTACH DE AUSENTE — catalogar um arquivo que não existe (ou um arquivo
//     que não é um banco SQLite válido) deve devolver um ERRO CLARO (nunca um
//     panic nem uma leitura degradada silenciosa).
//
//  3. NÃO-ESCRITA — depois de um passe COMPLETO de leitura pelo agregador, o
//     arquivo .db não cresceu, o conjunto de tabelas é idêntico e uma escrita
//     direta numa conexão `mode=ro` falha na camada do SQLite. Isso prova que o
//     contrato read-only é real e imposto por construção.
package vectoragg

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/modlink"
)

// buildTestDB cria um banco REAL em disco (diretório temporário) com as tabelas
// dos módulos e devolve o caminho absoluto + uma conexão de escrita para o
// teste popular os dados. O banco é criado ANTES de o agregador abri-lo (a
// escrita é do teste; o agregador só LÊ).
func buildTestDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "knowledge.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
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
		if _, err := db.Exec(s); err != nil {
			_ = db.Close()
			t.Fatalf("create schema: %v", err)
		}
	}

	seedTestData(t, db)

	if err := db.Close(); err != nil {
		t.Fatalf("close test db: %v", err)
	}
	return path
}

// seedTestData popula as tabelas com dados conhecidos para asserção.
func seedTestData(t *testing.T, db *sql.DB) {
	t.Helper()

	// ── Vectors (2) — dim=4 float32 LE ────────────────────────────────────
	mkVec := func(vals ...float32) []byte {
		b := make([]byte, 4*len(vals))
		for i, v := range vals {
			binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
		}
		return b
	}
	vecs := []struct {
		id, content, doc, chunk, entity string
		vec                              []float32
	}{
		{"vec-1", "oracle database engine internals", "doc-1", "chunk-1", "", []float32{1, 0, 0, 0}},
		{"vec-2", "postgres query planner", "doc-2", "chunk-2", "ent-1", []float32{0, 1, 0, 0}},
	}
	for _, v := range vecs {
		if _, err := db.Exec(`INSERT INTO vectors (id, vector, content, document_id, chunk_id, entity_id) VALUES (?,?,?,?,?,?)`,
			v.id, mkVec(v.vec...), v.content, v.doc, v.chunk, v.entity); err != nil {
			t.Fatalf("insert vector %s: %v", v.id, err)
		}
	}

	// ── Entities (3) ──────────────────────────────────────────────────────
	ents := []struct {
		id, typ, name, path, meta string
	}{
		{"ent-1", "agent", "alpha", "/p/agent/alpha.md", `{"vendor":"internal"}`},
		{"ent-2", "module", "beta", "/p/module/beta.md", `{}`},
		{"ent-3", "skill", "gamma", "/p/skill/gamma.md", `{"lang":"go"}`},
	}
	for _, e := range ents {
		if _, err := db.Exec(`INSERT INTO entities (id, entity_type, name, path, metadata_json) VALUES (?,?,?,?,?)`,
			e.id, e.typ, e.name, e.path, e.meta); err != nil {
			t.Fatalf("insert entity %s: %v", e.id, err)
		}
	}

	// ── Relationships (cadeia A→B→C) ──────────────────────────────────────
	rels := []struct {
		id, s, st, t, tt, rt string
		weight               float64
	}{
		{"rel-1", "ent-1", "agent", "ent-2", "module", "implements", 1.0},
		{"rel-2", "ent-2", "module", "ent-3", "skill", "contains", 0.7},
		{"rel-3", "ent-1", "agent", "ent-3", "skill", "related_to", 0.5},
	}
	for _, r := range rels {
		if _, err := db.Exec(`INSERT INTO relationships (id, source_id, source_type, target_id, target_type, rel_type, weight) VALUES (?,?,?,?,?,?,?)`,
			r.id, r.s, r.st, r.t, r.tt, r.rt, r.weight); err != nil {
			t.Fatalf("insert relationship %s: %v", r.id, err)
		}
	}

	// ── FTS5 (chunks + entities) ──────────────────────────────────────────
	// Standalone FTS5: populate directly (the aggregator reads whatever FTS
	// table exists; external-content mode is a storage detail irrelevant here).
	fts := []struct {
		table, sql string
		args       []any
	}{
		{"chunks_fts", `INSERT INTO chunks_fts(rowid, content, heading, section_type) VALUES (?,?,?,?)`,
			[]any{1, "oracle database engine internals", "Oracle", "text"}},
		{"chunks_fts", `INSERT INTO chunks_fts(rowid, content, heading, section_type) VALUES (?,?,?,?)`,
			[]any{2, "postgres query planner", "Postgres", "text"}},
		{"entities_fts", `INSERT INTO entities_fts(rowid, name, entity_type, metadata) VALUES (?,?,?,?)`,
			[]any{1, "alpha", "agent", "internal"}},
		{"entities_fts", `INSERT INTO entities_fts(rowid, name, entity_type, metadata) VALUES (?,?,?,?)`,
			[]any{2, "beta", "module", ""}},
		{"entities_fts", `INSERT INTO entities_fts(rowid, name, entity_type, metadata) VALUES (?,?,?,?)`,
			[]any{3, "gamma", "skill", "go"}},
	}
	for _, f := range fts {
		if _, err := db.Exec(f.sql, f.args...); err != nil {
			t.Fatalf("insert fts %s: %v", f.table, err)
		}
	}
}

// openTestAgg abre o agregador sobre o banco de teste usando o ESPELHO
// (todos os módulos lógicos apontam para o mesmo arquivo knowledge.db).
func openTestAgg(t *testing.T, path string) *Aggregator {
	t.Helper()
	a, err := Open(MirrorCatalog(path))
	if err != nil {
		t.Fatalf("open aggregator: %v", err)
	}
	return a
}

// ── 1. LEITURA REAL ─────────────────────────────────────────────────────────

func TestAggregatorReadsTypedProjections(t *testing.T) {
	path := buildTestDB(t)
	a := openTestAgg(t, path)
	defer a.Close()

	if got := a.Modules(); len(got) != 4 {
		t.Fatalf("expected 4 logical modules, got %d: %v", len(got), got)
	}

	// Contagens de cardinalidade
	c, err := a.Counts()
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if c.Vectors != 2 || c.Entities != 3 || c.Relationships != 3 {
		t.Errorf("unexpected counts: %+v", c)
	}
	if c.ChunksFTS != 2 || c.EntitiesFTS != 3 {
		t.Errorf("unexpected fts counts: %+v", c)
	}

	// Projeção tipada de vetores (com embedding decodificado)
	vecs, err := a.Vectors(0)
	if err != nil {
		t.Fatalf("vectors: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
	if vecs[0].Dim != 4 || len(vecs[0].Embedding) != 4 {
		t.Errorf("vector embedding decode wrong: dim=%d len=%d", vecs[0].Dim, len(vecs[0].Embedding))
	}
	if vecs[0].RefID != "chunk-1" || vecs[1].RefID != "ent-1" {
		t.Errorf("ref resolution wrong: %q %q", vecs[0].RefID, vecs[1].RefID)
	}
	if vecs[1].EntityID != "ent-1" {
		t.Errorf("expected entity_id ent-1, got %q", vecs[1].EntityID)
	}

	// Projeção tipada de entidades (com metadata JSON decodificada)
	ents, err := a.Entities(0)
	if err != nil {
		t.Fatalf("entities: %v", err)
	}
	if len(ents) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(ents))
	}
	if ents[0].Name != "alpha" || ents[0].Metadata["vendor"] != "internal" {
		t.Errorf("entity projection wrong: %+v", ents[0])
	}

	// Projeção tipada de relacionamentos
	rels, err := a.Relationships(0)
	if err != nil {
		t.Fatalf("relationships: %v", err)
	}
	if len(rels) != 3 {
		t.Fatalf("expected 3 relationships, got %d", len(rels))
	}

	// Distância real no grafo (re-ranking)
	if d, ok := a.GraphDistance("ent-1", "ent-2"); !ok || d != 1 {
		t.Errorf("GraphDistance ent-1→ent-2 = %d,%v want 1,true", d, ok)
	}
	if d, ok := a.GraphDistance("ent-1", "ent-3"); !ok || d != 1 {
		t.Errorf("GraphDistance ent-1→ent-3 = %d,%v want 1,true (direct edge)", d, ok)
	}
	if d, ok := a.GraphDistance("ent-1", "missing"); ok || d != -1 {
		t.Errorf("GraphDistance to missing = %d,%v want -1,false", d, ok)
	}

	// BM25 sobre o módulo FTS
	hits, err := a.FTSBM25("oracle", 10)
	if err != nil {
		t.Fatalf("ftsbm25: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected at least 1 fts hit for 'oracle'")
	}
	foundChunk := false
	for _, h := range hits {
		if h.Table == "chunks_fts" {
			foundChunk = true
			if h.Rank <= 0 {
				t.Errorf("expected positive rank for a match, got %v", h.Rank)
			}
		}
	}
	if !foundChunk {
		t.Errorf("expected a chunks_fts hit, got %+v", hits)
	}
}

// ── 2. ATTACH DE AUSENTE / INVÁLIDO (erro claro, sem panic) ──────────────────

func TestOpenMissingModuleReturnsClearError(t *testing.T) {
	path := buildTestDB(t)
	defer os.Remove(path)

	// "graph" aponta para um arquivo que NÃO existe.
	missing := filepath.Join(t.TempDir(), "graph-missing.db")
	catalog := ModuleCatalog{
		ModuleVector: path,
		ModuleGraph:  missing, // não existe
		ModuleFTS:    path,
	}
	a, err := Open(catalog)
	if err == nil {
		_ = a.Close()
		t.Fatalf("expected a clear error for a missing module file")
	}
	if got := err.Error(); !strings.Contains(got, "not found") && !strings.Contains(got, "missing") && !strings.Contains(got, "unreadable") {
		t.Errorf("error should mention the missing file, got: %v", err)
	}
}

func TestOpenInvalidDatabaseReturnsClearError(t *testing.T) {
	// Um arquivo que existe mas NÃO é um banco SQLite válido.
	path := buildTestDB(t)
	defer os.Remove(path)

	garbage := filepath.Join(t.TempDir(), "fake.db")
	if err := os.WriteFile(garbage, []byte("this is not a sqlite database"), 0o600); err != nil {
		t.Fatalf("write garbage db: %v", err)
	}
	defer os.Remove(garbage)

	catalog := ModuleCatalog{
		ModuleVector: path,
		ModuleFTS:    garbage, // existe, mas não é SQLite
	}
	a, err := Open(catalog)
	if err == nil {
		_ = a.Close()
		t.Fatalf("expected a clear error for an invalid (non-SQLite) module file")
	}
}

// ── 3. CONTRATO READ-ONLY (não-escrita PROVADA) ─────────────────────────────

func TestReadOnlyNoWriteProven(t *testing.T) {
	path := buildTestDB(t)
	defer os.Remove(path)

	// Conjunto de tabelas ANTES (via conexão mode=ro separada).
	tablesBefore := dbTables(t, path)
	sizeBefore := fileSize(t, path)

	// Passe COMPLETO de leitura pelo agregador.
	a := openTestAgg(t, path)
	_, _ = a.Counts()
	_, _ = a.Vectors(0)
	_, _ = a.Entities(0)
	_, _ = a.Relationships(0)
	_, _ = a.FTSBM25("oracle", 10)
	_, _ = a.GraphDistance("ent-1", "ent-3")
	_, _ = a.QueryScope(&modlink.SearchScope{Modules: []string{"vector", "graph", "fts"}})
	if err := a.Close(); err != nil {
		t.Fatalf("close aggregator: %v", err)
	}

	// Depois do passe, TUDO deve ser idêntico: o arquivo não cresceu e o
	// conjunto de tabelas não mudou (nenhuma escrita foi persistida).
	tablesAfter := dbTables(t, path)
	sizeAfter := fileSize(t, path)

	if sizeAfter != sizeBefore {
		t.Errorf("database file grew: before=%d after=%d (READ-ONLY violated)", sizeBefore, sizeAfter)
	}
	if len(tablesAfter) != len(tablesBefore) {
		t.Errorf("table set changed: before=%d after=%d (READ-ONLY violated)", len(tablesBefore), len(tablesAfter))
	}
	for i := range tablesBefore {
		if tablesBefore[i] != tablesAfter[i] {
			t.Errorf("table set changed at %d: %q -> %q", i, tablesBefore[i], tablesAfter[i])
		}
	}

	// Prova de que a conexão mode=ro em si não aceita escrita (o contrato é
	// imposto pela engine, não só por intenção).
	if err := probeReadonlyWriteRejected(t, path); err != nil {
		t.Errorf("read-only enforcement: %v", err)
	}
}

// ── 4. ROUTED SCOPE (ADR-013 §3.2) ──────────────────────────────────────────

func TestQueryScopeConfinement(t *testing.T) {
	path := buildTestDB(t)
	defer os.Remove(path)

	a := openTestAgg(t, path)
	defer a.Close()

	// (a) scope nil → espelho completo (todos os módulos). Como não há query
	// de roteamento, o FTS não é consultado (FTS exige um MATCH); o que se
	// prova é que TODOS os módulos lógicos entram no espelho.
	full, err := a.QueryScope(nil)
	if err != nil {
		t.Fatalf("queryscope(nil): %v", err)
	}
	if len(full.Modules) != 4 || len(full.Vectors) != 2 {
		t.Errorf("nil scope should read all modules: modules=%v vec=%d",
			full.Modules, len(full.Vectors))
	}

	// (b) scope roteado {vector, graph} → NÃO lê fts (espaço confinado).
	routed := &modlink.SearchScope{
		Modules:  []string{"vector", "graph"},
		RouteID:  "oracle|planner",
		Priority: 1,
	}
	scoped, err := a.QueryScope(routed)
	if err != nil {
		t.Fatalf("queryscope(routed): %v", err)
	}
	// veio de vector+graph
	if len(scoped.Modules) != 2 || scoped.Modules[0] != "graph" || scoped.Modules[1] != "vector" {
		t.Errorf("expected Modules [graph vector], got %v", scoped.Modules)
	}
	if len(scoped.Vectors) != 2 || len(scoped.Entities) != 3 || len(scoped.Relationships) != 3 {
		t.Errorf("routed projection incomplete: vec=%d ent=%d rel=%d",
			len(scoped.Vectors), len(scoped.Entities), len(scoped.Relationships))
	}
	// NEM UM FTSHIT veio (fts não faz parte do espaço roteado).
	if len(scoped.FTSHits) != 0 {
		t.Errorf("fts module was not confined: got %d hits", len(scoped.FTSHits))
	}

	// (b2) scope roteado que INCLUI fts → o probe BM25 é disparado sobre o
	// módulo FTS usando o RouteID do escopo (determinístico).
	withFTS := &modlink.SearchScope{
		Modules: []string{"fts"},
		RouteID: "oracle",
	}
	ftsd, err := a.QueryScope(withFTS)
	if err != nil {
		t.Fatalf("queryscope(withfts): %v", err)
	}
	if len(ftsd.Modules) != 1 || ftsd.Modules[0] != "fts" {
		t.Errorf("expected Modules [fts], got %v", ftsd.Modules)
	}
	if len(ftsd.FTSHits) == 0 {
		t.Errorf("expected fts hits for a routed fts scope, got 0")
	}

	// (c) NoRoute=true → espaço vazio, NUNCA "pesquisar tudo".
	noRoute := &modlink.SearchScope{NoRoute: true}
	empty, err := a.QueryScope(noRoute)
	if err != nil {
		t.Fatalf("queryscope(noroute): %v", err)
	}
	if len(empty.Modules) != 0 || len(empty.Vectors) != 0 {
		t.Errorf("NoRoute should yield an empty projection, got modules=%v vec=%d",
			empty.Modules, len(empty.Vectors))
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func dbTables(t *testing.T, path string) []string {
	t.Helper()
	db, err := sql.Open("sqlite", roDSN(path))
	if err != nil {
		t.Fatalf("open ro db: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type IN ('table','view') ORDER BY name`)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table: %v", err)
		}
		out = append(out, name)
	}
	return out
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat db: %v", err)
	}
	return info.Size()
}

// probeReadonlyWriteRejected abre uma conexão mode=ro separada sobre o mesmo
// arquivo e tenta um INSERT. Devolve um erro SE a escrita for aceita (violação
// do contrato) OU se falhar por um motivo diferente de "readonly". Isso prova
// que o modo somente-leitura é imposto pela engine, não só por intenção.
func probeReadonlyWriteRejected(t *testing.T, path string) error {
	t.Helper()
	db, err := sql.Open("sqlite", roDSN(path))
	if err != nil {
		t.Fatalf("open ro db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping ro db: %v", err)
	}
	_, err = db.Exec(`INSERT INTO entities (id, entity_type, name, path) VALUES ('x','y','z','w')`)
	if err == nil {
		return errors.New("write on a mode=ro connection unexpectedly succeeded — READ-ONLY violated")
	}
	if !strings.Contains(err.Error(), "readonly") {
		return fmt.Errorf("expected a 'readonly' error from a mode=ro connection, got: %v", err)
	}
	return nil // the engine correctly rejected the write
}

func roDSN(path string) string {
	return "file:" + filepath.ToSlash(path) + "?mode=ro"
}
