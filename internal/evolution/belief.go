// belief.go — Belief Ledger com CONTAINMENT de poison (gema #4, ADR-022;
// mineração RuVector TARL).
//
// Sanidade epistemológica (I4) mecanizada: uma crença SÓ é aceita com PROVA
// (proof-gated, dependências aceitas) — e, se uma crença aceita é REFUTADA,
// TODOS os dependentes aceitos são REBAIXADOS para Pending (containment
// transitivo). Reversível, determinístico (I1), fail-closed (I2).
//
// Isto fecha o I4 do Cosca: conhecimento jamais entra por declaração; e rejeitar
// uma crença falsa NÃO deixa descendentes "aceitos" desatualizados.
package evolution

import "fmt"

// BeliefState é o estado epistemológico de uma crença.
type BeliefState string

const (
	BeliefAccepted BeliefState = "accepted"
	BeliefPending  BeliefState = "pending"
	BeliefRejected BeliefState = "rejected"
)

// Belief é uma proposição com proveniência e dependências.
type Belief struct {
	ID           string      `json:"id"`
	Statement    string      `json:"statement"`
	State        BeliefState `json:"state"`
	Evidence     string      `json:"evidence"`
	Dependencies []string    `json:"dependencies"`
}

// BeliefLedger mantém crenças com aceptação proof-gated + containment.
type BeliefLedger struct {
	beliefs map[string]*Belief
}

// NewBeliefLedger cria um ledgervazio.
func NewBeliefLedger() *BeliefLedger {
	return &BeliefLedger{beliefs: map[string]*Belief{}}
}

// Add registra uma crença como Pending (nunca aceita direto — proof-gated I4).
func (l *BeliefLedger) Add(b Belief) error {
	if b.ID == "" {
		return fmt.Errorf("belief: id required")
	}
	if _, ok := l.beliefs[b.ID]; ok {
		return fmt.Errorf("belief: %q already exists", b.ID)
	}
	b.State = BeliefPending
	cp := b
	l.beliefs[b.ID] = &cp
	return nil
}

// Accept promove Pending→Accepted SÓ se todas as dependências estão aceitas
// (fail-closed I2; proof by dependency — I4). Sem dependência? aceita.
func (l *BeliefLedger) Accept(id string) error {
	b, ok := l.beliefs[id]
	if !ok {
		return fmt.Errorf("belief: %q not found", id)
	}
	if b.State != BeliefPending {
		return fmt.Errorf("belief: %q is %s (must be pending to accept)", id, b.State)
	}
	for _, dep := range b.Dependencies {
		d, ok := l.beliefs[dep]
		if !ok || d.State != BeliefAccepted {
			return fmt.Errorf("belief: dependency %q not accepted (fail-closed I2)", dep)
		}
	}
	b.State = BeliefAccepted
	return nil
}

// Reject rebaixa uma crença para Rejected E faz CONTAINMENT transitivo:
// todo dependente aceito (direta ou transitivamente) volta a Pending. I4/I5.
func (l *BeliefLedger) Reject(id string) error {
	b, ok := l.beliefs[id]
	if !ok {
		return fmt.Errorf("belief: %q not found", id)
	}
	b.State = BeliefRejected
	l.demoteDependents(id)
	return nil
}

// State devolve o estado de uma crença.
func (l *BeliefLedger) State(id string) (BeliefState, bool) {
	b, ok := l.beliefs[id]
	if !ok {
		return "", false
	}
	return b.State, true
}

// demoteDependents: encontra crenças aceitas que dependem de `id` e as rebaixa a
// Pending; repete transitivamente (containment).
func (l *BeliefLedger) demoteDependents(id string) {
	for _, b := range l.beliefs {
		if b.State != BeliefAccepted {
			continue
		}
		for _, dep := range b.Dependencies {
			if dep == id {
				b.State = BeliefPending
				l.demoteDependents(b.ID) // containment transitivo
				break
			}
		}
	}
}
