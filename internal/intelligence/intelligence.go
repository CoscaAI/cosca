// Package intelligence implementa o Intelligence Engine (ADR-047).
//
// O engine é a parte DETERMINÍSTICA e COMPUTÁVEL da inteligência da casa:
// ele decide o que estudar (curriculum), detecta conflito de conhecimento (R6)
// e propõe substituição/promoção — SEMPRE passando pelo freio (guardrails).
// Ele NUNCA edita sozinho: sabe, propõe, passa pelo Contrato de Salvaguarda,
// e só o que for aprovado é aplicado. Em shadow-first, nem aplica.
//
// Princípio do Don: validar com CPU/cache (execução determinística), não
// depender de LLM "achismo" para decidir o que a casa sabe.
package intelligence

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/CoscaAI/cosca/internal/guardrails"
)

// ============================================================
// FONTES DE CONHECIMENTO
// ============================================================

// Source é uma unidade de conhecimento considerada pelo engine.
type Source struct {
	ID           string    `json:"id"`
	Content      string    `json:"content"`
	Topic        string    `json:"topic"`
	Evidence     int       `json:"evidence"`   // 0-5 (regra: >=4 é forte)
	Confidence   float64   `json:"confidence"` // 0-1
	Recency      time.Time `json:"recency"`
	UpdatedAt    time.Time `json:"updated_at"`
	ParentCommit string    `json:"parent_commit"` // hash do commit git que gerou o aprendizado (proveniência de versão)
	ParentDate   string    `json:"parent_date"`   // data do aprendizado no índice (id pai temporal; granular — cada aprendizado tem a sua)
}

// SourceProvider fornece as fontes do conhecimento semântico da casa.
type SourceProvider func(ctx context.Context) ([]Source, error)

// ============================================================
// PLANNING (CURRICULUM) — o que estudar e em que ordem
// ============================================================

// StudyItem é um item do plano de estudo (curriculum).
type StudyItem struct {
	SourceID string  `json:"source_id"`
	Priority float64 `json:"priority"`
	Reason   string  `json:"reason"`
}

// Plan é o curriculum — a ordem decidida deterministicamente.
type Plan struct {
	Items []StudyItem `json:"items"`
}

// ============================================================
// ENGINE
// ============================================================

// Engine é o Intelligence Engine determinístico.
type Engine struct {
	guard   guardrails.Deps // o freio (injetável)
	sources SourceProvider  // fonte do conhecimento
}

// New cria um Engine com o freio e a fonte de conhecimento.
func New(guard guardrails.Deps, sources SourceProvider) *Engine {
	if guard.IsImmutable == nil {
		guard.IsImmutable = guardrails.DefaultIsImmutable
	}
	return &Engine{guard: guard, sources: sources}
}

// ============================================================
// CURRICULUM — decide o que estudar (determinístico)
// ============================================================

// weightPrioridade é a prioridade de estudo de uma fonte.
// Determinístico: evidência (pesada) + confiança + recência.
// Não usa LLM — é o "validar com CPU/cache" do Don.
func weightPriority(s Source) float64 {
	// evidência: 0-5 → 0-1
	ev := float64(s.Evidence) / 5.0
	conf := clamp01(s.Confidence)
	rec := recencyScore(s.Recency)

	// pesos: evidência 0.5, confiança 0.3, recência 0.2
	p := 0.5*ev + 0.3*conf + 0.2*rec
	// evidência é dominante: conhecimento comprovado por código/teste vale mais
	if s.Evidence >= 4 {
		p *= 1.2 // bônus de "prova observável" (G6)
	}
	return clamp01(p)
}

// Sources retorna as fontes de conhecimento (via o provider).
func (e *Engine) Sources(ctx context.Context) ([]Source, error) {
	if e.sources == nil {
		return nil, errors.New("engine sem fonte de conhecimento")
	}
	return e.sources(ctx)
}

// Plan retorna o curriculum ordenado por prioridade descendente.
func (e *Engine) Plan(ctx context.Context) (*Plan, error) {
	if e.sources == nil {
		return nil, errors.New("engine sem fonte de conhecimento")
	}
	srcs, err := e.sources(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]StudyItem, 0, len(srcs))
	for _, s := range srcs {
		prio := weightPriority(s)
		items = append(items, StudyItem{
			SourceID: s.ID,
			Priority: prio,
			Reason:   fmt.Sprintf("evidência=%d confiança=%.2f recência=%.2f", s.Evidence, s.Confidence, recencyScore(s.Recency)),
		})
	}

	// ordena por prioridade descendente (estável)
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Priority > items[i].Priority {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	return &Plan{Items: items}, nil
}

// ============================================================
// CONFLITO (R6) — detecta, NÃO resolve (escala ao Don)
// ============================================================

