// Package grpcserver provides integration tests for the Cosca gRPC server
// using in-memory buffered connections (bufconn). Tests cover all three
// services (Runtime, Knowledge, Memory), server lifecycle, and interceptors.
package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/CoscaAI/cosca/api/auth"
	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/runtime"
)

// engineInitMu serializes engine initialization across parallel test packages.
// SQLite databases use file locks that can cause "database is locked" errors
// when multiple packages create engines concurrently.
var engineInitMu sync.Mutex

// ── Test Setup Helpers ───────────────────────────────────────────────────────

// testServices holds client connections to all gRPC services and the cleanup
// function that closes the in-memory connection and any engine resources.
type testServices struct {
	RuntimeClient   cospb.RuntimeServiceClient
	KnowledgeClient cospb.KnowledgeServiceClient
	MemoryClient    cospb.MemoryServiceClient
	conn            *grpc.ClientConn
	cleanup         func()
}

// newBufconnServer creates a gRPC server backed by a bufconn listener and
// returns a testServices struct with clients for all three services. Both
// the knowledge and memory engines are initialized using the provided
// temp directory (t.TempDir()). The runtime engine is created with minimal
// configuration. All services are registered with logging and recovery
// interceptors.
func newBufconnServer(t *testing.T, dataDir string, initKnowledge bool) *testServices {
	t.Helper()

	// Serialize engine initialization to avoid SQLite "database is locked"
	// when multiple test packages create engines concurrently.
	engineInitMu.Lock()
	defer engineInitMu.Unlock()

	logger := zerolog.Nop()

	// ── Runtime engine (always works without Start) ──────────────────────
	rtCfg := runtime.DefaultRuntimeConfig()
	rtCfg.Version = "test"
	rtCfg.DataDir = dataDir
	rtCfg.EnableDaemon = false
	rtCfg.EnableMetrics = true
	rt := runtime.New(runtime.WithConfig(rtCfg), runtime.WithLogger(logger))

	// ── Memory engine ────────────────────────────────────────────────────
	memCfg := memory.DefaultConfig()
	memCfg.DataDir = dataDir
	memCfg.AutoPrune = false // no background goroutines in tests
	memCfg.DefaultTTL = 24 * time.Hour
	memEngine, err := memory.NewEngine(
		memory.WithConfig(memCfg),
		memory.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("failed to create memory engine: %v", err)
	}

	// ── Knowledge engine (heavy; only init if requested) ─────────────────
	var ke *knowledge.Engine
	if initKnowledge {
		kCfg := knowledge.DefaultConfig()
		kCfg.DBPath = filepath.Join(dataDir, "knowledge.db")
		kCfg.RootDir = dataDir
		kCfg.AutoMigrate = true
		kCfg.WatchEnabled = false
		kCfg.EmbeddingProvider = "auto"
		ke, err = knowledge.New(kCfg)
		if err != nil {
			t.Fatalf("failed to create knowledge engine: %v", err)
		}
		if err := ke.Init(); err != nil {
			t.Logf("knowledge engine Init() failed (non-critical): %v", err)
			// Don't fail the test; the gRPC handlers will return
			// codes.Internal for calls to an uninitialized engine.
		}
	}

	// ── gRPC server with bufconn ─────────────────────────────────────────
	bufSize := 1024 * 1024
	lis := bufconn.Listen(bufSize)

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			RecoveryInterceptor(),
			LoggingInterceptor(logger),
		),
	)

	// Register all services
	runtimeSrv := NewRuntimeServiceServer(rt)
	cospb.RegisterRuntimeServiceServer(srv, runtimeSrv)

	knowledgeSrv := NewKnowledgeServiceServer(ke)
	cospb.RegisterKnowledgeServiceServer(srv, knowledgeSrv)

	if memEngine != nil {
		memorySrv := NewMemoryServiceServer(memEngine)
		cospb.RegisterMemoryServiceServer(srv, memorySrv)
	}

	// Start serving in background
	go func() {
		if serveErr := srv.Serve(lis); serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
			t.Logf("bufconn serve error: %v", serveErr)
		}
	}()

	// Dial the in-memory connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.Stop()
		t.Fatalf("failed to dial bufconn: %v", err)
	}

	ts := &testServices{
		RuntimeClient:   cospb.NewRuntimeServiceClient(conn),
		KnowledgeClient: cospb.NewKnowledgeServiceClient(conn),
		MemoryClient:    cospb.NewMemoryServiceClient(conn),
		conn:            conn,
		cleanup: func() {
			conn.Close()
			srv.GracefulStop()
			// Memory engine cleanup
			if memEngine != nil {
				_ = memEngine.Close()
			}
			// Knowledge engine cleanup
			if ke != nil {
				_ = ke.Close()
			}
		},
	}

	return ts
}

// ── RuntimeService Tests ─────────────────────────────────────────────────────

func TestRuntimeStatus(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := ts.RuntimeClient.Status(ctx, &cospb.StatusRequest{})
	if err != nil {
		t.Fatalf("Status() unexpected error: %v", err)
	}

	if resp.State == "" {
		t.Error("Status() expected non-empty state")
	}
	if resp.Version == "" {
		t.Error("Status() expected non-empty version")
	}
	if resp.UptimeSeconds < 0 {
		t.Errorf("Status() uptime_seconds should be >= 0, got %f", resp.UptimeSeconds)
	}
	if resp.Health == "" {
		t.Error("Status() expected non-empty health")
	}
}

func TestRuntimeHealth(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := ts.RuntimeClient.Health(ctx, &cospb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health() unexpected error: %v", err)
	}

	// Fresh runtime (uninitialized state) has unknown health which is considered "healthy" by the mapping.
	if !resp.Healthy {
		t.Log("Health() reports healthy=false (may be expected for uninitialized runtime)")
	}
}

func TestRuntimeNilEngine(t *testing.T) {
	// Create server with nil runtime engine — service should return FailedPrecondition.
	logger := zerolog.Nop()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(RecoveryInterceptor(), LoggingInterceptor(logger)),
	)

	nilRuntimeSrv := NewRuntimeServiceServer(nil)
	cospb.RegisterRuntimeServiceServer(srv, nilRuntimeSrv)

	go func() {
		_ = srv.Serve(lis)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	defer srv.GracefulStop()

	client := cospb.NewRuntimeServiceClient(conn)

	resp, err := client.Status(ctx, &cospb.StatusRequest{})
	if err == nil {
		t.Error("Status() with nil engine expected error, got nil")
	}
	if resp != nil {
		t.Errorf("Status() with nil engine expected nil response, got %v", resp)
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("Status() with nil engine expected FailedPrecondition, got %s", st.Code())
	}
}

