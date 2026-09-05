package autonomy

import (
	"context"
	"testing"
	"time"
)

// snapA/snapB são snapshots de workspace distintos (change-detection).
func snapA() WorkspaceSnapshot { return WorkspaceSnapshot{Status: "?? go.mod", Diff: "a", UntrackedHash: "h1"} }
func snapB() WorkspaceSnapshot { return WorkspaceSnapshot{Status: "M  main.go", Diff: "b", UntrackedHash: "h2"} }

// TestChangeDetection_UnchangedSkipsGate — (c) workspace INALTERADO desde a
// última falha → o gate NÃO é re-executado (contagem de execuções = 1); quando
// o workspace MUDOU → re-executa (contagem = 2). Idempotência de verificação.
func TestChangeDetection_UnchangedSkipsGate(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	s := New(Config{Enabled: true, Gates: []GateConfig{{Command: "go test ./...", MaxRetries: 3}}})
	s.Enable(now)
	dir := t.TempDir()
	snap := &mutableSnap{current: snapA()}
	runner := &countingRunner{result: sampleBuildFail()}

	// (1) Primeira execução: gate FALHA (exit ≠0) e grava o snapshot pós-falha.
	out, err := evaluateQualityGates(context.Background(), s, dir, snap, runner)
	if err != nil {
		t.Fatalf("call1: %v", err)
	}
	if out != GateOutcomeFailed {
		t.Fatalf("call1 outcome = %q, want failed", out)
	}
	if runner.callsSoFar() != 1 {
		t.Fatalf("call1: runner.calls = %d, want 1 (gate rodou)", runner.callsSoFar())
	}
	if s.LastGateFailure == nil || s.LastGateFailure.Attempt != 1 {
		t.Fatalf("call1: LastGateFailure = %+v, want attempt=1", s.LastGateFailure)
	}

	// (2) Workspace INALTERADO → gate NÃO re-executa (contagem continua em 1).
	out, err = evaluateQualityGates(context.Background(), s, dir, snap, runner)
	if err != nil {
		t.Fatalf("call2: %v", err)
	}
	if out != GateOutcomeFailed {
		t.Fatalf("call2 outcome = %q, want failed", out)
	}
	if runner.callsSoFar() != 1 {
		t.Fatalf("call2 (inalterado): runner.calls = %d, want STILL 1 (gate NÃO re-executou)", runner.callsSoFar())
	}
	if s.LastGateFailure.Attempt != 2 {
		t.Fatalf("call2: attempt = %d, want 2 (incrementada sem re-executar)", s.LastGateFailure.Attempt)
	}
	if s.LastGateFailure.ExitText != "not rerun: workspace unchanged since previous failed gate" {
		t.Fatalf("call2: ExitText = %q (change-detection)", s.LastGateFailure.ExitText)
	}

	// (3) Workspace MUDOU → gate RE-EXECUTA (contagem sobe para 2).
	snap.current = snapB()
	out, err = evaluateQualityGates(context.Background(), s, dir, snap, runner)
	if err != nil {
		t.Fatalf("call3: %v", err)
	}
	if out != GateOutcomeFailed {
		t.Fatalf("call3 outcome = %q, want failed", out)
	}
	if runner.callsSoFar() != 2 {
		t.Fatalf("call3 (mudou): runner.calls = %d, want 2 (gate RE-executou)", runner.callsSoFar())
	}
	if s.LastGateFailure.Attempt != 3 {
		t.Fatalf("call3: attempt = %d, want 3", s.LastGateFailure.Attempt)
	}
}

// TestChangeDetection_RetryExhausted — quando o retry do gate esgota (proveniente
// do change-detection: workspace inalterado), a avaliação devolve
// GateOutcomeRetryExhausted e o loop NÃO continua.
func TestChangeDetection_RetryExhausted(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	s := New(Config{Enabled: true, Gates: []GateConfig{{Command: "go test", MaxRetries: 1}}})
	s.Enable(now)
	dir := t.TempDir()
	snap := &mutableSnap{current: snapA()}
	runner := &countingRunner{result: sampleBuildFail()}

	out, err := evaluateQualityGates(context.Background(), s, dir, snap, runner)
	if err != nil || out != GateOutcomeFailed {
		t.Fatalf("call1: out=%q err=%v, want failed", out, err)
	}
	// Workspace inalterado → tenta de novo, mas esgota (attempt=2 > maxRetries=1).
	out, err = evaluateQualityGates(context.Background(), s, dir, snap, runner)
	if err != nil {
		t.Fatalf("call2: %v", err)
	}
	if out != GateOutcomeRetryExhausted {
		t.Fatalf("call2 outcome = %q, want retry_exhausted", out)
	}
	if runner.callsSoFar() != 1 {
		t.Fatalf("esgotado SEM re-executar: runner.calls = %d, want 1", runner.callsSoFar())
	}
	// ShouldContinue propaga o veredito fail-closed.
	d, err := ShouldContinue(context.Background(), s, dir, snap, runner, now)
	if err != nil {
		t.Fatalf("ShouldContinue: %v", err)
	}
	if d.Continue {
		t.Fatalf("retry exhausted deveria NÃO continuar, got Continue=true")
	}
}

