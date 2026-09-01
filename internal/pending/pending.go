// Package pending — PENDING RESOLUTION (decisão do Don + professor, 2026-09-01).
//
// "Resolve a pendência encarnada mesmo que o step atinja o limite — só
// finaliza, não inventa. Só pra terminar o que estava quase terminado."
//
// Quando o executor atinge STEP_LIMIT com uma pendência objetiva ainda aberta,
// o COSCA NÃO reinicia o raciocínio, NÃO cria tarefa nova, NÃO chama o LLM
// para "pensar no que fazer" — ele olha o ESTADO e pergunta deterministicamente:
//
//	existe pendência resolvível a partir do estado atual?
//	    ├── SIM → devolve a continuação MÍNIMA (a ação já implicada)
//	    └── NÃO → FINALIZE / ESCALATE (nunca inventa próximo passo)
//
// REGRA DE OURO (professor): a recuperação só executa o que JÁ ESTÁ IMPLICADO
// pelo estado existente. Sem ação determinística derivável → CANNOT_RESOLVE →
// ESCALATE. E há um LIMITE PRÓPRIO de recuperação (recoverySteps) — separado
// do limite normal de steps — para não virar loop infinito (o problema do
// Trader: eventos gerando Continue gerando eventos).
//
// Puro e determinístico: sem LLM, sem opinião. Opera sobre os campos do
// TaskState que JÁ existem (PendingActions, Checkpoints, Observations) — não
// inventa estado novo.
package pending

import (
	"github.com/CoscaAI/cosca/internal/task"
)

// Verdict é o resultado da inspeção de pendências.
type Verdict string

const (
	// Resolve indica que existe uma pendência resolvível: devolver a ação
	// mínima implicada para continuar com limite próprio.
	Resolve Verdict = "RESOLVE"
	// CannotResolve indica que não há ação determinística derivável —
	// finalizar/escalar (fail-closed, nunca inventa).
	CannotResolve Verdict = "CANNOT_RESOLVE"
	// NothingPending indica que não há pendência aberta — a task pode
	// finalizar limpa.
	NothingPending Verdict = "NOTHING_PENDING"
)

// Result é a saída da inspeção.
type Result struct {
	// Verdict decide o caminho: Resolve | CannotResolve | NothingPending.
	Verdict Verdict
	// Action é a continuação MÍNIMA implicada (só quando Verdict=Resolve).
	Action string
	// Reason é a justificativa determinística (para audit/trace).
	Reason string
}

// State é o subconjunto do TaskState que a inspeção precisa (evita acoplar o
// pacote ao orchestrator; o chamador projeta o TaskState aqui).
type State struct {
	// PendingActions são as ações aguardando execução.
	PendingActions []string
	// Checkpoints são as etapas concluídas.
	Checkpoints []string
	// Observations são leituras/evidências do runtime.
	Observations []string
	// CurrentStep é o passo atual (para o relatório).
	CurrentStep int
	// MaxSteps é o teto de steps NORMAL (para o relatório).
	MaxSteps int
}

// Resolver decide se uma pendência pode ser resolvida a partir do estado.
type Resolver struct {
	// recoverySteps é o teto próprio de recuperações (default 2 — o
	// professor: normal_steps=20, recovery_steps=2). Nunca vira loop.
	recoverySteps int
	// recoveryUsed contabiliza recuperações já gastas.
	recoveryUsed int
}

// New cria um Resolver com o teto próprio de recuperação.
func New(recoverySteps int) *Resolver {
	if recoverySteps <= 0 {
		recoverySteps = 2
	}
	return &Resolver{recoverySteps: recoverySteps}
}

// RecoveryRemaining devolve quantas recuperações ainda restam.
func (r *Resolver) RecoveryRemaining() int {
	left := r.recoverySteps - r.recoveryUsed
	if left < 0 {
		return 0
	}
	return left
}

// Inspect devolve o veredito sobre o estado. Regras determinísticas:
//
//  1. Sem pendências → NothingPending (a task pode finalizar limpa).
//  2. Pendência que é apenas "registrar resultado" (já observado) e ainda há
//     recuperação disponível → Resolve (ação mínima: persistir observação).
//  3. Pendência que NÃO é derivável do estado (não sabemos como resolver) →
//     CannotResolve (finalizar/escalar — nunca inventa).
//  4. Recuperações esgotadas → CannotResolve (não vira loop).
func (r *Resolver) Inspect(st State) Result {
	if len(st.PendingActions) == 0 {
		return Result{Verdict: NothingPending, Reason: "sem pendências abertas — task finaliza limpa"}
	}

	// Limite próprio de recuperação: se esgotou, não tenta de novo (loop-safe).
	if r.RecoveryRemaining() <= 0 {
		return Result{
			Verdict: CannotResolve,
			Reason:  "limite de recuperação esgotado — sem mais tentativas de pendência",
		}
	}

	// Pendências resolvíveis: ações cuja execução está IMPLICADA pelo estado
	// (ex.: "persistir observação" quando já existe observação; "confirmar
	// checkpoint" quando a etapa já consta). Só resolvemos o que o estado
	// prova — nunca inferimos um próximo passo novo.
	for _, action := range st.PendingActions {
		switch {
		case isObservationPersist(action, st):
			r.recoveryUsed++
			return Result{
				Verdict: Resolve,
				Action:  action,
				Reason:  "pendência 'registrar observação' resolvível — a observação já existe no estado",
			}
		case isCheckpointConfirm(action, st):
			r.recoveryUsed++
			return Result{
				Verdict: Resolve,
				Action:  action,
				Reason:  "pendência 'confirmar checkpoint' resolvível — a etapa já consta nos checkpoints",
			}
		}
	}

	// Nenhuma pendência derivável → não inventa próximo passo.
	return Result{
		Verdict: CannotResolve,
		Reason:  "pendências não resolvíveis a partir do estado — sem ação determinística (não inventa)",
	}
}

// isObservationPersist reconhece a pendência "persistir observação" — a mais
// comum no limite de steps: a task executou, observou, e só falta registrar.
// Resolvível QUANDO existe observação no estado (a ação está implicada).
func isObservationPersist(action string, st State) bool {
	actionLower := lower(action)
	if !containsAny(actionLower, "persist", "registrar", "record", "salvar", "save", "write_result") {
		return false
	}
	return len(st.Observations) > 0
}

// isCheckpointConfirm reconhece a pendência "confirmar etapa concluída" —
// resolvível quando a etapa já está nos checkpoints (a confirmação é
// idempotente e está implicada).
func isCheckpointConfirm(action string, st State) bool {
	actionLower := lower(action)
	if !containsAny(actionLower, "checkpoint", "confirm", "etapa", "concluir", "finalizar") {
		return false
	}
	return len(st.Checkpoints) > 0
}

// lower é um lower-case sem dependência externa.
func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// containsAny reporta se a string contém algum dos termos.
func containsAny(s string, terms ...string) bool {
	for _, t := range terms {
		if len(t) > 0 && indexOf(s, t) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// FromTaskState projeta um TaskState no subconjunto que a inspeção precisa.
func FromTaskState(st *task.TaskState, maxSteps int) State {
	if st == nil {
		return State{}
	}
	return State{
		PendingActions: append([]string(nil), st.PendingActions...),
		Checkpoints:    append([]string(nil), st.Checkpoints...),
		Observations:   append([]string(nil), st.Observations...),
		CurrentStep:    st.CurrentStep,
		MaxSteps:       maxSteps,
	}
}
