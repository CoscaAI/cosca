// Tests for the inter-department conversations package (internal/department).
//
// Covers:
//   - Send assigns DM-XXXX (format DM-YYYYMMDD-XXXX); Thread returns ordered
//     messages; List (DESC); Count
//   - Ask creates a thread (topic+date key); Answer appends; Resolve appends
//   - Unknown department → error pt-BR
//   - Append-only (no Update/Delete exposed); reopen persists
//   - Concurrent Send (thread-safe)

package department

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// ID helpers / validation
// =============================================================================

func TestNewMessageID_Format(t *testing.T) {
	re := regexp.MustCompile(`^DM-\d{8}-[0-9A-F]{16}$`)
	for i := 0; i < 50; i++ {
		id := NewMessageID()
		if !re.MatchString(id) {
			t.Fatalf("NewMessageID() format inválido: %q", id)
		}
		datePart := strings.TrimPrefix(id, "DM-")[:8]
		if len(datePart) != 8 {
			t.Fatalf("NewMessageID() data inesperada: %q", id)
		}
	}
}

func TestNewMessageID_Uniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := NewMessageID()
		if seen[id] {
			t.Fatalf("NewMessageID() colidiu: %s", id)
		}
		seen[id] = true
	}
}

func TestValidDepartment(t *testing.T) {
	for _, known := range KnownDepartments {
		if err := ValidDepartment(known); err != nil {
			t.Errorf("ValidDepartment(%q) deveria ser válido: %v", known, err)
		}
	}
	// Erro em pt-BR para departamentos fora da lista estática.
	err := ValidDepartment("finance")
	if err == nil {
		t.Fatal("ValidDepartment(finance) deveria falhar")
	}
	if !strings.Contains(err.Error(), "departamento desconhecido") {
		t.Errorf("erro deveria ser em pt-BR: %v", err)
	}
	if !strings.Contains(err.Error(), "developer") || !strings.Contains(err.Error(), "security") {
		t.Errorf("erro deveria listar os conhecidos: %v", err)
	}
}

func TestThreadKey(t *testing.T) {
	key := ThreadKey("release-approval", time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC))
	if key != "release-approval:20260802" {
		t.Errorf("ThreadKey() = %q, esperava release-approval:20260802", key)
	}
	// Mesmo tópico no mesmo dia → mesma thread.
	if key != ThreadKey("release-approval", time.Date(2026, 8, 2, 23, 59, 0, 0, time.UTC)) {
		t.Errorf("mesmo tópico no mesmo dia deveria manter a mesma thread")
	}
	// Dia diferente → thread diferente.
	if key == ThreadKey("release-approval", time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("dias diferentes deveriam criar threads diferentes")
	}
}

// =============================================================================
// Store — Send/Thread/List, append-only
// =============================================================================

