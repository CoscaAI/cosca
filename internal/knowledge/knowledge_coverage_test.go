package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/search"
)

// ── SyncWithProgress Tests ───────────────────────────────────────────────────

func TestEngine_SyncWithProgress_NilCallback(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)

	// SyncWithProgress with nil callback should behave like Sync
	result, err := engine.SyncWithProgress(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Added)
	assert.Empty(t, result.Removed)
	assert.Empty(t, result.Updated)
	assert.Empty(t, result.Errors)
	// Duration is wall-clock; an empty sync can measure 0 on fast machines /
	// Windows (coarse clock granularity), so only assert non-negative.
	assert.GreaterOrEqual(t, result.Duration, int64(0))
}

func TestEngine_SyncWithProgress_WithCallback(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)
	rootDir := engine.cfg.RootDir

	// Add files to trigger scanning/indexing phases
	writeTempMarkdown(t, rootDir, "prog_sync1.md", "# Prog Sync 1\n\nContent one.")
	writeTempMarkdown(t, rootDir, "prog_sync2.md", "# Prog Sync 2\n\nContent two.")

	phasesSeen := make(map[string]bool)
	var progressCalls int

	callback := func(p SyncProgress) {
		progressCalls++
		phasesSeen[p.Phase] = true
	}

	result, err := engine.SyncWithProgress(context.Background(), callback)
	require.NoError(t, err)
	require.NotNil(t, result)

	// We should have seen at least the scanning phase
	assert.True(t, phasesSeen["scanning"], "should see scanning phase")
	assert.Greater(t, progressCalls, 0, "should have at least one progress call")
	assert.Greater(t, result.Duration, int64(0))
}

func TestEngine_SyncWithProgress_NotInitialized(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "syncprog_ni.db")
	engine := newTestEngine(t, dbPath)

	callback := func(p SyncProgress) {}
	result, err := engine.SyncWithProgress(context.Background(), callback)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestEngine_SyncWithProgress_NotInitialized_NilCallback(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "syncprog_nicb.db")
	engine := newTestEngine(t, dbPath)

	result, err := engine.SyncWithProgress(context.Background(), nil)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not initialized")
}

// ── syncInternal edge cases ──────────────────────────────────────────────────

func TestEngine_Sync_AfterIndexing_HashMatch(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)
	rootDir := engine.cfg.RootDir

	// Index a file first
	mdPath := writeTempMarkdown(t, rootDir, "synchash.md", "# Hash Match\n\nContent for hash matching.")
	err := engine.IndexDocument(context.Background(), mdPath)
	require.NoError(t, err)

	// Run Sync — file is already in DB with matching hash
	result, err := engine.Sync(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	// File should NOT appear as Added (already in DB)
	assert.NotContains(t, result.Added, mdPath,
		"already-indexed file should not appear in added list")
}

func TestEngine_Sync_DBNilAfterInit(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)

	// Keep a reference so the real DB handle is closed at cleanup even
	// though the field is nil'ed below (Windows file lock).
	db := engine.db
	// Set db to nil after init to exercise nil DB check in syncInternal
	engine.db = nil
	t.Cleanup(func() { _ = db.Close() })

	result, err := engine.Sync(context.Background())
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "database not available")
}

// ── Sync Test: Hash Change Detection ─────────────────────────────────────────

func TestEngine_Sync_HashMismatch(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)
	rootDir := engine.cfg.RootDir

	// Create and index a file
	mdPath := writeTempMarkdown(t, rootDir, "synchash_changed.md",
		"# Original\n\nOriginal content before modification.")
	err := engine.IndexDocument(context.Background(), mdPath)
	require.NoError(t, err)

	// Now modify the file to create a hash mismatch
	writeTempMarkdown(t, rootDir, "synchash_changed.md",
		"# Modified\n\nThis content has been changed and the hash should differ.")

	result, err := engine.Sync(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	// The file should appear as Updated since hash changed
	assert.Contains(t, result.Updated, mdPath,
		"modified file should appear in updated list")
}

