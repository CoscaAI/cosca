// Package gate implements the "Motor de transições Gate" for the Cosca
// approval workflow (Gate 0-4).
//
// É uma máquina de estados real onde apenas o papel certo move um plano entre
// estados:
//
//	plan → approving → approved → executed
//	          └→ rejected        └→ rolled-back
//
// A "guarda de papel" (inspirada no ColumnMoveRestrictionModel do Kanboard)
// garante que só o approver (don/admin) aprova um plano e que o executor
// (ex.: specialist) NÃO executa sem que o Don tenha aprovado — transições
// são restritas por papel e validadas contra uma TransitionTable.
//
// Cada gate vive em .cosca/gate.db (SQLite): a tabela gate_records guarda o
// estado atual e o histórico imutável de transições (coluna JSON), append-only.
package gate

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

	_ "modernc.org/sqlite" // Import SQLite driver for gate record storage
)

// ID prefix and zero-padding for gate records (G-0001).
const (
	IDPrefix = "G-"
	IDWidth  = 4
)

// Papéis da guarda. Aprovação é privilégio do Don/admin (approver); o
// executor (ex.: "specialist") pode PLANEJAR, mas nunca EXECUTAR por conta
// própria.
const (
	RoleDon        = "don"
	RoleAdmin      = "admin"
	RoleEditor     = "editor"
	RoleSpecialist = "specialist"
)

// State é um estado do ciclo de vida de um plano no Gate (Gate 0-4).
type State string

// Estados possíveis de um plano no Gate.
const (
	StatePlan       State = "plan"        // plano criado (Gate 0)
	StateApproving  State = "approving"   // em análise/aprovação (Gate 1)
	StateApproved   State = "approved"    // aprovado pelo Don (Gate 2)
	StateExecuted   State = "executed"    // executado (Gate 3)
	StateRejected   State = "rejected"    // rejeitado pelo Don (Gate 4)
	StateRolledBack State = "rolled-back" // revertido após execução (Gate 4)
)

// Transition é uma transição permitida entre estados, com os papéis que a
// podem executar (guarda de papel).
type Transition struct {
	From         State
	To           State
	AllowedRoles []string
}

// TransitionTable é a máquina de estados do Gate: só as transições listadas
// aqui existem, e cada uma só pode ser executada pelos papéis autorizados.
//
//	plan        → approving  : editor+ (editor, admin, don)
//	approving   → approved   : SOMENTE don/admin (o approver)
//	approving   → rejected   : don/admin
//	approved    → executed   : SOMENTE don/admin (approver ≠ executor)
//	executed    → rolled-back: don/admin
var TransitionTable = []Transition{
	{From: StatePlan, To: StateApproving, AllowedRoles: []string{RoleEditor, RoleAdmin, RoleDon}},
	{From: StateApproving, To: StateApproved, AllowedRoles: []string{RoleDon, RoleAdmin}},
	{From: StateApproving, To: StateRejected, AllowedRoles: []string{RoleDon, RoleAdmin}},
	{From: StateApproved, To: StateExecuted, AllowedRoles: []string{RoleDon, RoleAdmin}},
	{From: StateExecuted, To: StateRolledBack, AllowedRoles: []string{RoleDon, RoleAdmin}},
}

// CanTransition valida uma transição contra a TransitionTable: devolve true
// apenas quando a transição existe E o papel está autorizado (guarda de
// papel). Transição inexistente ou papel não autorizado → false.
func CanTransition(from, to State, role string) bool {
	t, ok := findTransition(from, to)
	if !ok {
		return false
	}
	for _, r := range t.AllowedRoles {
		if r == role {
			return true
		}
	}
	return false
}

// findTransition procura a transição (from, to) na TransitionTable.
func findTransition(from, to State) (Transition, bool) {
	for _, t := range TransitionTable {
		if t.From == from && t.To == to {
			return t, true
		}
	}
	return Transition{}, false
}

// ParseState converte a string de um estado em um State válido.
func ParseState(s string) (State, error) {
	switch State(strings.ToLower(strings.TrimSpace(s))) {
	case StatePlan:
		return StatePlan, nil
	case StateApproving:
		return StateApproving, nil
	case StateApproved:
		return StateApproved, nil
	case StateExecuted:
		return StateExecuted, nil
	case StateRejected:
		return StateRejected, nil
	case StateRolledBack:
		return StateRolledBack, nil
	}
	return "", fmt.Errorf("gate: estado inválido %q (esperava plan, approving, approved, executed, rejected ou rolled-back)", s)
}

// TransitionTableString documenta a máquina de estados em forma legível.
func TransitionTableString() string {
	return "plan → approving → approved → executed; approving → rejected; executed → rolled-back"
}

// GateTransition é uma transição registrada no ledger do Gate — imutável.
type GateTransition struct {
	From        State     `json:"from"`
	To          State     `json:"to"`
	By          string    `json:"by"`
	At          time.Time `json:"at"`
	DurationSec int64     `json:"duration_sec"` // segundos desde a transição anterior
}

