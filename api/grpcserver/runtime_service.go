package grpcserver

import (
	"context"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/internal/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RuntimeServiceServer implements the cosca.v1.RuntimeServiceServer interface
// generated from proto/cosca/v1/runtime.proto. It delegates all RPCs to the
// underlying runtime.Runtime engine.
type RuntimeServiceServer struct {
	cospb.UnimplementedRuntimeServiceServer
	engine *runtime.Runtime
}

// NewRuntimeServiceServer creates a new RuntimeServiceServer backed by the
// given runtime engine. engine may be nil — calls to RPCs will return
// codes.FailedPrecondition in that case.
func NewRuntimeServiceServer(engine *runtime.Runtime) *RuntimeServiceServer {
	return &RuntimeServiceServer{engine: engine}
}

// Status returns the current runtime status including state, health, uptime,
// version, and component details. Returns codes.FailedPrecondition if the
// runtime engine was not initialized.
func (s *RuntimeServiceServer) Status(ctx context.Context, req *cospb.StatusRequest) (*cospb.StatusResponse, error) {
	if s.engine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "runtime engine not initialized")
	}
	return runtimeStatusToPb(s.engine), nil
}

// Health returns a simple health check indicating whether the runtime is
// healthy and any warnings from degraded components. Returns
// codes.FailedPrecondition if the runtime engine was not initialized.
func (s *RuntimeServiceServer) Health(ctx context.Context, req *cospb.HealthRequest) (*cospb.HealthResponse, error) {
	if s.engine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "runtime engine not initialized")
	}
	return runtimeHealthToPb(s.engine), nil
}
