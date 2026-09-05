// Package policy — o guard determinístico da casa (GOVERNANCE_PROTOCOL, L366).
//
// Nível 2 da hierarquia de autoridade: regras avaliáveis POR CÓDIGO, entre a
// proposta (ACTION_PROPOSAL) e a execução — independentes do LLM. O runtime
// (nível 3, sandbox/jaula) é a última barreira; o policy é a primeira camada
// determinística que responde ALLOW / DENY / CONFIRM / ESCALATE.
//
// Regras de base: CONSTITUTION.md (P1 segurança, P8 embed, P10 autoridade,
// P13 evidência) + as leis da família (L56 — nunca pkill, sempre kill -TERM;
// L351 — transformação sem backup é DENY; L358 — ação destrutiva DENY por
// padrão).
package policy

import "strings"

// Decision é o veredito determinístico do guard.
type Decision int

const (
	// Allow — a ação pode prosseguir (leitura, escrita comum no projeto).
	Allow Decision = iota
	// Deny — a ação é bloqueada por regra (destrutiva, fora de escopo).
	Deny
	// Confirm — a ação requer confirmação explícita (humano/Don).
	Confirm
	// Escalate — a ação requer revisão superior (risco alto, impacto amplo).
	Escalate
)

func (d Decision) String() string {
	switch d {
	case Allow:
		return "ALLOW"
	case Deny:
		return "DENY"
	case Confirm:
		return "CONFIRM"
	case Escalate:
		return "ESCALATE"
	default:
		return "UNKNOWN"
	}
}

// Action descreve uma proposta de execução (ACTION_PROPOSAL).
type Action struct {
	// Tool é o nome da ferramenta ("bash", "write", "edit", "read", ...).
	Tool string
	// Args são os parâmetros crus da chamada.
	Args map[string]interface{}
	// RawCommand é o comando efetivo (extraído de Args["command"] para bash).
	RawCommand string
	// TargetPath é o alvo de arquivo (extraído de Args["path"/"filePath"]).
	TargetPath string
}

// NewAction monta uma Action a partir da chamada de ferramenta, extraindo o
// comando e o alvo dos argumentos crus (sem depender do LLM para a decisão).
func NewAction(tool string, args map[string]interface{}) Action {
	return Action{
		Tool:       tool,
		Args:       args,
		RawCommand: extractCommand(tool, args),
		TargetPath: strArg(args, "path", "filePath", "file"),
	}
}

// Evaluation é o veredito + a regra que o produziu.
type Evaluation struct {
	Decision Decision
	Rule     string
	Reason   string
}

// Rule é uma regra avaliável: quando Match(ação) é true, a decisão se aplica.
type Rule struct {
	Name     string
	Match    func(a Action) bool
	Decision Decision
	Reason   string
}

// Engine avalia ações contra um conjunto ordenado de regras (a primeira
// regra que casa vence).
type Engine struct {
	rules []Rule
}

// New cria um Engine com as regras padrão da casa.
func New() *Engine {
	return &Engine{rules: DefaultRules()}
}

// WithRules cria um Engine com regras customizadas (testes/contextos).
func WithRules(rules []Rule) *Engine {
	return &Engine{rules: rules}
}

// Evaluate aplica as regras em ordem; a primeira que casa decide. Nenhuma
// regra casa → Allow (com a razão explícita).
func (e *Engine) Evaluate(a Action) Evaluation {
	if e == nil {
		return Evaluation{Allow, "no-policy", "sem guard configurado"}
	}
	for _, r := range e.rules {
		if r.Match(a) {
			return Evaluation{r.Decision, r.Name, r.Reason}
		}
	}
	return Evaluation{Allow, "default", "nenhuma regra específica — permitido"}
}

// ── Helpers de extração ────────────────────────────────────────────────────

func strArg(args map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := args[k].(string); ok {
			return v
		}
	}
	return ""
}

// extractCommand monta o comando efetivo a partir dos args da ferramenta.
func extractCommand(tool string, args map[string]interface{}) string {
	switch tool {
	case "bash", "exec", "run":
		return strArg(args, "command", "cmd")
	case "write", "edit", "delete", "remove":
		return tool + " " + strArg(args, "path", "filePath", "file")
	default:
		return tool
	}
}

// ── Regras padrão da casa (DefaultRules) ───────────────────────────────────

// DefaultRules devolve as regras da casa na ordem de precedência:
// destruição → pkill → embed → produção → escrita comum → leitura.
func DefaultRules() []Rule {
	has := func(s, sub string) bool { return strings.Contains(s, sub) }

	return []Rule{
		// 1. Ação destrutiva: DENY por padrão (P1, L358).
		{
			Name: "destructive-command",
			Match: func(a Action) bool {
				c := a.RawCommand
				return has(c, "rm -rf") || has(c, "rm -r ") || has(c, "rm -fr") ||
					has(c, "DROP TABLE") || has(c, "TRUNCATE") || has(c, "DROP DATABASE") ||
					has(c, "git reset --hard") || has(c, "git clean -f") ||
					has(c, "mkfs") || has(c, "dd if=") || has(c, ":(){ :|:& };:") ||
					has(c, "DROP INDEX") || has(c, "DELETE FROM")
			},
			Decision: Deny,
			Reason:   "ação destrutiva — DENY por padrão; requer confirmação explícita do Don (P1, L358)",
		},
		// 2. pkill/killall: DENY (L56 — nunca pkill; sempre kill -TERM <pid>).
		{
			Name: "pkill-forbidden",
			Match: func(a Action) bool {
				return has(a.RawCommand, "pkill") || has(a.RawCommand, "killall")
			},
			Decision: Deny,
			Reason:   "pkill proibido (L56) — use kill -TERM <pid> com o PID explícito",
		},
		// 3. Escrita/remoção no cérebro (embed): CONFIRM (P8).
		{
			Name: "embed-write",
			Match: func(a Action) bool {
				return (a.Tool == "write" || a.Tool == "edit" || a.Tool == "delete" ||
					has(a.RawCommand, "internal/embed/cosca")) &&
					has(a.TargetPath, "internal/embed/cosca") && !has(a.RawCommand, "memory/agent/cosca-kernel/blocks")
			},
			Decision: Confirm,
			Reason:   "escrita no cérebro (internal/embed/cosca) requer confirmação explícita (P8)",
		},
		// 4. Transformação em produção (schema/dados reais): CONFIRM.
		{
			Name: "production-transform",
			Match: func(a Action) bool {
				c := a.RawCommand
				return (has(c, "ALTER TABLE") || has(c, "DROP ") || has(c, "DELETE FROM") ||
					has(c, "UPDATE ") && has(c, ".cosca")) &&
					(has(c, ".cosca/knowledge.db") || has(c, ".cosca"))
			},
			Decision: Confirm,
			Reason:   "transformação de dados reais (produção) requer aprovação + backup (L351)",
		},
		// 5. git forçado: CONFIRM.
		{
			Name: "git-force",
			Match: func(a Action) bool {
				return has(a.RawCommand, "git push --force") || has(a.RawCommand, "git push -f")
			},
			Decision: Confirm,
			Reason:   "push forçado requer confirmação (risco de reescrita de histórico)",
		},
		// 6. Leitura: Allow (qualquer ferramenta de leitura).
		{
			Name: "read-only",
			Match: func(a Action) bool {
				switch a.Tool {
				case "read", "glob", "grep", "search", "list", "stat", "diff", "status":
					return true
				}
				return false
			},
			Decision: Allow,
			Reason:   "operação de leitura — permitida",
		},
	}
}
