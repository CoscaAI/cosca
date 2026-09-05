package memory

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"github.com/CoscaAI/cosca/internal/safe"

	// sqlite3 driver used for memory persistence layer.
	_ "modernc.org/sqlite"
)

// memoryIDPattern matches simple memory identifiers: UUIDs, slugs, or
// alphanumeric strings with dots, underscores, and hyphens.
var memoryIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidateMemoryID rejects memory IDs that could be abused for path
// traversal (C2). IDs must be simple identifiers — UUID, slug, or
// alphanumeric with hyphen/dot/underscore — never path-like values.
// It is used by the file store (Get/Delete/Save) and by the REST handlers
// that accept an id from the query string.
func ValidateMemoryID(id string) error {
	if id == "" {
		return fmt.Errorf("invalid memory id: empty")
	}
	// Belt-and-suspenders: reject separators and parent-directory segments
	// explicitly so the check never depends on regex edge cases.
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return fmt.Errorf("invalid memory id: %q", id)
	}
	if !memoryIDPattern.MatchString(id) {
		return fmt.Errorf("invalid memory id: %q", id)
	}
	return nil
}

// Store defines the interface for memory storage backends.
type Store interface {
	Save(ctx context.Context, record MemoryRecord) (*MemoryRecord, error)
	Get(ctx context.Context, id string) (*MemoryRecord, error)
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, query string, opts SearchOptions) ([]MemoryRecord, error)
	Index(ctx context.Context, record MemoryRecord) error
	Prune(ctx context.Context) (int, error)
	Stats(ctx context.Context) (LayerStats, error)
	Close() error
}

