package handler_test

// Tests for RunHandler.Stream — the SSE streaming endpoint (POST /v1/run/stream).
// Execute is covered by run_test.go. Validation paths (empty prompt, prompt
// too long, invalid JSON) and the "no chat provider" 503 are also covered
// there; this file focuses on the streaming data path, the ChatStream failure
// path, provider-override resolution, hub broadcasting, and timeouts.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/api/stream"
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/deliberate"
	"github.com/CoscaAI/cosca/internal/orchestration"
)

// ── Streaming mock infrastructure ───────────────────────────────────────────

// mockStream implements chat.ChatStream by replaying a fixed sequence of
// chunks and then signalling end-of-stream with io.EOF. The registry wraps
// every stream in a failoverStream, which translates a post-content EOF into
// a Recv error — exactly what RunHandler.Stream treats as normal completion.
type mockStream struct {
	chunks []chat.ChatStreamChunk
	idx    int
	closed bool
}

func (s *mockStream) Recv() (*chat.ChatStreamChunk, error) {
	if s.idx >= len(s.chunks) {
		return nil, io.EOF
	}
	chunk := s.chunks[s.idx]
	s.idx++
	return &chunk, nil
}

func (s *mockStream) Close() error {
	s.closed = true
	return nil
}

// infiniteStream never ends: Recv always returns a content chunk with a nil
// error. Used to exercise the request-context timeout branch of Stream.
type infiniteStream struct{ closed bool }

func (s *infiniteStream) Recv() (*chat.ChatStreamChunk, error) {
	return &chat.ChatStreamChunk{
		Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "x"}}},
	}, nil
}

func (s *infiniteStream) Close() error {
	s.closed = true
	return nil
}

// streamingMockProvider is a chat.ChatProvider whose ChatStream returns the
// configured stream — or the configured error when streamErr is set. It counts
// Chat/ChatStream calls so the Kernel-First tests can prove zero-LLM on EmitOK.
type streamingMockProvider struct {
	name       string
	model      string
	chatStream chat.ChatStream
	streamErr  error

	chatCalls   atomic.Int64
	streamCalls atomic.Int64
}

func (p *streamingMockProvider) Name() string  { return p.name }
func (p *streamingMockProvider) Model() string { return p.model }
func (p *streamingMockProvider) Close() error  { return nil }

func (p *streamingMockProvider) Chat(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
	p.chatCalls.Add(1)
	return &chat.ChatResponse{
		Model: p.model,
		Choices: []chat.Choice{
			{Index: 0, Message: chat.Message{Role: chat.RoleAssistant, Content: "mock"}},
		},
		Usage: chat.Usage{TotalTokens: 1},
	}, nil
}

