// "How to think" — classificação de afirmações (claims) durante o raciocínio.
//
// Conforme a conversa do Don: o General Context passa a distinguir FACT,
// EVIDENCE, INFERENCE, ASSUMPTION, HYPOTHESIS e UNKNOWN — em vez de
// simplesmente produzir uma resposta plausível. É a base para Root Cause
// Analysis, Five Whys, Decision Trees, Evidence Evaluation e Uncertainty.
//
// A classificação é DETERMINÍSTICA (sem LLM): regras puras sobre sinais
// discretos (evidência, validação, derivação, teste, assunção explícita). O
// sistema sabe QUE TIPO de afirmação é — e decide se ela pode ser usada como
// está (IsTrustworthy) ou precisa ser sinalizada como suposição/desconhecida.
//
// Diferente do estado epistemológico (epistemic.go, sobre KNOWLEDGE ITEMS) e
// do nível do CKL (evidence.go, sobre a maturidade de um item), este arquivo
// classifica CLAIMS — afirmações feitas DURANTE o raciocínio.
//
// Persistência: base SQLite dedicada .cosca/claims.db (runtime, gitignored),
// espelhando o estilo do ConflictStore — para NÃO tocar em laws.json nem em
// knowledge.db.
package knowledge

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

	_ "modernc.org/sqlite" // Import SQLite driver for claim storage
)

// ── Tipos de afirmação ───────────────────────────────────────────────────────

// ClaimKind é o tipo de uma afirmação (claim) durante o raciocínio.
type ClaimKind string

const (
	// ClaimFact é estabelecido com evidência validada.
	ClaimFact ClaimKind = "FACT"
	// ClaimEvidence é uma observação registrada (evidência não validada).
	ClaimEvidence ClaimKind = "EVIDENCE"
	// ClaimInference é derivada de outra afirmação.
	ClaimInference ClaimKind = "INFERENCE"
	// ClaimAssumption é assumida sem evidência.
	ClaimAssumption ClaimKind = "ASSUMPTION"
	// ClaimHypothesis é testável, mas não confirmada.
	ClaimHypothesis ClaimKind = "HYPOTHESIS"
	// ClaimUnknown é sem base.
	ClaimUnknown ClaimKind = "UNKNOWN"
)

// Valid devolve true quando o tipo é um dos seis tipos de afirmação.
func (k ClaimKind) Valid() bool {
	switch k {
	case ClaimFact, ClaimEvidence, ClaimInference, ClaimAssumption, ClaimHypothesis, ClaimUnknown:
		return true
	default:
		return false
	}
}

// Description devolve a descrição pt-BR do tipo de afirmação.
func (k ClaimKind) Description() string {
	switch k {
	case ClaimFact:
		return "estabelecido com evidência validada"
	case ClaimEvidence:
		return "observação registrada"
	case ClaimInference:
		return "derivada de outra afirmação"
	case ClaimAssumption:
		return "assumida sem evidência"
	case ClaimHypothesis:
		return "não confirmada, testável"
	case ClaimUnknown:
		return "sem base"
	default:
		return "tipo de afirmação desconhecido"
	}
}

// ── Classificação determinística ─────────────────────────────────────────────

// Classifier aplica as regras determinísticas de classificação de uma
// afirmação. O campo ExplicitlyAssumed é a bandeira "assumida explicitamente"
// que sobrepõe TODA a precedência: quando o autor marca a afirmação como
// assumida, ela é sempre ASSUMPTION — mesmo com evidência validada (uma
// suposição assumida nunca deve virar fato silenciosamente).
type Classifier struct {
	// ExplicitlyAssumed marca a afirmação como assumida explicitamente pelo
	// autor — a exceção de precedência mais alta (sobrepõe FACT/EVIDENCE).
	ExplicitlyAssumed bool
}

