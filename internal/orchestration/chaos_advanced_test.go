package orchestration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/stallwatch"
)

// ── P2 v3: Chaos avançado — as dimensões que o professor pediu ─────────
//
// Crashes de runtime · eventos atrasados · contexto conflitante · memória
// parcial · restart do orchestrator. Métricas: sessions, deadlocks=0,
// state_inconsistencies=0, race limpo.

// chaosFatalProvider simula um CRASH de runtime: depois de algumas chamadas,
// emite um erro FATAL não-transiente (como se o processo tivesse morrido).
type chaosFatalProvider struct {
	name    string
	model   string
	crashAt int64 // morre na chamada N
	call    atomic.Int64
}

func (p *chaosFatalProvider) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	n := p.call.Add(1)
	if n >= p.crashAt {
		return nil, fmt.Errorf("runtime: process crashed (fatal, non-transient)")
	}
	return &chat.ChatResponse{
		Model:   p.model,
		Choices: []chat.Choice{{Index: 0, Message: chat.Message{Role: chat.RoleAssistant, Content: "ok before crash"}, FinishReason: chat.FinishReasonStop}},
	}, nil
}

func (p *chaosFatalProvider) ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
	return nil, nil
}

func (p *chaosFatalProvider) Model() string { return p.model }
func (p *chaosFatalProvider) Name() string  { return p.name }
func (p *chaosFatalProvider) Close() error  { return nil }

// chaosDelayedProvider atrasa a resposta ALÉM do timeout (ignora o ctx) —
// o pior caso de "evento atrasado" que o watchdog precisa domar.
type chaosDelayedProvider struct {
	name  string
	model string
	delay time.Duration
}

func (p *chaosDelayedProvider) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	time.Sleep(p.delay) // ignora o ctx — resposta atrasada
	return &chat.ChatResponse{
		Model:   p.model,
		Choices: []chat.Choice{{Index: 0, Message: chat.Message{Role: chat.RoleAssistant, Content: "late but ok"}, FinishReason: chat.FinishReasonStop}},
	}, nil
}

func (p *chaosDelayedProvider) ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
	return nil, nil
}

func (p *chaosDelayedProvider) Model() string { return p.model }
func (p *chaosDelayedProvider) Name() string  { return p.name }
func (p *chaosDelayedProvider) Close() error  { return nil }

// ── Cenário 1: crashes de runtime → terminal SEM retry ─────────────────

func TestChaos_RuntimeCrashesAreTerminalWithoutRetry(t *testing.T) {
	var wg sync.WaitGroup
	var terminal, retried atomic.Int64
	collector := stallwatch.NewCollector()

	const sessions = 30
	for i := 0; i < sessions; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			// crashAt=1: TODAS as sessões morrem na primeira chamada.
			p := &chaosFatalProvider{name: fmt.Sprintf("fatal-%d", seed), model: "m", crashAt: 1}
			cfg := DefaultExecutorConfig()
			cfg.MaxRetries = 3 // mesmo com retries disponíveis...
			cfg.RetryDelay = time.Millisecond
			cfg.Timeout = 50 * time.Millisecond

			exec := NewExecutor(p, cfg, nil)
			exec.SetStallCollector(collector)

			start := p.call.Load()
			_, err := exec.Execute(context.Background(), NewPipelineContext(fmt.Sprintf("crash-%d", seed), "x"))
			used := p.call.Load() - start

			// O executor sanitiza o erro via safeerror (por design — nunca
			// expõe mensagens brutas), mas a FALHA é terminal.
			if err == nil {
				t.Errorf("session %d: crash must be terminal error", seed)
				return
			}
			// INVARIANTE: erro fatal NÃO é transient → exatamente 1 chamada,
			// ZERO retries (mesmo com MaxRetries=3 disponíveis).
			if used > 1 {
				retried.Add(1)
			}
			terminal.Add(1)
		}(i)
	}
	wg.Wait()

	if terminal.Load() != sessions {
		t.Fatalf("terminal = %d, want %d", terminal.Load(), sessions)
	}
	if retried.Load() != 0 {
		t.Fatalf("fatal crashes were retried %d times — non-transient must not retry", retried.Load())
	}
	t.Logf("crashes: %d sessões, todas terminais, ZERO retries (erro fatal não é transient)", terminal.Load())
}

