// Package intelligence implementa o Intelligence Engine (ADR-047).
//
// O engine Ã© a parte DETERMINÃSTICA e COMPUTÃVEL da inteligÃªncia da casa:
// ele decide o que estudar (curriculum), detecta conflito de conhecimento (R6)
// e propÃµe substituiÃ§Ã£o/promoÃ§Ã£o â€” SEMPRE passando pelo freio (guardrails).
// Ele NUNCA edita sozinho: sabe, propÃµe, passa pelo Contrato de Salvaguarda,
// e sÃ³ o que for aprovado Ã© aplicado. Em shadow-first, nem aplica.
//
// PrincÃ­pio do Don: validar com CPU/cache (execuÃ§Ã£o determinÃ­stica), nÃ£o
// depender de LLM "achismo" para decidir o que a casa sabe.
package intelligence

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/guardrails"
)

// ============================================================
// FONTES DE CONHECIMENTO
// ============================================================

// Source Ã© uma unidade de conhecimento considerada pelo engine.
type Source struct {
	ID           string    `json:"id"`
	Content      string    `json:"content"`
	Topic        string    `json:"topic"`
	Evidence     int       `json:"evidence"`   // 0-5 (regra: >=4 Ã© forte)
	Confidence   float64   `json:"confidence"` // 0-1
	Recency      time.Time `json:"recency"`
	UpdatedAt    time.Time `json:"updated_at"`
	ParentCommit string    `json:"parent_commit"` // hash do commit git que gerou o aprendizado (proveniÃªncia de versÃ£o)
	ParentDate   string    `json:"parent_date"`   // data do aprendizado no Ã­ndice (id pai temporal; granular â€” cada aprendizado tem a sua)
}

// SourceProvider fornece as fontes do conhecimento semÃ¢ntico da casa.
type SourceProvider func(ctx context.Context) ([]Source, error)

// ============================================================
// PLANNING (CURRICULUM) â€” o que estudar e em que ordem
// ============================================================

// StudyItem Ã© um item do plano de estudo (curriculum).
type StudyItem struct {
	SourceID string  `json:"source_id"`
	Priority float64 `json:"priority"`
	Reason   string  `json:"reason"`
}

// Plan Ã© o curriculum â€” a ordem decidida deterministicamente.
type Plan struct {
	Items []StudyItem `json:"items"`
}

// ============================================================
// ENGINE
// ============================================================

// Engine Ã© o Intelligence Engine determinÃ­stico.
type Engine struct {
	guard   guardrails.Deps // o freio (injetÃ¡vel)
	sources SourceProvider  // fonte do conhecimento
	// embedder (opcional): se setado, a similaridade Ã© por EMBEDDING semÃ¢ntico
	// (cosine no vetor) em vez de Jaccard de texto. Detecta conflito por
	// SIGNIFICADO (sinÃ´nimos/parÃ¡frases), mais preciso que palavras. Quando
	// nil, usa Jaccard (determinÃ­stico, sem dependÃªncia externa).
	embedder func(text string) ([]float64, error)
}

// New cria um Engine com o freio e a fonte de conhecimento.
func New(guard guardrails.Deps, sources SourceProvider) *Engine {
	if guard.IsImmutable == nil {
		guard.IsImmutable = guardrails.DefaultIsImmutable
	}
	return &Engine{guard: guard, sources: sources}
}

// SetEmbedder injeta um provedor de embedding (vetor de um texto). Se setado,
// o DetectConflicts/DetectDuplicates usam similaridade semÃ¢ntica por cosine.
// Ex.: EmbeddingProvider.GenerateEmbedding da casa (Ollama/nomic).
func (e *Engine) SetEmbedder(embed func(text string) ([]float64, error)) *Engine {
	e.embedder = embed
	return e
}

// similarity devolve a similaridade entre dois textos:
//   - se embedder setado: cosine no embedding (semÃ¢ntico);
//   - senÃ£o: Jaccard de token (texto) â€” determinÃ­stico, sem dependÃªncia.
func (e *Engine) similarity(a, b string) float64 {
	if e.embedder != nil {
		va, err := e.embedder(a)
		if err == nil {
			vb, err := e.embedder(b)
			if err == nil {
				if sim, err := embeddings.CosineSimilarity(va, vb); err == nil {
					return sim
				}
			}
		}
		// fallback: se o embedding falhar, usa Jaccard
	}
	return similarityApprox(a, b)
}

// ============================================================
// CURRICULUM â€” decide o que estudar (determinÃ­stico)
// ============================================================