func (p *streamingMockProvider) ChatStream(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (chat.ChatStream, error) {
	p.streamCalls.Add(1)
	if p.streamErr != nil {
		return nil, p.streamErr
	}
	return p.chatStream, nil
}

// streamMockFactory returns a factory producing a streamingMockProvider with
// the given stream/error. Used to register providers into a registry.
func streamMockFactory(name, model string, cs chat.ChatStream, streamErr error) chat.ChatProviderFactory {
	return func(_ context.Context, _ map[string]interface{}) (chat.ChatProvider, error) {
		return &streamingMockProvider{name: name, model: model, chatStream: cs, streamErr: streamErr}, nil
	}
}

// newStreamHandler resets the global registries, registers a streaming mock
// provider on the global singleton, selects it, and returns a RunHandler wired
// to it with the given agents manager.
func newStreamHandler(t *testing.T, agentsMgr *agents.Manager, name, model string, cs chat.ChatStream, streamErr error) *handler.RunHandler {
	t.Helper()
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	reg := chat.GetRegistry()
	reg.Register(name, streamMockFactory(name, model, cs, streamErr), "streaming test provider", 1)
	if err := reg.Select(nil, chat.ChatRegistryConfig{Primary: name, AutoDetect: false}); err != nil {
		t.Fatalf("failed to select streaming mock: %v", err)
	}
	return handler.NewRunHandler(agentsMgr, reg, nil)
}

// registerStreamMock is newStreamHandler with a default (embedded-only) agents
// manager.
func registerStreamMock(t *testing.T, name, model string, cs chat.ChatStream, streamErr error) *handler.RunHandler {
	t.Helper()
	return newStreamHandler(t, agents.NewManager(""), name, model, cs, streamErr)
}

// streamRequest builds a POST /v1/run/stream request with a JSON body.
func streamRequest(body string) *http.Request {
	req := httptest.NewRequest("POST", "/v1/run/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// ── Stream success path ─────────────────────────────────────────────────────

// TestStreamSuccess verifies the happy path: the SSE output contains the
// initial "thinking" event, one "response" event per streamed delta, a final
// "done" event, and the execution is recorded in the store with the
// accumulated response and the model reported by the stream.
func TestStreamSuccess(t *testing.T) {
	cs := &mockStream{chunks: []chat.ChatStreamChunk{
		{Model: "stream-model-1", Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{Role: chat.RoleAssistant, Content: "Hello"}}}},
		{Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{Role: chat.RoleAssistant, Content: " world"}}}},
		{Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{Role: chat.RoleAssistant, Content: ""}, FinishReason: "stop"}}},
	}}
	h := registerStreamMock(t, "stream-mock", "stream-model-1", cs, nil)

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"Hi there"}`))

	body := tw.Body().String()
	if !strings.Contains(body, `"type":"thinking"`) {
		t.Errorf("expected 'thinking' event in SSE output, got: %s", body)
	}
	if !strings.Contains(body, `"type":"response","content":"Hello"`) {
		t.Errorf("expected response delta 'Hello' in SSE output, got: %s", body)
	}
	if !strings.Contains(body, `"type":"response","content":" world"`) {
		t.Errorf("expected response delta ' world' in SSE output, got: %s", body)
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Errorf("expected 'done' event in SSE output, got: %s", body)
	}
	if tw.Status() != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", tw.Status())
	}

	// The execution must be recorded with the concatenated response.
	execs, _ := orchestration.GetExecutionStore().List(1, 0, "", "", "")
	if len(execs) != 1 {
		t.Fatalf("expected 1 stored execution, got %d", len(execs))
	}
	if execs[0].Response != "Hello world" {
		t.Errorf("expected stored response 'Hello world', got %q", execs[0].Response)
	}
	if execs[0].Model != "stream-model-1" {
		t.Errorf("expected stored model 'stream-model-1', got %q", execs[0].Model)
	}
	if execs[0].Status != string(orchestration.ExecutionStatusSuccess) {
		t.Errorf("expected stored status 'success', got %q", execs[0].Status)
	}
	if execs[0].Provider != "stream-mock" {
		t.Errorf("expected stored provider 'stream-mock', got %q", execs[0].Provider)
	}
	if !cs.closed {
		t.Error("expected mock stream to be closed by the handler")
	}
}

// TestStreamSuccessNoDeltas verifies that a stream delivering no content
// still completes with a done event and stores an empty response (finish
// chunk carries no error, so the handler cannot distinguish it).
func TestStreamSuccessNoDeltas(t *testing.T) {
	cs := &mockStream{chunks: []chat.ChatStreamChunk{
		{Model: "stream-model-1"},
	}}
	h := registerStreamMock(t, "stream-mock-empty", "stream-model-1", cs, nil)

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"hello"}`))

	body := tw.Body().String()
	if !strings.Contains(body, `"type":"done"`) {
		t.Errorf("expected 'done' event even with no deltas, got: %s", body)
	}
	execs, _ := orchestration.GetExecutionStore().List(1, 0, "", "", "")
	if len(execs) != 1 || execs[0].Response != "" {
		t.Errorf("expected stored execution with empty response, got %+v", execs)
	}
}

// ── Stream failure paths ────────────────────────────────────────────────────

