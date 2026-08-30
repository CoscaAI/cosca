// Decision Trace (trilha de decisão) — explicabilidade de decisão.
//
// Toda decisão/tarefa da Cosca deve conseguir responder "Por que fiz isso?" com
// um rastro estruturado e auditável: Decision ID, Input, Knowledge used,
// Laws applied, Evidence, Provider, Model, Aprovação humana, Resultado e
// Rollback. É ouro para auditoria.
//
// O DecisionStore usa a MESMA base SQLite do audit (por padrão
// .cosca/audit.db), adicionando uma tabela `decision_log` com auto-migração
// no open — o comportamento existente do audit.Store permanece intocado.
package audit

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

	_ "modernc.org/sqlite" // Import SQLite driver for decision trace storage
)

// ID prefix and zero-padding for decision traces (D-1842).
const (
	DecisionIDPrefix = "D-"
	DecisionIDWidth  = 4
)

// Status possíveis de uma decisão na trilha.
const (
	DecisionStatusApproved   = "approved"
	DecisionStatusRejected   = "rejected"
	DecisionStatusExecuted   = "executed"
	DecisionStatusRolledBack = "rolled-back"
)

// DecisionRecord é o rastro estruturado e auditável de uma decisão da Cosca.
type DecisionRecord struct {
	DecisionID    string   `json:"decision_id"`    // D-000X auto-increment
	Input         string   `json:"input"`          // o estímulo/decisão ("Por que fiz isso?")
	KnowledgeUsed []string `json:"knowledge_used"` // [K-18, K-91]
	LawsApplied   []string `json:"laws_applied"`   // [L-07, L-13]
	Evidence      []string `json:"evidence"`       // [E-182, E-201]
	Provider      string   `json:"provider"`       // "ollama"/"none"/etc
	Model         string   `json:"model"`
	Approval      string   `json:"approval"` // "Don / Gate 0"
	Result        string   `json:"result"`
	Rollback      string   `json:"rollback"`
	Timestamp     int64    `json:"timestamp"`
	Status        string   `json:"status"` // approved|rejected|executed|rolled-back

	// KnowledgeSnapshot (Fase 2A, ADR-029 §2.4) identifica o snapshot do
	// conhecimento/memória que estava carregado no momento da decisão. Responde
	// deterministicamente "qual conhecimento o cérebro tinha quando decidiu?".
	// O caller normalmente a preenche via ResolveKnowledgeSnapshot (best-effort);
	// vazio ("") é válido e significa que o snapshot não pôde ser resolvido.
	KnowledgeSnapshot string `json:"knowledge_snapshot"` // ex: ks_2026_08_29_001
	MemorySnapshot    string `json:"memory_snapshot"`    // hash/id do snapshot de memória, quando disponível
}

// DecisionStore é um armazenamento thread-safe, SQLite-backed, da trilha de
// decisão. Usa a mesma base do audit.Store, apenas com a tabela decision_log.
type DecisionStore struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// NewDecisionStore abre ou cria a base na caminho dado (normalmente o mesmo
// .cosca/audit.db do audit). Garante a existência do diretório pai, cria a
// tabela decision_log (auto-migração idempotente) e configura WAL.
func NewDecisionStore(dbPath string) (*DecisionStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create decision db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open decision database: %w", err)
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

	s := &DecisionStore{db: db, dbPath: dbPath}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate decision database: %w", err)
	}

	return s, nil
}