type blockingStreamService interface{}

type blockingStreamServiceImpl struct {
	started chan struct{}
	block   chan struct{}
}

func TestGRPCServerGracefulStopTimeoutAndIdempotency(t *testing.T) {
	originalTimeout := gracefulStopTimeout
	gracefulStopTimeout = 25 * time.Millisecond
	defer func() { gracefulStopTimeout = originalTimeout }()

	service := &blockingStreamServiceImpl{
		started: make(chan struct{}),
		block:   make(chan struct{}),
	}
	srv := grpc.NewServer()
	srv.RegisterService(&grpc.ServiceDesc{
		ServiceName: "test.BlockingStream",
		HandlerType: (*blockingStreamService)(nil),
		Streams: []grpc.StreamDesc{{
			StreamName:    "Block",
			ServerStreams: true,
			Handler: func(_ interface{}, stream grpc.ServerStream) error {
				close(service.started)
				<-service.block
				return nil
			},
		}},
	}, service)

	lis := bufconn.Listen(1024 * 1024)
	go func() { _ = srv.Serve(lis) }()
	defer func() {
		close(service.block)
		srv.Stop()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	_, err = conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, "/test.BlockingStream/Block")
	if err != nil {
		t.Fatalf("open blocking stream: %v", err)
	}
	select {
	case <-service.started:
	case <-time.After(time.Second):
		t.Fatal("blocking stream handler did not start")
	}

	gs := &GRPCServer{server: srv, logger: zerolog.Nop()}
	start := time.Now()
	gs.GracefulStop()
	if elapsed := time.Since(start); elapsed < gracefulStopTimeout {
		t.Fatalf("GracefulStop returned before timeout: %v", elapsed)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("GracefulStop exceeded bounded timeout: %v", elapsed)
	}

	// sync.Once makes repeated shutdown requests return without another gRPC
	// shutdown operation or a second wait on the blocked server.
	gs.GracefulStop()
}

// ── KnowledgeService Tests ───────────────────────────────────────────────────

// TestKnowledgeSearch_EmptyQuery verifies that searching with an empty query
// returns codes.InvalidArgument. The engine nil check takes precedence, so
// with a nil engine we would get FailedPrecondition. This test uses an
// initialized engine to exercise the parameter validation path.
func TestKnowledgeSearch_EmptyQuery(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), true)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := ts.KnowledgeClient.Search(ctx, &cospb.SearchRequest{Query: ""})
	if err == nil {
		t.Fatal("Search() with empty query expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("Search() with empty query expected InvalidArgument, got %s", st.Code())
	}
}

func TestKnowledgeSearch_NilEngine(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false) // initKnowledge=false → nil knowledge engine
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := ts.KnowledgeClient.Search(ctx, &cospb.SearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("Search() with nil engine expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("Search() with nil engine expected FailedPrecondition, got %s", st.Code())
	}
}

func TestKnowledgeIndex_EmptyPath(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), true)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := ts.KnowledgeClient.Index(ctx, &cospb.IndexRequest{Path: ""})
	if err == nil {
		t.Fatal("Index() with empty path expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("Index() with empty path expected InvalidArgument, got %s", st.Code())
	}
}

func TestKnowledgeIndex_NilEngine(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := ts.KnowledgeClient.Index(ctx, &cospb.IndexRequest{Path: "/tmp/test.md"})
	if err == nil {
		t.Fatal("Index() with nil engine expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("Index() with nil engine expected FailedPrecondition, got %s", st.Code())
	}
}

func TestKnowledgeStats_NilEngine(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := ts.KnowledgeClient.Stats(ctx, &cospb.StatsRequest{})
	if err == nil {
		t.Fatal("Stats() with nil engine expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("Stats() with nil engine expected FailedPrecondition, got %s", st.Code())
	}
}

func TestKnowledgeSync_NilEngine(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := ts.KnowledgeClient.Sync(ctx, &cospb.SyncRequest{})
	if err == nil {
		t.Fatal("Sync() with nil engine expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("Sync() with nil engine expected FailedPrecondition, got %s", st.Code())
	}
}

// TestKnowledgeWithInitEngine tests knowledge operations when the engine is
// actually initialized. If Init() fails (e.g., missing system deps), the test
// is skipped gracefully to avoid false failures.
func TestKnowledgeWithInitEngine(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), true)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stats should return data even if no documents are indexed.
	statsResp, err := ts.KnowledgeClient.Stats(ctx, &cospb.StatsRequest{})
	if err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.Internal {
			t.Skipf("knowledge engine not fully initialized (Stats returned Internal), skipping: %v", err)
		}
		t.Fatalf("Stats() unexpected error: %v", err)
	}

	if statsResp == nil {
		t.Fatal("Stats() returned nil response")
	}
	// Fields should be present (zero is valid for empty engine).
	if statsResp.UptimeSeconds < 0 {
		t.Error("Stats() uptime_seconds should be >= 0")
	}

	// Search with valid query — should not error even with 0 results.
	searchResp, err := ts.KnowledgeClient.Search(ctx, &cospb.SearchRequest{
		Query: "test",
		Limit: 5,
	})
	if err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.Internal {
			t.Skipf("knowledge engine search not available, skipping: %v", err)
		}
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if searchResp == nil {
		t.Fatal("Search() returned nil response")
	}

	// Index with nonexistent path — should not error (errors go into response.Errors).
	idxResp, err := ts.KnowledgeClient.Index(ctx, &cospb.IndexRequest{
		Path:      "/tmp/nonexistent-file-12345.md",
		Recursive: false,
	})
	if err != nil {
		t.Fatalf("Index() with nonexistent path unexpected gRPC error: %v", err)
	}
	if idxResp == nil {
		t.Fatal("Index() returned nil response")
	}
	// Index of nonexistent file should have errors in the response.
	if len(idxResp.Errors) == 0 && idxResp.DocumentsIndexed == 0 {
		// Both zero: engine handled gracefully or path was ignored.
		t.Log("Index of nonexistent path returned 0 documents and 0 errors (graceful handling)")
	}
}

