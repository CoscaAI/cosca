package brainweb

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
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

// Handler serve o visualizador 3D (/brain) e o contrato de dados (/brain/graph,
// /brain/activity, /brain/observatory, /brain/perception). A rota base /brain é
// pública (adicionada a publicPaths) e serve HTML estático self-hostado; os
// endpoints devolvem as projeções mínimas sanitizadas (read-only).
type Handler struct {
	builder      *Builder
	static       fs.FS
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

// NewHandler cria um Handler com o go:embed da UI e um Builder de grafo.
// managers nil-safe — o grafo renderiza vazio em vez de pânico.
func NewHandler(agentsMgr *agents.Manager, skillsMgr *skills.Manager, version string) *Handler {
	return &Handler{
		static:  WebFS,
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

// Serve devolve o index.html do visualizador na raiz /brain.
func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	h.mount(w, r)
}

// mount serve o asset estático sob /brain/<arquivo>. Resolve a rota para o
// arquivo embutido, com proteção contra path traversal.
func (h *Handler) mount(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/brain")
	name = strings.TrimPrefix(name, "/")
	if name == "" || name == "/" {
		name = "index.html"
	}
	// Limpa o prefixo ./../ e normaliza para path do embed.FS (sempre "/").
	name = strings.ReplaceAll(name, "..", "")
	name = strings.TrimPrefix(name, "/")
	clean := path.Clean(name)

	f, err := h.static.Open("web/" + clean)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	// Lê o conteúdo em memória para servir via io.ReadSeeker (fs.File
	// do go:embed não implementa Seek, exigido por http.ServeContent).
	data, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "falha ao ler asset", http.StatusInternalServerError)
		return
	}

	// Define o tipo de conteúdo por extensão (para CSP correto).
	ct := contentType(clean)
	if ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	// Não usa ETag/Last-Modified agressivo — o index.html é revalidado no
	// browser; assets versionados ficam em cache curto.
	http.ServeContent(w, r, clean, time.Time{}, bytes.NewReader(data))
}

// writeJSON serializa JSON com os headers corretos.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// contentType resolve o MIME type por extensão.
func contentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".mjs"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".wasm"):
		return "application/wasm"
	case strings.HasSuffix(name, ".obj"):
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
