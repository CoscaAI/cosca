// context_packet.go — O CONTEXTO do "cérebro consultável".
//
// Este é o artefato honesto que o servidor MCP do COSCA devolve: NUNCA um
// passthrough cru de uma função Go, e sim um pacote de contexto que carrega
// a EPISTEMOLOGIA (source), a RELEVÂNCIA e a CONFIANÇA (zero-LLM) do que o
// COSCA sabe — e o que ele não sabe.
//
// Regra de ouro (ADR-017 §1): toda capacidade que a IA demonstra, o Cosca
// cristaliza em mecanismo verificável. Aqui, cristalizada como o contrato:
//
//	{ query, context: [{content, source, relevance}], confidence, trace_id }
//
// O `confidence` é DETERMINÍSTICO (I1): derivado da classe epistêmica
// (internal/knowledge/epistemic_class.go) + relevância do score híbrido.
// Nunca é opinado. `source` é a KnowledgeEpistemic — nunca um rótulo de
// adivinhação.
package mcpserver

import (
	"math"

	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/search"
	"github.com/CoscaAI/cosca/internal/trace"
)

// ContextItem é um item de contexto do packet — uma unidade do que o cérebro
// "sabe" sobre a consulta, com a natureza (source) e a relevância.
type ContextItem struct {
	Content   string  `json:"content"`
	Source    string  `json:"source"`
	Relevance float64 `json:"relevance"`
}

// ContextPacket é o artefato que o OpenCode (e qualquer agente) recebe.
//
// ┌─────────────── o contrato do "cérebro consultável" (ADR-028 §3) ────────┐
type ContextPacket struct {
	Query      string        `json:"query"`
	Context    []ContextItem `json:"context"`
	Confidence float64       `json:"confidence"`
	TraceID    string        `json:"trace_id"`

	// ── Instrumentação do "cérebro instrumentado" (evolução do professor) ──
	// Campos opcionais (omitempty) para retrocompatibilidade: o packet base
	// (query/context/confidence/trace_id) é preservado; estes enriquecem a
	// resposta para o Auto-Audit e para o OpenCode raciocinar sobre o que o
	// COSCA fez (não só o que sabe).
	Capability    string       `json:"capability,omitempty"`
	EpistemicClass string      `json:"epistemic_class,omitempty"` // classe predominante do packet
	Artifacts     []ArtifactRef `json:"artifacts,omitempty"`      // o que foi produzido (ADR-031)
	Decisions     []DecisionRef `json:"decisions,omitempty"`      // o que foi decidido
	Cost          *CostRef     `json:"cost,omitempty"`            // tokens + useful work
}

// ArtifactRef referência um artefato produzido por uma execução (ADR-031).
type ArtifactRef struct {
	Kind string `json:"kind"` // write_file | build | test | ...
	Name string `json:"name"` // caminho/identidade do artefato
}

// DecisionRef referência uma decisão (divergência, inferência, roteamento).
type DecisionRef struct {
	Kind    string `json:"kind"`  // semantic_high | divergence | fallback | ...
	Details string `json:"details,omitempty"`
}

// CostRef resume o custo/valor de uma execução (Token Efficiency / ADR-031).
type CostRef struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TokensTotal  int     `json:"tokens_total"`
	UsefulWork   float64 `json:"useful_work"`
}

// ─── Epistemologia → source ───────────────────────────────────────────────

// sourceFromEpistemic devolve a classe epistêmica VÁLIDA de um item. FAIL-CLOSED
// (I2): um item com classe fora das 7 válidas — ou sem classe — é NORMALIZADO
// para EVIDENCE (a classe honesta de "observação registrada, ainda não
// validada"); um source inválido NUNCA chega ao packet. Isto espelha o
// fail-closed de SearchParams.Epistemic[] já implementado no knowledge.Engine.
func sourceFromEpistemic(ep string) string {
	e := knowledge.KnowledgeEpistemic(ep)
	if e.Valid() {
		return string(e)
	}
	return string(knowledge.EpistemicEVIDENCE)
}

// sourceFromResult extrai a classe epistêmica de um search.SearchResult
// (preenchida por knowledge.Engine.enrichEpistemic → Metadata["epistemic"]).
func sourceFromResult(sr search.SearchResult) string {
	if sr.Metadata != nil {
		if ep, ok := sr.Metadata["epistemic"]; ok && ep != "" {
			return sourceFromEpistemic(ep)
		}
	}
	return string(knowledge.EpistemicEVIDENCE)
}

// ─── Confidence (zero-LLM, determinístico) ────────────────────────────────

// classConfidence devolve a confiança BASE de uma classe epistêmica — a régua
// que impede INFERRED de virar FACT (ADR-028 §3.3.1):
//
//	FACT/MEASURED → alta  |  EVIDENCE/INFERRED → baixa  |  RULE/DECISION/PROFILE
//	→ valor próprio (não é medição-fato)  |  inválida → mínima (honesta).
func classConfidence(e knowledge.KnowledgeEpistemic) float64 {
	switch e {
	case knowledge.EpistemicFACT:
		return 0.90
	case knowledge.EpistemicMEASURED:
		return 0.80
	case knowledge.EpistemicRULE, knowledge.EpistemicDECISION, knowledge.EpistemicPROFILE:
		return 0.70
	case knowledge.EpistemicEVIDENCE:
		return 0.60
	case knowledge.EpistemicINFERRED:
		return 0.50
	default:
		return 0.30
	}
}

// itemConfidence combina a base da classe + a relevância como peso (ADR-028
// §3.3.4: o relevance entra como peso). Determinístico — arredondado a 4 casas.
func itemConfidence(e knowledge.KnowledgeEpistemic, relevance float64) float64 {
	w := 0.5 + 0.5*clamp01(relevance) // relevância 0..1 → peso 0.5..1.0
	c := classConfidence(e) * w
	return math.Round(clamp01(c)*10000) / 10000
}

// packetConfidence agrega os itens em UMA confiança do packet (ADR-028 §3.3.5):
// média ponderada pela relevância. Sem itens → 0 (honesto: "o cérebro não
// sabe"). Recompute por item para ser função pura dos campos do packet.
func packetConfidence(items []ContextItem) float64 {
	if len(items) == 0 {
		return 0
	}
	var sum, weight float64
	for _, it := range items {
		r := clamp01(it.Relevance)
		weight += r
		sum += itemConfidence(knowledge.KnowledgeEpistemic(it.Source), it.Relevance) * r
	}
	if weight == 0 {
		// Nenhum item pesa (relevância 0): média simples das confianças.
		var sumC float64
		for _, it := range items {
			sumC += itemConfidence(knowledge.KnowledgeEpistemic(it.Source), it.Relevance)
		}
		return math.Round(clamp01(sumC/float64(len(items)))*10000) / 10000
	}
	return math.Round(clamp01(sum/weight)*10000) / 10000
}

// NewContextPacket monta o packet final, gerando um Trace ID universal
// (internal/trace) que amarra o "cérebro lembrou disso" a uma operação
// auditável. Gabarito do ADR-028 §3.1.
func NewContextPacket(query string, items []ContextItem) *ContextPacket {
	if items == nil {
		items = []ContextItem{}
	}
	return &ContextPacket{
		Query:      query,
		Context:    items,
		Confidence: packetConfidence(items),
		TraceID:    trace.NewID().String(),
	}
}

// clamp01 limita v ao intervalo [0,1] — nunca deixa a confiança "vazar" fora.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
