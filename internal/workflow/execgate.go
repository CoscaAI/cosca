// execgate.go — Engine-gated execution (Fase 2 do ADR-023; mineração n8n
// EngineRequest). O I1 DENTRO do workflow.
//
// Um nó de IA devolve uma PROPOSTA TIPADA (intenção + args + ref). O gate do
// engine — determinístico, fail-closed — decide se pode executar. O LLM propõe;
// o sistema decide. Só executa se aprovado; a seguir o nó resume.
//
// Compõe I1 (LLM propõe / sistema decide), I2 (fail-closed: high-risk → DENY),
// I8 (proposta é dado, não autoridade — o gate é a fronteira).
package workflow

import (
	"strings"
)

// Proposal é a intenção tipada que um nó de IA propõe.
type Proposal struct {
	Intent string         `json:"intent"`
	Args   map[string]any `json:"args"`
	Ref    string         `json:"ref"` // proveniência (I3)
}

// Risk é a classe de risco da proposta (determinística por intenção).
type Risk int

const (
	RiskLow Risk = iota    // leitura / computação pura
	RiskMedium             // mutação de estado (transação)
	RiskHigh               // shell/rede/escrita/destrutivo
)

func (r Risk) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	default:
		return "unknown"
	}
}

// Verdict é o veredito do gate.
type Verdict string

const (
	VerdictApprove       Verdict = "approve"
	VerdictDeny          Verdict = "deny"
	VerdictNeedsApproval Verdict = "needs_approval"
)

// ExecGate decide se uma proposta pode executar.
type ExecGate struct {
	// DenyHighRisk: high-risk → DENY (fail-closed I2). Default true.
	DenyHighRisk bool
}

// NewExecGate cria um gate com fail-closed por padrão.
func NewExecGate() *ExecGate { return &ExecGate{DenyHighRisk: true} }

// Evaluate decide o veredito da proposta. Determinístico (I1).
//   - high-risk  → DENY (fail-closed) ou NeedsApproval se DenyHighRisk=false
//   - medium     → NeedsApproval (gate do Don)
//   - low        → Approve
func (g *ExecGate) Evaluate(p Proposal) (Verdict, string) {
	risk := classifyRisk(p.Intent)
	switch risk {
	case RiskHigh:
		if g.DenyHighRisk {
			return VerdictDeny, "high-risk action (I2: fail-closed)"
		}
		return VerdictNeedsApproval, "high-risk action (approval gate)"
	case RiskMedium:
		return VerdictNeedsApproval, "state-mutating action (approval gate)"
	default:
		return VerdictApprove, "low-risk read/compute"
	}
}

// classifyRisk: heurística determinística por intenção (como policy.toolCapability).
func classifyRisk(intent string) Risk {
	i := strings.ToLower(strings.TrimSpace(intent))
	// Destrutivo/de alto privilégio → High.
	for _, k := range []string{"delete", "remove", "rm", "destroy", "shell", "exec", "bash",
		"network", "upload", "send", "http", "fetch", "write", "create", "deploy", "grant", "sudo"} {
		if strings.Contains(i, k) {
			return RiskHigh
		}
	}
	// Mutação de estado → Medium.
	for _, k := range []string{"update", "edit", "set", "mutate", "place", "add", "change", "move"} {
		if strings.Contains(i, k) {
			return RiskMedium
		}
	}
	return RiskLow
}
