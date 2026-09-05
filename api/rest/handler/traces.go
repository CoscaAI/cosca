package handler

import (
	"fmt"
	"net/http"

	"github.com/CoscaAI/cosca/internal/trace"
)

// TracesHandler handles execution timeline + replay endpoints backed by the
// trace flight recorder (internal/trace, SQLite .cosca/trace.db). It is a
// read-only projection over the append-only event ledger.
type TracesHandler struct {
	store *trace.Store
}

// NewTracesHandler creates a new TracesHandler backed by the given store.
// A nil store is allowed (nil-safe): every route returns 503 with a clear
// message, so the rest of the server keeps working even when the ledger
// cannot be opened.
func NewTracesHandler(store *trace.Store) *TracesHandler {
	return &TracesHandler{store: store}
}

// List handles GET /v1/traces — returns the most recent traces as a summary
// list (most recent first), one entry per trace: id, event count, first
// action and last timestamp. The event ledger is append-only — reads only.
func (h *TracesHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "trace store indisponível")
		return
	}

	limit := parseIntParam(r.URL.Query().Get("limit"), 20)
	latest, err := h.store.Latest(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao consultar traces: "+err.Error())
		return
	}

	// Preserve recency order (Latest is timestamp DESC), deduplicating by
	// trace ID so each trace appears once in the summary.
	var traceIDs []string
	seen := make(map[string]bool)
	for _, e := range latest {
		if !seen[e.TraceID] {
			seen[e.TraceID] = true
			traceIDs = append(traceIDs, e.TraceID)
		}
	}

	type summary struct {
		ID          string `json:"id"`
		Events      int    `json:"events"`
		FirstAction string `json:"first_action"`
		LastTS      int64  `json:"last_ts"`
	}

	summaries := make([]summary, 0, len(traceIDs))
	for _, id := range traceIDs {
		evs, err := h.store.Get(id)
		if err != nil || len(evs) == 0 {
			continue
		}
		// Get returns events ordered by timestamp ASC — first event action,
		// last event timestamp.
		summaries = append(summaries, summary{
			ID:          id,
			Events:      len(evs),
			FirstAction: evs[0].Action,
			LastTS:      evs[len(evs)-1].Timestamp,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"traces": summaries})
}

// Get handles GET /v1/traces/{id} — returns the full flight-recorder
// timeline of a single trace: all events ordered by timestamp ASC.
func (h *TracesHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "trace store indisponível")
		return
	}

	id := r.PathValue("id")
	parsed, ok := trace.Parse(id)
	if !ok {
		writeError(w, http.StatusBadRequest, "Trace ID inválido — formato esperado: TRACE-YYYYMMDD-XXXXXXXX")
		return
	}

	events, err := h.store.Get(parsed.String())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao consultar trace: "+err.Error())
		return
	}
	if len(events) == 0 {
		writeError(w, http.StatusNotFound, "trace não encontrado")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"trace_id": parsed.String(),
		"events":   events,
	})
}

