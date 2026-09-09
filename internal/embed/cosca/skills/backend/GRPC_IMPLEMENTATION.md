# GRPC IMPLEMENTATION — Enterprise Grade

> **Version**: 1.0.0 | **Status**: active | **Owner**: Backend Chief | **Last Updated**: 2026-07-27

## Description
Implement production-grade gRPC servers and clients in Go for Cosca. Covers proto generation, server setup with interceptors, client with retry/backoff, streaming, health checks, and testing. Project-specific: proto definitions at `proto/cosca/v1/`.

## Prerequisites
- Proto files exist at `proto/cosca/v1/` (knowledge.proto, memory.proto, runtime.proto)
- protoc + protoc-gen-go + protoc-gen-go-grpc installed
- Go package path: `github.com/CoscaAI/cosca/api/grpc/pb`

## Implementation Steps

### Step 1 — Generate Go Code from Proto
```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       proto/cosca/v1/*.proto
```
Output: `api/grpc/pb/*.pb.go` + `api/grpc/pb/*_grpc.pb.go`

### Step 2 — Server Implementation
```go
// api/grpc/server.go
package grpc

import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    "google.golang.org/grpc/health/grpc_health_v1"
    pb "github.com/CoscaAI/cosca/api/grpc/pb"
)

type Server struct {
    pb.UnimplementedKnowledgeServiceServer
    pb.UnimplementedMemoryServiceServer
    pb.UnimplementedRuntimeServiceServer
    grpcServer *grpc.Server
    knowledge  KnowledgeService  // inject interface
    memory     MemoryService
    runtime    RuntimeService
    health     *health.Server
}

func NewServer(k KnowledgeService, m MemoryService, r RuntimeService) *Server {
    s := &Server{
        knowledge: k,
        memory:    m,
        runtime:   r,
        health:    health.NewServer(),
    }

    s.grpcServer = grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            loggingInterceptor,
            authInterceptor,
            recoveryInterceptor,
        ),
        grpc.MaxRecvMsgSize(10*1024*1024), // 10MB
    )

    pb.RegisterKnowledgeServiceServer(s.grpcServer, s)
    pb.RegisterMemoryServiceServer(s.grpcServer, s)
    pb.RegisterRuntimeServiceServer(s.grpcServer, s)
    grpc_health_v1.RegisterHealthServer(s.grpcServer, s.health)

    return s
}

func (s *Server) Serve(addr string) error {
    lis, err := net.Listen("tcp", addr)
    if err != nil {
        return fmt.Errorf("failed to listen on %s: %w", addr, err)
    }
    s.health.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
    return s.grpcServer.Serve(lis)
}

func (s *Server) GracefulStop() {
    s.health.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
    s.grpcServer.GracefulStop()
}
```

### Step 3 — Service Implementation (Example: KnowledgeService)
```go
func (s *Server) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
    if req.Query == "" {
        return nil, status.Error(codes.InvalidArgument, "query is required")
    }
    if req.Limit <= 0 || req.Limit > 100 {
        req.Limit = 20
    }

    results, err := s.knowledge.Search(ctx, SearchParams{
        Query:    req.Query,
        Limit:    int(req.Limit),
        Types:    req.Types,
        MinScore: req.MinScore,
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "search failed: %v", err)
    }

    pbResults := make([]*pb.SearchResult, len(results.Items))
    for i, r := range results.Items {
        pbResults[i] = &pb.SearchResult{
            Id:    r.ID,
            Title: r.Title,
            Score: r.Score,
            Type:  r.Type,
            Path:  r.Path,
        }
    }

    return &pb.SearchResponse{
        Results:    pbResults,
        Total:      int32(results.Total),
        DurationMs: results.DurationMs,
    }, nil
}
```

