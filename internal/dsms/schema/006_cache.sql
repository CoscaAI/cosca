-- COSCA-CACHE.DB Schema
-- Cache TTL-based: search, llm, computed (max 100MB)
-- Criado: 2026-09-08 | ADR-046

PRAGMA journal_mode=WAL;

-- ============================================================
-- CACHE DE BUSCA
-- ============================================================
CREATE TABLE IF NOT EXISTS search_cache (
    query_hash  TEXT PRIMARY KEY,
    query       TEXT NOT NULL,
    results     TEXT NOT NULL,  -- JSON results
    result_count INTEGER,
    model       TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at  DATETIME NOT NULL,
    hit_count   INTEGER DEFAULT 0,
    last_hit    DATETIME
);

CREATE INDEX IF NOT EXISTS idx_search_cache_expires ON search_cache(expires_at);
CREATE INDEX IF NOT EXISTS idx_search_cache_hits ON search_cache(hit_count);

-- ============================================================
-- CACHE DE LLM
-- ============================================================
CREATE TABLE IF NOT EXISTS llm_cache (
    prompt_hash TEXT PRIMARY KEY,
    prompt      TEXT NOT NULL,
    response    TEXT NOT NULL,
    model       TEXT,
    tokens_in   INTEGER,
    tokens_out  INTEGER,
    duration_ms INTEGER,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at  DATETIME NOT NULL,
    hit_count   INTEGER DEFAULT 0,
    last_hit    DATETIME
);

CREATE INDEX IF NOT EXISTS idx_llm_cache_expires ON llm_cache(expires_at);
CREATE INDEX IF NOT EXISTS idx_llm_cache_model ON llm_cache(model);
CREATE INDEX IF NOT EXISTS idx_llm_cache_hits ON llm_cache(hit_count);

-- ============================================================
-- CACHE DE CÁLCULOS
-- ============================================================
CREATE TABLE IF NOT EXISTS computed_cache (
    key             TEXT PRIMARY KEY,
    computation     TEXT NOT NULL,  -- Descrição do cálculo
    value           TEXT NOT NULL,  -- JSON result
    computation_time_ms INTEGER,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at      DATETIME NOT NULL,
    hit_count       INTEGER DEFAULT 0,
    last_hit        DATETIME
);

CREATE INDEX IF NOT EXISTS idx_computed_cache_expires ON computed_cache(expires_at);

-- ============================================================
-- CACHE DE EMBEDDINGS
-- ============================================================
CREATE TABLE IF NOT EXISTS embedding_cache (
    text_hash   TEXT PRIMARY KEY,
    text        TEXT NOT NULL,
    embedding   BLOB NOT NULL,
    model       TEXT NOT NULL,
    dimension   INTEGER NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at  DATETIME NOT NULL,
    hit_count   INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_embedding_cache_expires ON embedding_cache(expires_at);
CREATE INDEX IF NOT EXISTS idx_embedding_cache_model ON embedding_cache(model);

-- ============================================================
-- POLÍTICAS DE CACHE
-- ============================================================
CREATE TABLE IF NOT EXISTS cache_policies (
    id              TEXT PRIMARY KEY,
    cache_type      TEXT NOT NULL,
    max_size_mb     INTEGER NOT NULL,
    ttl_hours       INTEGER NOT NULL,
    eviction_policy TEXT DEFAULT 'lru',
    enabled         INTEGER DEFAULT 1,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Políticas padrão (max 100MB total)
INSERT OR IGNORE INTO cache_policies (id, cache_type, max_size_mb, ttl_hours) VALUES
    ('search', 'search_cache', 30, 1),
    ('llm', 'llm_cache', 50, 2),
    ('computed', 'computed_cache', 10, 24),
    ('embedding', 'embedding_cache', 10, 48);

-- ============================================================
-- MÉTRICAS DE CACHE
-- ============================================================
CREATE TABLE IF NOT EXISTS cache_metrics (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    cache_type  TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    metric_value REAL NOT NULL,
    measured_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cache_metrics_type ON cache_metrics(cache_type);
CREATE INDEX IF NOT EXISTS idx_cache_metrics_time ON cache_metrics(measured_at);

-- ============================================================
-- VIEWS
-- ============================================================
CREATE VIEW IF NOT EXISTS v_cache_summary AS
SELECT 
    'search_cache' as cache_type,
    COUNT(*) as total_entries,
    SUM(CASE WHEN expires_at > datetime('now') THEN 1 ELSE 0 END) as valid_entries,
    SUM(LENGTH(results)) / 1024.0 / 1024.0 as size_mb,
    AVG(hit_count) as avg_hits
FROM search_cache
UNION ALL
SELECT 
    'llm_cache',
    COUNT(*),
    SUM(CASE WHEN expires_at > datetime('now') THEN 1 ELSE 0 END),
    SUM(LENGTH(response)) / 1024.0 / 1024.0,
    AVG(hit_count)
FROM llm_cache
UNION ALL
SELECT 
    'computed_cache',
    COUNT(*),
    SUM(CASE WHEN expires_at > datetime('now') THEN 1 ELSE 0 END),
    SUM(LENGTH(value)) / 1024.0 / 1024.0,
    AVG(hit_count)
FROM computed_cache
UNION ALL
SELECT 
    'embedding_cache',
    COUNT(*),
    SUM(CASE WHEN expires_at > datetime('now') THEN 1 ELSE 0 END),
    SUM(LENGTH(embedding)) / 1024.0 / 1024.0,
    AVG(hit_count)
FROM embedding_cache;

CREATE VIEW IF NOT EXISTS v_cache_hit_rates AS
SELECT 
    'search_cache' as cache_type,
    SUM(hit_count) as total_hits,
    COUNT(*) as total_entries,
    CASE 
        WHEN COUNT(*) > 0 
        THEN ROUND(SUM(hit_count) * 1.0 / COUNT(*), 2)
        ELSE 0 
    END as avg_hits_per_entry
FROM search_cache
UNION ALL
SELECT 
    'llm_cache',
    SUM(hit_count),
    COUNT(*),
    CASE WHEN COUNT(*) > 0 THEN ROUND(SUM(hit_count) * 1.0 / COUNT(*), 2) ELSE 0 END
FROM llm_cache
UNION ALL
SELECT 
    'computed_cache',
    SUM(hit_count),
    COUNT(*),
    CASE WHEN COUNT(*) > 0 THEN ROUND(SUM(hit_count) * 1.0 / COUNT(*), 2) ELSE 0 END
FROM computed_cache
UNION ALL
SELECT 
    'embedding_cache',
    SUM(hit_count),
    COUNT(*),
    CASE WHEN COUNT(*) > 0 THEN ROUND(SUM(hit_count) * 1.0 / COUNT(*), 2) ELSE 0 END
FROM embedding_cache;

-- ============================================================
-- FUNÇÕES DE LIMPEZA
-- ============================================================

-- Limpar entradas expiradas
-- (será implementado em Go com frequency configurável)

-- Limitar tamanho por tipo
-- (será implementado com eviction policy LRU)