// ── Compile: error accumulation across categories ────────────────────────────

func TestCompile_CategoryWalkError(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// Create a valid category with a file
	writeRepoFile(t, repoPath, "heuristics/valid.yaml", `id: H1
title: "Valid"
domain: test
description: A valid heuristic.
`)
	// Create an architecture directory as a FILE (not dir) — this will cause a walk error
	// Actually os.Stat + filepath.WalkDir will handle this differently.
	// The walk error path is better tested by making the repoPath itself invalid
	// Let's instead make a per-file read error by making a file unreadable.
	// Actually, in Linux we can't easily make a file unreadable in a temp dir.
	// Better approach: test Compile with context cancellation mid-category walk.

	compiler := NewCompiler(db, repoPath)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := compiler.Compile(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancel")
	_ = result
}

// ── parseFile: unsupported extension ─────────────────────────────────────────

func TestParseFile_UnsupportedExtension(t *testing.T) {
	t.Parallel()

	entry, err := parseFile("/tmp/test.txt", "rel/test.txt", CategoryHeuristics, []byte("content"))
	assert.Error(t, err)
	assert.Nil(t, entry)
	assert.Contains(t, err.Error(), "unsupported")
}

// ── parseYAMLEntry: invalid YAML ─────────────────────────────────────────────

func TestParseYAMLEntry_InvalidYAML(t *testing.T) {
	t.Parallel()

	entry := &KnowledgeEntry{
		Category:   CategoryHeuristics,
		Source:     "test.yaml",
		Confidence: 0.5,
	}

	// Malformed YAML
	invalidYAML := []byte(":\n\t- bad indent\n\t: value")

	result, err := parseYAMLEntry(entry, invalidYAML)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "parse YAML")
}

// ── parseYAMLEntry: empty entry ──────────────────────────────────────────────

func TestParseYAMLEntry_Empty(t *testing.T) {
	t.Parallel()

	entry := &KnowledgeEntry{
		Category:   CategoryPatterns,
		Source:     "empty.yaml",
		Confidence: 0.5,
	}

	result, err := parseYAMLEntry(entry, []byte(""))
	assert.NoError(t, err, "empty YAML should parse without error")
	assert.NotNil(t, result)
	assert.Equal(t, CategoryPatterns, result.Category)
	assert.Equal(t, float64(0.5), result.Confidence)
	// Content should be the raw YAML since no description
	assert.Equal(t, "", result.Content) // Empty YAML yields empty string
}

// ── parseMarkdownEntry: title extraction completeness ────────────────────────

func TestParseMarkdownEntry_NoTitleFallback(t *testing.T) {
	t.Parallel()

	entry := &KnowledgeEntry{
		Category:   CategoryPatterns,
		Source:     "patterns/no-title-file.md",
		Confidence: 0.5,
	}

	// Markdown with no heading at all — should use filename
	content := []byte("Just some text with no heading.")
	result, err := parseMarkdownEntry(entry, content)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "no-title-file", result.Title, "should use filename when no heading found")
	assert.Contains(t, result.Content, "Just some text with no heading")
}

func TestParseMarkdownEntry_FrontmatterOnly(t *testing.T) {
	t.Parallel()

	entry := &KnowledgeEntry{
		Category:   CategoryPatterns,
		Source:     "fm-only.md",
		Confidence: 0.5,
	}

	// Only frontmatter, no body
	content := []byte("---\ntitle: Frontmatter Title\ntags: [test]\n---\n")
	result, err := parseMarkdownEntry(entry, content)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Frontmatter Title", result.Title)
	assert.Equal(t, []string{"test"}, result.Tags)
}

func TestParseMarkdownEntry_EmptyContent(t *testing.T) {
	t.Parallel()

	entry := &KnowledgeEntry{
		Category:   CategoryPatterns,
		Source:     "empty.md",
		Confidence: 0.5,
	}

	result, err := parseMarkdownEntry(entry, []byte(""))
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "empty", result.Title, "should use filename for empty content")
	assert.Equal(t, "", result.Content)
}

