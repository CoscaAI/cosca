package task

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/CoscaAI/cosca/internal/privfile"

	_ "modernc.org/sqlite"
)

// SQLStore é a implementação concreta do TaskRepository (ADR-015 F2) apoiada
// em SQLite. Persiste TaskState como uma linha JSON — a PRIMITIVA (internal/
// task) continua pura (stdlib only); esta é a borda de domínio que injeta a
// persistência.
//
// Convenção do contrato (repository.go):
//   - Save: UPSERT idempotente pela TaskID;
//   - Load: devolve UM TaskState (nil se não existir) — cópia via Clone;
//   - List: devolve TODOS — cópia de cada um;
//   - Delete: remove pela TaskID.
//
// O arquivo é tasks.db (o módulo de tarefas do runtime). WAL + busy_timeout,
// mesmo padrão dos demais stores da casa (audit, trace, ...).
type SQLStore struct {
	db     *sql.DB
	dbPath string
}

// NewSQLStore abre (ou cria) o store de tarefas em dbPath.
func NewSQLStore(dbPath string) (*SQLStore, error) {
	if !isMemoryDSN(dbPath) {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, fmt.Errorf("create tasks db directory: %w", err)
		}
		if err := privfile.EnsurePrivateDBFile(dbPath); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open tasks database: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA cache_size=-20000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	s := &SQLStore{db: db, dbPath: dbPath}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate tasks database: %w", err)
	}
	return s, nil
}

// migrate cria a tabela tasks se não existir.
func (s *SQLStore) migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS tasks (
		task_id  TEXT PRIMARY KEY,
		state    TEXT NOT NULL,      -- JSON do TaskState
		status   TEXT NOT NULL,      -- coluna derivada p/ filtros rápidos
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create tasks table: %w", err)
	}
	return nil
}

// Close encerra a conexão.
func (s *SQLStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Save upserta um TaskState (idempotente pela TaskID).
func (s *SQLStore) Save(st *TaskState) error {
	if st == nil {
		return fmt.Errorf("task: save nil state")
	}
	stateJSON, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("task: marshal state: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO tasks (task_id, state, status) VALUES (?, ?, ?)
		 ON CONFLICT(task_id) DO UPDATE SET state=excluded.state, status=excluded.status,
		   updated_at=datetime('now')`,
		string(st.TaskID), string(stateJSON), string(st.Status),
	)
	if err != nil {
		return fmt.Errorf("task: save %q: %w", st.TaskID, err)
	}
	return nil
}

// Load devolve UM TaskState (nil se não existir). Retorna cópia (segura).
func (s *SQLStore) Load(taskID TaskID) (*TaskState, error) {
	var stateJSON string
	err := s.db.QueryRow(`SELECT state FROM tasks WHERE task_id = ?`, string(taskID)).Scan(&stateJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("task: load %q: %w", taskID, err)
	}
	var st TaskState
	if err := json.Unmarshal([]byte(stateJSON), &st); err != nil {
		return nil, fmt.Errorf("task: unmarshal %q: %w", taskID, err)
	}
	cloned := st.Clone()
	return &cloned, nil
}

// List devolve TODOS os TaskStates (cópias independentes).
func (s *SQLStore) List() ([]*TaskState, error) {
	rows, err := s.db.Query(`SELECT state FROM tasks ORDER BY updated_at`)
	if err != nil {
		return nil, fmt.Errorf("task: list: %w", err)
	}
	defer rows.Close()

	var out []*TaskState
	for rows.Next() {
		var stateJSON string
		if err := rows.Scan(&stateJSON); err != nil {
			return nil, fmt.Errorf("task: scan list: %w", err)
		}
		var st TaskState
		if err := json.Unmarshal([]byte(stateJSON), &st); err != nil {
			return nil, fmt.Errorf("task: unmarshal list row: %w", err)
		}
		cloned := st.Clone()
		out = append(out, &cloned)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task: iterate list: %w", err)
	}
	return out, nil
}

// Delete remove uma tarefa pela TaskID.
func (s *SQLStore) Delete(taskID TaskID) error {
	if _, err := s.db.Exec(`DELETE FROM tasks WHERE task_id = ?`, string(taskID)); err != nil {
		return fmt.Errorf("task: delete %q: %w", taskID, err)
	}
	return nil
}

// isMemoryDSN reporta se o DSN é um banco in-memory (":memory:...").
func isMemoryDSN(dsn string) bool {
	return len(dsn) >= len(":memory:") && dsn[:len(":memory:")] == ":memory:"
}