// SearchOptions configures memory search.
type SearchOptions struct {
	Types    []MemoryType  `json:"types,omitempty"`
	Layers   []MemoryLayer `json:"layers,omitempty"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
	MinScore float64       `json:"min_score"`

	// OwnerFilter restricts results to records owned by the given user
	// (claims.Sub) plus system/global records (empty owner). An empty
	// OwnerFilter disables owner scoping (admin / system callers see
	// everything) — A6: prevents IDOR across user memory.
	OwnerFilter  string `json:"owner_filter,omitempty"`
	TenantFilter string `json:"tenant_filter,omitempty"`

	// AgentFilter restricts results to records belonging to the given agent
	// (Metadata["agent"]) plus system/global records (empty agent). An empty
	// AgentFilter disables agent scoping (admin / system callers see
	// everything) — A7: prevents cross-agent memory contamination.
	AgentFilter string `json:"agent_filter,omitempty"`
}

// FileStore implements Store using Markdown files with YAML frontmatter.
type FileStore struct {
	mu     sync.RWMutex
	logger zerolog.Logger
	dir    string
	layer  MemoryLayer
	index  *SQLiteIndex
}

// NewFileStore creates a new file-based memory store.
func NewFileStore(dataDir string, layer MemoryLayer, logger zerolog.Logger) (*FileStore, error) {
	dir := filepath.Join(dataDir, "memory", string(layer))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating memory directory: %w", err)
	}

	idx, err := NewSQLiteIndex(filepath.Join(dataDir, "memory", "index.db"), logger)
	if err != nil {
		return nil, fmt.Errorf("creating index: %w", err)
	}

	return &FileStore{
		logger: logger,
		dir:    dir,
		layer:  layer,
		index:  idx,
	}, nil
}

// Save stores a memory record as a Markdown file with YAML frontmatter.
func (fs *FileStore) Save(ctx context.Context, record MemoryRecord) (*MemoryRecord, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	// Defense in depth: never write a memory file outside fs.dir.
	if err := ValidateMemoryID(record.ID); err != nil {
		return nil, err
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	record.UpdatedAt = time.Now()

	// Build YAML frontmatter
	frontmatter := struct {
		ID        string            `yaml:"id"`
		Type      MemoryType        `yaml:"type"`
		Layer     MemoryLayer       `yaml:"layer"`
		Scope     string            `yaml:"scope,omitempty"`
		Owner     string            `yaml:"owner,omitempty"`
		TenantID  string            `yaml:"tenant_id,omitempty"`
		Agent     string            `yaml:"agent,omitempty"`
		CreatedAt time.Time         `yaml:"created_at"`
		UpdatedAt time.Time         `yaml:"updated_at"`
		TTL       string            `yaml:"ttl,omitempty"`
		Priority  int               `yaml:"priority"`
		Version   int               `yaml:"version"`
		Metadata  map[string]string `yaml:"metadata,omitempty"`
	}{
		ID:        record.ID,
		Type:      record.Type,
		Layer:     record.Layer,
		Scope:     record.Scope,
		Owner:     record.Owner,
		TenantID:  record.TenantID,
		Agent:     record.Agent,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
		TTL:       record.TTL.String(),
		Priority:  record.Priority,
		Version:   record.Version,
		Metadata:  record.Metadata,
	}

	fmBytes, err := yaml.Marshal(frontmatter)
	if err != nil {
		return nil, fmt.Errorf("marshaling frontmatter: %w", err)
	}

	// Build file content: YAML frontmatter + body
	content := fmt.Sprintf("---\n%s---\n\n%s\n", string(fmBytes), record.Content)

	// Write file
	filePath := filepath.Join(fs.dir, record.ID+".md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("writing memory file: %w", err)
	}

	// Index for search
	if err := fs.index.AddRecord(ctx, record); err != nil {
		fs.logger.Warn().Err(err).Str("id", record.ID).Msg("indexing record")
	}

	fs.logger.Debug().
		Str("id", record.ID).
		Str("path", filePath).
		Msg("memory record saved to file")

	return &record, nil
}

// Get retrieves a memory record by ID.
func (fs *FileStore) Get(_ context.Context, id string) (*MemoryRecord, error) {
	// Reject path-like IDs (C2: path traversal prevention).
	if err := ValidateMemoryID(id); err != nil {
		return nil, err
	}

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	filePath, err := fs.resolveMemoryPath(id)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("record %s not found", id)
	}

	return fs.parseFile(filePath)
}

// Delete removes a memory record.
func (fs *FileStore) Delete(ctx context.Context, id string) error {
	// Reject path-like IDs (C2: path traversal prevention).
	if err := ValidateMemoryID(id); err != nil {
		return err
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	filePath, err := fs.resolveMemoryPath(id)
	if err != nil {
		return err
	}

	if err := os.Remove(filePath); os.IsNotExist(err) {
		return nil // Already deleted
	} else if err != nil {
		return fmt.Errorf("deleting memory file: %w", err)
	}

	if err := fs.index.RemoveRecord(ctx, id); err != nil {
		fs.logger.Warn().Err(err).Str("id", id).Msg("removing from index")
	}

	return nil
}

// resolveMemoryPath joins a validated memory id with the store directory and
// performs a defense-in-depth, symlink-aware containment check so the final
// path can never escape fs.dir (C2). The id must already pass
// ValidateMemoryID; this second layer guards against symlink tricks and
// any future caller that bypasses the id pattern.
func (fs *FileStore) resolveMemoryPath(id string) (string, error) {
	filePath := filepath.Join(fs.dir, id+".md")

	dirAbs, err := filepath.Abs(fs.dir)
	if err != nil {
		return "", fmt.Errorf("resolving memory directory: %w", err)
	}
	dirAbs = filepath.Clean(dirAbs)

	// Resolve the store directory through symlinks so a symlinked data dir
	// does not produce false rejections (or false accepts).
	rootResolved := dirAbs
	if r, err := filepath.EvalSymlinks(dirAbs); err == nil {
		rootResolved = r
	}

	resolved := filePath
	if r, err := filepath.EvalSymlinks(filePath); err == nil {
		resolved = r
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("resolving memory path: %w", err)
	}

	if !pathWithinDir(rootResolved, resolved) {
		return "", fmt.Errorf("invalid memory id: %q", id)
	}
	return filePath, nil
}

// pathWithinDir reports whether child is equal to or a descendant of parent.
func pathWithinDir(parent, child string) bool {
	rel, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Search searches for memory records.
func (fs *FileStore) Search(ctx context.Context, query string, opts SearchOptions) ([]MemoryRecord, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Use SQLite index for full-text search
	indexResults, err := fs.index.Search(ctx, query, opts)
	if err != nil {
		fs.logger.Warn().Err(err).Msg("index search failed, falling back to file scan")
		return fs.searchFallback(ctx, query, opts)
	}

	var records []MemoryRecord
	for _, idx := range indexResults {
		record, err := fs.Get(ctx, idx.ID)
		if err != nil {
			fs.logger.Warn().Err(err).Str("id", idx.ID).Msg("loading indexed record")
			continue
		}
		records = append(records, *record)
	}

	return records, nil
}

// Index indexes a record in the search index.
func (fs *FileStore) Index(ctx context.Context, record MemoryRecord) error {
	return fs.index.AddRecord(ctx, record)
}

// RebuildIndex rebuilds the FTS index for this layer from the files on
// disk. Orphaned entries whose files no longer exist are dropped, and files
// that were never indexed (e.g. written before the SQLite index existed, or
// restored manually into .cosca) become searchable again.
func (fs *FileStore) RebuildIndex(ctx context.Context) (int, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if err := fs.index.DeleteLayer(ctx, fs.layer); err != nil {
		return 0, fmt.Errorf("clearing layer index: %w", err)
	}

	entries, err := os.ReadDir(fs.dir)
	if err != nil {
		return 0, fmt.Errorf("reading memory dir: %w", err)
	}

	indexed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(fs.dir, entry.Name())
		record, err := fs.parseFile(filePath)
		if err != nil {
			fs.logger.Warn().Err(err).Str("path", filePath).Msg("skipping unreadable memory file during reindex")
			continue
		}
		if record.ID == "" {
			record.ID = strings.TrimSuffix(entry.Name(), ".md")
		}
		if err := fs.index.AddRecord(ctx, *record); err != nil {
			fs.logger.Warn().Err(err).Str("id", record.ID).Msg("indexing record during reindex")
			continue
		}
		indexed++
	}

	return indexed, nil
}

// Prune removes expired records.
func (fs *FileStore) Prune(ctx context.Context) (int, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	entries, err := os.ReadDir(fs.dir)
	if err != nil {
		return 0, fmt.Errorf("reading memory directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(fs.dir, entry.Name())
		record, err := fs.parseFile(filePath)
		if err != nil {
			continue
		}

		if record.TTL > 0 && time.Since(record.CreatedAt) > record.TTL {
			if err := os.Remove(filePath); err == nil {
				_ = fs.index.RemoveRecord(ctx, record.ID)
				count++
			}
		}
	}

	return count, nil
}

// Stats returns layer statistics.
func (fs *FileStore) Stats(_ context.Context) (LayerStats, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	entries, err := os.ReadDir(fs.dir)
	if err != nil {
		return LayerStats{}, err
	}

	stats := LayerStats{
		Name:  string(fs.layer),
		Count: 0,
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			stats.Count++
			filePath := filepath.Join(fs.dir, entry.Name())
			record, err := fs.parseFile(filePath)
			if err != nil {
				continue
			}
			stats.TotalSize += len(record.Content)
			if record.Priority > stats.HighestPriority {
				stats.HighestPriority = record.Priority
			}
		}
	}

	return stats, nil
}

// Close closes the store and its index.
func (fs *FileStore) Close() error {
	return fs.index.Close()
}

// parseFile parses a Markdown file with YAML frontmatter.
func (fs *FileStore) parseFile(filePath string) (*MemoryRecord, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(data)

	// Check for YAML frontmatter
	if !strings.HasPrefix(content, "---") {
		return &MemoryRecord{
			ID:      strings.TrimSuffix(filepath.Base(filePath), ".md"),
			Content: content,
			Layer:   fs.layer,
		}, nil
	}

	// Extract frontmatter
	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid frontmatter in %s", filePath)
	}

	var fm struct {
		ID        string            `yaml:"id"`
		Type      MemoryType        `yaml:"type"`
		Layer     MemoryLayer       `yaml:"layer"`
		Scope     string            `yaml:"scope,omitempty"`
		Owner     string            `yaml:"owner,omitempty"`
		TenantID  string            `yaml:"tenant_id,omitempty"`
		Agent     string            `yaml:"agent,omitempty"`
		CreatedAt time.Time         `yaml:"created_at"`
		UpdatedAt time.Time         `yaml:"updated_at"`
		TTL       string            `yaml:"ttl,omitempty"`
		Priority  int               `yaml:"priority"`
		Version   int               `yaml:"version"`
		Metadata  map[string]string `yaml:"metadata,omitempty"`
	}

	if err := yaml.Unmarshal([]byte(parts[0]), &fm); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	// Parse TTL duration
	var ttl time.Duration
	if fm.TTL != "" {
		ttl, _ = time.ParseDuration(fm.TTL)
	}

	body := strings.TrimSpace(parts[1])
	// Remove leading blank lines
	body = strings.TrimLeft(body, "\n")

	return &MemoryRecord{
		ID:        fm.ID,
		Type:      fm.Type,
		Layer:     fm.Layer,
		Scope:     fm.Scope,
		Owner:     fm.Owner,
		TenantID:  fm.TenantID,
		Agent:     fm.Agent,
		Content:   body,
		Metadata:  fm.Metadata,
		CreatedAt: fm.CreatedAt,
		UpdatedAt: fm.UpdatedAt,
		TTL:       ttl,
		Priority:  fm.Priority,
		Version:   fm.Version,
	}, nil
}

// searchFallback performs a linear scan when the index is unavailable.
func (fs *FileStore) searchFallback(_ context.Context, query string, opts SearchOptions) ([]MemoryRecord, error) {
	entries, err := os.ReadDir(fs.dir)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []MemoryRecord

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		record, err := fs.parseFile(filepath.Join(fs.dir, entry.Name()))
		if err != nil {
			continue
		}

		if query == "" || strings.Contains(strings.ToLower(record.Content), queryLower) {
			// Apply type filter
			if len(opts.Types) > 0 {
				typeMatch := false
				for _, t := range opts.Types {
					if record.Type == t {
						typeMatch = true
						break
					}
				}
				if !typeMatch {
					continue
				}
			}
			// Apply owner filter (A6): a non-empty filter limits results to
			// the given owner plus system/global records (empty owner).
			if opts.OwnerFilter != "" && record.Owner != opts.OwnerFilter && record.Owner != "" {
				continue
			}
			if opts.TenantFilter != "" && record.TenantID != opts.TenantFilter && record.TenantID != "" {
				continue
			}
			// Apply agent filter (A7): a non-empty filter limits results to
			// the given agent plus system/global records (empty agent).
			if opts.AgentFilter != "" {
				ra := agentOf(record)
				if ra != opts.AgentFilter && ra != "" {
					continue
				}
			}
			results = append(results, *record)
		}
	}

	// Sort by priority
	sort.Slice(results, func(i, j int) bool {
		return results[i].Priority > results[j].Priority
	})

	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

// SQLiteIndex provides full-text search indexing using SQLite FTS5.
type SQLiteIndex struct {
	mu     sync.RWMutex
	db     *sql.DB
	logger zerolog.Logger
}

// IndexEntry represents a search index entry.
type IndexEntry struct {
	ID      string  `json:"id"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// NewSQLiteIndex creates a new SQLite-backed search index.
func NewSQLiteIndex(dbPath string, logger zerolog.Logger) (*SQLiteIndex, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening index database: %w", err)
	}

	idx := &SQLiteIndex{
		db:     db,
		logger: logger,
	}

	if err := idx.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing index schema: %w", err)
	}

	return idx, nil
}

