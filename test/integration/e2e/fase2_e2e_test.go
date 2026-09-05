// Package e2e — ETAPA 5: provas de INTEGRAÇÃO de ponta a ponta (FASE 2).
//
// Diferente dos testes de handler (api/rest/handler) que chamam o handler
// diretamente, ESTES testes exercitam a cadeia REAL através do HTTP e do SDK
// público pkg/cosca, sobre um httptest.Server que monta o MESMO RunHandler
// (que usa o MESMO Engine/orquestrador/rede de produção). Nenhuma abstração
// paralela, nenhum endpoint novo, nenhum router/provider/registry duplicado —
// apenas o REUSE: Desktop/OpenCode → HTTP /v1/run → Kernel → Deliberação
// (ADR-032) → Oracle Gate → (resolve sem LLM | escala → Router → Provider
// mock → streaming) → sessão (persist/resume/fork) → cancelamento.
//
// Prove a professora:
//   (A) SDK.Run → Kernel resolve (EmitOK) → resposta DETERMINÍSTICA sem LLM;
//   (B) SDK.Stream → Kernel escala → Router → Provider mock → streaming,
//       com sequência de eventos coerente;
//   (C) SDK.Stream com WithSession → persist/resume/fork;
//   (D) SDK.Stream com ctx cancelado → termina com cancelled, sem goroutine leak.
//
// Os providers são MOCKS (sem LLM real). Nada roda um daemon: httptest isola.
//
// Estes arquivos NÃO têm build tag `integration` para que a validação
// `go test ./test/integration/... -count=1` (sem -tags) os execute — os testes
// de integração "pesados" (com tag) ficam no pacote irmão `test/integration`.
package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/deliberate"
	"github.com/CoscaAI/cosca/internal/orchestration"
	cosca "github.com/CoscaAI/cosca/pkg/cosca"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mocks (provider e stream) — SEM LLM real
// ─────────────────────────────────────────────────────────────────────────────

// e2eStream replay a sequence of chunks; after exhausting them it either
// returns io.EOF (normal end) or, when cancelCh is set, blocks until the
// channel closes and then returns context.Canceled (simulates the provider
// honouring a cancellation). Contract-compliant: the FINAL chunk carries a
// non-empty FinishReason, exactly like failover_stream expects.
type e2eStream struct {
	chunks   []chat.ChatStreamChunk
	idx      int
	closed   atomic.Bool
	cancelCh <-chan struct{}
}

func (s *e2eStream) Recv() (*chat.ChatStreamChunk, error) {
	if s.idx < len(s.chunks) {
		c := s.chunks[s.idx]
		s.idx++
		return &c, nil
	}
	if s.cancelCh != nil {
		<-s.cancelCh
		return nil, context.Canceled
	}
	return nil, io.EOF
}

func (s *e2eStream) Close() error {
	s.closed.Store(true)
	return nil
}

// finishChunk builds the contract-compliant final stream chunk.
func finishChunk() chat.ChatStreamChunk {
	return chat.ChatStreamChunk{
		Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{}, FinishReason: chat.FinishReasonStop}},
	}
}

// contentChunk builds a stream chunk carrying a text delta.
func contentChunk(model, content string) chat.ChatStreamChunk {
	return chat.ChatStreamChunk{
		Model:   model,
		Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{Role: chat.RoleAssistant, Content: content}}},
	}
}

// streamChunks returns the standard streaming turn: two content chunks + stop.
func streamChunks(model, a, b string) []chat.ChatStreamChunk {
	return []chat.ChatStreamChunk{contentChunk(model, a), contentChunk(model, b), finishChunk()}
}

// e2eProvider implements chat.ChatProvider with configurable Chat / ChatStream
// behaviour. It captures the messages it receives so tests can prove that a
// resumed session's history was injected. All call counters are atomic.
type e2eProvider struct {
	name  string
	model string

	// chatFn: optional synchronous completion (default: a fixed "mock" reply).
	chatFn func(msgs []chat.Message) (*chat.ChatResponse, error)

	// streamFactory: returns a FRESH stream per ChatStream call (so a resumed
	// session gets an unused stream). When nil, ChatStream errors.
	streamFactory func() (chat.ChatStream, error)

	mu        sync.Mutex
	lastMsgs  []chat.Message
	chatCalls atomic.Int64
	strCalls  atomic.Int64
}