// GateRecord é o registro de um plano no Gate: estado atual + histórico
// completo (append-only) de transições.
type GateRecord struct {
	ID          string           `json:"id"` // G-0001 auto-increment
	PlanRef     string           `json:"plan_ref"`
	State       State            `json:"state"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Transitions []GateTransition `json:"transitions"`
}

// GateStore é um armazenamento thread-safe, SQLite-backed, dos registros do
// Gate (.cosca/gate.db). Espelha o estilo do audit/decision store.
type GateStore struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// NewGateStore abre ou cria a base do Gate no caminho dado (normalmente
// <projeto>/.cosca/gate.db). Garante o diretório pai, cria a tabela
// gate_records (auto-migração idempotente) e configura WAL.
func NewGateStore(dbPath string) (*GateStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create gate db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open gate database: %w", err)
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

	s := &GateStore{db: db, dbPath: dbPath}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate gate database: %w", err)
	}
	return s, nil
}

// migrate cria a tabela gate_records e os índices se não existirem. O
// histórico de transições é uma coluna JSON (append-only, em ordem).
func (s *GateStore) migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS gate_records (
		id          TEXT PRIMARY KEY,
		plan_ref    TEXT NOT NULL DEFAULT '',
		state       TEXT NOT NULL DEFAULT 'plan',
		created_at  TEXT NOT NULL DEFAULT '',
		updated_at  TEXT NOT NULL DEFAULT '',
		transitions TEXT NOT NULL DEFAULT '[]'
	);

	CREATE INDEX IF NOT EXISTS idx_gate_updated_at ON gate_records(updated_at);
	`

	_, err := s.db.Exec(ddl)
	return err
}

