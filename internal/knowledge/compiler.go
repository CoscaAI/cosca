// Package knowledge provides the Knowledge Compiler that reads from the
// Knowledge Repository (.cosca/framework/knowledge/) and compiles structured
// knowledge into SQLite tables with FTS5 full-text search indexing.
package knowledge

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// ── Types ────────────────────────────────────────────────────────────────────

// KnowledgeCategory represents a top-level knowledge domain in the repository.
type KnowledgeCategory string

const (
	CategoryPatterns      KnowledgeCategory = "patterns"
	CategoryHeuristics    KnowledgeCategory = "heuristics"
	CategoryArchitecture  KnowledgeCategory = "architecture"
	CategoryFailures      KnowledgeCategory = "failures"
	CategoryBestPractices KnowledgeCategory = "best-practices"
	CategoryCognitive     KnowledgeCategory = "cognitive"
	CategoryAcquired      KnowledgeCategory = "acquired"
)

// AllCategories returns the list of all known knowledge categories.
func AllCategories() []KnowledgeCategory {
	return []KnowledgeCategory{
		CategoryPatterns,
		CategoryHeuristics,
		CategoryArchitecture,
		CategoryFailures,
		CategoryBestPractices,
		CategoryCognitive,
		CategoryAcquired,
	}
}

// KnowledgeEntry represents a single compiled knowledge unit.
type KnowledgeEntry struct {
	Category    KnowledgeCategory `json:"category"`
	SubCategory string            `json:"sub_category,omitempty"`
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Tags        []string          `json:"tags,omitempty"`
	Confidence  float64           `json:"confidence"`
	Source      string            `json:"source"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// CompileResult summarizes the outcome of a compile operation.
type CompileResult struct {
	Total      int      `json:"total"`
	New        int      `json:"new"`
	Updated    int      `json:"updated"`
	Skipped    int      `json:"skipped"`
	Categories []string `json:"categories,omitempty"`
	Errors     []string `json:"errors,omitempty"`
	Duration   int64    `json:"duration_ms"`
}

// Compiler reads knowledge files from the repository and indexes them in SQLite.
type Compiler struct {
	db       *sql.DB
	repoPath string
}

// NewCompiler creates a new Compiler.
// db must be an open *sql.DB connection to the target SQLite database.
// repoPath is the root path of the knowledge repository (e.g. .cosca/framework/knowledge/).
func NewCompiler(db *sql.DB, repoPath string) *Compiler {
	return &Compiler{
		db:       db,
		repoPath: repoPath,
	}
}

// ── Schema ───────────────────────────────────────────────────────────────────

// schemaDDL returns the DDL statements to create the compiler tables.
func schemaDDL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS knowledge_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category TEXT NOT NULL,
			sub_category TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT NOT NULL DEFAULT '[]',
			confidence REAL NOT NULL DEFAULT 0.5,
			source TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			UNIQUE(source)
		)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_fts USING fts5(
			title, content, tags, category, sub_category,
			tokenize='porter unicode61'
		)`,

		// Auto-sync triggers: keep knowledge_fts in sync with knowledge_entries.
		// Uses DELETE FROM (not INSERT ... VALUES('delete', ...)) because
		// knowledge_fts is a standalone FTS5 table without content= option.
		`CREATE TRIGGER IF NOT EXISTS knowledge_entries_ai AFTER INSERT ON knowledge_entries BEGIN
			INSERT INTO knowledge_fts(rowid, title, content, tags, category, sub_category)
			VALUES (new.id, new.title, new.content, new.tags, new.category, new.sub_category);
		END`,

		`CREATE TRIGGER IF NOT EXISTS knowledge_entries_ad AFTER DELETE ON knowledge_entries BEGIN
			DELETE FROM knowledge_fts WHERE rowid = old.id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS knowledge_entries_au AFTER UPDATE ON knowledge_entries BEGIN
			DELETE FROM knowledge_fts WHERE rowid = old.id;
			INSERT INTO knowledge_fts(rowid, title, content, tags, category, sub_category)
			VALUES (new.id, new.title, new.content, new.tags, new.category, new.sub_category);
		END`,
	}
}

// initSchema creates the compiler tables if they do not exist.
func (c *Compiler) initSchema() error {
	for _, stmt := range schemaDDL() {
		if _, err := c.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
}

// ── Compile ──────────────────────────────────────────────────────────────────

// Compile compiles ALL knowledge categories from the repository.
func (c *Compiler) Compile(ctx context.Context) (*CompileResult, error) {
	start := time.Now()

	if err := c.initSchema(); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}

	result := &CompileResult{}

	for _, cat := range AllCategories() {
		catResult, err := c.CompileCategory(ctx, cat)
		if err != nil {
			// Propagate context cancellation as fatal
			if ctx.Err() != nil {
				return result, fmt.Errorf("compile cancelled: %w", ctx.Err())
			}
			result.Errors = append(result.Errors, fmt.Sprintf("category %s: %v", cat, err))
			continue
		}
		result.Total += catResult.Total
		result.New += catResult.New
		result.Updated += catResult.Updated
		result.Skipped += catResult.Skipped
		result.Errors = append(result.Errors, catResult.Errors...)
		result.Categories = append(result.Categories, string(cat))
	}

	result.Duration = time.Since(start).Milliseconds()

	return result, nil
}

