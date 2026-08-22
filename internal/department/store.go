// Conversation ledger (trilha de auditoria Dept→Dept) — armazenamento SQLite
// append-only em .cosca/department.db.
//
// Espelha o estilo do trace.Store (internal/trace): cria a base e a tabela
// `conversation_messages` com auto-migração no open, configura WAL e expõe
// apenas operações de escrita append (Send) e leitura (Thread/List). NÃO
// existe Update nem Delete — o diálogo nunca reescreve o passado.
package department

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Import SQLite driver for conversation storage
)

// messageIDRe é o padrão canônico de um ID de mensagem entre departamentos.
// Aceita tanto o sufixo legado de 4 hex chars quanto o novo de 16 hex chars
// (64 bits de entropia), para não invalidar mensagens antigas já persistidas.
var messageIDRe = regexp.MustCompile(`^DM-\d{8}-[0-9A-Fa-f]{4}(?:[0-9A-Fa-f]{12})?$`)

// NewMessageID gera um novo ID de mensagem: "DM-20260802-7F92AB3C4D5E6F".
// O sufixo são 16 hex chars (8 bytes, 64 bits) derivados de crypto/rand —
// entropia suficiente para unicidade sob o paradoxo do aniversário (bug
// pré-existente de 16 bits colidia em testes com 100+ IDs). Se o RNG falhar
// (extremamente raro), cai para uma derivação do relógio — nunca bloqueia.
func NewMessageID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		n := uint64(time.Now().UnixNano())
		for i := range b {
			b[i] = byte(n >> (8 * (len(b) - 1 - i)))
		}
	}
	return fmt.Sprintf("DM-%s-%X",
		time.Now().UTC().Format("20060102"), b)
}

// ConversationStore é o ledger append-only das mensagens entre departamentos,
// SQLite-backed.
type ConversationStore struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// NewConversationStore abre ou cria a base no caminho dado (normalmente
// <projeto>/.cosca/department.db). Garante a existência do diretório pai, cria
// a tabela conversation_messages (auto-migração idempotente) e configura WAL.
func NewConversationStore(dbPath string) (*ConversationStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create department db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open department database: %w", err)
	}

	// Configura SQLite como no trace.Store.
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

	s := &ConversationStore{db: db, dbPath: dbPath}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate department database: %w", err)
	}

	return s, nil
}

// migrate cria a tabela conversation_messages e os índices se não existirem
// (auto-migração no open). A tabela é append-only por contrato: as operações
// expostas são apenas INSERT e SELECT.
func (s *ConversationStore) migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS conversation_messages (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		msg_id     TEXT NOT NULL UNIQUE,
		from_dept  TEXT NOT NULL,
		to_dept    TEXT NOT NULL,
		topic      TEXT NOT NULL DEFAULT '',
		message    TEXT NOT NULL DEFAULT '',
		kind       TEXT NOT NULL DEFAULT 'question',
		created_at INTEGER NOT NULL,
		thread_id  TEXT NOT NULL,
		trace_id   TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_conversation_thread   ON conversation_messages(thread_id);
	CREATE INDEX IF NOT EXISTS idx_conversation_created  ON conversation_messages(created_at);
	`

	_, err := s.db.Exec(ddl)
	return err
}

// Send persiste uma mensagem no ledger (append-only). Se ID estiver vazio,
// gera um novo DM-YYYYMMDD-<hex16>; se CreatedAt for zero, assume o momento
// atual (UTC). Retorna o ID da mensagem gravada. Send é a ÚNICA operação de
// escrita — nunca há UPDATE/DELETE.
func (s *ConversationStore) Send(m ConversationMessage) (string, error) {
	if m.ID == "" {
		m.ID = NewMessageID()
	}
	if !messageIDRe.MatchString(m.ID) {
		return "", fmt.Errorf("department: ID de mensagem inválido %q (esperava DM-YYYYMMDD-<hex4 ou hex16>)", m.ID)
	}
	if m.From == "" || m.To == "" {
		return "", fmt.Errorf("department: remetente (from) e destinatário (to) são obrigatórios")
	}
	if strings.TrimSpace(m.Message) == "" {
		return "", fmt.Errorf("department: mensagem vazia")
	}
	if strings.TrimSpace(m.ThreadID) == "" {
		return "", fmt.Errorf("department: thread_id é obrigatório")
	}
	if m.Kind == "" {
		m.Kind = "question"
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO conversation_messages
		 (msg_id, from_dept, to_dept, topic, message, kind, created_at, thread_id, trace_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.From.String(), m.To.String(), m.Topic, m.Message, m.Kind,
		m.CreatedAt.Unix(), m.ThreadID, m.TraceID,
	)
	if err != nil {
		return "", fmt.Errorf("append conversation message: %w", err)
	}
	return m.ID, nil
}

// Thread recupera todas as mensagens de uma thread, ordenadas por created_at
// (ascendente) e, em empate, por ordem de inserção — a trilha de auditoria
// exata do diálogo. Retorna slice vazio (não nil) quando não há mensagens.
func (s *ConversationStore) Thread(threadID string) ([]ConversationMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT msg_id, from_dept, to_dept, topic, message, kind, created_at, thread_id, trace_id
		 FROM conversation_messages WHERE thread_id = ?
		 ORDER BY created_at ASC, id ASC`,
		threadID,
	)
	if err != nil {
		return nil, fmt.Errorf("query thread messages: %w", err)
	}
	defer rows.Close()

	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

// List retorna as mensagens mais recentes de todas as threads (created_at
// DESC), limitadas ao valor dado. Limit <= 0 assume 20; > 1000 é truncado.
func (s *ConversationStore) List(limit int) ([]ConversationMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT msg_id, from_dept, to_dept, topic, message, kind, created_at, thread_id, trace_id
		 FROM conversation_messages
		 ORDER BY created_at DESC, id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query latest conversation messages: %w", err)
	}
	defer rows.Close()

	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

// Count retorna o total de mensagens registradas no ledger.
func (s *ConversationStore) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM conversation_messages").Scan(&total); err != nil {
		return 0, fmt.Errorf("count conversation messages: %w", err)
	}
	return total, nil
}

// DBPath devolve o caminho da base (usado na saída dos comandos).
func (s *ConversationStore) DBPath() string {
	return s.dbPath
}

// Close encerra a conexão após um WAL checkpoint (paridade com trace.Store).
func (s *ConversationStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		return s.db.Close()
	}
	return nil
}

// scanMessages converte as linhas do SELECT canônico em []ConversationMessage.
func scanMessages(rows *sql.Rows) ([]ConversationMessage, error) {
	msgs := make([]ConversationMessage, 0)
	for rows.Next() {
		var m ConversationMessage
		var created int64
		if err := rows.Scan(
			&m.ID, &m.From, &m.To, &m.Topic, &m.Message, &m.Kind,
			&created, &m.ThreadID, &m.TraceID,
		); err != nil {
			return nil, fmt.Errorf("scan conversation message: %w", err)
		}
		m.CreatedAt = time.Unix(created, 0).UTC()
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversation messages: %w", err)
	}
	if msgs == nil {
		msgs = make([]ConversationMessage, 0)
	}
	return msgs, nil
}