func (p *e2eProvider) store(msgs []chat.Message) {
	p.mu.Lock()
	p.lastMsgs = append([]chat.Message(nil), msgs...)
	p.mu.Unlock()
}

func (p *e2eProvider) lastMessages() []chat.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]chat.Message(nil), p.lastMsgs...)
}

func (p *e2eProvider) Name() string  { return p.name }
func (p *e2eProvider) Model() string { return p.model }
func (p *e2eProvider) Close() error  { return nil }

func (p *e2eProvider) Chat(_ context.Context, msgs []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
	p.chatCalls.Add(1)
	p.store(msgs)
	if p.chatFn != nil {
		return p.chatFn(msgs)
	}
	return &chat.ChatResponse{
		Model:   p.model,
		Choices: []chat.Choice{{Index: 0, Message: chat.Message{Role: chat.RoleAssistant, Content: "mock"}}},
		Usage:   chat.Usage{TotalTokens: 1},
	}, nil
}

func (p *e2eProvider) ChatStream(_ context.Context, msgs []chat.Message, _ chat.ChatOptions) (chat.ChatStream, error) {
	p.strCalls.Add(1)
	p.store(msgs)
	if p.streamFactory == nil {
		return nil, errors.New("e2e: no stream configured")
	}
	return p.streamFactory()
}

// ─────────────────────────────────────────────────────────────────────────────
// Harness: httptest.Server montando o RunHandler real (REUSE) sobre /v1/run
// ─────────────────────────────────────────────────────────────────────────────