// Replay handles GET /v1/traces/{id}/replay — returns the ordered action
// sequence of the trace (the replay signature) plus divergence info when a
// prior trace exists to compare against. When there is no prior trace the
// divergence field is omitted.
func (h *TracesHandler) Replay(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "trace store indisponível")
		return
	}

	id := r.PathValue("id")
	parsed, ok := trace.Parse(id)
	if !ok {
		writeError(w, http.StatusBadRequest, "Trace ID inválido — formato esperado: TRACE-YYYYMMDD-XXXXXXXX")
		return
	}

	events, err := h.store.Get(parsed.String())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao consultar trace: "+err.Error())
		return
	}
	if len(events) == 0 {
		writeError(w, http.StatusNotFound, "trace não encontrado")
		return
	}

	sequence := trace.SequenceFromEvents(events)

	resp := replayResponse{
		TraceID:  parsed.String(),
		Sequence: sequence.Actions,
	}

	// Divergence is a deterministic heuristic (no LLM) against a prior trace
	// when one exists. No prior trace ⇒ no comparison data ⇒ divergence
	// omitted. Best-effort: any store error just skips the comparison.
	if baseline := h.findComparisonTrace(parsed.String()); baseline != "" {
		baselineEvents, err := h.store.Get(baseline)
		if err == nil && len(baselineEvents) > 0 {
			baselineSeq := trace.SequenceFromEvents(baselineEvents)
			positions := trace.DetectDivergence([]trace.Sequence{baselineSeq}, sequence)
			resp.Divergence = &divergenceInfo{
				Positions: positions,
				Note:      divergenceNote(baseline, positions),
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// Causal handles GET /v1/traces/{id}/causal — builds the causal graph
// (causa → consequência) from the trace events: each event becomes a node and
// consecutive events get a "caused" edge. Deterministic, no LLM. The
// divergence field is included only when a prior trace exists to compare
// against (same heuristic as Replay).
func (h *TracesHandler) Causal(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "trace store indisponível")
		return
	}

	id := r.PathValue("id")
	parsed, ok := trace.Parse(id)
	if !ok {
		writeError(w, http.StatusBadRequest, "Trace ID inválido — formato esperado: TRACE-YYYYMMDD-XXXXXXXX")
		return
	}

	events, err := h.store.Get(parsed.String())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao consultar trace: "+err.Error())
		return
	}
	if len(events) == 0 {
		writeError(w, http.StatusNotFound, "trace não encontrado")
		return
	}

	graph := trace.BuildCausalGraph(events)
	resp := causalResponse{
		TraceID: parsed.String(),
		Nodes:   graph.Nodes,
		Edges:   graph.Edges,
	}

	// Divergence is a deterministic heuristic (no LLM) against a prior trace
	// when one exists. No prior trace ⇒ no comparison data ⇒ divergence
	// omitted. Best-effort: any store error just skips the comparison.
	if baseline := h.findComparisonTrace(parsed.String()); baseline != "" {
		baselineEvents, err := h.store.Get(baseline)
		if err == nil && len(baselineEvents) > 0 {
			positions := graph.DivergencePoints(baselineEvents)
			resp.Divergence = &divergenceInfo{
				Positions: positions,
				Note:      divergenceNote(baseline, positions),
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

type causalResponse struct {
	TraceID    string             `json:"trace_id"`
	Nodes      []trace.CausalNode `json:"nodes"`
	Edges      []trace.CausalEdge `json:"edges"`
	Divergence *divergenceInfo    `json:"divergence,omitempty"`
}

// findComparisonTrace devolve o ID de um trace anterior (diferente do atual)
// para servir de baseline na detecção de divergência — ou "" quando não há
// histórico para comparar.
func (h *TracesHandler) findComparisonTrace(current string) string {
	latest, err := h.store.Latest(20)
	if err != nil {
		return ""
	}
	for _, e := range latest {
		if e.TraceID != current {
			return e.TraceID
		}
	}
	return ""
}

// divergenceNote descreve o resultado da comparação em pt-BR, no espírito do
// marcador "SUSPICIOUS DIVERGENCE — onde esta execução difere das anteriores".
func divergenceNote(baseline string, positions []int) string {
	if len(positions) == 0 {
		return "Sem divergência — a sequência coincide com a execução anterior (ou é uma extensão normal ao final)."
	}
	return fmt.Sprintf("Divergência suspeita (SUSPICIOUS DIVERGENCE) em relação ao trace %s nas posições %v — a execução difere das anteriores.", baseline, positions)
}

type divergenceInfo struct {
	Positions []int  `json:"positions"`
	Note      string `json:"note"`
}

type replayResponse struct {
	TraceID    string          `json:"trace_id"`
	Sequence   []string        `json:"sequence"`
	Divergence *divergenceInfo `json:"divergence,omitempty"`
}
