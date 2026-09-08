-- COSCA-CORE.DB Schema
-- Configuração, autenticação, segredos e versionamento de schema
-- Criado: 2026-09-08 | ADR-046

PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- ============================================================
-- CONFIGURAÇÃO (chave-valor)
-- ============================================================
CREATE TABLE IF NOT EXISTS config (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    category    TEXT DEFAULT 'general',
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Configurações padrão do sistema
INSERT OR IGNORE INTO config (key, value, category) VALUES
    ('version', '1.6.0', 'system'),
    ('created_at', datetime('now'), 'system'),
    ('dsms.enabled', 'true', 'dsms'),
    ('dsms.health_interval', '5m', 'dsms'),
    ('dsms.compact_interval', '24h', 'dsms'),
    ('dsms.archive_interval', '7d', 'dsms'),
    ('cache.max_size_mb', '100', 'cache'),
    ('cache.ttl_hours', '1', 'cache'),
    ('memory.retention_days', '90', 'memory'),
    ('operations.retention_days', '30', 'operations');

-- ============================================================
-- AUTENTICAÇÃO (tokens de API)
-- ============================================================
CREATE TABLE IF NOT EXISTS auth_tokens (
    id          TEXT PRIMARY KEY,
    token_hash  TEXT NOT NULL,
    provider    TEXT NOT NULL,
    scope       TEXT DEFAULT 'all',
    expires_at  DATETIME,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_used   DATETIME
);

CREATE INDEX IF NOT EXISTS idx_auth_tokens_provider ON auth_tokens(provider);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_expires ON auth_tokens(expires_at);

-- ============================================================
-- SEGREDS (credenciais criptografadas)
-- ============================================================
CREATE TABLE IF NOT EXISTS secrets (
    id              TEXT PRIMARY KEY,
    encrypted_value BLOB NOT NULL,
    provider        TEXT NOT NULL,
    secret_type     TEXT DEFAULT 'api_key',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    rotated_at      DATETIME
);

CREATE INDEX IF NOT EXISTS idx_secrets_provider ON secrets(provider);

-- ============================================================
-- VERSIONAMENTO DE SCHEMA
-- ============================================================
CREATE TABLE IF NOT EXISTS schema_version (
    version     INTEGER PRIMARY KEY,
    applied_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    description TEXT,
    checksum    TEXT
);

-- Versão inicial do DSMS
INSERT OR IGNORE INTO schema_version (version, description) VALUES
    (1, 'DSMS initial schema - ADR-046');

-- ============================================================
-- AUDIT LOG (quem mudou o quê)
-- ============================================================
CREATE TABLE IF NOT EXISTS audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    action      TEXT NOT NULL,
    table_name  TEXT NOT NULL,
    record_id   TEXT,
    old_value   TEXT,
    new_value   TEXT,
    user_id     TEXT DEFAULT 'system',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_log_table ON audit_log(table_name);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_log_time ON audit_log(created_at);

-- ============================================================
-- VIEWS
-- ============================================================
CREATE VIEW IF NOT EXISTS v_config_summary AS
SELECT 
    category,
    COUNT(*) as config_count,
    MAX(updated_at) as last_updated
FROM config
GROUP BY category;

CREATE VIEW IF NOT EXISTS v_auth_summary AS
SELECT 
    provider,
    COUNT(*) as token_count,
    SUM(CASE WHEN expires_at > datetime('now') THEN 1 ELSE 0 END) as active_count
FROM auth_tokens
GROUP BY provider;
