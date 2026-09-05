// Contradição como entidade primeira classe (CONFLICT-XXX).
//
// Conforme a conversa do Don: quando duas fontes divergem sobre o mesmo item
// ("A: X funciona dessa maneira." / "B: X não funciona dessa maneira."), o
// Cosca NÃO escolhe uma — ele registra o conflito e se recusa a promover o
// item para lei enquanto o conflito estiver aberto. É o modelo:
//
//	Fonte A ──┐
//	          ├── CONFLICT ──┘
//	Fonte B ──┘
//
// O ConflictStore vive em uma base dedicada (.cosca/conflict.db, SQLite),
// espelhando o estilo do audit/decision/gate store, para NÃO tocar no schema
// de conhecimento existente (laws.json / knowledge.db).
//
// Integração é OPT-IN e não-regressiva: StatusWithConflicts(item, open)
// devolve CONFLICTING quando o item tem um conflito aberto; o PromotionEngine
// e o KnowledgeItem não são alterados — chamadores decidem quando usar.
package knowledge

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Import SQLite driver for conflict record storage
)

// ID prefix and zero-padding for conflict records (CONFLICT-001, width 3).
const (
	ConflictIDPrefix = "CONFLICT-"
	ConflictIDWidth  = 3
)

// ConflictStatus é o estado de um registro de contradição.
type ConflictStatus string

const (
	// ConflictOpen é o estado inicial: a contradição permanece em aberto.
	ConflictOpen ConflictStatus = "open"
	// ConflictResolved é o estado final: a contradição foi resolvida.
	ConflictResolved ConflictStatus = "resolved"
)

// Valid devolve true quando o status é um dos estados de conflito.
func (s ConflictStatus) Valid() bool {
	switch s {
	case ConflictOpen, ConflictResolved:
		return true
	default:
		return false
	}
}

// ConflictRecord é um registro de contradição primeira classe: duas alegações
// (claim_a, claim_b) divergentes sobre o mesmo item de conhecimento. A
// convenção de Claim é "item:evidence" (ex.: "K-27:E-101").
type ConflictRecord struct {
	ID          string         `json:"id"`                    // CONFLICT-XXX (largura 3)
	ClaimA      string         `json:"claim_a"`               // ex.: "K-27:E-101" (item:evidence)
	ClaimB      string         `json:"claim_b"`               // ex.: "K-27:E-203"
	ItemID      string         `json:"item_id"`               // o item de conhecimento afetado
	Description string         `json:"description"`           // ex.: "fonte A diz X, fonte B diz o contrário"
	DetectedAt  time.Time      `json:"detected_at"`           // quando o conflito foi detectado
	Status      ConflictStatus `json:"status"`                // open | resolved
	ResolvedAt  time.Time      `json:"resolved_at,omitempty"` // quando foi resolvido
}

// ConflictStore é um armazenamento thread-safe, SQLite-backed, dos registros
// de contradição (.cosca/conflict.db). Espelha o estilo do gate/audit store.
type ConflictStore struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// NewConflictStore abre ou cria a base de conflitos no caminho dado
// (normalmente <projeto>/.cosca/conflict.db). Garante o diretório pai, cria a
// tabela conflict_records (auto-migração idempotente) e configura WAL.
func NewConflictStore(dbPath string) (*ConflictStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create conflict db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open conflict database: %w", err)
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

	s := &ConflictStore{db: db, dbPath: dbPath}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate conflict database: %w", err)
	}
	return s, nil
}

