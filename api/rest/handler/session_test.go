package handler_test

// Tests for ETAPA 2 — Session. Cobre a circulação do session_id pelo fluxo de
// chat + persistência + resume + fork (engine.SessionManager reusado) e as
// provas E do professor:
//   (a) request com session_id -> mensagem persistida;
//   (b) resume (mesmo session_id -> histórico recontado/injetado);
//   (c) fork (parent_session_id -> child com meta parent);
//   (d) SessionID preenchido na resposta/SSE;
//   (e) request sem session_id -> handler cria e devolve.
//
// Todas as provas usam um diretório temporário via SetSessionsDir — a
// persistência NUNCA toca o .cosca do repo. Cada request recebe um stream
// NOVO (provider cria um chunks-cópia por chamada) para simular turnos
// independentes.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/api/stream"
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/orchestration"
)

// capturingStreamProvider é um chat.ChatProvider que devolve um stream NOVO a
// cada ChatStream (cópia dos chunks) e captura as mensagens recebidas — permite
// provar que o histórico foi injetado sem reutilizar um stream já esgotado.
type capturingStreamProvider struct {
	name    string
	model   string
	chunks  []chat.ChatStreamChunk
	mu       sync.Mutex
	lastMsg  []chat.Message
}

func (p *capturingStreamProvider) Name() string       { return p.name }
func (p *capturingStreamProvider) Model() string      { return p.model }
func (p *capturingStreamProvider) HealthCheck() error { return nil }
func (p *capturingStreamProvider) Close() error       { return nil }

func (p *capturingStreamProvider) lastMessages() []chat.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]chat.Message(nil), p.lastMsg...)
}

func (p *capturingStreamProvider) Chat(_ context.Context, msgs []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
	p.mu.Lock()
	p.lastMsg = append([]chat.Message(nil), msgs...)
	p.mu.Unlock()
	return &chat.ChatResponse{
		Model:   p.model,
		Choices: []chat.Choice{{Index: 0, Message: chat.Message{Role: chat.RoleAssistant, Content: "sync ok"}}},
	}, nil
}

func (p *capturingStreamProvider) ChatStream(_ context.Context, msgs []chat.Message, _ chat.ChatOptions) (chat.ChatStream, error) {
	p.mu.Lock()
	p.lastMsg = append([]chat.Message(nil), msgs...)
	p.mu.Unlock()
	return &mockStream{chunks: p.chunks}, nil
}

// newSessionStreamHandler registra um provider que captura messages, seta o
// diretório de sessões e devolve o handler.
func newSessionStreamHandler(t *testing.T, sessDir, name, model string, chunks []chat.ChatStreamChunk) (*handler.RunHandler, *capturingStreamProvider) {
	t.Helper()
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	provider := &capturingStreamProvider{name: name, model: model, chunks: chunks}
	reg := chat.GetRegistry()
	reg.Register(name, func(_ context.Context, _ map[string]interface{}) (chat.ChatProvider, error) {
		return provider, nil
	}, "session test provider", 1)
	if err := reg.Select(nil, chat.ChatRegistryConfig{Primary: name, AutoDetect: false}); err != nil {
		t.Fatalf("failed to select session mock: %v", err)
	}
	h := handler.NewRunHandler(agents.NewManager(""), reg, nil)
	h.SetSessionsDir(sessDir)
	return h, provider
}

// readSessionJSONL lê .cosca/sessions/{id}.jsonl e devolve as linhas como mapas.
func readSessionJSONL(t *testing.T, dir, id string) []map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, id+".jsonl"))
	if err != nil {
		t.Fatalf("read session file %s.jsonl: %v", id, err)
	}
	var out []map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("parse session line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

// countMessages devolve o número de linhas do tipo message.
func countMessages(lines []map[string]interface{}) int {
	n := 0
	for _, l := range lines {
		if l["type"] == "message" {
			n++
		}
	}
	return n
}

// sessionStreamChunks devolve os chunks de um turno contract-compliant.
func sessionStreamChunks(content string) []chat.ChatStreamChunk {
	return []chat.ChatStreamChunk{
		{Model: "m", Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{Role: chat.RoleAssistant, Content: content}}}},
		{Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{}, FinishReason: "stop"}}},
	}
}

// ── Proof (a): request com session_id -> mensagem persistida ─────────────────

