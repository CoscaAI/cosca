// Package sqlite provides the database schema definitions for the Cosca Knowledge Engine.
// It defines all tables, indexes, FTS5 virtual tables, and vector storage schemas.
package sqlite

import (
	"fmt"
	"strings"
)

// SchemaVersion is the current database schema version.
const SchemaVersion = 1

// Schema contains all DDL statements for creating the Cosca Knowledge Engine database.
type Schema struct {
	version    int
	statements []string
}

// NewSchema creates a Schema with all table definitions.
func NewSchema() *Schema {
	s := &Schema{
		version: SchemaVersion,
	}
	s.statements = s.allStatements()
	return s
}

// Version returns the schema version.
func (s *Schema) Version() int { return s.version }

// Statements returns all DDL statements.
func (s *Schema) Statements() []string { return s.statements }

// DDL returns the complete schema as a single DDL string.
func (s *Schema) DDL() string {
	return strings.Join(s.statements, "\n\n")
}

// allStatements returns all DDL statements in dependency order.
func (s *Schema) allStatements() []string {
	var stmts []string
	stmts = make([]string, 0, 52)

	// ── Core content tables ──────────────────────────────────────────────

	// documents: Represents indexed files (Markdown, code, configs, etc.)
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS documents (
		id            TEXT PRIMARY KEY,
		path          TEXT NOT NULL UNIQUE,
		hash          TEXT NOT NULL,
		title         TEXT NOT NULL DEFAULT '',
		doc_type      TEXT NOT NULL DEFAULT 'markdown',
		metadata_json TEXT NOT NULL DEFAULT '{}',
		frontmatter_json TEXT NOT NULL DEFAULT '{}',
		size          INTEGER NOT NULL DEFAULT 0,
		token_count   INTEGER NOT NULL DEFAULT 0,
		created_at    TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
	)`)

	// chunks: Document chunks for semantic search and context building.
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS chunks (
		id            TEXT PRIMARY KEY,
		document_id   TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
		content       TEXT NOT NULL,
		heading       TEXT NOT NULL DEFAULT '',
		section_type  TEXT NOT NULL DEFAULT 'text',
		position      INTEGER NOT NULL DEFAULT 0,
		hash          TEXT NOT NULL DEFAULT '',
		token_count   INTEGER NOT NULL DEFAULT 0,
		metadata_json TEXT NOT NULL DEFAULT '{}',
		embedding     BLOB
	)`)

	// headings: Extracted headings from documents.
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS headings (
		id          TEXT PRIMARY KEY,
		document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
		level       INTEGER NOT NULL DEFAULT 1,
		text        TEXT NOT NULL,
		position    INTEGER NOT NULL DEFAULT 0
	)`)

	// code_blocks: Extracted code blocks from documents.
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS code_blocks (
		id          TEXT PRIMARY KEY,
		document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
		language    TEXT NOT NULL DEFAULT '',
		content     TEXT NOT NULL,
		position    INTEGER NOT NULL DEFAULT 0,
		token_count INTEGER NOT NULL DEFAULT 0
	)`)

	// tables: Extracted tables from documents.
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS tables (
		id          TEXT PRIMARY KEY,
		document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
		caption     TEXT NOT NULL DEFAULT '',
		headers_json TEXT NOT NULL DEFAULT '[]',
		rows_json   TEXT NOT NULL DEFAULT '[]',
		position    INTEGER NOT NULL DEFAULT 0
	)`)

	// symbols: Extracted code symbols (functions, types, variables, etc.).
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS symbols (
		id          TEXT PRIMARY KEY,
		document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
		name        TEXT NOT NULL,
		kind        TEXT NOT NULL DEFAULT 'unknown',
		signature   TEXT NOT NULL DEFAULT '',
		package_path TEXT NOT NULL DEFAULT '',
		position    INTEGER NOT NULL DEFAULT 0,
		metadata_json TEXT NOT NULL DEFAULT '{}'
	)`)

	// entities: Domain entities — agents, skills, prompts, workflows, templates, etc.
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS entities (
		id            TEXT PRIMARY KEY,
		entity_type   TEXT NOT NULL,
		name          TEXT NOT NULL,
		path          TEXT NOT NULL,
		metadata_json TEXT NOT NULL DEFAULT '{}',
		embedding     BLOB,
		created_at    TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
	)`)

	// relationships: Relationships between entities, documents, and chunks.
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS relationships (
		id          TEXT PRIMARY KEY,
		source_id   TEXT NOT NULL,
		source_type TEXT NOT NULL,
		target_id   TEXT NOT NULL,
		target_type TEXT NOT NULL,
		rel_type    TEXT NOT NULL,
		weight      REAL NOT NULL DEFAULT 1.0,
		metadata_json TEXT NOT NULL DEFAULT '{}',
		created_at  TEXT NOT NULL DEFAULT (datetime('now'))
	)`)

	// frontmatter: Key-value pairs extracted from document frontmatter.
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS frontmatter (
		id          TEXT PRIMARY KEY,
		document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
		key         TEXT NOT NULL,
		value       TEXT NOT NULL,
		UNIQUE(document_id, key)
	)`)

	// metadata: Arbitrary key-value metadata for documents.
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS metadata (
		id          TEXT PRIMARY KEY,
		document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
		key         TEXT NOT NULL,
		value       TEXT NOT NULL,
		UNIQUE(document_id, key)
	)`)

	// cache: Persistent cache store for embeddings, search results, etc.
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS cache (
		key         TEXT PRIMARY KEY,
		value       BLOB NOT NULL,
		content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
		ttl_seconds INTEGER NOT NULL DEFAULT 3600,
		created_at  TEXT NOT NULL DEFAULT (datetime('now')),
		expires_at  TEXT NOT NULL DEFAULT (datetime('now', '+1 hour'))
	)`)

	// migration_history: Tracks which migrations have been applied.
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS migration_history (
		version     INTEGER PRIMARY KEY,
		name        TEXT NOT NULL,
		applied_at  TEXT NOT NULL DEFAULT (datetime('now')),
		checksum    TEXT NOT NULL DEFAULT '',
		applied_by  TEXT NOT NULL DEFAULT 'cosca'
	)`)

	// snapshots: Knowledge engine snapshots for export/import.
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS snapshots (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		version     INTEGER NOT NULL DEFAULT 1,
		size_bytes  INTEGER NOT NULL DEFAULT 0,
		entity_count INTEGER NOT NULL DEFAULT 0,
		created_at  TEXT NOT NULL DEFAULT (datetime('now')),
		data_json   TEXT NOT NULL DEFAULT '{}'
	)`)

	// sync_log: Tracks filesystem synchronization.
	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS sync_log (
		id          TEXT PRIMARY KEY,
		path        TEXT NOT NULL,
		action      TEXT NOT NULL,
		status      TEXT NOT NULL DEFAULT 'pending',
		hash_before TEXT NOT NULL DEFAULT '',
		hash_after  TEXT NOT NULL DEFAULT '',
		error_msg   TEXT NOT NULL DEFAULT '',
		timestamp   TEXT NOT NULL DEFAULT (datetime('now'))
	)`)

	// ── Indexes for performance ─────────────────────────────────────────

	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(hash)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_documents_type ON documents(doc_type)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_documents_updated ON documents(updated_at)`)

	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_chunks_document ON chunks(document_id)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_chunks_position ON chunks(document_id, position)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_chunks_section ON chunks(document_id, section_type)`)

	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_headings_document ON headings(document_id, position)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_code_blocks_document ON code_blocks(document_id, position)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_code_blocks_lang ON code_blocks(language)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_tables_document ON tables(document_id, position)`)

	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_symbols_document ON symbols(document_id)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_symbols_kind ON symbols(kind)`)

	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_entities_type ON entities(entity_type)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name)`)

	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_relationships_source ON relationships(source_id, source_type)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_relationships_target ON relationships(target_id, target_type)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_relationships_type ON relationships(rel_type)`)

	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_frontmatter_key ON frontmatter(document_id, key)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_metadata_key ON metadata(document_id, key)`)

	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache(expires_at)`)
	stmts = append(stmts, `CREATE INDEX IF NOT EXISTS idx_sync_status ON sync_log(status)`)

	// ── FTS5 virtual tables for full-text search ────────────────────────

	// documents_fts: Full-text search index on document titles and doc_type.
	// Documents do not have a body/content column — the content column here is
	// always empty and exists only for FTS5 column-count consistency.
	stmts = append(stmts, `CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
		title,
		content,
		doc_type,
		tokenize='porter unicode61'
	)`)

	// chunks_fts: Full-text search index on chunk content.
	stmts = append(stmts, `CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
		content,
		heading,
		section_type,
		tokenize='porter unicode61',
		content='chunks',
		content_rowid='rowid'
	)`)

	// code_blocks_fts: Full-text search index on code block content.
	stmts = append(stmts, `CREATE VIRTUAL TABLE IF NOT EXISTS code_blocks_fts USING fts5(
		content,
		language,
		tokenize='porter unicode61',
		content='code_blocks',
		content_rowid='rowid'
	)`)

	// entities_fts: Full-text search index on entity names and metadata.
	stmts = append(stmts, `CREATE VIRTUAL TABLE IF NOT EXISTS entities_fts USING fts5(
		name,
		entity_type,
		metadata,
		tokenize='porter unicode61',
		content='entities',
		content_rowid='rowid'
	)`)

	// ── FTS5 triggers to keep indexes in sync ───────────────────────────

	// Documents sync triggers
	stmts = append(stmts, `CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN
		INSERT INTO documents_fts(rowid, title, content, doc_type)
		VALUES (new.rowid, new.title, '', new.doc_type);
	END`)

	stmts = append(stmts, `CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
		DELETE FROM documents_fts WHERE rowid = old.rowid;
	END`)

	stmts = append(stmts, `CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN
		DELETE FROM documents_fts WHERE rowid = old.rowid;
		INSERT INTO documents_fts(rowid, title, content, doc_type)
		VALUES (new.rowid, new.title, '', new.doc_type);
	END`)

	// Chunks sync triggers
	stmts = append(stmts, `CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
		INSERT INTO chunks_fts(rowid, content, heading, section_type)
		VALUES (new.rowid, new.content, new.heading, new.section_type);
	END`)

	stmts = append(stmts, `CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
		INSERT INTO chunks_fts(chunks_fts, rowid, content, heading, section_type)
		VALUES ('delete', old.rowid, old.content, old.heading, old.section_type);
	END`)

	stmts = append(stmts, `CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
		INSERT INTO chunks_fts(chunks_fts, rowid, content, heading, section_type)
		VALUES ('delete', old.rowid, old.content, old.heading, old.section_type);
		INSERT INTO chunks_fts(rowid, content, heading, section_type)
		VALUES (new.rowid, new.content, new.heading, new.section_type);
	END`)

	// Code blocks sync triggers
	stmts = append(stmts, `CREATE TRIGGER IF NOT EXISTS code_blocks_ai AFTER INSERT ON code_blocks BEGIN
		INSERT INTO code_blocks_fts(rowid, content, language)
		VALUES (new.rowid, new.content, new.language);
	END`)

	stmts = append(stmts, `CREATE TRIGGER IF NOT EXISTS code_blocks_ad AFTER DELETE ON code_blocks BEGIN
		INSERT INTO code_blocks_fts(code_blocks_fts, rowid, content, language)
		VALUES ('delete', old.rowid, old.content, old.language);
	END`)

	stmts = append(stmts, `CREATE TRIGGER IF NOT EXISTS code_blocks_au AFTER UPDATE ON code_blocks BEGIN
		INSERT INTO code_blocks_fts(code_blocks_fts, rowid, content, language)
		VALUES ('delete', old.rowid, old.content, old.language);
		INSERT INTO code_blocks_fts(rowid, content, language)
		VALUES (new.rowid, new.content, new.language);
	END`)

	// Entities sync triggers
	stmts = append(stmts, `CREATE TRIGGER IF NOT EXISTS entities_ai AFTER INSERT ON entities BEGIN
		INSERT INTO entities_fts(rowid, name, entity_type, metadata)
		VALUES (new.rowid, new.name, new.entity_type, new.metadata_json);
	END`)

	stmts = append(stmts, `CREATE TRIGGER IF NOT EXISTS entities_ad AFTER DELETE ON entities BEGIN
		DELETE FROM entities_fts WHERE rowid = old.rowid;
	END`)

	stmts = append(stmts, `CREATE TRIGGER IF NOT EXISTS entities_au AFTER UPDATE ON entities BEGIN
		DELETE FROM entities_fts WHERE rowid = old.rowid;
		INSERT INTO entities_fts(rowid, name, entity_type, metadata)
		VALUES (new.rowid, new.name, new.entity_type, new.metadata_json);
	END`)

	return stmts
}

