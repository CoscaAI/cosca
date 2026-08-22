package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chunker"
	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/markdown"
	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/CoscaAI/cosca/internal/vector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mockFileInfo ────────────────────────────────────────────────────────

// mockFileInfo implements os.FileInfo for testing shouldIndex.
type mockFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return 0o644 }
func (m mockFileInfo) ModTime() time.Time { return time.Now() }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() interface{}   { return nil }

// ── DefaultConfig ──────────────────────────────────────────────────────

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	assert.Equal(t, 20, cfg.EmbedBatchSize)
	assert.True(t, cfg.IndexCodeBlocks)
	assert.True(t, cfg.IndexTables)
	assert.True(t, cfg.ExtractEntities)
	assert.True(t, cfg.BuildGraph)
	assert.False(t, cfg.FollowSymlinks)
	assert.False(t, cfg.RequireEmbeddings)
	assert.Equal(t, int64(10*1024*1024), cfg.MaxFileSize)
	assert.Nil(t, cfg.ExcludedPatterns)
	assert.Nil(t, cfg.AllowedExtensions)
}

// ── IndexStats ─────────────────────────────────────────────────────────

func TestIndexStatsZeroValue(t *testing.T) {
	t.Parallel()

	stats := IndexStats{}
	assert.Equal(t, 0, stats.TotalDocuments)
	assert.Equal(t, 0, stats.TotalChunks)
	assert.Zero(t, stats.TotalEntities)
	assert.Zero(t, stats.TotalVectors)
	assert.Zero(t, stats.MissingVectors)
	assert.Zero(t, stats.TotalErrors)
}

func TestIndexStatsFields(t *testing.T) {
	t.Parallel()

	stats := IndexStats{
		TotalDocuments:  10,
		TotalChunks:     50,
		TotalEntities:   100,
		TotalVectors:    50,
		TotalErrors:     2,
		IndexedTypes:    map[string]int{"go": 5, "md": 5},
		LastIndexed:     time.Now(),
		Duration:        time.Second * 5,
		CacheHitRate:    0.8,
		DocumentsByType: map[string]int{"doc": 10},
	}

	assert.Equal(t, 10, stats.TotalDocuments)
	assert.Len(t, stats.IndexedTypes, 2)
	assert.Equal(t, time.Second*5, stats.Duration)
	assert.Equal(t, 0.8, stats.CacheHitRate)
	assert.Len(t, stats.DocumentsByType, 1)
}

// ── IndexError ─────────────────────────────────────────────────────────

func TestIndexError(t *testing.T) {
	t.Parallel()

	err := IndexError{
		Path:  "/test/file.md",
		Error: "parse error",
		Phase: "parse",
	}
	assert.Equal(t, "/test/file.md", err.Path)
	assert.Equal(t, "parse error", err.Error)
	assert.Equal(t, "parse", err.Phase)
}

// ── IndexerConfig ──────────────────────────────────────────────────────

func TestIndexerConfig(t *testing.T) {
	t.Parallel()

	cfg := IndexerConfig{
		RootDir:           "/project",
		EmbedBatchSize:    10,
		IndexCodeBlocks:   false,
		ExcludedPatterns:  []string{"*.gitignore"},
		AllowedExtensions: []string{".go", ".md"},
	}

	assert.Equal(t, "/project", cfg.RootDir)
	assert.Equal(t, 10, cfg.EmbedBatchSize)
	assert.False(t, cfg.IndexCodeBlocks)
	assert.Len(t, cfg.ExcludedPatterns, 1)
	assert.Len(t, cfg.AllowedExtensions, 2)
}

// ── New ────────────────────────────────────────────────────────────────

func TestNew(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	idx := New(
		cfg,
		nil, // mdParser
		nil, // entityParser
		nil, // chunker
		nil, // embRegistry
		nil, // vecStore
		nil, // ftsClient
		nil, // db
		nil, // graphBuilder
	)

	require.NotNil(t, idx)
	assert.Equal(t, cfg, idx.cfg)
	assert.NotNil(t, idx.docHashCache)
	assert.Empty(t, idx.docHashCache)
	assert.NotNil(t, idx.stats.IndexedTypes)
	assert.NotNil(t, idx.stats.DocumentsByType)
	assert.Equal(t, 0, idx.stats.TotalDocuments)
	assert.Equal(t, 0, idx.stats.TotalChunks)
}

