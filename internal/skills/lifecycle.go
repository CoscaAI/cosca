package skills

// LifecycleState é o estado unificado de ciclo de vida de um artefato de skill
// (F1 — Skill como Artefato Governado). É a visão pública do estado; o
// enforcement da fase de admissão é delegado à primitiva `internal/quarantine`
// (pending→validating→promoted|discarded) e o da fase operacional à
// `internal/skills/usage.go` (active→stale→archived). Nenhum mecanismo paralelo
// de estado é criado aqui — apenas a projeção unificada + as regras de transição.
type LifecycleState string

const (
	// LifecycleProposed: a candidata existe (nova/evoluída/importada), ainda
	// não admitida.
	LifecycleProposed LifecycleState = "proposed"
	// LifecycleQuarantined: em admissão (quarantine pending/validating).
	LifecycleQuarantined LifecycleState = "quarantined"
	// LifecycleValidated: passou o gate determinístico; aguarda ativação.
	LifecycleValidated LifecycleState = "validated"
	// LifecycleActive: em uso em produção.
	LifecycleActive LifecycleState = "active"
	// LifecycleDeprecated: aposentada (aging/regressão/substituição). NUNCA é
	// apagada — apenas marcada (coerente com o princípio do Don).
	LifecycleDeprecated LifecycleState = "deprecated"
)

// ValidOrigins são as origens reconhecidas de um artefato de skill (I3).
const (
	OriginHuman     = "human"
	OriginAgent     = "agent"
	OriginEvolution = "evolution"
	OriginImported  = "imported"
)

// lifecycleTransitions é a tabela de transições permitidas (F1 §5). from==to é
// no-op e sempre permitido (espelha quarantine.applyTransition). O estado
// "quarantined" é a projeção de pending/validating (o sub-estado é imposto pela
// máquina da quarantine, não aqui).
var lifecycleTransitions = map[LifecycleState][]LifecycleState{
	LifecycleProposed:    {LifecycleQuarantined},
	LifecycleQuarantined: {LifecycleValidated, LifecycleDeprecated}, // gate pass → validated; gate fail → deprecated(discarded)
	LifecycleValidated:   {LifecycleActive},                          // ativação (PR revisada)
	LifecycleActive:      {LifecycleDeprecated, LifecycleActive},     // aging/regressão → deprecated; supersede → active(vNova)
	LifecycleDeprecated:  {LifecycleActive},                          // rollback/restauração de versão do ledger
}

// DefaultLifecycle retorna o estado padrão de uma skill sem governança
// explícita: active (cláusula de avô — skills existentes em uso não quebram).
func DefaultLifecycle() LifecycleState { return LifecycleActive }

// ValidLifecycle reports whether st é um estado reconhecido.
func ValidLifecycle(st LifecycleState) bool {
	switch st {
	case LifecycleProposed, LifecycleQuarantined, LifecycleValidated,
		LifecycleActive, LifecycleDeprecated:
		return true
	}
	return false
}

// CanTransition decide se a transição from→to é permitida pela máquina de
// estados. É determinística e pura (I1): não consulta modelo nem rede.
// from==to é no-op (allowed), coerente com quarantine.applyTransition.
func CanTransition(from, to LifecycleState) bool {
	if from == to {
		return true
	}
	if !ValidLifecycle(from) || !ValidLifecycle(to) {
		return false
	}
	for _, next := range lifecycleTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// IsTerminalLifecycle reports se o estado é terminal (não deve continuar
// evoluindo sem intervenção). Deprecated só sai via rollback explícito.
func IsTerminalLifecycle(st LifecycleState) bool { return st == LifecycleDeprecated }

// IsActiveLifecycle reports se o estado é de produção (ativo).
func IsActiveLifecycle(st LifecycleState) bool { return st == LifecycleActive }