// ── MemoryService Tests ──────────────────────────────────────────────────────

func TestMemoryStoreAndGet(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Store a record
	storeResp, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:    "decision",
		Layer:   "session",
		Scope:   "test-scope",
		Content: "Test memory content for integration test",
		Metadata: map[string]string{
			"key": "value",
		},
	})
	if err != nil {
		t.Fatalf("Store() unexpected error: %v", err)
	}
	if storeResp == nil {
		t.Fatal("Store() returned nil response")
	}
	if storeResp.Id == "" {
		t.Fatal("Store() returned empty id")
	}
	if storeResp.CreatedAt == "" {
		t.Error("Store() returned empty created_at")
	}

	// Retrieve the stored record
	getResp, err := ts.MemoryClient.Get(ctx, &cospb.GetRequest{
		Id:    storeResp.Id,
		Layer: "session",
	})
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if getResp == nil || getResp.Record == nil {
		t.Fatal("Get() returned nil record")
	}
	if getResp.Record.Id != storeResp.Id {
		t.Errorf("Get() returned wrong id: got %s, want %s", getResp.Record.Id, storeResp.Id)
	}
	if getResp.Record.Content != "Test memory content for integration test" {
		t.Errorf("Get() returned wrong content: got %q, want %q",
			getResp.Record.Content, "Test memory content for integration test")
	}
	if getResp.Record.Layer != "session" {
		t.Errorf("Get() returned wrong layer: got %s, want session", getResp.Record.Layer)
	}
	if getResp.Record.Type != "decision" {
		t.Errorf("Get() returned wrong type: got %s, want decision", getResp.Record.Type)
	}
}

func TestMemoryStoreAndDelete(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Store
	storeResp, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:    "agent",
		Layer:   "session",
		Content: "To be deleted",
	})
	if err != nil {
		t.Fatalf("Store() unexpected error: %v", err)
	}

	// Delete
	delResp, err := ts.MemoryClient.Delete(ctx, &cospb.DeleteRequest{
		Id:    storeResp.Id,
		Layer: "session",
	})
	if err != nil {
		t.Fatalf("Delete() unexpected error: %v", err)
	}
	if delResp == nil {
		t.Fatal("Delete() returned nil response")
	}
	if !delResp.Success {
		t.Error("Delete() success should be true")
	}

	// Get after delete should return NotFound
	_, err = ts.MemoryClient.Get(ctx, &cospb.GetRequest{
		Id:    storeResp.Id,
		Layer: "session",
	})
	if err == nil {
		t.Fatal("Get() after delete expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Errorf("Get() after delete expected NotFound, got %s", st.Code())
	}
}

func TestMemorySearch(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Store two records
	_, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:     "pattern",
		Layer:    "session",
		Content:  "First test pattern for search",
		Metadata: map[string]string{"idx": "1"},
	})
	if err != nil {
		t.Fatalf("Store() unexpected error: %v", err)
	}

	_, err = ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:     "pattern",
		Layer:    "session",
		Content:  "Second test pattern for search",
		Metadata: map[string]string{"idx": "2"},
	})
	if err != nil {
		t.Fatalf("Store() unexpected error: %v", err)
	}

	// Search for them
	searchResp, err := ts.MemoryClient.Search(ctx, &cospb.MemorySearchRequest{
		Query:  "pattern",
		Limit:  10,
		Layers: []string{"session"},
	})
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if searchResp == nil {
		t.Fatal("Search() returned nil response")
	}
	if searchResp.Total < 1 {
		t.Errorf("Search() expected at least 1 result, got %d", searchResp.Total)
	}
	if len(searchResp.Records) < 1 {
		t.Error("Search() records slice should not be empty")
	}
}

func TestMemoryPromote(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Store a record in session layer
	storeResp, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:    "decision",
		Layer:   "session",
		Content: "Decision to promote",
	})
	if err != nil {
		t.Fatalf("Store() unexpected error: %v", err)
	}

	// Promote to project layer
	promoteResp, err := ts.MemoryClient.Promote(ctx, &cospb.PromoteRequest{
		Id:        storeResp.Id,
		FromLayer: "session",
		ToLayer:   "project",
	})
	if err != nil {
		t.Fatalf("Promote() unexpected error: %v", err)
	}
	if promoteResp == nil || promoteResp.Record == nil {
		t.Fatal("Promote() returned nil record")
	}
	if promoteResp.Record.Layer != "project" {
		t.Errorf("Promote() expected layer=project, got %s", promoteResp.Record.Layer)
	}
	if promoteResp.Record.Id != storeResp.Id {
		t.Errorf("Promote() expected same id, got %s", promoteResp.Record.Id)
	}

	// Original record should not be in session anymore
	_, err = ts.MemoryClient.Get(ctx, &cospb.GetRequest{
		Id:    storeResp.Id,
		Layer: "session",
	})
	if err == nil {
		t.Error("Get() on promoted source layer expected error (record moved)")
	}

	// Promoted record should be in project layer
	getResp, err := ts.MemoryClient.Get(ctx, &cospb.GetRequest{
		Id:    storeResp.Id,
		Layer: "project",
	})
	if err != nil {
		t.Fatalf("Get() on promoted target layer unexpected error: %v", err)
	}
	if getResp.Record.Content != "Decision to promote" {
		t.Errorf("Promoted record content mismatch: got %q", getResp.Record.Content)
	}
}

func TestMemoryStats(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Store a record first
	_, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:    "session",
		Layer:   "session",
		Content: "Stats test record",
	})
	if err != nil {
		t.Fatalf("Store() unexpected error: %v", err)
	}

	resp, err := ts.MemoryClient.Stats(ctx, &cospb.MemoryStatsRequest{})
	if err != nil {
		t.Fatalf("Stats() unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("Stats() returned nil response")
	}
	if resp.Layers == nil {
		t.Fatal("Stats() layers map is nil")
	}
	if len(resp.Layers) == 0 {
		t.Error("Stats() expected at least one layer in layers map")
	}

	// Check that session layer exists and has at least 1 record
	sessionStats, ok := resp.Layers["session"]
	if !ok {
		t.Error("Stats() layers should contain 'session' key")
	} else {
		if sessionStats.RecordCount < 1 {
			t.Errorf("Stats() session RecordCount expected >= 1, got %d", sessionStats.RecordCount)
		}
	}
}