// initSchema creates the FTS5 virtual table and the metadata table. The
// metadata table carries an owner column (A6 — user scoping); databases
// created before the column existed are migrated with ALTER TABLE, which
// fails harmlessly when the column already exists.
func (idx *SQLiteIndex) initSchema() error {
	schema := `
		CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
			id UNINDEXED,
			content,
			type UNINDEXED,
			layer UNINDEXED,
			scope UNINDEXED,
			tokenize='porter unicode61'
		);
		CREATE TABLE IF NOT EXISTS memory_meta (
			id TEXT PRIMARY KEY,
			type TEXT,
			layer TEXT,
			scope TEXT,
			owner TEXT,
			tenant_id TEXT,
			priority INTEGER,
			created_at TEXT,
			ttl TEXT,
			agent TEXT
		);
	`
	if _, err := idx.db.Exec(schema); err != nil {
		return err
	}
	// Migration for pre-owner databases. Duplicate column errors are ignored.
	_, _ = idx.db.Exec(`ALTER TABLE memory_meta ADD COLUMN owner TEXT`)
	_, _ = idx.db.Exec(`ALTER TABLE memory_meta ADD COLUMN tenant_id TEXT`)
	// Migration for pre-agent databases. Duplicate column errors are ignored.
	_, _ = idx.db.Exec(`ALTER TABLE memory_meta ADD COLUMN agent TEXT`)
	return nil
}

