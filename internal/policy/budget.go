package policy

// DefaultMaxToolCalls é o orçamento padrão de tool-calls por turno
// (ADR-011, Bloco 2; mcp-policy maxToolCallsPerTurn).
const DefaultMaxToolCalls = 200

// ToolBudget é um contador de tool-calls com um teto (orçamento por turno).
// Quando o orçamento estoura, Consume retorna false e o turno deve parar de
// chamar tools (controle determinístico, sem depender do LLM).
type ToolBudget struct {
	// Calls é o número de tool-calls já consumidas no turno corrente.
	Calls int
	// Max é o teto de chamadas por turno. Se <= 0, usa DefaultMaxToolCalls.
	Max int
}

// NewToolBudget cria um ToolBudget com o teto informado, normalizando Max <= 0
// para DefaultMaxToolCalls (default da casa = 200).
func NewToolBudget(max int) *ToolBudget {
	if max <= 0 {
		max = DefaultMaxToolCalls
	}
	return &ToolBudget{Max: max}
}

// Consume registra uma tool-call e retorna true enquanto houver orçamento
// (Calls < Max). Retorna false quando o orçamento estourou (Calls >= Max).
// Um ToolBudget nil nunca autoriza a chamada.
func (b *ToolBudget) Consume() bool {
	if b == nil {
		return false
	}
	max := b.Max
	if max <= 0 {
		max = DefaultMaxToolCalls
	}
	if b.Calls >= max {
		return false
	}
	b.Calls++
	return true
}

// Reset zera o contador de tool-calls (início de novo turno).
func (b *ToolBudget) Reset() {
	if b == nil {
		return
	}
	b.Calls = 0
}
