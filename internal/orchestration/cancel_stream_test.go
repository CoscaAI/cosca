package orchestration

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── Prova D — Cancelamento (engine-level) ────────────────────────────────────
//
// A mecânica de cancelamento (ETAPA 3) é reuso do context.Context já existente:
// ExecuteStream → runStreamingPipeline → Executor.ExecuteStream → readStream.
// Estes testes provam que, ao cancelar o ctx do request, o engine emite um
// estado final DISTINTO "cancelled", fecha o canal UMA vez e NÃO deixa
// goroutine pendurada.

// cancelAwareStream é um chat.ChatStream context-aware que espelha um provider
// real cujo Recv honra o ctx recebido em ChatStream(ctx, ...): um provider real
// derruba o corpo HTTP / fecha o canal de eventos no cancel, fazendo o Recv
// retornar context.Canceled. Aqui usamos o mesmo ctx para que o Recv bloqueie
// até o cancel e retorne context.Canceled — provando o encerramento LIMPO do
// pipeline sem goroutine presa.
//
// Sem `noChunks`: Recv emite exatamente 1 chunk (prova que o pipeline estava de
// fato streaming) e depois BLOQUEIA. Com `noChunks=true`: Recv já bloqueia de
// primeira (disconnect antes de qualquer conteúdo).
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
	// Bloqueia até o ctx do request ser cancelado (provider "lento" / disconnect).
	<-s.ctx.Done()
	return nil, s.ctx.Err()
}

func (s *cancelAwareStream) Close() error {
	s.closed.Store(true)
	return nil
}

// waitGoroutinesSettled espera (deterministicamente) que o número de goroutines
// volte a <= before. Usamos runtime.NumGoroutine() em vez de goleak porque
// goleak não é dependência direta do go.mod. Um wait de até 2s absorve timers/
// GC transitórios sem mascarar um vazamento real (+1+ goroutines).
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

// TestEngine_Stream_Cancel_EmitsCancelled_NoGoroutineLeak prova que, com o
// pipeline em streaming (um chunk já entregue), cancelar o ctx faz o engine:
//  1. emitir StreamEventCancelled (estado final distinto — NÃO "error");
//  2. fechar o canal UMA vez (eventCh);
//  3. fechar o ChatStream (Close → sem recurso pendurado);
//  4. NÃO chamar o provider de novo após o cancel;
//  5. voltar ao baseline de goroutines (sem leak).
func TestEngine_Stream_Cancel_EmitsCancelled_NoGoroutineLeak(t *testing.T) {
	provider := newMockChatProvider("cancel-test", "cancel-model")
	streamFnCalls := 0
	var stream *cancelAwareStream
	provider.streamFn = func(ctx context.Context, _ []chat.Message, _ chat.ChatOptions) (chat.ChatStream, error) {
		streamFnCalls++
		stream = &cancelAwareStream{ctx: ctx}
		return stream, nil
	}

	resolver := newMockAgentResolver()
	resolver.add("canceller", "Canceller", "test", "canceller")
	engine := NewEngine(nil, nil, nil, resolver, nil, provider, DefaultOrchestratorConfig(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	before := runtime.NumGoroutine()

	eventCh, err := engine.ExecuteStream(ctx, &Request{Prompt: "cancel me"})
	if err != nil {
		t.Fatalf("unexpected error starting stream: %v", err)
	}

	gotChunk := false
	gotCancelled := false
	sawClose := false
	timeout := time.After(3 * time.Second)
	for !sawClose {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				sawClose = true
				continue
			}
			switch ev.Type {
			case StreamEventChunk:
				gotChunk = true
				// O pipeline está em streaming — cancela o request.
				cancel()
			case StreamEventCancelled:
				gotCancelled = true
			case StreamEventError:
				// Se o cancel chegar via Recv que já retornou context.Canceled
				// (provider context-aware), o engine deve mapear p/ cancelled
				// e nunca vazar "error".
				t.Errorf("expected StreamEventCancelled after cancel, got StreamEventError: %+v", ev)
			}
		case <-timeout:
			t.Fatal("timeout waiting for stream events after cancel")
		}
	}

	if !gotChunk {
		t.Error("expected at least one chunk before cancel (pipeline was streaming)")
	}
	if !gotCancelled {
		t.Error("expected StreamEventCancelled after ctx cancel")
	}
	if stream == nil {
		t.Fatal("expected the ChatStream to be opened by the engine")
	}
	if !stream.closed.Load() {
		t.Error("expected the ChatStream to be Closed after cancel (no goroutine leak)")
	}
	if streamFnCalls != 1 {
		t.Errorf("expected exactly 1 ChatStream open before/after cancel, got %d", streamFnCalls)
	}
	if got := provider.streamCalled.Load(); got != 1 {
		t.Errorf("provider must NOT be called again after cancel (streamCalled=%d, want 1)", got)
	}
	if got := provider.chatCalled.Load(); got != 0 {
		t.Errorf("Chat() must never be called (chatCalled=%d, want 0)", got)
	}

	// Corrobora: sem goroutine pendurada.
	waitGoroutinesSettled(t, before)
}

// TestEngine_Stream_DisconnectBeforeContent_CleansUp prova que um disconnect
// (ctx cancel) que ocorre ANTES de qualquer conteúdo entregue (Recv nunca
// retornou chunk — provider "lento" de primeira) ainda faz o pipeline encerrar
// LIMPO: canal fecha, stream fecha e sem goroutine leak. O estado emitido é
// cancellod (disconnect), nunca um error de provider.
func TestEngine_Stream_DisconnectBeforeContent_CleansUp(t *testing.T) {
	provider := newMockChatProvider("cancel-disconnect", "cancel-model")
	var stream *cancelAwareStream
	provider.streamFn = func(ctx context.Context, _ []chat.Message, _ chat.ChatOptions) (chat.ChatStream, error) {
		stream = &cancelAwareStream{ctx: ctx, noChunks: true}
		return stream, nil
	}

	resolver := newMockAgentResolver()
	engine := NewEngine(nil, nil, nil, resolver, nil, provider, DefaultOrchestratorConfig(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	before := runtime.NumGoroutine()

	eventCh, err := engine.ExecuteStream(ctx, &Request{Prompt: "disconnect me"})
	if err != nil {
		t.Fatalf("unexpected error starting stream: %v", err)
	}

	// Disconnect imediato: cancela SEM esperar conteúdo.
	cancel()

	gotCancelled := false
	sawClose := false
	timeout := time.After(3 * time.Second)
	for !sawClose {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				sawClose = true
				continue
			}
			if ev.Type == StreamEventCancelled {
				gotCancelled = true
			}
		case <-timeout:
			t.Fatal("timeout waiting for stream close after disconnect")
		}
	}

	if !gotCancelled {
		t.Error("expected StreamEventCancelled on disconnect before content")
	}
	// Invariante de cancelamento: se o ChatStream foi ABERTO (o cancel pode ter
	// acontecido antes do executor abrir o stream, via o guard pré-handoff — o
	// que também é uma finalização limpa), então ele DEVE ter sido fechado. Se
	// o provider nunca foi alcançado, não há recurso a liberar.
	if stream != nil && !stream.closed.Load() {
		t.Error("expected the opened ChatStream to be Closed after disconnect (no goroutine leak)")
	}
	// O provider nunca pode ser re-chamado após o cancel.
	if got := provider.streamCalled.Load(); got > 1 {
		t.Errorf("provider must not be re-opened after cancel (streamCalled=%d, want <= 1)", got)
	}
	waitGoroutinesSettled(t, before)
}