// AddRecord adds a record to the search index.
func (idx *SQLiteIndex) AddRecord(ctx context.Context, record MemoryRecord) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer safe.Rollback(tx)

	// Upsert into FTS
	_, err = tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO memory_fts (id, content, type, layer, scope) VALUES (?, ?, ?, ?, ?)`,
		record.ID, record.Content, string(record.Type), string(record.Layer), record.Scope,
	)
	if err != nil {
		return err
	}

	// Upsert into metadata
	_, err = tx.ExecContext(ctx,
		`INSERT INTO memory_meta (id, type, layer, scope, owner, tenant_id, agent, priority, created_at, ttl) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET type=excluded.type, layer=excluded.layer, scope=excluded.scope, owner=excluded.owner, tenant_id=excluded.tenant_id, agent=excluded.agent, priority=excluded.priority, created_at=excluded.created_at, ttl=excluded.ttl`,
		record.ID, string(record.Type), string(record.Layer), record.Scope, record.Owner, record.TenantID, agentOf(&record), record.Priority,
		record.CreatedAt.Format(time.RFC3339), record.TTL.String(),
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// agentOf returns the record's agent, preferring the dedicated field then
// the metadata map (frontmatter metadata.agent).
func agentOf(r *MemoryRecord) string {
	if r.Agent != "" {
		return r.Agent
	}
	if r.Metadata != nil {
		return r.Metadata["agent"]
	}
	return ""
}

// RemoveRecord removes a record from the search index.
func (idx *SQLiteIndex) RemoveRecord(ctx context.Context, id string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer safe.Rollback(tx)

	_, err = tx.ExecContext(ctx, `DELETE FROM memory_fts WHERE id = ?`, id)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM memory_meta WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteLayer removes every index entry belonging to a memory layer, both
// from the FTS table and the metadata table. Used when rebuilding the index
// from the files on disk.
func (idx *SQLiteIndex) DeleteLayer(ctx context.Context, layer MemoryLayer) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer safe.Rollback(tx)

	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_fts WHERE layer = ?`, string(layer)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_meta WHERE layer = ?`, string(layer)); err != nil {
		return err
	}

	return tx.Commit()
}

// Search searches the index using FTS5.
func (idx *SQLiteIndex) Search(ctx context.Context, query string, opts SearchOptions) ([]IndexEntry, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if query == "" {
		// Return recent records when no query
		if opts.OwnerFilter != "" || opts.AgentFilter != "" {
			tenantClause := ""
			args := []interface{}{}
			ownerClause := ""
			if opts.OwnerFilter != "" {
				ownerClause = " AND (meta.owner = ? OR meta.owner = '')"
				args = append(args, opts.OwnerFilter)
			}
			// Agent scoping (A7): a non-empty AgentFilter limits results to
			// the given agent plus system/global records (empty agent).
			agentClause := ""
			if opts.AgentFilter != "" {
				agentClause = " AND (meta.agent = ? OR meta.agent = '')"
				args = append(args, opts.AgentFilter)
			}
			if opts.TenantFilter != "" {
				tenantClause = " AND (meta.tenant_id = ? OR meta.tenant_id = '')"
				args = append(args, opts.TenantFilter)
			}
			rows, err := idx.db.QueryContext(ctx,
				`SELECT mfts.id, mfts.content FROM memory_fts mfts
				 JOIN memory_meta meta ON mfts.id = meta.id
				 WHERE 1=1`+ownerClause+agentClause+tenantClause+`
				 ORDER BY meta.priority DESC, meta.created_at DESC
				 LIMIT ?`,
				append(args, opts.Limit)...,
			)
			if err != nil {
				return nil, err
			}
			defer func() { _ = rows.Close() }()

			var results []IndexEntry
			for rows.Next() {
				var entry IndexEntry
				if err := rows.Scan(&entry.ID, &entry.Content); err != nil {
					return nil, err
				}
				results = append(results, entry)
			}
			return results, nil
		}

		rows, err := idx.db.QueryContext(ctx,
			`SELECT mfts.id, mfts.content FROM memory_fts mfts
			 JOIN memory_meta meta ON mfts.id = meta.id
			 ORDER BY meta.priority DESC, meta.created_at DESC
			 LIMIT ?`,
			opts.Limit,
		)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()

		var results []IndexEntry
		for rows.Next() {
			var entry IndexEntry
			if err := rows.Scan(&entry.ID, &entry.Content); err != nil {
				return nil, err
			}
			results = append(results, entry)
		}
		return results, nil
	}

	// Build query with type/layer filters
	ftsQuery := sanitizeFTS5Query(query)
	if ftsQuery == "" {
		// All punctuation/symbols — nothing meaningful to match.
		return []IndexEntry{}, nil
	}
	var filters []string
	var args []interface{}

	if len(opts.Types) > 0 {
		typeFilters := make([]string, len(opts.Types))
		for i, t := range opts.Types {
			typeFilters[i] = "meta.type = ?"
			args = append(args, string(t))
		}
		filters = append(filters, "("+strings.Join(typeFilters, " OR ")+")")
	}

	if len(opts.Layers) > 0 {
		layerFilters := make([]string, len(opts.Layers))
		for i, l := range opts.Layers {
			layerFilters[i] = "meta.layer = ?"
			args = append(args, string(l))
		}
		filters = append(filters, "("+strings.Join(layerFilters, " OR ")+")")
	}

	// Apply owner filter on the MATCH path too (A6 — IDOR): the fts query
	// path above the empty-query branch handles owner scoping, but without
	// this clause a queried search would leak every user's records.
	if opts.OwnerFilter != "" {
		filters = append(filters, "(meta.owner = ? OR meta.owner = '')")
		args = append(args, opts.OwnerFilter)
	}
	if opts.TenantFilter != "" {
		filters = append(filters, "(meta.tenant_id = ? OR meta.tenant_id = '')")
		args = append(args, opts.TenantFilter)
	}
	// Apply agent filter on the MATCH path too (A7 — cross-agent
	// contamination): without this clause a queried search would leak every
	// agent's records.
	if opts.AgentFilter != "" {
		filters = append(filters, "(meta.agent = ? OR meta.agent = '')")
		args = append(args, opts.AgentFilter)
	}

	filterClause := ""
	if len(filters) > 0 {
		filterClause = "AND " + strings.Join(filters, " AND ")
	}

	sqlQuery := fmt.Sprintf(`
		SELECT mfts.id, mfts.content, rank
		FROM memory_fts mfts
		JOIN memory_meta meta ON mfts.id = meta.id
		WHERE memory_fts MATCH ?
		%s
		ORDER BY rank, meta.priority DESC
		LIMIT ?
	`, filterClause)

	queryArgs := append([]interface{}{ftsQuery}, args...)
	queryArgs = append(queryArgs, opts.Limit)

	rows, err := idx.db.QueryContext(ctx, sqlQuery, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []IndexEntry
	for rows.Next() {
		var entry IndexEntry
		if err := rows.Scan(&entry.ID, &entry.Content, &entry.Score); err != nil {
			return nil, err
		}
		results = append(results, entry)
	}

	return results, nil
}

// sanitizeFTS5Query converts free-form user input into a safe FTS5 MATCH
// expression. Raw user text is dangerous in FTS5: its own query syntax makes
// commas, colons, quotes, parentheses and other punctuation raise
// "syntax error" (e.g. a spoken "oi, responda em uma frase: ..." crashed the
// knowledge search with `no such column` and killed the whole answer).
// We tokenize on anything that is not a letter or digit, quote every token,
// and AND them together — the exact behavior of a plain multi-word search,
// with no way to inject FTS5 column syntax or operators.
func sanitizeFTS5Query(query string) string {
	var sb strings.Builder
	first := true
	start := -1
	for i, r := range query {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			if !first {
				sb.WriteByte(' ')
			}
			sb.WriteByte('"')
			sb.WriteString(query[start:i])
			sb.WriteByte('"')
			first = false
			start = -1
		}
	}
	if start >= 0 {
		if !first {
			sb.WriteByte(' ')
		}
		sb.WriteByte('"')
		sb.WriteString(query[start:])
		sb.WriteByte('"')
	}
	return sb.String()
}

// Close closes the database connection.
func (idx *SQLiteIndex) Close() error {
	return idx.db.Close()
}