// TableNames returns all table names in the schema.
func TableNames() []string {
	return []string{
		"documents", "chunks", "headings", "code_blocks", "tables",
		"symbols", "entities", "relationships", "frontmatter", "metadata",
		"cache", "migration_history", "snapshots", "sync_log",
		"documents_fts", "chunks_fts", "code_blocks_fts", "entities_fts",
	}
}

// EntityTypes returns the supported entity type constants.
func EntityTypes() []string {
	return []string{
		"agent", "skill", "prompt", "workflow", "template",
		"provider", "plugin", "adr", "doc", "symbol",
		"module", "component", "api", "pattern", "playbook",
		"runbook", "incident", "benchmark", "reference_architecture",
		"memory_record", "config", "capability",
	}
}

// RelationshipTypes returns the supported relationship type constants.
func RelationshipTypes() []string {
	return []string{
		"depends_on", "extends", "implements", "invokes",
		"references", "defines", "contains", "related_to",
		"imports", "deploys_to", "triggers", "produces",
		"consumes", "specializes", "supersedes", "documents",
	}
}

// SectionTypes returns the supported chunk section types.
func SectionTypes() []string {
	return []string{
		"text", "code", "table", "list", "heading",
		"frontmatter", "quote", "math", "diagram",
	}
}

// DocumentTypes returns the supported document types.
func DocumentTypes() []string {
	return []string{
		"markdown", "yaml", "json", "toml", "go",
		"typescript", "javascript", "python", "rust", "shell",
		"sql", "dockerfile", "makefile", "config", "unknown",
	}
}

// SchemaHash returns a hash of the schema definition for migration checksumming.
func (s *Schema) SchemaHash() string {
	return fmt.Sprintf("cosca-kg-v%d", s.version)
}
