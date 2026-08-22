package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/memory"
	"google.golang.org/grpc/metadata"
)

// grpcAuthCtx returns a context carrying the authenticated caller's identity
// (subject, tenant, role) as outgoing gRPC metadata. The runtime daemon reads
// these on the server side and enforces owner/tenant scoping — without this,
// every authenticated user could read/write everyone's memory (IDOR).
func grpcAuthCtx(r *http.Request) context.Context {
	md := metadata.Pairs(
		"x-cosca-sub", claimsSubject(r),
		"x-cosca-tenant", claimsTenant(r),
		"x-cosca-role", claimsRole(r),
	)
	return metadata.NewOutgoingContext(r.Context(), md)
}

// MemoryHandler handles REST API requests for the Memory Engine.
// When a gRPC MemoryClient is configured (Single Owner Model, Fase 3),
// operations are delegated to the runtime daemon instead of the local engine.
type MemoryHandler struct {
	engine     *memory.MemoryEngine
	auditStore *audit.Store
	// gRPC client for the runtime daemon (Single Owner Model, Fase 3).
	grpcClient interface {
		Store(ctx context.Context, req *cospb.StoreRequest) (*cospb.StoreResponse, error)
		Search(ctx context.Context, req *cospb.MemorySearchRequest) (*cospb.MemorySearchResponse, error)
		Get(ctx context.Context, req *cospb.GetRequest) (*cospb.GetResponse, error)
		Delete(ctx context.Context, req *cospb.DeleteRequest) (*cospb.DeleteResponse, error)
		Promote(ctx context.Context, req *cospb.PromoteRequest) (*cospb.PromoteResponse, error)
		Stats(ctx context.Context, req *cospb.MemoryStatsRequest) (*cospb.MemoryStatsResponse, error)
	}
}

func (h *MemoryHandler) grpcAvailable() bool { return h.grpcClient != nil }

// maxMemorySearchLimit is the hard cap for the memory search limit
// (M9b — DoS hardening): a single request can never ask for an unbounded
// number of records.
const maxMemorySearchLimit = 100

// NewMemoryHandler creates a new MemoryHandler.
func NewMemoryHandler(engine *memory.MemoryEngine, auditStore *audit.Store) *MemoryHandler {
	return &MemoryHandler{engine: engine, auditStore: auditStore}
}

// SetMemoryClient configures the gRPC client for delegating memory operations
// to the runtime daemon (Single Owner Model, Fase 3).
func (h *MemoryHandler) SetMemoryClient(client interface {
	Store(ctx context.Context, req *cospb.StoreRequest) (*cospb.StoreResponse, error)
	Search(ctx context.Context, req *cospb.MemorySearchRequest) (*cospb.MemorySearchResponse, error)
	Get(ctx context.Context, req *cospb.GetRequest) (*cospb.GetResponse, error)
	Delete(ctx context.Context, req *cospb.DeleteRequest) (*cospb.DeleteResponse, error)
	Promote(ctx context.Context, req *cospb.PromoteRequest) (*cospb.PromoteResponse, error)
	Stats(ctx context.Context, req *cospb.MemoryStatsRequest) (*cospb.MemoryStatsResponse, error)
}) {
	h.grpcClient = client
}

// --- Request / Response types ---

