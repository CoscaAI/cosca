// Package proposal implements the Kernel Isolado — the independent
// validation gate between the main Kernel (which receives untrusted
// external output) and execution.
//
// Fluxo (desenhado pelo Don):
//
//	IA externa → UNTRUSTED OUTPUT → Evidence/Provenance → Kernel principal
//	→ PROPOSAL → Kernel isolado → Independent validation
//	→ APPROVE / DENY / REVIEW → Execution
//
// Princípios:
//   - Tratar tudo que vem de fora como não confiável até provar inocência
//   - Sem veredicto, sem execução (fail-closed absoluto)
//   - O Kernel isolado julga contra a lei imutável, nunca por opinião
//   - Ações destrutivas/estratégicas exigem o Don (guarda de papel)
//   - Toda etapa registrada com proveniência (forense)
package proposal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ─── Classe de risco ─────────────────────────────────────────────────────────

// RiskClass classifica o impacto potencial de uma proposta.
type RiskClass string

const (
	// RiskTrivial são ações sem efeito duradouro (responder, consultar).
	RiskTrivial RiskClass = "trivial"
	// RiskNormal são ações com efeito local e reversível.
	RiskNormal RiskClass = "normal"
	// RiskDestructive são ações que destroem, apagam ou revertem sem volta.
	// Exigem o Don.
	RiskDestructive RiskClass = "destructive"
	// RiskStrategic são ações que mudam contrato, arquitetura ou gastam
	// recursos. Exigem o Don.
	RiskStrategic RiskClass = "strategic"
)

// ParseRiskClass converte uma string em RiskClass válida.
func ParseRiskClass(s string) (RiskClass, error) {
	switch RiskClass(strings.ToLower(strings.TrimSpace(s))) {
	case RiskTrivial:
		return RiskTrivial, nil
	case RiskNormal:
		return RiskNormal, nil
	case RiskDestructive:
		return RiskDestructive, nil
	case RiskStrategic:
		return RiskStrategic, nil
	}
	return "", fmt.Errorf("classe de risco inválida %q (use: trivial, normal, destructive, strategic)", s)
}

// RequiresDon informa se a classe de risco exige a aprovação do Don.
func (c RiskClass) RequiresDon() bool {
	return c == RiskDestructive || c == RiskStrategic
}

// String devolve a representação textual da classe.
func (c RiskClass) String() string { return string(c) }

// ─── Veredicto ───────────────────────────────────────────────────────────────

// Verdict é o resultado da validação independente do Kernel isolado.
type Verdict string

const (
	// VerdictApprove: a proposta está de acordo com a lei e pode seguir.
	VerdictApprove Verdict = "APPROVE"
	// VerdictDeny: a proposta viola a lei imutável. Não se discute.
	VerdictDeny Verdict = "DENY"
	// VerdictReview: a proposta é consertável — volta com o motivo anexado.
	VerdictReview Verdict = "REVIEW"
)

// ParseVerdict converte uma string em Verdict válida.
func ParseVerdict(s string) (Verdict, error) {
	switch Verdict(strings.ToUpper(strings.TrimSpace(s))) {
	case VerdictApprove:
		return VerdictApprove, nil
	case VerdictDeny:
		return VerdictDeny, nil
	case VerdictReview:
		return VerdictReview, nil
	}
	return "", fmt.Errorf("veredicto inválido %q (use: APPROVE, DENY, REVIEW)", s)
}

// ─── Proveniência / Evidência ────────────────────────────────────────────────

// Provenance registra a origem de uma proposta — a evidência bruta
// (UNTRUSTED OUTPUT) que chegou da IA externa e de onde veio.
type Provenance struct {
	// Source identifica a origem do output (ex: "ia-externa:big-pickle",
	// "arquivo:docs/ADR-007.md", "comando:cosca plan").
	Source string `json:"source"`
	// ReceivedAt é o momento em que a evidência chegou ao Kernel principal.
	ReceivedAt time.Time `json:"received_at"`
	// Evidence é o output bruto, não confiável, exatamente como chegou.
	Evidence string `json:"evidence"`
	// EvidenceHash é o sha256 da evidência bruta — garante integridade e
	// permite ao Kernel isolado conferir se nada foi alterado no caminho.
	EvidenceHash string `json:"evidence_hash"`
}

