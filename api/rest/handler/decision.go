package handler

import (
	"net/http"
	"sort"

	"github.com/CoscaAI/cosca/internal/deliberate"
	"github.com/CoscaAI/cosca/internal/shadow"
	"github.com/CoscaAI/cosca/internal/trace"
)

// DecisionHandler expõe o Decision Trace operacional — a cadeia causal
// intenção do usuário → contexto assembrado → evidência → deliberação do kernel
// → decisão → capacidade → agente → ferramentas → resultado — para o painel
// "Casa Visível" (níveis L2/L3). Combina o ledger do Shadow (a decisão
// contrafactual do kernel, ADR-033) com o flight recorder causal (internal/trace).
//
// Read-only e nil-safe: o store do shadow cai no DefaultStore; o store do trace
// é opcional — quando nil, o enriquecimento causal é simplesmente omitido.
type DecisionHandler struct {
	shadow *shadow.Store
	trace  *trace.Store
}

// NewDecisionHandler cria um DecisionHandler. shadow nil → DefaultStore
// (.cosca do projeto); trace nil → enriquecimento causal ausente.
func NewDecisionHandler(shadowStore *shadow.Store, traceStore *trace.Store) *DecisionHandler {
	if shadowStore == nil {
		shadowStore = shadow.DefaultStore()
	}
	return &DecisionHandler{shadow: shadowStore, trace: traceStore}
}

// List handles GET /v1/decisions — L2: lista das decisões recentes (a partir do
// ledger do shadow), mais recente primeiro, paginadas (limit/offset).
func (h *DecisionHandler) List(w http.ResponseWriter, r *http.Request) {
	traces, err := h.shadow.Read()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao consultar decisões: "+err.Error())
		return
	}

	offset := parseIntParam(r.URL.Query().Get("offset"), 0)
	limit := parseIntParam(r.URL.Query().Get("limit"), 20)
	if limit > 200 {
		limit = 200
	}

	sort.SliceStable(traces, func(i, j int) bool {
		return traces[i].At.After(traces[j].At)
	})

	if offset > len(traces) {
		offset = len(traces)
	}
	end := offset + limit
	if end > len(traces) {
		end = len(traces)
	}

	items := make([]map[string]interface{}, 0, end-offset)
	for _, t := range traces[offset:end] {
		items = append(items, decisionSummaryItem(t))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"decisions": items,
		"total":     len(traces),
		"offset":    offset,
		"limit":     limit,
	})
}

// decisionSummaryItem projeta um ShadowTrace no item de decisão listável (L2).
func decisionSummaryItem(t shadow.ShadowTrace) map[string]interface{} {
	return map[string]interface{}{
		"id":             t.RequestID,
		"request_id":     t.RequestID,
		"agent":          t.Agent,
		"decision":       t.Decision,
		"would_escalate": t.WouldEscalate,
		"confidence":     t.Confidence,
		"convergence":    t.Convergence,
		"positions":      t.Positions,
		"evidence_ids":   t.EvidenceIDs,
		"reason":         t.Reason,
		"timestamp":      t.At,
	}
}