// Precedência de classificação (avaliada em ordem — a primeira que casar
// vence):
//
//	FACT > EVIDENCE > INFERENCE > HYPOTHESIS > ASSUMPTION > UNKNOWN
//
//	1. ASSUMPTION — ExplicitlyAssumed (sobrepõe qualquer evidência).
//	2. FACT        — evidenceCount >= 1 E validated.
//	3. EVIDENCE    — evidenceCount >= 1 (observada, ainda não validada).
//	4. INFERENCE   — derived (construída a partir de outra afirmação). A
//	                 derivação NÃO exige evidência própria — o rastro de onde
//	                 a afirmação veio é a trilha Supports. Se a afirmação
//	                 derivada também tiver evidência própria não validada,
//	                 vence EVIDENCE; se validada, vence FACT.
//	5. HYPOTHESIS  — tested mas não confirmada (testável, sem validação).
//	6. ASSUMPTION  — sem evidência, sem derivação, sem teste: assumida.
//	7. UNKNOWN     — afirmação vazia / sem nenhum sinal (default).
func (c *Classifier) Classify(statement string, evidenceCount int, validated bool, derived bool, tested bool) ClaimKind {
	if c != nil && c.ExplicitlyAssumed {
		return ClaimAssumption
	}
	switch {
	case evidenceCount >= 1 && validated:
		return ClaimFact
	case evidenceCount >= 1:
		return ClaimEvidence
	case derived:
		return ClaimInference
	case tested:
		return ClaimHypothesis
	case strings.TrimSpace(statement) == "":
		return ClaimUnknown
	default:
		return ClaimAssumption
	}
}

// Classify aplica as regras determinísticas (sem LLM) com o Classifier
// default (nenhuma assunção explícita).
func Classify(statement string, evidenceCount int, validated bool, derived bool, tested bool) ClaimKind {
	return (&Classifier{}).Classify(statement, evidenceCount, validated, derived, tested)
}

// ClassifyWithReason devolve a classificação e a razão pt-BR que a disparou —
// usada pelo CLI para explicar a decisão ("por que isso é um FACT?").
func (c *Classifier) ClassifyWithReason(statement string, evidenceCount int, validated bool, derived bool, tested bool) (ClaimKind, string) {
	if c != nil && c.ExplicitlyAssumed {
		return ClaimAssumption, "assumida explicitamente pelo autor — sobrepõe qualquer evidência"
	}
	switch {
	case evidenceCount >= 1 && validated:
		return ClaimFact, "evidência validada registrada"
	case evidenceCount >= 1:
		return ClaimEvidence, "observação registrada (evidência não validada)"
	case derived:
		return ClaimInference, "derivada de outra afirmação (Supports)"
	case tested:
		return ClaimHypothesis, "testada, mas não confirmada"
	case strings.TrimSpace(statement) == "":
		return ClaimUnknown, "sem base — afirmação vazia"
	default:
		return ClaimAssumption, "assumida sem evidência"
	}
}

// ── Registro ─────────────────────────────────────────────────────────────────

// ClaimRecord é uma afirmação classificada durante o raciocínio.
type ClaimRecord struct {
	ID             string    `json:"id"`                        // CL-XXXX
	Kind           ClaimKind `json:"kind"`                      // FACT/EVIDENCE/...
	Statement      string    `json:"statement"`                 // a afirmação
	Supports       []string  `json:"supports,omitempty"`        // item/claim IDs que suportam
	ContradictedBy []string  `json:"contradicted_by,omitempty"` // claim IDs que contradizem
	Confidence     float64   `json:"confidence"`                // 0-1
	CreatedAt      time.Time `json:"created_at"`
}

// IsTrustworthy devolve true quando a afirmação pode ser usada como está pelo
// General Context: FACT (evidência validada) ou EVIDENCE com confiança >= 0.7.
// Caso contrário (INFERENCE/ASSUMPTION/HYPOTHESIS/UNKNOWN, ou EVIDENCE fraca)
// a afirmação deve ser sinalizada como suposição/desconhecida no contexto —
// nunca apresentada como fato.
func (c ClaimRecord) IsTrustworthy() bool {
	switch c.Kind {
	case ClaimFact:
		return true
	case ClaimEvidence:
		return c.Confidence >= 0.7
	default:
		return false
	}
}

// ── ClaimStore ───────────────────────────────────────────────────────────────

// ID prefix and zero-padding for claim records (CL-0001, width 4).
const (
	ClaimIDPrefix = "CL-"
	ClaimIDWidth  = 4
)

