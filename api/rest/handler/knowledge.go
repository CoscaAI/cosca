package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/api/stream"
	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/search"
)

// KnowledgeHandler handles REST API requests for the Knowledge Engine.
// When a gRPC KnowledgeClient is configured (Single Owner Model, Fase 3),
// operations are delegated to the runtime daemon instead of the local engine.
type KnowledgeHandler struct {
	engine     *knowledge.Engine
	auditStore *audit.Store
	hub        *stream.Hub
	// resolver é o roteador determinístico (modlink) — FASE 1 routing/scope.
	// Nil = sem roteamento (legacy), comportamento atual. Configurado via
	// SetRouteResolver com o boot.RouteResolver do bootstrap.
	resolver *modlink.Resolver
	// mode é o modo de busca ("legacy" | "modular"). Só ativa o confinamento por
	// escopo quando both resolver não-nil e mode=="modular".
	mode string
	// lawsPath é o caminho do arquivo runtime das leis do CKL
	// (.cosca/knowledge/laws.json). Vazio → resolvido por request (cwd,
	// espelhando o CLI). Campo aditivo — handlers existentes não são afetados.
	lawsPath string
	// gRPC client for the runtime daemon (Single Owner Model, Fase 3).
	// When set, the handler delegates knowledge operations to the daemon
	// instead of using the local engine.
	grpcClient interface {
		Search(ctx context.Context, req *cospb.SearchRequest) (*cospb.SearchResponse, error)
		Index(ctx context.Context, req *cospb.IndexRequest) (*cospb.IndexResponse, error)
		Stats(ctx context.Context, req *cospb.StatsRequest) (*cospb.StatsResponse, error)
		Sync(ctx context.Context, req *cospb.SyncRequest) (*cospb.SyncResponse, error)
	}
}

// grpcAvailable returns true when the handler should delegate to the runtime
// daemon via gRPC (Single Owner Model).
func (h *KnowledgeHandler) grpcAvailable() bool { return h.grpcClient != nil }

// Pagination hard caps (M9b — DoS hardening). Client-supplied limit/offset
// are clamped so a single request can never force the engine to materialize
// an unbounded result set.
const (
	maxSearchLimit  = 100
	maxSearchOffset = 1000
)

// NewKnowledgeHandler creates a new KnowledgeHandler.
func NewKnowledgeHandler(engine *knowledge.Engine, auditStore *audit.Store) *KnowledgeHandler {
	return &KnowledgeHandler{engine: engine, auditStore: auditStore}
}

// SetHub sets the WebSocket Hub for broadcasting real-time events.
// May be nil if WebSocket is disabled.
func (h *KnowledgeHandler) SetHub(hub *stream.Hub) {
	h.hub = hub
}

// SetLawsPath define o caminho do arquivo runtime de leis do CKL
// (.cosca/knowledge/laws.json). Vazio restaura o default (cwd, espelhando o
// CLI). Opção aditiva para testes e deploys — handlers existentes seguem sem
// essa configuração.
func (h *KnowledgeHandler) SetLawsPath(path string) {
	h.lawsPath = path
}

// SetRouteResolver configura o roteador determinístico (modlink) para o
// confinamento por escopo (FASE 1 routing/scope). Passar nil restaura o
// comportamento atual (legacy, sem roteamento).
func (h *KnowledgeHandler) SetRouteResolver(resolver *modlink.Resolver) {
	h.resolver = resolver
}

// SetSearchMode define o modo de busca ("legacy" | "modular"). Só tem efeito
// quando um resolver não-nil está configurado e o modo é "modular".
func (h *KnowledgeHandler) SetSearchMode(mode string) {
	h.mode = mode
}

// modularSearchActive reporta se o confinamento por escopo roteado está ativo.
func (h *KnowledgeHandler) modularSearchActive() bool {
	return h.resolver != nil && h.mode == search.ModeModular
}