// ── SetProgressCallback ────────────────────────────────────────────────

func TestSetProgressCallback(t *testing.T) {
	t.Parallel()

	idx := New(DefaultConfig(), nil, nil, nil, nil, nil, nil, nil, nil)

	var called atomic.Bool
	idx.SetProgressCallback(func(current, total int, phase string, err error) {
		called.Store(true)
	})

	require.NotNil(t, idx.progressCb)

	// reportProgress should invoke the callback
	idx.reportProgress(1, 5, "test", nil)
	assert.True(t, called.Load(), "callback should have been called")
}

func TestReportProgress_NilCallback(t *testing.T) {
	t.Parallel()

	idx := New(DefaultConfig(), nil, nil, nil, nil, nil, nil, nil, nil)

	// Should not panic when progressCb is nil
	idx.reportProgress(1, 5, "test", nil)
	idx.reportProgress(3, 10, "error", assert.AnError)
}

func TestReportProgress_CallbackReceivesValues(t *testing.T) {
	t.Parallel()

	idx := New(DefaultConfig(), nil, nil, nil, nil, nil, nil, nil, nil)

	var (
		gotCurrent atomic.Int32
		gotTotal   atomic.Int32
		gotPhase   atomic.Value
		gotErr     atomic.Value
	)

	idx.SetProgressCallback(func(current, total int, phase string, err error) {
		gotCurrent.Store(int32(current))
		gotTotal.Store(int32(total))
		gotPhase.Store(phase)
		if err != nil {
			gotErr.Store(err)
		}
	})

	testErr := assert.AnError
	idx.reportProgress(3, 7, "indexing", testErr)

	assert.EqualValues(t, 3, gotCurrent.Load())
	assert.EqualValues(t, 7, gotTotal.Load())
	assert.Equal(t, "indexing", gotPhase.Load())
	assert.NotNil(t, gotErr.Load())
}

// ── recordError / GetErrors ────────────────────────────────────────────

func TestRecordError_GetErrors(t *testing.T) {
	t.Parallel()

	idx := New(DefaultConfig(), nil, nil, nil, nil, nil, nil, nil, nil)

	// Initially no errors
	errors := idx.GetErrors()
	assert.Empty(t, errors)

	// Record an error
	idx.recordError("/path/file.md", assert.AnError, "parse")

	errors = idx.GetErrors()
	require.Len(t, errors, 1)
	assert.Equal(t, "/path/file.md", errors[0].Path)
	assert.Equal(t, "parse", errors[0].Phase)
	assert.Contains(t, errors[0].Error, assert.AnError.Error())
	assert.False(t, errors[0].Timestamp.IsZero())
}

func TestRecordError_MultipleErrors(t *testing.T) {
	t.Parallel()

	idx := New(DefaultConfig(), nil, nil, nil, nil, nil, nil, nil, nil)

	idx.recordError("/a", assert.AnError, "parse")
	idx.recordError("/b", assert.AnError, "chunk")
	idx.recordError("/c", assert.AnError, "embed")

	errors := idx.GetErrors()
	require.Len(t, errors, 3)
	assert.Equal(t, "/a", errors[0].Path)
	assert.Equal(t, "/b", errors[1].Path)
	assert.Equal(t, "/c", errors[2].Path)
	assert.Equal(t, "parse", errors[0].Phase)
	assert.Equal(t, "chunk", errors[1].Phase)
	assert.Equal(t, "embed", errors[2].Phase)
}

func TestGetErrors_ReturnsCopy(t *testing.T) {
	t.Parallel()

	idx := New(DefaultConfig(), nil, nil, nil, nil, nil, nil, nil, nil)
	idx.recordError("/original", assert.AnError, "parse")

	errors := idx.GetErrors()
	require.Len(t, errors, 1)

	// Mutate the returned slice — should not affect internal state
	errors[0].Path = "/modified"
	errors = idx.GetErrors()
	assert.Equal(t, "/original", errors[0].Path)
}

// ── GetIndexStats ──────────────────────────────────────────────────────