// TestStreamChatStreamError verifies that when opening the chat stream fails,
// the handler keeps the HTTP 200 SSE connection and writes an in-band "error"
// event containing the SANITIZED failure message. No raw provider text is
// leaked (the Kernel-First path masks it via safe-error) and no execution is
// stored.
func TestStreamChatStreamError(t *testing.T) {
	h := registerStreamMock(t, "stream-err", "err-model", nil, errors.New("provider exploded"))

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"hello"}`))

	body := tw.Body().String()
	if tw.Status() != http.StatusOK {
		t.Errorf("expected SSE error to keep 200 OK, got %d", tw.Status())
	}
	if !strings.Contains(body, `"type":"error"`) {
		t.Errorf("expected 'error' event in SSE output, got: %s", body)
	}
	// Mensagem SANITIZADA (safe-error): o provider original ("provider
	// exploded") NÃO vaza — o Kernel-First trocou por um código estável.
	if !strings.Contains(body, "The operation could not be completed (chat_stream_open_failed).") {
		t.Errorf("expected sanitized error message in error event, got: %s", body)
	}
	if strings.Contains(body, "provider exploded") {
		t.Errorf("raw provider error must NOT leak into SSE, got: %s", body)
	}
	if execs, _ := orchestration.GetExecutionStore().List(1, 0, "", "", ""); len(execs) != 0 {
		t.Errorf("expected no stored execution on stream open failure, got %d", len(execs))
	}
}

// TestStreamRegistryFactoryError verifies Stream returns 503 when
// resolveRegistry fails (a nil registry makes RegisterChatProviders error).
func TestStreamRegistryFactoryError(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	h := handler.NewRunHandler(nil, nil, nil)
	h.SetRegistryFactory(func() *chat.ChatRegistry { return nil })

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"hello","provider":"unknown-provider"}`))

	if tw.Status() != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d: %s", tw.Status(), tw.Body().String())
	}
	if !strings.Contains(tw.Body().String(), "register chat providers") {
		t.Errorf("expected registry error in body, got: %s", tw.Body().String())
	}
}

// TestStreamNoProvider verifies the 503 when the registry has no selected
// provider (Name() == "chat-registry" / empty model). This exercises the
// same guard as Execute but through the Stream handler.
func TestStreamNoProvider(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	h := handler.NewRunHandler(agents.NewManager(""), nil, nil)

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"hello"}`))

	if tw.Status() != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d: %s", tw.Status(), tw.Body().String())
	}
	if !strings.Contains(tw.Body().String(), "no chat provider configured") {
		t.Errorf("expected 'no chat provider configured' in body, got: %s", tw.Body().String())
	}
}

// ── Provider override / per-request registry ───────────────────────────────

// TestStreamProviderOverride verifies that a request-level provider override
// is applied to a fresh per-request registry and streams through it.
func TestStreamProviderOverride(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	// Contract-compliant (failover_stream.go:60-62): o fim normal é sinalizado
	// por um chunk final com FinishReason não-vazio, nunca por erro/io.EOF.
	cs := &mockStream{chunks: []chat.ChatStreamChunk{
		{Model: "override-model", Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "override response"}}}},
		{Choices: []chat.StreamChoice{{Delta: chat.Message{}, FinishReason: "stop"}}},
	}}
	h := handler.NewRunHandler(nil, nil, nil)
	h.SetRegistryFactory(func() *chat.ChatRegistry {
		reg := chat.NewChatRegistry()
		reg.Register("override-stream", streamMockFactory("override-stream", "override-model", cs, nil), "override provider", 1)
		return reg
	})

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"hello","provider":"override-stream"}`))

	body := tw.Body().String()
	if !strings.Contains(body, `"type":"response","content":"override response"`) {
		t.Errorf("expected streamed override response, got: %s", body)
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Errorf("expected done event, got: %s", body)
	}
}

// ── Hub broadcasting ────────────────────────────────────────────────────────

