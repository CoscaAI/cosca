// Package vectoragg implements the READ-ONLY aggregated read-model (the
// "espelho" / split-de-leitura) of ADR-013 §6 (Fatia 1).
//
// # WHAT IT IS
//
// The modular-knowledge architecture (ADR-013) splits a single `knowledge.db`
// into cohesive, per-responsibility modules — `vector.db`, `graph.db`,
// `fts.db`, `projects.db`, `core.db`. Each module keeps only its own table(s)
// and stays < 100 MB (Decision 1). The semantic search needs to UNIFY those
// modules into one comparable space — but it must never write to them.
//
// This package is that read-side: a read-model aggregator (read-model =
// projection/materialized query). It reads from modules SEPARATE by
// responsibility (§2.0) and exposes TYPED projections (structs) ready for
// re-ranking (vector similarity, graph distance, BM25), without ever being a
// database itself.
//
// # THE ESPELHO (mirror) — WHY THIS DOES NOT MIGRATE ANYTHING
//
// The separate module files do NOT exist yet physically: table rows for
// `vectors`, `entities`, `relationships` and the FTS indexes still live inside
// `knowledge.db`. The aggregator therefore treats a LOGICAL module name (e.g.
// "vector") as a route to a PHYSICAL file, and — for the espelho state — the
// caller points every logical module at the same `knowledge.db`. The union is
// done purely by SQLite `ATTACH DATABASE ... AS <alias>` aliases; no row is
// moved, no table is created, no index is rebuilt. It is a read-only
// projection over the current file(s).
//
// # READ-ONLY CONTRACT (inviolável, imposta POR CONSTRUÇÃO)
//
//   - This package NEVER writes. It never emits INSERT/UPDATE/DELETE/DROP/
//     CREATE/VACUUM/REINDEX.
//   - It opens EVERY database through a single internal path — `openReadOnly`
//     — which ALWAYS appends `?mode=ro` to the DSN. There is NO public API
//     that yields a writable `*sql.DB` or a writable handle. The only exports
//     are read-only projections (structs) and read functions.
//   - Because the connection is `mode=ro`, any accidental write fails at the
//     SQLite layer with `attempt to write a readonly database` — the invariant
//     is enforced by the engine, not just by intent (see the no-write test).
//   - It never migrates, never deletes, never moves, never alters
//     `knowledge.db` or any user database. It never touches
//     `internal/embed/cosca/` (that would force re-signing the family chain).
//   - `ATTACH DATABASE` is used READ-ONLY. If a module file does not exist,
//     the call returns a CLEAR error (never a silent panic or a degraded
//     read).
//
// # THE ROUTED SPACE (ADR-013 §3.2)
//
// The router (`internal/modlink`) fixes the search space deterministically;
// semantic search only refines inside it. This aggregator honors that: the
// routed query (`QueryScope`) reads ONLY the modules named in a
// `*modlink.SearchScope` — it never fabricates a wider space and never falls
// back to "search everything" when the scope is empty.
//
// # RESPONSIBILITY (anti-monster rule, §2.0)
//
// The aggregator is NOT a database. It is the READING that unites the modules.
// It materializes projections and consults the routed space; it owns no
// source of truth and is always reconstructible.
//
// # CANDIDATE RETRIEVAL + TOP-K CONTRACT (Caminho A — ADR-013 §10, audit P1-P4)
//
// The pre-existing readers (Vectors/Entities/Relationships/FTSBM25/QueryScope/
// Counts) are the LOW-LEVEL projection readers and are kept for compatibility;
// they are NOT the search path. The search path is the following bounded
// contract, which uses the SAME vocabulary as internal/search.SearchParams
// (Query/Scope/CandidateIDs/TopK) but is intentionally isolated from it:
//
//	// SearchRequest: QUERY (o que) separado de SCOPE (onde). RouteID é metadado.
//	type SearchRequest struct {
//	    Query        string               // conteúdo pesquisado (o que) → chega ao FTS MATCH
//	    Scope        *modlink.SearchScope // onde pode pesquisar (opcional) — confinamento físico (P1)
//	    CandidateIDs []string             // ids permitidos (fonte de candidatos) — NUNCA "ler tudo"
//	    TopK         int                  // top-K explícito; 0 = SEM resultados, nunca "sem limite"
//	}
//
// // CandidateVectorResult: métricas de confinamento (prova do P4).
// type CandidateVectorResult struct {
//	    Request      SearchRequest
//	    Vectors      []VectorRecord // decodificados (no máximo TopK)
//	    ScopeModules []string
//	    CandidateSet int            // ids de candidatos aceitos (fonte)
//	    Decoded      int64          // BLOBs decodificados (<= TopK)
//	    BytesDecoded int64          // bytes de vetor decodificados
//	    Truncated    bool           // existiam mais candidatos que TopK
//	}
//
// Contract invariants (enforced by implementation, proven by tests):
//   - P4: Vectors are decoded ONLY for the permitted CandidateIDs, at most TopK
//     of them. CandidateIDs empty = zero candidates (never materialize all).
//   - P1: Scope → allowed modules → allowed candidate IDs → retrieval → decode.
//     A scope that does not name the vector module yields NO candidates; it is
//     physical confinement, NOT "module + LIMIT 10" truncation.
//   - P3: Search/FTSMatch send the ORIGINAL Query to the FTS5 `MATCH ?`; the
//     RouteID is metadata and is never used as search text (its '.'/'|'
//     separators would raise an FTS5 syntax error).
//   - P2: CatalogCounts() is the TOTAL catalog; ScopeCounts(scope) counts only
//     the modules in scope. Distinct semantics, both explicit.
//   - TOPK<=0 means "no results" in every search API with a scope — never the
//     legacy "LIMIT 0 == no limit" behaviour (which remains only on the
//     low-level readers, not the search path).
package vectoragg

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CoscaAI/cosca/internal/modlink"

	// SQLite driver (pure Go, no CGO). Imported for its side effect.
	_ "modernc.org/sqlite"
)

