-- COSCA-INTELLIGENCE.DB Schema
-- Inteligência codificada: rules, patterns, expert_systems, decision_trees, metrics
-- Criado: 2026-09-08 | ADR-046 (para futuro ADR-047)

PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- ============================================================
-- REGRAS DETERMINÍSTICAS
-- ============================================================
CREATE TABLE IF NOT EXISTS rules (
    id          TEXT PRIMARY KEY,
    domain      TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    condition   TEXT NOT NULL,  -- JSON logic: {"op": "AND", "rules": [...]}
    action      TEXT NOT NULL,  -- JSON action: {"type": "flag", "severity": "high"}
    priority    INTEGER DEFAULT 0,
    confidence  REAL DEFAULT 0.5,
    version     INTEGER DEFAULT 1,
    enabled     INTEGER DEFAULT 1,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_rules_domain ON rules(domain);
CREATE INDEX IF NOT EXISTS idx_rules_priority ON rules(priority);
CREATE INDEX IF NOT EXISTS idx_rules_confidence ON rules(confidence);
CREATE INDEX IF NOT EXISTS idx_rules_enabled ON rules(enabled);

-- ============================================================
-- PADRÕES DE CÓDIGO
-- ============================================================
CREATE TABLE IF NOT EXISTS code_patterns (
    id          TEXT PRIMARY KEY,
    language    TEXT NOT NULL,
    pattern     TEXT NOT NULL,  -- Regex ou AST pattern
    description TEXT,
    severity    TEXT DEFAULT 'info',
    category    TEXT DEFAULT 'style',
    auto_fix    TEXT,  -- JSON fix template
    examples    TEXT,  -- JSON array de exemplos
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_code_patterns_language ON code_patterns(language);
CREATE INDEX IF NOT EXISTS idx_code_patterns_severity ON code_patterns(severity);
CREATE INDEX IF NOT EXISTS idx_code_patterns_category ON code_patterns(category);

-- ============================================================
-- SISTEMAS ESPECIALIZADOS
-- ============================================================
CREATE TABLE IF NOT EXISTS expert_systems (
    id          TEXT PRIMARY KEY,
    domain      TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    rules       TEXT NOT NULL,  -- JSON array of rule IDs
    version     INTEGER DEFAULT 1,
    accuracy    REAL,
    last_trained DATETIME,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_expert_systems_domain ON expert_systems(domain);

-- ============================================================
-- ÁRVORES DE DECISÃO
-- ============================================================
CREATE TABLE IF NOT EXISTS decision_trees (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    domain      TEXT NOT NULL,
    description TEXT,
    tree        TEXT NOT NULL,  -- JSON tree structure
    accuracy    REAL,
    f1_score    REAL,
    last_trained DATETIME,
    training_samples INTEGER,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_decision_trees_domain ON decision_trees(domain);

-- ============================================================
-- MÉTRICAS DE CÓDIGO
-- ============================================================
CREATE TABLE IF NOT EXISTS code_metrics (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path   TEXT NOT NULL,
    metric_type TEXT NOT NULL,
    value       REAL NOT NULL,
    language    TEXT,
    measured_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_code_metrics_file ON code_metrics(file_path);
CREATE INDEX IF NOT EXISTS idx_code_metrics_type ON code_metrics(metric_type);
CREATE INDEX IF NOT EXISTS idx_code_metrics_time ON code_metrics(measured_at);

-- ============================================================
-- BENCHMARKS
-- ============================================================
CREATE TABLE IF NOT EXISTS benchmarks (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    category    TEXT NOT NULL,
    input       TEXT,
    output      TEXT,
    duration_ms INTEGER,
    memory_mb   REAL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_benchmarks_category ON benchmarks(category);

-- ============================================================
-- FEEDBACK LOOP (aprendizado de erros)
-- ============================================================
CREATE TABLE IF NOT EXISTS feedback (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id     TEXT REFERENCES rules(id),
    pattern_id  TEXT REFERENCES code_patterns(id),
    outcome     TEXT NOT NULL,  -- 'correct' | 'incorrect' | 'partial'
    context     TEXT,
    correction  TEXT,
    agent       TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_feedback_rule ON feedback(rule_id);
CREATE INDEX IF NOT EXISTS idx_feedback_outcome ON feedback(outcome);

-- ============================================================
-- KNOWLEDGE DISTILLATION (extração de regras de LLM)
-- ============================================================
CREATE TABLE IF NOT EXISTS distilled_rules (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    source_prompt   TEXT NOT NULL,
    source_response TEXT NOT NULL,
    extracted_rule  TEXT NOT NULL,
    confidence      REAL DEFAULT 0.5,
    validated       INTEGER DEFAULT 0,
    agent           TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_distilled_rules_confidence ON distilled_rules(confidence);
CREATE INDEX IF NOT EXISTS idx_distilled_rules_validated ON distilled_rules(validated);

-- ============================================================
-- DOMÍNIOS DE INTELIGÊNCIA
-- ============================================================
CREATE TABLE IF NOT EXISTS intelligence_domains (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    rule_count  INTEGER DEFAULT 0,
    accuracy    REAL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO intelligence_domains (id, name, description) VALUES
    ('security', 'Segurança', 'Regras de segurança e vulnerabilidades'),
    ('architecture', 'Arquitetura', 'Padrões arquiteturais e design'),
    ('performance', 'Performance', 'Otimização e benchmarks'),
    ('code_quality', 'Qualidade de Código', 'Padrões e convenções'),
    ('testing', 'Testes', 'Estratégias de teste'),
    ('devops', 'DevOps', 'CI/CD e infraestrutura');

-- ============================================================
-- VIEWS
-- ============================================================
CREATE VIEW IF NOT EXISTS v_intelligence_summary AS
SELECT 
    d.id as domain,
    d.name as domain_name,
    COUNT(DISTINCT r.id) as rules,
    COUNT(DISTINCT cp.id) as code_patterns,
    COUNT(DISTINCT es.id) as expert_systems,
    COUNT(DISTINCT dt.id) as decision_trees
FROM intelligence_domains d
LEFT JOIN rules r ON r.domain = d.id AND r.enabled = 1
LEFT JOIN code_patterns cp ON cp.language = d.id
LEFT JOIN expert_systems es ON es.domain = d.id
LEFT JOIN decision_trees dt ON dt.domain = d.id
GROUP BY d.id;

CREATE VIEW IF NOT EXISTS v_rule_effectiveness AS
SELECT 
    r.id as rule_id,
    r.name as rule_name,
    r.domain,
    COUNT(f.id) as total_feedback,
    SUM(CASE WHEN f.outcome = 'correct' THEN 1 ELSE 0 END) as correct,
    SUM(CASE WHEN f.outcome = 'incorrect' THEN 1 ELSE 0 END) as incorrect,
    ROUND(SUM(CASE WHEN f.outcome = 'correct' THEN 1.0 ELSE 0 END) / COUNT(f.id) * 100, 2) as effectiveness_pct
FROM rules r
LEFT JOIN feedback f ON f.rule_id = r.id
GROUP BY r.id;

CREATE VIEW IF NOT EXISTS v_pattern_frequency AS
SELECT 
    language,
    category,
    COUNT(*) as pattern_count,
    SUM(CASE WHEN severity IN ('high', 'critical') THEN 1 ELSE 0 END) as critical_count
FROM code_patterns
GROUP BY language, category;
