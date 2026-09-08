// Package learning implements the Learning Vaults (ADR-044): per-department
// SQLite vaults holding ONLY trigger metadata (id, agent, date, title, level,
// tags, hash16, block_hash, chain_prev) — never full content. Full content
// stays immutable in blocks/{sha256}.md (chain-tracked). The vault is a
// semantic trigger index, not a content repository.
//
// Deterministic, no LLM, no network. Regenerable from zero via Rebuild().
package learning

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// VaultName is the physical vault identifier (also the file name: <vault>.db).
type VaultName string

// agentVault maps every agent to its department vault (ADR-044 §2.1).
// Deterministic — the family's routing table. Unknown agents fall back to "misc".
var agentVault = map[string]VaultName{
	"cosca-kernel": "kernel",

	"cosca-backend":                    "backend",
	"cosca-specialist-backend-api":     "backend",
	"cosca-specialist-backend-service": "backend",
	"cosca-api":                        "backend",

	"cosca-frontend":                    "frontend",
	"cosca-specialist-frontend-component": "frontend",
	"cosca-uiux":                        "frontend",

	"cosca-database":                 "database",
	"cosca-specialist-database-sql":  "database",
	"cosca-migration":                "database",

	"cosca-security":   "security",
	"cosca-compliance": "security",

	"cosca-ai":             "ai",
	"cosca-semantic-memory": "ai",
	"cosca-provider":       "ai",
	"cosca-analytics":      "ai",

	"cosca-devops":        "devops",
	"cosca-infrastructure": "devops",
	"cosca-platform":      "devops",
	"cosca-automation":    "devops",

	"cosca-architecture":           "architecture",
	"cosca-review":                 "architecture",
	"cosca-technical-debt":         "architecture",
	"cosca-critic":                 "architecture",
	"cosca-specialist-review-code": "architecture",

	"cosca-qa":                          "qa",
	"cosca-testing":                     "qa",
	"cosca-specialist-testing-e2e":      "qa",
	"cosca-specialist-testing-integration": "qa",
	"cosca-specialist-testing-unit":     "qa",

	"cosca-product":    "product",
	"cosca-governance": "product",

	"cosca-runtime":      "runtime",
	"cosca-monitoring":   "runtime",
	"cosca-performance":  "runtime",
	"cosca-context":      "runtime",
	"cosca-memory-chief": "runtime",
	"cosca-bootstrap":    "runtime",

	"cosca-cli":    "cli",
	"cosca-sdk":    "cli",
	"cosca-plugin": "cli",

	"cosca-workflow-chief": "workflow",
	"cosca-messaging":      "workflow",
	"cosca-integrations":   "workflow",

	"cosca-ceo":       "strategy",
	"cosca-cto":       "strategy",
	"cosca-paradigm":  "strategy",
	"cosca-evolution": "strategy",
	"cosca-discovery": "strategy",

	"cosca-documentation":                 "docs",
	"cosca-specialist-documentation-writer": "docs",

	"cosca-mobile":  "mobile",
	"cosca-release": "release",
	"cosca-cache":   "cache",
}

// VaultForAgent resolves the department vault for an agent. Deterministic.
func VaultForAgent(agent string) VaultName {
	if v, ok := agentVault[agent]; ok {
		return v
	}
	return "misc"
}

// AgentVaults returns a copy of the agent → vault routing table.
func AgentVaults() map[string]VaultName {
	out := make(map[string]VaultName, len(agentVault))
	for k, v := range agentVault {
		out[k] = v
	}
	return out
}