// Create abre um novo gate para o plano planRef: atribui um ID G-XXXX
// sequencial e o registra no estado StatePlan. Retorna o ID atribuído.
func (s *GateStore) Create(planRef, by string) (string, error) {
	if strings.TrimSpace(planRef) == "" {
		return "", fmt.Errorf("gate: plan_ref é obrigatório")
	}

	id, err := s.nextID()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.db.Exec(
		`INSERT INTO gate_records (id, plan_ref, state, created_at, updated_at, transitions)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, planRef, string(StatePlan), fmtTime(now), fmtTime(now), "[]",
	)
	if err != nil {
		return "", fmt.Errorf("gate: criar %s: %w", id, err)
	}
	return id, nil
}

// Move valida a transição do gate para o estado `to` executada por `by` com
// o papel `role` (guarda de papel). Em caso de sucesso, registra a transição
// no ledger (append-only) e atualiza o estado. Em caso de negação, devolve
// erro e o estado permanece inalterado. Movimentos para o próprio estado são
// no-ops (paridade com a quarentena).
func (s *GateStore) Move(id string, to State, by, role string) error {
	norm, err := NormalizeID(id)
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
		return fmt.Errorf("gate: registro %s não encontrado", norm)
	}

	from := rec.State
	if from == to {
		return nil // já está no estado alvo — no-op
	}

	t, ok := findTransition(from, to)
	if !ok {
		return fmt.Errorf("gate: transição inválida %s→%s (permitidas: %s)",
			from, to, TransitionTableString())
	}
	if !roleAllowed(t, role) {
		return fmt.Errorf("gate: papel '%s' não pode mover %s→%s", role, from, to)
	}

	now := time.Now().UTC()
	prev := rec.UpdatedAt
	if prev.IsZero() {
		prev = rec.CreatedAt
	}
	secs := int64(0)
	if !prev.IsZero() {
		secs = int64(now.Sub(prev).Seconds())
	}

	rec.Transitions = append(rec.Transitions, GateTransition{
		From:        from,
		To:          to,
		By:          by,
		At:          now,
		DurationSec: secs,
	})
	rec.State = to
	rec.UpdatedAt = now

	transJSON, err := json.Marshal(rec.Transitions)
	if err != nil {
		return fmt.Errorf("gate: serializar transições de %s: %w", norm, err)
	}

	if _, err := s.db.Exec(
		`UPDATE gate_records SET state = ?, updated_at = ?, transitions = ? WHERE id = ?`,
		string(rec.State), fmtTime(rec.UpdatedAt), string(transJSON), norm,
	); err != nil {
		return fmt.Errorf("gate: mover %s → %s: %w", norm, to, err)
	}
	return nil
}

// Get recupera o registro do gate pelo ID normalizado. Retorna nil, nil
// quando o gate não existe.
func (s *GateStore) Get(id string) (*GateRecord, error) {
	norm, err := NormalizeID(id)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getLocked(norm)
}

// getLocked lê um registro assumindo que o lock já foi adquirido.
func (s *GateStore) getLocked(id string) (*GateRecord, error) {
	var rec GateRecord
	var state, createdAt, updatedAt, transitions string
	err := s.db.QueryRow(
		`SELECT id, plan_ref, state, created_at, updated_at, transitions
		 FROM gate_records WHERE id = ?`,
		id,
	).Scan(&rec.ID, &rec.PlanRef, &state, &createdAt, &updatedAt, &transitions)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gate: ler %s: %w", id, err)
	}

	rec.State = State(state)
	rec.CreatedAt = parseTime(createdAt)
	rec.UpdatedAt = parseTime(updatedAt)
	if err := json.Unmarshal([]byte(transitions), &rec.Transitions); err != nil {
		return nil, fmt.Errorf("gate: parsear transições de %s: %w", id, err)
	}
	if rec.Transitions == nil {
		rec.Transitions = make([]GateTransition, 0)
	}
	return &rec, nil
}

// List retorna os gates mais recentes (updated_at DESC), limitados ao valor
// dado. Limit <= 0 assume 20; > 1000 é truncado.
func (s *GateStore) List(limit int) ([]GateRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, plan_ref, state, created_at, updated_at, transitions
		 FROM gate_records ORDER BY updated_at DESC, id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("gate: listar: %w", err)
	}
	defer rows.Close()

	records := make([]GateRecord, 0)
	for rows.Next() {
		var rec GateRecord
		var state, createdAt, updatedAt, transitions string
		if err := rows.Scan(&rec.ID, &rec.PlanRef, &state, &createdAt, &updatedAt, &transitions); err != nil {
			return nil, fmt.Errorf("gate: scan: %w", err)
		}
		rec.State = State(state)
		rec.CreatedAt = parseTime(createdAt)
		rec.UpdatedAt = parseTime(updatedAt)
		if err := json.Unmarshal([]byte(transitions), &rec.Transitions); err != nil {
			return nil, fmt.Errorf("gate: parsear transições de %s: %w", rec.ID, err)
		}
		if rec.Transitions == nil {
			rec.Transitions = make([]GateTransition, 0)
		}
		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("gate: iterar: %w", err)
	}
	if records == nil {
		records = make([]GateRecord, 0)
	}
	return records, nil
}

// Ledger devolve o histórico imutável de transições do gate, em ordem
// cronológica (append-only). Devolve erro se o gate não existir.
func (s *GateStore) Ledger(id string) ([]GateTransition, error) {
	rec, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		norm, _ := NormalizeID(id)
		return nil, fmt.Errorf("gate: registro %s não encontrado", norm)
	}
	return rec.Transitions, nil
}

// Close encerra a conexão após um WAL checkpoint (paridade com audit.Store).
func (s *GateStore) Close() error {
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

// roleAllowed verifica se o papel pode executar a transição.
func roleAllowed(t Transition, role string) bool {
	for _, r := range t.AllowedRoles {
		if r == role {
			return true
		}
	}
	return false
}

// nextID retorna o próximo ID sequencial (G-0001 quando a tabela está vazia),
// varrendo o maior número já usado em gate_records — IDs nunca são reutilizados.
func (s *GateStore) nextID() (string, error) {
	var max int64
	err := s.db.QueryRow(
		"SELECT COALESCE(MAX(CAST(SUBSTR(id, ?) AS INTEGER)), 0) FROM gate_records",
		len(IDPrefix)+1,
	).Scan(&max)
	if err != nil {
		return "", fmt.Errorf("gate: next id: %w", err)
	}
	return NextID(int(max) + 1), nil
}

// NextID formata o número dado como ID de gate zero-padded (largura 4):
// NextID(7) → "G-0007".
func NextID(n int) string {
	return fmt.Sprintf("%s%0*d", IDPrefix, IDWidth, n)
}

// NormalizeID aceita "G-0001", "g-0001", "G-1" e "0001" e devolve a forma
// canônica zero-padded "G-0001". Erro para entradas não numéricas.
func NormalizeID(id string) (string, error) {
	digits := strings.TrimPrefix(strings.TrimSpace(strings.ToUpper(id)), IDPrefix)
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return "", fmt.Errorf("gate: ID inválido %q (esperava G-0001)", id)
	}
	return NextID(n), nil
}

// fmtTime serializa um time.Time como RFC3339 UTC com FRACÃO FIXA (9 dígitos,
// zero-padded). Nano é armazenada como TEXT e ordenada com ORDER BY updated_at
// DESC — para isso a largura da fração DEVE ser fixa: time.RFC3339Nano remove
// zeros à direita ("0.5Z" vs "0.500000001Z"), e lexicograficamente "0.5Z" >
// "0.500000001Z" mesmo sendo um instante ANTERIOR → ordenação errada. A forma
// "0.000000000" é sortable e preserva nanossegundo.
const sortableRFC3339Nano = "2006-01-02T15:04:05.000000000Z07:00"

func fmtTime(t time.Time) string {
	return t.UTC().Format(sortableRFC3339Nano)
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