// weightPrioridade Ã© a prioridade de estudo de uma fonte.
// DeterminÃ­stico: evidÃªncia (pesada) + confianÃ§a + recÃªncia.
// NÃ£o usa LLM â€” Ã© o "validar com CPU/cache" do Don.
func weightPriority(s Source) float64 {
	// evidÃªncia: 0-5 â†’ 0-1
	ev := float64(s.Evidence) / 5.0
	conf := clamp01(s.Confidence)
	rec := recencyScore(s.Recency)

	// pesos: evidÃªncia 0.5, confianÃ§a 0.3, recÃªncia 0.2
	p := 0.5*ev + 0.3*conf + 0.2*rec
	// evidÃªncia Ã© dominante: conhecimento comprovado por cÃ³digo/teste vale mais
	if s.Evidence >= 4 {
		p *= 1.2 // bÃ´nus de "prova observÃ¡vel" (G6)
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
			Reason:   fmt.Sprintf("evidÃªncia=%d confianÃ§a=%.2f recÃªncia=%.2f", s.Evidence, s.Confidence, recencyScore(s.Recency)),
		})
	}

	// ordena por prioridade descendente (estÃ¡vel)
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
// CONFLITO (R6) â€” detecta, NÃƒO resolve (escala ao Don)
// ============================================================

// DetectConflicts compara fontes novas contra as atuais e sinaliza CONFLITOS
// reais (R6). Diferencia DUPLICATA de CONFLITO:
//   - sim > duplicateSimilarityThreshold (0.85): MESMO aprendizado gravado de
//     novo (duplicata) â€” NÃƒO Ã© conflito, Ã© redundÃ¢ncia para condensar (R2).
//     NÃ£o sinaliza como conflito (o Don nÃ£o resolve "duplicata" como
//     contradiÃ§Ã£o).
//   - similarityThreshold <= sim <= duplicateSimilarityThreshold: conclusÃµes
//     que DIVERGEM de fato (mesmo tema, conteÃºdo diferente) â†’ conflito R6.
//
// SÃ³ detecta e escala â€” quem decide Ã© o Don (guardrails G5).
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
			sim := e.similarity(inc.Content, k.Content)
			// duplicata (mesmo aprendizado): condensar, nÃ£o conflito
			if sim > duplicateSimilarityThreshold {
				continue
			}
			if sim < similarityThreshold {
				continue
			}
			// o novo tem evidÃªncia melhor ou igual â†’ sinaliza conflito (R6)
			if inc.Evidence >= k.Evidence {
				c := *guardrails.DetectConflict(k.ID, inc.ID, sim, true)
				// ProveniÃªncia (id pai): commit (versÃ£o) + data (contexto temporal).
				c.OldCommit = k.ParentCommit
				c.NewCommit = inc.ParentCommit
				c.OldDate = k.ParentDate
				c.NewDate = inc.ParentDate
				switch {
				case k.ParentDate != "" && k.ParentDate == inc.ParentDate:
					c.Note = "conflito na MESMA data (" + k.ParentDate + ") â€” mesmo contexto temporal, conclusÃµes divergem; contradiÃ§Ã£o provÃ¡vel (R6)"
				case k.ParentCommit != "" && k.ParentCommit == inc.ParentCommit:
					c.Note = "conflito no MESMO commit (" + k.ParentCommit + ") â€” mesma versÃ£o, conclusÃµes divergem (R6)"
				default:
					c.Note = "conflito (conclusÃµes divergem) â€” escala ao Don, o engine nÃ£o decide quem vence (R6)"
				}
				conflicts = append(conflicts, c)
			}
		}
	}

	return conflicts
}

// duplicateSimilarityThreshold: acima dele, duas fontes sÃ£o o MESMO aprendizado
// (duplicata), nÃ£o conclusÃµes divergentes. Usado para nÃ£o confundir redundÃ¢ncia
// (condensar/R2) com contradiÃ§Ã£o (conflito/R6).
const duplicateSimilarityThreshold = 0.85

// DetectedDuplicate Ã© um par de fontes que sÃ£o o mesmo aprendizado gravado mais
// de uma vez (similaridade muito alta) â€” candidatos Ã  condensaÃ§Ã£o (R2).
type DetectedDuplicate struct {
	SourceA    string  `json:"source_a"`
	SourceB    string  `json:"source_b"`
	Similarity float64 `json:"similarity"`
}

