-- COSCA-MEMORY.DB Schema
-- Memória operacional: sessions, learnings, patterns, failures, decisions, traces
-- Criado: 2026-09-08 | ADR-046

PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- ============================================================
-- SESSÕES DE TRABALHO
-- ============================================================
CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    agent       TEXT NOT NULL,
    started_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    ended_at    DATETIME,
    summary     TEXT,
    status      TEXT DEFAULT 'active',
    metadata    TEXT
);

CREATE INDEX IF NOT EXISTS idx_sessions_agent ON sessions(agent);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_started ON sessions(started_at);

-- ============================================================
-- APRENDIZADOS POR AGENTE
-- ============================================================
CREATE TABLE IF NOT EXISTS learnings (
    id          TEXT PRIMARY KEY,
    agent       TEXT NOT NULL,
    session_id  TEXT REFERENCES sessions(id),
    content     TEXT NOT NULL,
    type        TEXT NOT NULL,
    confidence  REAL DEFAULT 0.5,
    domain      TEXT,
    tags        TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    validated   INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_learnings_agent ON learnings(agent);
CREATE INDEX IF NOT EXISTS idx_learnings_type ON learnings(type);
CREATE INDEX IF NOT EXISTS idx_learnings_domain ON learnings(domain);
CREATE INDEX IF NOT EXISTS idx_learnings_confidence ON learnings(confidence);
CREATE INDEX IF NOT EXISTS idx_learnings_created ON learnings(created_at);

-- ============================================================
-- PADRÕES DETECTADOS
-- ============================================================
CREATE TABLE IF NOT EXISTS patterns (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    pattern_type TEXT DEFAULT 'code',
    frequency   INTEGER DEFAULT 1,
    confidence  REAL DEFAULT 0.5,
    examples    TEXT,
    last_seen   DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_patterns_type ON patterns(pattern_type);
CREATE INDEX IF NOT EXISTS idx_patterns_frequency ON patterns(frequency);
CREATE INDEX IF NOT EXISTS idx_patterns_confidence ON patterns(confidence);

-- ============================================================
-- LIÇÕES DE ERRO
-- ============================================================
CREATE TABLE IF NOT EXISTS failures (
    id          TEXT PRIMARY KEY,
    agent       TEXT NOT NULL,
    session_id  TEXT REFERENCES sessions(id),
    error       TEXT NOT NULL,
    stack_trace TEXT,
    resolution  TEXT,
    severity    TEXT DEFAULT 'medium',
    domain      TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    resolved_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_failures_agent ON failures(agent);
CREATE INDEX IF NOT EXISTS idx_failures_severity ON failures(severity);
CREATE INDEX IF NOT EXISTS idx_failures_domain ON failures(domain);
CREATE INDEX IF NOT EXISTS idx_failures_created ON failures(created_at);

-- ============================================================
-- DECISÕES TOMADAS
-- ============================================================
CREATE TABLE IF NOT EXISTS decisions (
    id          TEXT PRIMARY KEY,
    context     TEXT NOT NULL,
    decision    TEXT NOT NULL,
    rationale   TEXT,
    outcome     TEXT,
    confidence  REAL DEFAULT 0.5,
    agent       TEXT,
    session_id  TEXT REFERENCES sessions(id),
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_decisions_agent ON decisions(agent);
CREATE INDEX IF NOT EXISTS idx_decisions_confidence ON decisions(confidence);
CREATE INDEX IF NOT EXISTS idx_decisions_created ON decisions(created_at);

-- ============================================================
-- TRILHAS DE AUDITORIA (TRACES)
-- ============================================================
CREATE TABLE IF NOT EXISTS traces (
    id          TEXT PRIMARY KEY,
    operation   TEXT NOT NULL,
    agent       TEXT,
    session_id  TEXT REFERENCES sessions(id),
    input       TEXT,
    output      TEXT,
    duration_ms INTEGER,
    status      TEXT DEFAULT 'success',
    error       TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_traces_agent ON traces(agent);
CREATE INDEX IF NOT EXISTS idx_traces_operation ON traces(operation);
CREATE INDEX IF NOT EXISTS idx_traces_status ON traces(status);
CREATE INDEX IF NOT EXISTS idx_traces_created ON traces(created_at);
CREATE INDEX IF NOT EXISTS idx_traces_duration ON traces(duration_ms);

-- ============================================================
-- CONTEXTO DE SESSÃO (cache de contexto)
-- ============================================================
CREATE TABLE IF NOT EXISTS session_context (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    key         TEXT NOT NULL,
    value       TEXT NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_session_context_session ON session_context(session_id);

-- ============================================================
-- SNAPSHOTS (backup de estado)
-- ============================================================
CREATE TABLE IF NOT EXISTS snapshots (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    data        TEXT NOT NULL,
    agent       TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_snapshots_agent ON snapshots(agent);
CREATE INDEX IF NOT EXISTS idx_snapshots_created ON snapshots(created_at);

-- ============================================================
-- POLÍTICAS DE RETENÇÃO
-- ============================================================
CREATE TABLE IF NOT EXISTS retention_policies (
    id              TEXT PRIMARY KEY,
    table_name      TEXT NOT NULL,
    retention_days  INTEGER NOT NULL,
    archive_to      TEXT,
    enabled         INTEGER DEFAULT 1,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Políticas padrão
INSERT OR IGNORE INTO retention_policies (id, table_name, retention_days, archive_to) VALUES
    ('sessions_hot', 'sessions', 7, 'sessions_archive'),
    ('sessions_archive', 'sessions_archive', 90, NULL),
    ('learnings_hot', 'learnings', 30, NULL),
    ('traces_hot', 'traces', 30, 'traces_archive'),
    ('traces_archive', 'traces_archive', 90, NULL),
    ('failures_hot', 'failures', 60, NULL),
    ('decisions_hot', 'decisions', 90, NULL);

-- ============================================================
-- VIEWS
-- ============================================================
CREATE VIEW IF NOT EXISTS v_memory_summary AS
SELECT 
    'sessions' as table_name,
    COUNT(*) as total,
    SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END) as active,
    MIN(started_at) as oldest,
    MAX(started_at) as newest
FROM sessions
UNION ALL
SELECT 
    'learnings',
    COUNT(*),
    SUM(CASE WHEN validated = 1 THEN 1 ELSE 0 END),
    MIN(created_at),
    MAX(created_at)
FROM learnings
UNION ALL
SELECT 
    'failures',
    COUNT(*),
    SUM(CASE WHEN resolved_at IS NULL THEN 1 ELSE 0 END),
    MIN(created_at),
    MAX(created_at)
FROM failures
UNION ALL
SELECT 
    'traces',
    COUNT(*),
    SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END),
    MIN(created_at),
    MAX(created_at)
FROM traces;

CREATE VIEW IF NOT EXISTS v_agent_activity AS
SELECT 
    agent,
    COUNT(DISTINCT session_id) as sessions,
    COUNT(*) as learnings,
    AVG(confidence) as avg_confidence
FROM learnings
GROUP BY agent;

CREATE VIEW IF NOT EXISTS v_error_rate AS
SELECT 
    agent,
    COUNT(*) as total_traces,
    SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as errors,
    ROUND(SUM(CASE WHEN status = 'error' THEN 1.0 ELSE 0 END) / COUNT(*) * 100, 2) as error_rate_pct
FROM traces
GROUP BY agent;