// Get handles GET /v1/decisions/{id} — L3: o Decision Trace completo. O {id}
// pode ser OU um TraceID do flight recorder (TRACE-YYYYMMDD-XXXX) OU um
// request_id do shadow (UUID). A resposta mistura o registro de decisão do
// shadow (autoritativo: decision/confidence/convergence/evidence/positions)
// com a cadeia causal do flight recorder (enriquecimento), quando o trace pode
// ser resolvido.
func (h *DecisionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	resp := map[string]interface{}{
		"id":          id,
		"resolved_as": "none",
	}

	// 1. Registro de decisão do shadow (autoritativo). Match por request_id.
	var shadowObs *shadow.ShadowTrace
	if all, err := h.shadow.Read(); err == nil {
		for i := range all {
			if all[i].RequestID == id {
				shadowObs = &all[i]
				break
			}
		}
	}

	// 2. Flight recorder causal. Match por TraceID (apenas quando o store existe).
	if parsed, ok := trace.Parse(id); ok && h.trace != nil {
		if events, err := h.trace.Get(parsed.String()); err == nil && len(events) > 0 {
			cg := trace.BuildCausalGraph(events)
			resp["resolved_as"] = "trace"
			resp["trace_id"] = parsed.String()
			resp["sequence"] = trace.SequenceFromEvents(events).Actions
			resp["causal"] = map[string]interface{}{
				"trace_id": cg.TraceID,
				"nodes":    cg.Nodes,
				"edges":    cg.Edges,
			}
			resp["events"] = events
			resp["first_action"] = events[0].Action
			resp["last_action"] = events[len(events)-1].Action
			resp["outcome"] = events[len(events)-1].Result
			resp["event_count"] = len(events)
		}
	}

	// 3. Aplica o registro de decisão do shadow sempre que disponível.
	if shadowObs != nil {
		resp["request_id"] = shadowObs.RequestID
		resp["agent"] = shadowObs.Agent
		resp["decision"] = shadowObs.Decision
		resp["would_escalate"] = shadowObs.WouldEscalate
		resp["confidence"] = shadowObs.Confidence
		resp["convergence"] = shadowObs.Convergence
		resp["evidence_ids"] = shadowObs.EvidenceIDs
		resp["positions"] = shadowObs.Positions
		resp["reason"] = shadowObs.Reason
		resp["breakdown"] = shadowObs.Breakdown
		resp["duration_ms"] = shadowObs.DurationMs
		resp["timestamp"] = shadowObs.At
		resp["deliberation"] = buildDeliberation(shadowObs)
		if shadowObs.WouldRespond != "" {
			resp["would_respond"] = shadowObs.WouldRespond
		}
		if resp["resolved_as"] == "none" {
			resp["resolved_as"] = "shadow"
		}
	}

	if resp["resolved_as"] == "none" {
		writeError(w, http.StatusNotFound, "decisão não encontrada para o id dado")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Stats handles GET /v1/deliberation/stats — L2: estatísticas agregadas da
// deliberação (convergência/confiança médias + taxas de escalada /
// auto-resolução + distribuição), a partir do ledger do shadow.
func (h *DecisionHandler) Stats(w http.ResponseWriter, _ *http.Request) {
	sum, err := h.shadow.Summary()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao consultar estatísticas de deliberação: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":             sum.Total,
		"avg_confidence":    sum.AvgConfidence,
		"avg_convergence":   sum.AvgConvergence,
		"escalation_rate":   sum.EscalationRate,
		"self_resolve_rate": sum.SelfResolveRate,
		"decisions":         sum.Decisions,
	})
}

// buildDeliberation projeta o objeto de deliberação (deliberate) a partir de
// uma observação Shadow — o subconjunto aritmético que o dashboard L3 consome:
// verdict emit (mapa ShadowDecision→Emit), confidence, convergence, evidence,
// conflicts (derivado do Reason) e a trilha breakdown. Reusa deliberate para a
// taxonomia Emit (ADR-032) e mantém a decisão Shadow como fonte (ADR-033).
func buildDeliberation(obs *shadow.ShadowTrace) map[string]interface{} {
	emit := emitForDecision(obs.Decision)
	return map[string]interface{}{
		"verdict":       emit,
		"deterministic": emit == deliberate.EmitOK,
		"confidence":    obs.Confidence,
		"convergence":   obs.Convergence,
		"evidence_ids":  obs.EvidenceIDs,
		"positions":     obs.Positions,
		"conflicts":     obs.Reason == "conflicting evidence",
		"breakdown":     obs.Breakdown,
	}
}

// emitForDecision mapeia a taxonomia do Shadow (ADR-033) para o Emit do
// deliberate (ADR-032). RETRIEVAL_INSUFFICIENT e ESCALATE convergem em
// "escalate"; EMIT_WITH_RESERVATIONS preserva o rótulo.
func emitForDecision(d shadow.ShadowDecision) deliberate.Emit {
	switch d {
	case shadow.EMIT_OK:
		return deliberate.EmitOK
	case shadow.EMIT_WITH_RESERVATIONS:
		return deliberate.EmitWithReservations
	default:
		return deliberate.Escalate
	}
}