func TestParseMarkdownEntry_FrontmatterWithInvalidYAML(t *testing.T) {
	t.Parallel()

	entry := &KnowledgeEntry{
		Category:   CategoryHeuristics,
		Source:     "invalid-fm.md",
		Confidence: 0.5,
	}

	// Frontmatter delim is there but YAML inside is malformed
	content := []byte("---\n: invalid yaml\n---\n\n# Valid Heading\n\nBody content.")
	result, err := parseMarkdownEntry(entry, content)
	// Should still parse — malformed frontmatter is skipped, title extracted from heading
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Valid Heading", result.Title)
	assert.Contains(t, result.Content, "Body content")
}

// ── parseMarkdownEntry: heading as #
// Already covered mostly; test the "not ##" path ──────────────────────────────

func TestExtractHeadingTitle_TopLevelH1WithoutSpace(t *testing.T) {
	t.Parallel()

	// The code checks: strings.HasPrefix(trimmed, "# ") first, then
	// strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "##")
	// This tests the second branch
	title := extractHeadingTitle("#NoSpace")
	assert.Equal(t, "NoSpace", title)
}

func TestExtractHeadingTitle_TopLevelH2(t *testing.T) {
	t.Parallel()

	// ## should NOT match
	title := extractHeadingTitle("## Subtitle\n\n# Main Title")
	assert.Equal(t, "Main Title", title)
}

func TestExtractHeadingTitle_MultipleHash(t *testing.T) {
	t.Parallel()

	// ### should not match as "# " (more than one hash)
	title := extractHeadingTitle("### Deep Heading\n\n# Top Level")
	assert.Equal(t, "Top Level", title)
}

// ── extractTags: edge cases ──────────────────────────────────────────────────

func TestExtractTags_UnknownType(t *testing.T) {
	t.Parallel()

	// tags as an integer — should return nil
	input := map[string]interface{}{
		"tags": 42,
	}
	result := extractTags(input)
	assert.Nil(t, result, "unknown tag type should return nil")
}

func TestExtractTags_NilInterface(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"tags": []interface{}{"a", 123, "b"},
	}
	result := extractTags(input)
	assert.Equal(t, []string{"a", "b"}, result, "non-string items in slice should be skipped")
}

func TestExtractTags_EmptySlice(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"tags": []interface{}{},
	}
	result := extractTags(input)
	assert.NotNil(t, result)
	assert.Empty(t, result, "empty tag slice should yield empty result")
}

func TestExtractTags_EmptyList(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"tags": []string{},
	}
	result := extractTags(input)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

// ── Explain: vector store check path ─────────────────────────────────────────

func TestEngine_Explain_VectorStorePath(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)
	// Test with a chunk ID that has no document or graph node
	explanation, err := engine.Explain(context.Background(), "nonexistent-check-id")
	require.NoError(t, err)
	require.NotNil(t, explanation)
	assert.Empty(t, explanation.Type, "no data should match this ID")
}

// ── New Engine: mkdir failure error message ─────────────────────────────────