// Vaults returns the sorted list of all known vault names.
func Vaults() []VaultName {
	set := make(map[VaultName]bool)
	for _, v := range agentVault {
		set[v] = true
	}
	out := make([]VaultName, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Trigger is one row of a vault — the trigger metadata of a learning block.
type Trigger struct {
	ID        string // L<id> (ex: L435) or legacy date-id
	Agent     string
	Vault     VaultName
	Date      string
	Title     string
	Level     int
	Tags      string
	Hash16    string
	BlockHash string
	ChainPrev string
}

// vaultSchema is the enxuto schema (ADR-044 §2.2) — metadata only, never content.
// PK is block_hash: sha256 of the full block content — GLOBALLY unique and
// content-addressed. The id field (L# or legacy date) is per-agent and may
// repeat (legacy chains used dates as ids); block_hash never repeats.
const vaultSchema = `
CREATE TABLE IF NOT EXISTS triggers (
	block_hash TEXT PRIMARY KEY,
	id         TEXT NOT NULL,
	agent      TEXT NOT NULL,
	vault      TEXT NOT NULL,
	date       TEXT NOT NULL DEFAULT '',
	title      TEXT NOT NULL,
	level      INTEGER NOT NULL DEFAULT 0,
	tags       TEXT NOT NULL DEFAULT '',
	hash16     TEXT NOT NULL DEFAULT '',
	chain_prev TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_triggers_agent ON triggers(agent);
CREATE INDEX IF NOT EXISTS idx_triggers_tags  ON triggers(tags);
CREATE INDEX IF NOT EXISTS idx_triggers_vault ON triggers(vault);
CREATE INDEX IF NOT EXISTS idx_triggers_id    ON triggers(id);
CREATE VIRTUAL TABLE IF NOT EXISTS triggers_fts USING fts5(
	id, title, tags, agent, content='triggers', content_rowid='rowid'
);
CREATE TRIGGER IF NOT EXISTS triggers_ai AFTER INSERT ON triggers BEGIN
	INSERT INTO triggers_fts(rowid, id, title, tags, agent)
	VALUES (new.rowid, new.id, new.title, new.tags, new.agent);
END;
CREATE TRIGGER IF NOT EXISTS triggers_ad AFTER DELETE ON triggers BEGIN
	INSERT INTO triggers_fts(triggers_fts, rowid, id, title, tags, agent)
	VALUES ('delete', old.rowid, old.id, old.title, old.tags, old.agent);
END;
`

// OpenVault opens (creating if needed) the vault DB for a department.
func OpenVault(dataDir string, vault VaultName) (*sql.DB, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("learning: mkdir vault dir: %w", err)
	}
	path := filepath.Join(dataDir, string(vault)+".db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("learning: open vault %s: %w", vault, err)
	}
	if _, err := db.Exec(vaultSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("learning: schema vault %s: %w", vault, err)
	}
	return db, nil
}

// UpsertTrigger inserts or replaces a trigger row in the vault
// (idempotent by block_hash — globally unique, content-addressed).
func UpsertTrigger(db *sql.DB, t Trigger) error {
	_, err := db.Exec(`
		INSERT INTO triggers (block_hash, id, agent, vault, date, title, level, tags, hash16, chain_prev)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(block_hash) DO UPDATE SET
			id=excluded.id, agent=excluded.agent, vault=excluded.vault, date=excluded.date,
			title=excluded.title, level=excluded.level, tags=excluded.tags,
			hash16=excluded.hash16, chain_prev=excluded.chain_prev`,
		t.BlockHash, t.ID, t.Agent, string(t.Vault), t.Date, t.Title, t.Level, t.Tags,
		t.Hash16, t.ChainPrev)
	if err != nil {
		return fmt.Errorf("learning: upsert trigger %s: %w", t.BlockHash[:min(16, len(t.BlockHash))], err)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Count returns the number of triggers in the vault.
func Count(db *sql.DB) (int, error) {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM triggers`).Scan(&n); err != nil {
		return 0, fmt.Errorf("learning: count: %w", err)
	}
	return n, nil
}

// Search runs an FTS5 search over trigger titles/tags/agent in the vault.
func Search(db *sql.DB, query string, limit int) ([]Trigger, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.Query(`
		SELECT t.id, t.agent, t.vault, t.date, t.title, t.level, t.tags, t.hash16, t.block_hash, t.chain_prev
		FROM triggers_fts f JOIN triggers t ON t.rowid = f.rowid
		WHERE triggers_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("learning: search: %w", err)
	}
	defer rows.Close()

	var out []Trigger
	for rows.Next() {
		var t Trigger
		var v string
		if err := rows.Scan(&t.ID, &t.Agent, &v, &t.Date, &t.Title, &t.Level,
			&t.Tags, &t.Hash16, &t.BlockHash, &t.ChainPrev); err != nil {
			return nil, fmt.Errorf("learning: scan: %w", err)
		}
		t.Vault = VaultName(v)
		out = append(out, t)
	}
	return out, rows.Err()
}

// AllTriggers returns every trigger in the vault (sorted by id).
func AllTriggers(db *sql.DB) ([]Trigger, error) {
	rows, err := db.Query(`
		SELECT id, agent, vault, date, title, level, tags, hash16, block_hash, chain_prev
		FROM triggers ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("learning: all: %w", err)
	}
	defer rows.Close()

	var out []Trigger
	for rows.Next() {
		var t Trigger
		var v string
		if err := rows.Scan(&t.ID, &t.Agent, &v, &t.Date, &t.Title, &t.Level,
			&t.Tags, &t.Hash16, &t.BlockHash, &t.ChainPrev); err != nil {
			return nil, fmt.Errorf("learning: scan: %w", err)
		}
		t.Vault = VaultName(v)
		out = append(out, t)
	}
	return out, rows.Err()
}

// SanitizeFTS escapes a user query into a safe FTS5 MATCH expression.
// Falls back to a simple title LIKE when the query is not FTS-safe.
func SanitizeFTS(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return `""`
	}
	// Remove FTS operators that could error; keep words and quoted phrases.
	var b strings.Builder
	for _, r := range q {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == ' ', r == '-', r == '_', r == '"', r == 'ç', r == 'ã', r == 'á',
			r == 'é', r == 'í', r == 'ó', r == 'ú', r == 'ê', r == 'ô', r == 'â':
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	return b.String()
}