// ClaimStore é um armazenamento thread-safe, SQLite-backed, de afirmações
// classificadas (.cosca/claims.db). Espelha o estilo do ConflictStore.
type ClaimStore struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// NewClaimStore abre ou cria a base de claims no caminho dado (normalmente
// <projeto>/.cosca/claims.db). Garante o diretório pai, cria a tabela claims
// (auto-migração idempotente) e configura WAL.
func NewClaimStore(dbPath string) (*ClaimStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create claim db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open claim database: %w", err)
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

	s := &ClaimStore{db: db, dbPath: dbPath}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate claim database: %w", err)
	}
	return s, nil
}

// migrate cria a tabela claims e os índices se não existirem. Colunas de
// lista são armazenadas como JSON text (paridade com decision_log).
func (s *ClaimStore) migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS claims (
		id               TEXT PRIMARY KEY,
		kind             TEXT NOT NULL DEFAULT '',
		statement        TEXT NOT NULL DEFAULT '',
		supports         TEXT NOT NULL DEFAULT '[]',
		contradicted_by  TEXT NOT NULL DEFAULT '[]',
		confidence       REAL NOT NULL DEFAULT 0,
		created_at       TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_claims_kind       ON claims(kind);
	CREATE INDEX IF NOT EXISTS idx_claims_created_at ON claims(created_at);
	`

	_, err := s.db.Exec(ddl)
	return err
}

// Add persiste uma nova afirmação. Se o ID estiver vazio, atribui a próxima
// sequencial (CL-0001, CL-0002, ...). Se o CreatedAt for zero, usa o momento
// atual. Um Kind inválido, uma Statement vazia e uma Confidence fora de [0,1]
// são rejeitados. Retorna o ID atribuído.
func (s *ClaimStore) Add(c ClaimRecord) (string, error) {
	if !c.Kind.Valid() {
		return "", fmt.Errorf("claims: tipo inválido %q", c.Kind)
	}
	if strings.TrimSpace(c.Statement) == "" {
		return "", fmt.Errorf("claims: statement é obrigatória")
	}
	if c.Confidence < 0 || c.Confidence > 1 {
		return "", fmt.Errorf("claims: confidence %v fora do intervalo [0,1]", c.Confidence)
	}
	if c.ID == "" {
		id, err := s.nextID()
		if err != nil {
			return "", err
		}
		c.ID = id
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	if c.Supports == nil {
		c.Supports = []string{}
	}
	if c.ContradictedBy == nil {
		c.ContradictedBy = []string{}
	}

	supports, err := json.Marshal(c.Supports)
	if err != nil {
		return "", fmt.Errorf("claims: marshal supports: %w", err)
	}
	contradicted, err := json.Marshal(c.ContradictedBy)
	if err != nil {
		return "", fmt.Errorf("claims: marshal contradicted_by: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.db.Exec(
		`INSERT INTO claims
		 (id, kind, statement, supports, contradicted_by, confidence, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, string(c.Kind), c.Statement, string(supports), string(contradicted),
		c.Confidence, claimFmtTime(c.CreatedAt),
	)
	if err != nil {
		return "", fmt.Errorf("claims: adicionar %s: %w", c.ID, err)
	}
	return c.ID, nil
}

// Get recupera uma afirmação pelo ID normalizado (CL-0001, cl-7, "7").
// Retorna nil, nil quando a afirmação não existe.
func (s *ClaimStore) Get(id string) (*ClaimRecord, error) {
	norm, err := NormalizeClaimID(id)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var c ClaimRecord
	var kind, supports, contradicted, createdAt string
	err = s.db.QueryRow(
		`SELECT id, kind, statement, supports, contradicted_by, confidence, created_at
		 FROM claims WHERE id = ?`,
		norm,
	).Scan(&c.ID, &kind, &c.Statement, &supports, &contradicted, &c.Confidence, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claims: ler %s: %w", norm, err)
	}

	c.Kind = ClaimKind(kind)
	if err := json.Unmarshal([]byte(supports), &c.Supports); err != nil {
		return nil, fmt.Errorf("claims: parse supports: %w", err)
	}
	if err := json.Unmarshal([]byte(contradicted), &c.ContradictedBy); err != nil {
		return nil, fmt.Errorf("claims: parse contradicted_by: %w", err)
	}
	c.CreatedAt = claimParseTime(createdAt)
	return &c, nil
}

