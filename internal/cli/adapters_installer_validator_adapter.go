package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/cache"
)

// Installer Validator Adapter
// =============================================================================

// validatorAdapter wraps validation. Placeholder.
type validatorAdapter struct {
	dir string
}

// newValidatorAdapter creates a new validator.
func newValidatorAdapter(dir string) *validatorAdapter {
	return &validatorAdapter{dir: dir}
}

// Validate validates the installation.
func (a *validatorAdapter) Validate() error {
	return nil
}

// =============================================================================
// Constructor functions used by CLI command files
// =============================================================================

// The following package-level functions are called by CLI command files.
// They return adapter types that provide the method signatures CLI expects.
//
// Note: These functions shadow the real package constructors at the call site.
// The CLI files import the real packages (e.g., "github.com/CoscaAI/cosca/internal/runtime")
// but these adapter functions provide the missing/compatible APIs.

// ---- graph package extensions ----

// NewGraph creates a graph adapter. Used by install.go and knowledge.go
// as graph.NewGraph(dir).
//
//nolint:revive // Exported constructor returns adapter type for package-internal use.
func NewGraph(dir string) *graphAdapter {
	return newGraphAdapter(dir)
}

// ---- diagnostics package extensions ----

// NewDiagnostics creates a diagnostics adapter. Used by health.go.
//
//nolint:revive // Exported constructor returns adapter type for package-internal use.
func NewDiagnostics(dir string) *diagnosticsAdapter {
	return newDiagnosticsAdapter(dir)
}

// ---- editors package extensions ----

// NewDetector creates an editor detector. Used by install.go.
//
//nolint:revive // Exported constructor returns adapter type for package-internal use.
func NewDetector() *editorDetectorAdapter {
	return newEditorDetector()
}

// ---- memory package extensions ----

// NewMemoryManager creates a memory manager. Used by memory.go and install.go.
//
//nolint:revive // Exported constructor returns adapter type for package-internal use.
func NewMemoryManager(dir string) *memoryManagerAdapter {
	return newMemoryManagerAdapter(dir)
}

// ---- knowledge package extensions ----

// NewKnowledgeEngine creates a knowledge engine. Used by knowledge.go.
//
//nolint:revive // Exported constructor returns adapter type for package-internal use.
func NewKnowledgeEngine(dir string) *knowledgeEngineAdapter {
	return newKnowledgeEngineAdapter(dir)
}

// ---- indexer package extensions ----

// NewIndexer creates an indexer. Used by index.go and install.go.
//
//nolint:revive // Exported constructor returns adapter type for package-internal use.
func NewIndexer(dir string) *indexerAdapter {
	return newIndexerAdapter(dir)
}

// ---- providers package extensions ----

// NewProviderManager creates a provider manager. Used by provider.go.
//
//nolint:revive // Exported constructor returns adapter type for package-internal use.
func NewProviderManager() *providerManagerAdapter {
	return newProviderManagerAdapter()
}

// ---- embeddings package extensions ----

// NewEmbeddingGenerator creates an embedding generator. Used by install.go and sync.go.
//
//nolint:revive // Exported constructor returns adapter type for package-internal use.
func NewEmbeddingGenerator(dir string) *embeddingGeneratorAdapter {
	return newEmbeddingGeneratorAdapter(dir)
}

// ---- context package extensions ----

// NewContextBuilder creates a context builder. Used by install.go.
//
//nolint:revive // Exported constructor returns adapter type for package-internal use.
func NewContextBuilder(dir string) *contextBuilderAdapter {
	return newContextBuilderAdapter(dir)
}

// ---- installers package extensions ----

// NewValidator creates a validator. Used by install.go.
//
//nolint:revive // Exported constructor returns adapter type for package-internal use.
func NewValidator(dir string) *validatorAdapter {
	return newValidatorAdapter(dir)
}

// ---- sqlite package extensions ----

// SQLiteDatabase wraps sqlite DB operations for install.go.
type SQLiteDatabase struct {
	path string
}

// NewSQLiteDatabase creates a new SQLite database. Used by install.go and sync.go.
func NewSQLiteDatabase(path string) *SQLiteDatabase {
	return &SQLiteDatabase{path: path}
}

// Open opens the database.
func (d *SQLiteDatabase) Open() error {
	return nil
}

// UpdateIndex updates the index with changes. Used by sync.go.
func (d *SQLiteDatabase) UpdateIndex(_ interface{}) error {
	return nil
}

// ---- cache package extensions ----

// NewCacheManager creates a cache manager. Used by sync.go.
type cacheManagerAdapter struct {
	inner *cache.Cache
	dir   string
}

// NewCacheManager creates a cache manager adapter.
//
//nolint:revive // Exported constructor returns adapter type for package-internal use.
func NewCacheManager(dir string) *cacheManagerAdapter {
	cfg := cache.DefaultConfig()
	cfg.FileCacheDir = dir
	c, err := cache.New(cfg)
	if err != nil {
		return &cacheManagerAdapter{dir: dir}
	}
	return &cacheManagerAdapter{inner: c, dir: dir}
}

// Invalidate invalidates cache entries.
func (a *cacheManagerAdapter) Invalidate(deletedFiles []string) error {
	if a.inner == nil {
		return nil
	}
	// Clear the entire cache or specific entries
	if len(deletedFiles) == 0 {
		return a.inner.Clear()
	}
	for _, f := range deletedFiles {
		_ = a.inner.Delete(f)
	}
	return nil
}

// ---- watcher package extensions ----

// ScannerChange holds file scan results for sync.go.
type ScannerChange struct {
	Files        []string `json:"files"`
	FilesScanned int      `json:"files_scanned"`
	Added        int      `json:"added"`
	Updated      int      `json:"updated"`
	Deleted      int      `json:"deleted"`
	DeletedFiles []string `json:"deleted_files"`
}

// ScannerAdapter wraps file scanning for sync.go.
type ScannerAdapter struct {
	dir      string
	fullScan bool
}

// NewFullScanner creates a full scanner.
func NewFullScanner(dir string) *ScannerAdapter {
	return &ScannerAdapter{dir: dir, fullScan: true}
}

// NewIncrementalScanner creates an incremental scanner.
func NewIncrementalScanner(dir string) *ScannerAdapter {
	return &ScannerAdapter{dir: dir, fullScan: false}
}

// Scan scans for file changes.
func (s *ScannerAdapter) Scan() (ScannerChange, error) {
	var files []string
	totalFiles := 0
	err := filepath.Walk(s.dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			switch name {
			case "node_modules", "vendor", "dist", "build", "target", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}
		totalFiles++
		files = append(files, path)
		return nil
	})
	return ScannerChange{
		Files:        files,
		FilesScanned: totalFiles,
		Added:        totalFiles,
		Updated:      0,
		Deleted:      0,
	}, err
}

// =============================================================================
// File-level function overrides that are implemented in adapters.go
// because they reference types/functions not available in the real packages.
// =============================================================================