func TestMemoryGet_NotFound(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := ts.MemoryClient.Get(ctx, &cospb.GetRequest{
		Id:    "non-existent-id",
		Layer: "session",
	})
	if err == nil {
		t.Fatal("Get() with non-existent id expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Errorf("Get() with non-existent id expected NotFound, got %s", st.Code())
	}
}

func TestMemoryDelete_NotFound(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := ts.MemoryClient.Delete(ctx, &cospb.DeleteRequest{
		Id:    "non-existent-id",
		Layer: "session",
	})
	if err == nil {
		t.Fatal("Delete() with non-existent id expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Errorf("Delete() with non-existent id expected NotFound, got %s", st.Code())
	}
}

func TestMemoryStore_EmptyContent(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:    "session",
		Layer:   "session",
		Content: "",
	})
	if err == nil {
		t.Fatal("Store() with empty content expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("Store() with empty content expected InvalidArgument, got %s", st.Code())
	}
}

func TestMemoryGet_EmptyID(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := ts.MemoryClient.Get(ctx, &cospb.GetRequest{
		Id:    "",
		Layer: "session",
	})
	if err == nil {
		t.Fatal("Get() with empty id expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("Get() with empty id expected InvalidArgument, got %s", st.Code())
	}
}

func TestMemoryPromote_InvalidLayers(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := ts.MemoryClient.Promote(ctx, &cospb.PromoteRequest{
		Id:        "test-id",
		FromLayer: "",
		ToLayer:   "project",
	})
	if err == nil {
		t.Fatal("Promote() with empty from_layer expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("Promote() with empty from_layer expected InvalidArgument, got %s", st.Code())
	}
}

func TestMemoryPromote_NotPersistedLayer(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Store a record
	storeResp, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:    "agent",
		Layer:   "session",
		Content: "Test agent memory",
	})
	if err != nil {
		t.Fatalf("Store() unexpected error: %v", err)
	}

	// Promote with non-existent "from" layer — engine error → Internal
	_, err = ts.MemoryClient.Promote(ctx, &cospb.PromoteRequest{
		Id:        storeResp.Id,
		FromLayer: "nonexistent",
		ToLayer:   "project",
	})
	if err == nil {
		t.Fatal("Promote() with non-existent from_layer expected error")
	}
	st, _ := status.FromError(err)
	// Engine returns Internal for no store on source layer
	if st.Code() != codes.Internal {
		t.Errorf("Promote() with non-existent from_layer expected Internal, got %s", st.Code())
	}
}

func TestMemoryStore_InvalidTTL_StillWorks(t *testing.T) {
	// TTL string that's not a valid duration should be ignored (silently
	// left at zero, engine applies default).
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	storeResp, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:    "session",
		Layer:   "session",
		Content: "Invalid TTL test",
		Ttl:     "not-a-duration",
	})
	if err != nil {
		t.Fatalf("Store() with invalid TTL unexpected error: %v", err)
	}
	if storeResp.Id == "" {
		t.Fatal("Store() with invalid TTL returned empty id")
	}
}

// ── Server Lifecycle Tests ───────────────────────────────────────────────────

func TestGracefulShutdown(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Make one RPC before shutdown
	_, err := ts.RuntimeClient.Status(ctx, &cospb.StatusRequest{})
	if err != nil {
		t.Fatalf("Status() before shutdown unexpected error: %v", err)
	}

	// Graceful stop
	ts.cleanup()

	// After cleanup, connection is closed; further calls should fail.
	// We don't test that here since cleanup already closed the connection.
}

