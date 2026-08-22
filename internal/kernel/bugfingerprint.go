package kernel

// Bug Fingerprint — agrupar bugs iguais por campos estruturados.
//
// Quando o mesmo tipo de bug aparecer novamente, o Cosca NÃO deve depender da
// descrição humana para reconhecê-lo. Cada ocorrência carrega um fingerprint
// estruturado (component, error, path, phase) que identifica o bug
// independentemente da mensagem:
//
//	BUG-FP: runtime.backup, error=permission_denied, path=/backups, phase=snapshot
//
// BUG-184, BUG-201 e BUG-244 podem assim ser reconhecidos como a mesma família
// de fingerprint mesmo que as mensagens sejam diferentes.
//
// O fingerprint e as famílias vivem em .cosca/bug.db (SQLite), espelhando o
// estilo do audit/decision store. A tabela é deduplicada por fingerprint: a
// MESMA assinatura reaparece → TimesSeen++ e LastSeen atualizado (nunca um
// registro duplicado).

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Import SQLite driver for bug fingerprint storage
)

// Bug ID prefix and zero-padding for bug records (BUG-0001).
const (
	BugIDPrefix = "BUG-"
	BugIDWidth  = 4
)

// Status possíveis de um bug.
const (
	BugStatusOpen     = "open"     // bug visto e aguardando correção
	BugStatusFixed    = "fixed"    // corrigido
	BugStatusArchived = "archived" // arquivado (não mais ativo)
)

// BugFingerprint é a assinatura estruturada de um bug: a identidade do bug
// NÃO é a descrição humana, e sim estes campos. Component e Error são sempre
// obrigatórios; Path e Phase são contexto opcional (omitempty).
type BugFingerprint struct {
	Component string `json:"component"`       // e.g. "runtime.backup"
	Error     string `json:"error"`           // e.g. "permission_denied"
	Path      string `json:"path,omitempty"`  // e.g. "/backups"
	Phase     string `json:"phase,omitempty"` // e.g. "snapshot"
}

// Key devolve a chave canônica do fingerprint — "component:error:path:phase"
// em minúsculas e com whitespace normalizado (múltiplos espaços/tabs viram um
// espaço único; trim nas pontas). É ESTA chave que identifica o bug, nunca a
// descrição humana.
func (f BugFingerprint) Key() string {
	return strings.Join([]string{
		normalizeFingerprintPart(f.Component),
		normalizeFingerprintPart(f.Error),
		normalizeFingerprintPart(f.Path),
		normalizeFingerprintPart(f.Phase),
	}, ":")
}

// FamilyKey devolve a chave da família do fingerprint — "component:error" —
// o núcleo obrigatório que agrupa bugs da mesma família. Bugs que compartilham
// a família podem divergir em path/phase (ou nas mensagens humanas).
func (f BugFingerprint) FamilyKey() string {
	return strings.Join([]string{
		normalizeFingerprintPart(f.Component),
		normalizeFingerprintPart(f.Error),
	}, ":")
}

// Matches reporta se dois fingerprints pertencem à MESMA família: component e
// error são sempre exigidos e comparados; path/phase só são comparados quando
// AMBOS os lados estão preenchidos (um lado vazio é curinga). Mensagens
// humanas não participam — BUG-184, BUG-201 e BUG-244 podem casar aqui mesmo
// com textos diferentes.
func (f BugFingerprint) Matches(other BugFingerprint) bool {
	if normalizeFingerprintPart(f.Component) != normalizeFingerprintPart(other.Component) {
		return false
	}
	if normalizeFingerprintPart(f.Error) != normalizeFingerprintPart(other.Error) {
		return false
	}
	if f.Path != "" && other.Path != "" && normalizeFingerprintPart(f.Path) != normalizeFingerprintPart(other.Path) {
		return false
	}
	if f.Phase != "" && other.Phase != "" && normalizeFingerprintPart(f.Phase) != normalizeFingerprintPart(other.Phase) {
		return false
	}
	return true
}

