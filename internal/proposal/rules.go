package proposal

import (
	"path/filepath"
	"strings"
)

// ─── A Lei Imutável ──────────────────────────────────────────────────────────
//
// O Kernel isolado NÃO julga por opinião — julga contra a lei. Estas são as
// regras duras da família: determinísticas, imunes a manipulação de IA,
// inspiradas na CONSTITUTION e nas lições registradas (ex: L213 — nunca
// pkill; sempre kill -TERM por PID exato).

// Severity classifica o peso de uma regra.
type Severity string

const (
	// SevFatal: violar a regra é DENY imediato, sem discussão.
	SevFatal Severity = "fatal"
	// SevGuard: violar a regra não mata a proposta, mas exige o Don no loop.
	SevGuard Severity = "guard"
)

// Rule é uma regra da lei imutável. Check devolve a razão da violação
// (string vazia = regra respeitada).
type Rule struct {
	// ID identifica a regra (R01, R02, ...) para auditoria e forense.
	ID string `json:"id"`
	// Summary resume a regra em uma linha.
	Summary string `json:"summary"`
	// Severity é fatal (DENY) ou guard (exige Don).
	Severity Severity `json:"severity"`
	// Check avalia a proposta contra a regra.
	Check func(*Proposal) string `json:"-"`
}

// RuleHit registra uma regra acionada na validação.
type RuleHit struct {
	RuleID   string `json:"rule_id"`
	Severity Severity `json:"severity"`
	Reason   string `json:"reason"`
}

// help — palavra-chave auxiliar para detectar comandos que matam processos.
func mentionsProcKill(action string) bool {
	a := strings.ToLower(action)
	for _, kw := range []string{"pkill", "killall", "kill -9", "kill -kill"} {
		if strings.Contains(a, kw) {
			return true
		}
	}
	return false
}

// help — detecta operações de reescrita destrutiva de histórico git.
func mentionsGitRewrite(action string) bool {
	a := strings.ToLower(action)
	return strings.Contains(a, "git reset") ||
		strings.Contains(a, "force push") ||
		strings.Contains(a, "push --force") ||
		strings.Contains(a, "git push -f") ||
		strings.Contains(a, "git rebase") ||
		strings.Contains(a, "git filter")
}

// ─── Alvos de autoridade protegidos (FROZEN > LIVE > RUNTIME) ───────────────
//
// Estes são os alvos do Contrato de Autoridade (docs/specs/frozen-live-authority-contract.md):
//   - FROZEN: internal/embed/cosca/   (compilado no binário via go:embed; autoridade semântica)
//   - LIVE:   .opencode/cosca/        (superfície operacional ao OpenCode; editável, NUNCA removida)
//   - RUNTIME: .cosca/                (estado derivado, regenerável)
//
// Uma operação destrutiva (RemoveAll/rm -rf/delete) dentro de um destes alvos é
// FAIL-CLOSED: não executa. O contrato (§5) manda "nenhuma rotina pode deletar/
// conteúdo-destruir o LIVE/G2" e "não escrever implicitamente no FROZEN/G5".

type protectedZone struct {
	root string
	zone string
}

var protectedZones = []protectedZone{
	{root: "internal/embed/cosca", zone: "FROZEN"},
	{root: ".opencode/cosca", zone: "LIVE"},
	{root: ".opencode", zone: "LIVE"},
	{root: ".cosca", zone: "RUNTIME"},
}

// canonicalRmPath normaliza um alvo para comparação determinística de caminho:
// separador canônico "/", resolve "./" e "..", caixa normalizada. Assim a mesma
// árvore é identificada de forma idêntica em qualquer SO (o bug histórico de
// filepath.Rel no Windows não se repete aqui).
func canonicalRmPath(target string) string {
	if target == "" {
		return ""
	}
	return strings.ToLower(filepath.ToSlash(filepath.Clean(target)))
}

// withinRoot reporta se o caminho canônico t é igual ou está sob root como
// SEGMENTO de caminho completo. Isto evita o falso-positivo de prefixo de
// string (".cosca-backup" NÃO é protegido por ".cosca") e cobre caminhos
// absolutos (ex.: C:\...\cosca\.opencode\cosca).
func withinRoot(t, root string) bool {
	root = strings.TrimSuffix(root, "/")
	if root == "" {
		return false
	}
	if t == root {
		return true
	}
	if strings.HasPrefix(t, root+"/") { // t sob root (root é ancestral)
		return true
	}
	if strings.HasSuffix(t, "/"+root) { // root aparece no fim (caminho absoluto)
		return true
	}
	return strings.Contains(t, "/"+root+"/") // root como segmento no meio
}

// IsProtectedRmTarget reporta se o alvo pertence a uma zona de autoridade
// protegida do contrato (FROZEN/LIVE/RUNTIME). Uma operação destrutiva contra
// ele DEVE ser FAIL-CLOSED (não executar). Consulta determinística, read-only.
func IsProtectedRmTarget(target string) bool {
	return ProtectedZoneOf(target) != ""
}

