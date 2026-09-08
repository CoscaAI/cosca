-- COSCA DSMS Migration Script
-- Cria todos os bancos a partir dos schemas
-- Criado: 2026-09-08 | ADR-046

-- ============================================================
-- BANCO 1: cosca-core.db
-- ============================================================
.read schema/001_core.sql

-- ============================================================
-- BANCO 2: cosca-knowledge.db
-- ============================================================
-- Nota: knowledge.db é o maior banco (840MB)
-- Schema será aplicado incrementalmente

-- ============================================================
-- BANCO 3: cosca-memory.db
-- ============================================================
.read schema/003_memory.sql

-- ============================================================
-- BANCO 4: cosca-intelligence.db
-- ============================================================
.read schema/004_intelligence.sql

-- ============================================================
-- BANCO 5: cosca-operations.db
-- ============================================================
.read schema/005_operations.sql

-- ============================================================
-- BANCO 6: cosca-cache.db
-- ============================================================
.read schema/006_cache.sql

-- ============================================================
-- VALIDAÇÃO FINAL
-- ============================================================
-- Verificar schemas versionados
SELECT 'Schema version applied successfully' as status;
SELECT version, description, applied_at FROM schema_version ORDER BY version DESC LIMIT 5;