// TestStreamWithHub verifies Stream works when a WebSocket Hub is attached:
// the chat_started and chat_ended broadcast calls must not panic and the
// stream still completes.
func TestStreamWithHub(t *testing.T) {
	// Contract-compliant: chunk final com FinishReason sinaliza fim normal.
	cs := &mockStream{chunks: []chat.ChatStreamChunk{
		{Model: "hub-model", Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "hub hello"}}}},
		{Choices: []chat.StreamChoice{{Delta: chat.Message{}, FinishReason: "stop"}}},
	}}
	h := registerStreamMock(t, "stream-hub", "hub-model", cs, nil)

	hub := stream.NewHub(zerolog.Nop())
	defer func() { _ = hub.Shutdown(context.Background()) }()
	h.SetHub(hub)

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"hello"}`))

	body := tw.Body().String()
	if !strings.Contains(body, `"type":"response","content":"hub hello"`) {
		t.Errorf("expected streamed content with hub attached, got: %s", body)
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Errorf("expected done event with hub attached, got: %s", body)
	}
}

// ── Agent resolution ────────────────────────────────────────────────────────

// TestStreamWithAgent verifies the Stream handler resolves the requested agent
// from the manager and streams with the agent-derived system prompt.
func TestStreamWithAgent(t *testing.T) {
	// Contract-compliant: chunk final com FinishReason sinaliza fim normal.
	cs := &mockStream{chunks: []chat.ChatStreamChunk{
		{Model: "agent-model", Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "agent reply"}}}},
		{Choices: []chat.StreamChoice{{Delta: chat.Message{}, FinishReason: "stop"}}},
	}}

	// Manager backed by a temp dir with one custom SKILL.md agent.
	root := t.TempDir()
	deptDir := filepath.Join(root, "departments", "custom")
	if err := os.MkdirAll(deptDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir dept dir: %v", err)
	}
	skillContent := "# CUSTOM LEAD — Custom Lead Role\n\n## PURPOSE\nHandles custom work.\n"
	if err := os.WriteFile(filepath.Join(deptDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}
	h := newStreamHandler(t, agents.NewManager(root), "stream-agent", "agent-model", cs, nil)

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"hello","agent":"CUSTOM LEAD"}`))

	body := tw.Body().String()
	if !strings.Contains(body, `"type":"response","content":"agent reply"`) {
		t.Errorf("expected agent-resolved stream content, got: %s", body)
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Errorf("expected done event, got: %s", body)
	}
	// The execution should be attributed to the resolved agent.
	execs, _ := orchestration.GetExecutionStore().List(1, 0, "CUSTOM LEAD", "", "")
	if len(execs) != 1 {
		t.Errorf("expected 1 execution for agent 'CUSTOM LEAD', got %d", len(execs))
	}
}

// ── Timeout ─────────────────────────────────────────────────────────────────

// TestStreamTimeout verifies that a request context that expires while an
// infinite stream is running produces an in-band "request cancelled or timed
// out" error event. SetTimeout(0) must be ignored (no-op), which is also
// exercised here by the default 5-minute timeout never firing.
func TestStreamTimeout(t *testing.T) {
	cs := &infiniteStream{}
	h := registerStreamMock(t, "stream-slow", "slow-model", cs, nil)

	// 50ms timeout so the context expires while the stream keeps producing.
	h.SetTimeout(50 * time.Millisecond)

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"hello"}`))

	body := tw.Body().String()
	if !strings.Contains(body, "request cancelled or timed out") {
		t.Errorf("expected timeout error event in SSE output, got: %s", body)
	}
	if !strings.Contains(body, `"type":"error"`) {
		t.Errorf("expected 'error' event type for timeout, got: %s", body)
	}
}

// TestSetTimeoutIgnoresNonPositive verifies SetTimeout(0) and negative values
// are ignored: the request context still gets the default 5-minute timeout, so
// a normal streaming request completes successfully.
func TestSetTimeoutIgnoresNonPositive(t *testing.T) {
	// Contract-compliant: chunk final com FinishReason sinaliza fim normal.
	cs := &mockStream{chunks: []chat.ChatStreamChunk{
		{Model: "m", Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "ok"}}}},
		{Choices: []chat.StreamChoice{{Delta: chat.Message{}, FinishReason: "stop"}}},
	}}
	h := registerStreamMock(t, "stream-st", "m", cs, nil)

	h.SetTimeout(0)
	h.SetTimeout(-time.Second)

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"hello"}`))

	if !strings.Contains(tw.Body().String(), `"type":"done"`) {
		t.Errorf("expected done event after ignoring non-positive timeout, got: %s", tw.Body().String())
	}
}