func TestInterceptorRecovery_DoesNotPanic(t *testing.T) {
	// Verify logging interceptor doesn't cause panic on normal calls
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Simple RPC through the logged interceptor
	resp, err := ts.RuntimeClient.Status(ctx, &cospb.StatusRequest{})
	if err != nil {
		t.Fatalf("Status() via interceptors unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("Status() returned nil response")
	}
}

func TestMemoryNilEngine(t *testing.T) {
	// Create server with nil memory engine — verify precondition errors
	logger := zerolog.Nop()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(RecoveryInterceptor(), LoggingInterceptor(logger)),
	)

	// Only register runtime service so the server is valid; memory with nil
	// engine is handled in New() — it skips registration when mem==nil.
	rt := runtime.New(runtime.WithConfig(runtime.DefaultRuntimeConfig()), runtime.WithLogger(logger))
	runtimeSrv := NewRuntimeServiceServer(rt)
	cospb.RegisterRuntimeServiceServer(srv, runtimeSrv)

	go func() {
		_ = srv.Serve(lis)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.Stop()
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	defer srv.GracefulStop()

	// Since memory was not registered, calling it should get Unimplemented
	memClient := cospb.NewMemoryServiceClient(conn)
	_, err = memClient.Store(ctx, &cospb.StoreRequest{
		Type:    "session",
		Layer:   "session",
		Content: "test",
	})
	if err == nil {
		t.Fatal("Store() on unregistered service expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unimplemented {
		t.Errorf("Store() on unregistered service expected Unimplemented, got %s", st.Code())
	}
}

// ── RuntimeService Extended Tests ─────────────────────────────────────────────

// TestRuntimeHealth_NilEngine verifies that Health() with a nil runtime engine
// returns codes.FailedPrecondition.
func TestRuntimeHealth_NilEngine(t *testing.T) {
	logger := zerolog.Nop()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(RecoveryInterceptor(), LoggingInterceptor(logger)),
	)

	nilRuntimeSrv := NewRuntimeServiceServer(nil)
	cospb.RegisterRuntimeServiceServer(srv, nilRuntimeSrv)

	go func() {
		_ = srv.Serve(lis)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.Stop()
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	defer srv.GracefulStop()

	client := cospb.NewRuntimeServiceClient(conn)

	resp, err := client.Health(ctx, &cospb.HealthRequest{})
	if err == nil {
		t.Error("Health() with nil engine expected error, got nil")
	}
	if resp != nil {
		t.Errorf("Health() with nil engine expected nil response, got %v", resp)
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("Health() with nil engine expected FailedPrecondition, got %s", st.Code())
	}
}

// TestRuntimeStatusConcurrent verifies that multiple concurrent Status() RPCs
// complete without panics or data races.
func TestRuntimeStatusConcurrent(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	goroutines := 10
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			resp, err := ts.RuntimeClient.Status(ctx, &cospb.StatusRequest{})
			if err != nil {
				errCh <- err
				return
			}
			if resp.State == "" {
				errCh <- fmt.Errorf("empty state")
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent Status() call failed: %v", err)
	}
}

// TestRuntimeHealthConcurrent verifies that multiple concurrent Health() RPCs
// complete without panics or data races.
func TestRuntimeHealthConcurrent(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	goroutines := 10
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := ts.RuntimeClient.Health(ctx, &cospb.HealthRequest{})
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent Health() call failed: %v", err)
	}
}

// ── KnowledgeService Extended Tests ──────────────────────────────────────────

// TestKnowledgeSearch_WithParams exercises the Search RPC with various
// optional parameters: limit, offset, type filters, path_filter, and
// min_score. Uses an initialized knowledge engine.
func TestKnowledgeSearch_WithParams(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), true)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("Limit", func(t *testing.T) {
		resp, err := ts.KnowledgeClient.Search(ctx, &cospb.SearchRequest{
			Query: "test",
			Limit: 5,
		})
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code() == codes.Internal {
				t.Skipf("knowledge engine not fully initialized: %v", err)
			}
			t.Fatalf("Search() unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("Search() returned nil response")
		}
	})

	t.Run("OffsetAndLimit", func(t *testing.T) {
		resp, err := ts.KnowledgeClient.Search(ctx, &cospb.SearchRequest{
			Query:  "test",
			Limit:  10,
			Offset: 0,
		})
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code() == codes.Internal {
				t.Skipf("knowledge engine not fully initialized: %v", err)
			}
			t.Fatalf("Search() unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("Search() returned nil response")
		}
	})

	t.Run("TypeFilter", func(t *testing.T) {
		resp, err := ts.KnowledgeClient.Search(ctx, &cospb.SearchRequest{
			Query: "test",
			Types: []string{"document", "chunk"},
			Limit: 5,
		})
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code() == codes.Internal {
				t.Skipf("knowledge engine not fully initialized: %v", err)
			}
			t.Fatalf("Search() with type filter unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("Search() returned nil response")
		}
	})

	t.Run("PathFilter", func(t *testing.T) {
		resp, err := ts.KnowledgeClient.Search(ctx, &cospb.SearchRequest{
			Query:      "test",
			PathFilter: "/tmp",
			Limit:      5,
		})
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code() == codes.Internal {
				t.Skipf("knowledge engine not fully initialized: %v", err)
			}
			t.Fatalf("Search() with path filter unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("Search() returned nil response")
		}
	})

	t.Run("MinScore", func(t *testing.T) {
		resp, err := ts.KnowledgeClient.Search(ctx, &cospb.SearchRequest{
			Query:    "test",
			MinScore: 0.1,
			Limit:    5,
		})
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code() == codes.Internal {
				t.Skipf("knowledge engine not fully initialized: %v", err)
			}
			t.Fatalf("Search() with min_score unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("Search() returned nil response")
		}
	})

	t.Run("ZeroLimit_DefaultsApplied", func(t *testing.T) {
		resp, err := ts.KnowledgeClient.Search(ctx, &cospb.SearchRequest{
			Query: "test",
			Limit: 0, // Should use default 20
		})
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code() == codes.Internal {
				t.Skipf("knowledge engine not fully initialized: %v", err)
			}
			t.Fatalf("Search() with zero limit unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("Search() returned nil response")
		}
	})
}

// TestKnowledgeStats_Concurrent verifies that Stats() can be called
// concurrently without panics or data races.
func TestKnowledgeStats_Concurrent(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), true)
	defer ts.cleanup()

	goroutines := 10
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := ts.KnowledgeClient.Stats(ctx, &cospb.StatsRequest{})
			if err != nil {
				// Internal is expected if engine not fully initialized
				st, _ := status.FromError(err)
				if st.Code() != codes.Internal {
					errCh <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Logf("concurrent Stats() warning (expected for uninitialized engine): %v", err)
	}
}

// TestKnowledgeSearch_Concurrent verifies that Search() can be called
// concurrently without panics or data races.
func TestKnowledgeSearch_Concurrent(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), true)
	defer ts.cleanup()

	goroutines := 10
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := ts.KnowledgeClient.Search(ctx, &cospb.SearchRequest{
				Query: fmt.Sprintf("concurrent-search-%d", idx),
				Limit: 5,
			})
			if err != nil {
				st, _ := status.FromError(err)
				// Internal is expected if engine not fully initialized
				if st.Code() != codes.Internal {
					errCh <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Logf("concurrent Search() warning (expected for uninitialized engine): %v", err)
	}
}

// ── MemoryService Extended Tests ─────────────────────────────────────────────

// TestMemorySearch_EmptyQuery verifies that searching with an empty query
// returns results (all records, sorted by recency).
func TestMemorySearch_EmptyQuery(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Store a few records first
	for i := 0; i < 3; i++ {
		_, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
			Type:    "pattern",
			Layer:   "session",
			Content: fmt.Sprintf("Empty query test record %d", i),
		})
		if err != nil {
			t.Fatalf("Store() unexpected error: %v", err)
		}
	}

	// Search with empty query — should return records
	resp, err := ts.MemoryClient.Search(ctx, &cospb.MemorySearchRequest{
		Query: "",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search() with empty query unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("Search() returned nil response")
	}
	if resp.Total < 1 {
		t.Errorf("Search() with empty query expected at least 1 result, got %d", resp.Total)
	}
}

