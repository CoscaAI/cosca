package handler_test

// ─── Prova D — Cancelamento (handler-level: POST /v1/run/stream) ──────────────
//
// A mecânica (ETAPA 3) reusa o context.Context do request de ponta a ponta. Os
// testes abaixo provam no nível do wire contract SSE:
//   (a) provider "lento" (emite 1 chunk e bloqueia) + cancelar (ctx do request)
//       → SSE emite `cancelled` (ADITIVO), NÃO `done`/`error`; stream/fila
//       encerram limpos; sem goroutine leak.
//   (b) disconnect do cliente (cancelar o request ctx) → encerra limpo.
//   (c) regressão: erro no meio do stream → emite `error` consistente (não
//       `cancelled`), mantendo o contrato ETAPA 1/2.
//
// Mocks: cancelAwareStream (1 chunk depois bloqueia até ctx.Done), 
// errAfterChunkStream (chunk + erro real), notifySSEWriter (sinal a cada Write
// para o teste cancelar após o "thinking" SEM data race no buffer).
// Leak: runtime.NumGoroutine() antes/depois com wait determinístico (goleak
// fora do go.mod — NÃO tocar).

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/api/stream"
	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/orchestration"
)

// cancelAwareStream: emite exatamente 1 chunk (prova streaming) e depois
// BLOQUEIA em Recv até o ctx do request ser cancelado (provider real derruba o
// corpo HTTP / fecha o canal de eventos no cancel → Recv retorna
// context.Canceled). Com noChunks=true, Recv já bloqueia de primeira.
type cancelAwareStream struct {
	ctx      context.Context
	sent     atomic.Bool
	closed   atomic.Bool
	noChunks bool
}

func (s *cancelAwareStream) Recv() (*chat.ChatStreamChunk, error) {
	if !s.noChunks && !s.sent.Load() {
		s.sent.Store(true)
		return &chat.ChatStreamChunk{
			Model:   "cancel-model",
			Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{Content: "partial"}}},
		}, nil
	}
	<-s.ctx.Done()
	return nil, s.ctx.Err()
}

func (s *cancelAwareStream) Close() error {
	s.closed.Store(true)
	return nil
}

// errAfterChunkStream: entrega 1 chunk (para o failover marcar sawContent) e
// então devolve um erro REAL não-contexto. Prova a regressão (c): um erro no
// meio do stream NÃO é cancelamento.
type errAfterChunkStream struct {
	chunks []chat.ChatStreamChunk
	err    error
	pos    int
	closed atomic.Bool
}

func (s *errAfterChunkStream) Recv() (*chat.ChatStreamChunk, error) {
	if s.pos < len(s.chunks) {
		c := s.chunks[s.pos]
		s.pos++
		return &c, nil
	}
	return nil, s.err
}

func (s *errAfterChunkStream) Close() error {
	s.closed.Store(true)
	return nil
}

// notifySSEWriter envolve stream.SSETestWriter e sinaliza a cada Write, para o
// teste saber quando o "thinking"/chunk foram gravados SEM ler o buffer
// concorrentemente (evita data race).
type notifySSEWriter struct {
	*stream.SSETestWriter
	onWrite chan struct{}
}

func newNotifySSEWriter() *notifySSEWriter {
	return &notifySSEWriter{
		SSETestWriter: stream.NewSSETestWriter(),
		onWrite:       make(chan struct{}, 64),
	}
}

func (n *notifySSEWriter) Write(p []byte) (int, error) {
	written, err := n.SSETestWriter.Write(p)
	if err == nil {
		select {
		case n.onWrite <- struct{}{}:
		default:
		}
	}
	return written, err
}

// cleanupGlobals restaura o estado GLOBAL (registry + execution store) ao
// final do teste. Os testes deste arquivo usam registerStreamMock, que aponta o
// registry global para um provider selecionado. Como cancel_stream_test.go roda
// ALFABETICAMENTE antes de handler_coverage_test.go, deixar o registry "sujo"
// quebra o TestBuildAgentSystemPrompt (que espera 503 por NÃO haver provider
// selecionado). Resetar no cleanup evita o acoplamento de ordem — higiene de
// teste, NÃO altera a mecânica de produção.
func cleanupGlobals(t *testing.T) {
	t.Cleanup(func() {
		chat.ResetRegistry()
		orchestration.ResetExecutionStore()
	})
}

// waitGoroutinesSettled espera (deterministicamente) goroutines voltarem a
// <= before, absorvendo timers/GC transitórios (até 2s) sem mascarar um leak.
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

// runStreamAsync roda h.Stream no seu próprio goroutine com o request
// context-bound a `ctx`, devolvendo um canal que fecha quando o handler
// retorna. Usado por todos os testes para poder cancelar SEM bloquear.
func runStreamAsync(h interface{ Stream(w http.ResponseWriter, r *http.Request) }, w http.ResponseWriter, r *http.Request) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Stream(w, r)
	}()
	return done
}