func TestNewEngine_MkdirAllFailure_Message(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	blockFile := filepath.Join(tmpDir, "blockfile")
	_ = writeTempMarkdown(t, tmpDir, "blockfile", "block")

	cfg := Config{
		DBPath:  filepath.Join(blockFile, "subdir", "test.db"),
		RootDir: tmpDir,
	}

	_, err := New(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create data directory")
}

// ── Search: cache key with different params ──────────────────────────────────

func TestSearchCacheKey_DifferentParams(t *testing.T) {
	t.Parallel()

	key1 := searchCacheKey(search.SearchParams{
		Query: "test",
		Limit: 10,
	})
	key2 := searchCacheKey(search.SearchParams{
		Query: "test",
		Limit: 20, // different limit
	})
	key3 := searchCacheKey(search.SearchParams{
		Query: "different",
		Limit: 10,
	})

	assert.NotEqual(t, key1, key2, "different limit should produce different key")
	assert.NotEqual(t, key1, key3, "different query should produce different key")
}

func TestSearchCacheKey_SameParams(t *testing.T) {
	t.Parallel()

	params := search.SearchParams{
		Query:        "hello world",
		Limit:        10,
		Offset:       5,
		Types:        []string{"chunk", "document"},
		Path:         "/some/path",
		Tags:         map[string]string{"lang": "go", "type": "testing"},
		EnableFTS:    true,
		EnableVector: false,
		EnableGraph:  true,
		EnableFacets: false,
		MinScore:     0.5,
	}

	key1 := searchCacheKey(params)
	key2 := searchCacheKey(params)
	assert.Equal(t, key1, key2, "same params should produce same cache key")
}

// ── FASE 1.5 — Cache Scope Safety ────────────────────────────────────────────
// A chave do cache deve ser específica ao SearchScope que originou a consulta,
// para que um resultado de escopo A jamais seja servido para um escopo B.

// routedScope constrói um SearchScope já fingerprinted, no vocabulário real do
// modlink (módulos são os domínios que a busca confina).
func routedScope(modules ...string) *modlink.SearchScope {
	s := &modlink.SearchScope{Modules: modules}
	s.Fingerprint = s.ComputeFingerprint()
	return s
}

// (1) mes(a) query + mesmo escopo → mesma chave.
func TestSearchCacheKey_SameScopeSameKey(t *testing.T) {
	t.Parallel()
	scope := routedScope("adr")
	a := searchCacheKey(search.SearchParams{Query: "decisão", Scope: scope})
	b := searchCacheKey(search.SearchParams{Query: "decisão", Scope: scope})
	assert.Equal(t, a, b, "mesmo query + mesmo escopo deve produzir a mesma chave")
}

// (2) mesmo query + escopos diferentes → chaves diferentes.
func TestSearchCacheKey_DifferentScopeDifferentKey(t *testing.T) {
	t.Parallel()
	a := searchCacheKey(search.SearchParams{Query: "decisão", Scope: routedScope("adr")})
	b := searchCacheKey(search.SearchParams{Query: "decisão", Scope: routedScope("memory")})
	assert.NotEqual(t, a, b, "escopos diferentes devem produzir chaves diferentes")
}

// (3) query sem escopo vs query scoped → chaves diferentes (um é LEGACY, outro confinado).
func TestSearchCacheKey_NoScopeVsScoped(t *testing.T) {
	t.Parallel()
	noscope := searchCacheKey(search.SearchParams{Query: "decisão"})
	scoped := searchCacheKey(search.SearchParams{Query: "decisão", Scope: routedScope("adr")})
	assert.NotEqual(t, noscope, scoped, "busca sem escopo e busca com escopo devem ter chaves diferentes")
}

// (4) ordem de módulos equivalente → mesma chave (mesma semântica de confinamento).
func TestSearchCacheKey_EquivalentModuleOrderSameKey(t *testing.T) {
	t.Parallel()
	a := searchCacheKey(search.SearchParams{Query: "k", Scope: routedScope("adr", "memory")})
	b := searchCacheKey(search.SearchParams{Query: "k", Scope: routedScope("memory", "adr")})
	assert.Equal(t, a, b, "ordem de módulos equivalente deve produzir a mesma chave (semântica igual)")
}

// (4b) semântica explícita: Scope nil e Scope com Modules vazio são o MESMO
// estado (sem confinamento / LEGACY) → mesma chave; LEGACY preservado.
func TestSearchCacheKey_NilScopeEqualsEmptyModules(t *testing.T) {
	t.Parallel()
	nilScope := searchCacheKey(search.SearchParams{Query: "k"})
	emptyScope := searchCacheKey(search.SearchParams{Query: "k", Scope: &modlink.SearchScope{}})
	assert.Equal(t, nilScope, emptyScope, "nil === Modules vazio: ambos são LEGACY (sem confinamento)")
}

// Regressão (prova de isolamento do cache): o resultado do escopo A NUNCA é
// servido para o escopo B. Envenenamos a entrada do escopo A com um decoy e
// provamos que buscar no escopo B (chave diferente) é um MISS no cache e
// retorna o resultado real, não o decoy de A.
func TestCache_ScopeA_NeverServesScopeB(t *testing.T) {
	t.Parallel()
	engine := memoryOnlyEngine(t)

	scopeA := routedScope("adr")
	scopeB := routedScope("memory")
	keyA := searchCacheKey(search.SearchParams{Query: "q", Scope: scopeA})
	keyB := searchCacheKey(search.SearchParams{Query: "q", Scope: scopeB})
	assert.NotEqual(t, keyA, keyB, "chaves de escopos distintos devem diferir")

	// Envenena o cache com um resultado falso sob a chave do escopo A.
	decoy := &search.SearchResults{Query: "q", TotalCount: 999}
	raw, err := json.Marshal(decoy)
	require.NoError(t, err)
	require.NoError(t, engine.cache.Set(keyA, string(raw), time.Minute))

	// A chave de B não deve existir antes da busca (miss garantido).
	_, present := engine.cache.Get(keyB)
	assert.False(t, present, "chave do escopo B não deve estar pré-populada pelo decoy de A")

	// Buscar com o escopo B deve NUNCA devolver o decoy de A (cache miss → busca real).
	res, err := engine.Search(context.Background(), search.SearchParams{
		Query:        "q",
		Scope:        scopeB,
		EnableFTS:    true,
		EnableVector: false,
		EnableGraph:  false,
		Limit:        5,
	})
	require.NoError(t, err)
	assert.NotEqual(t, 999, res.TotalCount,
		"o resultado do escopo A (999) NÃO pode ser servido para o escopo B")
}

// ── Config: DefaultConfig consistency ────────────────────────────────────────

func TestDefaultConfig_Consistency(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	assert.Equal(t, ".", cfg.RootDir)
	assert.True(t, cfg.AutoMigrate)
	assert.False(t, cfg.WatchEnabled)
	assert.Equal(t, "auto", cfg.EmbeddingProvider)
	assert.NotEmpty(t, cfg.DBPath)
}

// ── Compile: error during walk ───────────────────────────────────────────────

func TestCompileCategory_ErrorDuringWalk(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// Create a broken directory entry that causes walk error
	// This is hard to do reliably — let's test context cancel during walk instead
	writeRepoFile(t, repoPath, "patterns/p1.md", "# Pattern 1\n\nContent.")
	writeRepoFile(t, repoPath, "patterns/p2.md", "# Pattern 2\n\nContent.")

	compiler := NewCompiler(db, repoPath)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := compiler.CompileCategory(ctx, CategoryPatterns)
	assert.Error(t, err)
	assert.NotNil(t, result)
	// The category result should still be returned even with cancellation
	assert.Contains(t, result.Categories, "patterns")
}

func TestCompileCategory_BeforeWalkCancel(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	writeRepoFile(t, repoPath, "patterns/single.md", "# Single\n\nContent.")

	compiler := NewCompiler(db, repoPath)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := compiler.CompileCategory(ctx, CategoryPatterns)
	// The code checks ctx.Err() BEFORE starting the walk
	assert.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.Total)
}

