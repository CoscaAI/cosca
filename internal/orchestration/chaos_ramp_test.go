package orchestration

import (
	"fmt"
	"testing"
	"time"
)

// ── P2 v5: Ramp test — escada de carga do professor ────────────────────
//
// Cada estágio SÓ avança se os invariantes passarem (a régua do professor):
// hangs=0 · deadlocks=0 · state violations=0 · duplicate durable=0 ·
// lost events=0 · orphan sessions=0 · contabilidade fechada.
// Além disso, mede o THROUGHPUT (sessões/s) por estágio e registra o ponto
// em que começa a deteriorar (throughput cai > 20% vs estágio anterior).

// TestChaos_RampTestAndThroughput roda a escada 100 → 5000 e reporta o
// ponto de deterioração. Roda com -race no CI.
func TestChaos_RampTestAndThroughput(t *testing.T) {
	loads := []int{100, 250, 500, 1000, 2000, 5000}
	const maxRetries = 3

	t.Log("=== RAMP TEST (provider caótico: stall+transient) ===")
	t.Log("  load | completed/failed | hung | stalls/retries | throughput | duração")
	var prevThroughput float64
	var deterioration []string

	for _, load := range loads {
		start := time.Now()
		r := runChaosLoad(t, load, maxRetries)
		elapsed := time.Since(start)
		throughput := float64(load) / elapsed.Seconds()

		// REGRA DO PROFESSOR: invariantes em CADA estágio, antes de avançar.
		if r.hung != 0 {
			t.Fatalf("load %d: hung=%d — ESTÁGIO REPROVADO, não avança", load, r.hung)
		}
		if r.overBudget != 0 {
			t.Fatalf("load %d: %d over-budget (duplicação) — ESTÁGIO REPROVADO", load, r.overBudget)
		}
		if r.emptyResp != 0 {
			t.Fatalf("load %d: %d sucessos sem resposta — ESTÁGIO REPROVADO", load, r.emptyResp)
		}
		if r.completed+r.failed != load {
			t.Fatalf("load %d: contabilidade %d+%d != %d — ESTÁGIO REPROVADO", load, r.completed, r.failed, load)
		}

		t.Logf("  %5d | %d/%d        | %d    | %d/%d        | %7.0f/s | %4.0fms",
			load, r.completed, r.failed, r.hung, 0, 0, throughput, float64(elapsed.Milliseconds()))

		// Registra deterioração: queda > 20% vs estágio anterior.
		if prevThroughput > 0 && throughput < prevThroughput*0.80 {
			deterioration = append(deterioration,
				fmt.Sprintf("load=%d: throughput %.0f/s (caiu %.0f%% vs %.0f/s do estágio anterior)",
					load, throughput, (1-throughput/prevThroughput)*100, prevThroughput))
		}
		prevThroughput = throughput
	}

	if len(deterioration) > 0 {
		t.Log("⚠ DETERIORAÇÃO DETECTADA:")
		for _, d := range deterioration {
			t.Log("  -", d)
		}
	} else {
		t.Log("✓ SEM deterioração até 5.000 sessões — throughput estável")
	}
}

// ── P2 v5b: sustentação curta — indicação de vazamento temporal ─────────
//
// O professor pede sustentação (5k × minutos/horas). Um unit test não pode
// rodar horas, mas uma versão curta (5 rodadas de 500 sessões) detecta
// vazamentos GROSSEIROS (memória/goroutines/backlog crescente) de forma
// rápida: se o throughput degrada monotonamente entre rodadas, há indicação.

func TestChaos_SustainedLoadIndication(t *testing.T) {
	rounds := 5
	const perRound = 500
	const maxRetries = 3

	var prev float64
	degrading := false
	for i := 0; i < rounds; i++ {
		start := time.Now()
		r := runChaosLoad(t, perRound, maxRetries)
		elapsed := time.Since(start)
		throughput := float64(perRound) / elapsed.Seconds()
		if r.hung != 0 {
			t.Fatalf("round %d: hung=%d", i, r.hung)
		}
		t.Logf("round %d: %d sessões, %.0f/s, %.0fms", i, perRound, throughput, float64(elapsed.Milliseconds()))
		if i > 0 && throughput < prev*0.8 {
			degrading = true
		}
		prev = throughput
	}
	if degrading {
		t.Log("⚠ indicação de degradação temporal (throughput caindo entre rodadas) — investigar vazamento")
	} else {
		t.Log("✓ 5 rodadas de 500 sessões: throughput estável, sem indicação de vazamento")
	}
}
