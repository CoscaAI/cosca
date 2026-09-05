package handler_test

// Tests for RunHandler.Stream — the SSE streaming endpoint (POST /v1/run/stream).
// Execute is covered by run_test.go. Validation paths (empty prompt, prompt
// too long, invalid JSON) and the "no chat provider" 503 are also covered
// there; this file focuses on the streaming data path, the ChatStream failure
// path, provider-override resolution, hub broadcasting, and timeouts.

import (
	"context"
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