// ── Cenário 2: eventos atrasados → comportamento CORRIGIDO (L324 fechado) ─
//
// O chatWithRetry agora é NÃO-cooperativo (goroutine+select no deadline): um
// provider que dorme além do timeout (ignorando o ctx) NÃO bloqueia o worker.
// O stall é detectado no deadline, retryado, e — como este provider SEMPRE
// demora 200ms >> timeout de 30ms — todas as tentativas stall e a sessão
// termina em erro em TEMPO FINITO, nunca esperando os 200ms do provider.

func TestChaos_DelayedEventsNeverHang(t *testing.T) {
	var wg sync.WaitGroup
	var completed, failed, hung atomic.Int64

	const sessions = 20
	for i := 0; i < sessions; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			p := &chaosDelayedProvider{name: fmt.Sprintf("slow-%d", seed), model: "m", delay: 200 * time.Millisecond}
			cfg := DefaultExecutorConfig()
			cfg.MaxRetries = 1
			cfg.RetryDelay = time.Millisecond
			cfg.Timeout = 30 * time.Millisecond

			exec := NewExecutor(p, cfg, nil)

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			start := time.Now()
			_, err := exec.Execute(ctx, NewPipelineContext(fmt.Sprintf("slow-%d", seed), "x"))
			elapsed := time.Since(start)

			if err == nil {
				completed.Add(1)
			} else {
				failed.Add(1)
			}
			// INVARIANTE: o worker NÃO esperou os 200ms do provider — a
			// sessão terminou em tempo finito (muito abaixo do delay).
			if elapsed > 3*time.Second {
				hung.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if hung.Load() != 0 {
		t.Fatalf("hung = %d — delayed events penduraram sessões", hung.Load())
	}
	// Comportamento corrigido: o provider que sempre atrasa NÃO é aceito — o
	// stall é domado no deadline e a sessão termina em erro (0 completas).
	if completed.Load() != 0 || failed.Load() != sessions {
		t.Fatalf("completas=%d erro=%d, want 0/%d (resposta sempre atrasada → terminal)", completed.Load(), failed.Load(), sessions)
	}
	t.Logf("atraso: %d sessões, %d erro em tempo finito, hung=0 — L324 FECHADO: resposta atrasada não bloqueia mais o worker (stall no deadline)",
		sessions, failed.Load())
}

// ── Cenário 3: contexto conflitante sob carga — sem cross-contamination ─

func TestChaos_ContextIsolationUnderLoad(t *testing.T) {
	var wg sync.WaitGroup
	var contaminated atomic.Int64

	const sessions = 25
	for i := 0; i < sessions; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			// Cada sessão tem conteúdo PRÓPRIO e um provider que responde
			// exatamente o seu marcador.
			mine := fmt.Sprintf("CONTENT-%d", seed)
			p := &chaosProvider{name: fmt.Sprintf("ctx-%d", seed), model: "m", stallEvery: 3, failEvery: 5}
			cfg := DefaultExecutorConfig()
			cfg.MaxRetries = 2
			cfg.RetryDelay = time.Millisecond
			cfg.Timeout = 30 * time.Millisecond
			exec := NewExecutor(p, cfg, nil)

			res, err := exec.Execute(context.Background(), NewPipelineContext(fmt.Sprintf("req-%d", seed), mine))
			if err != nil {
				return // sessão falhou — ok (caos)
			}
			// INVARIANTE de isolamento: a resposta NÃO pode conter o marcador
			// de outra sessão.
			if strings.Contains(res.Data.LLMResponse, "CONTENT-") && !strings.Contains(res.Data.LLMResponse, fmt.Sprintf("CONTENT-%d", seed)) {
				contaminated.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if contaminated.Load() != 0 {
		t.Fatalf("cross-contamination: %d sessões receberam contexto de outra sessão", contaminated.Load())
	}
	t.Logf("isolamento: %d sessões concorrentes, cross-contamination=0", sessions)
}
