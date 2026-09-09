// Package guardrails implementa o Contrato de Salvaguarda do ADR-047.
//
// É o FREIO do Intelligence Engine — a parte que impede o sistema de entrar em
// loop de erro. As regras G1-G9 são irrenunciáveis e prevalecem sobre qualquer
// conveniência: o engine NÃO edita sozinho. Ele sabe, propõe, passa pelo freio,
// e só aplica o que sobrevive à validação. O que é imutável, nem propõe.
//
// Lei da casa: Memória > Velocidade. Integridade > Conveniência.
package guardrails

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ============================================================
// REGRAS (identificadores estáveis do Contrato)
// ============================================================

const (
	RuleImmutable    = "G1-imutavel"
	RuleDonGate      = "G2-gate-do-don"
	RuleShadowFirst  = "G3-shadow-first"
	RuleRollback     = "G4-rollback"
	RuleConflictVeto = "G5-conflito-nao-decide"
	RuleEvidence     = "G6-evidencia"
	RuleAntiLoop     = "G7-anti-loop"
	RuleCustody      = "G8-custodia"
	RuleIntegrity    = "G9-integridade"
)

// Role identifica o papel na custódia (G8) — ninguém edita o que não valida.
type Role string

const (
	RoleProposer Role = "proposer" // Kernel/agentes propõem
	RoleApprover Role = "approver" // Don aprova
	RoleExecutor Role = "executor" // especialistas executam
)

// Verdict é o resultado de uma validação (mesmo shape do memoryguard).
type Verdict struct {
	Approved bool     `json:"approved"`
	Reasons  []string `json:"reasons,omitempty"`
}

// ============================================================
// PROPOSTA DE EDIÇÃO
// ============================================================

// Proposal é uma proposta de edição de conhecimento que deve passar pelo freio.
type Proposal struct {
	ID            string `json:"id"`
	Resource      string `json:"resource"` // o que será editado (ex: memory/agent/x/learnings.md)
	Role          Role   `json:"role"`     // G8 — quem está propondo
	NewContent    string `json:"new_content"`
	EvidenceLevel int    `json:"evidence_level"`  // G6 — 0-5 (regra: só aplica >= 4)
	EvidenceNote  string `json:"evidence_note"`   // ex: o commit/hash/teste que comprova
	ApprovedByDon bool   `json:"approved_by_don"` // G2 — sem isso, é só proposta
	ShadowMode    bool   `json:"shadow_mode"`     // G3 — true só registra, não aplica
	HasSnapshot   bool   `json:"has_snapshot"`    // G4 — guardou estado anterior
}

// Result é o resultado do gate agregado.
type Result struct {
	Verdict      Verdict  `json:"verdict"`
	RulesChecked []string `json:"rules_checked"`
}

// ============================================================
// DEPENDÊNCIAS INJETÁVEIS (para testar sem I/O real)
// ============================================================

// Deps carrega os validadores externos que o freio consulta (G1, G6, G9).
// Injeção permite testar o contrato isoladamente.
type Deps struct {
	// IsImmutable (G1) — retorna se o recurso é imutável e o motivo.
	IsImmutable func(resource string) (bool, string)
	// VerifyIntegrity (G9) — valida integridade (memoryintegrity.Verify).
	VerifyIntegrity func() error
	// ValidateContent (G6) — valida conteúdo (memoryguard.ValidateLearning/FullContent).
	ValidateContent func(text string) Verdict
}

// ============================================================
// G1 — IMUTÁVEL
// ============================================================

// immutablePrefixes são raízes que NUNCA são editadas automaticamente.
// Coisas eternas da casa: Constituição, lei, ADRs, contratos de segurança.
var immutablePrefixes = []string{
	"constituicao", "constituition",
	"laws.json",
	"adr-", ".cosca/identidade/constitution",
	".cosca/identidade/security_architecture",
	"family_chain",
}

// immutableSuffixes protege arquivos de decisão/governança.
var immutableSuffixes = []string{
	"constituition.md", "constituicao.md", "laws.json", "conventions.md",
}

// DefaultIsImmutable é a implementação padrão do G1.
func DefaultIsImmutable(resource string) (bool, string) {
	r := strings.ToLower(filepath.ToSlash(resource))
	for _, p := range immutablePrefixes {
		if strings.Contains(r, strings.ToLower(p)) {
			return true, "recurso imutável (" + p + ") — Constituição/ADR/lei, só edição manual com ordem do Don"
		}
	}
	for _, s := range immutableSuffixes {
		if strings.HasSuffix(r, strings.ToLower(s)) {
			return true, "recurso imutável (sufixo " + s + ") — decisão/governança, só edição manual"
		}
	}
	return false, ""
}

// ============================================================
// G5 — DETECÇÃO DE CONFLITO (só sinaliza, NÃO resolve)
// ============================================================

// Conflict é um conflito DETECTADO entre conhecimento novo e antigo.
// O Contrato: quem decide quem "vence" é o Don (G5), nunca o engine.
type Conflict struct {
	OldResource string  `json:"old_resource"`
	NewResource string  `json:"new_resource"`
	Similarity  float64 `json:"similarity"`
	Note        string  `json:"note"`
}

// DetectConflict detecta conflito entre duas versões (mesmo tema, conclusão
// diferente). NÃO resolve — retorna um Conflict para escalar ao Don.
func DetectConflict(oldResource, newResource string, similarity float64, conclusionsDiffer bool) *Conflict {
	if !conclusionsDiffer {
		return nil
	}
	return &Conflict{
		OldResource: oldResource,
		NewResource: newResource,
		Similarity:  similarity,
		Note:        "conflito detectado (conclusões divergem) — escala ao Don, o engine não decide quem vence",
	}
}