// Logical module names per responsibility (ADR-013 §2.1 / §3.2). Every method
// that reads a feature looks up the module by this logical name; the catalog
// maps each logical name to a physical file.
const (
	ModuleVector   = "vector"   // `vectors` (embeddings) — read-model derivado
	ModuleGraph    = "graph"    // `entities` + `relationships` — relacional
	ModuleFTS      = "fts"      // `chunks_fts` + `entities_fts` — BM25 textual
	ModuleProjects = "projects" // `chunks`, `headings`, ... — projeção por projeto
)

// Sentinel errors for the read-only contract: a missing module/table never
// panics and never degrades into a silent empty read — it reports clearly.
var (
	// ErrModuleMissing is returned when the catalog does not carry the module
	// that a feature (vectors/graph/fts) requires.
	ErrModuleMissing = errors.New("vectoragg: required module not present in catalog")
	// ErrTableMissing is returned when a module is present but does not contain
	// the table a feature needs.
	ErrTableMissing = errors.New("vectoragg: required table not present in module")
)

// ModuleCatalog maps a logical module name (e.g. "vector") to the path of the
// SQLite file that holds its tables. It mirrors the catalog of datasources in
// `internal/modlink` (module → path) but is scoped to the READ model.
//
// For the ESPELHO state (the modules still live in `knowledge.db`), every
// logical name points at the SAME physical file — see MirrorCatalog.
type ModuleCatalog map[string]string

// MirrorCatalog builds the read-only mirror catalog over a single physical
// file — the current `knowledge.db`. All logical modules route to the same
// file; each is `ATTACH`ed read-only under its own alias. This is the honest
// representation of the espelho: logical separation, physical unity.
//
//	{"vector": p, "graph": p, "fts": p, "projects": p}
func MirrorCatalog(knowledgeDBPath string) ModuleCatalog {
	return ModuleCatalog{
		ModuleVector:   knowledgeDBPath,
		ModuleGraph:    knowledgeDBPath,
		ModuleFTS:      knowledgeDBPath,
		ModuleProjects: knowledgeDBPath,
	}
}

// Counts is the read-only cardinality report of the aggregated modules.
type Counts struct {
	Vectors       int64 `json:"vectors"`
	Entities      int64 `json:"entities"`
	Relationships int64 `json:"relationships"`
	ChunksFTS     int64 `json:"chunks_fts"`
	EntitiesFTS   int64 `json:"entities_fts"`
}

// VectorRecord is the typed projection of one row of the vector module
// (`vectors`). It carries the embedding (float32, little-endian BLOB) already
// decoded so a caller can re-rank by cosine similarity without knowing the
// storage details.
type VectorRecord struct {
	ID         string    `json:"id"`
	Module     string    `json:"module"` // the logical module it was read from
	RefID      string    `json:"ref_id"` // chosen ref (chunk/entity/document)
	RefHash    string    `json:"ref_hash"`
	Dim        int       `json:"dim"`
	Embedding  []float32 `json:"-"` // decoded; empty when the vector BLOB was short/misaligned
	Content    string    `json:"content"`
	DocumentID string    `json:"document_id,omitempty"`
	ChunkID    string    `json:"chunk_id,omitempty"`
	EntityID   string    `json:"entity_id,omitempty"`
}