// BugRecord é o registro persistido de um bug (ou de uma família). ID é o
// BUG-XXXX; Family é a chave da família (component:error). TimesSeen cresce
// quando o MESMO fingerprint reaparece.
type BugRecord struct {
	ID          string         `json:"id"` // BUG-0001 auto-increment
	Title       string         `json:"title"`
	Fingerprint BugFingerprint `json:"fingerprint"`
	Family      string         `json:"family"` // chave canônica component:error
	Status      string         `json:"status"` // open | fixed | archived
	TimesSeen   int            `json:"times_seen"`
	FirstSeen   time.Time      `json:"first_seen"`
	LastSeen    time.Time      `json:"last_seen"`
	Evidence    []string       `json:"evidence,omitempty"`
}

// BugStore é um armazenamento thread-safe, SQLite-backed, dos bugs e suas
// famílias (.cosca/bug.db). Espelha o estilo do audit/decision/gate store.
type BugStore struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// NewBugStore abre ou cria a base de bugs no caminho dado (normalmente
// <projeto>/.cosca/bug.db). Garante o diretório pai, cria a tabela
// bug_records (auto-migração idempotente) e configura WAL.
func NewBugStore(dbPath string) (*BugStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create bug db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open bug database: %w", err)
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

	s := &BugStore{db: db, dbPath: dbPath}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate bug database: %w", err)
	}
	return s, nil
}

// migrate cria a tabela bug_records e os índices se não existirem. A evidência
// é uma coluna JSON; fingerprint e family são chaves canônicas normalizadas.
func (s *BugStore) migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS bug_records (
		id          TEXT PRIMARY KEY,
		title       TEXT NOT NULL DEFAULT '',
		component   TEXT NOT NULL DEFAULT '',
		error       TEXT NOT NULL DEFAULT '',
		path        TEXT NOT NULL DEFAULT '',
		phase       TEXT NOT NULL DEFAULT '',
		fingerprint TEXT NOT NULL DEFAULT '',
		family      TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'open',
		times_seen  INTEGER NOT NULL DEFAULT 1,
		first_seen  TEXT NOT NULL DEFAULT '',
		last_seen   TEXT NOT NULL DEFAULT '',
		evidence    TEXT NOT NULL DEFAULT '[]'
	);

	CREATE INDEX IF NOT EXISTS idx_bug_family ON bug_records(family);
	CREATE INDEX IF NOT EXISTS idx_bug_fingerprint ON bug_records(fingerprint);
	CREATE INDEX IF NOT EXISTS idx_bug_last_seen ON bug_records(last_seen);
	`

	_, err := s.db.Exec(ddl)
	return err
}

// Register registra um bug. Atribui um ID BUG-XXXX sequencial quando o
// fingerprint é novo. Se um registro com o MESMO fingerprint já existir,
// NÃO duplica: incrementa TimesSeen, atualiza LastSeen (e o título, quando o
// novo título não for vazio) e devolve o ID existente.
func (s *BugStore) Register(b BugRecord) (string, error) {
	if strings.TrimSpace(b.Fingerprint.Component) == "" {
		return "", fmt.Errorf("bug: component é obrigatório")
	}
	if strings.TrimSpace(b.Fingerprint.Error) == "" {
		return "", fmt.Errorf("bug: error é obrigatório")
	}

	b.Fingerprint.Component = normalizeFingerprintPart(b.Fingerprint.Component)
	b.Fingerprint.Error = normalizeFingerprintPart(b.Fingerprint.Error)
	b.Fingerprint.Path = normalizeFingerprintPart(b.Fingerprint.Path)
	b.Fingerprint.Phase = normalizeFingerprintPart(b.Fingerprint.Phase)

	if b.Status == "" {
		b.Status = BugStatusOpen
	}
	if b.Status != BugStatusOpen && b.Status != BugStatusFixed && b.Status != BugStatusArchived {
		return "", fmt.Errorf("bug: status inválido %q (esperava open, fixed ou archived)", b.Status)
	}
	if b.TimesSeen < 1 {
		b.TimesSeen = 1
	}
	now := time.Now().UTC()
	if b.FirstSeen.IsZero() {
		b.FirstSeen = now
	}
	b.LastSeen = now
	b.Family = b.Fingerprint.FamilyKey()
	fpKey := b.Fingerprint.Key()

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.getByFingerprintLocked(fpKey)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return s.revisitLocked(existing, b)
	}

	id, err := s.nextIDLocked()
	if err != nil {
		return "", err
	}
	evJSON, err := json.Marshal(b.Evidence)
	if err != nil {
		return "", fmt.Errorf("bug: serializar evidência: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO bug_records
		   (id, title, component, error, path, phase, fingerprint, family, status, times_seen, first_seen, last_seen, evidence)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, b.Title, b.Fingerprint.Component, b.Fingerprint.Error, b.Fingerprint.Path,
		b.Fingerprint.Phase, fpKey, b.Family, b.Status, b.TimesSeen,
		fmtBugTime(b.FirstSeen), fmtBugTime(b.LastSeen), string(evJSON),
	)
	if err != nil {
		return "", fmt.Errorf("bug: registrar %s: %w", id, err)
	}
	return id, nil
}

// revisitLocked atualiza um registro existente após reincidência do MESMO
// fingerprint: TimesSeen++, LastSeen = agora. A chamadora já detém o lock.
func (s *BugStore) revisitLocked(existing *BugRecord, b BugRecord) (string, error) {
	now := time.Now().UTC()
	times := existing.TimesSeen + 1

	title := existing.Title
	if strings.TrimSpace(b.Title) != "" {
		title = b.Title
	}
	evidence := existing.Evidence
	if len(b.Evidence) > 0 {
		evidence = append(evidence, b.Evidence...)
	}

	_, err := s.db.Exec(
		`UPDATE bug_records
		    SET times_seen = ?, last_seen = ?, title = ?, evidence = ?
		  WHERE id = ?`,
		times, fmtBugTime(now), title, mustJSON(evidence), existing.ID,
	)
	if err != nil {
		return "", fmt.Errorf("bug: revisitar %s: %w", existing.ID, err)
	}
	return existing.ID, nil
}

// Match devolve os bugs da MESMA família do fingerprint dado: component+error
// iguais, path/phase comparados apenas quando ambos preenchidos. Ordenados do
// mais recente (last_seen DESC) para o mais antigo.
func (s *BugStore) Match(fp BugFingerprint) ([]BugRecord, error) {
	return s.Family(fp.FamilyKey())
}

// Family devolve todos os bugs da família de chave canônica component:error
// (ex.: "runtime.backup:permission_denied"). A chave é normalizada antes da
// consulta. Ordenados do mais recente para o mais antigo.
func (s *BugStore) Family(key string) ([]BugRecord, error) {
	key = normalizeFamilyKey(key)
	if key == ":" {
		return nil, fmt.Errorf("bug: família inválida %q (esperava component:error)", key)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, title, component, error, path, phase, fingerprint, family,
		        status, times_seen, first_seen, last_seen, evidence
		   FROM bug_records WHERE family = ?
		  ORDER BY last_seen DESC, id DESC`,
		key,
	)
	if err != nil {
		return nil, fmt.Errorf("bug: família %s: %w", key, err)
	}
	defer rows.Close()

	records := make([]BugRecord, 0)
	for rows.Next() {
		rec, err := scanBugRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bug: iterar família %s: %w", key, err)
	}
	if records == nil {
		records = make([]BugRecord, 0)
	}
	return records, nil
}