// ProtectedZoneOf devolve a zona de autoridade (FROZEN/LIVE/RUNTIME) do alvo,
// ou "" se o alvo não for protegido. Fail-closed de consulta: se o caminho não
// puder ser associado a uma zona explicitamente, NÃO é bloqueado pelo guard.
func ProtectedZoneOf(target string) string {
	t := canonicalRmPath(target)
	if t == "" {
		return ""
	}
	for _, z := range protectedZones {
		if withinRoot(t, z.root) {
			return z.zone
		}
	}
	return ""
}

// help — detecta remoção recursiva de diretórios sensíveis.
func sensitiveRmTarget(target string) bool {
	t := canonicalRmPath(target)
	if t == "" {
		return false
	}
	// Alvos de autoridade (FROZEN/LIVE/RUNTIME) — remoção é FAIL-CLOSED.
	for _, z := range protectedZones {
		if withinRoot(t, z.root) {
			return true
		}
	}
	// Legado: raízes e artefatos críticos sempre foram alvos sensíveis.
	for _, s := range []string{
		"/", "/*",
		".git",
		"knowledge.db", "gate.db", "audit.db",
		"$home", "~",
		"/home", "/etc", "/usr", "/var", "/boot",
	} {
		sNorm := strings.TrimSuffix(strings.ToLower(filepath.ToSlash(filepath.Clean(s))), "/")
		if sNorm == "" {
			continue
		}
		if t == sNorm || strings.HasPrefix(t, sNorm+"/") || strings.HasSuffix(t, "/"+sNorm) {
			return true
		}
	}
	return false
}

// LawRules é a lei imutável do Kernel isolado — a CONSTITUTION em código.
var LawRules = []Rule{
	{
		ID:       "R01",
		Summary:  "Nunca pkill — sempre kill -TERM por PID exato (L213)",
		Severity: SevFatal,
		Check: func(p *Proposal) string {
			if mentionsProcKill(p.Action) {
				return "ação contém pkill/killall/kill -9 — regra da família: kill -TERM por PID exato (L213)"
			}
			return ""
		},
	},
	{
		ID:       "R02",
		Summary:  "Reescrita de histórico git (reset/force push) é destrutiva — exige Don",
		Severity: SevGuard,
		Check: func(p *Proposal) string {
			if mentionsGitRewrite(p.Action) {
				return "operação de reescrita git detectada — exige aprovação do Don (nunca sozinho)"
			}
			return ""
		},
	},
	{
		ID:       "R03",
		Summary:  "Remoção recursiva em diretório sensível é proibida sem Don",
		Severity: SevGuard,
		Check: func(p *Proposal) string {
			if strings.Contains(strings.ToLower(p.Action), "rm -rf") && sensitiveRmTarget(p.Target) {
				return "rm -rf em alvo sensível — proibido sem aprovação do Don"
			}
			return ""
		},
	},
	{
		ID:       "R04",
		Summary:  "Proposta deve declarar classe de risco verdadeira",
		Severity: SevFatal,
		Check: func(p *Proposal) string {
			if p.Risk == "" {
				return "classe de risco não declarada — contrato mínimo violado"
			}
			return ""
		},
	},
	{
		ID:       "R05",
		Summary:  "Ação sem alvo claro é suspeita (proposta vaga = julgamento cego)",
		Severity: SevFatal,
		Check: func(p *Proposal) string {
			if strings.TrimSpace(p.Target) == "" && !strings.Contains(strings.ToLower(p.Risk.String()), "trivial") {
				return "ação sem alvo claro — o Kernel isolado não julga proposta vaga"
			}
			return ""
		},
	},
	{
		ID:       "R06",
		Summary:  "Destrutiva sem plano de mitigação declarado exige revisão",
		Severity: SevGuard,
		Check: func(p *Proposal) string {
			if p.RequiresDon() && !strings.Contains(strings.ToLower(p.State), "backup") &&
				!strings.Contains(strings.ToLower(p.State), "rollback") &&
				!strings.Contains(strings.ToLower(p.State), "mitigação") &&
				!strings.Contains(strings.ToLower(p.State), "mitigacao") {
				return "ação destrutiva sem backup/rollback/mitigação declarado no estado — exige Don"
			}
			return ""
		},
	},
}

// CheckLaw aplica a lei imutável sobre a proposta e devolve todas as
// violações encontradas (fatais e de guarda).
func CheckLaw(p *Proposal) []RuleHit {
	var hits []RuleHit
	for _, r := range LawRules {
		if reason := r.Check(p); reason != "" {
			hits = append(hits, RuleHit{RuleID: r.ID, Severity: r.Severity, Reason: reason})
		}
	}
	return hits
}

// fatalHits filtra apenas as violações fatais.
func fatalHits(hits []RuleHit) []RuleHit {
	var out []RuleHit
	for _, h := range hits {
		if h.Severity == SevFatal {
			out = append(out, h)
		}
	}
	return out
}

// hasFatal informa se existe alguma violação fatal.
func hasFatal(hits []RuleHit) bool { return len(fatalHits(hits)) > 0 }

// ruleIDs extrai os IDs das regras acionadas.
func ruleIDs(hits []RuleHit) []string {
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.RuleID)
	}
	return ids
}
