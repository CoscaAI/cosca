-- ============================================================================
-- Cosca Knowledge Base — Schema Version Control
-- ============================================================================
-- Tabela que rastreia quais migrations foram aplicadas no cosca.db.
-- Inspirado no migration_history do internal/sqlite, mas com coluna status
-- adicional para suportar o ciclo de vida da KB (applied/failed/rolled_back).
--
-- Política: migrations são forward-only. Rollback = nova migration corretiva.
-- ============================================================================

CREATE TABLE IF NOT EXISTS schema_version (
    version     INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL,
    applied_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    checksum    TEXT    NOT NULL,
    status      TEXT    NOT NULL DEFAULT 'applied'
        CHECK (status IN ('applied', 'failed', 'rolled_back'))
);

CREATE INDEX IF NOT EXISTS idx_schema_version_status ON schema_version(status);