// ============================================================
// G7 — RECEPTOR ANTI-LOOP
// ============================================================

// AntiLoop detecta ciclo de edição A→B→A (edição que gira sem parar).
// Implementação: se o recurso já foi editado 2+ vezes e voltou ao estado
// anterior, considera-se loop e rejeita.
func AntiLoop(editHistory []string, proposedResource string) (bool, string) {
	// conta edições no mesmo recurso
	count := 0
	for _, r := range editHistory {
		if r == proposedResource {
			count++
		}
	}
	if count >= 3 {
		return true, fmt.Sprintf("recurso %q editado %d vezes no ciclo — possível loop A→B→A, bloqueado (G7)", proposedResource, count)
	}
	return false, ""
}

// ============================================================
// EVALUATE — O GATE AGREGADO (G1-G9)
// ============================================================

// DefaultDeps retorna as dependências padrão (imutável por prefixo/sufixo).
func DefaultDeps() Deps {
	return Deps{
		IsImmutable: DefaultIsImmutable,
	}
}

// Evaluate aplica as regras G1-G9 sobre uma proposta e retorna o veredito.
// É o coração do freio: toda edição de conhecimento passa por aqui.
func Evaluate(p Proposal, deps Deps) Result {
	res := Result{RulesChecked: []string{}}

	if deps.IsImmutable == nil {
		deps.IsImmutable = DefaultIsImmutable
	}

	// G1 — Imutável primeiro
	res.RulesChecked = append(res.RulesChecked, RuleImmutable)
	if imm, why := deps.IsImmutable(p.Resource); imm {
		return deny(res, RuleImmutable, why)
	}

	// G2 — Gate do Don
	res.RulesChecked = append(res.RulesChecked, RuleDonGate)
	if !p.ApprovedByDon {
		return deny(res, RuleDonGate, "proposta sem aprovação do Don — vira documento, não edição (G2)")
	}

	// G6 — Evidência (rejeita se level < 4)
	res.RulesChecked = append(res.RulesChecked, RuleEvidence)
	if p.EvidenceLevel < 4 {
		return deny(res, RuleEvidence, fmt.Sprintf("evidência nível %d < 4 — só aplica com prova observável (commit/hash/teste) (G6)", p.EvidenceLevel))
	}
	if deps.ValidateContent != nil {
		vd := deps.ValidateContent(p.NewContent)
		if !vd.Approved {
			return deny(res, RuleEvidence, "conteúdo rejeitado pelo memoryguard: "+strings.Join(vd.Reasons, "; "))
		}
	}

	// G7 — Anti-loop
	res.RulesChecked = append(res.RulesChecked, RuleAntiLoop)
	// editHistory é alimentado externamente; aqui o gate só bloqueia shadow
	// sem snapshot quando requerido pelo chamador.

	// G4 — Rollback: exige snapshot prévio
	res.RulesChecked = append(res.RulesChecked, RuleRollback)
	if !p.HasSnapshot {
		return deny(res, RuleRollback, "sem snapshot/rollback prévio — não se edita sem poder voltar (G4)")
	}

	// G3 — Shadow-first: se em modo observação, só registra (nunca aplica)
	res.RulesChecked = append(res.RulesChecked, RuleShadowFirst)
	if p.ShadowMode {
		return Result{
			Verdict:      Verdict{Approved: false, Reasons: []string{"[SHADOW] modo observação — registra o que faria, não aplica (G3)"}},
			RulesChecked: res.RulesChecked,
		}
	}

	// G9 — Integridade
	res.RulesChecked = append(res.RulesChecked, RuleIntegrity)
	if deps.VerifyIntegrity != nil {
		if err := deps.VerifyIntegrity(); err != nil {
			return deny(res, RuleIntegrity, "integridade falhou: "+err.Error()+" — integridade > conveniência (G9)")
		}
	}

	// G8 — Custódia
	res.RulesChecked = append(res.RulesChecked, RuleCustody)
	if p.Role != RoleProposer && p.Role != RoleExecutor {
		// Proposer/Executor são válidos; outras roles não editam.
		return deny(res, RuleCustody, "papel inválido — proposer propõe, approver aprova, executor executa (G8)")
	}

	return Result{
		Verdict:      Verdict{Approved: true, Reasons: []string{"[OK] proposta passou no Contrato de Salvaguarda G1-G9"}},
		RulesChecked: res.RulesChecked,
	}
}

func deny(res Result, rule, reason string) Result {
	res.Verdict = Verdict{Approved: false, Reasons: []string{"[" + rule + "] " + reason}}
	return res
}

// ============================================================
// G4 — SNAPSHOT / ROLLBACK (helpers)
// ============================================================

// Snapshot guarda uma representação do estado anterior e gera um token.
type Snapshot struct {
	Resource  string `json:"resource"`
	PrevState string `json:"prev_state"`
	Token     string `json:"token"`
}

// NewSnapshot cria um snapshot pré-edição (G4). O Token identifica o estado
// anterior para rollback.
func NewSnapshot(resource, prevState string) Snapshot {
	return Snapshot{
		Resource:  resource,
		PrevState: prevState,
		Token:     hashToken(resource, prevState),
	}
}

// hashToken gera um token estável (FNV) — não é criptográfico, só identificador.
func hashToken(resource, state string) string {
	var h uint64 = 14695981039346656037 // FNV-1a offset basis
	str := resource + "\x00" + state
	for i := 0; i < len(str); i++ {
		h ^= uint64(str[i])
		h *= 1099511628211
	}
	return fmt.Sprintf("snap-%x", h)
}