// SetKnowledgeClient configures the gRPC client for delegating knowledge
// operations to the runtime daemon (Single Owner Model, Fase 3).
func (h *KnowledgeHandler) SetKnowledgeClient(client interface {
	Search(ctx context.Context, req *cospb.SearchRequest) (*cospb.SearchResponse, error)
	Index(ctx context.Context, req *cospb.IndexRequest) (*cospb.IndexResponse, error)
	Stats(ctx context.Context, req *cospb.StatsRequest) (*cospb.StatsResponse, error)
	Sync(ctx context.Context, req *cospb.SyncRequest) (*cospb.SyncResponse, error)
}) {
	h.grpcClient = client
}

// --- Request / Response types ---

// SearchRequest is the JSON body for knowledge search.
type SearchRequest struct {
	Query       string            `json:"query"`
	Limit       int               `json:"limit,omitempty"`
	Offset      int               `json:"offset,omitempty"`
	Types       []string          `json:"types,omitempty"`
	PathFilter  string            `json:"path_filter,omitempty"`
	MinScore    float64           `json:"min_score,omitempty"`
	EnableFTS   *bool             `json:"enable_fts,omitempty"`
	EnableVec   *bool             `json:"enable_vector,omitempty"`
	EnableGraph *bool             `json:"enable_graph,omitempty"`
	EnableFacet *bool             `json:"enable_facets,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// SearchResult is a single search result in the API response.
type SearchResult struct {
	ID      string  `json:"id"`
	Title   string  `json:"title,omitempty"`
	Snippet string  `json:"snippet,omitempty"`
	Score   float64 `json:"score"`
	Type    string  `json:"type"`
	Path    string  `json:"path,omitempty"`
}

// SearchResponse is the JSON response for knowledge search.
type SearchResponse struct {
	Results    []SearchResult            `json:"results"`
	Total      int                       `json:"total"`
	DurationMs float64                   `json:"duration_ms"`
	Facets     map[string]map[string]int `json:"facets,omitempty"`
	// NoRoute (FASE 1 routing/scope, modo modular) sinaliza que o roteador não
	// encontrou um espaço semântico confiável para a consulta → 0 resultados,
	// sem full-scan. Sempre false em modo legacy.
	NoRoute bool `json:"no_route,omitempty"`
	// Scope lista os módulos aos quais a busca foi confinada (modo modular).
	// Vazio em modo legacy ou em NoRoute.
	Scope []string `json:"scope,omitempty"`
}

// IndexRequest is the JSON body for knowledge index.
type IndexRequest struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
}

// IndexResponse is the JSON response for knowledge index.
type IndexResponse struct {
	DocumentsIndexed int      `json:"documents_indexed"`
	ChunksIndexed    int      `json:"chunks_indexed"`
	Errors           []string `json:"errors,omitempty"`
}

// StatsResponse is the JSON response for knowledge stats.
type StatsResponse struct {
	DocumentCount int                    `json:"document_count"`
	ChunkCount    int                    `json:"chunk_count"`
	EntityCount   int                    `json:"entity_count"`
	VectorCount   int                    `json:"vector_count"`
	DBSizeBytes   int64                  `json:"db_size_bytes"`
	UptimeSeconds float64                `json:"uptime_seconds"`
	CacheStats    map[string]int         `json:"cache_stats,omitempty"`
	GraphStats    map[string]interface{} `json:"graph_stats,omitempty"`
	LastIndexed   string                 `json:"last_indexed,omitempty"`
}

// SyncResponse is the JSON response for knowledge sync.
type SyncResponse struct {
	Added      int      `json:"added"`
	Updated    int      `json:"updated"`
	Removed    int      `json:"removed"`
	Errors     []string `json:"errors,omitempty"`
	DurationMs float64  `json:"duration_ms"`
}

// EpistemologyResponse é a resposta de GET /v1/knowledge/epistemology: o
// resumo honesto do que o sistema sabe (e NÃO sabe) sobre as leis do CKL.
type EpistemologyResponse struct {
	Total       int                       `json:"total"`
	ByStatus    map[string]int            `json:"by_status"`
	Problematic []EpistemologyProblemItem `json:"problematic"`
}

// EpistemologyProblemItem é um item "problemático" (CONFLICTING/UNKNOWN/
// STALE) no resumo — o que a tela Unknowns/Unverified precisa destacar.
type EpistemologyProblemItem struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Status            string `json:"status"`
	StatusDescription string `json:"status_description"`
}

// EpistemologyStatusResponse é a resposta de GET /v1/knowledge/epistemology/
// {status}: os itens em um estado epistemológico específico.
type EpistemologyStatusResponse struct {
	Status string                   `json:"status"`
	Items  []EpistemologyStatusItem `json:"items"`
}

// EpistemologyStatusItem é um item com um estado epistemológico específico.
type EpistemologyStatusItem struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Status             string `json:"status"`
	LastVerified       string `json:"last_verified"`
	VerificationCount  int    `json:"verification_count"`
	ContradictionCount int    `json:"contradiction_count"`
}

// --- Handlers ---

// Search handles POST /v1/knowledge/search
func (h *KnowledgeHandler) Search(w http.ResponseWriter, r *http.Request) {
	// Single Owner Model (Fase 3): delegate to runtime daemon via gRPC.
	if h.grpcAvailable() {
		h.searchViaGRPC(w, r)
		return
	}
	var req SearchRequest
	limitBody(w, r, bodyLimitMedium)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	params := search.DefaultSearchParams()
	params.Query = req.Query
	if req.Limit > 0 {
		params.Limit = req.Limit
	}
	if req.Offset > 0 {
		params.Offset = req.Offset
	}
	// Clamp pagination to hard caps (M9b): limit ≤ 100, offset ≤ 1000.
	if params.Limit > maxSearchLimit {
		params.Limit = maxSearchLimit
	}
	if params.Offset > maxSearchOffset {
		params.Offset = maxSearchOffset
	}
	if len(req.Types) > 0 {
		params.Types = req.Types
	}
	if req.PathFilter != "" {
		params.Path = req.PathFilter
	}
	if req.MinScore > 0 {
		params.MinScore = req.MinScore
	}
	if req.EnableFTS != nil {
		params.EnableFTS = *req.EnableFTS
	}
	if req.EnableVec != nil {
		params.EnableVector = *req.EnableVec
	}
	if req.EnableGraph != nil {
		params.EnableGraph = *req.EnableGraph
	}
	if req.EnableFacet != nil {
		params.EnableFacets = *req.EnableFacet
	}
	if len(req.Tags) > 0 {
		params.Tags = req.Tags
	}

	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge engine not available")
		return
	}
	ctx := r.Context()

	// FASE 1 routing/scope (modo modular): o roteador determina o espaço de
	// busca; a busca confina a ele. NoRoute → resposta vazia + no_route:true,
	// NUNCA full-scan silencioso. Em modo legacy nada muda.
	if h.modularSearchActive() {
		scoped, scope := search.ApplyScope(h.resolver, req.Query, params)
		if scope.NoRoute {
			writeJSON(w, http.StatusOK, SearchResponse{
				Results: make([]SearchResult, 0),
				Total:   0,
				NoRoute: true,
			})
			return
		}
		params = scoped
	}

	results, err := h.engine.Search(ctx, params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed: "+err.Error())
		return
	}

	resp := SearchResponse{
		Results:    make([]SearchResult, 0, len(results.Results)),
		Total:      results.TotalCount,
		DurationMs: results.Duration.Seconds() * 1000,
	}
	if h.modularSearchActive() && params.Scope != nil {
		resp.Scope = params.Scope.Modules
	}

	for _, res := range results.Results {
		resp.Results = append(resp.Results, SearchResult{
			ID:      res.ID,
			Title:   res.DocumentID,
			Snippet: res.Heading,
			Score:   res.Score,
			Type:    string(res.Type),
			Path:    res.DocumentPath,
		})
	}

	if results.Facets != nil {
		resp.Facets = results.Facets
	}

	writeJSON(w, http.StatusOK, resp)
}

// Index handles POST /v1/knowledge/index
func (h *KnowledgeHandler) Index(w http.ResponseWriter, r *http.Request) {
	// Single Owner Model (Fase 3): delegate to runtime daemon via gRPC.
	if h.grpcAvailable() {
		h.indexViaGRPC(w, r)
		return
	}
	var req IndexRequest
	limitBody(w, r, bodyLimitMedium)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	// Lightweight pre-validation: normalize the path (relative paths are
	// resolved against the working directory) and reject obvious ".."
	// escape attempts. The authoritative, symlink-aware containment check
	// against the engine RootDir lives in the indexer (validatePathWithin).
	resolvedPath, err := validateIndexPath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "index failed: "+err.Error())
		return
	}

	ctx := r.Context()
	if req.Recursive {
		err = h.engine.IndexDirectory(ctx, resolvedPath)
	} else {
		err = h.engine.IndexDocument(ctx, resolvedPath)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "index failed: "+err.Error())
		LogEvent(h.auditStore, r, "knowledge.index", "knowledge:"+req.Path, audit.DetailsJSON(map[string]interface{}{"path": req.Path, "recursive": req.Recursive, "error": err.Error()}), "error")
		return
	}

	LogEvent(h.auditStore, r, "knowledge.index", "knowledge:"+req.Path, audit.DetailsJSON(map[string]interface{}{"path": req.Path, "recursive": req.Recursive}), "success")

	resp := IndexResponse{
		DocumentsIndexed: 1,
	}

	writeJSON(w, http.StatusOK, resp)
}

// validateIndexPath performs a lightweight normalization and traversal
// sanity check on a user-supplied indexing path. Relative paths are resolved
// against the current working directory (matching the engine's default root),
// and raw ".." segments are rejected so paths like "../../../etc/passwd" are
// never resolved against the server working directory. The authoritative
// containment check (symlink-resolved, RootDir-scoped) lives in the indexer.
func validateIndexPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path is required")
	}
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("invalid path: NUL byte")
	}

	// Reject raw traversal segments. filepath.Clean would silently resolve
	// "..", but a user-supplied relative path must not escape the working
	// directory via traversal before the indexer sees it.
	rest := p
	if vol := filepath.VolumeName(p); vol != "" {
		rest = p[len(vol):]
	}
	for _, seg := range strings.Split(rest, string(filepath.Separator)) {
		if seg == ".." {
			return "", fmt.Errorf("invalid path: traversal not allowed")
		}
	}

	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	return filepath.Clean(abs), nil
}

// Stats handles GET /v1/knowledge/stats
func (h *KnowledgeHandler) Stats(w http.ResponseWriter, _ *http.Request) {
	// Single Owner Model (Fase 3): delegate to runtime daemon via gRPC.
	if h.grpcAvailable() {
		h.statsViaGRPC(w)
		return
	}
	stats, err := h.engine.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats: "+err.Error())
		return
	}

	resp := StatsResponse{
		DocumentCount: stats.DocumentCount,
		ChunkCount:    stats.ChunkCount,
		EntityCount:   stats.EntityCount,
		VectorCount:   stats.VectorCount,
		DBSizeBytes:   stats.DBSize,
		UptimeSeconds: stats.Uptime.Seconds(),
	}

	if stats.CacheStats != nil {
		resp.CacheStats = stats.CacheStats
	}

	resp.GraphStats = map[string]interface{}{
		"nodes":      stats.GraphStats.Nodes,
		"edges":      stats.GraphStats.Edges,
		"density":    stats.GraphStats.Density,
		"components": stats.GraphStats.Components,
		"is_empty":   stats.GraphStats.IsEmpty,
	}

	if !stats.LastIndexed.IsZero() {
		resp.LastIndexed = stats.LastIndexed.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, resp)
}

// SymbolsSearch busca símbolos de código semanticamente (GET /v1/symbols/search?q=...).
// Gera o embedding da query e ordena por similaridade coseno — achar a função
// pelo que ela FAZ, não só pelo nome.
func (h *KnowledgeHandler) SymbolsSearch(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge engine unavailable")
		return
	}
	q := r.URL.Query().Get("q")
	if strings.TrimSpace(q) == "" {
		writeError(w, http.StatusBadRequest, "missing query parameter 'q'")
		return
	}
	limit := 10
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
		if limit > maxSearchLimit {
			limit = maxSearchLimit
		}
	}

	hits, err := h.engine.SearchSymbols(r.Context(), q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "symbols search failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"query":   q,
		"results": hits,
	})
}

// epistemicStatuses é a ordem estável dos 6 estados para o by_status.
var epistemicStatuses = []knowledge.EpistemicStatus{
	knowledge.StatusKnown,
	knowledge.StatusSupported,
	knowledge.StatusUncertain,
	knowledge.StatusConflicting,
	knowledge.StatusUnknown,
	knowledge.StatusStale,
}

// isProblematicStatus marca os estados que a tela Unknowns/Unverified precisa
// destacar: evidências contraditórias, sem evidência ou expirado.
func isProblematicStatus(s knowledge.EpistemicStatus) bool {
	switch s {
	case knowledge.StatusConflicting, knowledge.StatusUnknown, knowledge.StatusStale:
		return true
	default:
		return false
	}
}

// formatVerified serializa last_verified como RFC3339 (vazio quando zero).
func formatVerified(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// knowledgeItems devolve os itens de conhecimento do CKL. Quando o arquivo
// não pode ser lido (inexistente/corrompido), degrada para lista vazia —
// nunca 500: a UI mostra "eu não sei" em vez de erro.
func (h *KnowledgeHandler) knowledgeItems() []knowledge.KnowledgeItem {
	path := h.lawsPath
	if path == "" {
		path = knowledge.ProjectLawsPath()
	}
	items, err := knowledge.ListKnowledgeItems(path)
	if err != nil {
		return nil
	}
	return items
}

// Epistemology handles GET /v1/knowledge/epistemology — o resumo
// epistemológico: total de itens, contagem por estado (os 6 do CKL) e a lista
// dos "problemáticos" (CONFLICTING/UNKNOWN/STALE).
func (h *KnowledgeHandler) Epistemology(w http.ResponseWriter, _ *http.Request) {
	items := h.knowledgeItems()

	resp := EpistemologyResponse{
		Total:       len(items),
		ByStatus:    make(map[string]int, len(epistemicStatuses)),
		Problematic: make([]EpistemologyProblemItem, 0),
	}
	for _, s := range epistemicStatuses {
		resp.ByStatus[string(s)] = 0
	}

	for _, it := range items {
		st := it.Status
		if st == "" {
			st = knowledge.StatusUnknown
		}
		resp.ByStatus[string(st)]++
		if isProblematicStatus(st) {
			resp.Problematic = append(resp.Problematic, EpistemologyProblemItem{
				ID:                it.ID,
				Title:             it.Title,
				Status:            string(st),
				StatusDescription: st.Description(),
			})
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// EpistemologyByStatus handles GET /v1/knowledge/epistemology/{status} — os
// itens em um estado epistemológico específico. Status inválido → 400 pt-BR.
func (h *KnowledgeHandler) EpistemologyByStatus(w http.ResponseWriter, r *http.Request) {
	status := knowledge.EpistemicStatus(r.PathValue("status"))
	if status == "" || !status.Valid() {
		writeError(w, http.StatusBadRequest, "status inválido — use KNOWN, SUPPORTED, UNCERTAIN, CONFLICTING, UNKNOWN ou STALE")
		return
	}

	items := h.knowledgeItems()
	resp := EpistemologyStatusResponse{
		Status: string(status),
		Items:  make([]EpistemologyStatusItem, 0),
	}
	for _, it := range items {
		st := it.Status
		if st == "" {
			st = knowledge.StatusUnknown
		}
		if st != status {
			continue
		}
		resp.Items = append(resp.Items, EpistemologyStatusItem{
			ID:                 it.ID,
			Title:              it.Title,
			Status:             string(st),
			LastVerified:       formatVerified(it.LastVerified),
			VerificationCount:  it.VerificationCount,
			ContradictionCount: it.ContradictionCount,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// Sync handles POST /v1/knowledge/sync
func (h *KnowledgeHandler) Sync(w http.ResponseWriter, r *http.Request) {
	// Single Owner Model (Fase 3): delegate to runtime daemon via gRPC.
	if h.grpcAvailable() {
		h.syncViaGRPC(w, r)
		return
	}
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge engine not available")
		return
	}
	ctx := r.Context()
	result, err := h.engine.Sync(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sync failed: "+err.Error())
		LogEvent(h.auditStore, r, "knowledge.sync", "knowledge", audit.DetailsJSON(map[string]interface{}{"error": err.Error()}), "error")
		return
	}

	LogEvent(h.auditStore, r, "knowledge.sync", "knowledge", audit.DetailsJSON(map[string]interface{}{"added": len(result.Added), "updated": len(result.Updated), "removed": len(result.Removed)}), "success")

	resp := SyncResponse{
		Added:      len(result.Added),
		Updated:    len(result.Updated),
		Removed:    len(result.Removed),
		Errors:     result.Errors,
		DurationMs: result.Duration.Seconds() * 1000,
	}

	writeJSON(w, http.StatusOK, resp)
}

// SyncStream handles POST /v1/knowledge/sync/stream — executes a knowledge
// sync and streams progress back as Server-Sent Events (SSE).
func (h *KnowledgeHandler) SyncStream(w http.ResponseWriter, r *http.Request) {
	// Single Owner Model (Fase 3): delegate to runtime daemon via gRPC.
	if h.grpcAvailable() {
		h.syncViaGRPC(w, r)
		return
	}
	// Set up SSE writer and headers.
	sw, err := stream.NewSSEWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	defer sw.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Monitor client disconnect.
	go func() {
		select {
		case <-r.Context().Done():
			cancel()
		case <-ctx.Done():
		}
	}()

	// Send initial progress event.
	_ = sw.WriteEvent(stream.EventProgress, knowledge.SyncProgress{
		Phase:   "starting",
		Message: "Knowledge sync starting...",
	})

	// Broadcast sync started to WebSocket subscribers.
	if h.hub != nil {
		h.hub.BroadcastEvent([]string{"sync"}, "sync_started", nil)
	}

	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge engine not available")
		return
	}
	result, syncErr := h.engine.SyncWithProgress(ctx, func(p knowledge.SyncProgress) {
		_ = sw.WriteEvent(stream.EventProgress, p)
		// Forward progress to WebSocket subscribers.
		if h.hub != nil {
			h.hub.BroadcastEvent([]string{"sync"}, "sync_progress", map[string]interface{}{
				"phase":     p.Phase,
				"message":   p.Message,
				"processed": p.Processed,
				"total":     p.Total,
			})
		}
	})

	if syncErr != nil {
		LogEvent(h.auditStore, r, "knowledge.sync.stream", "knowledge",
			audit.DetailsJSON(map[string]interface{}{"error": syncErr.Error()}), "error")
		_ = sw.WriteError(fmt.Errorf("sync failed: %v", syncErr))
		return
	}

	LogEvent(h.auditStore, r, "knowledge.sync.stream", "knowledge",
		audit.DetailsJSON(map[string]interface{}{
			"added":   len(result.Added),
			"updated": len(result.Updated),
			"removed": len(result.Removed),
			"errors":  len(result.Errors),
		}), "success")

	// Broadcast sync completed to WebSocket subscribers.
	if h.hub != nil {
		h.hub.BroadcastEvent([]string{"sync"}, "sync_completed", map[string]interface{}{
			"added":       len(result.Added),
			"updated":     len(result.Updated),
			"removed":     len(result.Removed),
			"errors":      len(result.Errors),
			"duration_ms": result.Duration.Milliseconds(),
		})
	}

	// Send final result as a "done" event with the sync summary.
	syncSummary := map[string]interface{}{
		"type":        stream.EventDone,
		"added":       len(result.Added),
		"updated":     len(result.Updated),
		"removed":     len(result.Removed),
		"errors":      result.Errors,
		"duration_ms": result.Duration.Milliseconds(),
	}
	_ = sw.WriteEvent(stream.EventDone, syncSummary)
}

// ─── gRPC delegation methods (Single Owner Model, Fase 3) ────────────────────

func (h *KnowledgeHandler) searchViaGRPC(w http.ResponseWriter, r *http.Request) {
	if !h.grpcAvailable() {
		writeError(w, http.StatusServiceUnavailable, "gRPC client not configured")
		return
	}

	var req SearchRequest
	limitBody(w, r, bodyLimitMedium)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}
	grpcReq := &cospb.SearchRequest{
		Query:      req.Query,
		Limit:      int32(req.Limit),
		Types:      req.Types,
		PathFilter: req.PathFilter,
		MinScore:   req.MinScore,
	}
	resp, err := h.grpcClient.Search(r.Context(), grpcReq)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge search via runtime daemon failed: "+err.Error())
		return
	}
	results := make([]SearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = SearchResult{
			ID:      r.Id,
			Title:   r.Title,
			Snippet: r.Snippet,
			Score:   float64(r.Score),
			Type:    r.Type,
			Path:    r.Path,
		}
	}
	writeJSON(w, http.StatusOK, SearchResponse{
		Results:    results,
		Total:      int(resp.Total),
		DurationMs: float64(resp.DurationMs),
	})
}

func (h *KnowledgeHandler) indexViaGRPC(w http.ResponseWriter, r *http.Request) {
	if !h.grpcAvailable() {
		writeError(w, http.StatusServiceUnavailable, "gRPC client not configured")
		return
	}

	var req IndexRequest
	limitBody(w, r, bodyLimitMedium)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	grpcReq := &cospb.IndexRequest{
		Path:      req.Path,
		Recursive: req.Recursive,
	}
	resp, err := h.grpcClient.Index(r.Context(), grpcReq)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge index via runtime daemon failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, IndexResponse{
		DocumentsIndexed: int(resp.DocumentsIndexed),
		ChunksIndexed:    int(resp.ChunksIndexed),
		Errors:           resp.Errors,
	})
}

func (h *KnowledgeHandler) statsViaGRPC(w http.ResponseWriter) {
	if !h.grpcAvailable() {
		writeError(w, http.StatusServiceUnavailable, "gRPC client not configured")
		return
	}

	resp, err := h.grpcClient.Stats(context.Background(), &cospb.StatsRequest{})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge stats via runtime daemon failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, StatsResponse{
		DocumentCount: int(resp.DocumentCount),
		ChunkCount:    int(resp.ChunkCount),
		EntityCount:   int(resp.EntityCount),
		VectorCount:   int(resp.VectorCount),
		DBSizeBytes:   resp.DbSizeBytes,
		UptimeSeconds: float64(resp.UptimeSeconds),
	})
}

func (h *KnowledgeHandler) syncViaGRPC(w http.ResponseWriter, r *http.Request) {
	if !h.grpcAvailable() {
		writeError(w, http.StatusServiceUnavailable, "gRPC client not configured")
		return
	}

	resp, err := h.grpcClient.Sync(r.Context(), &cospb.SyncRequest{})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge sync via runtime daemon failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, SyncResponse{
		Added:   int(resp.Added),
		Updated: int(resp.Updated),
		Removed: int(resp.Removed),
		Errors:  resp.Errors,
	})
}