// EntityRecord is the typed projection of one row of the graph entity module.
type EntityRecord struct {
	ID       string            `json:"id"`
	Module   string            `json:"module"`
	Type     string            `json:"type"`
	Name     string            `json:"name"`
	Path     string            `json:"path"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// RelationshipRecord is the typed projection of one edge of the graph module.
type RelationshipRecord struct {
	ID         string  `json:"id"`
	Module     string  `json:"module"`
	SourceID   string  `json:"source_id"`
	SourceType string  `json:"source_type"`
	TargetID   string  `json:"target_id"`
	TargetType string  `json:"target_type"`
	RelType    string  `json:"rel_type"`
	Weight     float64 `json:"weight"`
}

// FTSHit is a single BM25 hit from the FTS module. Rank is `-bm25(...)` so that
// a HIGHER rank means a better match (the raw bm25 from SQLite is negative;
// negating it is the conventional "higher is better" presentation used when
// merging into a re-ranking score).
type FTSHit struct {
	Table   string  `json:"table"` // "chunks_fts" | "entities_fts"
	RowID   int64   `json:"rowid"`
	Rank    float64 `json:"rank"`
	Snippet string  `json:"snippet"`
}

// ScopeProjection is the typed result of a ROUTED query: the projection of the
// modules selected by a *modlink.SearchScope, plus the scope it was confined
// to. It is the input for re-ranking (vector + graph + BM25 fused).
type ScopeProjection struct {
	Scope         *modlink.SearchScope `json:"scope"`
	Modules       []string             `json:"modules"` // logical modules actually read
	Vectors       []VectorRecord       `json:"vectors"`
	Entities      []EntityRecord       `json:"entities"`
	Relationships []RelationshipRecord `json:"relationships"`
	FTSHits       []FTSHit             `json:"fts_hits"`
	Counts        Counts               `json:"counts"`
}

// SearchRequest is the bounded search contract of the read-model (Caminho A,
// ADR-013 §10). It carries the QUERY (what to search) SEPARATED from the SCOPE
// (where to search), matching the vocabulary of internal/search.SearchParams
// (Query / Scope / CandidateIDs / TopK) without importing that package.
//
//   - Query is the original user text. It is the ONLY thing that reaches the
//     FTS5 `MATCH ?` operator. The scope's RouteID is metadata and is never
//     used as search text (audit P3).
//   - CandidateIDs is the SOURCE of the permitted vector IDs: the decode step
//     runs ONLY on these candidates, never on the whole index (audit P4).
//   - TopK is explicit. TopK == 0 means "no results" (never "no limit").
//
//nolint:revive // Stutter name aligned with search.SearchRequest vocabulary.
type SearchRequest struct {
	Query        string               // conteúdo pesquisado (o que) → chega ao FTS MATCH
	Scope        *modlink.SearchScope // onde pode pesquisar (opcional) — confinamento físico (P1)
	CandidateIDs []string             // ids permitidos (fonte de candidatos) — NUNCA "ler tudo"
	TopK         int                  // top-K explícito; 0 = SEM resultados, nunca "sem limite"
}

// CandidateVectorResult is the outcome of candidate retrieval + top-K. It
// reports the CONFINEMENT metrics (candidate set, decoded count, decoded bytes,
// truncation) so a caller can PROVE the physical confinement, not just assume
// it. `Vectors` holds the at-most-TopK decoded projections; `Decoded` equals
// `len(Vectors)`.
type CandidateVectorResult struct {
	Request      SearchRequest
	Vectors      []VectorRecord `json:"vectors"` // decodificados (no máximo TopK)
	ScopeModules []string       `json:"scope_modules,omitempty"`
	CandidateSet int            `json:"candidate_set"` // ids de candidatos aceitos (fonte)
	Decoded      int64          `json:"decoded"`       // BLOBs decodificados (<= TopK)
	BytesDecoded int64          `json:"bytes_decoded"` // bytes de vetor decodificados
	Truncated    bool           `json:"truncated"`     // existiam mais candidatos que TopK
}

// Aggregator is the read-only aggregated read-model. It holds one read-only
// connection on which every module is `ATTACH`ed under its own alias. There is
// NO public field or method that exposes a writable handle; reads are the only
// operations. It is safe for concurrent read-only use (each read is a separate
// query on the connection).
type Aggregator struct {
	conn *sql.DB
	// modules is the sorted list of logical module names in the catalog.
	modules []string
	// tables maps a module name to the set of tables present in that module's
	// schema (discovered at Open via sqlite_master on the attached alias).
	tables map[string]map[string]bool
}

// Open opens the read-only aggregator over the given module catalog.
//
// It validates every entry (module name safe, path exists, not a directory),
// opens the first (sorted) module as the anchor connection in `mode=ro`, then
// `ATTACH DATABASE 'file:<path>?mode=ro' AS "<alias>"` for EVERY module — so
// every module is addressed uniformly by its alias. If any module file does
// not exist or is not a valid SQLite database, Open returns a CLEAR error
// naming the module and path (it never panics and never silently skips it).
func Open(catalog ModuleCatalog) (*Aggregator, error) {
	if len(catalog) == 0 {
		return nil, fmt.Errorf("vectoragg: empty module catalog")
	}

	names := make([]string, 0, len(catalog))
	for name := range catalog {
		names = append(names, name)
	}
	sort.Strings(names)

	abs := make(map[string]string, len(names))
	for _, name := range names {
		if !validModuleName(name) {
			return nil, fmt.Errorf("vectoragg: invalid module name %q (allow [A-Za-z0-9_])", name)
		}
		p := strings.TrimSpace(catalog[name])
		if p == "" {
			return nil, fmt.Errorf("vectoragg: module %q has an empty path", name)
		}
		full, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("vectoragg: module %q path: %w", name, err)
		}
		info, err := os.Stat(full)
		if err != nil {
			return nil, fmt.Errorf("vectoragg: module %q database not found or unreadable: %s: %w", name, full, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("vectoragg: module %q path is a directory, not a database: %s", name, full)
		}
		abs[name] = full
	}

	// Anchor connection opens the first (sorted) module read-only; every module
	// is then ATTACHed under its own alias (including the anchor file) so all
	// reads go through the aliases uniformly.
	anchor := abs[names[0]]
	conn, err := openReadOnly(anchor)
	if err != nil {
		return nil, fmt.Errorf("vectoragg: open anchor module %q: %w", names[0], err)
	}
	// A single connection: reads are serialized and no write path can sneak in.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	a := &Aggregator{
		conn:   conn,
		tables: make(map[string]map[string]bool, len(names)),
	}

	for _, name := range names {
		stmt := "ATTACH DATABASE '" + readOnlyDSN(abs[name]) + "' AS " + quoteIdent(name)
		if _, err := conn.Exec(stmt); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("vectoragg: attach module %q (%s): %w", name, abs[name], err)
		}
		a.modules = append(a.modules, name)
	}
	sort.Strings(a.modules)

	// Discover, per module, which tables it actually carries. This makes the
	// aggregator honest about what a module provides and lets it skip a missing
	// table (e.g. a module without an FTS index) without guessing.
	for _, name := range a.modules {
		tb, err := a.tablesOf(name)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("vectoragg: inspect module %q: %w", name, err)
		}
		a.tables[name] = tb
	}
	return a, nil
}

// Close closes the underlying read-only connection. It never writes; it only
// closes. Safe to call more than once.
func (a *Aggregator) Close() error {
	if a == nil || a.conn == nil {
		return nil
	}
	return a.conn.Close()
}

// Modules returns the sorted logical module names present in the catalog.
func (a *Aggregator) Modules() []string {
	out := make([]string, len(a.modules))
	copy(out, a.modules)
	return out
}

// Tables returns the tables present in a module's schema, sorted. It reports
// an empty slice (not an error) for an unknown module.
func (a *Aggregator) Tables(module string) []string {
	tb := a.tables[module]
	out := make([]string, 0, len(tb))
	for t := range tb {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// HasModule reports whether the catalog carries the given logical module.
func (a *Aggregator) HasModule(module string) bool { return a.tables[module] != nil }

// HasTable reports whether a module carries a given table (FTS virtual tables
// included, since they appear in sqlite_master as tables).
func (a *Aggregator) HasTable(module, table string) bool {
	return a.tables[module] != nil && a.tables[module][table]
}

// Counts reports the read-only cardinality of the aggregated modules. Modules
// (or tables) that are not present contribute zero; a present table that cannot
// be counted returns an error.
//
// It is equivalent to CatalogCounts (the TOTAL catalog). Prefer the explicit
// CatalogCounts/ScopeCounts pair (audit P2) when the caller needs to prove
// whether a count is global or scoped.
func (a *Aggregator) Counts() (Counts, error) { return a.CatalogCounts() }

// CatalogCounts reports the TOTAL cardinality across every logical module
// present in the catalog, independent of any scope (audit P2). It is the
// explicit global count.
func (a *Aggregator) CatalogCounts() (Counts, error) {
	return a.countsForModules(append([]string(nil), a.modules...))
}

// ScopeCounts reports the cardinality ONLY within the logical modules selected
// by `scope` (audit P2), using the same confinement rules as the rest of the
// package: nil scope → all catalog modules; NoRoute or empty Modules → zeros
// (never a silent global count); otherwise the intersection with the catalog.
func (a *Aggregator) ScopeCounts(scope *modlink.SearchScope) (Counts, error) {
	return a.countsForModules(a.selectModules(scope))
}

// countsForModules counts each table ONLY when its logical module is in
// `selected`. This is what makes CatalogCounts and ScopeCounts distinct.
func (a *Aggregator) countsForModules(selected []string) (Counts, error) {
	var c Counts
	var err error
	if contains(selected, ModuleVector) && a.HasTable(ModuleVector, "vectors") {
		if c.Vectors, err = a.countTable(ModuleVector, "vectors"); err != nil {
			return c, fmt.Errorf("vectoragg: count vectors: %w", err)
		}
	}
	if contains(selected, ModuleGraph) && a.HasTable(ModuleGraph, "entities") {
		if c.Entities, err = a.countTable(ModuleGraph, "entities"); err != nil {
			return c, fmt.Errorf("vectoragg: count entities: %w", err)
		}
	}
	if contains(selected, ModuleGraph) && a.HasTable(ModuleGraph, "relationships") {
		if c.Relationships, err = a.countTable(ModuleGraph, "relationships"); err != nil {
			return c, fmt.Errorf("vectoragg: count relationships: %w", err)
		}
	}
	if contains(selected, ModuleFTS) && a.HasTable(ModuleFTS, "chunks_fts") {
		if c.ChunksFTS, err = a.countTable(ModuleFTS, "chunks_fts"); err != nil {
			return c, fmt.Errorf("vectoragg: count chunks_fts: %w", err)
		}
	}
	if contains(selected, ModuleFTS) && a.HasTable(ModuleFTS, "entities_fts") {
		if c.EntitiesFTS, err = a.countTable(ModuleFTS, "entities_fts"); err != nil {
			return c, fmt.Errorf("vectoragg: count entities_fts: %w", err)
		}
	}
	return c, nil
}

// Vectors reads the `vectors` table of the vector module and returns the typed
// projection with decoded float32 embeddings. It is the read-model that feeds
// vector-similarity re-ranking. An empty/zero limit means "read all".
func (a *Aggregator) Vectors(limit int) ([]VectorRecord, error) {
	if !a.HasModule(ModuleVector) {
		return nil, fmt.Errorf("%w: %q", ErrModuleMissing, ModuleVector)
	}
	if !a.HasTable(ModuleVector, "vectors") {
		return nil, fmt.Errorf("%w: %q.%s", ErrTableMissing, ModuleVector, "vectors")
	}

	q := "SELECT id, vector, content, document_id, chunk_id, entity_id FROM " + ident(ModuleVector) + ".vectors"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := a.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("vectoragg: query vectors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []VectorRecord
	for rows.Next() {
		var id, content, docID, chunkID, entityID string
		var blob []byte
		if err := rows.Scan(&id, &blob, &content, &docID, &chunkID, &entityID); err != nil {
			return nil, fmt.Errorf("vectoragg: scan vector: %w", err)
		}
		rec := VectorRecord{
			ID:         id,
			Module:     ModuleVector,
			Content:    content,
			DocumentID: docID,
			ChunkID:    chunkID,
			EntityID:   entityID,
		}
		// Choose the ref: entity vectors (entity_id) and chunk vectors
		// (chunk_id) both point at a graph/derived row; document_id is the
		// parent. Prefer entity_id, then chunk_id, then document_id.
		switch {
		case entityID != "":
			rec.RefID = entityID
		case chunkID != "":
			rec.RefID = chunkID
		default:
			rec.RefID = docID
		}
		rec.Embedding, rec.Dim = decodeVector(blob)
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vectoragg: iterate vectors: %w", err)
	}
	return out, nil
}

// Entities reads the `entities` table of the graph module. It feeds graph
// re-ranking by giving the caller the nodes of the routed space.
func (a *Aggregator) Entities(limit int) ([]EntityRecord, error) {
	if !a.HasModule(ModuleGraph) {
		return nil, fmt.Errorf("%w: %q", ErrModuleMissing, ModuleGraph)
	}
	if !a.HasTable(ModuleGraph, "entities") {
		return nil, fmt.Errorf("%w: %q.%s", ErrTableMissing, ModuleGraph, "entities")
	}

	q := "SELECT id, entity_type, name, path, metadata_json FROM " + ident(ModuleGraph) + ".entities"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := a.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("vectoragg: query entities: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []EntityRecord
	for rows.Next() {
		var id, etype, name, path, meta string
		if err := rows.Scan(&id, &etype, &name, &path, &meta); err != nil {
			return nil, fmt.Errorf("vectoragg: scan entity: %w", err)
		}
		out = append(out, EntityRecord{
			ID:       id,
			Module:   ModuleGraph,
			Type:     etype,
			Name:     name,
			Path:     path,
			Metadata: parseMetadata(meta),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vectoragg: iterate entities: %w", err)
	}
	return out, nil
}

// Relationships reads the `relationships` table of the graph module. It is the
// edge set used for real GraphDistance re-ranking.
func (a *Aggregator) Relationships(limit int) ([]RelationshipRecord, error) {
	if !a.HasModule(ModuleGraph) {
		return nil, fmt.Errorf("%w: %q", ErrModuleMissing, ModuleGraph)
	}
	if !a.HasTable(ModuleGraph, "relationships") {
		return nil, fmt.Errorf("%w: %q.%s", ErrTableMissing, ModuleGraph, "relationships")
	}

	q := "SELECT id, source_id, source_type, target_id, target_type, rel_type, weight FROM " + ident(ModuleGraph) + ".relationships"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := a.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("vectoragg: query relationships: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []RelationshipRecord
	for rows.Next() {
		var id, srcID, srcType, tgtID, tgtType, relType string
		var weight float64
		if err := rows.Scan(&id, &srcID, &srcType, &tgtID, &tgtType, &relType, &weight); err != nil {
			return nil, fmt.Errorf("vectoragg: scan relationship: %w", err)
		}
		out = append(out, RelationshipRecord{
			ID:         id,
			Module:     ModuleGraph,
			SourceID:   srcID,
			SourceType: srcType,
			TargetID:   tgtID,
			TargetType: tgtType,
			RelType:    relType,
			Weight:     weight,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vectoragg: iterate relationships: %w", err)
	}
	return out, nil
}

// FTSBM25 runs a BM25 full-text search over the FTS module's tables
// (`chunks_fts` and `entities_fts`) and returns typed hits with a hard-coded
// highlight snippet. Higher Rank is better (Rank = -bm25). It is read-only.
//
// NOTE on FTS5 syntax: the `MATCH` operator and the `bm25`/`snippet` functions
// must reference the FTS table by its UNQUALIFIED name, even though the table
// is read from a schema-qualified (`"alias".table`) FROM clause.
//
// This is the LOW-LEVEL reader. The SEARCH path uses FTSMatch (bounded top-K,
// TopK == 0 → no results). FTSBM25 keeps the historical semantics where
// limit <= 0 means "no limit" only for backward compatibility.
func (a *Aggregator) FTSBM25(query string, limit int) ([]FTSHit, error) {
	if limit < 0 {
		limit = 0
	}
	return a.ftsBM25SQL(query, limitOrAll(limit))
}

// FTSMatch is the SEARCH-path BM25 reader (audit P3): it sends the ORIGINAL
// query string straight to the FTS5 `MATCH ?` operator — never the scope's
// RouteID (whose '.'/'|' separators would raise an FTS5 syntax error). TopK is
// explicit: TopK == 0 means NO results (never "no limit"); TopK < 0 is an
// error; otherwise at most TopK hits per table are returned.
func (a *Aggregator) FTSMatch(query string, topK int) ([]FTSHit, error) {
	if topK < 0 {
		return nil, fmt.Errorf("vectoragg: topK must be >= 0 (got %d); 0 means 'no results', never 'no limit'", topK)
	}
	if topK == 0 {
		return nil, nil // sem resultados — nunca "sem limite"
	}
	return a.ftsBM25SQL(query, fmt.Sprintf("%d", topK))
}

// ftsBM25SQL is the shared BM25 query over the FTS tables. `limitSQL` is the raw
// SQL LIMIT fragment (e.g. "10", or "-1" for the legacy "no limit" reader).
func (a *Aggregator) ftsBM25SQL(query, limitSQL string) ([]FTSHit, error) {
	if !a.HasModule(ModuleFTS) {
		return nil, fmt.Errorf("%w: %q", ErrModuleMissing, ModuleFTS)
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("vectoragg: empty FTS query")
	}

	var out []FTSHit
	seen := map[string]bool{} // dedupe by table+rowid

	for _, ft := range []struct {
		table string
		cols  int // number of columns in the FTS table (bm25 weight count)
		col   int // column index used for the snippet
	}{
		{table: "chunks_fts", cols: 3, col: 0},
		{table: "entities_fts", cols: 3, col: 0},
	} {
		if !a.HasTable(ModuleFTS, ft.table) {
			continue
		}
		weights := strings.TrimSuffix(strings.Repeat("1.0,", ft.cols), ",")
		q := fmt.Sprintf(
			"SELECT rowid, bm25(%s, %s) AS rank, snippet(%s, %d, '<b>', '</b>', '...', 12) "+
				"FROM %s.%s WHERE %s MATCH ? ORDER BY rank LIMIT %s",
			ft.table, weights, ft.table, ft.col, ident(ModuleFTS), ft.table, ft.table, limitSQL,
		)
		rows, err := a.conn.Query(q, query)
		if err != nil {
			return nil, fmt.Errorf("vectoragg: bm25 %s: %w", ft.table, err)
		}
		for rows.Next() {
			var rowID int64
			var rank float64
			var snip string
			if err := rows.Scan(&rowID, &rank, &snip); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("vectoragg: scan %s: %w", ft.table, err)
			}
			key := ft.table + ":" + fmt.Sprint(rowID)
			if seen[key] {
				continue
			}
			seen[key] = true
			// Higher rank = better (negate the raw negative bm25).
			out = append(out, FTSHit{Table: ft.table, RowID: rowID, Rank: -rank, Snippet: snip})
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("vectoragg: iterate %s: %w", ft.table, err)
		}
		_ = rows.Close()
	}
	return out, nil
}

// QueryScope is the ROUTED query: it materializes the projection of ONLY the
// logical modules named in `scope`, never a wider space (ADR-013 §3.2 — the
// search refines the routed space, it never chooses it).
//
// Confinement rules (deterministic):
//   - scope == nil → read ALL catalog modules (retro-compatible full mirror).
//   - scope.NoRoute == true → read NOTHING (empty projection; never a silent
//     fallback to "search everything").
//   - otherwise → read only the catalog modules whose name is listed in
//     scope.Modules (the intersection). An empty intersection ⇒ empty
//     projection.
func (a *Aggregator) QueryScope(scope *modlink.SearchScope) (*ScopeProjection, error) {
	selected := a.selectModules(scope)
	proj := &ScopeProjection{
		Scope:   scope,
		Modules: append([]string(nil), selected...),
	}

	var err error
	if contains(selected, ModuleVector) {
		proj.Vectors, err = a.Vectors(0)
		if err != nil && !errors.Is(err, ErrModuleMissing) && !errors.Is(err, ErrTableMissing) {
			return nil, err
		}
	}
	if contains(selected, ModuleGraph) {
		proj.Entities, err = a.Entities(0)
		if err != nil && !errors.Is(err, ErrModuleMissing) && !errors.Is(err, ErrTableMissing) {
			return nil, err
		}
		proj.Relationships, err = a.Relationships(0)
		if err != nil && !errors.Is(err, ErrModuleMissing) && !errors.Is(err, ErrTableMissing) {
			return nil, err
		}
	}
	if contains(selected, ModuleFTS) {
		if q := scopeQuery(scope); q != "" {
			proj.FTSHits, err = a.FTSBM25(q, 0)
			if err != nil && !errors.Is(err, ErrModuleMissing) && !errors.Is(err, ErrTableMissing) {
				return nil, err
			}
		}
	}
	proj.Counts, err = a.Counts()
	if err != nil {
		return nil, err
	}
	return proj, nil
}

// RetrieveCandidates is the CANDIDATE retrieval + top-K search path (audit
// P1/P4). It decodes at MOST TopK vectors and uses CandidateIDs as the ONLY
// source of permitted IDs — never the whole index. The decode step runs
// exclusively on the candidates, in the order the caller provided them.
//
// Physical confinement (P1): a non-nil Scope must name the vector module as an
// allowed module; otherwise there are NO permitted candidates (never a
// "module + LIMIT" truncation or a read-everything fallback). Empty
// CandidateIDs means zero candidates. TopK == 0 means no results (never "no
// limit"); TopK < 0 is an error.
func (a *Aggregator) RetrieveCandidates(req SearchRequest) (*CandidateVectorResult, error) {
	res := &CandidateVectorResult{Request: req}
	if !a.HasModule(ModuleVector) {
		return res, fmt.Errorf("%w: %q", ErrModuleMissing, ModuleVector)
	}
	if !a.HasTable(ModuleVector, "vectors") {
		return res, fmt.Errorf("%w: %q.%s", ErrTableMissing, ModuleVector, "vectors")
	}
	if req.TopK < 0 {
		return res, fmt.Errorf("vectoragg: TopK must be >= 0 (got %d); 0 means 'no results', never 'no limit'", req.TopK)
	}

	// Confinamento físico (P1): quem nomeia os módulos permitidos é o scope. Se
	// "vector" não está no conjunto permitido → nenhum candidato permitido.
	allowed := a.selectModules(req.Scope)
	res.ScopeModules = append([]string(nil), allowed...)
	if !contains(allowed, ModuleVector) {
		return res, nil
	}

	ids := dedupePreserve(req.CandidateIDs)
	res.CandidateSet = len(ids)
	if len(ids) == 0 || req.TopK == 0 {
		return res, nil // zero candidatos (nunca materializar tudo) / TopK 0 = sem resultados
	}

	dec := make([]VectorRecord, 0, req.TopK)
	var decoded, byteCount int64
	const cols = "id, vector, content, document_id, chunk_id, entity_id"
	query := "SELECT " + cols + " FROM " + ident(ModuleVector) + ".vectors WHERE id = ?"
	for _, cid := range ids {
		if int64(len(dec)) >= int64(req.TopK) {
			res.Truncated = true
			break
		}
		var id, content, docID, chunkID, entityID string
		var blob []byte
		err := a.conn.QueryRow(query, cid).Scan(&id, &blob, &content, &docID, &chunkID, &entityID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue // candidato permitido, mas ausente no módulo → skip
			}
			return res, fmt.Errorf("vectoragg: candidate %q: %w", cid, err)
		}
		rec := VectorRecord{
			ID: id, Module: ModuleVector, Content: content,
			DocumentID: docID, ChunkID: chunkID, EntityID: entityID,
		}
		switch {
		case entityID != "":
			rec.RefID = entityID
		case chunkID != "":
			rec.RefID = chunkID
		default:
			rec.RefID = docID
		}
		rec.Embedding, rec.Dim = decodeVector(blob)
		dec = append(dec, rec)
		decoded++
		byteCount += int64(len(blob))
	}
	res.Vectors = dec
	res.Decoded = decoded
	res.BytesDecoded = byteCount
	return res, nil
}

// Search is the end-to-end SEARCH path of the new contract (audit P1/P2/P3/P4).
// It confines the query to `req.Scope`, sends `req.Query` (not the RouteID) to
// the FTS5 MATCH, retrieves candidates bounded by CandidateIDs/TopK, and
// reports scoped counts. It never falls back to "read everything".
func (a *Aggregator) Search(req SearchRequest) (*ScopeProjection, error) {
	allowed := a.selectModules(req.Scope)
	proj := &ScopeProjection{
		Scope:   req.Scope,
		Modules: append([]string(nil), allowed...),
	}
	var err error
	if contains(allowed, ModuleVector) {
		cres, cerr := a.RetrieveCandidates(req)
		if cerr != nil && !errors.Is(cerr, ErrModuleMissing) && !errors.Is(cerr, ErrTableMissing) {
			return nil, cerr
		}
		if cres != nil {
			proj.Vectors = cres.Vectors
		}
	}
	if contains(allowed, ModuleFTS) && strings.TrimSpace(req.Query) != "" {
		hits, ferr := a.FTSMatch(req.Query, req.TopK)
		if ferr != nil && !errors.Is(ferr, ErrModuleMissing) && !errors.Is(ferr, ErrTableMissing) {
			return nil, ferr
		}
		proj.FTSHits = hits
	}
	proj.Counts, err = a.ScopeCounts(req.Scope)
	if err != nil {
		return nil, err
	}
	return proj, nil
}

// GraphDistance returns the shortest-path length between two nodes in the
// graph module (edges from `relationships`, traversed undirected), used for
// real re-ranking. It is a pure read-only BFS. Returns (−1, false) when the
// target is unreachable; returns (0, true) when src == tgt.
func (a *Aggregator) GraphDistance(src, tgt string) (int, bool) {
	if src == "" || tgt == "" {
		return -1, false
	}
	if src == tgt {
		return 0, true
	}
	if !a.HasTable(ModuleGraph, "relationships") {
		return -1, false
	}

	rows, err := a.conn.Query("SELECT source_id, target_id FROM " + ident(ModuleGraph) + ".relationships")
	if err != nil {
		return -1, false
	}
	defer func() { _ = rows.Close() }()

	adj := map[string][]string{}
	for rows.Next() {
		var s, t string
		if err := rows.Scan(&s, &t); err != nil {
			return -1, false
		}
		adj[s] = append(adj[s], t)
		adj[t] = append(adj[t], s)
	}
	if err := rows.Err(); err != nil {
		return -1, false
	}

	type node struct {
		id   string
		dist int
	}
	visited := map[string]bool{src: true}
	queue := []node{{id: src}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, n := range adj[cur.id] {
			if n == tgt {
				return cur.dist + 1, true
			}
			if visited[n] {
				continue
			}
			visited[n] = true
			queue = append(queue, node{id: n, dist: cur.dist + 1})
		}
	}
	return -1, false
}

// ── internal helpers ─────────────────────────────────────────────────────────

// selectModules decides which catalog modules a scope confines the query to.
func (a *Aggregator) selectModules(scope *modlink.SearchScope) []string {
	if scope == nil {
		return append([]string(nil), a.modules...)
	}
	if scope.NoRoute || len(scope.Modules) == 0 {
		return nil
	}
	avail := map[string]bool{}
	for _, m := range a.modules {
		avail[m] = true
	}
	var out []string
	for _, m := range scope.Modules {
		if avail[m] {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

// dedupePreserve returns a copy of `in` with duplicate IDs removed while
// preserving the first-occurrence order, so each permitted candidate is decoded
// at most once.
func dedupePreserve(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func (a *Aggregator) countTable(module, table string) (int64, error) {
	if !a.HasTable(module, table) {
		return 0, nil
	}
	var n int64
	q := "SELECT COUNT(*) FROM " + ident(module) + "." + table
	if err := a.conn.QueryRow(q).Scan(&n); err != nil {
		return 0, fmt.Errorf("count %s.%s: %w", module, table, err)
	}
	return n, nil
}

// tablesOf lists the tables/views present in a module's attached schema by
// reading its sqlite_master (read-only). FTS5 virtual tables are reported.
func (a *Aggregator) tablesOf(module string) (map[string]bool, error) {
	q := "SELECT name FROM " + ident(module) + ".sqlite_master WHERE type IN ('table','view')"
	rows, err := a.conn.Query(q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// scopeQuery derives an FTS MATCH query from a SearchScope. The scope carries
// no free-text signal in this read-model; the natural deterministic probe is
// the RouteID (the sorted triggers of the matched routes), which can be used as
// a BM25 probe over the routed module. Empty when there is no route.
func scopeQuery(scope *modlink.SearchScope) string {
	if scope == nil || scope.NoRoute {
		return ""
	}
	if strings.TrimSpace(scope.RouteID) != "" {
		return scope.RouteID
	}
	return ""
}

// decodeVector decodes a float32 little-endian BLOB into a []float32. It
// returns nil when the blob is empty, misaligned, or not a multiple of 4 bytes
// (a malformed vector is surfaced as an empty Embedding/Dim=0, never a panic).
func decodeVector(blob []byte) ([]float32, int) {
	if len(blob) == 0 || len(blob)%4 != 0 {
		return nil, 0
	}
	dim := len(blob) / 4
	out := make([]float32, dim)
	for i := 0; i < dim; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
	}
	return out, dim
}

// parseMetadata decodes a JSON object into a map. A malformed/empty value
// yields nil (never an error) so a single bad row cannot fail the read.
func parseMetadata(raw string) map[string]string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(trimmed), &m); err != nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = fmt.Sprint(v)
	}
	return out
}

// validModuleName reports whether a module name is a safe SQLite identifier
// (the alias is quoted, but we still reject anything that could obscure intent).
func validModuleName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// readOnlyDSN builds a read-only SQLite URI DSN. Windows backslashes are
// normalized to forward slashes so the `file:` URI is valid on every platform.
func readOnlyDSN(absPath string) string {
	return "file:" + filepath.ToSlash(absPath) + "?mode=ro"
}

// openReadOnly opens a database in mode=ro and validates it with Ping. The
// returned *sql.DB is read-only by construction (mode=ro); any write attempt
// fails at the SQLite layer.
func openReadOnly(absPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", readOnlyDSN(absPath))
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ident quotes a module name as a SQLite schema identifier for use in FROM
// clauses: `"vector".vectors`.
func ident(module string) string { return quoteIdent(module) }

// quoteIdent wraps a module name in double quotes so it is a valid, safe
// schema identifier. Module names are pre-validated to [A-Za-z0-9_].
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func limitOrAll(limit int) string {
	if limit <= 0 {
		return "-1" // SQLite LIMIT -1 = no limit
	}
	return fmt.Sprintf("%d", limit)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