func newTestStore(t *testing.T) *ConversationStore {
	t.Helper()
	s, err := NewConversationStore(filepath.Join(t.TempDir(), "department.db"))
	if err != nil {
		t.Fatalf("NewConversationStore() error: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStore_SendAssignsDMIDAndThreadOrdered(t *testing.T) {
	s := newTestStore(t)
	thread := "release-approval:20260802"

	// Envia fora de ordem de timestamp para provar que Thread ordena por tempo.
	msgs := []ConversationMessage{
		{From: "executive", To: "security", Topic: "release-approval", Message: "podemos liberar?", Kind: "question", ThreadID: thread, CreatedAt: time.Unix(100, 0)},
		{From: "security", To: "executive", Topic: "release-approval", Message: "há 2 riscos críticos", Kind: "answer", ThreadID: thread, CreatedAt: time.Unix(300, 0)},
		{From: "developer", To: "security", Topic: "release-approval", Message: "um deles já foi corrigido", Kind: "answer", ThreadID: thread, CreatedAt: time.Unix(200, 0)},
	}
	for _, m := range msgs {
		id, err := s.Send(m)
		if err != nil {
			t.Fatalf("Send(%v): %v", m, err)
		}
	re := regexp.MustCompile(`^DM-\d{8}-[0-9A-F]{16}$`)
		if !re.MatchString(id) {
			t.Fatalf("Send() ID não segue DM-XXXX: %q", id)
		}
	}

	got, err := s.Thread(thread)
	if err != nil {
		t.Fatalf("Thread(): %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("Thread() len = %d, esperava 3", len(got))
	}
	// Ordenado por created_at: executive(100) → developer(200) → security(300),
	// independente da ordem de inserção.
	if got[0].From.String() != "executive" || got[1].From.String() != "developer" || got[2].From.String() != "security" {
		t.Fatalf("Thread() fora de ordem por created_at: %+v", got)
	}
	// Defaults aplicados no Send.
	for _, g := range got {
		if g.ThreadID != thread {
			t.Errorf("ThreadID divergente: %+v", g)
		}
		if g.CreatedAt.IsZero() {
			t.Errorf("CreatedAt deveria ser preenchido: %+v", g)
		}
		if g.Kind == "" {
			t.Errorf("Kind deveria ter default preenchido: %+v", g)
		}
	}
}

func TestStore_Thread_UnknownReturnsEmpty(t *testing.T) {
	s := newTestStore(t)
	got, err := s.Thread("nao-existe:20260101")
	if err != nil {
		t.Fatalf("Thread(): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Thread() de thread desconhecida deveria ser vazio, got %d", len(got))
	}
}

func TestStore_List(t *testing.T) {
	s := newTestStore(t)
	// Mensagens recentes: B (mais novo) → A2 → A1 (mais antigo).
	_, _ = s.Send(ConversationMessage{From: "executive", To: "security", Topic: "t1", Message: "a1", Kind: "question", ThreadID: "t1:20260802", CreatedAt: time.Unix(100, 0)})
	_, _ = s.Send(ConversationMessage{From: "security", To: "executive", Topic: "t1", Message: "a2", Kind: "answer", ThreadID: "t1:20260802", CreatedAt: time.Unix(200, 0)})
	_, _ = s.Send(ConversationMessage{From: "developer", To: "executive", Topic: "t2", Message: "b1", Kind: "question", ThreadID: "t2:20260802", CreatedAt: time.Unix(400, 0)})

	all, err := s.List(0)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("List() len = %d, esperava 3", len(all))
	}
	// DESC por created_at.
	if all[0].Message != "b1" || all[1].Message != "a2" || all[2].Message != "a1" {
		t.Fatalf("List() fora de ordem DESC: %+v", all)
	}

	two, err := s.List(2)
	if err != nil {
		t.Fatalf("List(2): %v", err)
	}
	if len(two) != 2 || two[0].Message != "b1" || two[1].Message != "a2" {
		t.Fatalf("List(2) inesperado: %+v", two)
	}
}

func TestStore_Count(t *testing.T) {
	s := newTestStore(t)
	if n, _ := s.Count(); n != 0 {
		t.Fatalf("Count() inicial = %d, esperava 0", n)
	}
	_, _ = s.Send(ConversationMessage{From: "developer", To: "security", Topic: "t", Message: "m", ThreadID: "t:20260802"})
	_, _ = s.Send(ConversationMessage{From: "security", To: "developer", Topic: "t", Message: "r", Kind: "answer", ThreadID: "t:20260802"})
	if n, _ := s.Count(); n != 2 {
		t.Fatalf("Count() = %d, esperava 2", n)
	}
}

func TestStore_Send_Validation(t *testing.T) {
	s := newTestStore(t)
	base := ConversationMessage{From: "developer", To: "security", Topic: "t", Message: "m", ThreadID: "t:20260802"}

	bad := []ConversationMessage{
		{From: "developer", To: "security", Topic: "t", Message: "m"},                                   // sem ThreadID
		{From: "developer", To: "security", Topic: "t", Message: "", ThreadID: "t:x"},                   // mensagem vazia
		{From: "", To: "security", Topic: "t", Message: "m", ThreadID: "t:x"},                           // sem from
		{From: "developer", To: "", Topic: "t", Message: "m", ThreadID: "t:x"},                          // sem to
		{ID: "NAO-EH-DM", From: "developer", To: "security", Topic: "t", Message: "m", ThreadID: "t:x"}, // ID inválido
	}
	for _, m := range bad {
		if _, err := s.Send(m); err == nil {
			t.Errorf("Send(%+v) deveria falhar", m)
		}
	}
	_ = base
}

func TestStore_AppendOnly(t *testing.T) {
	s := newTestStore(t)

	// O contrato append-only é garantido pela API: os únicos métodos de escrita
	// expostos são Send. Não deve existir Update nem Delete.
	if _, ok := any(s).(interface {
		Update(ConversationMessage) error
	}); ok {
		t.Fatal("ConversationStore não deveria expor Update")
	}
	if _, ok := any(s).(interface {
		Delete(string) error
	}); ok {
		t.Fatal("ConversationStore não deveria expor Delete")
	}
}

func TestStore_ReopenExisting(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "department.db")

	s1, err := NewConversationStore(dbPath)
	if err != nil {
		t.Fatalf("primeiro NewConversationStore(): %v", err)
	}
	thread := "release-approval:20260802"
	if _, err := s1.Send(ConversationMessage{From: "executive", To: "security", Topic: "release-approval", Message: "podemos liberar?", ThreadID: thread}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	_ = s1.Close()

	// Reabre a mesma base — as mensagens persistidas devem continuar lá.
	s2, err := NewConversationStore(dbPath)
	if err != nil {
		t.Fatalf("segundo NewConversationStore(): %v", err)
	}
	defer s2.Close()
	got, err := s2.Thread(thread)
	if err != nil {
		t.Fatalf("Thread() após reabertura: %v", err)
	}
	if len(got) != 1 || got[0].From.String() != "executive" {
		t.Fatalf("mensagens perdidas após reabertura: %+v", got)
	}
}

// =============================================================================
// Director — Ask/Answer/Resolve (fluxo "Security audita Developer")
// =============================================================================

func newTestDirector(t *testing.T) (*Director, *ConversationStore) {
	t.Helper()
	s := newTestStore(t)
	return NewDirector(s), s
}

func TestDirector_AskCreatesThread(t *testing.T) {
	d, s := newTestDirector(t)

	msgID, thread, err := d.Ask("developer", "security", "release-approval", "podemos liberar?")
	if err != nil {
		t.Fatalf("Ask(): %v", err)
	}
	re := regexp.MustCompile(`^DM-\d{8}-[0-9A-F]{16}$`)
	if !re.MatchString(msgID) {
		t.Fatalf("Ask() msgID inválido: %q", msgID)
	}
	// Thread key: <topic>:YYYYMMDD (hoje UTC).
	want := ThreadKey("release-approval", time.Now())
	if thread != want {
		t.Fatalf("Ask() thread = %q, esperava %q", thread, want)
	}

	msgs, err := s.Thread(thread)
	if err != nil {
		t.Fatalf("Thread(): %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("Thread() len = %d, esperava 1", len(msgs))
	}
	m := msgs[0]
	if m.From.String() != "developer" || m.To.String() != "security" || m.Kind != "question" {
		t.Fatalf("Ask() mensagem fora do esperado: %+v", m)
	}
	if m.Topic != "release-approval" || m.Message != "podemos liberar?" {
		t.Fatalf("Ask() tópico/mensagem fora do esperado: %+v", m)
	}
}

func TestDirector_AnswerAppends(t *testing.T) {
	d, s := newTestDirector(t)
	_, thread, err := d.Ask("developer", "security", "release-approval", "podemos liberar?")
	if err != nil {
		t.Fatalf("Ask(): %v", err)
	}

	if err := d.Answer(thread, "security", "há 2 riscos críticos"); err != nil {
		t.Fatalf("Answer(): %v", err)
	}

	msgs, err := s.Thread(thread)
	if err != nil {
		t.Fatalf("Thread(): %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("Thread() len = %d, esperava 2", len(msgs))
	}
	// A resposta volta para o último interlocutor (developer).
	ans := msgs[1]
	if ans.From.String() != "security" || ans.To.String() != "developer" || ans.Kind != "answer" {
		t.Fatalf("Answer() mensagem fora do esperado: %+v", ans)
	}
	if ans.Message != "há 2 riscos críticos" || ans.Topic != "release-approval" {
		t.Fatalf("Answer() conteúdo fora do esperado: %+v", ans)
	}
}

func TestDirector_ResolveAppends(t *testing.T) {
	d, s := newTestDirector(t)
	_, thread, err := d.Ask("executive", "security", "release-approval", "podemos liberar a versão?")
	if err != nil {
		t.Fatalf("Ask(): %v", err)
	}
	if err := d.Answer(thread, "security", "validação passou"); err != nil {
		t.Fatalf("Answer(): %v", err)
	}

	if err := d.Resolve(thread, "executive", "release aprovado"); err != nil {
		t.Fatalf("Resolve(): %v", err)
	}

	msgs, err := s.Thread(thread)
	if err != nil {
		t.Fatalf("Thread(): %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("Thread() len = %d, esperava 3", len(msgs))
	}
	// A decisão final volta para o último interlocutor (security).
	dec := msgs[2]
	if dec.From.String() != "executive" || dec.To.String() != "security" || dec.Kind != "approval" {
		t.Fatalf("Resolve() mensagem fora do esperado: %+v", dec)
	}
	if dec.Message != "release aprovado" {
		t.Fatalf("Resolve() decisão fora do esperado: %+v", dec)
	}
}

func TestDirector_UnknownDepartment(t *testing.T) {
	d, _ := newTestDirector(t)

	// Ask com departamento desconhecido → erro pt-BR.
	if _, _, err := d.Ask("finance", "security", "t", "m"); err == nil || !strings.Contains(err.Error(), "departamento desconhecido") {
		t.Errorf("Ask() com departamento desconhecido deveria falhar em pt-BR, got: %v", err)
	}
	if _, _, err := d.Ask("developer", "finance", "t", "m"); err == nil || !strings.Contains(err.Error(), "departamento desconhecido") {
		t.Errorf("Ask() com --to desconhecido deveria falhar em pt-BR, got: %v", err)
	}
	// Answer com departamento desconhecido → erro pt-BR.
	if err := d.Answer("t:20260802", "finance", "m"); err == nil || !strings.Contains(err.Error(), "departamento desconhecido") {
		t.Errorf("Answer() com departamento desconhecido deveria falhar em pt-BR, got: %v", err)
	}
}

func TestDirector_Answer_UnknownThread(t *testing.T) {
	d, _ := newTestDirector(t)
	if err := d.Answer("nao-existe:20260101", "security", "m"); err == nil || !strings.Contains(err.Error(), "não encontrada") {
		t.Errorf("Answer() de thread desconhecida deveria falhar em pt-BR, got: %v", err)
	}
}

// =============================================================================
// Concorrência leve no Send (thread-safe)
// =============================================================================

func TestStore_ConcurrentSend(t *testing.T) {
	s := newTestStore(t)
	thread := "release-approval:20260802"

	const n = 50
	done := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			_, err := s.Send(ConversationMessage{
				From:     "security",
				To:       "executive",
				Topic:    "release-approval",
				Message:  "check",
				Kind:     "answer",
				ThreadID: thread,
			})
			done <- err
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-done; err != nil {
			t.Fatalf("Send concorrente: %v", err)
		}
	}

	got, err := s.Thread(thread)
	if err != nil {
		t.Fatalf("Thread(): %v", err)
	}
	if len(got) != n {
		t.Fatalf("Thread() len = %d, esperava %d", len(got), n)
	}
}