// StoreRequest is the JSON body for memory store.
type StoreRequest struct {
	Type     string            `json:"type"`
	Layer    string            `json:"layer"`
	Scope    string            `json:"scope,omitempty"`
	Content  string            `json:"content"`
	Priority int               `json:"priority,omitempty"`
	TTL      string            `json:"ttl,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// StoreResponse is the JSON response for memory store.
type StoreResponse struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
}

// MemoryRecord is a single memory record in the API response.
type MemoryRecord struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Layer        string            `json:"layer"`
	Content      string            `json:"content"`
	ContentTrust string            `json:"content_trust"`
	Priority     int               `json:"priority"`
	CreatedAt    string            `json:"created_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// MemorySearchResponse is the JSON response for memory search.
type MemorySearchResponse struct {
	Records []MemoryRecord `json:"records"`
	Total   int            `json:"total"`
}

// GetResponse is the JSON response for memory get.
type GetResponse struct {
	Record MemoryRecord `json:"record"`
}

// DeleteResponse is the JSON response for memory delete.
type DeleteResponse struct {
	Success bool `json:"success"`
}

// PromoteRequest is the JSON body for memory promote.
type PromoteRequest struct {
	ID        string `json:"id"`
	FromLayer string `json:"from_layer"`
	ToLayer   string `json:"to_layer"`
}

// PromoteResponse is the JSON response for memory promote.
type PromoteResponse struct {
	Record MemoryRecord `json:"record"`
}

// LayerStats holds stats for a single memory layer.
type LayerStats struct {
	RecordCount int   `json:"record_count"`
	SizeBytes   int64 `json:"size_bytes"`
}

// MemoryStatsResponse is the JSON response for memory stats.
type MemoryStatsResponse struct {
	Layers map[string]LayerStats `json:"layers"`
}

// --- Handlers ---

// Store handles POST /v1/memory/store
func (h *MemoryHandler) Store(w http.ResponseWriter, r *http.Request) {
	if h.grpcAvailable() {
		h.storeViaGRPC(w, r)
		return
	}
	var req StoreRequest
	limitBody(w, r, bodyLimitLarge)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	record := memory.MemoryRecord{
		Type:     memory.MemoryType(req.Type),
		Layer:    memory.MemoryLayer(req.Layer),
		Scope:    req.Scope,
		Owner:    claimsSubject(r), // A6 — attribute the record to the authenticated user
		TenantID: claimsTenant(r),
		Content:  req.Content,
		Priority: req.Priority,
		Metadata: req.Metadata,
	}

	if req.TTL != "" {
		ttl, err := time.ParseDuration(req.TTL)
		if err == nil {
			record.TTL = ttl
		}
	}

	if record.Layer == "" {
		record.Layer = memory.LayerSession
	}

	ctx := r.Context()
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "memory engine not available")
		return
	}
	saved, err := h.engine.Store(ctx, record)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store failed: "+err.Error())
		// saved may be nil when Store returns an error — never dereference it
		// here. Fall back to a placeholder resource for the audit trail.
		resource := "unknown"
		if saved != nil {
			resource = saved.ID
		}
		LogEvent(h.auditStore, r, "memory.store", "memory:"+resource, audit.DetailsJSON(map[string]interface{}{"type": req.Type, "layer": req.Layer, "error": err.Error()}), "error")
		return
	}

	LogEvent(h.auditStore, r, "memory.store", "memory:"+saved.ID, audit.DetailsJSON(map[string]interface{}{"type": req.Type, "layer": string(record.Layer)}), "success")

	resp := StoreResponse{
		ID:        saved.ID,
		CreatedAt: saved.CreatedAt.Format(time.RFC3339),
	}

	writeJSON(w, http.StatusCreated, resp)
}

// Search handles GET /v1/memory/search?query=...&types=...&layers=...&limit=...
func (h *MemoryHandler) Search(w http.ResponseWriter, r *http.Request) {
	if h.grpcAvailable() {
		h.searchViaGRPC(w, r)
		return
	}
	query := r.URL.Query().Get("query")
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}
	// Clamp to the hard cap (M9b): limit ≤ 100.
	if limit > maxMemorySearchLimit {
		limit = maxMemorySearchLimit
	}
	types := r.URL.Query()["types"]
	layers := r.URL.Query()["layers"]
	// Accept the historical singular spelling used by older SDK clients.
	if len(layers) == 0 {
		layers = r.URL.Query()["layer"]
	}

	opts := memory.SearchOptions{
		Limit: limit,
		// A6 — scope results to the authenticated user's records plus
		// system/global records (empty owner). Anonymous callers see all.
		OwnerFilter:  claimsSubject(r),
		TenantFilter: claimsTenant(r),
	}
	if isAdmin(r) {
		// Empty filters are intentional: an admin explicitly has global scope.
	}

	for _, t := range types {
		opts.Types = append(opts.Types, memory.MemoryType(t))
	}
	for _, l := range layers {
		opts.Layers = append(opts.Layers, memory.MemoryLayer(l))
	}

	ctx := r.Context()
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "memory engine not available")
		return
	}
	records, err := h.engine.Search(ctx, query, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed: "+err.Error())
		return
	}

	resp := MemorySearchResponse{
		Records: make([]MemoryRecord, 0, len(records)),
		Total:   len(records),
	}

	for _, rec := range records {
		resp.Records = append(resp.Records, recordToAPI(rec))
	}

	writeJSON(w, http.StatusOK, resp)
}

// Get handles GET /v1/memory/get?id=...&layer=...
func (h *MemoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.grpcAvailable() {
		h.getViaGRPC(w, r)
		return
	}
	id := r.URL.Query().Get("id")
	layer := r.URL.Query().Get("layer")

	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := memory.ValidateMemoryID(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}
	if layer == "" {
		writeError(w, http.StatusBadRequest, "layer is required")
		return
	}

	ctx := r.Context()
	record, err := h.engine.Retrieve(ctx, id, memory.MemoryLayer(layer))
	if err != nil {
		writeError(w, http.StatusNotFound, "record not found: "+err.Error())
		return
	}
	if !ownsMemory(r, record.Owner, record.TenantID) {
		writeError(w, http.StatusNotFound, "record not found")
		return
	}

	resp := GetResponse{Record: recordToAPI(*record)}
	writeJSON(w, http.StatusOK, resp)
}

