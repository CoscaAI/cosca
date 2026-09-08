-- COSCA-KNOWLEDGE.DB Schema
-- Conhecimento curado: entries, chunks, embeddings, graph, entities
-- Criado: 2026-09-08 | ADR-046

PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- ============================================================
-- ENTRADAS DE CONHECIMENTO (118K+)
-- ============================================================
CREATE TABLE IF NOT EXISTS entries (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    content     TEXT NOT NULL,
    category    TEXT,
    source      TEXT,
    confidence  REAL DEFAULT 0.5,
    domain      TEXT DEFAULT 'general',
    language    TEXT DEFAULT 'pt',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    archived_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_entries_category ON entries(category);
CREATE INDEX IF NOT EXISTS idx_entries_domain ON entries(domain);
CREATE INDEX IF NOT EXISTS idx_entries_confidence ON entries(confidence);
CREATE INDEX IF NOT EXISTS idx_entries_source ON entries(source);
CREATE INDEX IF NOT EXISTS idx_entries_created ON entries(created_at);

-- ============================================================
-- CHUNKS DE TEXTO (para busca)
-- ============================================================
CREATE TABLE IF NOT EXISTS chunks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id    INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    token_count INTEGER,
    chunk_index INTEGER DEFAULT 0,
    chunk_type  TEXT DEFAULT 'text',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chunks_entry ON chunks(entry_id);
CREATE INDEX IF NOT EXISTS idx_chunks_type ON chunks(chunk_type);

-- ============================================================
-- EMBEDDINGS VETORIAIS (122K+)
-- ============================================================
CREATE TABLE IF NOT EXISTS embeddings (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    chunk_id    INTEGER NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    vector      BLOB NOT NULL,
    model       TEXT DEFAULT 'cosca-default',
    dimension   INTEGER DEFAULT 384,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_embeddings_chunk ON embeddings(chunk_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_model ON embeddings(model);

-- ============================================================
-- GRAFO DE CONHECIMENTO (122K+ nós)
-- ============================================================
CREATE TABLE IF NOT EXISTS graph_nodes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    type        TEXT NOT NULL,
    name        TEXT NOT NULL,
    properties  TEXT,
    domain      TEXT DEFAULT 'general',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_graph_nodes_type ON graph_nodes(type);
CREATE INDEX IF NOT EXISTS idx_graph_nodes_domain ON graph_nodes(domain);
CREATE INDEX IF NOT EXISTS idx_graph_nodes_name ON graph_nodes(name);

CREATE TABLE IF NOT EXISTS graph_edges (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id   INTEGER NOT NULL REFERENCES graph_nodes(id) ON DELETE CASCADE,
    target_id   INTEGER NOT NULL REFERENCES graph_nodes(id) ON DELETE CASCADE,
    relationship TEXT NOT NULL,
    weight      REAL DEFAULT 1.0,
    properties  TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_graph_edges_source ON graph_edges(source_id);
CREATE INDEX IF NOT EXISTS idx_graph_edges_target ON graph_edges(target_id);
CREATE INDEX IF NOT EXISTS idx_graph_edges_relationship ON graph_edges(relationship);

-- ============================================================
-- ENTIDADES EXTRAÍDAS
-- ============================================================
CREATE TABLE IF NOT EXISTS entities (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,
    properties  TEXT,
    confidence  REAL DEFAULT 0.5,
    source_entry_id INTEGER REFERENCES entries(id),
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_entities_type ON entities(type);
CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name);

-- ============================================================
-- RELAÇÕES ENTRE ENTIDADES
-- ============================================================
CREATE TABLE IF NOT EXISTS entity_relationships (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    source_entity_id INTEGER NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    target_entity_id INTEGER NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    relationship    TEXT NOT NULL,
    weight          REAL DEFAULT 1.0,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_entity_rel_source ON entity_relationships(source_entity_id);
CREATE INDEX IF NOT EXISTS idx_entity_rel_target ON entity_relationships(target_entity_id);

-- ============================================================
-- DOMÍNIOS (para particionamento lógico)
-- ============================================================
CREATE TABLE IF NOT EXISTS domains (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    entry_count INTEGER DEFAULT 0,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO domains (id, name, description) VALUES
    ('general', 'Geral', 'Conhecimento geral'),
    ('code', 'Código', 'Padrões de código e implementação'),
    ('docs', 'Documentação', 'Documentação do projeto'),
    ('architecture', 'Arquitetura', 'Decisões arquiteturais'),
    ('security', 'Segurança', 'Padrões de segurança'),
    ('performance', 'Performance', 'Otimização e benchmarks'),
    ('testing', 'Testes', 'Estratégias de teste'),
    ('devops', 'DevOps', 'CI/CD e infraestrutura');

-- ============================================================
-- VIEWS
-- ============================================================
CREATE VIEW IF NOT EXISTS v_knowledge_summary AS
SELECT 
    domain,
    COUNT(*) as entry_count,
    AVG(confidence) as avg_confidence,
    MAX(updated_at) as last_updated
FROM entries
WHERE archived_at IS NULL
GROUP BY domain;

CREATE VIEW IF NOT EXISTS v_graph_summary AS
SELECT 
    type,
    COUNT(*) as node_count
FROM graph_nodes
GROUP BY type;

CREATE VIEW IF NOT EXISTS v_embedding_stats AS
SELECT 
    model,
    COUNT(*) as embedding_count,
    AVG(dimension) as avg_dimension
FROM embeddings
GROUP BY model;

-- ============================================================
-- FUNÇÕES AUXILIARES
-- ============================================================

-- Busca por similaridade (será implementada em Go)
-- CREATE VIRTUAL TABLE IF NOT EXISTS embeddings_fts USING fts5(content, chunk_id);

-- Estatísticas de domínio
CREATE VIEW IF NOT EXISTS v_domain_stats AS
SELECT 
    d.id as domain_id,
    d.name as domain_name,
    COUNT(DISTINCT e.id) as entries,
    COUNT(DISTINCT c.id) as chunks,
    COUNT(DISTINCT em.id) as embeddings,
    COUNT(DISTINCT gn.id) as graph_nodes
FROM domains d
LEFT JOIN entries e ON e.domain = d.id AND e.archived_at IS NULL
LEFT JOIN chunks c ON c.entry_id = e.id
LEFT JOIN embeddings em ON em.chunk_id = c.id
LEFT JOIN graph_nodes gn ON gn.domain = d.id
GROUP BY d.id;
