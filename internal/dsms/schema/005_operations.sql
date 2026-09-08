-- COSCA-OPERATIONS.DB Schema
-- Operações: projects, tasks, agents, skills, plugins
-- Criado: 2026-09-08 | ADR-046

PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- ============================================================
-- PROJETOS REGISTRADOS
-- ============================================================
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    path        TEXT NOT NULL,
    stack       TEXT,  -- JSON array de tecnologias
    status      TEXT DEFAULT 'active',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);

-- ============================================================
-- TAREFAS/EXECUÇÕES
-- ============================================================
CREATE TABLE IF NOT EXISTS tasks (
    id          TEXT PRIMARY KEY,
    project_id  TEXT REFERENCES projects(id),
    type        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    priority    TEXT DEFAULT 'medium',
    input       TEXT,  -- JSON input
    output      TEXT,  -- JSON output
    agent       TEXT,
    started_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    duration_ms INTEGER,
    error       TEXT
);

CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_agent ON tasks(agent);
CREATE INDEX IF NOT EXISTS idx_tasks_started ON tasks(started_at);

-- ============================================================
-- ESTADO DOS AGENTES
-- ============================================================
CREATE TABLE IF NOT EXISTS agents (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL,
    status          TEXT DEFAULT 'idle',
    current_task_id TEXT REFERENCES tasks(id),
    last_active     DATETIME,
    tasks_completed INTEGER DEFAULT 0,
    tasks_failed    INTEGER DEFAULT 0,
    avg_duration_ms INTEGER DEFAULT 0,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agents_type ON agents(type);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);

-- ============================================================
-- SKILLS INSTALADOS
-- ============================================================
CREATE TABLE IF NOT EXISTS skills (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    version     TEXT,
    description TEXT,
    category    TEXT,
    enabled     INTEGER DEFAULT 1,
    config      TEXT,  -- JSON config
    installed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_skills_category ON skills(category);
CREATE INDEX IF NOT EXISTS idx_skills_enabled ON skills(enabled);

-- ============================================================
-- PLUGINS ATIVOS
-- ============================================================
CREATE TABLE IF NOT EXISTS plugins (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    version     TEXT,
    status      TEXT DEFAULT 'active',
    config      TEXT,  -- JSON config
    hooks       TEXT,  -- JSON array de hooks
    installed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_plugins_status ON plugins(status);

-- ============================================================
-- WORKFLOWS
-- ============================================================
CREATE TABLE IF NOT EXISTS workflows (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    steps       TEXT NOT NULL,  -- JSON array de steps
    status      TEXT DEFAULT 'active',
    trigger     TEXT,  -- JSON trigger config
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_workflows_status ON workflows(status);

-- ============================================================
-- EXECUÇÕES DE WORKFLOW
-- ============================================================
CREATE TABLE IF NOT EXISTS workflow_runs (
    id          TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id),
    task_id     TEXT REFERENCES tasks(id),
    status      TEXT NOT NULL DEFAULT 'running',
    step_index  INTEGER DEFAULT 0,
    input       TEXT,
    output      TEXT,
    started_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    error       TEXT
);

CREATE INDEX IF NOT EXISTS idx_workflow_runs_workflow ON workflow_runs(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_runs_status ON workflow_runs(status);

-- ============================================================
-- LOG DE OPERAÇÕES
-- ============================================================
CREATE TABLE IF NOT EXISTS operation_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    operation   TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id   TEXT,
    details     TEXT,
    agent       TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_operation_log_operation ON operation_log(operation);
CREATE INDEX IF NOT EXISTS idx_operation_log_entity ON operation_log(entity_type);
CREATE INDEX IF NOT EXISTS idx_operation_log_time ON operation_log(created_at);

-- ============================================================
-- MÉTRICAS DE OPERAÇÃO
-- ============================================================
CREATE TABLE IF NOT EXISTS operation_metrics (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    metric_name TEXT NOT NULL,
    metric_value REAL NOT NULL,
    dimensions  TEXT,  -- JSON: {"project": "x", "agent": "y"}
    measured_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_operation_metrics_name ON operation_metrics(metric_name);
CREATE INDEX IF NOT EXISTS idx_operation_metrics_time ON operation_metrics(measured_at);

-- ============================================================
-- VIEWS
-- ============================================================
CREATE VIEW IF NOT EXISTS v_operations_summary AS
SELECT 
    (SELECT COUNT(*) FROM projects WHERE status = 'active') as active_projects,
    (SELECT COUNT(*) FROM tasks WHERE status = 'running') as running_tasks,
    (SELECT COUNT(*) FROM tasks WHERE status = 'completed') as completed_tasks,
    (SELECT COUNT(*) FROM tasks WHERE status = 'failed') as failed_tasks,
    (SELECT COUNT(*) FROM agents WHERE status != 'offline') as active_agents,
    (SELECT COUNT(*) FROM skills WHERE enabled = 1) as active_skills,
    (SELECT COUNT(*) FROM plugins WHERE status = 'active') as active_plugins;

CREATE VIEW IF NOT EXISTS v_agent_performance AS
SELECT 
    a.id as agent_id,
    a.name as agent_name,
    a.type as agent_type,
    a.tasks_completed,
    a.tasks_failed,
    a.avg_duration_ms,
    CASE 
        WHEN a.tasks_completed + a.tasks_failed > 0 
        THEN ROUND(a.tasks_completed * 100.0 / (a.tasks_completed + a.tasks_failed), 2)
        ELSE 0 
    END as success_rate_pct
FROM agents a;

CREATE VIEW IF NOT EXISTS v_task_stats AS
SELECT 
    type,
    status,
    COUNT(*) as count,
    AVG(duration_ms) as avg_duration,
    MIN(duration_ms) as min_duration,
    MAX(duration_ms) as max_duration
FROM tasks
GROUP BY type, status;

CREATE VIEW IF NOT EXISTS v_recent_activity AS
SELECT 
    'task' as entity_type,
    id as entity_id,
    type as operation,
    status,
    started_at as timestamp
FROM tasks
WHERE started_at > datetime('now', '-24 hours')
UNION ALL
SELECT 
    'workflow_run',
    id,
    workflow_id,
    status,
    started_at
FROM workflow_runs
WHERE started_at > datetime('now', '-24 hours')
ORDER BY timestamp DESC
LIMIT 100;