// Delete handles DELETE /v1/memory/delete?id=...&layer=...
// NOTE: internal/memory.MemoryEngine does not expose a public Delete method.
// This endpoint is a placeholder until the engine provides that capability.
func (h *MemoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.grpcAvailable() {
		h.deleteViaGRPC(w, r)
		return
	}
	id := r.URL.Query().Get("id")
	layer := r.URL.Query().Get("layer")

	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := memory.ValidateMemoryID(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	ctx := r.Context()

	layerKey := memory.MemoryLayer(layer)
	if layerKey == "" {
		layerKey = memory.LayerSession
	}

	// Check if record exists
	record, err := h.engine.Retrieve(ctx, id, layerKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "record not found: "+err.Error())
		return
	}
	if !ownsMemory(r, record.Owner, record.TenantID) {
		writeError(w, http.StatusNotFound, "record not found")
		return
	}

	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "memory engine not available")
		return
	}
	if err := h.engine.Delete(ctx, id, layerKey); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete record: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": id}); err != nil {
		log.Printf("memory delete: encode error: %v", err)
	}
}

// Promote handles POST /v1/memory/promote
func (h *MemoryHandler) Promote(w http.ResponseWriter, r *http.Request) {
	if h.grpcAvailable() {
		h.promoteViaGRPC(w, r)
		return
	}
	var req PromoteRequest
	limitBody(w, r, bodyLimitMedium)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.ID == "" || req.FromLayer == "" || req.ToLayer == "" {
		writeError(w, http.StatusBadRequest, "id, from_layer, and to_layer are required")
		return
	}

	ctx := r.Context()
	source, err := h.engine.Retrieve(ctx, req.ID, memory.MemoryLayer(req.FromLayer))
	if err != nil {
		writeError(w, http.StatusNotFound, "record not found")
		return
	}
	if !ownsMemory(r, source.Owner, source.TenantID) {
		writeError(w, http.StatusNotFound, "record not found")
		return
	}
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "memory engine not available")
		return
	}
	promoted, err := h.engine.Promote(ctx, req.ID,
		memory.MemoryLayer(req.FromLayer),
		memory.MemoryLayer(req.ToLayer))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "promote failed: "+err.Error())
		LogEvent(h.auditStore, r, "memory.promote", "memory:"+req.ID, audit.DetailsJSON(map[string]interface{}{"from": req.FromLayer, "to": req.ToLayer, "error": err.Error()}), "error")
		return
	}

	LogEvent(h.auditStore, r, "memory.promote", "memory:"+req.ID, audit.DetailsJSON(map[string]interface{}{"from": req.FromLayer, "to": req.ToLayer}), "success")

	resp := PromoteResponse{Record: recordToAPI(*promoted)}
	writeJSON(w, http.StatusOK, resp)
}

