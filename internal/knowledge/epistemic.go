//
// Estados epistemológicos tipados (CKL) — o sistema sabe quando NÃO sabe.
//
// Conforme a conversa do Don: estados explícitos KNOWN, SUPPORTED,
// UNCERTAIN, CONFLICTING, UNKNOWN e STALE. Uma resposta honesta é:
// "Não possuo evidência suficiente para transformar isso em conhecimento."
//
// DefaultStatus é a ponte de migração: leis existentes sem status ganham um
// estado coerente no load (learning → SUPPORTED, law → KNOWN) sem alterar o
// nível nem a validação existentes.
//

package knowledge

import "time"

// EpistemicStatus é o estado epistemológico de um item de conhecimento.
type EpistemicStatus string

const (
	// StatusKnown é conhecimento estabelecido com evidência.
	StatusKnown EpistemicStatus = "KNOWN"
	// StatusSupported é apoiado por evidência parcial.
	StatusSupported EpistemicStatus = "SUPPORTED"
	// StatusUncertain é evidência insuficiente.
	StatusUncertain EpistemicStatus = "UNCERTAIN"
	// StatusConflicting é evidências contraditórias.
	StatusConflicting EpistemicStatus = "CONFLICTING"
	// StatusUnknown é sem evidência.
	StatusUnknown EpistemicStatus = "UNKNOWN"
	// StatusStale é expirado — requer revalidação.
	StatusStale EpistemicStatus = "STALE"
)

// Valid devolve true quando o status é um dos seis estados epistemológicos.
func (s EpistemicStatus) Valid() bool {
	switch s {
	case StatusKnown, StatusSupported, StatusUncertain, StatusConflicting, StatusUnknown, StatusStale:
		return true
	default:
		return false
	}
}

// Description devolve a descrição pt-BR do estado epistemológico.
func (s EpistemicStatus) Description() string {
	switch s {
	case StatusKnown:
		return "conhecimento estabelecido com evidência"
	case StatusSupported:
		return "apoiado por evidência parcial"
	case StatusUncertain:
		return "evidência insuficiente"
	case StatusConflicting:
		return "evidências contraditórias"
	case StatusUnknown:
		return "sem evidência"
	case StatusStale:
		return "expirado — requer revalidação"
	default:
		return "sem status definido"
	}
}

// DefaultStatus mapeia um nível do CKL para o estado epistemológico default:
// learning → SUPPORTED, law → KNOWN, qualquer outro nível → UNKNOWN.
// Garante a migração segura das leis existentes (que não têm status).
func DefaultStatus(level string) EpistemicStatus {
	switch KnowledgeLevel(level) {
	case LevelLearning:
		return StatusSupported
	case LevelLaw:
		return StatusKnown
	default:
		return StatusUnknown
	}
}

// EpistemicMeta carrega os metadados epistemológicos de um item de
// conhecimento (estado + trilha de verificação).
type EpistemicMeta struct {
	Status             EpistemicStatus `json:"status"`
	LastVerified       time.Time       `json:"last_verified"`
	VerificationCount  int             `json:"verification_count"`
	ContradictionCount int             `json:"contradiction_count"`
}