// migrate cria a tabela conflict_records e os índices se não existirem.
func (s *ConflictStore) migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS conflict_records (
		id          TEXT PRIMARY KEY,
		claim_a     TEXT NOT NULL DEFAULT '',
		claim_b     TEXT NOT NULL DEFAULT '',
		item_id     TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		detected_at TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'open',
		resolved_at TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_conflict_item   ON conflict_records(item_id);
	CREATE INDEX IF NOT EXISTS idx_conflict_status ON conflict_records(status);
	`

	_, err := s.db.Exec(ddl)
	return err
}

// Add persiste um novo conflito. Se o ID estiver vazio, atribui a próxima
// sequencial (CONFLICT-001, CONFLICT-002, ...). Se o DetectedAt for zero, usa
// o momento atual; se o Status estiver vazio, assume open. Retorna o ID
// atribuído.
func (s *ConflictStore) Add(c ConflictRecord) (string, error) {
	if c.ID == "" {
		id, err := s.nextID()
		if err != nil {
			return "", err
		}
		c.ID = id
	}
	if c.DetectedAt.IsZero() {
		c.DetectedAt = time.Now().UTC()
	}
	if c.Status == "" {
		c.Status = ConflictOpen
	}
	if !c.Status.Valid() {
		return "", fmt.Errorf("conflict: status inválido %q", c.Status)
	}
	if strings.TrimSpace(c.ItemID) == "" {
		return "", fmt.Errorf("conflict: item_id é obrigatório")
	}
	if strings.TrimSpace(c.ClaimA) == "" || strings.TrimSpace(c.ClaimB) == "" {
		return "", fmt.Errorf("conflict: claim_a e claim_b são obrigatórias")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO conflict_records
		 (id, claim_a, claim_b, item_id, description, detected_at, status, resolved_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.ClaimA, c.ClaimB, c.ItemID, c.Description,
		conflictFmtTime(c.DetectedAt), string(c.Status), conflictFmtTime(c.ResolvedAt),
	)
	if err != nil {
		return "", fmt.Errorf("conflict: adicionar %s: %w", c.ID, err)
	}
	return c.ID, nil
}

// Get recupera um conflito pelo ID normalizado. Retorna nil, nil quando o
// conflito não existe.
func (s *ConflictStore) Get(id string) (*ConflictRecord, error) {
	norm, err := NormalizeConflictID(id)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getLocked(norm)
}

// getLocked lê um registro assumindo que o lock já foi adquirido.
func (s *ConflictStore) getLocked(id string) (*ConflictRecord, error) {
	var c ConflictRecord
	var status, detectedAt, resolvedAt string
	err := s.db.QueryRow(
		`SELECT id, claim_a, claim_b, item_id, description, detected_at, status, resolved_at
		 FROM conflict_records WHERE id = ?`,
		id,
	).Scan(&c.ID, &c.ClaimA, &c.ClaimB, &c.ItemID, &c.Description,
		&detectedAt, &status, &resolvedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("conflict: ler %s: %w", id, err)
	}

	c.Status = ConflictStatus(status)
	c.DetectedAt = conflictParseTime(detectedAt)
	c.ResolvedAt = conflictParseTime(resolvedAt)
	return &c, nil
}

// List retorna os conflitos registrados (detected_at DESC, id DESC). Quando
// um ou mais status são informados, filtra apenas os conflitos nesses estados.
func (s *ConflictStore) List(status ...ConflictStatus) ([]ConflictRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, claim_a, claim_b, item_id, description, detected_at, status, resolved_at
	          FROM conflict_records`
	args := make([]interface{}, 0, len(status))
	if len(status) > 0 {
		valid := make([]ConflictStatus, 0, len(status))
		seen := make(map[ConflictStatus]bool)
		for _, st := range status {
			if st.Valid() && !seen[st] {
				valid = append(valid, st)
				seen[st] = true
			}
		}
		if len(valid) > 0 {
			placeholders := make([]string, len(valid))
			for i, st := range valid {
				placeholders[i] = "?"
				args = append(args, string(st))
			}
			query += " WHERE status IN (" + strings.Join(placeholders, ", ") + ")"
		}
	}
	query += " ORDER BY detected_at DESC, id DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("conflict: listar: %w", err)
	}
	defer rows.Close()

	return scanConflictRows(rows)
}