// List retorna as afirmações mais recentes (created_at DESC, id DESC),
// limitadas ao valor dado. Limit <= 0 assume 20; > 1000 é truncado.
func (s *ClaimStore) List(limit int) ([]ClaimRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, kind, statement, supports, contradicted_by, confidence, created_at
		 FROM claims ORDER BY created_at DESC, id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("claims: listar: %w", err)
	}
	defer rows.Close()

	records := make([]ClaimRecord, 0)
	for rows.Next() {
		var c ClaimRecord
		var kind, supports, contradicted, createdAt string
		if err := rows.Scan(&c.ID, &kind, &c.Statement, &supports, &contradicted,
			&c.Confidence, &createdAt); err != nil {
			return nil, fmt.Errorf("claims: scan: %w", err)
		}
		c.Kind = ClaimKind(kind)
		if err := json.Unmarshal([]byte(supports), &c.Supports); err != nil {
			return nil, fmt.Errorf("claims: parse supports: %w", err)
		}
		if err := json.Unmarshal([]byte(contradicted), &c.ContradictedBy); err != nil {
			return nil, fmt.Errorf("claims: parse contradicted_by: %w", err)
		}
		c.CreatedAt = claimParseTime(createdAt)
		records = append(records, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claims: iterar: %w", err)
	}
	if records == nil {
		records = make([]ClaimRecord, 0)
	}
	return records, nil
}

// ClassifyAndAdd classifica a afirmação (regras determinísticas, sem LLM) e a
// persiste num único passo. Devolve o registro completo com o ID atribuído.
func (s *ClaimStore) ClassifyAndAdd(statement string, evidenceCount int, validated bool, derived bool, tested bool, confidence float64) (ClaimRecord, error) {
	kind := Classify(statement, evidenceCount, validated, derived, tested)
	id, err := s.Add(ClaimRecord{
		Kind:       kind,
		Statement:  statement,
		Confidence: confidence,
	})
	if err != nil {
		return ClaimRecord{}, err
	}
	rec, err := s.Get(id)
	if err != nil {
		return ClaimRecord{}, err
	}
	if rec == nil {
		return ClaimRecord{}, fmt.Errorf("claims: %s não encontrada após add", id)
	}
	return *rec, nil
}

// Close encerra a conexão após um WAL checkpoint (paridade com ConflictStore).
func (s *ClaimStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		return s.db.Close()
	}
	return nil
}

// nextID retorna o próximo ID sequencial (CL-0001 quando a tabela está vazia),
// varrendo o maior número já usado em claims — IDs nunca são reutilizados.
func (s *ClaimStore) nextID() (string, error) {
	var max int64
	err := s.db.QueryRow(
		"SELECT COALESCE(MAX(CAST(SUBSTR(id, ?) AS INTEGER)), 0) FROM claims",
		len(ClaimIDPrefix)+1,
	).Scan(&max)
	if err != nil {
		return "", fmt.Errorf("claims: next id: %w", err)
	}
	return NextClaimID(int(max) + 1), nil
}

// NextClaimID formata o número dado como ID de claim zero-padded (largura 4):
// NextClaimID(7) → "CL-0007", NextClaimID(1024) → "CL-1024".
func NextClaimID(n int) string {
	return fmt.Sprintf("%s%0*d", ClaimIDPrefix, ClaimIDWidth, n)
}

// NormalizeClaimID aceita "CL-0007", "cl-7", "CL-07" e "7" e devolve a forma
// canônica zero-padded "CL-0007". Erro para entradas não numéricas.
func NormalizeClaimID(id string) (string, error) {
	digits := strings.TrimPrefix(strings.TrimSpace(strings.ToUpper(id)), ClaimIDPrefix)
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return "", fmt.Errorf("knowledge: ID de afirmação inválido %q (esperava CL-0001)", id)
	}
	return NextClaimID(n), nil
}

// ── time helpers ─────────────────────────────────────────────────────────────

// claimFmtTime serializa um time.Time como RFC3339Nano UTC (texto legível no
// SQLite), permitindo ordenação determinística.
func claimFmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// claimParseTime reconstrói um time.Time a partir do texto RFC3339(Nano);
// devolve zero time para entradas vazias/inválidas.
func claimParseTime(s string) time.Time {
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