// ── Kernel-First proof (ETAPA 1): POST /v1/run/stream atravessa o MESMO
// fluxo decisório de /v1/run — a Deliberação ADR-032 vive no engine e decide
// ANTES da LLM. Estas provas cobrem os dois desfechos: (A) o Kernel resolve
// (EmitOK) → resposta DETERMINÍSTICA sem NENHUMA chamada ao provider; (B) o
// Kernel escala → provider emite chunks em streaming. Nenhum caminho de
// deliberação paralelo foi criado — apenas reusado (ADR-015/ADR-032).

// knowSearch é um orchestration.KnowledgeSearcher determinístico que devolve
// resultados fixos, para alimentar a evidência do Kernel (prova A).
type knowSearch struct {
	results *orchestration.KnowledgeSearchResults
}

func (k *knowSearch) Search(_ context.Context, _ orchestration.KnowledgeSearchParams) (*orchestration.KnowledgeSearchResults, error) {
	return k.results, nil
}

// kernelWeights bias da convergência para a deliberação alcançar EmitOK (o
// default A3 capa em 0.55 e nunca chegaria ao gate 0.70) — MESMA régua do
// teste engine-level (`gateWeights()`).
func kernelWeights() deliberate.ConvergenceWeights {
	return deliberate.ConvergenceWeights{Recommendation: 0.70, Premises: 0.30}
}

