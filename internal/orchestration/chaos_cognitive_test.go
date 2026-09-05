package orchestration

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/stallwatch"
)

// chaosProvider simula um provider LLM caótico: às vezes stall (não responde
// até o deadline), às vezes falha transiente, às vezes responde.
type chaosProvider struct {
	name       string
	model      string
	stallEvery int // a cada N chamadas, stall
	failEvery  int // a cada M chamadas, falha transiente imediata
	call       atomic.Int64
}

func (p *chaosProvider) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	n := p.call.Add(1)
	if p.stallEvery > 0 && n%int64(p.stallEvery) == 0 {
		<-ctx.Done() // stall: espera o deadline
		return nil, ctx.Err()
	}
	if p.failEvery > 0 && n%int64(p.failEvery) == 0 {
		return nil, chaosErr{} // erro transiente imediato
	}
	return &chat.ChatResponse{
		Model: p.model,
		Choices: []chat.Choice{{
			Index:        0,
			Message:      chat.Message{Role: chat.RoleAssistant, Content: "chaos ok"},
			FinishReason: chat.FinishReasonStop,
		}},
	}, nil
}

func (p *chaosProvider) ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
	return nil, nil
}

func (p *chaosProvider) Model() string { return p.model }
func (p *chaosProvider) Name() string  { return p.name }
func (p *chaosProvider) Close() error  { return nil }

type chaosErr struct{}

func (chaosErr) Error() string { return "chaos: transient provider error" }

// ── P2 (v2): Chaos Cognitive Test — bateria de escala ──────────────────
//
// Resposta aos 4 pontos do professor:
//   1. Os erros são CONTROLADOS? → cada sessão falha com retry_count <=
//      MaxRetries e é TERMINAL (o executor retorna erro, não pendura).
//   2. Os sucessos chegam ao estado terminal? → resposta não vazia (COMPLETED).
//   3. Não há duplicação? → cada sessão não excede o orçamento de tentativas
//      (MaxRetries+1); o chaos não duplica execuções.
//   4. Stallwatch interpretado: stall → detecção → retry → TERMINAL (nunca
//      retry infinito).
//
// A bateria roda com concorrência crescente: 20 / 50 / 100 / 250 / 500.
// Indicador central: hung = 0 em TODAS as cargas.

type chaosResult struct {
	completed  int
	failed     int
	hung       int
	overBudget int // sessões que excederam o orçamento de tentativas (duplicação)
	emptyResp  int // sucessos sem resposta (estado terminal inválido)
}

func runChaosLoad(t *testing.T, sessions int, maxRetries int) chaosResult {
	t.Helper()
	collector := stallwatch.NewCollector()

	var wg sync.WaitGroup
	var res chaosResult
	var mu sync.Mutex

	for i := 0; i < sessions; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(seed)))
			p := &chaosProvider{
				name:       fmt.Sprintf("chaos-%d", seed),
				model:      "gpt-4o",
				stallEvery: 1 + rng.Intn(3),
				failEvery:  2 + rng.Intn(4),
			}
			cfg := DefaultExecutorConfig()
			cfg.MaxRetries = maxRetries
			cfg.RetryDelay = time.Millisecond
			cfg.Timeout = 15 * time.Millisecond

			exec := NewExecutor(p, cfg, nil)
			exec.SetStallCollector(collector)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			start := p.call.Load()
			resp, err := exec.Execute(ctx, NewPipelineContext(fmt.Sprintf("req-%d", seed), "task"))
			used := p.call.Load() - start

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				// PONTO 1: erro controlado — tentativas dentro do orçamento
				// (MaxRetries+1) e TERMINAL (o executor retornou, não pendurou).
				res.failed++
				if used > int64(maxRetries+1) {
					res.overBudget++
				}
				return
			}
			// PONTO 2: estado terminal válido — sucesso com resposta.
			res.completed++
			if resp.Data.LLMResponse == "" {
				res.emptyResp++
			}
			// PONTO 3: sem duplicação — tentativas dentro do orçamento.
			if used > int64(maxRetries+1) {
				res.overBudget++
			}
		}(i)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		mu.Lock()
		res.hung = sessions // nenhuma terminou a tempo
		mu.Unlock()
		t.Fatalf("load %d: HUNG — sessions did not complete in 60s", sessions)
	}

	report := collector.Report()
	t.Logf("  load=%d → completed=%d failed=%d hung=%d | stalls=%d retries=%d recovered=%d terminal_failed=%d | dur_ok",
		sessions, res.completed, res.failed, res.hung,
		report.TotalStalls, report.TotalRetries, report.Recovered, report.Failed)
	return res
}

func TestChaos_ScaleBattery(t *testing.T) {
	loads := []int{20, 50, 100, 250, 500}
	const maxRetries = 3

	t.Log("=== CHAOS COGNITIVE — BATERIA DE ESCALA (stall+transient, MaxRetries=3) ===")
	t.Log("  colunas: completed | failed | hung | stalls | retries | recovered | terminal_failed")
	for _, load := range loads {
		r := runChaosLoad(t, load, maxRetries)

		// PONTO CENTRAL: hung = 0 em TODAS as cargas.
		if r.hung != 0 {
			t.Fatalf("load %d: hung = %d — sistema pendurou sob carga!", load, r.hung)
		}
		// PONTO 1/3: erros dentro do orçamento (sem duplicação/excesso).
		if r.overBudget != 0 {
			t.Fatalf("load %d: %d sessões excederam o orçamento de tentativas (duplicação)", load, r.overBudget)
		}
		// PONTO 2: sucessos com resposta terminal válida.
		if r.emptyResp != 0 {
			t.Fatalf("load %d: %d sucessos sem resposta (estado terminal inválido)", load, r.emptyResp)
		}
		// Contabilidade fecha.
		if r.completed+r.failed != load {
			t.Fatalf("load %d: accounting %d+%d != %d", load, r.completed, r.failed, load)
		}
	}
	t.Log("=== INVARIANTES: hung=0, duplicação=0, estado terminal válido, contabilidade fechada ===")
}

// TestChaos_RetriesAreTerminal prova o PONTO 4: o stallwatch nunca faz retry
// infinito — o terminal_failed é sempre 1 por sessão que esgotou.
func TestChaos_RetriesAreTerminal(t *testing.T) {
	p := &chaosProvider{name: "terminal", model: "m", stallEvery: 1, failEvery: 0} // sempre stall
	cfg := DefaultExecutorConfig()
	cfg.MaxRetries = 2
	cfg.RetryDelay = time.Millisecond
	cfg.Timeout = 10 * time.Millisecond

	collector := stallwatch.NewCollector()
	exec := NewExecutor(p, cfg, nil)
	exec.SetStallCollector(collector)

	_, err := exec.Execute(context.Background(), NewPipelineContext("req", "x"))
	if err == nil {
		t.Fatal("expected terminal error")
	}
	r := collector.Report()
	// 3 tentativas (1 + 2 retries) → exatamente 1 terminal failure, e o
	// stallwatch NÃO continuou retentando além do orçamento.
	if r.Failed != 1 {
		t.Fatalf("terminal failures = %d, want 1 (no infinite retry)", r.Failed)
	}
	if r.TotalStalls != 3 {
		t.Fatalf("stalls = %d, want 3 (1 initial + 2 retries)", r.TotalStalls)
	}
}
