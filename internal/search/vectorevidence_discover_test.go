package search

// TestVectorEvidenceDiscover — DIAGNÓSTICO (read-only). NÃO é o benchmark.
// Inspeciona o knowledge.db real: schema, contagens por módulo (pastas de
// conteúdo), dimensão dos vetores, e valida o JOIN documents→vectors que o
// roteado usa para derivar CandidateIDs. Imprime tudo para stdout para eu
// escolher o ground-truth ANTES de rodar o benchmark.

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

const evidenceDBPath = `C:\Users\Henrique\Documents\cosca\.cosca\knowledge.db`

func evidenceOpenRO(tb testing.TB) *sql.DB {
	tb.Helper()
	// mode=ro: qualquer escrita falha no SQLite. Nunca escreve.
	dsn := "file:" + filepath.ToSlash(evidenceDBPath) + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		tb.Fatalf("open ro: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		tb.Fatalf("ping ro: %v", err)
	}
	return db
}

func evidenceCount(tb testing.TB, db *sql.DB, q string, args ...interface{}) int64 {
	tb.Helper()
	var n int64
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		tb.Fatalf("count query %q: %v", q, err)
	}
	return n
}

func TestVectorEvidenceDiscover(t *testing.T) {
	db := evidenceOpenRO(t)
	defer db.Close()

	// 1. Tabelas presentes.
	tables := []string{"documents", "chunks", "vectors", "entities", "relationships"}
	for _, tb := range tables {
		var cnt int64
		row := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table') AND name = ?", tb)
		_ = row.Scan(&cnt)
		fmt.Printf("[table] %-14s present=%d\n", tb, cnt)
	}

	// 2. Contagens globais.
	fmt.Printf("\n[counts]\n")
	if hasTable(db, "documents") {
		fmt.Printf("  documents          = %d\n", evidenceCount(t, db, "SELECT COUNT(*) FROM documents"))
	}
	if hasTable(db, "vectors") {
		fmt.Printf("  vectors (raw)      = %d\n", evidenceCount(t, db, "SELECT COUNT(*) FROM vectors"))
		fmt.Printf("  vectors doc_id=''  = %d\n", evidenceCount(t, db, "SELECT COUNT(*) FROM vectors WHERE document_id=''"))
		fmt.Printf("  vectors chunk_id<>''= %d\n", evidenceCount(t, db, "SELECT COUNT(*) FROM vectors WHERE chunk_id<>''"))
	}

	// 3. Dimensão (comprimento do primeiro BLOB).
	if hasTable(db, "vectors") {
		var blob []byte
		_ = db.QueryRow("SELECT vector FROM vectors WHERE length(vector)>0 LIMIT 1").Scan(&blob)
		if len(blob) > 0 {
			fmt.Printf("\n[dim] blob bytes = %d  => dim = %d (float32)\n", len(blob), len(blob)/4)
		}
	}

	// 4. Contagens por módulo: JOIN documents→vectors via path-segment.
	if hasTable(db, "documents") && hasTable(db, "vectors") {
		fmt.Printf("\n[module vector counts via documents.path JOIN vectors.document_id]\n")
		modules := []string{"memory", "knowledge", "architecture", "runtime", "cli", "security",
			"vector", "unreal", "world", "vegetation", "materials"}
		for _, m := range modules {
			// path-segment: usamos LIKE com separadores para isolar o segmento.
			// (aproximação de pathHasSegment; o benchmark real replica a função exata)
			docs := evidenceCount(t, db,
				"SELECT COUNT(*) FROM documents WHERE '/'||REPLACE(path,'\\','/')||'/' LIKE '%/'||?||'/%'", m)
			vecs := evidenceCount(t, db,
				`SELECT COUNT(DISTINCT v.id) FROM vectors v JOIN documents d ON v.document_id=d.id
				 WHERE '/'||REPLACE(d.path,'\','/')||'/' LIKE '%/'||?||'/%'`, m)
			fmt.Printf("  %-14s docs=%-6d vectors=%d\n", m, docs, vecs)
		}
	}

	// 5. JOIN exaustivo: COUNT(DISTINCT v.id via JOIN) == COUNT(*) de vectors?
	if hasTable(db, "documents") && hasTable(db, "vectors") {
		joined := evidenceCount(t, db, "SELECT COUNT(DISTINCT v.id) FROM vectors v JOIN documents d ON v.document_id=d.id")
		total := evidenceCount(t, db, "SELECT COUNT(*) FROM vectors")
		fmt.Printf("\n[JOIN exhaustiveness] COUNT(DISTINCT v.id via JOIN)=%d  total vectors=%d  => %v\n",
			joined, total, joined == total)
	}
	if hasTable(db, "documents") {
		fmt.Printf("\n[sample documents matching path segment 'knowledge' (limit 25)]\n")
		rows, err := db.Query(`SELECT id, path FROM documents WHERE '/'||REPLACE(path,'\','/')||'/' LIKE '%/knowledge/%' LIMIT 25`)
		if err != nil {
			t.Fatalf("query docs knowledge: %v", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id, path string
			_ = rows.Scan(&id, &path)
			fmt.Printf("  %-12s %s\n", id, path)
		}
	}
	if hasTable(db, "documents") {
		fmt.Printf("\n[sample documents 'runtime' (limit 25)]\n")
		rows, err := db.Query(`SELECT id, path FROM documents WHERE '/'||REPLACE(path,'\','/')||'/' LIKE '%/runtime/%' LIMIT 25`)
		if err != nil {
			t.Fatalf("query docs runtime: %v", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id, path string
			_ = rows.Scan(&id, &path)
			fmt.Printf("  %-12s %s\n", id, path)
		}
	}

	// 6. Column list of documents and vectors for reference.
	fmt.Printf("\n[documents columns]\n")
	cols, _ := evidenceColumns(db, "documents")
	fmt.Printf("  %v\n", cols)
	fmt.Printf("[vectors columns]\n")
	cols, _ = evidenceColumns(db, "vectors")
	fmt.Printf("  %v\n", cols)

	// 7. Total de vetores / filesystem fallback (para contexto).
	fmt.Printf("\n[workspace context]\n")
	pwd, _ := os.Getwd()
	fmt.Printf("  cwd=%s\n", pwd)

	// 8. BUSCA POR TÓPICO (ground-truth). Procura chunks.content e reporta o
	// documento (id + path) junto com um trecho. Usado para escolher o GT.
	fmt.Printf("\n[ground-truth content search]\n")
	for _, kw := range []string{"systemd", "serve como serviço", "lei da familia", "lei da família", "velocidade da memoria", "velocidade da memória", "gate de integridade", "cosseno", "similaridade", "entre sessões", "entre sessoes", "persistencia", "persistência"} {
		evidenceSearchDB(t, db, kw)
	}

	// 9. BUSCA POR MÓDULO+TERMO (escolha do GT dentro do módulo esperado).
	fmt.Printf("\n[ground-truth module+term search (constrained to module path)]\n")
	type mk struct {
		module string
		terms  []string
	}
	moduleSearches := []mk{
		{"runtime", []string{"serve", "systemd", "service", "daemon", "deploy", "production"}},
		{"memory", []string{"família", "familia", "velocidade", "memoria", "sessões", "sessoes"}},
		{"knowledge", []string{"gate", "integridade", "chain", "cadeia", "assinatura", "verificação", "bootstrap"}},
		{"knowledge", []string{"cosseno", "cosine", "similaridade", "vetorial"}},
		{"security", []string{"gate", "integridade", "chain", "verificação"}},
		{"runtime", []string{"memoria", "memória", "sessão", "sessao", "persist", "reload"}},
		{"memory", []string{"memoria entre sessoes", "continuidade", "persist"}},
	}
	for _, m := range moduleSearches {
		for _, term := range m.terms {
			evidenceSearchDBModule(t, db, m.module, term)
		}
	}

	// 10. Lookup direto de IDs por path exato (escolha final do GT).
	fmt.Printf("\n[exact path id lookup]\n")
	for _, p := range []string{
		`docs\knowledge\search.md`,
		`internal\embed\cosca\engines\runtime\SKILL.md`,
		`.cosca\fallback\memory\agent\cosca-kernel\learnings.md`,
		`internal\embed\cosca\memory\agent\cosca-runtime\learnings.md`,
		`internal\embed\cosca\memory\agent\cosca-kernel\archive\L001-L214.md`,
	} {
		var id, path string
		if err := db.QueryRow(`SELECT id, path FROM documents WHERE path LIKE ? LIMIT 1`, "%"+p).Scan(&id, &path); err != nil {
			fmt.Printf("  %-70s -> ERR %v\n", p, err)
			continue
		}
		fmt.Printf("  %-70s -> %s\n", path, id)
	}
}

// evidenceSearchDBModule: como evidenceSearchDB mas restringe ao módulo (via
// path-segment). Permite escolher o GT DENTRO do módulo esperado.
func evidenceSearchDBModule(tb testing.TB, db *sql.DB, module, term string) {
	tb.Helper()
	q := `SELECT d.id, d.path, substr(c.content,1,150)
		FROM chunks c JOIN documents d ON c.document_id = d.id
		WHERE ('/'||REPLACE(d.path,'\','/')||'/' LIKE '%/'||?||'/%')
		  AND c.content LIKE ? LIMIT 6`
	rows, err := db.Query(q, module, "%"+term+"%")
	if err != nil {
		fmt.Printf("  [mod=%s term=%q] ERR %v\n", module, term, err)
		return
	}
	defer rows.Close()
	count := 0
	fmt.Printf("\n  --- module=%q term=%q ---\n", module, term)
	for rows.Next() {
		var id, path, snip string
		_ = rows.Scan(&id, &path, &snip)
		snip = strings.ReplaceAll(snip, "\n", " ")
		snip = strings.ReplaceAll(snip, "  ", " ")
		fmt.Printf("  %-12s %s\n      %s\n", id, path, snip)
		count++
	}
	if count == 0 {
		fmt.Printf("  (no matches in module %q)\n", module)
	}
}

// evidenceSearchDB procura a palavra-chave no conteúdo dos chunks e reporta os
// documentos (id, path) que a contêm, com um trecho do chunk. Apenas leitura.
func evidenceSearchDB(tb testing.TB, db *sql.DB, kw string) {
	tb.Helper()
	q := `SELECT d.id, d.path, substr(c.content,1,180)
		FROM chunks c JOIN documents d ON c.document_id = d.id
		WHERE c.content LIKE ? LIMIT 12`
	rows, err := db.Query(q, "%"+kw+"%")
	if err != nil {
		fmt.Printf("  [kw=%q] ERR %v\n", kw, err)
		return
	}
	defer rows.Close()
	count := 0
	fmt.Printf("\n  --- keyword %q ---\n", kw)
	for rows.Next() {
		var id, path, snip string
		_ = rows.Scan(&id, &path, &snip)
		snip = strings.ReplaceAll(snip, "\n", " ")
		snip = strings.ReplaceAll(snip, "  ", " ")
		fmt.Printf("  %-12s %s\n      %s\n", id, path, snip)
		count++
	}
	if count == 0 {
		fmt.Printf("  (no matches)\n")
	}
}

func hasTable(db *sql.DB, name string) bool {
	var n int64
	_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table') AND name=?", name).Scan(&n)
	return n > 0
}

func evidenceColumns(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query("SELECT * FROM " + table + " LIMIT 0")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	sort.Strings(cols)
	return cols, nil
}

var _ = strings.TrimSpace