// TestChangeDetection_PassClearsFailure — quando um gate que já falhou PASSAR
// (workspace editado → re-executado → exit 0), o historial de falha é limpo.
func TestChangeDetection_PassClearsFailure(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	s := New(Config{Enabled: true, Gates: []GateConfig{{Command: "go build ./...", MaxRetries: 3}}})
	s.Enable(now)
	dir := t.TempDir()
	snap := &mutableSnap{current: snapA()}
	runner := &seqRunner{results: []CommandResult{sampleBuildFail(), sampleBuildOK()}}

	// Falha (grava snapshot A).
	if out, err := evaluateQualityGates(context.Background(), s, dir, snap, runner); err != nil || out != GateOutcomeFailed {
		t.Fatalf("call1: out=%q err=%v, want failed", out, err)
	}
	// Workspace muda e o gate agora PASSAR → limpa o historial + outcome passed.
	snap.current = snapB()
	if out, err := evaluateQualityGates(context.Background(), s, dir, snap, runner); err != nil || out != GateOutcomePassed {
		t.Fatalf("call2: out=%q err=%v, want passed", out, err)
	}
	if s.LastGateFailure != nil || s.LastGateFailureSnapshot != nil {
		t.Fatalf("passou → historial de falha deveria ser limpo, got %+v / %+v", s.LastGateFailure, s.LastGateFailureSnapshot)
	}
	if s.GateAttempts["go build ./..."] != 0 {
		t.Fatalf("passou → contador do comando deveria zerar, got %d", s.GateAttempts["go build ./..."])
	}
}

// TestMultiGate_AllMustPass — TODOS os gates precisam passar para o veredito ser
// "passed"; um só falhando → "failed" (a evidência é conjunta, não parcial).
func TestMultiGate_AllMustPass(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	s := New(Config{Enabled: true, Gates: []GateConfig{
		{Command: "go build ./...", MaxRetries: 2},
		{Command: "go test ./...", MaxRetries: 2},
	}})
	s.Enable(now)
	dir := t.TempDir()

	// Ambos passam → passed.
	ok := &countingRunner{result: sampleBuildOK()}
	if out, err := evaluateQualityGates(context.Background(), s, dir, &staticSnap{}, ok); err != nil || out != GateOutcomePassed {
		t.Fatalf("ambos passam: out=%q err=%v, want passed", out, err)
	}
	if ok.callsSoFar() != 2 {
		t.Fatalf("ambos passam: runner.calls = %d, want 2 (2 gates)", ok.callsSoFar())
	}

	// Um falha → failed (sem evidência conjunta).
	s2 := New(Config{Enabled: true, Gates: []GateConfig{
		{Command: "go build ./...", MaxRetries: 2},
		{Command: "go test ./...", MaxRetries: 2},
	}})
	s2.Enable(now)
	flaky := &seqRunner{results: []CommandResult{sampleBuildOK(), sampleBuildFail()}}
	if out, err := evaluateQualityGates(context.Background(), s2, dir, &staticSnap{}, flaky); err != nil || out != GateOutcomeFailed {
		t.Fatalf("um falha: out=%q err=%v, want failed", out, err)
	}
	if s2.LastGateFailure == nil || s2.LastGateFailure.Command != "go test ./..." {
		t.Fatalf("última falha deveria ser o gate 'go test', got %+v", s2.LastGateFailure)
	}
}

// TestWorkspaceSnapshot_Equal — snapshot é igual somente quando TODOS os três
// componentes (status/diff/untrackedHash) batem.
func TestWorkspaceSnapshot_Equal(t *testing.T) {
	a := snapA()
	if !a.Equal(snapA()) {
		t.Fatal("snapshots idênticos deveriam ser iguais")
	}
	if a.Equal(snapB()) {
		t.Fatal("snapshots diferentes NÃO deveriam ser iguais")
	}
	// Diff só mudando → diferente.
	if a.Equal(WorkspaceSnapshot{Status: a.Status, Diff: "xx", UntrackedHash: a.UntrackedHash}) {
		t.Fatal("diff diferente deveria ser diferente")
	}
	// UntrackedHash só mudando → diferente.
	if a.Equal(WorkspaceSnapshot{Status: a.Status, Diff: a.Diff, UntrackedHash: "rrr"}) {
		t.Fatal("untrackedHash diferente deveria ser diferente")
	}
}
