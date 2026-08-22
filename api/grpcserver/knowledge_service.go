package grpcserver

import (
	"context"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/internal/knowledge"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// KnowledgeServiceServer implements the cosca.v1.KnowledgeServiceServer interface
// generated from proto/cosca/v1/knowledge.proto. It delegates all RPCs to the
// underlying knowledge.Engine.
type KnowledgeServiceServer struct {
	cospb.UnimplementedKnowledgeServiceServer
	engine *knowledge.Engine
}

// NewKnowledgeServiceServer creates a new KnowledgeServiceServer backed by the
// given knowledge engine. engine may be nil — calls to RPCs will return
// codes.FailedPrecondition in that case.
func NewKnowledgeServiceServer(engine *knowledge.Engine) *KnowledgeServiceServer {
	return &KnowledgeServiceServer{engine: engine}
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