func TestGetIndexStats_Initial(t *testing.T) {
	t.Parallel()

	idx := New(DefaultConfig(), nil, nil, nil, nil, nil, nil, nil, nil)
	stats := idx.GetIndexStats()

	assert.Equal(t, 0, stats.TotalDocuments)
	assert.Equal(t, 0, stats.TotalChunks)
	assert.Equal(t, 0, stats.TotalEntities)
	assert.Equal(t, 0, stats.TotalVectors)
	assert.Equal(t, 0, stats.TotalErrors)
	assert.NotNil(t, stats.IndexedTypes)
	assert.NotNil(t, stats.DocumentsByType)
}

// ── computeHash ────────────────────────────────────────────────────────

func TestComputeHash(t *testing.T) {
	t.Parallel()

	h1 := computeHash("test content")
	h2 := computeHash("test content")
	h3 := computeHash("different")

	assert.Equal(t, h1, h2, "same content should produce same hash")
	assert.NotEqual(t, h1, h3, "different content should produce different hash")
	assert.Len(t, h1, 64) // SHA-256 hex is 64 chars
}

func TestComputeHash_EmptyString(t *testing.T) {
	t.Parallel()

	h := computeHash("")
	assert.Len(t, h, 64)
	// SHA-256 of empty string
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", h)
}

// ── toJSON ─────────────────────────────────────────────────────────────