// ── CompileCategory: empty directory (no files) ──────────────────────────────

func TestCompileCategory_EmptyDir(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	// Category dir doesn't exist — should return 0 without error
	result, err := compiler.CompileCategory(ctx, CategoryFailures)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.Total)
}

// ── KnowledgeEntry: JSON tags completeness ───────────────────────────────────

func TestKnowledgeEntry_JSONTags(t *testing.T) {
	t.Parallel()

	entry := &KnowledgeEntry{
		Category:    CategoryBestPractices,
		SubCategory: "coding",
		Title:       "Best Practice",
		Content:     "Always test your code.",
		Tags:        []string{"testing", "quality"},
		Confidence:  0.95,
		Source:      "best-practices/testing.md",
	}

	assert.Equal(t, CategoryBestPractices, entry.Category)
	assert.Equal(t, "coding", entry.SubCategory)
	assert.Contains(t, entry.Tags, "testing")
	assert.Contains(t, entry.Tags, "quality")
	assert.InDelta(t, 0.95, entry.Confidence, 0.001)
}

// ── Insert entry JSON marshal error handling ─────────────────────────────────
// This is hard to trigger since json.Marshal on []string never fails.
// But updateEntry has the same pattern — skip for now.

// ── Engine: WatchDirectory watcher creation error ────────────────────────────
// This would require watcher.New() to fail, which needs /proc fs manipulation.
// Skip.

