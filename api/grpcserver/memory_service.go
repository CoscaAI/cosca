package grpcserver

import (
	"context"
	"time"

	apiauth "github.com/CoscaAI/cosca/api/auth"
	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/internal/memory"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// grpcAuthFromCtx extracts the authenticated caller's identity propagated by
// the REST handler via outgoing metadata (handler.grpcAuthCtx). Callers that
// skip the metadata (e.g. direct gRPC clients) are treated as anonymous: empty
// subject/tenant and no admin privileges — they only see global records.
func grpcAuthFromCtx(ctx context.Context) (sub, tenant, role string) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", "", ""
	}
	if v := md.Get("x-cosca-sub"); len(v) > 0 {
		sub = v[0]
	}
	if v := md.Get("x-cosca-tenant"); len(v) > 0 {
		tenant = v[0]
	}
	if v := md.Get("x-cosca-role"); len(v) > 0 {
		role = v[0]
	}
	return sub, tenant, role
}

// grpcCanAccess reports whether a caller may access a record owned by
// owner/tenant. Mirror of the local handler.ownsMemory semantics: admins can
// access everything; global records (empty owner) are readable by anyone;
// otherwise subject and tenant must match.
func grpcCanAccess(sub, tenant, role, owner, recTenant string) bool {
	if role == string(apiauth.RoleAdmin) {
		return true
	}
	if owner != "" && owner != sub {
		return false
	}
	return recTenant == "" || recTenant == tenant
}

// MemoryServiceServer implements the cosca.v1.MemoryServiceServer interface
// generated from proto/cosca/v1/memory.proto. It delegates all RPCs to the
// underlying memory.MemoryEngine.
type MemoryServiceServer struct {
	cospb.UnimplementedMemoryServiceServer
	engine *memory.MemoryEngine
}

// NewMemoryServiceServer creates a new MemoryServiceServer backed by the
// given memory engine. engine may be nil — calls to RPCs will return
// codes.FailedPrecondition in that case.
func NewMemoryServiceServer(engine *memory.MemoryEngine) *MemoryServiceServer {
	return &MemoryServiceServer{engine: engine}
}

// Store persists a new memory record. It validates that content is non-empty,
// converts the protobuf request into a domain MemoryRecord (parsing TTL from
// the string field), and delegates to the engine. Ownership is ALWAYS taken
// from the authenticated caller's metadata — never from the client payload —
// so a user cannot store records owned by someone else.
func (s *MemoryServiceServer) Store(ctx context.Context, req *cospb.StoreRequest) (*cospb.StoreResponse, error) {
	if s.engine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "memory engine not initialized")
	}
	if req.GetContent() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "content is required")
	}

	sub, tenant, _ := grpcAuthFromCtx(ctx)

	record := pbToMemoryRecord(req)
	record.Owner = sub
	record.TenantID = tenant

	saved, err := s.engine.Store(ctx, record)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "store failed: %v", err)
	}

	return &cospb.StoreResponse{
		Id:        saved.ID,
		CreatedAt: saved.CreatedAt.Format(time.RFC3339),
	}, nil
}

// Search queries memory records across layers using the provided search
// criteria (query string, type filters, layer filters, limit). Results are
// scoped to the authenticated caller's records plus global records, unless
// the caller is an admin.
func (s *MemoryServiceServer) Search(ctx context.Context, req *cospb.MemorySearchRequest) (*cospb.MemorySearchResponse, error) {
	if s.engine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "memory engine not initialized")
	}

	sub, tenant, role := grpcAuthFromCtx(ctx)

	opts := pbToSearchOptions(req)
	if role != string(apiauth.RoleAdmin) {
		opts.OwnerFilter = sub
		opts.TenantFilter = tenant
	}

	records, err := s.engine.Search(ctx, req.GetQuery(), opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	return &cospb.MemorySearchResponse{
		Records: memoryRecordsToPb(records),
		Total:   int32(len(records)),
	}, nil
}

