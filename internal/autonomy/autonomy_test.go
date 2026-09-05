package autonomy

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestLimitReason_Bounds — (a) os BOUNDS param o loop: cada teto
// (continuations/turns/tokens/timeout) dispara sua própria razão de limite.
func TestLimitReason_Bounds(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

	t.Run("max_continuations", func(t *testing.T) {
		s := New(Config{Enabled: true, Limits: Limits{MaxContinuations: 2}})
		s.Enable(now)
		s.AddContinuation()
		s.AddContinuation()
		if got, ok := s.LimitReason(now); !ok || got != LimitContinuations {
			t.Fatalf("LimitReason = (%q,%v), want (max_continuations,true)", got, ok)
		}
	})

	t.Run("max_turns", func(t *testing.T) {
		s := New(Config{Enabled: true, Limits: Limits{MaxTurns: 3}})
		s.Enable(now)
		for i := 0; i < 3; i++ {
			s.AddTurn(0, 0, 0)
		}
		if got, ok := s.LimitReason(now); !ok || got != LimitTurns {
			t.Fatalf("LimitReason = (%q,%v), want (max_turns,true)", got, ok)
		}
	})

	t.Run("max_tokens", func(t *testing.T) {
		s := New(Config{Enabled: true, Limits: Limits{MaxTokens: 1000}})
		s.Enable(now)
		s.AddTurn(900, 100, 0) // budget = 1000
		if got, ok := s.LimitReason(now); !ok || got != LimitTokens {
			t.Fatalf("LimitReason = (%q,%v), want (max_tokens,true)", got, ok)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		s := New(Config{Enabled: true, Limits: Limits{Timeout: 10 * time.Minute}})
		s.Enable(now)
		if _, ok := s.LimitReason(now); ok {
			t.Fatal("no início do loop não deveria haver limite (timeout não atingido)")
		}
		if got, ok := s.LimitReason(now.Add(11 * time.Minute)); !ok || got != LimitTimeout {
			t.Fatalf("LimitReason = (%q,%v), want (timeout,true)", got, ok)
		}
	})

	t.Run("under_bounds_no_limit", func(t *testing.T) {
		s := New(Config{Enabled: true, Limits: Limits{MaxContinuations: 5, MaxTurns: 10, MaxTokens: 100000}})
		s.Enable(now)
		s.AddContinuation()
		s.AddTurn(100, 50, 0)
		if got, ok := s.LimitReason(now); ok {
			t.Fatalf("sob limites não deveria haver limite, got %q", got)
		}
	})
}

// TestShouldContinue_BoundsStopLoop — (a) ShouldContinue DEVOLVE não-continue
// com ReasonLimitReached quando um teto é atingido (os bounds param o loop).
func TestShouldContinue_BoundsStopLoop(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	s := New(Config{Enabled: true, Limits: Limits{MaxContinuations: 1}})
	s.Enable(now)
	s.AddContinuation() // 1 >= 1 → limite

	d, err := ShouldContinue(context.Background(), s, "", &staticSnap{}, &countingRunner{}, now)
	if err != nil {
		t.Fatalf("ShouldContinue: %v", err)
	}
	if d.Continue {
		t.Fatalf("no limite deveria NÃO continuar, got Continue=true")
	}
	if d.Reason != ReasonLimitReached || d.Limit != LimitContinuations {
		t.Fatalf("Reason/Limit = %q/%q, want limit_reached/max_continuations", d.Reason, d.Limit)
	}
}

// TestShouldContinue_GateExitCode — (b) o gate é determinístico pelo exit-code,
// MODELO A (prime-agent): exit 0 → qualidade atingida → STOP; exit ≠0 → qualidade
// não atingida → CONTINUE (consertar).
func TestShouldContinue_GateExitCode(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()

	t.Run("exit_0_stops", func(t *testing.T) {
		s := New(Config{Enabled: true, Gates: []GateConfig{{Command: "go test ./..."}}})
		s.Enable(now)
		d, err := ShouldContinue(context.Background(), s, dir, &staticSnap{}, &countingRunner{result: sampleBuildOK()}, now)
		if err != nil {
			t.Fatalf("ShouldContinue: %v", err)
		}
		if d.Continue {
			t.Fatalf("gate exit 0 (qualidade atingida) deveria PARAR, got Continue=true")
		}
		if d.Reason != ReasonNotNeeded {
			t.Fatalf("Reason = %q, want not_needed", d.Reason)
		}
	})

	t.Run("exit_nonzero_continues", func(t *testing.T) {
		s := New(Config{Enabled: true, Gates: []GateConfig{{Command: "go build ./..."}}})
		s.Enable(now)
		d, err := ShouldContinue(context.Background(), s, dir, &staticSnap{}, &countingRunner{result: sampleBuildFail()}, now)
		if err != nil {
			t.Fatalf("ShouldContinue: %v", err)
		}
		if !d.Continue {
			t.Fatalf("gate exit ≠0 (qualidade não atingida) deveria CONTINUAR, got Continue=false")
		}
		if d.Reason != ReasonGateFailed {
			t.Fatalf("Reason = %q, want gate_failed", d.Reason)
		}
	})
}

// TestShouldContinue_GateFailed_Continues — (e, MODELO A) gate que FALHOU
// (exit ≠0, sem evidência terminal) → CONTINUE: o loop segue trabalhando pra
// consertar. O gate foi executado exatamente uma vez (evidência = exit-code fake).
func TestShouldContinue_GateFailed_Continues(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	s := New(Config{Enabled: true, Gates: []GateConfig{{Command: "npm run check"}}})
	s.Enable(now)
	runner := &countingRunner{result: sampleBuildFail()}
	d, err := ShouldContinue(context.Background(), s, t.TempDir(), &staticSnap{}, runner, now)
	if err != nil {
		t.Fatalf("ShouldContinue: %v", err)
	}
	if !d.Continue {
		t.Fatal("gate falhou (sem evidência terminal) → MODELO A deve CONTINUAR")
	}
	if d.Reason != ReasonGateFailed {
		t.Fatalf("Reason = %q, want gate_failed", d.Reason)
	}
	if runner.callsSoFar() != 1 {
		t.Fatalf("runner.calls = %d, want 1 (gate rodou uma vez)", runner.callsSoFar())
	}
}

// TestShouldContinue_GateError_FailClosed — (e, MODELO A) erro de INFRA no gate
// (comando não encontrado / spawn falhou) → fail-closed: NÃO continua (não se
// sabe o estado) e o erro é propagado. Diferente de gate_failed (qualidade).
func TestShouldContinue_GateError_FailClosed(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	s := New(Config{Enabled: true, Gates: []GateConfig{{Command: "go test"}}})
	s.Enable(now)
	runner := &countingRunner{err: errors.New("exec: not found")}
	d, err := ShouldContinue(context.Background(), s, t.TempDir(), &staticSnap{}, runner, now)
	if err == nil {
		t.Fatal("erro de infra do gate deveria ser propagado")
	}
	if d.Continue {
		t.Fatalf("erro de infra do gate → fail-closed (NÃO continua), got Continue=true")
	}
	if d.Reason != ReasonGateError {
		t.Fatalf("Reason = %q, want gate_error", d.Reason)
	}
}

// TestShouldContinue_Disabled — autonomia desligada → nada a decidir
// (non-continue, ReasonDisabled), sem rodar gate.
func TestShouldContinue_Disabled(t *testing.T) {
	s := New(Config{Enabled: false, Gates: []GateConfig{{Command: "go test"}}})
	runner := &countingRunner{result: sampleBuildOK()}
	d, err := ShouldContinue(context.Background(), s, t.TempDir(), &staticSnap{}, runner, time.Now())
	if err != nil {
		t.Fatalf("ShouldContinue: %v", err)
	}
	if d.Continue || d.Reason != ReasonDisabled {
		t.Fatalf("desabilitado: Continue=%v Reason=%q, want false/disabled", d.Continue, d.Reason)
	}
	if runner.callsSoFar() != 0 {
		t.Fatalf("gate NÃO deveria rodar quando disabled, calls=%d", runner.callsSoFar())
	}
}

// TestShouldContinue_NoGates_ContinuesUnderBounds — sem gates e sob limites, o
// loop segue (ReasonMissingEvidence) — o "primeiro caso" do loop autônomo.
func TestShouldContinue_NoGates_ContinuesUnderBounds(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	s := New(Config{Enabled: true, Limits: Limits{MaxContinuations: 5}})
	s.Enable(now)
	d, err := ShouldContinue(context.Background(), s, "", nil, nil, now)
	if err != nil {
		t.Fatalf("ShouldContinue: %v", err)
	}
	if !d.Continue || d.Reason != ReasonMissingEvidence {
		t.Fatalf("sem gates/limites deveria continuar (missing_terminal_evidence), got %t/%q", d.Continue, d.Reason)
	}
}

// TestBudgetDelta — (d) o orçamento da nu-ance ADR-031 soma input+output+cacheWrite
// e EXCLUI cacheRead. Loop longo que só recarrega contexto do cache não esgota.
func TestBudgetDelta(t *testing.T) {
	// 900 uncached + 100 output + 0 cacheWrite = 1000 (cacheRead=900 IGNORADO).
	if got := BudgetDelta(900, 100, 0); got != 1000 {
		t.Fatalf("BudgetDelta(900,100,0) = %d, want 1000 (cacheRead excluído)", got)
	}
	if got := BudgetDelta(900, 100, 50); got != 1050 {
		t.Fatalf("BudgetDelta(900,100,50) = %d, want 1050 (cacheWrite entra)", got)
	}
}

// TestBudgetExcludesCacheRead_LongLoopDoesNotExhaust — (d) loop longo NÃO
// esgota o orçamento por cacheRead. 12 turns, cada um: input=900 (uncached),
// output=100, cacheRead=900 (recontexto repetido). O orçamento conta 1000/turn
// (12k < 15k teto); SE contasse o cacheRead naively (1900/turn), daria 22.8k e
// o loop pararia por max_tokens — desperdício.
func TestBudgetExcludesCacheRead_LongLoopDoesNotExhaust(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	const maxTokens = 15000
	s := New(Config{Enabled: true, Limits: Limits{MaxTokens: maxTokens, MaxTurns: 100, MaxContinuations: 100}})
	s.Enable(now)

	naive := 0
	for i := 0; i < 12; i++ {
		const input, output, cacheRead, cacheWrite = 900, 100, 900, 0
		s.AddTurn(input, output, cacheWrite)
		naive += input + output + cacheRead // o que seria contado SEM a nuance
	}

	if s.TokensUsed != 12000 {
		t.Fatalf("TokensUsed = %d, want 12000 (orçamento exclui cacheRead)", s.TokensUsed)
	}
	if naive != 22800 {
		t.Fatalf("naive sum = %d, want 22800 (demonstra por que a nuance importa)", naive)
	}
	// Com a nuance: loop NÃO esgotou (12000 < 15000) → LimitReason = nil.
	if got, ok := s.LimitReason(now); ok {
		t.Fatalf("com a nuance o loop NÃO deveria esgotar, got limit=%q (naive=%d)", got, naive)
	}
	// Se contássemos cacheRead, o loop EXCEDERIA o teto: prove que o alvo é o cache.
	if naive <= maxTokens {
		t.Fatalf("o cenário não é válido: naive=%d deveria exceder %d", naive, maxTokens)
	}
	d, err := ShouldContinue(context.Background(), s, "", nil, nil, now)
	if err != nil {
		t.Fatalf("ShouldContinue: %v", err)
	}
	if !d.Continue {
		t.Fatalf("loop sob o teto (com nuance) deveria continuar, got %q", d.Reason)
	}
}

// TestAddTurn_Disabled_Noop — AddTurn/AddContinuation são no-op quando a camada
// está desabilitada (não consomem "budget" de loop desligado).
func TestAddTurn_Disabled_Noop(t *testing.T) {
	s := New(Config{Enabled: false})
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	s.AddTurn(1000, 500, 100)
	s.AddContinuation()
	if s.TurnsUsed != 0 || s.TokensUsed != 0 || s.ContinuationsUsed != 0 {
		t.Fatalf("no-op esperado, got turns=%d tokens=%d cont=%d", s.TurnsUsed, s.TokensUsed, s.ContinuationsUsed)
	}
	if _, ok := s.LimitReason(now); ok {
		t.Fatal("estado desabilitado não tem limite")
	}
}

// TestEnable_ResetsCounters — Enable reinicia contadores e historial (pré-requisito
// da reaproveitamento da mesma RuntimeState entre loops).
func TestEnable_ResetsCounters(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	s := New(Config{Enabled: true})
	s.Enable(now)
	s.AddTurn(100, 100, 0)
	s.AddContinuation()
	s.LastGateFailure = &GateFailure{Command: "x", Attempt: 2}
	s.LastGateFailureSnapshot = &WorkspaceSnapshot{Status: "??"}
	if s.TokensUsed == 0 || s.GateAttempts == nil {
		t.Fatal("precondição do estado inicial falhou")
	}
	s.Enable(now) // reinicia
	if s.TokensUsed != 0 || s.ContinuationsUsed != 0 || s.TurnsUsed != 0 {
		t.Fatalf("Enable não zerou contadores: tokens=%d cont=%d turns=%d", s.TokensUsed, s.ContinuationsUsed, s.TurnsUsed)
	}
	if s.LastGateFailure != nil || s.LastGateFailureSnapshot != nil {
		t.Fatal("Enable não limpou o historial de gates")
	}
}

// TestGateFailureStructRoundTrip garante que o GateFailure é simples e legível.
func TestGateFailureStructRoundTrip(t *testing.T) {
	g := GateFailure{Command: "go build ./...", Attempt: 2, ExitText: "exited 1", Output: "compile failed"}
	if g.Command == "" || g.Attempt != 2 || g.ExitText == "" || g.Output == "" {
		t.Fatalf("GateFailure malformado: %+v", g)
	}
}