func TestToJSON(t *testing.T) {
	t.Parallel()

	data, err := toJSON(map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.Contains(t, data, `"key"`)
	assert.Contains(t, data, `"value"`)
}

func TestToJSON_Empty(t *testing.T) {
	t.Parallel()

	data, err := toJSON(map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "{}", data)
}

func TestToJSON_Nil(t *testing.T) {
	t.Parallel()

	data, err := toJSON(nil)
	require.NoError(t, err)
	assert.Equal(t, "null", data)
}

func TestToJSON_Complex(t *testing.T) {
	t.Parallel()

	data, err := toJSON(map[string]interface{}{
		"title":    "Test Doc",
		"path":     "/docs/test.md",
		"type":     "markdown",
		"headings": 5,
		"links":    10,
	})
	require.NoError(t, err)
	assert.Contains(t, data, `"Test Doc"`)
	assert.Contains(t, data, `"markdown"`)
}

// ── shouldIndex ────────────────────────────────────────────────────────

func TestShouldIndex(t *testing.T) {
	t.Parallel()

	const oneMB = 1024 * 1024
	defaultCfg := IndexerConfig{MaxFileSize: 10 * oneMB}

	tests := []struct {
		name string
		cfg  IndexerConfig
		path string
		info mockFileInfo
		want bool
	}{
		// ── Size tests ──
		{
			name: "within size limit",
			cfg:  defaultCfg,
			path: "/test/file.md",
			info: mockFileInfo{name: "file.md", size: 100},
			want: true,
		},
		{
			name: "at exact size limit",
			cfg:  defaultCfg,
			path: "/test/big.md",
			info: mockFileInfo{name: "big.md", size: 10 * oneMB},
			want: true, // size <= MaxFileSize
		},
		{
			name: "exceeds size limit by one byte",
			cfg:  defaultCfg,
			path: "/test/huge.md",
			info: mockFileInfo{name: "huge.md", size: 10*oneMB + 1},
			want: false,
		},
		{
			name: "far exceeds size limit",
			cfg:  defaultCfg,
			path: "/test/giant.bin",
			info: mockFileInfo{name: "giant.bin", size: 100 * oneMB},
			want: false,
		},
		{
			name: "zero size file",
			cfg:  defaultCfg,
			path: "/test/empty.md",
			info: mockFileInfo{name: "empty.md", size: 0},
			want: true,
		},
		{
			name: "tiny size limit",
			cfg:  IndexerConfig{MaxFileSize: 10},
			path: "/test/small.go",
			info: mockFileInfo{name: "small.go", size: 11},
			want: false,
		},
		{
			name: "tiny file with tiny limit",
			cfg:  IndexerConfig{MaxFileSize: 10},
			path: "/test/tiny.go",
			info: mockFileInfo{name: "tiny.go", size: 5},
			want: true,
		},

		// ── Extension tests ──
		{
			name: "allowed extension with dot prefix",
			cfg:  IndexerConfig{MaxFileSize: oneMB, AllowedExtensions: []string{".md"}},
			path: "/test/file.md",
			info: mockFileInfo{name: "file.md", size: 100},
			want: true,
		},
		{
			name: "allowed extension without dot prefix",
			cfg:  IndexerConfig{MaxFileSize: oneMB, AllowedExtensions: []string{"md"}},
			path: "/test/file.MD",
			info: mockFileInfo{name: "file.MD", size: 100},
			want: true, // case insensitive
		},
		{
			name: "not in allowed extensions",
			cfg:  IndexerConfig{MaxFileSize: oneMB, AllowedExtensions: []string{".md"}},
			path: "/test/file.go",
			info: mockFileInfo{name: "file.go", size: 100},
			want: false,
		},
		{
			name: "multiple allowed extensions, one matches",
			cfg:  IndexerConfig{MaxFileSize: oneMB, AllowedExtensions: []string{".go", ".md", ".yaml"}},
			path: "/test/file.yaml",
			info: mockFileInfo{name: "file.yaml", size: 200},
			want: true,
		},
		{
			name: "multiple allowed extensions, none match",
			cfg:  IndexerConfig{MaxFileSize: oneMB, AllowedExtensions: []string{".go", ".md"}},
			path: "/test/script.py",
			info: mockFileInfo{name: "script.py", size: 300},
			want: false,
		},
		{
			name: "no extension in path but extensions required",
			cfg:  IndexerConfig{MaxFileSize: oneMB, AllowedExtensions: []string{".md"}},
			path: "/test/README",
			info: mockFileInfo{name: "README", size: 100},
			want: false,
		},
		{
			name: "empty allowed extensions list accepts all",
			cfg:  IndexerConfig{MaxFileSize: oneMB, AllowedExtensions: []string{}},
			path: "/test/anything.xyz",
			info: mockFileInfo{name: "anything.xyz", size: 100},
			want: true,
		},
		{
			name: "nil allowed extensions list accepts all",
			cfg:  IndexerConfig{MaxFileSize: oneMB, AllowedExtensions: nil},
			path: "/test/anything.xyz",
			info: mockFileInfo{name: "anything.xyz", size: 100},
			want: true,
		},

		// ── Excluded patterns tests ──
		{
			name: "matches excluded pattern",
			cfg:  IndexerConfig{MaxFileSize: oneMB, ExcludedPatterns: []string{"*.gitignore"}},
			path: "/test/.gitignore",
			info: mockFileInfo{name: ".gitignore", size: 50},
			want: false,
		},
		{
			name: "does not match excluded pattern",
			cfg:  IndexerConfig{MaxFileSize: oneMB, ExcludedPatterns: []string{"*.gitignore"}},
			path: "/test/file.md",
			info: mockFileInfo{name: "file.md", size: 50},
			want: true,
		},
		{
			name: "multiple excluded patterns, matches one",
			cfg:  IndexerConfig{MaxFileSize: oneMB, ExcludedPatterns: []string{"*.tmp", "*.bak", "*.swp"}},
			path: "/test/backup.bak",
			info: mockFileInfo{name: "backup.bak", size: 50},
			want: false,
		},
		{
			name: "multiple excluded patterns, matches none",
			cfg:  IndexerConfig{MaxFileSize: oneMB, ExcludedPatterns: []string{"*.tmp", "*.bak", "*.swp"}},
			path: "/test/file.go",
			info: mockFileInfo{name: "file.go", size: 50},
			want: true,
		},
		{
			name: "empty excluded patterns list",
			cfg:  IndexerConfig{MaxFileSize: oneMB, ExcludedPatterns: []string{}},
			path: "/test/file.md",
			info: mockFileInfo{name: "file.md", size: 50},
			want: true,
		},
		{
			name: "nil excluded patterns list",
			cfg:  IndexerConfig{MaxFileSize: oneMB, ExcludedPatterns: nil},
			path: "/test/file.md",
			info: mockFileInfo{name: "file.md", size: 50},
			want: true,
		},
		{
			name: "excluded pattern matches hidden files",
			cfg:  IndexerConfig{MaxFileSize: oneMB, ExcludedPatterns: []string{".*"}},
			path: "/test/.hidden",
			info: mockFileInfo{name: ".hidden", size: 50},
			want: false,
		},

		// ── Combination tests ──
		{
			name: "allowed extension but excluded",
			cfg: IndexerConfig{
				MaxFileSize:       oneMB,
				AllowedExtensions: []string{".md", ".go"},
				ExcludedPatterns:  []string{"*_test.go"},
			},
			path: "/test/file_test.go",
			info: mockFileInfo{name: "file_test.go", size: 500},
			want: false, // matches extension but excluded
		},
		{
			name: "allowed extension and not excluded",
			cfg: IndexerConfig{
				MaxFileSize:       oneMB,
				AllowedExtensions: []string{".md", ".go"},
				ExcludedPatterns:  []string{"*_test.go"},
			},
			path: "/test/file.go",
			info: mockFileInfo{name: "file.go", size: 500},
			want: true,
		},
		{
			name: "exceeds size regardless of extension match",
			cfg: IndexerConfig{
				MaxFileSize:       10,
				AllowedExtensions: []string{".md"},
			},
			path: "/test/small.md",
			info: mockFileInfo{name: "small.md", size: 100},
			want: false, // size check comes first
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			idx := &Indexer{cfg: tt.cfg}
			got := idx.shouldIndex(tt.path, tt.info)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ── embedChunks ────────────────────────────────────────────────────────

func TestEmbedChunks_NilRegistry(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	idx := New(cfg,
		nil, // mdParser
		nil, // entityParser
		nil, // chunker
		nil, // embRegistry — explicitly nil
		nil, // vecStore
		nil, // ftsClient
		nil, // db
		nil, // graphBuilder
	)

	chunks := []chunker.Chunk{
		{ID: "c1", Content: "hello world"},
		{ID: "c2", Content: "another chunk"},
	}
	vectors, err := idx.embedChunks(context.Background(), chunks)
	require.NoError(t, err)
	assert.Nil(t, vectors, "should return nil vectors when embRegistry is nil")
}

func TestEmbedChunks_NilRegistry_ReportsMissingVectors(t *testing.T) {
	t.Parallel()

	idx := New(DefaultConfig(), nil, nil, nil, nil, nil, nil, nil, nil)
	chunks := []chunker.Chunk{{ID: "c1", Content: "hello"}, {ID: "c2", Content: "world"}}

	vectors, missing, err := idx.embedChunksWithMissing(context.Background(), chunks)
	require.NoError(t, err)
	assert.Empty(t, vectors)
	assert.Equal(t, 2, missing)
}

func TestEmbedChunks_NilRegistry_RequiredFailsClearly(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.RequireEmbeddings = true
	idx := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil)

	_, missing, err := idx.embedChunksWithMissing(context.Background(), []chunker.Chunk{{ID: "c1", Content: "hello"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embeddings required")
	assert.Zero(t, missing)
}

func TestEmbedChunks_EmptyChunks(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	// Use a non-nil ProviderRegistry so we don't hit the nil-registry early return
	embRegistry := &embeddings.ProviderRegistry{}
	idx := New(cfg,
		nil,         // mdParser
		nil,         // entityParser
		nil,         // chunker
		embRegistry, // non-nil, hits len(chunks)==0 early return
		nil,         // vecStore
		nil,         // ftsClient
		nil,         // db
		nil,         // graphBuilder
	)

	vectors, err := idx.embedChunks(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, vectors)

	vectors, err = idx.embedChunks(context.Background(), []chunker.Chunk{})
	require.NoError(t, err)
	assert.Nil(t, vectors)
}

// ── IndexDocument error paths via invalid paths ─────────────────────────

func TestIndexDocument_NonexistentFile(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.RootDir = t.TempDir()
	idx := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil)

	// The file is inside RootDir but does not exist — the read path must
	// fail with a "read file" error (after passing containment).
	err := idx.IndexDocument(context.Background(), filepath.Join(cfg.RootDir, "missing.md"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read file")
}

func TestIndexDocument_EmptyPath(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	idx := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil)

	err := idx.IndexDocument(context.Background(), "")
	require.Error(t, err)
}

// TestIndexPathTraversal verifies the C1 containment fix: documents outside
// the configured RootDir — including absolute paths to system files, ".."
// escapes, and symlink escapes — cannot be indexed.
func TestIndexPathTraversal(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	outsideDir := t.TempDir()

	inside := filepath.Join(rootDir, "inside.md")
	require.NoError(t, os.WriteFile(inside, []byte("# Inside"), 0o644))

	outside := filepath.Join(outsideDir, "outside.md")
	require.NoError(t, os.WriteFile(outside, []byte("# Outside"), 0o644))

	cfg := DefaultConfig()
	cfg.RootDir = rootDir
	idx := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	t.Run("absolute path outside root", func(t *testing.T) {
		err := idx.IndexDocument(ctx, outside)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path outside root directory")
	})

	t.Run("system file outside root", func(t *testing.T) {
		err := idx.IndexDocument(ctx, "/etc/passwd")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path outside root directory")
	})

	t.Run("dotdot escape resolves outside root", func(t *testing.T) {
		escaped := filepath.Join(rootDir, "..", filepath.Base(outsideDir), "outside.md")
		err := idx.IndexDocument(ctx, escaped)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path outside root directory")
	})

	t.Run("symlink inside root pointing outside", func(t *testing.T) {
		link := filepath.Join(rootDir, "escape.md")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("symlinks not supported: %v", err)
		}
		err := idx.IndexDocument(ctx, link)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path outside root directory")
	})

	t.Run("legitimate file inside root passes containment", func(t *testing.T) {
		resolved, err := validatePathWithin(cfg.RootDir, inside)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(rootDir, "inside.md"), resolved)
	})

	t.Run("missing file inside root passes containment", func(t *testing.T) {
		resolved, err := validatePathWithin(cfg.RootDir, filepath.Join(rootDir, "future.md"))
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(rootDir, "future.md"), resolved)
	})
}

// IndexDocument requires non-nil markdown.Parser, chunker, sqlite.DB, etc.
// It cannot be tested with nil dependencies — attempting to call Parse
// on a nil Parser triggers a nil pointer dereference. Full integration
// tests for the indexing pipeline require a complete test harness with
// real or mock implementations of all concrete dependency types.

// ── IndexDirectory traversal and filtering ─────────────────────────────

func TestIndexDirectory_NonexistentDir(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	idx := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil)

	// IndexDirectory logs walk errors but continues; a nonexistent directory
	// results in 0 files being found and returns nil (no error).
	err := idx.IndexDirectory(context.Background(), "/nonexistent/directory")
	require.NoError(t, err)

	// Verify stats are reset
	stats := idx.GetIndexStats()
	assert.Equal(t, 0, stats.TotalDocuments)
}

// IndexDirectory requires full pipeline dependencies (markdown.Parser, chunker, etc.)
// which are concrete types. Testing the full walk would require real database and
// embedding infrastructure. The filtering logic is tested via shouldIndex below.

func TestIndexDirectory_ExtensionFiltering(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create files with various extensions
	require.NoError(t, os.WriteFile(dir+"/readme.md", []byte("# Readme"), 0o644))
	require.NoError(t, os.WriteFile(dir+"/main.go", []byte("package main"), 0o644))
	require.NoError(t, os.WriteFile(dir+"/config.yaml", []byte("key: val"), 0o644))
	require.NoError(t, os.WriteFile(dir+"/notes.txt", []byte("notes"), 0o644))

	cfg := DefaultConfig()
	cfg.AllowedExtensions = []string{".md", ".go"}
	idx := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil)

	// Verify shouldIndex works correctly with real os.FileInfo
	readmeInfo, err := os.Stat(dir + "/readme.md")
	require.NoError(t, err)
	assert.True(t, idx.shouldIndex(dir+"/readme.md", readmeInfo))

	goInfo, err := os.Stat(dir + "/main.go")
	require.NoError(t, err)
	assert.True(t, idx.shouldIndex(dir+"/main.go", goInfo))

	yamlInfo, err := os.Stat(dir + "/config.yaml")
	require.NoError(t, err)
	assert.False(t, idx.shouldIndex(dir+"/config.yaml", yamlInfo))

	txtInfo, err := os.Stat(dir + "/notes.txt")
	require.NoError(t, err)
	assert.False(t, idx.shouldIndex(dir+"/notes.txt", txtInfo))
}

func TestIndexDirectory_SizeFiltering(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a small file
	require.NoError(t, os.WriteFile(dir+"/small.md", []byte("tiny"), 0o644))

	// Create a larger file
	largeData := make([]byte, 5*1024) // 5KB
	for i := range largeData {
		largeData[i] = 'a'
	}
	require.NoError(t, os.WriteFile(dir+"/large.md", largeData, 0o644))

	cfg := DefaultConfig()
	cfg.MaxFileSize = 1024 // 1KB
	idx := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil)

	smallInfo, err := os.Stat(dir + "/small.md")
	require.NoError(t, err)
	assert.True(t, idx.shouldIndex(dir+"/small.md", smallInfo))

	largeInfo, err := os.Stat(dir + "/large.md")
	require.NoError(t, err)
	assert.False(t, idx.shouldIndex(dir+"/large.md", largeInfo))
}