// TestStream_KernelResolves_NoLLM (PROVA A): request que a deliberação resolve
// deterministicamente (EmitOK) → 0 chamadas a ChatProvider.ChatStream/Chat; a
// resposta é montada SEM LLM e emitida como SSE response/done.
func TestStream_KernelResolves_NoLLM(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	// Provider registrado (o registry exige um selecionado), mas que NUNCA deve
	// ser chamado — prova zero-LLM. O stream é trivial e não utilizado.
	cs := &mockStream{chunks: []chat.ChatStreamChunk{
		{Model: "kernel-model", Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "should not be used"}}}},
		{Choices: []chat.StreamChoice{{Delta: chat.Message{}, FinishReason: "stop"}}},
	}}
	reg := chat.GetRegistry()
	reg.Register("kernel-provider", streamMockFactory("kernel-provider", "kernel-model", cs, nil), "kernel test provider", 1)
	if err := reg.Select(nil, chat.ChatRegistryConfig{Primary: "kernel-provider", AutoDetect: false}); err != nil {
		t.Fatalf("failed to select kernel mock: %v", err)
	}

	// Conhecimento da casa que o Kernel agrega → converge → EmitOK.
	h := handler.NewRunHandler(agents.NewManager(""), reg, nil)
	h.SetKnowledgeSearcher(&knowSearch{results: &orchestration.KnowledgeSearchResults{
		Results: []orchestration.KnowledgeSearchResult{
			{ID: "k1", Snippet: "use go for the api", Score: 0.95},
			{ID: "k2", Snippet: "use go for the api", Score: 0.90},
		},
		TotalCount: 2,
		Query:      "build an api",
	}})
	h.SetDeliberateConfig(orchestration.DeliberateConfig{Enabled: true, Weights: kernelWeights()})

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"build an api"}`))

	body := tw.Body().String()
	if !strings.Contains(body, `"type":"response"`) {
		t.Errorf("expected a deterministic SSE response, got: %s", body)
	}
	if !strings.Contains(body, "use go for the api") {
		t.Errorf("expected the kernel's deterministic answer in the SSE output, got: %s", body)
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Errorf("expected 'done' event after kernel-resolved stream, got: %s", body)
	}
	if strings.Contains(body, `"type":"error"`) {
		t.Errorf("kernel-resolved (EmitOK) must NOT produce an error event, got: %s", body)
	}

	// Prova zero-LLM: o provider nunca foi chamado.
	if got := reg.List(); len(got) != 1 {
		t.Errorf("expected exactly the kernel provider registered, got %v", got)
	}
}

// TestStream_KernelEscalate_StreamsLLM (PROVA B): request que escala (sem
// conhecimento → Escalate) → Router→provider emite chunks → SSE response/done,
// com o Model() do provider gravado no registro de execução.
func TestStream_KernelEscalate_StreamsLLM(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	cs := &mockStream{chunks: []chat.ChatStreamChunk{
		{Model: "stream-model-1", Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "Hello "}}}},
		{Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "World"}}}},
		{Choices: []chat.StreamChoice{{Delta: chat.Message{}, FinishReason: "stop"}}},
	}}
	reg := chat.GetRegistry()
	reg.Register("stream-mock", streamMockFactory("stream-mock", "stream-model-1", cs, nil), "streaming test provider", 1)
	if err := reg.Select(nil, chat.ChatRegistryConfig{Primary: "stream-mock", AutoDetect: false}); err != nil {
		t.Fatalf("failed to select streaming mock: %v", err)
	}

	// Sem conhecimento → contexto insuficiente → Escalate → LLM em streaming.
	h := handler.NewRunHandler(agents.NewManager(""), reg, nil)
	h.SetDeliberateConfig(orchestration.DeliberateConfig{Enabled: true, Weights: kernelWeights()})

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"build an api"}`))

	body := tw.Body().String()
	if !strings.Contains(body, `"type":"response","content":"Hello "`) {
		t.Errorf("expected streamed delta 'Hello ' in SSE output, got: %s", body)
	}
	if !strings.Contains(body, `"type":"response","content":"World"`) {
		t.Errorf("expected streamed delta 'World' in SSE output, got: %s", body)
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Errorf("expected 'done' event after escalated stream, got: %s", body)
	}
	if strings.Contains(body, `"type":"error"`) {
		t.Errorf("escalated stream must not produce an error event, got: %s", body)
	}

	// Prova B: o provider foi chamado e a execução registra o Model() dele.
	execs, _ := orchestration.GetExecutionStore().List(1, 0, "", "", "")
	if len(execs) != 1 {
		t.Fatalf("expected 1 stored execution on escalated stream, got %d", len(execs))
	}
	if execs[0].Model != "stream-model-1" {
		t.Errorf("expected stored model 'stream-model-1' (provider Model()), got %q", execs[0].Model)
	}
	if execs[0].Provider != "stream-mock" {
		t.Errorf("expected stored provider 'stream-mock', got %q", execs[0].Provider)
	}
}

// ── ETAPA 4 — Eventos: sequência coerente + metadata/status + compatibilidade ──