// OpenConflictsForItem devolve os conflitos ABERTOS do item dado — o conjunto
// usado por StatusWithConflicts / HasOpenConflict para elevar o estado
// epistemológico do item para CONFLICTING.
func (s *ConflictStore) OpenConflictsForItem(itemID string) ([]ConflictRecord, error) {
	if strings.TrimSpace(itemID) == "" {
		return nil, fmt.Errorf("conflict: item_id é obrigatório")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, claim_a, claim_b, item_id, description, detected_at, status, resolved_at
		 FROM conflict_records WHERE item_id = ? AND status = ?
		 ORDER BY detected_at DESC, id DESC`,
		itemID, string(ConflictOpen),
	)
	if err != nil {
		return nil, fmt.Errorf("conflict: listar abertos de %s: %w", itemID, err)
	}
	defer rows.Close()

	return scanConflictRows(rows)
}

// scanConflictRows materializa as linhas de conflict_records em registros.
func scanConflictRows(rows *sql.Rows) ([]ConflictRecord, error) {
	records := make([]ConflictRecord, 0)
	for rows.Next() {
		var c ConflictRecord
		var status, detectedAt, resolvedAt string
		if err := rows.Scan(&c.ID, &c.ClaimA, &c.ClaimB, &c.ItemID, &c.Description,
			&detectedAt, &status, &resolvedAt); err != nil {
			return nil, fmt.Errorf("conflict: scan: %w", err)
		}
		c.Status = ConflictStatus(status)
		c.DetectedAt = conflictParseTime(detectedAt)
		c.ResolvedAt = conflictParseTime(resolvedAt)
		records = append(records, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("conflict: iterar: %w", err)
	}
	if records == nil {
		records = make([]ConflictRecord, 0)
	}
	return records, nil
}

// Resolve marca um conflito como resolvido: define Status=resolved e grava
// ResolvedAt. Conflito inexistente → erro. Já resolvido → no-op (idempotente).
func (s *ConflictStore) Resolve(id string) error {
	norm, err := NormalizeConflictID(id)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rec, err := s.getLocked(norm)
	if err != nil {
		return err
	}
	if rec == nil {
		return fmt.Errorf("conflict: registro %s não encontrado", norm)
	}
	if rec.Status == ConflictResolved {
		return nil // já resolvido — no-op
	}

	now := time.Now().UTC()
	if _, err := s.db.Exec(
		`UPDATE conflict_records SET status = ?, resolved_at = ? WHERE id = ?`,
		string(ConflictResolved), conflictFmtTime(now), norm,
	); err != nil {
		return fmt.Errorf("conflict: resolver %s: %w", norm, err)
	}
	return nil
}

// Close encerra a conexão após um WAL checkpoint (paridade com gate.Store).
func (s *ConflictStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		return s.db.Close()
	}
	return nil
}

// nextID retorna o próximo ID sequencial (CONFLICT-001 quando a tabela está
// vazia), varrendo o maior número já usado em conflict_records — IDs nunca
// são reutilizados.
func (s *ConflictStore) nextID() (string, error) {
	var max int64
	err := s.db.QueryRow(
		"SELECT COALESCE(MAX(CAST(SUBSTR(id, ?) AS INTEGER)), 0) FROM conflict_records",
		len(ConflictIDPrefix)+1,
	).Scan(&max)
	if err != nil {
		return "", fmt.Errorf("conflict: next id: %w", err)
	}
	return NextConflictID(int(max) + 1), nil
}

// NextConflictID formata o número dado como ID de conflito zero-padded
// (largura 3): NextConflictID(7) → "CONFLICT-007", NextConflictID(102) →
// "CONFLICT-102".
func NextConflictID(n int) string {
	return fmt.Sprintf("%s%0*d", ConflictIDPrefix, ConflictIDWidth, n)
}

// NormalizeConflictID aceita "CONFLICT-102", "conflict-102", "CONFLICT-7" e
// "102" e devolve a forma canônica zero-padded "CONFLICT-102". Erro para
// entradas não numéricas.
func NormalizeConflictID(id string) (string, error) {
	digits := strings.TrimPrefix(strings.TrimSpace(strings.ToUpper(id)), ConflictIDPrefix)
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return "", fmt.Errorf("knowledge: ID de conflito inválido %q (esperava CONFLICT-001)", id)
	}
	return NextConflictID(n), nil
}

// HasOpenConflict devolve true quando existe pelo menos um conflito aberto
// referenciando este item de conhecimento (item.ItemID == k.ID). É um helper
// puro de consulta — não altera o item nem o PromotionEngine.
func (k *KnowledgeItem) HasOpenConflict(open []ConflictRecord) bool {
	if k == nil {
		return false
	}
	for _, c := range open {
		if c.ItemID == k.ID && c.Status == ConflictOpen {
			return true
		}
	}
	return false
}

// StatusWithConflicts devolve o estado epistemológico do item levando em conta
// conflitos abertos: se o item tem um conflito aberto, devolve CONFLICTING
// (regra do Don: não promover para lei enquanto houver contradição); caso
// contrário, devolve o próprio status do item (ou UNKNOWN quando vazio).
// Integração OPT-IN — o PromotionEngine e o KnowledgeItem não são alterados;
// o chamador decide quando aplicar.
func StatusWithConflicts(item *KnowledgeItem, openConflicts []ConflictRecord) EpistemicStatus {
	if item == nil {
		return StatusUnknown
	}
	if item.HasOpenConflict(openConflicts) {
		return StatusConflicting
	}
	if item.Status == "" {
		return StatusUnknown
	}
	return item.Status
}

// conflictFmtTime serializa um time.Time como RFC3339Nano UTC (texto legível
// no SQLite), permitindo ordenação determinística.
func conflictFmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// conflictParseTime reconstrói um time.Time a partir do texto RFC3339(Nano);
// devolve zero time para entradas vazias/inválidas.
func conflictParseTime(s string) time.Time {
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