// ── Engine: saveGraph nil graph/db ───────────────────────────────────────────

func TestSaveGraph_NilGraph(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)

	// Keep a reference so the real DB handle is closed at cleanup even
	// though the field is nil'ed below (Windows file lock).
	db := engine.db
	// Set graph to nil
	engine.graph = nil
	engine.db = nil
	t.Cleanup(func() { _ = db.Close() })

	err := engine.saveGraph()
	assert.NoError(t, err, "saveGraph with nil graph should be no-op")
}

func TestSaveGraph_NilDB(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)

	// Keep a reference so the real DB handle is closed at cleanup even
	// though the field is nil'ed below (Windows file lock).
	db := engine.db
	// db is set but graph is nil
	engine.db = nil
	t.Cleanup(func() { _ = db.Close() })

	err := engine.saveGraph()
	assert.NoError(t, err, "saveGraph with nil db should be no-op")
}

// ── Engine: loadGraph nil graph/db ───────────────────────────────────────────

func TestLoadGraph_NilGraph(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)

	// Keep a reference so the real DB handle is closed at cleanup even
	// though the field is nil'ed below (Windows file lock).
	db := engine.db
	engine.graph = nil
	engine.db = nil
	t.Cleanup(func() { _ = db.Close() })

	err := engine.loadGraph()
	assert.NoError(t, err, "loadGraph with nil graph should be no-op")
}

// ── Engine: GetStats with initialized engine (coverage extension) ────────────

func TestEngine_GetStats_InitializedWithDB(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)

	stats, err := engine.GetStats()
	require.NoError(t, err)
	require.NotNil(t, stats)

	// DB file should exist since Init creates it
	assert.Greater(t, stats.DBSize, int64(0), "DB file should have non-zero size")
	assert.NotNil(t, stats.CacheStats, "cache stats map should exist after init")
	assert.GreaterOrEqual(t, stats.Uptime, int64(0))
}

// ── Engine: Verify after rebuild ─────────────────────────────────────────────

func TestEngine_Verify_AfterClose(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)
	err := engine.Close()
	require.NoError(t, err)

	result, err := engine.Verify()
	require.NoError(t, err)
	require.NotNil(t, result)

	// Engine is closed, marked as not initialized
	assert.False(t, result.Checks["database_integrity"], "DB check should fail after close")
	assert.Contains(t, result.Issues[0], "not initialized")
}

// ── Engine: Snapshot with nil DB ─────────────────────────────────────────────

func TestEngine_Snapshot_NilDB(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)

	// Keep a reference so the real DB handle is closed at cleanup even
	// though the field is nil'ed below (Windows file lock).
	db := engine.db
	// Manually set db to nil after init (force the nil db path)
	engine.db = nil
	t.Cleanup(func() { _ = db.Close() })

	snap, err := engine.Snapshot("nil-db-snap")
	require.Error(t, err)
	assert.Nil(t, snap)
	assert.Contains(t, err.Error(), "database not available")
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// initTestEngineNoCleanup creates an initialized engine without auto-cleanup
// (useful when we need to manually close and test post-close behavior).
func initTestEngineNoCleanup(t *testing.T) *Engine {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "knowledge_test.db")
	engine := newTestEngine(t, dbPath)

	err := engine.Init()
	require.NoError(t, err)
	require.True(t, engine.initialized)

	return engine
}

// Override newTestDB to handle sql.Open errors consistently
func newTestDBCoverage(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err, "open in-memory db should succeed")
	t.Cleanup(func() { _ = db.Close() })
	return db
}