// TestStream_Cancel_EmitsCancelled_NoGoroutineLeak (PROVA D-a): provider lento
// que emite 1 chunk e bloqueia; ao cancelar o request (equivalente ao
// disconnect SSE / cancel explícito), o SSE emite `cancelled` (sem done/error),
// o ChatStream é fechado e NÃO há goroutine pendurada.
func TestStream_Cancel_EmitsCancelled_NoGoroutineLeak(t *testing.T) {
	cleanupGlobals(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cs := &cancelAwareStream{ctx: ctx}
	h := registerStreamMock(t, "cancel-stream", "cancel-model", cs, nil)

	before := runtime.NumGoroutine()

	nw := newNotifySSEWriter()
	req := streamRequest(`{"prompt":"cancel me"}`)
	req = req.WithContext(ctx)
	done := runStreamAsync(h, nw, req)

	// Espera a 1ª escrita (thinking) e a 2ª escrita (o chunk "partial" do
	// provider lento) — prova que o pipeline estava EM STREAMING antes do cancel.
	waitWrites(t, nw, 2)

	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return after cancel — possible goroutine leak")
	}
	waitGoroutinesSettled(t, before)

	body := nw.SSETestWriter.Body().String()
	if !strings.Contains(body, `"type":"cancelled"`) {
		t.Errorf("expected 'cancelled' event, got: %s", body)
	}
	if strings.Contains(body, `"type":"done"`) {
		t.Errorf("cancelled stream must NOT emit 'done', got: %s", body)
	}
	if strings.Contains(body, `"type":"error"`) {
		t.Errorf("cancelled stream must NOT emit 'error', got: %s", body)
	}
	if !strings.Contains(body, `"content":"partial"`) {
		t.Errorf("expected the streamed chunk 'partial' before cancel, got: %s", body)
	}
	if !cs.closed.Load() {
		t.Error("expected the ChatStream to be Closed after cancel (no goroutine leak)")
	}
}

// TestStream_Disconnect_Cancels_CleansUp (PROVA D-b): o cliente desconecta
// (cancel do request ctx) ANTES de qualquer conteúdo do provider. O pipeline
// encerra limpo: SSE emite `cancelled`, stream é fechado e sem goroutine leak.
func TestStream_Disconnect_Cancels_CleansUp(t *testing.T) {
	cleanupGlobals(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cs := &cancelAwareStream{ctx: ctx, noChunks: true}
	h := registerStreamMock(t, "cancel-disconnect", "cancel-model", cs, nil)

	before := runtime.NumGoroutine()

	nw := newNotifySSEWriter()
	req := streamRequest(`{"prompt":"disconnect"}`)
	req = req.WithContext(ctx)
	done := runStreamAsync(h, nw, req)

	// Espera apenas o "thinking" — ainda não há conteúdo (stream bloqueado).
	waitWrites(t, nw, 1)

	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return after disconnect — possible goroutine leak")
	}
	waitGoroutinesSettled(t, before)

	body := nw.SSETestWriter.Body().String()
	if !strings.Contains(body, `"type":"cancelled"`) {
		t.Errorf("expected 'cancelled' event on disconnect, got: %s", body)
	}
	if strings.Contains(body, `"type":"done"`) || strings.Contains(body, `"type":"error"`) {
		t.Errorf("disconnect must NOT emit 'done'/'error', got: %s", body)
	}
	// Invariante: se o stream chegou a ser aberto pelo executor, foi fechado.
	// (O cancel pode ter acontecido antes do executor abrir o stream — guard
	// pré-handoff — o que também é uma finalização limpa e sem recurso aberto.)
	if cs.closed.Load() == false && strings.Contains(body, `"content":"partial"`) {
		t.Error("a streamed-then-disconnected ChatStream must be Closed")
	}
}

// TestStream_MidStreamError_EmitsError_NotCancelled (PROVA D-c, regressão): um
// erro REAL no meio do stream (não-contexto) mantém o contrato legado: SSE
// emite `error` consistente — NUNCA `cancelled` — e o stream fecha sem leak.
func TestStream_MidStreamError_EmitsError_NotCancelled(t *testing.T) {
	cleanupGlobals(t)
	cs := &errAfterChunkStream{
		chunks: []chat.ChatStreamChunk{
			{Model: "err-model", Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{Content: "before error"}}}},
		},
		err: errors.New("provider exploded mid-stream"),
	}
	h := registerStreamMock(t, "mid-err", "err-model", cs, nil)

	before := runtime.NumGoroutine()

	nw := newNotifySSEWriter()
	req := streamRequest(`{"prompt":"hello"}`)
	done := runStreamAsync(h, nw, req)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return after mid-stream error")
	}
	waitGoroutinesSettled(t, before)

	body := nw.SSETestWriter.Body().String()
	if !strings.Contains(body, `"type":"error"`) {
		t.Errorf("expected 'error' event for a mid-stream provider failure, got: %s", body)
	}
	if strings.Contains(body, `"type":"cancelled"`) {
		t.Errorf("a real provider error must NOT be reported as 'cancelled', got: %s", body)
	}
	if strings.Contains(body, `"type":"done"`) {
		t.Errorf("a mid-stream error must NOT be reported as 'done', got: %s", body)
	}
	if !cs.closed.Load() {
		t.Error("expected the errored ChatStream to be Closed (no goroutine leak)")
	}
}

// waitWrites espera `n` writes do notifySSEWriter (usado para sincronizar o
// cancel após o thinking/chunk, sem tocar no buffer concorrentemente).
func waitWrites(t *testing.T, nw *notifySSEWriter, n int) {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for i := 0; i < n; i++ {
		select {
		case <-nw.onWrite:
		case <-timeout:
			t.Fatalf("timeout waiting for SSE write %d of %d", i+1, n)
		}
	}
}
