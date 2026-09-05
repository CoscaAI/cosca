package brainweb

import (
	"errors"
	"time"

	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/trace"
)

// Observatory é o contrato read-only do "Observatório Cognitivo". Agrega
// dados REAIS do Cosca: constelações de conhecimento com epistemologia e a
// cadeia causal dos traces. NENHUM dado é inventado — tudo vem de fontes
// expostas pelo sistema.
type Observatory struct {
	GeneratedAt   time.Time          `json:"generated_at"`
	Cognitive     CognitiveSnapshot  `json:"cognitive"`
	Constellations []Constellation   `json:"constellations"`
	TraceReplay   *TraceReplay       `json:"trace_replay,omitempty"`
}

// CognitiveSnapshot é o resumo do estado cognitivo (stats reais do runtime).
type CognitiveSnapshot struct {
	KnowledgeItems  int            `json:"knowledge_items"`
	Epistemology    map[string]int `json:"epistemology"` // status -> count
	MemoryEntries   int            `json:"memory_entries,omitempty"`
	Agents          int            `json:"agents,omitempty"`
	Skills          int            `json:"skills,omitempty"`
	UptimeSeconds   int64          `json:"uptime_seconds,omitempty"`
}

// Constellation é uma região de conhecimento agrupada por estado epistemológico.
// É a "física semântica visual": itens FACT/EVIDENCE/INFERENCE têm presença,
// densidade e solidez diferentes.
type Constellation struct {
	Epistemic string  `json:"epistemic"` // KNOWN|SUPPORTED|UNCERTAIN|CONFLICTING|UNKNOWN|STALE
	Label     string  `json:"label"`
	Count     int     `json:"count"`
	Items     []KItem `json:"items"`
	Confidence float64 `json:"confidence"`
}

// KItem é um item de conhecimento na visão sanitizada (read-only).
type KItem struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	Level              string    `json:"level,omitempty"`
	Confidence         float64   `json:"confidence"`
	Status             string    `json:"status"`
	EvidenceCount      int       `json:"evidence_count,omitempty"`
	VerificationCount  int       `json:"verification_count,omitempty"`
	ContradictionCount int       `json:"contradiction_count,omitempty"`
	LastVerified       time.Time `json:"last_verified,omitempty"`
}

// TraceReplay é a cadeia causal reproduzível de uma execução real.
type TraceReplay struct {
	TraceID string      `json:"trace_id"`
	Nodes   []RNode     `json:"nodes"`
	Edges   []REdge     `json:"edges"`
}

// RNode é um passo da cadeia causal (replay).
type RNode struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Actor  string `json:"actor"`
	Result string `json:"result,omitempty"`
}

// REdge é uma aresta causal (causa → consequência).
type REdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

// ObservatoryBuilder monta o observatório a partir das fontes reais.
// Nil-safe: qualquer fonte ausente degrada para vazio, nunca pânico.
type ObservatoryBuilder struct {
	knowledgeItemsFn func() []knowledge.KnowledgeItem
	tracesFn         func() ([]trace.CausalNode, []trace.CausalEdge, string)
	statsFn          func() CognitiveSnapshot
}

// NewObservatoryBuilder cria um builder com as fontes reais. Qualquer fonte
// pode ser nil (o observatório reporta o que está disponível, honestamente).
func NewObservatoryBuilder(
	knowledgeItemsFn func() []knowledge.KnowledgeItem,
	tracesFn func() ([]trace.CausalNode, []trace.CausalEdge, string),
	statsFn func() CognitiveSnapshot,
) *ObservatoryBuilder {
	return &ObservatoryBuilder{
		knowledgeItemsFn: knowledgeItemsFn,
		tracesFn:         tracesFn,
		statsFn:          statsFn,
	}
}

// Build monta o observatório.
func (b *ObservatoryBuilder) Build() Observatory {
	obs := Observatory{GeneratedAt: time.Now().UTC()}
	if b.statsFn != nil {
		obs.Cognitive = b.statsFn()
	}
	if obs.Cognitive.Epistemology == nil {
		obs.Cognitive.Epistemology = map[string]int{}
	}

	// Constelações por epistemologia.
	groups := map[string][]KItem{}
	if b.knowledgeItemsFn != nil {
		for _, it := range b.knowledgeItemsFn() {
			st := string(it.Status)
			if st == "" {
				st = "UNKNOWN"
			}
			groups[st] = append(groups[st], KItem{
				ID:                 it.ID,
				Title:              it.Title,
				Level:              string(it.Level),
				Confidence:         it.Confidence,
				Status:             st,
				EvidenceCount:      len(it.Evidence),
				VerificationCount:  it.VerificationCount,
				ContradictionCount: it.ContradictionCount,
				LastVerified:       it.LastVerified,
			})
		}
	}

	obs.Constellations = buildConstellations(groups)
	obs.Cognitive.KnowledgeItems = countItems(obs.Constellations)

	// Trace replay (cadeia causal real da execução mais recente).
	if b.tracesFn != nil {
		nodes, edges, traceID := b.tracesFn()
		if len(nodes) > 0 && traceID != "" {
			obs.TraceReplay = &TraceReplay{TraceID: traceID}
			for _, n := range nodes {
				obs.TraceReplay.Nodes = append(obs.TraceReplay.Nodes, RNode{
					ID: n.ID, Action: n.Action, Actor: n.Actor, Result: n.Result,
				})
			}
			for _, e := range edges {
				obs.TraceReplay.Edges = append(obs.TraceReplay.Edges, REdge{
					From: e.From, To: e.To, Type: e.Type,
				})
			}
		}
	}

	return obs
}

// epistemicOrder dá a ordem estável de leitura das constelações.
var epistemicOrder = []string{
	"KNOWN", "SUPPORTED", "UNCERTAIN", "CONFLICTING", "UNKNOWN", "STALE",
}

var epistemicLabels = map[string]string{
	"KNOWN": "Fato (FACT)", "SUPPORTED": "Evidência (EVIDENCE)",
	"UNCERTAIN": "Inferência (INFERENCE)", "CONFLICTING": "Conflito",
	"UNKNOWN": "Desconhecido", "STALE": "Desatualizado",
}

func buildConstellations(groups map[string][]KItem) []Constellation {
	out := make([]Constellation, 0, len(epistemicOrder))
	for _, st := range epistemicOrder {
		items := groups[st]
		if len(items) == 0 {
			continue
		}
		var conf float64
		for _, it := range items {
			conf += it.Confidence
		}
		conf /= float64(len(items))
		out = append(out, Constellation{
			Epistemic: st,
			Label:     epistemicLabels[st],
			Count:     len(items),
			Items:     items,
			Confidence: conf,
		})
	}
	return out
}

func countItems(cones []Constellation) int {
	n := 0
	for _, c := range cones {
		n += c.Count
	}
	return n
}

// ErrNoTrace indica que não há trace recente para replay.
var ErrNoTrace = errors.New("no recent trace available")
