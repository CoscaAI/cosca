package handler

import (
	"net/http"
	"sort"

	"github.com/CoscaAI/cosca/internal/shadow"
)

// ShadowHandler expõe a metrologia do Cognitive Shadow Mode (ADR-033) — o
// painel "Casa Visível" consome por aqui as visões L1/L2/L3 sobre o ledger da
// deliberação contrafactual (.cosca/shadow/records.jsonl), append-only.
//
// É read-only e nil-safe: quando o store não é injetado, o construtor cai no
// shadow.DefaultStore() (que resolve o diretório `.cosca` do projeto). A leitura
// é tolerante a arquivo inexistente/vazio → lista vazia, nunca panic.
type ShadowHandler struct {
	store *shadow.Store
}

// NewShadowHandler cria um ShadowHandler. store nil→ DefaultStore (o diretório
// `.cosca` do projeto). O default mantém as rotas vivas mesmo sem nenhuma
// observação gravada.
func NewShadowHandler(store *shadow.Store) *ShadowHandler {
	if store == nil {
		store = shadow.DefaultStore()
	}
	return &ShadowHandler{store: store}
}

// Summary handles GET /v1/shadow/summary — L1/L2: resumo agregado da metrologia
// (observações, taxa de auto-resolução EMIT_OK sem LLM, taxa de escalada,
// confiança média, convergência média e distribuição de decisões).
func (h *ShadowHandler) Summary(w http.ResponseWriter, _ *http.Request) {
	sum, err := h.store.Summary()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao consultar resumo do shadow: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

// Histogram handles GET /v1/shadow/histogram — L1/L2: histograma de confiança
// (buckets 0.0–1.0) para o gráfico de calibração de thresholds (emit 0.70 /
// reserva 0.50).
func (h *ShadowHandler) Histogram(w http.ResponseWriter, _ *http.Request) {
	rep, err := h.store.Report()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao consultar histograma do shadow: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"histogram": rep.Histogram,
		"total":     rep.Total,
	})
}

// Decisions handles GET /v1/shadow/decisions — L2: lista das observações
// Shadow mais recentes (mais recente primeiro), com paginação limit/offset.
func (h *ShadowHandler) Decisions(w http.ResponseWriter, r *http.Request) {
	traces, err := h.store.Read()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao consultar observações do shadow: "+err.Error())
		return
	}

	offset := parseIntParam(r.URL.Query().Get("offset"), 0)
	limit := parseIntParam(r.URL.Query().Get("limit"), 20)
	if limit > 200 {
		limit = 200
	}

	// Mais recente primeiro.
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
	page := traces[offset:end]
	if page == nil {
		page = []shadow.ShadowTrace{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"observations": page,
		"total":        len(traces),
		"offset":       offset,
		"limit":        limit,
	})
}

// Get handles GET /v1/shadow/decisions/{id} — L3: autópsia de UMA observação
// Shadow (a decisão contrafactual completa) para o request dado. O {id} é o
// request_id (UUID) da execução instrumentada.
func (h *ShadowHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	traces, err := h.store.List(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao consultar observação do shadow: "+err.Error())
		return
	}
	if len(traces) == 0 {
		writeError(w, http.StatusNotFound, "observação shadow não encontrada para o request_id dado")
		return
	}

	// Várias observações podem partilhar um request_id — devolve a mais recente
	// como primária e a lista completa como histórico.
	sort.SliceStable(traces, func(i, j int) bool {
		return traces[i].At.After(traces[j].At)
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"request_id":   id,
		"observation":  traces[0],
		"observations": traces,
	})
}