// newHandler builds a RunHandler wired to a selected e2e provider on the global
// registry, plus any extra wiring via opts. Every helper here resets the global
// registry + execution store so tests isolate cleanly (same as the handler tests).
func newHandler(t *testing.T, prov *e2eProvider, sessDir string, opts ...func(*handler.RunHandler)) *handler.RunHandler {
	t.Helper()
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	reg := chat.GetRegistry()
	reg.Register("e2e-provider", func(_ context.Context, _ map[string]interface{}) (chat.ChatProvider, error) {
		return prov, nil
	}, "e2e provider", 1)
	if err := reg.Select(nil, chat.ChatRegistryConfig{Primary: "e2e-provider", AutoDetect: false}); err != nil {
		t.Fatalf("select e2e provider: %v", err)
	}
	h := handler.NewRunHandler(agents.NewManager(""), reg, nil)
	if sessDir != "" {
		h.SetSessionsDir(sessDir)
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

// knowSearch is a deterministic KnowledgeSearcher feeding the Kernel's evidence.
type knowSearch struct {
	results *orchestration.KnowledgeSearchResults
}

func (k *knowSearch) Search(_ context.Context, _ orchestration.KnowledgeSearchParams) (*orchestration.KnowledgeSearchResults, error) {
	return k.results, nil
}

// kernelWeights bias the convergence so deliberation reaches EmitOK with two
// corroborating knowledge hits (same ruler as the engine-level tests).
func kernelWeights() deliberate.ConvergenceWeights {
	return deliberate.ConvergenceWeights{Recommendation: 0.70, Premises: 0.30}
}

// withDeliberationEnables opts the handler into Kernel-First Deliberation (ADR-032).
func withDeliberationEnabled(h *handler.RunHandler) {
	h.SetDeliberateConfig(orchestration.DeliberateConfig{Enabled: true, Weights: kernelWeights()})
}

// newMux mounts the REAL RunHandler Execute/Stream onto an http.ServeMux.
func newMux(h *handler.RunHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/run", h.Execute)
	mux.HandleFunc("POST /v1/run/stream", h.Stream)
	return mux
}

// newServer starts an httptest.Server serving the RunHandler and returns it.
func newServer(h *handler.RunHandler) *httptest.Server {
	return httptest.NewServer(newMux(h))
}

// newSDK creates a pkg/cosca Client pointed at the httptest server (host:port).
func newSDK(t *testing.T, srv *httptest.Server) *cosca.Client {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url %q: %v", srv.URL, err)
	}
	c, err := cosca.NewClient(cosca.ClientConfig{
		RuntimeAddr: u.Host,
		Timeout:     15 * time.Second,
		MaxRetries:  1,
	})
	if err != nil {
		t.Fatalf("new cosca client: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// sdkRun wraps a real HTTP POST /v1/run via the SDK and returns the result.
func sdkRun(t *testing.T, c *cosca.Client, prompt string, opts ...cosca.RunOption) *cosca.RunResult {
	t.Helper()
	res, err := c.Orchestration.Run(context.Background(), prompt, opts...)
	if err != nil {
		t.Fatalf("SDK Run(%q): %v", prompt, err)
	}
	return res
}

// drainStream reads every SDK StreamEvent until the channel closes.
func drainStream(t *testing.T, ch <-chan cosca.StreamEvent) []cosca.StreamEvent {
	t.Helper()
	var evs []cosca.StreamEvent
	for ev := range ch {
		evs = append(evs, ev)
	}
	return evs
}

// waitGoroutinesSettled waits (deterministically up to ~2s) for goroutines to
// return to <= before, absorbing transient timers/GC.
func waitGoroutinesSettled(t *testing.T, before int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runtime.Gosched()
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > before {
		t.Fatalf("goroutine leak: before=%d after=%d", before, got)
	}
}

// rawSSEEvents performs a plain POST to /v1/run/stream (no SDK) and returns the
// RAW ordered list of SSE JSON events — used to verify the full WIRE contract
// (thinking → status:processing → progress* → response* → metadata →
// status:done → done), which the SDK's typed StreamEvent intentionally collapses.
func rawSSEEvents(t *testing.T, srvURL string, body string) []map[string]interface{} {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srvURL+"/v1/run/stream", strings.NewReader(body))
	if err != nil {
		t.Fatalf("raw request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("raw stream do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("raw stream read: %v", err)
	}
	return parseSSEJSON(t, string(data))
}

// parseSSEJSON parses a body of `data: {...}\n\n` blocks into ordered maps.
func parseSSEJSON(t *testing.T, body string) []map[string]interface{} {
	t.Helper()
	var out []map[string]interface{}
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
		out = append(out, m)
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// PROVA A — SDK.Run → kernel resolve (EmitOK) → resposta SEM LLM
// ─────────────────────────────────────────────────────────────────────────────

// TestE2E_SDK_KernelResolve_NoLLM prows the END-TO-END 'resolve' branch: a
// pkg/cosca client posts to a real HTTP /v1/run; the Kernel-First Deliberation
// (ADR-032) converges on the injected knowledge (reuse of the knowledge searcher,
// NOT a parallel abstraction) and responds DETERMINISTICALLY — the mock provider
// is registered and selected but its Chat/ChatStream are NEVER called.
func TestE2E_SDK_KernelResolve_NoLLM(t *testing.T) {
	prov := &e2eProvider{name: "e2e", model: "e2e-model"}
	h := newHandler(t, prov, "", withDeliberationEnabled)
	h.SetKnowledgeSearcher(&knowSearch{results: &orchestration.KnowledgeSearchResults{
		Results: []orchestration.KnowledgeSearchResult{
			{ID: "k1", Snippet: "use go for the api", Score: 0.95},
			{ID: "k2", Snippet: "use go for the api", Score: 0.90},
		},
		TotalCount: 2,
		Query:      "build an api",
	}})
	srv := newServer(h)
	defer srv.Close()
	c := newSDK(t, srv)

	res := sdkRun(t, c, "build an api")

	if !strings.Contains(res.Response, "use go for the api") {
		t.Errorf("expected deterministic kernel answer in response, got %q", res.Response)
	}
	if res.Agent == "" {
		t.Error("expected a resolved agent name")
	}
	// PROVA zero-LLM: o provider foi registrado/selecionado mas NUNCA chamado.
	if got := prov.chatCalls.Load(); got != 0 {
		t.Errorf("provider.Chat calls = %d, want 0 (deterministic resolve)", got)
	}
	if got := prov.strCalls.Load(); got != 0 {
		t.Errorf("provider.ChatStream calls = %d, want 0 (deterministic resolve)", got)
	}
	if res.SessionID == "" {
		t.Error("expected a session_id (ETAPA 2) in the run response")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PROVA B — SDK.Stream → kernel escala → Router → Provider mock → streaming
// ─────────────────────────────────────────────────────────────────────────────

// TestE2E_SDK_KernelEscalate_StreamsLLM provs the END-TO-END 'escalate' branch:
// with NO knowledge evidence the Kernel can't converge → Escalate → the engine
// routes to the Router → Provider and streams the mock LLM chunks back through
// the SSE to the pkg/cosca reader, yielding a coherent SDK-visible sequence
// (thinking → response* → done) with no error.
func TestE2E_SDK_KernelEscalate_StreamsLLM(t *testing.T) {
	prov := &e2eProvider{name: "e2e", model: "e2e-model"}
	prov.streamFactory = func() (chat.ChatStream, error) {
		return &e2eStream{chunks: streamChunks("e2e-model", "Hello ", "World")}, nil
	}
	h := newHandler(t, prov, "", withDeliberationEnabled)
	srv := newServer(h)
	defer srv.Close()
	c := newSDK(t, srv)

	ch, err := c.Orchestration.Stream(context.Background(), "build an api")
	if err != nil {
		t.Fatalf("SDK Stream: %v", err)
	}
	evs := drainStream(t, ch)

	if len(evs) < 2 {
		t.Fatalf("expected >=2 SDK events, got %d: %v", len(evs), evs)
	}
	// Primeiro evento "thinking", último "done", sem "error".
	if evs[0].Type != "thinking" {
		t.Errorf("first event = %q, want 'thinking'", evs[0].Type)
	}
	last := evs[len(evs)-1]
	if last.Type != "done" {
		t.Errorf("last event = %q, want 'done'", last.Type)
	}
	var got strings.Builder
	seenDone := false
	for _, ev := range evs {
		switch ev.Type {
		case "response":
			got.WriteString(ev.Content)
		case "done":
			seenDone = true
		case "error":
			t.Errorf("unexpected error event: %+v", ev)
		}
	}
	if !seenDone {
		t.Error("expected a 'done' terminal event")
	}
	if s := got.String(); s != "Hello World" {
		t.Errorf("streamed content = %q, want %q", s, "Hello World")
	}
	// PROVA escala: o provider (mock) FOI chamado em streaming.
	if prov.strCalls.Load() == 0 {
		t.Error("expected the mock provider to be called in streaming (escalate branch)")
	}
	if evs[0].SessionID == "" {
		t.Error("expected session_id on the first SDK event (ETAPA 2)")
	}
}

// TestE2E_Wire_StreamCoherentSequence verifies the FULL wire contract of the
// stream endpoint (ETAPA 4) through a plain HTTP SSE read: thinking →
// status:processing → progress* → response* → metadata → status:done → done,
// in a STABLE order. This complements the SDK test above (which only surfaces
// the SDK-visible thinking/response/done subset).
func TestE2E_Wire_StreamCoherentSequence(t *testing.T) {
	prov := &e2eProvider{name: "e2e", model: "e2e-model"}
	prov.streamFactory = func() (chat.ChatStream, error) {
		return &e2eStream{chunks: streamChunks("e2e-model", "Hello", " world")}, nil
	}
	h := newHandler(t, prov, "")
	srv := newServer(h)
	defer srv.Close()

	evs := rawSSEEvents(t, srv.URL, `{"prompt":"Hi there"}`)
	if len(evs) == 0 {
		t.Fatal("no SSE events captured")
	}
	if evs[0]["type"] != "thinking" {
		t.Errorf("first event = %v, want 'thinking'", evs[0]["type"])
	}
	if evs[len(evs)-1]["type"] != "done" {
		t.Errorf("last event = %v, want 'done'", evs[len(evs)-1]["type"])
	}

	order := make([]string, 0, len(evs))
	for _, ev := range evs {
		if typ, ok := ev["type"].(string); ok {
			order = append(order, typ)
		}
	}
	idxOf := func(v string) int {
		for i, s := range order {
			if s == v {
				return i
			}
		}
		return -1
	}
	idxStatus := func(phase string) int {
		for i, ev := range evs {
			if ev["type"] == "status" {
				if s, ok := ev["status"].(string); ok && s == phase {
					return i
				}
			}
		}
		return -1
	}

	iThink := idxOf("thinking")
	iResp := idxOf("response")
	iMeta := idxOf("metadata")
	iStatusDone := idxStatus("done")
	iDone := idxOf("done")
	if iThink < 0 || iResp < 0 || iMeta < 0 || iStatusDone < 0 || iDone < 0 {
		t.Fatalf("missing canonical markers, order=%v", order)
	}
	if !(iThink < iResp && iResp < iMeta && iMeta < iStatusDone && iStatusDone < iDone) {
		t.Errorf("canonical order violated: order=%v", order)
	}
	// Pelo menos um "progress" e as fases processing/done presentes.
	progress, proc, doneStatus := 0, 0, 0
	for _, ev := range evs {
		switch ev["type"] {
		case "progress":
			progress++
		case "status":
			if s, ok := ev["status"].(string); ok {
				if s == "processing" {
					proc++
				}
				if s == "done" {
					doneStatus++
				}
			}
		}
	}
	if progress < 1 || proc < 1 || doneStatus < 1 {
		t.Errorf("missing additive events: progress=%d status:processing=%d status:done=%d", progress, proc, doneStatus)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PROVA C — Sessão via SDK: persist, resume, fork (ETAPA 2)
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_SDK_Session_PersistResumeFork(t *testing.T) {
	sessDir := t.TempDir()
	prov := &e2eProvider{name: "e2e", model: "e2e-model"}
	prov.streamFactory = func() (chat.ChatStream, error) {
		// fresh stream each turn so a resumed session re-uses an unused stream
		return &e2eStream{chunks: streamChunks("e2e-model", "olá", " de novo")}, nil
	}
	h := newHandler(t, prov, sessDir)
	srv := newServer(h)
	defer srv.Close()
	c := newSDK(t, srv)

	// ── (C1) persist: request com session_id → resposta carrega o mesmo id. ──
	ch, err := c.Orchestration.Stream(context.Background(), "ola", cosca.WithSession("sess1"))
	if err != nil {
		t.Fatalf("SDK Stream persist: %v", err)
	}
	evs := drainStream(t, ch)
	if last := evs[len(evs)-1]; last.Type != "done" || last.SessionID != "sess1" {
		t.Errorf("expected done + session_id=sess1, got %+v", last)
	}

	// ── (C2) resume: mesmo session_id → histórico injetado no provider. ──
	baseline := len(prov.lastMessages())
	ch2, err := c.Orchestration.Stream(context.Background(), "como vai", cosca.WithSession("sess1"))
	if err != nil {
		t.Fatalf("SDK Stream resume: %v", err)
	}
	drainStream(t, ch2)
	after := len(prov.lastMessages())
	if after <= baseline {
		t.Errorf("resume injected no history: lastMsgs before=%d after=%d", baseline, after)
	}
	// system + histórico(2) + user = >= 4.
	if after < 4 {
		t.Errorf("expected >=4 messages (system+history 2+user) on resume, got %d", after)
	}

	// ── (C3) fork: withParentSession cria child com parent_session_id. ──
	ch3, err := c.Orchestration.Stream(context.Background(), "filho", cosca.WithParentSession("sess1"))
	if err != nil {
		t.Fatalf("SDK Stream fork: %v", err)
	}
	drainStream(t, ch3)

	meta, err := readSessionMeta(t, sessDir, "sess1")
	if err != nil {
		t.Fatalf("read sess1 meta: %v", err)
	}
	if meta["type"] != "meta" || meta["id"] != "sess1" {
		t.Errorf("sess1 meta malformed: %v", meta)
	}
	// O child: qualquer arquivo .jsonl que NÃO seja sess1 e carregue parent_session_id.
	child, err := findChildSession(t, sessDir, "sess1")
	if err != nil {
		t.Fatalf("find child session: %v", err)
	}
	childMeta, err := readSessionMeta(t, sessDir, child)
	if err != nil {
		t.Fatalf("read child meta: %v", err)
	}
	if childMeta["parent_session_id"] != "sess1" {
		t.Errorf("child meta must carry parent_session_id=sess1, got %v", childMeta)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PROVA D — Cancelamento via SDK (ETAPA 3): cancela → termina limpo, sem leak
// ─────────────────────────────────────────────────────────────────────────────

// TestE2E_SDK_Cancel_ClosesNoLeak cancels the SDK context in-flight: the stream
// channel closes, NO 'done' is delivered, and no goroutine leaks.
func TestE2E_SDK_Cancel_ClosesNoLeak(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// provider stream: 1 chunk then block until ctx.Done (honours cancellation).
	stream := &e2eStream{chunks: []chat.ChatStreamChunk{contentChunk("m", "partial")}, cancelCh: ctx.Done()}
	prov := &e2eProvider{name: "e2e", model: "m"}
	prov.streamFactory = func() (chat.ChatStream, error) { return stream, nil }
	h := newHandler(t, prov, "")
	srv := newServer(h)
	defer srv.Close()
	c := newSDK(t, srv)

	before := runtime.NumGoroutine()

	ch, err := c.Orchestration.Stream(ctx, "cancel me")
	if err != nil {
		t.Fatalf("SDK Stream: %v", err)
	}

	// Le eventos até ver o chunk "partial" — prova que o pipeline estava EM
	// streaming (resolve/escala real) Antes de cancelar.
	sawStream := false
	for !sawStream {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("stream closed before the 'partial' chunk was delivered")
			}
			if ev.Type == "response" && ev.Content == "partial" {
				sawStream = true
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for streaming chunk")
		}
	}
	if !sawStream {
		t.Fatal("expected the pipeline to be streaming (partial chunk) before cancel")
	}

	cancel()

	// Após cancel: o canal fecha (sem goroutine pendurada) e NÃO emite 'done'.
	for ev := range ch {
		if ev.Type == "done" {
			t.Errorf("cancelled stream must NOT deliver 'done', got %+v", ev)
		}
	}
	waitGoroutinesSettled(t, before)
}

// TestE2E_SDK_Cancelled_Event proves the wire 'cancelled' terminal event reaches
// the SDK reader deterministically: the provider stream returns context.Canceled
// after content (server-side cancellation path), so the SDK reader is still alive
// and observes a final {"type":"cancelled"} — never a 'done'.
func TestE2E_SDK_Cancelled_Event(t *testing.T) {
	stream := &e2eStream{chunks: []chat.ChatStreamChunk{contentChunk("m", "before cancel")}, cancelCh: closedChan()}
	prov := &e2eProvider{name: "e2e", model: "m"}
	prov.streamFactory = func() (chat.ChatStream, error) { return stream, nil }
	h := newHandler(t, prov, "")
	srv := newServer(h)
	defer srv.Close()
	c := newSDK(t, srv)

	before := runtime.NumGoroutine()

	ch, err := c.Orchestration.Stream(context.Background(), "hello")
	if err != nil {
		t.Fatalf("SDK Stream: %v", err)
	}
	evs := drainStream(t, ch)
	if len(evs) == 0 {
		t.Fatal("no SDK events")
	}
	last := evs[len(evs)-1]
	if last.Type != "cancelled" {
		t.Errorf("last event = %q, want 'cancelled'", last.Type)
	}
	if last.Type == "done" {
		t.Error("cancelled stream must NOT deliver 'done'")
	}
	// Fecha o client (recolhe as conexões keep-alive idle do transport) antes de
	// medir goroutines — as readLoop/writeLoop paradas são conexões idle, NÃO leak.
	_ = c.Close()
	waitGoroutinesSettled(t, before)
}

// closedChan returns a channel that is already closed, so the stream's Recv
// immediately returns context.Canceled after the content chunk.
func closedChan() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

// ─────────────────────────────────────────────────────────────────────────────
// Session persistence shadows (best-effort, read-only)
// ─────────────────────────────────────────────────────────────────────────────

type sessionMeta map[string]interface{}

func readSessionJSONL(t *testing.T, dir, id string) []sessionMeta {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, id+".jsonl"))
	if err != nil {
		t.Fatalf("read session %s: %v", id, err)
	}
	var out []sessionMeta
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var m sessionMeta
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("parse session line: %v", err)
		}
		out = append(out, m)
	}
	return out
}

func readSessionMeta(t *testing.T, dir, id string) (sessionMeta, error) {
	t.Helper()
	lines := readSessionJSONL(t, dir, id)
	if len(lines) == 0 {
		return nil, errors.New("no lines")
	}
	return lines[0], nil
}

func findChildSession(t *testing.T, dir, parent string) (string, error) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		name := e.Name()
		if name == parent+".jsonl" {
			continue
		}
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(name, ".jsonl")
		if meta, err := readSessionMeta(t, dir, id); err == nil {
			if pid, _ := meta["parent_session_id"].(string); pid == parent {
				return id, nil
			}
		}
	}
	return "", errors.New("no child session found")
}
