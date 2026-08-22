// Event ledger (flight recorder) do Trace ID universal — armazenamento
// SQLite append-only em .cosca/trace.db.
//
// Espelha o estilo do audit/decision store (internal/audit): cria a base e a
// tabela `trace_events` com auto-migração no open, configura WAL e expõe
// apenas operações de escrita append (Append) e leitura (Get/Latest). NÃO
// existe Update nem Delete — o flight recorder nunca reescreve o passado.
package trace

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Import SQLite driver for trace event storage
)

// Store é o ledger append-only de eventos de trace, SQLite-backed.
type Store struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// NewStore abre ou cria a base no caminho dado (normalmente
// <projeto>/.cosca/trace.db). Garante a existência do diretório pai, cria a
// tabela trace_events (auto-migração idempotente) e configura WAL.
func NewStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create trace db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open trace database: %w", err)
	}

	// Configura SQLite como no audit.Store.
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

	s := &Store{db: db, dbPath: dbPath}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate trace database: %w", err)
	}

	return s, nil
}

// migrate cria a tabela trace_events e os índices se não existirem
// (auto-migração no open). A tabela é append-only por contrato: as operações
// expostas são apenas INSERT e SELECT.
func (s *Store) migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS trace_events (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		trace_id          TEXT NOT NULL,
		parent_event      TEXT NOT NULL DEFAULT '',
		timestamp         INTEGER NOT NULL,
		actor             TEXT NOT NULL DEFAULT '',
		action            TEXT NOT NULL DEFAULT '',
		input_hash        TEXT NOT NULL DEFAULT '',
		output_hash       TEXT NOT NULL DEFAULT '',
		code_version      TEXT NOT NULL DEFAULT '',
		knowledge_version TEXT NOT NULL DEFAULT '',
		cognitive_version TEXT NOT NULL DEFAULT '',
		environment       TEXT NOT NULL DEFAULT '',
		result            TEXT NOT NULL DEFAULT '',
		details           TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_trace_events_trace ON trace_events(trace_id);
	CREATE INDEX IF NOT EXISTS idx_trace_events_ts    ON trace_events(timestamp);
	`

	_, err := s.db.Exec(ddl)
	return err
}

// Append persiste um evento no ledger. O TraceID é obrigatório e precisa ser
// um Trace ID universal válido (TRACE-YYYYMMDD-XXXX). Defaults: Timestamp=0
// assume o momento atual (UTC); Environment vazio assume GOOS/GOARCH.
// Append é a ÚNICA operação de escrita — nunca há UPDATE/DELETE.
func (s *Store) Append(e Event) error {
	if _, ok := Parse(e.TraceID); !ok {
		return fmt.Errorf("trace: Trace ID inválido %q (esperava TRACE-YYYYMMDD-XXXX)", e.TraceID)
	}
	if s.db == nil {
		return fmt.Errorf("trace: store not initialized")
	}
	if e.Timestamp == 0 {
		e.Timestamp = time.Now().Unix()
	}
	if e.Environment == "" {
		e.Environment = runtime.GOOS + "/" + runtime.GOARCH
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO trace_events
		 (trace_id, parent_event, timestamp, actor, action, input_hash,
		  output_hash, code_version, knowledge_version, cognitive_version,
		  environment, result, details)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.TraceID, e.ParentEvent, e.Timestamp, e.Actor, e.Action, e.InputHash,
		e.OutputHash, e.CodeVersion, e.KnowledgeVersion, e.CognitiveVersion,
		e.Environment, e.Result, e.Details,
	)
	if err != nil {
		return fmt.Errorf("append trace event: %w", err)
	}
	return nil
}

// Get recupera todos os eventos de um trace, ordenados por timestamp
// (ascendente) e, em empate, por ordem de inserção — a linha do tempo exata
// do flight recorder. Retorna slice vazio (não nil) quando não há eventos.
func (s *Store) Get(traceID string) ([]Event, error) {
	if _, ok := Parse(traceID); !ok {
		return nil, fmt.Errorf("trace: Trace ID inválido %q (esperava TRACE-YYYYMMDD-XXXX)", traceID)
	}
	if s.db == nil {
		return nil, fmt.Errorf("trace: store not initialized")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT trace_id, parent_event, timestamp, actor, action, input_hash,
		        output_hash, code_version, knowledge_version, cognitive_version,
		        environment, result, details
		 FROM trace_events WHERE trace_id = ?
		 ORDER BY timestamp ASC, id ASC`,
		traceID,
	)
	if err != nil {
		return nil, fmt.Errorf("query trace events: %w", err)
	}
	defer rows.Close()

	events, err := scanEvents(rows)
	if err != nil {
		return nil, err
	}
	return events, nil
}

// Latest retorna os eventos mais recentes de todos os traces (timestamp DESC),
// limitados ao valor dado. Limit <= 0 assume 20; > 1000 é truncado.
func (s *Store) Latest(limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT trace_id, parent_event, timestamp, actor, action, input_hash,
		        output_hash, code_version, knowledge_version, cognitive_version,
		        environment, result, details
		 FROM trace_events
		 ORDER BY timestamp DESC, id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query latest trace events: %w", err)
	}
	defer rows.Close()

	events, err := scanEvents(rows)
	if err != nil {
		return nil, err
	}
	return events, nil
}

// Count retorna o total de eventos registrados no ledger.
func (s *Store) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return 0, fmt.Errorf("trace: store not initialized")
	}

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM trace_events").Scan(&total); err != nil {
		return 0, fmt.Errorf("count trace events: %w", err)
	}
	return total, nil
}

// DBPath devolve o caminho da base (usado na saída dos comandos).
func (s *Store) DBPath() string {
	return s.dbPath
}

// Close encerra a conexão após um WAL checkpoint (paridade com audit.Store).
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		return s.db.Close()
	}
	return nil
}

// scanEvents converte as linhas do SELECT canônico em []Event.
func scanEvents(rows *sql.Rows) ([]Event, error) {
	events := make([]Event, 0)
	for rows.Next() {
		var e Event
		if err := rows.Scan(
			&e.TraceID, &e.ParentEvent, &e.Timestamp, &e.Actor, &e.Action,
			&e.InputHash, &e.OutputHash, &e.CodeVersion,
			&e.KnowledgeVersion, &e.CognitiveVersion, &e.Environment,
			&e.Result, &e.Details,
		); err != nil {
			return nil, fmt.Errorf("scan trace event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trace events: %w", err)
	}
	if events == nil {
		events = make([]Event, 0)
	}
	return events, nil
}