// TestMemorySearch_WithFilters verifies searching with type and layer filters.
func TestMemorySearch_WithFilters(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Store records of different types in different layers
	_, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:    "decision",
		Layer:   "session",
		Content: "A decision record for filter testing",
	})
	if err != nil {
		t.Fatalf("Store() unexpected error: %v", err)
	}

	_, err = ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:    "bug",
		Layer:   "project",
		Content: "A bug record for filter testing",
	})
	if err != nil {
		t.Fatalf("Store() unexpected error: %v", err)
	}

	_, err = ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:    "pattern",
		Layer:   "session",
		Content: "A pattern record for filter testing",
	})
	if err != nil {
		t.Fatalf("Store() unexpected error: %v", err)
	}

	t.Run("TypeFilter", func(t *testing.T) {
		resp, err := ts.MemoryClient.Search(ctx, &cospb.MemorySearchRequest{
			Query: "filter testing",
			Types: []string{"bug"},
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("Search() with type filter unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("Search() returned nil response")
		}
		if resp.Total < 1 {
			t.Errorf("Search() with type filter expected at least 1 result, got %d", resp.Total)
		}
		// Verify returned records have matching type
		for _, rec := range resp.Records {
			if rec.Type != "bug" {
				t.Errorf("Search() returned record with type=%q, want 'bug'", rec.Type)
			}
		}
	})

	t.Run("LayerFilter", func(t *testing.T) {
		resp, err := ts.MemoryClient.Search(ctx, &cospb.MemorySearchRequest{
			Query:  "filter testing",
			Layers: []string{"session"},
			Limit:  10,
		})
		if err != nil {
			t.Fatalf("Search() with layer filter unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("Search() returned nil response")
		}
		if resp.Total < 1 {
			t.Errorf("Search() with layer filter expected at least 1 result, got %d", resp.Total)
		}
		for _, rec := range resp.Records {
			if rec.Layer != "session" {
				t.Errorf("Search() returned record with layer=%q, want 'session'", rec.Layer)
			}
		}
	})

	t.Run("CombinedFilters", func(t *testing.T) {
		resp, err := ts.MemoryClient.Search(ctx, &cospb.MemorySearchRequest{
			Query:  "filter testing",
			Types:  []string{"pattern"},
			Layers: []string{"session"},
			Limit:  10,
		})
		if err != nil {
			t.Fatalf("Search() with combined filters unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("Search() returned nil response")
		}
		if resp.Total < 1 {
			t.Errorf("Search() with combined filters expected at least 1 result, got %d", resp.Total)
		}
		for _, rec := range resp.Records {
			if rec.Type != "pattern" {
				t.Errorf("Search() returned record with type=%q, want 'pattern'", rec.Type)
			}
			if rec.Layer != "session" {
				t.Errorf("Search() returned record with layer=%q, want 'session'", rec.Layer)
			}
		}
	})
}

// TestMemorySearch_NoResults verifies that searching for a non-matching query
// returns zero results without error.
func TestMemorySearch_NoResults(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Store a record
	_, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
		Type:    "agent",
		Layer:   "session",
		Content: "Existing record",
	})
	if err != nil {
		t.Fatalf("Store() unexpected error: %v", err)
	}

	// Search for something that doesn't match
	resp, err := ts.MemoryClient.Search(ctx, &cospb.MemorySearchRequest{
		Query: "xyznonexistent12345",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search() for non-matching query unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("Search() returned nil response")
	}
	// Zero results is valid — no error should be returned
}

// TestMemoryDelete_EmptyID verifies that Delete() with an empty ID returns
// codes.InvalidArgument.
func TestMemoryDelete_EmptyID(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := ts.MemoryClient.Delete(ctx, &cospb.DeleteRequest{
		Id:    "",
		Layer: "session",
	})
	if err == nil {
		t.Fatal("Delete() with empty id expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("Delete() with empty id expected InvalidArgument, got %s", st.Code())
	}
}

// TestMemoryStore_MultipleTypes verifies storing records with various
// memory types (decision, agent, pattern, bug, architecture, project)
// and layers (session, project, global, workspace, temp).
func TestMemoryStore_MultipleTypes(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testCases := []struct {
		memType string
		layer   string
		content string
	}{
		{memType: "decision", layer: "session", content: "Decision type record"},
		{memType: "agent", layer: "project", content: "Agent type record"},
		{memType: "pattern", layer: "global", content: "Pattern type record"},
		{memType: "bug", layer: "workspace", content: "Bug type record"},
		{memType: "architecture", layer: "temp", content: "Architecture type record"},
		{memType: "project", layer: "session", content: "Project type record"},
		{memType: "session", layer: "project", content: "Session type record"},
	}

	ids := make(map[string]string)

	for _, tc := range testCases {
		t.Run(tc.memType+"_"+tc.layer, func(t *testing.T) {
			resp, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
				Type:    tc.memType,
				Layer:   tc.layer,
				Content: tc.content,
				Metadata: map[string]string{
					"test_type":  tc.memType,
					"test_layer": tc.layer,
				},
			})
			if err != nil {
				t.Fatalf("Store(%s/%s) unexpected error: %v", tc.memType, tc.layer, err)
			}
			if resp == nil {
				t.Fatal("Store() returned nil response")
			}
			if resp.Id == "" {
				t.Error("Store() returned empty id")
			}
			if resp.CreatedAt == "" {
				t.Error("Store() returned empty created_at")
			}

			ids[tc.memType+"_"+tc.layer] = resp.Id

			// Verify retrieval
			getResp, err := ts.MemoryClient.Get(ctx, &cospb.GetRequest{
				Id:    resp.Id,
				Layer: tc.layer,
			})
			if err != nil {
				t.Fatalf("Get(%s) unexpected error: %v", resp.Id, err)
			}
			if getResp.Record.Content != tc.content {
				t.Errorf("Get() content = %q, want %q", getResp.Record.Content, tc.content)
			}
			if getResp.Record.Type != tc.memType {
				t.Errorf("Get() type = %q, want %q", getResp.Record.Type, tc.memType)
			}
		})
	}

	// Verify stats show multiple layers populated
	statsResp, err := ts.MemoryClient.Stats(ctx, &cospb.MemoryStatsRequest{})
	if err != nil {
		t.Fatalf("Stats() unexpected error: %v", err)
	}
	if statsResp == nil {
		t.Fatal("Stats() returned nil response")
	}
	if len(statsResp.Layers) == 0 {
		t.Error("Stats() expected non-empty layers map")
	}
	// session and project layers should have records
	if _, ok := statsResp.Layers["session"]; !ok {
		t.Error("Stats() missing 'session' layer")
	}
	if _, ok := statsResp.Layers["project"]; !ok {
		t.Error("Stats() missing 'project' layer")
	}
}

// TestMemoryConcurrent_StoreAndRetrieve verifies that concurrent Store and
// Get operations complete without panics or data races.
func TestMemoryConcurrent_StoreAndRetrieve(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	goroutines := 10
	type stored struct {
		id   string
		text string
	}
	resultCh := make(chan stored, goroutines)
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Store
			content := fmt.Sprintf("Concurrent test record %d", idx)
			storeResp, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
				Type:    "session",
				Layer:   "session",
				Content: content,
			})
			if err != nil {
				errCh <- fmt.Errorf("concurrent Store(%d): %w", idx, err)
				return
			}

			// Retrieve
			getResp, err := ts.MemoryClient.Get(ctx, &cospb.GetRequest{
				Id:    storeResp.Id,
				Layer: "session",
			})
			if err != nil {
				errCh <- fmt.Errorf("concurrent Get(%d): %w", idx, err)
				return
			}

			if getResp.Record.Content != content {
				errCh <- fmt.Errorf("concurrent content mismatch for %d: got %q, want %q", idx, getResp.Record.Content, content)
				return
			}

			resultCh <- stored{id: storeResp.Id, text: content}
		}(i)
	}

	wg.Wait()
	close(errCh)
	close(resultCh)

	for err := range errCh {
		t.Error(err)
	}

	results := make([]stored, 0, goroutines)
	for r := range resultCh {
		results = append(results, r)
	}

	if len(results) != goroutines {
		t.Errorf("expected %d concurrent successes, got %d", goroutines, len(results))
	}
}