func TestIndexDirectory_ExcludedPatterns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	require.NoError(t, os.WriteFile(dir+"/file.go", []byte("code"), 0o644))
	require.NoError(t, os.WriteFile(dir+"/file_test.go", []byte("test code"), 0o644))
	require.NoError(t, os.WriteFile(dir+"/.gitignore", []byte("*.exe"), 0o644))

	cfg := DefaultConfig()
	cfg.ExcludedPatterns = []string{"*_test.go", "*.gitignore"}
	idx := New(cfg, nil, nil, nil, nil, nil, nil, nil, nil)

	normalInfo, _ := os.Stat(dir + "/file.go")
	assert.True(t, idx.shouldIndex(dir+"/file.go", normalInfo))

	testInfo, _ := os.Stat(dir + "/file_test.go")
	assert.False(t, idx.shouldIndex(dir+"/file_test.go", testInfo))

	gitInfo, _ := os.Stat(dir + "/.gitignore")
	assert.False(t, idx.shouldIndex(dir+"/.gitignore", gitInfo))
}

// ── SetProgressCallback with multiple goroutines ───────────────────────

func TestSetProgressCallback_Concurrent(t *testing.T) {
	t.Parallel()

	idx := New(DefaultConfig(), nil, nil, nil, nil, nil, nil, nil, nil)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			idx.SetProgressCallback(func(current, total int, phase string, err error) {})
		}
	}()

	// Concurrent reads should not race
	for i := 0; i < 100; i++ {
		idx.reportProgress(1, 5, "test", nil)
	}

	<-done
}