// Get retrieves a single memory record by ID and layer. Returns
// codes.NotFound if the record does not exist or has expired, and
// codes.PermissionDenied if the caller does not own it.
func (s *MemoryServiceServer) Get(ctx context.Context, req *cospb.GetRequest) (*cospb.GetResponse, error) {
	if s.engine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "memory engine not initialized")
	}
	if req.GetId() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "id is required")
	}

	sub, tenant, role := grpcAuthFromCtx(ctx)

	record, err := s.engine.Retrieve(ctx, req.GetId(), memory.MemoryLayer(req.GetLayer()))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "record not found: %v", err)
	}

	if !grpcCanAccess(sub, tenant, role, record.Owner, record.TenantID) {
		return nil, status.Errorf(codes.PermissionDenied, "record does not belong to caller")
	}

	return &cospb.GetResponse{
		Record: memoryRecordToPb(*record),
	}, nil
}

// Delete removes a memory record. It first checks for existence via Retrieve
// to ensure the caller gets codes.NotFound when the record is absent (the
// engine's Delete implementation returns nil for non-existent records), and
// enforces ownership before deleting.
func (s *MemoryServiceServer) Delete(ctx context.Context, req *cospb.DeleteRequest) (*cospb.DeleteResponse, error) {
	if s.engine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "memory engine not initialized")
	}
	if req.GetId() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "id is required")
	}

	sub, tenant, role := grpcAuthFromCtx(ctx)
	layer := memory.MemoryLayer(req.GetLayer())

	// Check existence before deletion — FileStore.Delete returns nil
	// for non-existent files, so we need an explicit existence check
	// to produce the correct NotFound status.
	record, err := s.engine.Retrieve(ctx, req.GetId(), layer)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "record not found: %v", err)
	}

	if !grpcCanAccess(sub, tenant, role, record.Owner, record.TenantID) {
		return nil, status.Errorf(codes.PermissionDenied, "record does not belong to caller")
	}

	if err := s.engine.Delete(ctx, req.GetId(), layer); err != nil {
		return nil, status.Errorf(codes.Internal, "delete failed: %v", err)
	}

	return &cospb.DeleteResponse{Success: true}, nil
}

// Promote moves a memory record from one layer to a higher-persistence layer
// (e.g. session → project → global). All three fields (id, from_layer, to_layer)
// are required, and the caller must own the record.
func (s *MemoryServiceServer) Promote(ctx context.Context, req *cospb.PromoteRequest) (*cospb.PromoteResponse, error) {
	if s.engine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "memory engine not initialized")
	}
	if req.GetId() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "id is required")
	}
	if req.GetFromLayer() == "" || req.GetToLayer() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "from_layer and to_layer are required")
	}

	sub, tenant, role := grpcAuthFromCtx(ctx)

	record, err := s.engine.Retrieve(ctx, req.GetId(), memory.MemoryLayer(req.GetFromLayer()))
	if err == nil && !grpcCanAccess(sub, tenant, role, record.Owner, record.TenantID) {
		return nil, status.Errorf(codes.PermissionDenied, "record does not belong to caller")
	}

	promoted, err := s.engine.Promote(ctx, req.GetId(),
		memory.MemoryLayer(req.GetFromLayer()),
		memory.MemoryLayer(req.GetToLayer()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "promote failed: %v", err)
	}

	return &cospb.PromoteResponse{
		Record: memoryRecordToPb(*promoted),
	}, nil
}

// Stats returns per-layer statistics (record count, total size in bytes) for
// all configured memory layers.
func (s *MemoryServiceServer) Stats(ctx context.Context, _ *cospb.MemoryStatsRequest) (*cospb.MemoryStatsResponse, error) {
	if s.engine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "memory engine not initialized")
	}

	stats := s.engine.GetLayerStats(ctx)

	return &cospb.MemoryStatsResponse{
		Layers: layerStatsToPb(stats),
	}, nil
}