// DetectConflicts compara fontes novas contra as atuais e sinaliza CONFLITOS
// reais (R6). Diferencia DUPLICATA de CONFLITO:
//   - sim > duplicateSimilarityThreshold (0.85): MESMO aprendizado gravado de
//     novo (duplicata) — NÃO é conflito, é redundância para condensar (R2).
//     Não sinaliza como conflito (o Don não resolve "duplicata" como
//     contradição).
//   - similarityThreshold <= sim <= duplicateSimilarityThreshold: conclusões
//     que DIVERGEM de fato (mesmo tema, conteúdo diferente) → conflito R6.
//
// Só detecta e escala — quem decide é o Don (guardrails G5).
func (e *Engine) DetectConflicts(known, incoming []Source, similarityThreshold float64) []guardrails.Conflict {
	var conflicts []guardrails.Conflict

	for _, inc := range incoming {
		for _, k := range known {
			if inc.ID == k.ID {
				continue
			}
			if inc.Topic != k.Topic {
				continue
			}
			if inc.Content == k.Content {
				continue
			}
			sim := similarityApprox(inc.Content, k.Content)
			// duplicata (mesmo aprendizado): condensar, não conflito
			if sim > duplicateSimilarityThreshold {
				continue
			}
			if sim < similarityThreshold {
				continue
			}
			// o novo tem evidência melhor ou igual → sinaliza conflito (R6)
			if inc.Evidence >= k.Evidence {
				c := *guardrails.DetectConflict(k.ID, inc.ID, sim, true)
				// Proveniência (id pai): commit (versão) + data (contexto temporal).
				c.OldCommit = k.ParentCommit
				c.NewCommit = inc.ParentCommit
				c.OldDate = k.ParentDate
				c.NewDate = inc.ParentDate
				switch {
				case k.ParentDate != "" && k.ParentDate == inc.ParentDate:
					c.Note = "conflito na MESMA data (" + k.ParentDate + ") — mesmo contexto temporal, conclusões divergem; contradição provável (R6)"
				case k.ParentCommit != "" && k.ParentCommit == inc.ParentCommit:
					c.Note = "conflito no MESMO commit (" + k.ParentCommit + ") — mesma versão, conclusões divergem (R6)"
				default:
					c.Note = "conflito (conclusões divergem) — escala ao Don, o engine não decide quem vence (R6)"
				}
				conflicts = append(conflicts, c)
			}
		}
	}

	return conflicts
}

// duplicateSimilarityThreshold: acima dele, duas fontes são o MESMO aprendizado
// (duplicata), não conclusões divergentes. Usado para não confundir redundância
// (condensar/R2) com contradição (conflito/R6).
const duplicateSimilarityThreshold = 0.85

// DetectedDuplicate é um par de fontes que são o mesmo aprendizado gravado mais
// de uma vez (similaridade muito alta) — candidatos à condensação (R2).
type DetectedDuplicate struct {
	SourceA    string  `json:"source_a"`
	SourceB    string  `json:"source_b"`
	Similarity float64 `json:"similarity"`
}

// DetectDuplicates retorna os pares duplicados (mesmo aprendizado, sim > 0.85)
// para a curadoria R2 (condensar) — separado dos CONFLITOS. Não tem relação com
// a regra de ouro: duplicata é redundância, não divergência.
func (e *Engine) DetectDuplicates(known, incoming []Source) []DetectedDuplicate {
	var dups []DetectedDuplicate
	for _, inc := range incoming {
		for _, k := range known {
			if inc.ID == k.ID || inc.ID <= k.ID {
				continue // só um lado do par (evita A<->B e B<->A)
			}
			// Conteúdo idêntico OU quase idêntico = MESMO aprendizado (duplicata).
			// Não se pula conteúdo igual — é o caso mais claro de duplicata.
			sim := similarityApprox(inc.Content, k.Content)
			if sim > duplicateSimilarityThreshold {
				dups = append(dups, DetectedDuplicate{SourceA: k.ID, SourceB: inc.ID, Similarity: sim})
			}
		}
	}
	return dups
}

// ============================================================
// PROMOÇÃO (R4) — propõe elevar a global (GATEADA)
// ============================================================

// PromoteProposal constrói uma proposta GATEADA de promoção.
// Passa pelo freio (guardrails) — sem aprovação do Don, é só proposta.
func (e *Engine) PromoteProposal(p Proposal) (guardrails.Proposal, guardrails.Result) {
	proposal := guardrails.Proposal{
		ID:            p.ID,
		Resource:      p.Resource,
		Role:          guardrails.RoleProposer,
		NewContent:    p.NewContent,
		EvidenceLevel: p.EvidenceLevel,
		EvidenceNote:  p.EvidenceNote,
		ApprovedByDon: p.ApprovedByDon,
		ShadowMode:    p.ShadowMode,
		HasSnapshot:   p.HasSnapshot,
	}
	res := guardrails.Evaluate(proposal, e.guard)
	return proposal, res
}

// Proposal é o DTO de promoção/substituição que o engine monta.
type Proposal struct {
	ID            string
	Resource      string
	NewContent    string
	EvidenceLevel int
	EvidenceNote  string
	ApprovedByDon bool
	ShadowMode    bool
	HasSnapshot   bool
}

// ============================================================
// HELPERS determinísticos
// ============================================================

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// recencyScore: 0-7d=1, 7-30=0.7, 30-90=0.4, >90=0.1
func recencyScore(t time.Time) float64 {
	if t.IsZero() {
		return 0.1
	}
	days := time.Since(t).Hours() / 24
	switch {
	case days < 7:
		return 1.0
	case days < 30:
		return 0.7
	case days < 90:
		return 0.4
	default:
		return 0.1
	}
}

// similarityApprox: aproximação determinística de similaridade de texto.
// Usa sobreposição de palavras (Jaccard simplificado) — sem LLM.
func similarityApprox(a, b string) float64 {
	wa := tokenize(a)
	wb := tokenize(b)
	if len(wa) == 0 || len(wb) == 0 {
		return 0
	}
	set := map[string]struct{}{}
	for _, w := range wa {
		set[w] = struct{}{}
	}
	common := 0
	for _, w := range wb {
		if _, ok := set[w]; ok {
			common++
		}
	}
	return float64(common) / float64(len(wa)+len(wb)-common)
}

func tokenize(s string) []string {
	var out []string
	var cur []rune
	for _, r := range s {
		if isAlphaNum(r) {
			cur = append(cur, r)
		} else if len(cur) > 0 {
			out = append(out, string(cur))
			cur = nil
		}
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

func isAlphaNum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r >= 0x80
}

// avoid unused import warning em builds sem math usage
var _ = math.MaxFloat64