func TestSession_PersistsOnSessionID(t *testing.T) {
	sessDir := t.TempDir()
	h, _ := newSessionStreamHandler(t, sessDir, "s-mock", "m", sessionStreamChunks("Hello"))

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"ola","session_id":"abc123"}`))

	lines := readSessionJSONL(t, sessDir, "abc123")
	if lines[0]["type"] != "meta" || lines[0]["id"] != "abc123" {
		t.Fatalf("meta line malformed: %v", lines[0])
	}
	if got := countMessages(lines); got != 2 {
		t.Fatalf("expected 2 messages (user+assistant), got %d: %v", got, lines)
	}
	if lines[1]["role"] != "user" || lines[1]["content"] != "ola" {
		t.Errorf("user message not persisted: %v", lines[1])
	}
	if lines[2]["role"] != "assistant" || lines[2]["content"] != "Hello" {
		t.Errorf("assistant message not persisted: %v", lines[2])
	}
	// (d) o session_id também circula no corpo SSE.
	if !strings.Contains(tw.Body().String(), "abc123") {
		t.Errorf("SSE output missing session_id, got: %s", tw.Body().String())
	}
}

// ── Proof (b): resume (mesmo session_id -> histórico recontado/injetado) ──────

func TestSession_ResumeRecountsHistory(t *testing.T) {
	sessDir := t.TempDir()
	h, prov := newSessionStreamHandler(t, sessDir, "s-mock", "m", sessionStreamChunks("bem"))

	// 1º turno.
	h.Stream(stream.NewSSETestWriter(), streamRequest(`{"prompt":"oi","session_id":"abc"}`))
	// 2º turno com o MESMO session_id -> injeta o histórico.
	h.Stream(stream.NewSSETestWriter(), streamRequest(`{"prompt":"como vai","session_id":"abc"}`))

	// O provider recebeu system + 2 históricos (user+assistant do 1º turno) + user.
	if got := len(prov.lastMessages()); got != 4 {
		t.Fatalf("expected 4 messages (system+history 2+user), got %d: %v", got, prov.lastMessages())
	}

	// Persistência acumulada: o jsonl tem 4 mensagens.
	if got := countMessages(readSessionJSONL(t, sessDir, "abc")); got != 4 {
		t.Errorf("expected 4 persisted messages after resume, got %d", got)
	}
}

// ── Proof (c): fork (parent_session_id -> child com meta parent) ─────────────

func TestSession_ForkCreatesChildWithParent(t *testing.T) {
	sessDir := t.TempDir()
	h, _ := newSessionStreamHandler(t, sessDir, "s-mock", "m", sessionStreamChunks("raiz"))

	// Criar um pai.
	h.Stream(stream.NewSSETestWriter(), streamRequest(`{"prompt":"pai","session_id":"parent"}`))

	// Fork: parent_session_id="parent" (child id auto-gerado).
	h.Stream(stream.NewSSETestWriter(), streamRequest(`{"prompt":"filho","parent_session_id":"parent"}`))

	// Identificar o arquivo filho (o único que não é parent.jsonl).
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		t.Fatalf("read sessions dir: %v", err)
	}
	var childID string
	for _, e := range entries {
		if name := strings.TrimSuffix(e.Name(), ".jsonl"); name != "parent" {
			childID = name
		}
	}
	if childID == "" {
		t.Fatalf("no child session created (entries=%v)", entries)
	}

	lines := readSessionJSONL(t, sessDir, childID)
	if lines[0]["parent_session_id"] != "parent" {
		t.Errorf("child meta must carry parent_session_id=parent, got: %v", lines[0])
	}
	// Seed do pai (2) + turno novo (2) = 4 mensagens.
	if got := countMessages(lines); got != 4 {
		t.Errorf("expected 4 messages (2 seed + 2 new), got %d", got)
	}
}

// ── Proof (d): SessionID preenchido na resposta de /v1/run ───────────────────

func TestSession_ExecuteReturnsSessionID(t *testing.T) {
	sessDir := t.TempDir()
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()
	chat.GetRegistry().Register("s-mock", namedMockProviderFactory("s-mock", "m"), "provider", 1)
	if err := chat.GetRegistry().Select(nil, chat.ChatRegistryConfig{Primary: "s-mock", AutoDetect: false}); err != nil {
		t.Fatalf("select provider: %v", err)
	}
	h := handler.NewRunHandler(agents.NewManager(""), chat.GetRegistry(), nil)
	h.SetSessionsDir(sessDir)

	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(`{"prompt":"ola","session_id":"exec123"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Execute(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["session_id"] != "exec123" {
		t.Errorf("expected response session_id=exec123, got %v", resp["session_id"])
	}
	// (a) persistiu.
	if got := countMessages(readSessionJSONL(t, sessDir, "exec123")); got != 2 {
		t.Errorf("expected 2 persisted messages, got %d", got)
	}
}

// ── Proof (e): sem session_id -> handler cria e devolve ──────────────────────

func TestSession_GeneratesSessionIDWhenAbsent(t *testing.T) {
	sessDir := t.TempDir()
	h, _ := newSessionStreamHandler(t, sessDir, "s-mock", "m", sessionStreamChunks("ok"))

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"ola"}`))

	body := tw.Body().String()
	if !strings.Contains(body, "session_id") {
		t.Errorf("SSE output missing session_id, got: %s", body)
	}

	// Exatamente 1 arquivo de sessão foi criado (id gerado pelo handler).
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		t.Fatalf("read sessions dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 session file (auto-generated id), got %d", len(entries))
	}
	if got := countMessages(readSessionJSONL(t, sessDir, strings.TrimSuffix(entries[0].Name(), ".jsonl"))); got != 2 {
		t.Errorf("expected 2 persisted messages, got %d", got)
	}
}
