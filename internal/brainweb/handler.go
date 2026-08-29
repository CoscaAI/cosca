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
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/CoscaAI/cosca/internal/trace"
)

// Handler serve o visualizador 3D (/brain) e o contrato de dados (/brain/graph,
// /brain/activity, /brain/observatory). A rota base /brain é pública (adicionada
// a publicPaths) e serve HTML estático self-hostado; os endpoints devolvem as
// projeções mínimas sanitizadas (read-only).
type Handler struct {
	builder      *Builder
	static       fs.FS
	activity     ActivitySource
	observatory  *ObservatoryBuilder
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