// List devolve todos os bugs, do mais recente (last_seen DESC) para o mais
// antigo.
func (s *BugStore) List() ([]BugRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, title, component, error, path, phase, fingerprint, family,
		        status, times_seen, first_seen, last_seen, evidence
		   FROM bug_records
		  ORDER BY last_seen DESC, id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("bug: listar: %w", err)
	}
	defer rows.Close()

	records := make([]BugRecord, 0)
	for rows.Next() {
		rec, err := scanBugRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bug: iterar: %w", err)
	}
	if records == nil {
		records = make([]BugRecord, 0)
	}
	return records, nil
}

// Get recupera um bug pelo ID. Retorna nil, nil quando o bug não existe.
func (s *BugStore) Get(id string) (*BugRecord, error) {
	norm, err := NormalizeBugID(id)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getLocked(norm)
}

// Close encerra a conexão após um WAL checkpoint (paridade com o gate store).
func (s *BugStore) Close() error {
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

// scanner abstracts *sql.Row and *sql.Rows for scanBugRecord.
type scanner interface {
	Scan(dest ...interface{}) error
}

// scanBugRecord lê uma linha de bug_records (a projeção da query padrão) para
// um BugRecord. A coluna fingerprint não é re-materializada — o fingerprint é
// derivado das colunas component/error/path/phase.
func scanBugRecord(row scanner) (BugRecord, error) {
	var rec BugRecord
	var firstSeen, lastSeen, evidence, fingerprint string
	err := row.Scan(
		&rec.ID, &rec.Title,
		&rec.Fingerprint.Component, &rec.Fingerprint.Error,
		&rec.Fingerprint.Path, &rec.Fingerprint.Phase,
		&fingerprint,
		&rec.Family, &rec.Status,
		&rec.TimesSeen, &firstSeen, &lastSeen, &evidence,
	)
	if err != nil {
		return BugRecord{}, fmt.Errorf("bug: scan: %w", err)
	}
	rec.FirstSeen = parseBugTime(firstSeen)
	rec.LastSeen = parseBugTime(lastSeen)
	if err := json.Unmarshal([]byte(evidence), &rec.Evidence); err != nil {
		return BugRecord{}, fmt.Errorf("bug: parsear evidência de %s: %w", rec.ID, err)
	}
	if rec.Evidence == nil {
		rec.Evidence = make([]string, 0)
	}
	return rec, nil
}

// getLocked lê um registro assumindo que o lock já foi adquirido.
func (s *BugStore) getLocked(id string) (*BugRecord, error) {
	row := s.db.QueryRow(
		`SELECT id, title, component, error, path, phase, fingerprint, family,
		        status, times_seen, first_seen, last_seen, evidence
		   FROM bug_records WHERE id = ?`,
		id,
	)
	rec, err := scanBugRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// getByFingerprintLocked procura um registro pelo fingerprint canônico.
// Devolve nil, nil quando ainda não existe. A chamadora deve segurar o lock.
func (s *BugStore) getByFingerprintLocked(fpKey string) (*BugRecord, error) {
	row := s.db.QueryRow(
		`SELECT id, title, component, error, path, phase, fingerprint, family,
		        status, times_seen, first_seen, last_seen, evidence
		   FROM bug_records WHERE fingerprint = ?`,
		fpKey,
	)
	rec, err := scanBugRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// nextIDLocked retorna o próximo ID sequencial (BUG-0001 quando a tabela está
// vazia), varrendo o maior número já usado — IDs nunca são reutilizados.
func (s *BugStore) nextIDLocked() (string, error) {
	var max int64
	err := s.db.QueryRow(
		"SELECT COALESCE(MAX(CAST(SUBSTR(id, ?) AS INTEGER)), 0) FROM bug_records",
		len(BugIDPrefix)+1,
	).Scan(&max)
	if err != nil {
		return "", fmt.Errorf("bug: next id: %w", err)
	}
	return NextBugID(int(max) + 1), nil
}

// NextBugID formata o número dado como ID de bug zero-padded (largura 4):
// NextBugID(7) → "BUG-0007".
func NextBugID(n int) string {
	return fmt.Sprintf("%s%0*d", BugIDPrefix, BugIDWidth, n)
}

// NormalizeBugID aceita "BUG-0001", "bug-0001", "BUG-1" e "0001" e devolve a
// forma canônica zero-padded "BUG-0001". Erro para entradas não numéricas.
func NormalizeBugID(id string) (string, error) {
	digits := strings.TrimPrefix(strings.TrimSpace(strings.ToUpper(id)), BugIDPrefix)
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return "", fmt.Errorf("bug: ID inválido %q (esperava BUG-0001)", id)
	}
	return NextBugID(n), nil
}

// normalizeFingerprintPart normaliza um componente do fingerprint: minúsculas,
// trim e colapso de whitespace (múltiplos espaços/tabs/linhas viram um espaço
// único).
func normalizeFingerprintPart(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// normalizeFamilyKey normaliza a chave de família passada pelo usuário,
// garantindo que component e error estejam em minúsculas sem espaços soltos.
func normalizeFamilyKey(key string) string {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return ":" + strings.TrimSpace(key)
	}
	return normalizeFingerprintPart(parts[0]) + ":" + normalizeFingerprintPart(parts[1])
}

// mustJSON serializa a evidência para a coluna JSON; em caso de erro devolve
// "[]" (melhor perder a evidência nova do que quebrar o registro).
func mustJSON(v []string) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// fmtBugTime serializa um time.Time como RFC3339Nano UTC (texto legível no
// SQLite). Nano assegura ordenação determinística (last_seen DESC).
func fmtBugTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// parseBugTime reconstrói um time.Time a partir do texto RFC3339(Nano);
// devolve zero time para entradas vazias/inválidas.
func parseBugTime(s string) time.Time {
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