// ── GetErrors concurrency safety ───────────────────────────────────────

func TestRecordError_Concurrent(t *testing.T) {
	t.Parallel()

	idx := New(DefaultConfig(), nil, nil, nil, nil, nil, nil, nil, nil)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		i := i
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 10; j++ {
				idx.recordError("/file"+string(rune('0'+i)), assert.AnError, "phase")
			}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	errors := idx.GetErrors()
	assert.Len(t, errors, 100)
}

// TestStoreDocument_ConcurrentSQLiteReplacement verifies that the vector
// writes participate in the document transaction and that concurrent callers
// of the indexer do not hit SQLITE_BUSY_SNAPSHOT while replacing documents.
func TestStoreDocument_ConcurrentSQLiteReplacement(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.DefaultConfig(filepath.Join(t.TempDir(), "knowledge.db")))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	vecStore, err := vector.NewSQLiteVec(vector.SQLiteVecConfig{DB: db.Conn(), Dimension: 2})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, vecStore.Close()) })

	idx := New(DefaultConfig(), nil, nil, nil, nil, vecStore, nil, db, nil)
	start := make(chan struct{})
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			<-start
			docID := fmt.Sprintf("doc-%d", i)
			chunkID := fmt.Sprintf("chunk-%d", i)
			doc := &markdown.Document{Title: docID, RawContent: "content", TokenCount: 1}
			chunks := []chunker.Chunk{{ID: chunkID, DocumentID: docID, Content: "content", Hash: chunkID, TokenCount: 1}}
			vectors := []vector.VectorRecord{{ID: "vector-" + docID, Vector: []float64{1, 0}, DocumentID: docID, ChunkID: chunkID, Content: "content"}}
			errs <- idx.storeDocument(docID, filepath.Join(t.TempDir(), docID+".md"), docID, doc, chunks, vectors)
		}()
	}
	close(start)
	for i := 0; i < 2; i++ {
		require.NoError(t, <-errs)
	}

	var documents, vectors int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&documents))
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&vectors))
	assert.Equal(t, 2, documents)
	// Concorrência: este teste usa o schema sem as colunas de dedup
	// (fail-safe do L371 — o dedup só atua quando a coluna dedup_of existe).
	// O dedup preventivo tem teste dedicado (TestCampaignDedupPreventive).
	assert.Equal(t, 2, vectors)
}