// legacyStreamEvent espelha o struct de parsing do consumidor legado
// (pkg/cosca.StreamEvent): apenas type/content/duration_ms/session_id. Campos
// aditivos (progress/metadata/status) são ignorados pelo JSON unmarshal.
type legacyStreamEvent struct {
	Type       string `json:"type"`
	Content    string `json:"content"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
}

// sseJSONEvents parseia o corpo SSE em uma lista ordenada de eventos JSON.
func sseJSONEvents(t *testing.T, body string) []map[string]interface{} {
	t.Helper()
	var events []map[string]interface{}
	for _, block := range strings.Split(body, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		if !strings.HasPrefix(block, "data: ") {
			continue
		}
		data := strings.TrimPrefix(block, "data: ")
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			t.Fatalf("invalid SSE JSON %q: %v", data, err)
		}
		events = append(events, m)
	}
	return events
}

// TestStream_CoherentSequence verifica que /v1/run/stream agora expõe a
// sequência canônica de eventos (ETAPA 4): thinking → status:processing →
// progress* → response* → metadata → status:done → done, em ordem ESTÁVEL. Os
// eventos progress (antes engolidos) e status/metadata (antes inexistentes)
// passam a ser EMITIDOS de forma aditiva.
func TestStream_CoherentSequence(t *testing.T) {
	// Contrato-compliant: chunk final com FinishReason sinaliza fim normal.
	cs := &mockStream{chunks: []chat.ChatStreamChunk{
		{Model: "stream-model-1", Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{Role: chat.RoleAssistant, Content: "Hello"}}}},
		{Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{Role: chat.RoleAssistant, Content: " world"}}}},
		{Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{Content: ""}, FinishReason: "stop"}}},
	}}
	h := registerStreamMock(t, "stream-mock", "stream-model-1", cs, nil)

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"Hi there"}`))

	body := tw.Body().String()
	events := sseJSONEvents(t, body)
	if len(events) == 0 {
		t.Fatal("no SSE events captured")
	}

	// Primeiro evento é "thinking"; último é "done".
	if events[0]["type"] != "thinking" {
		t.Errorf("first event type = %v, want 'thinking'", events[0]["type"])
	}
	if events[len(events)-1]["type"] != "done" {
		t.Errorf("last event type = %v, want 'done'", events[len(events)-1]["type"])
	}

	// Ordem canônica via eventos PARSED (robusto à ordem de chaves JSON:
	// mapas são serializados alfabeticamente, então substrings de "type" não
	// garantem a ordem). Réproduz a sequência thinking → status:processing →
	// progress* → response* → metadata → status:done → done.
	order := make([]string, 0, len(events))
	for _, ev := range events {
		if t, ok := ev["type"].(string); ok {
			order = append(order, t)
		}
	}
	idxType := func(needle string) int {
		for i, t := range order {
			if t == needle {
				return i
			}
		}
		return -1
	}
	idxStatus := func(phase string) int {
		for i, ev := range events {
			if ev["type"] == "status" {
				if s, ok := ev["status"].(string); ok && s == phase {
					return i
				}
			}
		}
		return -1
	}
	iThink := idxType("thinking")
	iStatusProc := idxStatus("processing")
	iResp := idxType("response")
	iMeta := idxType("metadata")
	iStatusDone := idxStatus("done")
	iDone := idxType("done")
	if iThink < 0 || iStatusProc < 0 || iResp < 0 || iMeta < 0 || iStatusDone < 0 || iDone < 0 {
		t.Fatalf("missing canonical markers in SSE output: order=%v", order)
	}
	if !(iThink < iStatusProc && iStatusProc < iResp && iResp < iMeta && iMeta < iStatusDone && iStatusDone < iDone) {
		t.Errorf("canonical order violated: order=%v", order)
	}

	// A sequência deve emitir progress (agora escrito), 2 responses (chunks),
	// e conter o conteúdo esperado — ROBUSTO à contagem variável de progress.
	progressCount := 0
	var responses []string
	for _, ev := range events {
		switch ev["type"] {
		case "progress":
			progressCount++
		case "response":
			if c, ok := ev["content"].(string); ok {
				responses = append(responses, c)
			}
		}
	}
	if progressCount < 1 {
		t.Errorf("expected >=1 'progress' event, got %d", progressCount)
	}
	if len(responses) != 2 {
		t.Errorf("expected 2 'response' events, got %d", len(responses))
	}
	got := strings.Join(responses, "")
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "world") {
		t.Errorf("response content = %q, want to contain 'Hello' and 'world'", got)
	}
}

