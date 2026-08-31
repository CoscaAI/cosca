package brainweb

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/cost"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/CoscaAI/cosca/internal/trace"
)

// Perception é o resultado read-only da percepção determinística frame-a-frame
// (visão sem VLM externa). Modelado a partir de vision.PipelineResult.
type Perception struct {
	GeneratedAt time.Time `json:"generated_at"`
	Source      string    `json:"source,omitempty"`
	Events      []PEvent  `json:"events"`
	Frames      int       `json:"frames"`
	Duration    float64   `json:"duration_seconds,omitempty"`
}

// PEvent é um evento derivado da percepção (frame-a-frame) com a primitiva
// epistêmica — o que o Cosca "viu" e em que nível de certeza.
type PEvent struct {
	Kind        string `json:"kind"` // currency_changed | unknown_interaction | none
	Frame       int    `json:"frame"`
	NeedsVLM    bool   `json:"needs_vlm"`
	Explanation string `json:"explanation"`
	Epistemic   string `json:"epistemic"`    // OBSERVED | UNKNOWN
	Level       string `json:"evidence_level"` // NONE | LOW | MEDIUM | HIGH
}

// PerceptionSource fornece a percepção determinística real do Cosca.
type PerceptionSource interface {
	Perceive() *Perception
}

// Handler expõe os data contracts read-only do "cérebro" Cosca: Graph
// (organograma), Observatory (observatório cognitivo), Activity (ações
// recentes) e Perception (sensor determinístico). A UI do visualizador 3D
// foi MOVIDA para o dashboard "Casa Visível" (app separada) — este pacote
// não serve mais HTML estático. Todo payload é a projeção mínima sanitizada.
type Handler struct {
	builder      *Builder
	activity     ActivitySource
	observatory  *ObservatoryBuilder
	perception   PerceptionSource
}

// ActivitySource fornece as ações recentes do cérebro. Implementado no
// server.go com o singleton de executions + o trace store — mantém o brainweb
// desacoplado do orchestration (testável e nil-safe).
type ActivitySource interface {
	Recent(limit int) []Activity
}

// NewHandler cria um Handler (data contracts do cérebro) com um Builder de
// grafo. managers nil-safe — o grafo renderiza vazio em vez de pânico.
// A UI do visualizador foi movida para o dashboard "Casa Visível" (app
// separada); este pacote agora expõe apenas os contratos read-only.
func NewHandler(agentsMgr *agents.Manager, skillsMgr *skills.Manager, version string) *Handler {
	return &Handler{
		builder: NewBuilder(agentsMgr, skillsMgr, version),
	}
}

// WithActivity injeta a fonte de ações reais (executions + traces).
func (h *Handler) WithActivity(src ActivitySource) *Handler {
	h.activity = src
	return h
}

// WithPerception injeta a fonte da percepção determinística (visão frame-a-frame).
func (h *Handler) WithPerception(src PerceptionSource) *Handler {
	h.perception = src
	return h
}

// WithCost injeta a fonte de telemetria de custo (cost.Store) que preenche a
// projeção de Token Efficiency/Energy por agente no grafo. Nil-safe: sem store
// → projeção neutra, nunca pânico.
func (h *Handler) WithCost(store *cost.Store) *Handler {
	h.builder.WithCost(store)
	return h
}

// WithObservatory injeta as fontes reais do observatório (epistemologia +
// traces causais + stats). Nil-safe.
func (h *Handler) WithObservatory(knowledgeItemsFn func() []knowledge.KnowledgeItem,
	tracesFn func() ([]trace.CausalNode, []trace.CausalEdge, string),
	statsFn func() CognitiveSnapshot) *Handler {
	h.observatory = NewObservatoryBuilder(knowledgeItemsFn, tracesFn, statsFn)
	return h
}

// Graph devolve o grafo sanitizado em JSON.
func (h *Handler) Graph(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.builder.Build())
}

// Observatory devolve o observatório cognitivo (read-only).
func (h *Handler) Observatory(w http.ResponseWriter, _ *http.Request) {
	if h.observatory == nil {
		writeJSON(w, http.StatusOK, NewObservatoryBuilder(nil, nil, nil).Build())
		return
	}
	writeJSON(w, http.StatusOK, h.observatory.Build())
}

// Activity devolve as ações recentes do cérebro (read-only).
func (h *Handler) Activity(w http.ResponseWriter, _ *http.Request) {
	limit := 30
	if h.activity != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"activities": h.activity.Recent(limit),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"activities": []Activity{}})
}

// Perception devolve a percepção determinística frame-a-frame (read-only) —
// o "sensor" do cérebro, sem VLM externa. Nil-safe: sem fonte → vazio.
func (h *Handler) Perception(w http.ResponseWriter, _ *http.Request) {
	if h.perception != nil {
		p := h.perception.Perceive()
		if p != nil {
			writeJSON(w, http.StatusOK, p)
			return
		}
	}
	writeJSON(w, http.StatusOK, &Perception{GeneratedAt: time.Now().UTC(), Events: []PEvent{}})
}

// writeJSON serializa JSON com os headers corretos.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