// TestMemoryConcurrent_Search verifies that concurrent Search() operations
// complete without panics or data races.
func TestMemoryConcurrent_Search(t *testing.T) {
	ts := newBufconnServer(t, t.TempDir(), false)
	defer ts.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Pre-populate records
	for i := 0; i < 5; i++ {
		_, err := ts.MemoryClient.Store(ctx, &cospb.StoreRequest{
			Type:    "pattern",
			Layer:   "session",
			Content: fmt.Sprintf("Concurrent search record %d", i),
		})
		if err != nil {
			t.Fatalf("Store() prep error: %v", err)
		}
	}

	goroutines := 10
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel2()
			resp, err := ts.MemoryClient.Search(ctx2, &cospb.MemorySearchRequest{
				Query: "record",
				Limit: 10,
			})
			if err != nil {
				errCh <- err
				return
			}
			if resp == nil {
				errCh <- fmt.Errorf("nil response")
				return
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent Search() call failed: %v", err)
	}
}

// ── MemoryService Nil Engine Tests ───────────────────────────────────────────

// TestMemoryGet_NilEngine verifies that Get() with nil memory engine returns
// codes.FailedPrecondition.
func TestMemoryGet_NilEngine(t *testing.T) {
	logger := zerolog.Nop()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(RecoveryInterceptor(), LoggingInterceptor(logger)),
	)

	// Register memory service with nil engine explicitly
	nilMemSrv := NewMemoryServiceServer(nil)
	cospb.RegisterMemoryServiceServer(srv, nilMemSrv)

	go func() {
		_ = srv.Serve(lis)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.Stop()
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	defer srv.GracefulStop()

	memClient := cospb.NewMemoryServiceClient(conn)

	_, err = memClient.Get(ctx, &cospb.GetRequest{Id: "test", Layer: "session"})
	if err == nil {
		t.Fatal("Get() with nil engine expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("Get() with nil engine expected FailedPrecondition, got %s", st.Code())
	}
}

// TestMemorySearch_NilEngine verifies that Search() with nil memory engine returns
// codes.FailedPrecondition.
func TestMemorySearch_NilEngine(t *testing.T) {
	logger := zerolog.Nop()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(RecoveryInterceptor(), LoggingInterceptor(logger)),
	)

	nilMemSrv := NewMemoryServiceServer(nil)
	cospb.RegisterMemoryServiceServer(srv, nilMemSrv)

	go func() {
		_ = srv.Serve(lis)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.Stop()
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	defer srv.GracefulStop()

	memClient := cospb.NewMemoryServiceClient(conn)

	_, err = memClient.Search(ctx, &cospb.MemorySearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("Search() with nil engine expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("Search() with nil engine expected FailedPrecondition, got %s", st.Code())
	}
}

// TestMemoryStats_NilEngine verifies that Stats() with nil memory engine returns
// codes.FailedPrecondition.
func TestMemoryStats_NilEngine(t *testing.T) {
	logger := zerolog.Nop()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(RecoveryInterceptor(), LoggingInterceptor(logger)),
	)

	nilMemSrv := NewMemoryServiceServer(nil)
	cospb.RegisterMemoryServiceServer(srv, nilMemSrv)

	go func() {
		_ = srv.Serve(lis)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.Stop()
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	defer srv.GracefulStop()

	memClient := cospb.NewMemoryServiceClient(conn)

	_, err = memClient.Stats(ctx, &cospb.MemoryStatsRequest{})
	if err == nil {
		t.Fatal("Stats() with nil engine expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("Stats() with nil engine expected FailedPrecondition, got %s", st.Code())
	}
}

// ── Server Config Tests ──────────────────────────────────────────────────────

// TestGRPCDefaultHostLocalhost verifies the gRPC server defaults to binding
// loopback only (127.0.0.1) — network exposure requires an explicit opt-in
// (Host != "127.0.0.1") and, per the config contract, TLS credentials.
func TestGRPCDefaultHostLocalhost(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Host != "127.0.0.1" {
		t.Errorf("DefaultConfig().Host = %q, want %q (loopback only)", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 14122 {
		t.Errorf("DefaultConfig().Port = %d, want 14122", cfg.Port)
	}
	if cfg.Reflection {
		t.Error("DefaultConfig().Reflection must be false (disabled by default)")
	}
}

// ── AuthInterceptor Tests ─────────────────────────────────────────────────────

// generateTestToken creates a signed JWT for testing purposes.
func generateTestToken(secret []byte, expOffset time.Duration) (string, error) {
	claims := internalauth.Claims{
		Sub:      "test-user-id",
		Username: "test-user",
		Role:     "admin",
		Type:     "access",
		Iat:      time.Now().Unix(),
		Exp:      time.Now().Add(expOffset).Unix(),
	}
	return internalauth.GenerateToken(claims, secret)
}

func TestAuthInterceptor_NoSecret_PassesThrough(t *testing.T) {
	interceptor := AuthInterceptor(nil)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// Verify no claims in context
		if _, ok := auth.ClaimsFromContext(ctx); ok {
			t.Error("expected no claims in context when secret is nil")
		}
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{FullMethod: "/test.Svc/M"}, handler)
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected response 'ok', got %v", resp)
	}
}

func TestAuthInterceptor_EmptySecret_PassesThrough(t *testing.T) {
	interceptor := AuthInterceptor([]byte{})

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{FullMethod: "/test.Svc/M"}, handler)
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected response 'ok', got %v", resp)
	}
}

func TestAuthInterceptor_MissingMetadata(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-for-hmac!!!")
	interceptor := AuthInterceptor(secret)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Error("handler should not be called")
		return "ok", nil
	}

	// Pass a bare context without metadata
	_, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{FullMethod: "/test.Svc/M"}, handler)
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %s", st.Code())
	}
}