// Stats handles GET /v1/memory/stats
func (h *MemoryHandler) Stats(w http.ResponseWriter, r *http.Request) {
	if h.grpcAvailable() {
		h.statsViaGRPC(w, r)
		return
	}
	if h.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "memory engine not available")
		return
	}
	ctx := r.Context()
	var layerStats map[memory.MemoryLayer]memory.LayerStats
	if isAdmin(r) || claimsSubject(r) == "" {
		layerStats = h.engine.GetLayerStats(ctx)
	} else {
		layerStats = h.engine.GetLayerStatsScoped(ctx, claimsSubject(r), claimsTenant(r))
	}

	resp := MemoryStatsResponse{
		Layers: make(map[string]LayerStats),
	}

	for layer, stats := range layerStats {
		resp.Layers[string(layer)] = LayerStats{
			RecordCount: stats.Count,
			SizeBytes:   int64(stats.TotalSize),
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Helpers ---

func recordToAPI(rec memory.MemoryRecord) MemoryRecord {
	return MemoryRecord{
		ID:           rec.ID,
		Type:         string(rec.Type),
		Layer:        string(rec.Layer),
		Content:      rec.Content,
		ContentTrust: "untrusted_data",
		Priority:     rec.Priority,
		CreatedAt:    rec.CreatedAt.Format(time.RFC3339),
		Metadata:     rec.Metadata,
	}
}

// ─── gRPC delegation methods (Single Owner Model, Fase 3) ────────────────────

func (h *MemoryHandler) storeViaGRPC(w http.ResponseWriter, r *http.Request) {
	if !h.grpcAvailable() {
		writeError(w, http.StatusServiceUnavailable, "gRPC client not configured")
		return
	}
	var req StoreRequest
	limitBody(w, r, bodyLimitLarge)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}
	resp, err := h.grpcClient.Store(grpcAuthCtx(r), &cospb.StoreRequest{
		Type:     req.Type,
		Layer:    req.Layer,
		Scope:    req.Scope,
		Content:  req.Content,
		Priority: int32(req.Priority),
		Ttl:      req.TTL,
		Metadata: req.Metadata,
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "memory store via runtime daemon failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, StoreResponse{ID: resp.Id, CreatedAt: resp.CreatedAt})
}

func (h *MemoryHandler) searchViaGRPC(w http.ResponseWriter, r *http.Request) {
	if !h.grpcAvailable() {
		writeError(w, http.StatusServiceUnavailable, "gRPC client not configured")
		return
	}
	query := r.URL.Query()
	limit := int32(10)
	if l, err := strconv.Atoi(query.Get("limit")); err == nil && l > 0 {
		limit = int32(l)
	}
	types := query["type"]
	layers := query["layer"]
	resp, err := h.grpcClient.Search(grpcAuthCtx(r), &cospb.MemorySearchRequest{
		Query:  query.Get("query"),
		Types:  types,
		Layers: layers,
		Limit:  limit,
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "memory search via runtime daemon failed: "+err.Error())
		return
	}
	records := make([]MemoryRecord, len(resp.Records))
	for i, rec := range resp.Records {
		records[i] = MemoryRecord{
			ID:        rec.Id,
			Type:      rec.Type,
			Layer:     rec.Layer,
			Content:   rec.Content,
			Priority:  int(rec.Priority),
			CreatedAt: rec.CreatedAt,
			Metadata:  rec.Metadata,
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"records": records, "total": resp.Total,
	})
}

func (h *MemoryHandler) getViaGRPC(w http.ResponseWriter, r *http.Request) {
	if !h.grpcAvailable() {
		writeError(w, http.StatusServiceUnavailable, "gRPC client not configured")
		return
	}
	id := r.URL.Query().Get("id")
	layer := r.URL.Query().Get("layer")
	resp, err := h.grpcClient.Get(grpcAuthCtx(r), &cospb.GetRequest{Id: id, Layer: layer})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "memory get via runtime daemon failed: "+err.Error())
		return
	}
	if resp.Record == nil {
		writeError(w, http.StatusNotFound, "memory record not found")
		return
	}
	rec := resp.Record
	writeJSON(w, http.StatusOK, MemoryRecord{
		ID:        rec.Id,
		Type:      rec.Type,
		Layer:     rec.Layer,
		Content:   rec.Content,
		Priority:  int(rec.Priority),
		CreatedAt: rec.CreatedAt,
		Metadata:  rec.Metadata,
	})
}

func (h *MemoryHandler) deleteViaGRPC(w http.ResponseWriter, r *http.Request) {
	if !h.grpcAvailable() {
		writeError(w, http.StatusServiceUnavailable, "gRPC client not configured")
		return
	}
	id := r.URL.Query().Get("id")
	layer := r.URL.Query().Get("layer")
	_, err := h.grpcClient.Delete(grpcAuthCtx(r), &cospb.DeleteRequest{Id: id, Layer: layer})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "memory delete via runtime daemon failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *MemoryHandler) promoteViaGRPC(w http.ResponseWriter, r *http.Request) {
	if !h.grpcAvailable() {
		writeError(w, http.StatusServiceUnavailable, "gRPC client not configured")
		return
	}
	var req PromoteRequest
	limitBody(w, r, bodyLimitMedium)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}
	resp, err := h.grpcClient.Promote(grpcAuthCtx(r), &cospb.PromoteRequest{
		Id:        req.ID,
		FromLayer: req.FromLayer,
		ToLayer:   req.ToLayer,
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "memory promote via runtime daemon failed: "+err.Error())
		return
	}
	if resp.Record == nil {
		writeError(w, http.StatusInternalServerError, "promotion returned no record")
		return
	}
	rec := resp.Record
	writeJSON(w, http.StatusOK, MemoryRecord{
		ID:        rec.Id,
		Type:      rec.Type,
		Layer:     rec.Layer,
		Content:   rec.Content,
		Priority:  int(rec.Priority),
		CreatedAt: rec.CreatedAt,
		Metadata:  rec.Metadata,
	})
}

func (h *MemoryHandler) statsViaGRPC(w http.ResponseWriter, _ *http.Request) {
	if !h.grpcAvailable() {
		writeError(w, http.StatusServiceUnavailable, "gRPC client not configured")
		return
	}
	resp, err := h.grpcClient.Stats(context.Background(), &cospb.MemoryStatsRequest{})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "memory stats via runtime daemon failed: "+err.Error())
		return
	}
	layers := make(map[string]interface{})
	for name, ls := range resp.Layers {
		layers[name] = map[string]interface{}{
			"record_count": ls.RecordCount,
			"size_bytes":   ls.SizeBytes,
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"layers": layers})
}
