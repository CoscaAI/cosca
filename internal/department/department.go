// Package department implements inter-department conversations (Dept→Dept)
// with a full audit trail — the "Departamentos conversam" feature approved by
// the Don.
//
// A department does not need to trust another blindly: any department can ask
// a question and receive answers with evidence. The conversation is recorded
// in an APPEND-ONLY SQLite ledger (.cosca/department.db) — who asked, who
// answered, what was decided — so Security can audit Developer.
//
// The flow mirrors the Don example:
//
//	Executive asks → Security answers ("há 2 riscos críticos") → Developer
//	answers ("um deles já foi corrigido") → Security validates → Executive
//	resolves ("release aprovado").
//
// Every message belongs to a thread (<topic>:YYYYMMDD) that groups the whole
// conversation and carries an optional TraceID linking it to the operation
// trace (TRACE-YYYYMMDD-XXXX).
package department

import (
	"fmt"
	"strings"
	"time"
)

// DepartmentID identifies a department in the organization ("developer",
// "security", "executive", ...). Always lowercase.
type DepartmentID string

// String devolve a representação textual do departamento.
func (d DepartmentID) String() string { return string(d) }

// KnownDepartments é a lista estática de departamentos que podem conversar.
// O Don aprovou esta lista fixa — sem depender do ORGCHART.
var KnownDepartments = []DepartmentID{
	"developer", "security", "executive", "research", "operations", "kernel",
}

// ValidDepartment valida um departamento contra a lista estática conhecida.
// Retorna um erro em pt-BR para qualquer nome fora da lista.
func ValidDepartment(id DepartmentID) error {
	for _, known := range KnownDepartments {
		if id == known {
			return nil
		}
	}
	known := make([]string, 0, len(KnownDepartments))
	for _, k := range KnownDepartments {
		known = append(known, string(k))
	}
	return fmt.Errorf("departamento desconhecido %q — conhecidos: %s",
		string(id), strings.Join(known, ", "))
}

// ConversationMessage é a unidade atômica do diálogo entre departamentos —
// uma mensagem com remetente, destinatário, tópico e tipo. A trilha de
// auditoria é a sequência ordenada de mensagens de uma thread.
type ConversationMessage struct {
	ID        string       `json:"id"`                 // DM-YYYYMMDD-<hex16>
	From      DepartmentID `json:"from"`               // departamento que envia
	To        DepartmentID `json:"to"`                 // departamento que recebe
	Topic     string       `json:"topic"`              // ex.: "release-approval"
	Message   string       `json:"message"`            // o conteúdo do diálogo
	Kind      string       `json:"kind"`               // "question" | "answer" | "request" | "approval" | "evidence"
	CreatedAt time.Time    `json:"created_at"`         // momento do envio (UTC)
	ThreadID  string       `json:"thread_id"`          // agrupa a conversa (ex.: release-approval:20260802)
	TraceID   string       `json:"trace_id,omitempty"` // link para o trace da operação
}

// ThreadKey monta a chave de uma conversa a partir do tópico e da data (UTC):
// "<topic>:YYYYMMDD". O mesmo tópico no mesmo dia continua a mesma thread —
// este é o esquema de agrupamento do Don (ex.: release-approval:20260802).
func ThreadKey(topic string, t time.Time) string {
	return fmt.Sprintf("%s:%s", topic, t.UTC().Format("20060102"))
}

// Director orquestra o fluxo "Security audita Developer": um departamento
// pergunta (Ask), o outro responde com evidência (Answer) e a decisão final é
// registrada (Resolve). Todas as mensagens ficam no ledger append-only.
type Director struct {
	store *ConversationStore
}

// NewDirector cria um Director sobre o store de conversas.
func NewDirector(store *ConversationStore) *Director {
	return &Director{store: store}
}

// Ask inicia uma conversa: <from> pergunta a <to> sobre <topic>. Cria a thread
// (<topic>:<data>) e registra a pergunta (kind "question"). Retorna o ID da
// mensagem (DM-YYYYMMDD-<hex16>) e a chave da thread.
func (d *Director) Ask(from, to DepartmentID, topic, message string) (string, string, error) {
	if err := ValidDepartment(from); err != nil {
		return "", "", err
	}
	if err := ValidDepartment(to); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(topic) == "" {
		return "", "", fmt.Errorf("departamento: tópico é obrigatório (ex.: --topic release-approval)")
	}
	thread := ThreadKey(topic, time.Now())
	msgID, err := d.store.Send(ConversationMessage{
		From:     from,
		To:       to,
		Topic:    topic,
		Message:  message,
		Kind:     "question",
		ThreadID: thread,
	})
	return msgID, thread, err
}

// Answer responde dentro de uma thread existente: <from> responde a quem
// enviou a última mensagem (determinado pelo ledger — sem confiança cega, o
// interlocutor é o último que falou). A mensagem é do tipo "answer"
// (evidência / contraprova).
func (d *Director) Answer(threadID, from, message string) error {
	return d.respond(threadID, from, message, "answer")
}

// Resolve registra a decisão final da thread: <from> decide e a mensagem é do
// tipo "approval" — o fechamento do diálogo ("release aprovado"). O
// destinatário também é determinado pelo último interlocutor do ledger.
func (d *Director) Resolve(threadID, from, decision string) error {
	return d.respond(threadID, from, decision, "approval")
}

// respond envia uma mensagem (kind dado) dentro de uma thread existente. O
// destinatário é o último interlocutor da thread que não seja <from> — a
// resposta volta para quem falou por último.
func (d *Director) respond(threadID, from, message, kind string) error {
	if err := ValidDepartment(DepartmentID(from)); err != nil {
		return err
	}
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("departamento: mensagem vazia (use --msg)")
	}

	msgs, err := d.store.Thread(threadID)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return fmt.Errorf("departamento: thread %q não encontrada — use \"cosca department ask\" primeiro", threadID)
	}

	to := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if string(msgs[i].From) != from {
			to = string(msgs[i].From)
			break
		}
	}
	if to == "" {
		return fmt.Errorf("departamento: sem interlocutor na thread %q para %q", threadID, from)
	}

	_, err = d.store.Send(ConversationMessage{
		From:     DepartmentID(from),
		To:       DepartmentID(to),
		Topic:    msgs[0].Topic,
		Message:  message,
		Kind:     kind,
		ThreadID: threadID,
	})
	return err
}