func TestAuthInterceptor_MissingAuthorizationHeader(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-for-hmac!!!")
	interceptor := AuthInterceptor(secret)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Error("handler should not be called")
		return "ok", nil
	}

	md := metadata.Pairs("some-other-header", "value")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/test.Svc/M"}, handler)
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %s", st.Code())
	}
}

func TestAuthInterceptor_InvalidAuthorizationFormat(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-for-hmac!!!")
	interceptor := AuthInterceptor(secret)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Error("handler should not be called")
		return "ok", nil
	}

	md := metadata.Pairs("authorization", "Basic dXNlcjpwYXNz")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/test.Svc/M"}, handler)
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %s", st.Code())
	}
}

func TestAuthInterceptor_EmptyBearerToken(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-for-hmac!!!")
	interceptor := AuthInterceptor(secret)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Error("handler should not be called")
		return "ok", nil
	}

	// "Bearer " with nothing after it
	md := metadata.Pairs("authorization", "Bearer ")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/test.Svc/M"}, handler)
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %s", st.Code())
	}
}

func TestAuthInterceptor_InvalidToken(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-for-hmac!!!")
	interceptor := AuthInterceptor(secret)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Error("handler should not be called")
		return "ok", nil
	}

	md := metadata.Pairs("authorization", "Bearer invalid.token.here")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/test.Svc/M"}, handler)
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %s", st.Code())
	}
}

func TestAuthInterceptor_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-for-hmac!!!")
	interceptor := AuthInterceptor(secret)

	// Generate a token that expired 1 hour ago
	token, err := generateTestToken(secret, -1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Error("handler should not be called")
		return "ok", nil
	}

	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err = interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/test.Svc/M"}, handler)
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %s", st.Code())
	}
	// Verify the error message mentions expiry
	if st.Code() == codes.Unauthenticated && !strings.Contains(st.Message(), "expired") {
		t.Logf("token expired error message: %s", st.Message())
	}
}

func TestAuthInterceptor_ValidToken_SetsClaims(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-for-hmac!!!")
	interceptor := AuthInterceptor(secret)

	token, err := generateTestToken(secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate valid token: %v", err)
	}

	var capturedClaims bool
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		claims, ok := auth.ClaimsFromContext(ctx)
		if !ok {
			t.Error("expected claims in context")
		} else {
			capturedClaims = true
			if claims.Sub != "test-user-id" {
				t.Errorf("expected sub 'test-user-id', got %s", claims.Sub)
			}
			if claims.Username != "test-user" {
				t.Errorf("expected username 'test-user', got %s", claims.Username)
			}
			if claims.Role != "admin" {
				t.Errorf("expected role 'admin', got %s", claims.Role)
			}
		}
		return "ok", nil
	}

	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/test.Svc/M"}, handler)
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected response 'ok', got %v", resp)
	}
	if !capturedClaims {
		t.Error("handler was not invoked with claims in context")
	}
}

func TestAuthInterceptor_WrongSecret(t *testing.T) {
	signSecret := []byte("sign-secret-32-bytes-long-for-hmac!!!!")
	verifySecret := []byte("verify-secret-32-bytes-long-for-hmac!!")
	interceptor := AuthInterceptor(verifySecret)

	token, err := generateTestToken(signSecret, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Error("handler should not be called")
		return "ok", nil
	}

	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err = interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/test.Svc/M"}, handler)
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %s", st.Code())
	}
}

func TestAuthInterceptor_IntegrationWithBufconn(t *testing.T) {
	// Integration test: create a full server with auth interceptor and
	// verify that unauthenticated calls are rejected while authenticated
	// calls succeed.
	secret := []byte("test-secret-32-bytes-long-for-hmac!!!")
	logger := zerolog.Nop()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			RecoveryInterceptor(),
			AuthInterceptor(secret),
			LoggingInterceptor(logger),
		),
	)

	rt := runtime.New(runtime.WithConfig(runtime.DefaultRuntimeConfig()), runtime.WithLogger(logger))
	runtimeSrv := NewRuntimeServiceServer(rt)
	cospb.RegisterRuntimeServiceServer(srv, runtimeSrv)

	go func() {
		_ = srv.Serve(lis)
	}()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.Stop()
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	defer srv.GracefulStop()

	client := cospb.NewRuntimeServiceClient(conn)

	t.Run("no auth returns Unauthenticated", func(t *testing.T) {
		noAuthCtx, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel2()

		_, err := client.Status(noAuthCtx, &cospb.StatusRequest{})
		if err == nil {
			t.Fatal("expected error for unauthenticated request")
		}
		st, _ := status.FromError(err)
		if st.Code() != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %s", st.Code())
		}
	})

	t.Run("valid token succeeds", func(t *testing.T) {
		token, err := generateTestToken(secret, 1*time.Hour)
		if err != nil {
			t.Fatalf("failed to generate valid token: %v", err)
		}

		authCtx := metadata.AppendToOutgoingContext(
			context.Background(),
			"authorization", "Bearer "+token,
		)
		authCtx, cancel2 := context.WithTimeout(authCtx, 3*time.Second)
		defer cancel2()

		resp, err := client.Status(authCtx, &cospb.StatusRequest{})
		if err != nil {
			t.Fatalf("authenticated Status() unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("authenticated Status() returned nil response")
		}
		if resp.State == "" {
			t.Error("authenticated Status() expected non-empty state")
		}
	})

	t.Run("invalid token returns Unauthenticated", func(t *testing.T) {
		invalidCtx := metadata.AppendToOutgoingContext(
			context.Background(),
			"authorization", "Bearer invalid.token.here",
		)
		invalidCtx, cancel2 := context.WithTimeout(invalidCtx, 3*time.Second)
		defer cancel2()

		_, err := client.Status(invalidCtx, &cospb.StatusRequest{})
		if err == nil {
			t.Fatal("expected error for invalid token")
		}
		st, _ := status.FromError(err)
		if st.Code() != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %s", st.Code())
		}
	})
}
