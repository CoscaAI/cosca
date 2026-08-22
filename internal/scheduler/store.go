// Store do cron — persistência SQLite em .cosca/cron.db.
//
// Espelha o estilo do audit/decision/gate store: thread-safe, auto-migração
// no open (CREATE TABLE IF NOT EXISTS), coluna JSON para o Schedule, WAL e
// IDs sequenciais C-XXXX. Nenhuma dependência externa além do driver SQLite
// modernc.org/sqlite já usado por audit/decision/gate.
package scheduler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Import SQLite driver for cron job storage
)

// ID prefix and zero-padding for cron jobs (C-0001).
const (
	JobIDPrefix = "C-"
	JobIDWidth  = 4
)

// Job é um agendamento determinístico: um comando (shell) executado conforme
// o Schedule. Só roda via `cosca cron daemon` — nada roda sozinho.
type Job struct {
	ID       string    `json:"id"`       // C-0001 auto-increment
	Name     string    `json:"name"`     // rótulo dado pelo Don (ex.: "backup")
	Command  string    `json:"command"`  // comando shell (sh -c), definido pelo Don
	Schedule Schedule  `json:"schedule"` // agenda normalizada
	Enabled  bool      `json:"enabled"`  // se o daemon deve executar
	LastRun  time.Time `json:"last_run"` // última execução (zero = nunca)
	NextRun  time.Time `json:"next_run"` // próxima ocorrência calculada
	Runs     int       `json:"runs"`     // total de execuções registradas
}

// Store é um armazenamento thread-safe, SQLite-backed, dos jobs do cron
// (.cosca/cron.db).
type Store struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// NewStore abre ou cria a base do cron no caminho dado (normalmente
// <projeto>/.cosca/cron.db). Garante o diretório pai, cria a tabela
// cron_jobs (auto-migração idempotente) e configura WAL.
func NewStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create cron db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open cron database: %w", err)
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

	s := &Store{db: db, dbPath: dbPath}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate cron database: %w", err)
	}
	return s, nil
}