// DetectDuplicates retorna os pares duplicados (mesmo aprendizado, sim > 0.85)
// para a curadoria R2 (condensar) â€” separado dos CONFLITOS. NÃ£o tem relaÃ§Ã£o com
// a regra de ouro: duplicata Ã© redundÃ¢ncia, nÃ£o divergÃªncia.
func (e *Engine) DetectDuplicates(known, incoming []Source) []DetectedDuplicate {
	var dups []DetectedDuplicate
	for _, inc := range incoming {
		for _, k := range known {
			if inc.ID == k.ID || inc.ID <= k.ID {
				continue // sÃ³ um lado do par (evita A<->B e B<->A)
			}
			// ConteÃºdo idÃªntico OU quase idÃªntico = MESMO aprendizado (duplicata).
			// NÃ£o se pula conteÃºdo igual â€” Ã© o caso mais claro de duplicata.
			sim := e.similarity(inc.Content, k.Content)
			if sim > duplicateSimilarityThreshold {
				dups = append(dups, DetectedDuplicate{SourceA: k.ID, SourceB: inc.ID, Similarity: sim})
			}
		}
	}
	return dups
}

// ============================================================
// SAÚDE COGNITIVA — índice de auto-inspeção (novo e melhor que o spec antigo)
// ============================================================
//
// Inspirado (como REFERÊNCIA) nos specs antigos do backup (Entropia Cognitiva,
// B1-B5) — mas MUITO melhor: é DETERMINÍSTICO e mede os dados REAIS do motor
// (conflitos, duplicatas, obsolescência, proveniência), não uma heurística
// manual. 100 = conhecimento saudável; 0 = caos.

// HealthReport é o índice de saúde cognitiva do conhecimento da casa.
type HealthReport struct {
	Health            int     `json:"health"` // 0-100 (100 = perfeito)
	ConflictScore     float64 `json:"conflict_score"`
	DuplicateScore    float64 `json:"duplicate_score"`
	StalenessScore    float64 `json:"staleness_score"`
	ProvenanceScore   float64 `json:"provenance_score"`
	Conflicts         int     `json:"conflicts"`
	Duplicates        int     `json:"duplicates"`
	Sources           int     `json:"sources"` // aprendizados únicos
	MissingProvenance int     `json:"missing_provenance"`
}

// Health calcula o índice de saúde a partir dos dados REAIS que o motor já
// produz (conflitos R6, duplicatas R2, recency decay, proveniência).
func (e *Engine) Health(srcs []Source, conflicts []guardrails.Conflict, dups []DetectedDuplicate) HealthReport {
	total := len(srcs)
	if total == 0 {
		return HealthReport{Health: 100}
	}

	// Conflitos: cada conflito penaliza (cap em 100)
	cScore := clamp01(float64(len(conflicts)) / float64(total) * 100)

	// Duplicatas: redundância penaliza (heu: pares/sources)
	dScore := clamp01(float64(len(dups)) / float64(total) * 100)

	// Obsolescência: fontes muito antigas (recency)
	stale := 0
	for _, s := range srcs {
		if recencyScore(s.Recency) <= 0.1 { // >90 dias
			stale++
		}
	}
	sScore := clamp01(float64(stale) / float64(total) * 100)

	// Proveniência: fontes sem commit nem data
	noProv := 0
	for _, s := range srcs {
		if s.ParentCommit == "" && s.ParentDate == "" {
			noProv++
		}
	}
	pScore := clamp01(float64(noProv) / float64(total) * 100)

	// Saúde = 100 - média ponderada (conflito pesa mais — corrompe decisão)
	health := int(100 - (0.35*cScore + 0.25*dScore + 0.20*sScore + 0.20*pScore))
	if health < 0 {
		health = 0
	}

	return HealthReport{
		Health:            health,
		ConflictScore:     cScore,
		DuplicateScore:    dScore,
		StalenessScore:    sScore,
		ProvenanceScore:   pScore,
		Conflicts:         len(conflicts),
		Duplicates:        len(dups),
		Sources:           total,
		MissingProvenance: noProv,
	}
}

// PromoteProposal constrÃ³i uma proposta GATEADA de promoÃ§Ã£o.
// Passa pelo freio (guardrails) â€” sem aprovaÃ§Ã£o do Don, Ã© sÃ³ proposta.
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

// Proposal Ã© o DTO de promoÃ§Ã£o/substituiÃ§Ã£o que o engine monta.
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
// HELPERS determinÃ­sticos
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

// similarityApprox: aproximaÃ§Ã£o determinÃ­stica de similaridade de texto.
// Usa sobreposiÃ§Ã£o de palavras (Jaccard simplificado) â€” sem LLM.
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