// CompileCategory compiles a single knowledge category directory.
func (c *Compiler) CompileCategory(ctx context.Context, cat KnowledgeCategory) (*CompileResult, error) {
	if err := c.initSchema(); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}

	catDir := filepath.Join(c.repoPath, string(cat))

	result := &CompileResult{
		Categories: []string{string(cat)},
	}

	// Check context cancellation before any I/O
	if ctx.Err() != nil {
		return result, ctx.Err()
	}

	// Check if the category directory exists
	info, err := os.Stat(catDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug().Str("category", string(cat)).Msg("category directory not found, skipping")
			return result, nil
		}
		return nil, fmt.Errorf("stat category dir %s: %w", catDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("category path is not a directory: %s", catDir)
	}

	// Walk the category directory
	err = filepath.WalkDir(catDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories and non-content files
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".md" && ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Read file content
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("read %s: %v", path, readErr))
			return nil // Continue processing other files
		}

		// Compute content hash for change detection
		hash := computeContentHash(string(content))

		// Determine relative source path from repo root. Normalize to forward
		// slashes: `source` is a stable cross-platform identifier (UNIQUE
		// constraint, content-hash lookups). On Windows filepath.Rel yields
		// backslashes, which would make the same file have different source
		// values depending on the OS; filepath.ToSlash is a no-op on Linux.
		relSource, relErr := filepath.Rel(c.repoPath, path)
		if relErr != nil {
			relSource = path
		} else {
			relSource = filepath.ToSlash(relSource)
		}

		// Check if entry exists with same hash
		var existingHash string
		rowErr := c.db.QueryRow(
			"SELECT content_hash FROM knowledge_entries WHERE source = ?",
			relSource,
		).Scan(&existingHash)

		if rowErr == nil && existingHash == hash {
			// Content unchanged — skip
			result.Skipped++
			result.Total++
			log.Debug().Str("source", relSource).Msg("content unchanged, skipping")
			return nil
		}

		// Parse the entry
		entry, parseErr := parseFile(path, relSource, cat, content)
		if parseErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("parse %s: %v", path, parseErr))
			return nil // Continue processing other files
		}
		entry.UpdatedAt = time.Now()

		// Update or insert
		if rowErr == nil {
			// Entry exists but hash changed — update
			if updateErr := c.updateEntry(entry, hash); updateErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("update %s: %v", path, updateErr))
				return nil
			}
			result.Updated++
			log.Debug().Str("source", relSource).Msg("entry updated")
		} else {
			// New entry
			if insertErr := c.insertEntry(entry, hash); insertErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("insert %s: %v", path, insertErr))
				return nil
			}
			result.New++
			log.Debug().Str("source", relSource).Msg("new entry inserted")
		}

		result.Total++
		return nil
	})

	if err != nil {
		return result, fmt.Errorf("walk category dir: %w", err)
	}

	return result, nil
}

// ── Parsing ──────────────────────────────────────────────────────────────────

// parseFile parses a knowledge file based on its extension.
func parseFile(fullPath, relSource string, cat KnowledgeCategory, content []byte) (*KnowledgeEntry, error) {
	entry := &KnowledgeEntry{
		Category:   cat,
		Source:     relSource,
		Confidence: 0.5, // default
	}

	ext := strings.ToLower(filepath.Ext(fullPath))

	switch ext {
	case ".yaml", ".yml":
		return parseYAMLEntry(entry, content)
	case ".md":
		return parseMarkdownEntry(entry, content)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}
}

// parseYAMLEntry parses a YAML knowledge file.
func parseYAMLEntry(entry *KnowledgeEntry, content []byte) (*KnowledgeEntry, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	// Extract title
	if t, ok := raw["title"].(string); ok && t != "" {
		entry.Title = t
	} else if id, ok := raw["id"].(string); ok {
		entry.Title = id
	}

	// Extract description — use as content body if present
	if desc, ok := raw["description"].(string); ok {
		entry.Content = desc
	}

	// Extract tags
	entry.Tags = extractTags(raw)

	// Extract confidence
	if conf, ok := raw["confidence"].(float64); ok {
		entry.Confidence = conf
	}

	// If content is empty, serialize the entire YAML as content
	if entry.Content == "" {
		entry.Content = string(content)
	}

	// Extract sub_category from type/domain fields
	if domain, ok := raw["domain"].(string); ok {
		entry.SubCategory = domain
	}
	if typ, ok := raw["type"].(string); ok && entry.SubCategory == "" {
		entry.SubCategory = typ
	}

	return entry, nil
}

