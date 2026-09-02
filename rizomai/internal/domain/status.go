// Status agregado do Post é uma FUNÇÃO do estado dos targets (ADR-007 §1):
// materializado para leitura, nunca editado à mão.
package domain

// DerivePostStatus calcula o status agregado a partir dos status dos targets.
//
// Regras (ADR-007 §1):
//   - todos published          → published
//   - todos failed             → failed
//   - algum published          → partial (o caso mais comum na prática)
//   - algum scheduled (não due)→ scheduled (agendado para o futuro)
//   - caso contrário           → publishing (fan-out em andamento)
func DerivePostStatus(targets []TargetStatus) PostStatus {
	if len(targets) == 0 {
		return PostStatusScheduled
	}

	all := func(s TargetStatus) bool {
		for _, t := range targets {
			if t != s {
				return false
			}
		}
		return true
	}
	any := func(s TargetStatus) bool {
		for _, t := range targets {
			if t == s {
				return true
			}
		}
		return false
	}

	switch {
	case all(TargetStatusPublished):
		return PostStatusPublished
	case all(TargetStatusFailed):
		return PostStatusFailed
	case any(TargetStatusPublished):
		return PostStatusPartial
	case any(TargetStatusScheduled):
		return PostStatusScheduled
	default:
		return PostStatusPublishing
	}
}