// migrate cria a tabela decision_log e os índices se não existirem
// (auto-migração no open). Colunas de lista são armazenadas como JSON text.
//
// Fase 2A (ADR-029 §2.4): a tabela carrega knowledge_snapshot/memory_snapshot.
// Para bancos JÁ criados (antes da Fase 2A) as colunas são adicionadas de forma
// idempotente via ALTER TABLE guarded por PRAGMA table_info — nunca quebra uma
// base existente.
func (s *DecisionStore) migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS decision_log (
		decision_id         TEXT PRIMARY KEY,
		input               TEXT NOT NULL DEFAULT '',
		knowledge_used      TEXT NOT NULL DEFAULT '[]',
		laws_applied        TEXT NOT NULL DEFAULT '[]',
		evidence            TEXT NOT NULL DEFAULT '[]',
		provider            TEXT NOT NULL DEFAULT '',
		model               TEXT NOT NULL DEFAULT '',
		approval            TEXT NOT NULL DEFAULT '',
		result              TEXT NOT NULL DEFAULT '',
		rollback            TEXT NOT NULL DEFAULT '',
		timestamp           INTEGER NOT NULL,
		status              TEXT NOT NULL DEFAULT 'executed',
		knowledge_snapshot  TEXT NOT NULL DEFAULT '',
		memory_snapshot     TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_decision_timestamp ON decision_log(timestamp);
	CREATE INDEX IF NOT EXISTS idx_decision_status    ON decision_log(status);
	`

	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create decision_log table: %w", err)
	}

	// Retro-compatibilidade: bancos criados antes da Fase 2A não têm as colunas
	// de snapshot — adiciona-as de forma idempotente (best-effort, sem quebrar).
	if err := s.addColumnIfMissing("decision_log", "knowledge_snapshot", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.addColumnIfMissing("decision_log", "memory_snapshot", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}

	return nil
}

// addColumnIfMissing adiciona a coluna `name` (com a declaração `decl`) à tabela
// `table` SE ela ainda não existir. Idempotente: reabrir uma base já migrada não
// altera nada. Usa PRAGMA table_info para inspecionar as colunas existentes.
func (s *DecisionStore) addColumnIfMissing(table, name, decl string) error {
	exists, err := s.columnExists(table, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	// Nome/declaração de coluna vêm de constantes internas — não é input de
	// usuário, então interpolar é seguro (sem risco de SQL injection).
	if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, name, decl)); err != nil {
		return fmt.Errorf("add column %s to %s: %w", name, table, err)
	}
	return nil
}

// columnExists informa se a coluna `name` existe na tabela `table`.
func (s *DecisionStore) columnExists(table, name string) (bool, error) {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, fmt.Errorf("pragmas table_info(%s): %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			colName   string
			colType   string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &dfltValue, &pk); err != nil {
			return false, fmt.Errorf("scan table_info(%s): %w", table, err)
		}
		if colName == name {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate table_info(%s): %w", table, err)
	}
	return false, nil
}

// Record persiste uma decisão na trilha. Se a DecisionID estiver vazia,
// atribui a próxima sequencial (D-0001, D-0002, ...). Se o Timestamp for zero,
// usa o momento atual; se o Status estiver vazio, assume "executed". Retorna
// a DecisionID atribuída.
func (s *DecisionStore) Record(d DecisionRecord) (string, error) {
	if d.DecisionID == "" {
		id, err := s.nextID()
		if err != nil {
			return "", err
		}
		d.DecisionID = id
	}
	if d.Timestamp == 0 {
		d.Timestamp = time.Now().Unix()
	}
	if d.Status == "" {
		d.Status = DecisionStatusExecuted
	}

	ku, err := json.Marshal(d.KnowledgeUsed)
	if err != nil {
		return "", fmt.Errorf("record decision: marshal knowledge_used: %w", err)
	}
	la, err := json.Marshal(d.LawsApplied)
	if err != nil {
		return "", fmt.Errorf("record decision: marshal laws_applied: %w", err)
	}
	ev, err := json.Marshal(d.Evidence)
	if err != nil {
		return "", fmt.Errorf("record decision: marshal evidence: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO decision_log
		 (decision_id, input, knowledge_used, laws_applied, evidence, provider,
		  model, approval, result, rollback, timestamp, status,
		  knowledge_snapshot, memory_snapshot)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.DecisionID, d.Input, string(ku), string(la), string(ev),
		d.Provider, d.Model, d.Approval, d.Result, d.Rollback,
		d.Timestamp, d.Status, d.KnowledgeSnapshot, d.MemorySnapshot,
	)
	if err != nil {
		return "", fmt.Errorf("record decision: %w", err)
	}

	return d.DecisionID, nil
}

// Get recupera uma decisão pela DecisionID normalizada. Retorna nil, nil
// quando a decisão não existe.
func (s *DecisionStore) Get(id string) (*DecisionRecord, error) {
	norm, err := NormalizeDecisionID(id)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var d DecisionRecord
	var ku, la, ev string
	err = s.db.QueryRow(
		`SELECT decision_id, input, knowledge_used, laws_applied, evidence,
		        provider, model, approval, result, rollback, timestamp, status,
		        knowledge_snapshot, memory_snapshot
		 FROM decision_log WHERE decision_id = ?`,
		norm,
	).Scan(&d.DecisionID, &d.Input, &ku, &la, &ev, &d.Provider,
		&d.Model, &d.Approval, &d.Result, &d.Rollback, &d.Timestamp, &d.Status,
		&d.KnowledgeSnapshot, &d.MemorySnapshot)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get decision: %w", err)
	}

	if err := json.Unmarshal([]byte(ku), &d.KnowledgeUsed); err != nil {
		return nil, fmt.Errorf("get decision: parse knowledge_used: %w", err)
	}
	if err := json.Unmarshal([]byte(la), &d.LawsApplied); err != nil {
		return nil, fmt.Errorf("get decision: parse laws_applied: %w", err)
	}
	if err := json.Unmarshal([]byte(ev), &d.Evidence); err != nil {
		return nil, fmt.Errorf("get decision: parse evidence: %w", err)
	}

	return &d, nil
}

// List retorna as decisões mais recentes (timestamp DESC), limitadas ao valor
// dado. Limit <= 0 assume 20; > 1000 é truncado.
func (s *DecisionStore) List(limit int) ([]DecisionRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT decision_id, input, knowledge_used, laws_applied, evidence,
		        provider, model, approval, result, rollback, timestamp, status,
		        knowledge_snapshot, memory_snapshot
		 FROM decision_log ORDER BY timestamp DESC, decision_id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query decisions: %w", err)
	}
	defer rows.Close()

	records := make([]DecisionRecord, 0)
	for rows.Next() {
		var d DecisionRecord
		var ku, la, ev string
		if err := rows.Scan(&d.DecisionID, &d.Input, &ku, &la, &ev,
			&d.Provider, &d.Model, &d.Approval, &d.Result, &d.Rollback,
			&d.Timestamp, &d.Status, &d.KnowledgeSnapshot, &d.MemorySnapshot); err != nil {
			return nil, fmt.Errorf("scan decision: %w", err)
		}
		if err := json.Unmarshal([]byte(ku), &d.KnowledgeUsed); err != nil {
			return nil, fmt.Errorf("scan decision: parse knowledge_used: %w", err)
		}
		if err := json.Unmarshal([]byte(la), &d.LawsApplied); err != nil {
			return nil, fmt.Errorf("scan decision: parse laws_applied: %w", err)
		}
		if err := json.Unmarshal([]byte(ev), &d.Evidence); err != nil {
			return nil, fmt.Errorf("scan decision: parse evidence: %w", err)
		}
		records = append(records, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate decisions: %w", err)
	}

	if records == nil {
		records = make([]DecisionRecord, 0)
	}

	return records, nil
}

// Count retorna o total de decisões registradas na trilha.
func (s *DecisionStore) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM decision_log").Scan(&total); err != nil {
		return 0, fmt.Errorf("count decisions: %w", err)
	}
	return total, nil
}

// Close encerra a conexão após um WAL checkpoint (paridade com audit.Store).
func (s *DecisionStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		return s.db.Close()
	}
	return nil
}

// nextID retorna a próxima DecisionID sequencial (D-0001 quando a tabela está
// vazia), varrendo o maior número já usado na decision_log.
func (s *DecisionStore) nextID() (string, error) {
	var max int64
	err := s.db.QueryRow(
		"SELECT COALESCE(MAX(CAST(SUBSTR(decision_id, ?) AS INTEGER)), 0) FROM decision_log",
		len(DecisionIDPrefix)+1,
	).Scan(&max)
	if err != nil {
		return "", fmt.Errorf("next decision id: %w", err)
	}
	return NextDecisionID(int(max) + 1), nil
}

// NextDecisionID formata o número dado como DecisionID zero-padded
// (largura 4): NextDecisionID(1842) → "D-1842", NextDecisionID(7) → "D-0007".
func NextDecisionID(n int) string {
	return fmt.Sprintf("%s%0*d", DecisionIDPrefix, DecisionIDWidth, n)
}

// NormalizeDecisionID aceita "D-1842", "d-1842", "D-1842" e "1842" e devolve a
// forma canônica zero-padded "D-1842". Erro para entradas não numéricas.
func NormalizeDecisionID(id string) (string, error) {
	digits := strings.TrimPrefix(strings.TrimSpace(strings.ToUpper(id)), DecisionIDPrefix)
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return "", fmt.Errorf("audit: ID de decisão inválido %q (esperava D-0001)", id)
	}
	return NextDecisionID(n), nil
}