// parseMarkdownEntry parses a Markdown knowledge file with optional YAML frontmatter.
func parseMarkdownEntry(entry *KnowledgeEntry, content []byte) (*KnowledgeEntry, error) {
	text := string(content)
	body := text

	// Try to extract YAML frontmatter between --- delimiters
	if strings.HasPrefix(text, "---\n") {
		endIdx := strings.Index(text[4:], "\n---\n")
		if endIdx > 0 {
			frontmatterStr := text[4 : 4+endIdx]
			body = text[4+endIdx+5:] // Skip the closing ---\n

			var fm map[string]interface{}
			if err := yaml.Unmarshal([]byte(frontmatterStr), &fm); err == nil {
				// Extract title from frontmatter
				if t, ok := fm["title"].(string); ok && t != "" {
					entry.Title = t
				}

				// Extract tags
				entry.Tags = extractTags(fm)

				// Extract confidence
				if conf, ok := fm["confidence"].(float64); ok {
					entry.Confidence = conf
				}

				// Extract sub_category
				if cat, ok := fm["category"].(string); ok {
					entry.SubCategory = cat
				}
				if typ, ok := fm["type"].(string); ok && entry.SubCategory == "" {
					entry.SubCategory = typ
				}
			}
		}
	}

	// If title not found in frontmatter, extract from first # heading
	if entry.Title == "" {
		entry.Title = extractHeadingTitle(body)
	}

	// If still no title, use filename
	if entry.Title == "" {
		entry.Title = strings.TrimSuffix(filepath.Base(entry.Source), filepath.Ext(entry.Source))
	}

	// Set content
	entry.Content = body

	return entry, nil
}

// ── Database operations ──────────────────────────────────────────────────────

// insertEntry inserts a new knowledge entry and its FTS index.
func (c *Compiler) insertEntry(entry *KnowledgeEntry, hash string) error {
	tagsJSON, err := json.Marshal(entry.Tags)
	if err != nil {
		tagsJSON = []byte("[]")
	}

	_, err = c.db.Exec(
		`INSERT INTO knowledge_entries (category, sub_category, title, content, tags, confidence, source, updated_at, content_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(entry.Category),
		entry.SubCategory,
		entry.Title,
		entry.Content,
		string(tagsJSON),
		entry.Confidence,
		entry.Source,
		entry.UpdatedAt.Format(time.RFC3339),
		hash,
	)
	if err != nil {
		return fmt.Errorf("insert entry: %w", err)
	}

	// FTS sync is handled by the knowledge_entries_ai trigger
	return nil
}

// updateEntry updates an existing knowledge entry when content has changed.
func (c *Compiler) updateEntry(entry *KnowledgeEntry, hash string) error {
	tagsJSON, err := json.Marshal(entry.Tags)
	if err != nil {
		tagsJSON = []byte("[]")
	}

	_, err = c.db.Exec(
		`UPDATE knowledge_entries SET
			title = ?, content = ?, tags = ?, confidence = ?,
			sub_category = ?, updated_at = ?, content_hash = ?
		 WHERE source = ?`,
		entry.Title,
		entry.Content,
		string(tagsJSON),
		entry.Confidence,
		entry.SubCategory,
		entry.UpdatedAt.Format(time.RFC3339),
		hash,
		entry.Source,
	)
	if err != nil {
		return fmt.Errorf("update entry: %w", err)
	}

	// FTS sync is handled by the knowledge_entries_au trigger
	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// computeContentHash returns a SHA-256 hex string for the given content.
func computeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:])
}

// extractHeadingTitle extracts the title from the first Markdown heading (# Title).
func extractHeadingTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
		if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "##") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		}
	}
	return ""
}

// extractTags extracts a slice of strings from a YAML map.
// It handles tags as []interface{} (YAML list) and []string.
func extractTags(raw map[string]interface{}) []string {
	tagsRaw, ok := raw["tags"]
	if !ok {
		return nil
	}

	switch v := tagsRaw.(type) {
	case []interface{}:
		tags := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				tags = append(tags, s)
			}
		}
		return tags
	case []string:
		return v
	case string:
		return strings.Split(v, ",")
	}

	return nil
}

// Ensure compile-time interface satisfaction.
var _ interface {
	Compile(ctx context.Context) (*CompileResult, error)
	CompileCategory(ctx context.Context, cat KnowledgeCategory) (*CompileResult, error)
} = (*Compiler)(nil)