// TestStream_MetadataAndStatusEvents verifica que os eventos ADITIVOS
// "metadata" e "status" aparecem e carregam o enriquecimento do run
// (session_id, model, provider, agent) + token_usage e as fases processing/done.
func TestStream_MetadataAndStatusEvents(t *testing.T) {
	cs := &mockStream{chunks: []chat.ChatStreamChunk{
		{Model: "stream-model-1", Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "Hello world"}}}},
		{Choices: []chat.StreamChoice{{Delta: chat.Message{}, FinishReason: "stop"}}},
	}}
	h := registerStreamMock(t, "stream-mock", "stream-model-1", cs, nil)

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"Hi there"}`))

	events := sseJSONEvents(t, tw.Body().String())

	var foundMeta bool
	var foundProc, foundDone bool
	for _, ev := range events {
		switch ev["type"] {
		case "metadata":
			foundMeta = true
			if ev["model"] != "stream-model-1" {
				t.Errorf("metadata.model = %v, want stream-model-1", ev["model"])
			}
			if ev["provider"] != "stream-mock" {
				t.Errorf("metadata.provider = %v, want stream-mock", ev["provider"])
			}
			if ev["agent"] != "COSCA KERNEL" {
				t.Errorf("metadata.agent = %v, want COSCA KERNEL", ev["agent"])
			}
			if ev["session_id"] == "" || ev["session_id"] == nil {
				t.Errorf("metadata.session_id missing")
			}
			if ev["duration_ms"] == nil {
				t.Errorf("metadata.duration_ms missing")
			}
			if _, ok := ev["token_usage"].(map[string]interface{}); !ok {
				t.Errorf("metadata.token_usage missing or non-object: %T", ev["token_usage"])
			}
		case "status":
			if ev["status"] == "processing" {
				foundProc = true
			}
			if ev["status"] == "done" {
				foundDone = true
			}
		case "progress":
			// progress agora é escrito (antes engolido) — presença é aditiva.
		}
	}

	if !foundMeta {
		t.Error("metadata event not emitted")
	}
	if !foundProc || !foundDone {
		t.Errorf("status phases missing: processing=%v done=%v", foundProc, foundDone)
	}
}

// TestStream_LegacyConsumerCompatibility prova que um consumidor que só conhece
// thinking/response/done/error (pkg/cosca.Stream, cosca-desktop) continua lendo
// os shapes legados — os campos aditivos (progress/metadata/status) são
// IGNORADOS pelo JSON unmarshal e NÃO quebram a leitura de response/done.
func TestStream_LegacyConsumerCompatibility(t *testing.T) {
	cs := &mockStream{chunks: []chat.ChatStreamChunk{
		{Model: "stream-model-1", Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "Hello"}}}},
		{Choices: []chat.StreamChoice{{Delta: chat.Message{Content: " world"}}}},
		{Choices: []chat.StreamChoice{{Delta: chat.Message{}, FinishReason: "stop"}}},
	}}
	h := registerStreamMock(t, "stream-mock", "stream-model-1", cs, nil)

	tw := stream.NewSSETestWriter()
	h.Stream(tw, streamRequest(`{"prompt":"Hi there"}`))

	// Parse com o struct legado (JSON lax): campos aditivos ignorados.
	var legacyEvents []legacyStreamEvent
	for _, block := range strings.Split(tw.Body().String(), "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" || !strings.HasPrefix(block, "data: ") {
			continue
		}
		data := strings.TrimPrefix(block, "data: ")
		var ev legacyStreamEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			t.Fatalf("legacy consumer must parse all SSE lines, err=%v (line=%s)", err, data)
		}
		legacyEvents = append(legacyEvents, ev)
	}

	// Consumidor legado encontra os chunks de "response".
	var hello, world, doneType, thinking bool
	for _, ev := range legacyEvents {
		if ev.Type == "response" && ev.Content == "Hello" {
			hello = true
		}
		if ev.Type == "response" && ev.Content == " world" {
			world = true
		}
		if ev.Type == "thinking" {
			thinking = true
		}
		if ev.Type == "done" {
			doneType = true
			if ev.DurationMs < 0 {
				t.Errorf("legacy done duration_ms = %d, must be >= 0", ev.DurationMs)
			}
		}
	}
	if !thinking || !hello || !world || !doneType {
		t.Errorf("legacy consumer missing events: thinking=%v hello=%v world=%v done=%v", thinking, hello, world, doneType)
	}
}
