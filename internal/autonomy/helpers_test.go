package autonomy

import (
	"context"
	"sync"
	"time"
)

// ── Mocks injetáveis (o caminho crítico do gate NUNCA abre um LLM) ──────────

// countingRunner é um CommandRunner fake que devolve um resultado fixo e conta
// as execuções (para o change-detection: gate NÃO re-executado → contagem 1).
type countingRunner struct {
	mu     sync.Mutex
	calls  int
	result CommandResult
	// err simula um ERRO DE INFRAESTRUTURA do runner (start/wait), que o
	// gate deve tratar como falha terminante (fail-closed), distinto de um
	// exit-code normal.
	err error
	// perCommand conta execuções por comando (útil no multi-gate).
	perCommand map[string]int
}

func (r *countingRunner) Run(_ context.Context, command, _ string, _ time.Duration) (CommandResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.perCommand == nil {
		r.perCommand = map[string]int{}
	}
	r.perCommand[command]++
	if r.err != nil {
		return r.result, r.err
	}
	return r.result, nil
}

// calls returns the number of executions observed.
func (r *countingRunner) callsSoFar() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// seqRunner devolve o erro/resultado da N-ésima execução (para testar retry
// com resultado variável, ex.: falha → falha → sucesso).
type seqRunner struct {
	mu      sync.Mutex
	results []CommandResult
	idx     int
	calls   int
}

func (r *seqRunner) Run(_ context.Context, command, _ string, _ time.Duration) (CommandResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.idx >= len(r.results) {
		return r.results[len(r.results)-1], nil
	}
	out := r.results[r.idx]
	r.idx++
	return out, nil
}

// staticSnap é um Snapshotter que sempre devolve o MESMO snapshot (workspace
// congelado — base do teste "gate não re-executa quando inalterado").
type staticSnap struct {
	snap WorkspaceSnapshot
}

func (s *staticSnap) Capture(context.Context, string) (WorkspaceSnapshot, error) {
	return s.snap, nil
}

// mutableSnap é um Snapshotter cujo workspace o teste pode MUTAR entre
// chamadas (inalterado → mesmo snapshot; alterado → snapshot diferente).
type mutableSnap struct {
	current WorkspaceSnapshot
}

func (m *mutableSnap) Capture(context.Context, string) (WorkspaceSnapshot, error) {
	return m.current, nil
}

// sampleBuildFail é um resultado de gate "problema de build" (exit ≠ 0).
func sampleBuildFail() CommandResult {
	return CommandResult{ExitCode: 2, Stdout: "# github.com/x\nerror: ", Stderr: "compile failed"}
}

// sampleBuildOK é um resultado de gate "build passou" (exit 0).
func sampleBuildOK() CommandResult {
	return CommandResult{ExitCode: 0, Stdout: "ok", Stderr: ""}
}
