-- ============================================================================
-- V001: Initial Knowledge Base Schema
-- ============================================================================
-- Cria as tabelas fundamentais para armazenamento de conhecimento no cosca.db:
--   - heuristics:   Regras heurísticas extraídas de heuristics/*.yaml
--   - patterns:     Padrões e anti-padrões de memory/pattern/*.md e knowledge/patterns/*.md
--   - playbooks:    Guias passo-a-passo de knowledge/playbooks/*.md
--   - heuristics_fts / patterns_fts: Índices FTS5 para busca textual
-- ============================================================================

-- ── Heurísticas ────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS heuristics (
    id            TEXT PRIMARY KEY,        -- ex: H-001
    title         TEXT NOT NULL,
    domain        TEXT NOT NULL,           -- testing, security, orchestration, etc.
    type          TEXT NOT NULL,           -- pattern, anti_pattern, rule
    severity      TEXT NOT NULL,           -- critical, high, medium, low
    description   TEXT NOT NULL,
    evidence      TEXT,                    -- JSON array of evidence strings
    action        TEXT NOT NULL,
    source_agents TEXT,                    -- JSON array de agentes fonte
    related       TEXT,                    -- JSON array de IDs relacionados
    confidence    REAL    DEFAULT 0.5,
    tags          TEXT,                    -- JSON array de tags
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ── Padrões ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS patterns (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL UNIQUE,
    domain      TEXT    NOT NULL,           -- architecture, design, code, testing, etc.
    description TEXT    NOT NULL,
    content     TEXT    NOT NULL,           -- Full markdown content
    source_file TEXT    NOT NULL,           -- Path relativo ao projeto
    tags        TEXT,                       -- JSON array de tags
    created_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ── Playbooks ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS playbooks (
    id          TEXT PRIMARY KEY,           -- ex: playbook-incident-response
    title       TEXT NOT NULL,
    type        TEXT NOT NULL,              -- incident_response, deployment, maintenance
    severity    TEXT NOT NULL,
    owner       TEXT NOT NULL,              -- Agente responsável
    content     TEXT NOT NULL,              -- Full markdown content
    source_file TEXT NOT NULL,
    tags        TEXT,                       -- JSON array de tags
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ── Índices de performance ─────────────────────────────────────────────────

CREATE INDEX IF NOT EXISTS idx_heuristics_domain  ON heuristics(domain);
CREATE INDEX IF NOT EXISTS idx_heuristics_type    ON heuristics(type);
CREATE INDEX IF NOT EXISTS idx_heuristics_severity ON heuristics(severity);

CREATE INDEX IF NOT EXISTS idx_patterns_domain ON patterns(domain);
CREATE INDEX IF NOT EXISTS idx_patterns_name   ON patterns(name);

CREATE INDEX IF NOT EXISTS idx_playbooks_type     ON playbooks(type);
CREATE INDEX IF NOT EXISTS idx_playbooks_severity ON playbooks(severity);
CREATE INDEX IF NOT EXISTS idx_playbooks_owner    ON playbooks(owner);

-- ── FTS5 — Full-Text Search ────────────────────────────────────────────────
-- Self-managed FTS5 (triggers-only, sem content= mode).
-- Consistente com as correções aplicadas nas migrations V2 e V3 do
-- internal/sqlite (fix_documents_fts, fix_entities_fts).
-- O modo content= foi removido porque as triggers já gerenciam a sincronização,
-- e content= com tabelas que usam TEXT PRIMARY KEY pode causar inconsistências.

CREATE VIRTUAL TABLE IF NOT EXISTS heuristics_fts USING fts5(
    title,
    description,
    domain,
    tags,
    tokenize='porter unicode61'
);

CREATE VIRTUAL TABLE IF NOT EXISTS patterns_fts USING fts5(
    name,
    description,
    content,
    domain,
    tags,
    tokenize='porter unicode61'
);

-- ── FTS5 Triggers: heuristics ──────────────────────────────────────────────

CREATE TRIGGER IF NOT EXISTS heuristics_ai AFTER INSERT ON heuristics BEGIN
    INSERT INTO heuristics_fts(rowid, title, description, domain, tags)
    VALUES (new.rowid, new.title, new.description, new.domain, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS heuristics_ad AFTER DELETE ON heuristics BEGIN
    INSERT INTO heuristics_fts(heuristics_fts, rowid, title, description, domain, tags)
    VALUES ('delete', old.rowid, old.title, old.description, old.domain, old.tags);
END;

CREATE TRIGGER IF NOT EXISTS heuristics_au AFTER UPDATE ON heuristics BEGIN
    INSERT INTO heuristics_fts(heuristics_fts, rowid, title, description, domain, tags)
    VALUES ('delete', old.rowid, old.title, old.description, old.domain, old.tags);
    INSERT INTO heuristics_fts(rowid, title, description, domain, tags)
    VALUES (new.rowid, new.title, new.description, new.domain, new.tags);
END;

-- ── FTS5 Triggers: patterns ────────────────────────────────────────────────

CREATE TRIGGER IF NOT EXISTS patterns_ai AFTER INSERT ON patterns BEGIN
    INSERT INTO patterns_fts(rowid, name, description, content, domain, tags)
    VALUES (new.rowid, new.name, new.description, new.content, new.domain, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS patterns_ad AFTER DELETE ON patterns BEGIN
    INSERT INTO patterns_fts(patterns_fts, rowid, name, description, content, domain, tags)
    VALUES ('delete', old.rowid, old.name, old.description, old.content, old.domain, old.tags);
END;

CREATE TRIGGER IF NOT EXISTS patterns_au AFTER UPDATE ON patterns BEGIN
    INSERT INTO patterns_fts(patterns_fts, rowid, name, description, content, domain, tags)
    VALUES ('delete', old.rowid, old.name, old.description, old.content, old.domain, old.tags);
    INSERT INTO patterns_fts(rowid, name, description, content, domain, tags)
    VALUES (new.rowid, new.name, new.description, new.content, new.domain, new.tags);
END;