// migrate cria a tabela cron_jobs e os índices se não existirem. O Schedule
// é armazenado como JSON text; times como RFC3339Nano UTC ("" = zero).
func (s *Store) migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS cron_jobs (
		id       TEXT PRIMARY KEY,
		name     TEXT NOT NULL DEFAULT '',
		command  TEXT NOT NULL DEFAULT '',
		schedule TEXT NOT NULL DEFAULT '{}',
		enabled  INTEGER NOT NULL DEFAULT 1,
		last_run TEXT NOT NULL DEFAULT '',
		next_run TEXT NOT NULL DEFAULT '',
		runs     INTEGER NOT NULL DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_cron_jobs_enabled  ON cron_jobs(enabled);
	CREATE INDEX IF NOT EXISTS idx_cron_jobs_next_run ON cron_jobs(next_run);
	`

	_, err := s.db.Exec(ddl)
	return err
}

// Add persiste um novo job. Se o ID estiver vazio, atribui o próximo C-XXXX
// sequencial. Se o NextRun for zero, calcula a primeira ocorrência a partir
// de agora. Retorna o ID atribuído.
func (s *Store) Add(j Job) (string, error) {
	if strings.TrimSpace(j.Name) == "" {
		return "", fmt.Errorf("scheduler: name é obrigatório")
	}
	if strings.TrimSpace(j.Command) == "" {
		return "", fmt.Errorf("scheduler: command é obrigatório")
	}
	if err := j.Schedule.Validate(); err != nil {
		return "", err
	}

	if j.ID == "" {
		id, err := s.nextID()
		if err != nil {
			return "", err
		}
		j.ID = id
	} else {
		norm, err := NormalizeID(j.ID)
		if err != nil {
			return "", err
		}
		j.ID = norm
	}

	if j.NextRun.IsZero() {
		next, err := j.Schedule.NextAfter(time.Now())
		if err != nil {
			return "", err
		}
		j.NextRun = next
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	schedJSON, err := json.Marshal(j.Schedule)
	if err != nil {
		return "", fmt.Errorf("scheduler: serializar agenda de %s: %w", j.ID, err)
	}

	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO cron_jobs
		 (id, name, command, schedule, enabled, last_run, next_run, runs)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.Name, j.Command, string(schedJSON), boolToInt(j.Enabled),
		fmtTime(j.LastRun), fmtTime(j.NextRun), j.Runs,
	)
	if err != nil {
		return "", fmt.Errorf("scheduler: adicionar job %s: %w", j.ID, err)
	}
	return j.ID, nil
}

// DBPath devolve o caminho da base SQLite (.cosca/cron.db).
func (s *Store) DBPath() string {
	return s.dbPath
}

// Get recupera um job pelo ID normalizado. Retorna nil, nil quando não existe.
func (s *Store) Get(id string) (*Job, error) {
	norm, err := NormalizeID(id)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getLocked(norm)
}

// getLocked lê um job assumindo que o lock já foi adquirido.
func (s *Store) getLocked(id string) (*Job, error) {
	var j Job
	var enabled int
	var schedJSON, lastRun, nextRun string
	err := s.db.QueryRow(
		`SELECT id, name, command, schedule, enabled, last_run, next_run, runs
		 FROM cron_jobs WHERE id = ?`,
		id,
	).Scan(&j.ID, &j.Name, &j.Command, &schedJSON, &enabled, &lastRun, &nextRun, &j.Runs)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scheduler: ler job %s: %w", id, err)
	}

	j.Enabled = intToBool(enabled)
	j.LastRun = parseTime(lastRun)
	j.NextRun = parseTime(nextRun)
	if err := json.Unmarshal([]byte(schedJSON), &j.Schedule); err != nil {
		return nil, fmt.Errorf("scheduler: parsear agenda de %s: %w", id, err)
	}
	return &j, nil
}

// List retorna todos os jobs ordenados por ID (ascendente).
func (s *Store) List() ([]Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, name, command, schedule, enabled, last_run, next_run, runs
		 FROM cron_jobs ORDER BY id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("scheduler: listar jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]Job, 0)
	for rows.Next() {
		var j Job
		var enabled int
		var schedJSON, lastRun, nextRun string
		if err := rows.Scan(&j.ID, &j.Name, &j.Command, &schedJSON, &enabled,
			&lastRun, &nextRun, &j.Runs); err != nil {
			return nil, fmt.Errorf("scheduler: scan job: %w", err)
		}
		j.Enabled = intToBool(enabled)
		j.LastRun = parseTime(lastRun)
		j.NextRun = parseTime(nextRun)
		if err := json.Unmarshal([]byte(schedJSON), &j.Schedule); err != nil {
			return nil, fmt.Errorf("scheduler: parsear agenda de %s: %w", j.ID, err)
		}
		jobs = append(jobs, j)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scheduler: iterar jobs: %w", err)
	}
	if jobs == nil {
		jobs = make([]Job, 0)
	}
	return jobs, nil
}

// Remove deleta o job. Devolve erro se o ID não existir.
func (s *Store) Remove(id string) error {
	norm, err := NormalizeID(id)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec("DELETE FROM cron_jobs WHERE id = ?", norm)
	if err != nil {
		return fmt.Errorf("scheduler: remover job %s: %w", norm, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("scheduler: job %s não encontrado em .cosca/cron.db", norm)
	}
	return nil
}

// SetEnabled liga/desliga o job. Se for reabilitado com NextRun zero (job
// antigo), recalcula a próxima ocorrência.
func (s *Store) SetEnabled(id string, enabled bool) error {
	norm, err := NormalizeID(id)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	j, err := s.getLocked(norm)
	if err != nil {
		return err
	}
	if j == nil {
		return fmt.Errorf("scheduler: job %s não encontrado em .cosca/cron.db", norm)
	}

	nextRun := j.NextRun
	if enabled && nextRun.IsZero() {
		next, err := j.Schedule.NextAfter(time.Now())
		if err != nil {
			return err
		}
		nextRun = next
	}

	res, err := s.db.Exec(
		"UPDATE cron_jobs SET enabled = ?, next_run = ? WHERE id = ?",
		boolToInt(enabled), fmtTime(nextRun), norm,
	)
	if err != nil {
		return fmt.Errorf("scheduler: atualizar enabled de %s: %w", norm, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("scheduler: job %s não encontrado em .cosca/cron.db", norm)
	}
	return nil
}

// UpdateNextRun atualiza apenas a próxima ocorrência do job.
func (s *Store) UpdateNextRun(id string, next time.Time) error {
	norm, err := NormalizeID(id)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec("UPDATE cron_jobs SET next_run = ? WHERE id = ?", fmtTime(next), norm)
	if err != nil {
		return fmt.Errorf("scheduler: atualizar next_run de %s: %w", norm, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("scheduler: job %s não encontrado em .cosca/cron.db", norm)
	}
	return nil
}

// RecordRun registra o resultado de uma execução: LastRun, próxima
// ocorrência e total de runs.
func (s *Store) RecordRun(id string, lastRun, nextRun time.Time, runs int) error {
	norm, err := NormalizeID(id)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec(
		"UPDATE cron_jobs SET last_run = ?, next_run = ?, runs = ? WHERE id = ?",
		fmtTime(lastRun), fmtTime(nextRun), runs, norm,
	)
	if err != nil {
		return fmt.Errorf("scheduler: registrar execução de %s: %w", norm, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("scheduler: job %s não encontrado em .cosca/cron.db", norm)
	}
	return nil
}

// Count retorna o total de jobs registrados.
func (s *Store) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM cron_jobs").Scan(&total); err != nil {
		return 0, fmt.Errorf("scheduler: contar jobs: %w", err)
	}
	return total, nil
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

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

// nextID retorna o próximo ID sequencial (C-0001 quando a tabela está vazia),
// varrendo o maior número já usado em cron_jobs — IDs nunca são reutilizados.
func (s *Store) nextID() (string, error) {
	var max int64
	err := s.db.QueryRow(
		"SELECT COALESCE(MAX(CAST(SUBSTR(id, ?) AS INTEGER)), 0) FROM cron_jobs",
		len(JobIDPrefix)+1,
	).Scan(&max)
	if err != nil {
		return "", fmt.Errorf("scheduler: next id: %w", err)
	}
	return NextID(int(max) + 1), nil
}

// NextID formata o número dado como ID de job zero-padded (largura 4):
// NextID(7) → "C-0007".
func NextID(n int) string {
	return fmt.Sprintf("%s%0*d", JobIDPrefix, JobIDWidth, n)
}

// NormalizeID aceita "C-0001", "c-0001", "C-1" e "0001" e devolve a forma
// canônica zero-padded "C-0001". Erro para entradas não numéricas.
func NormalizeID(id string) (string, error) {
	digits := strings.TrimPrefix(strings.TrimSpace(strings.ToUpper(id)), JobIDPrefix)
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return "", fmt.Errorf("scheduler: ID de job inválido %q (esperava C-0001)", id)
	}
	return NextID(n), nil
}

// fmtTime serializa um time.Time como RFC3339Nano UTC ("" para zero time).
func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// parseTime reconstrói um time.Time a partir do texto RFC3339(Nano); devolve
// zero time para entradas vazias/inválidas.
func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

// boolToInt converte bool em 1/0 para o SQLite.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// intToBool converte 1/0 do SQLite em bool.
func intToBool(n int) bool {
	return n != 0
}