### Step 4 — Interceptors
```go
// Logging interceptor
func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    start := time.Now()
    resp, err := handler(ctx, req)
    log.Info().
        Str("method", info.FullMethod).
        Dur("duration", time.Since(start)).
        Err(err).
        Msg("gRPC request")
    return resp, err
}

// Auth interceptor
func authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    // Skip auth for health checks
    if info.FullMethod == "/grpc.health.v1.Health/Check" {
        return handler(ctx, req)
    }
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return nil, status.Error(codes.Unauthenticated, "missing metadata")
    }
    token := md.Get("authorization")
    if len(token) == 0 {
        return nil, status.Error(codes.Unauthenticated, "missing authorization token")
    }
    // Validate JWT token (reuse api/auth/jwt.go)
    claims, err := ValidateJWT(strings.TrimPrefix(token[0], "Bearer "))
    if err != nil {
        return nil, status.Error(codes.Unauthenticated, "invalid token")
    }
    ctx = context.WithValue(ctx, "user", claims)
    return handler(ctx, req)
}

// Recovery interceptor (prevent panics from crashing server)
func recoveryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
    defer func() {
        if r := recover(); r != nil {
            log.Error().Str("method", info.FullMethod).Interface("panic", r).Msg("gRPC panic recovered")
            err = status.Error(codes.Internal, "internal server error")
        }
    }()
    return handler(ctx, req)
}
```

### Step 5 — Client with Retry
```go
// pkg/cosca/grpc_client.go
type Client struct {
    conn   *grpc.ClientConn
    knowledge pb.KnowledgeServiceClient
    memory    pb.MemoryServiceClient
    runtime   pb.RuntimeServiceClient
}

func NewClient(addr string, token string) (*Client, error) {
    conn, err := grpc.Dial(addr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithUnaryInterceptor(clientRetryInterceptor()),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to connect: %w", err)
    }
    return &Client{
        conn:      conn,
        knowledge: pb.NewKnowledgeServiceClient(conn),
        memory:    pb.NewMemoryServiceClient(conn),
        runtime:   pb.NewRuntimeServiceClient(conn),
    }, nil
}

// Retry with exponential backoff
func clientRetryInterceptor() grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        var lastErr error
        for attempt := 0; attempt < 3; attempt++ {
            lastErr = invoker(ctx, method, req, reply, cc, opts...)
            if lastErr == nil {
                return nil
            }
            if !isRetryable(lastErr) {
                return lastErr
            }
            time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond)
        }
        return lastErr
    }
}

func isRetryable(err error) bool {
    s, ok := status.FromError(err)
    return ok && (s.Code() == codes.Unavailable || s.Code() == codes.DeadlineExceeded)
}
```

### Step 6 — Testing
```go
func TestKnowledgeService_Search(t *testing.T) {
    // Use bufconn for in-memory gRPC testing (no network)
    lis := bufconn.Listen(1024 * 1024)
    s := grpc.NewServer()
    pb.RegisterKnowledgeServiceServer(s, &Server{
        knowledge: &mockKnowledgeService{},
    })
    go s.Serve(lis)
    defer s.Stop()

    conn, _ := grpc.Dial("bufnet",
        grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
            return lis.Dial()
        }),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    defer conn.Close()

    client := pb.NewKnowledgeServiceClient(conn)
    resp, err := client.Search(context.Background(), &pb.SearchRequest{
        Query: "test",
        Limit: 10,
    })

    require.NoError(t, err)
    assert.NotNil(t, resp)
}
```

## Server Startup (integrate with cmd/cosca/main.go)
```go
// Add gRPC server alongside REST server
grpcServer := grpc.NewServer(knowledgeSvc, memorySvc, runtimeSvc)
go func() {
    log.Info().Msg("gRPC server listening on :14121")
    if err := grpcServer.Serve(":14121"); err != nil {
        log.Fatal().Err(err).Msg("gRPC server failed")
    }
}()
```

## Quality Gates
- [ ] Proto files generate without errors
- [ ] All 3 services implemented (Knowledge, Memory, Runtime)
- [ ] Auth interceptor validates JWT
- [ ] Recovery interceptor prevents panics
- [ ] Health check endpoint working
- [ ] Client retry with exponential backoff
- [ ] Tests use bufconn (no network dependency)
- [ ] Streaming support for Search (server-side stream)
- [ ] Error codes mapped: InvalidArgument(400), NotFound(404), Internal(500), Unauthenticated(401)
- [ ] Graceful shutdown with health status transition

## References
- Proto definitions: `proto/cosca/v1/knowledge.proto`, `memory.proto`, `runtime.proto`
- REST API (for parity): `api/rest/handler/`
- JWT validation: `api/auth/jwt.go`