// Valid informa se a proveniência cumpre o mínimo: origem, evidência e hash.
func (p Provenance) Valid() bool {
	return strings.TrimSpace(p.Source) != "" &&
		strings.TrimSpace(p.Evidence) != "" &&
		strings.TrimSpace(p.EvidenceHash) != ""
}

// HashEvidence calcula o sha256 de uma evidência bruta.
func HashEvidence(evidence string) string {
	sum := sha256.Sum256([]byte(evidence))
	return hex.EncodeToString(sum[:])
}

// ProposalHash é a âncora que liga autorização ↔ artefato exato: serializa a
// proposta inteira e calcula o sha256. Duas propostas que diferem em um único
// byte produzem hashes diferentes — a base do binding anti-mutação (F11).
func ProposalHash(p *Proposal) string {
	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return HashEvidence(string(b))
}

// cloneProposal faz cópia profunda da proposta. O Fluxo sela uma CÓPIA no
// momento da aprovação: mutações externas no ponteiro original jamais
// alcançam a proposta selada (binding de integridade por construção).
func cloneProposal(p *Proposal) *Proposal {
	if p == nil {
		return nil
	}
	b, err := json.Marshal(p)
	if err != nil {
		return &Proposal{ID: p.ID}
	}
	var out Proposal
	if err := json.Unmarshal(b, &out); err != nil {
		return &Proposal{ID: p.ID}
	}
	return &out
}

// ─── Proposta (contrato mínimo) ──────────────────────────────────────────────

// Proposal é o contrato mínimo que todo pensamento do Kernel principal deve
// cumprir antes de ser julgado pelo Kernel isolado. Proposta incompleta não
// é julgada — volta em REVIEW com o motivo anexado.
type Proposal struct {
	// ID identifica a proposta de forma única (gerado ou fornecido).
	ID string `json:"id"`
	// Action é o que exatamente será feito (comando/alvo/escopo).
	Action string `json:"action"`
	// Target é o alvo da ação (arquivo, diretório, serviço, recurso).
	Target string `json:"target"`
	// Motive é o porquê — qual objetivo da família esta ação serve.
	Motive string `json:"motive"`
	// Origin é de onde veio a intenção (quem/o quê pediu).
	Origin string `json:"origin"`
	// State é o contexto atual relevante (o que existe agora, o que muda).
	State string `json:"state"`
	// Risk é a classe de risco (trivial, normal, destructive, strategic).
	Risk RiskClass `json:"risk"`
	// Evidence é a evidência bruta + origem (UNTRUSTED OUTPUT original).
	Evidence Provenance `json:"evidence"`
	// Revisions conta quantas vezes a proposta passou por REVIEW e voltou
	// corrigida. O fluxo permite no máximo MaxRevisions (padrão: 2).
	Revisions int `json:"revisions"`
	// SubmittedAt é o momento da submissão ao Kernel isolado.
	SubmittedAt time.Time `json:"submitted_at"`
}

// MissingFields devolve a lista de campos obrigatórios do contrato mínimo
// que estão vazios. Lista vazia = contrato completo.
func (p *Proposal) MissingFields() []string {
	var missing []string
	if strings.TrimSpace(p.Action) == "" {
		missing = append(missing, "action")
	}
	if strings.TrimSpace(p.Motive) == "" {
		missing = append(missing, "motive")
	}
	if strings.TrimSpace(p.Origin) == "" {
		missing = append(missing, "origin")
	}
	if p.Risk == "" {
		missing = append(missing, "risk")
	}
	return missing
}

// RequiresDon informa se a proposta exige o Don no loop (classe de risco
// destrutiva ou estratégica).
func (p *Proposal) RequiresDon() bool { return p.Risk.RequiresDon() }

// String devolve uma representação compacta da proposta para exibição.
func (p *Proposal) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "proposta %s [%s]", p.ID, p.Risk)
	if p.Target != "" {
		fmt.Fprintf(&b, " → %s", p.Target)
	}
	if p.Action != "" {
		fmt.Fprintf(&b, " (%s)", p.Action)
	}
	if p.RequiresDon() {
		b.WriteString(" ⚠ exige Don")
	}
	return b.String()
}
