package grpcserver

import (
	"context"
	"reflect"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/search"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// knowledgeEngine is the minimal surface the KnowledgeServiceServer needs from
// the knowledge engine. It is an interface (rather than the concrete
// *knowledge.Engine) so tests can inject a spy that counts Search calls —
// enabling the FASE 3.5 proof that a NoRoute query never reaches the engine
// (never an un-scoped full-scan). The concrete *knowledge.Engine satisfies it.
type knowledgeEngine interface {
	Search(ctx context.Context, params search.SearchParams) (*search.SearchResults, error)
	IndexDocument(ctx context.Context, path string) error
	IndexDirectory(ctx context.Context, dir string) error
	GetStats() (*knowledge.Stats, error)
	Sync(ctx context.Context) (*knowledge.SyncResult, error)
	RouteCandidateIDs(scope *modlink.SearchScope) ([]string, error)
}

// KnowledgeServiceServer implements the cosca.v1.KnowledgeServiceServer interface
// generated from proto/cosca/v1/knowledge.proto. It delegates all RPCs to the
// underlying knowledge engine.
//
// FASE 3.5 (routing/scope): in MODULAR mode the server applies the SAME
// deterministic router + scope as the local path (single source:
// modlink.DefaultRoutes() + search.ApplyScope). A query with no known route
// (NoRoute) returns an empty result WITHOUT calling engine.Search — never an
// un-scoped full-scan.
type KnowledgeServiceServer struct {
	cospb.UnimplementedKnowledgeServiceServer
	engine   knowledgeEngine
	resolver *modlink.Resolver // nil = no routing (legacy)
	mode     string            // search.ModeLegacy (default) | search.ModeModular
}

// NewKnowledgeServiceServer creates a new KnowledgeServiceServer backed by the
// given knowledge engine. engine may be nil — calls to RPCs will return
// codes.FailedPrecondition in that case. Default mode is legacy (no routing).
func NewKnowledgeServiceServer(engine knowledgeEngine) *KnowledgeServiceServer {
	// Normalize a typed-nil (e.g. a nil *knowledge.Engine wrapped in the
	// interface) to a real nil so the public "nil engine" contract holds:
	// the engine field must compare == nil.
	return &KnowledgeServiceServer{engine: normalizeEngine(engine), mode: search.ModeLegacy}
}

// normalizeEngine collapses a typed-nil inside a knowledgeEngine interface to a
// real nil interface value. Without this, a nil *knowledge.Engine wrapped in an
// interface is non-nil and the nil-guards would not fire.
func normalizeEngine(engine knowledgeEngine) knowledgeEngine {
	if engine == nil {
		return nil
	}
	rv := reflect.ValueOf(engine)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return nil
	}
	return engine
}

// WithRouting configures the deterministic router (modlink) and the search mode
// for this server. To honor the invariant (one router, no second mechanism),
// the resolver MUST come from modlink.NewResolver(modlink.DefaultRoutes()) — the
// same single source as the local path. Empty mode defaults to legacy.
// Returns the server for chaining.
func (s *KnowledgeServiceServer) WithRouting(resolver *modlink.Resolver, mode string) *KnowledgeServiceServer {
	s.resolver = resolver
	if mode == "" {
		mode = search.ModeLegacy
	}
	s.mode = mode
	return s
}

// Search performs a hybrid search across all knowledge indexes (FTS, vector,
// graph). Returns codes.InvalidArgument if the query is empty, and
// codes.FailedPrecondition if the engine is not initialized.
func (s *KnowledgeServiceServer) Search(ctx context.Context, req *cospb.SearchRequest) (*cospb.SearchResponse, error) {
	if s.engine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "knowledge engine not initialized")
	}
	if req.GetQuery() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "search query is required")
	}

	params := pbToSearchParams(req)

	// FASE 3.5 routing/scope: in MODULAR mode the daemon reuses the SAME router
	// (modlink.DefaultRoutes()) + search.ApplyScope as the local path. A NoRoute
	// query returns empty WITHOUT calling engine.Search — never an un-scoped
	// full-scan (the essential invariant). Legacy mode is unchanged.
	if s.mode == search.ModeModular && s.resolver != nil {
		scoped, scope := search.ApplyScope(s.resolver, req.GetQuery(), params)
		if scope.NoRoute || len(scope.Modules) == 0 {
			return searchResultsToPb(&search.SearchResults{Query: req.GetQuery()}), nil
		}
		params = scoped
		// FASE B (ADR-013 §3.2): confinar a fase vetorial aos candidatos
		// permitidos do escopo roteado (nunca full-scan do índice).
		if cands, cErr := s.engine.RouteCandidateIDs(scope); cErr == nil && len(cands) > 0 {
			params.CandidateIDs = cands
		}
	}

	results, err := s.engine.Search(ctx, params)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	return searchResultsToPb(results), nil
}

// Index indexes a single document or directory depending on the recursive flag.
// When recursive is true, it indexes an entire directory tree. Returns
// codes.InvalidArgument if the path is empty, and codes.FailedPrecondition
// if the engine is not initialized.
func (s *KnowledgeServiceServer) Index(ctx context.Context, req *cospb.IndexRequest) (*cospb.IndexResponse, error) {
	if s.engine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "knowledge engine not initialized")
	}
	if req.GetPath() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "index path is required")
	}

	var indexErr error
	documentsIndexed := int32(0)

	if req.GetRecursive() {
		indexErr = s.engine.IndexDirectory(ctx, req.GetPath())
	} else {
		indexErr = s.engine.IndexDocument(ctx, req.GetPath())
		if indexErr == nil {
			documentsIndexed = 1
		}
	}

	resp := &cospb.IndexResponse{
		DocumentsIndexed: documentsIndexed,
		ChunksIndexed:    0,
		Errors:           nil,
	}

	if indexErr != nil {
		resp.Errors = []string{indexErr.Error()}
	}

	return resp, nil
}

// Stats returns comprehensive statistics about the knowledge engine, including
// document/chunk/entity/vector counts, database size, and uptime. Returns
// codes.FailedPrecondition if the engine is not initialized.
func (s *KnowledgeServiceServer) Stats(ctx context.Context, req *cospb.StatsRequest) (*cospb.StatsResponse, error) {
	if s.engine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "knowledge engine not initialized")
	}

	stats, err := s.engine.GetStats()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get stats: %v", err)
	}

	return statsToPb(stats), nil
}

// Sync synchronizes the knowledge index with the filesystem, detecting added,
// modified, and removed files. Returns counts of each category along with any
// errors encountered during processing. Returns codes.FailedPrecondition if
// the engine is not initialized.
func (s *KnowledgeServiceServer) Sync(ctx context.Context, req *cospb.SyncRequest) (*cospb.SyncResponse, error) {
	if s.engine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "knowledge engine not initialized")
	}

	result, err := s.engine.Sync(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sync failed: %v", err)
	}

	return syncResultToPb(result), nil
}
