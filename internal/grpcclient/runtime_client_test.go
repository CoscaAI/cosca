package grpcclient

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/api/grpcserver"
	"github.com/CoscaAI/cosca/internal/runtime"
	"google.golang.org/grpc"
)

// =============================================================================
// Helpers — real gRPC daemon on an ephemeral loopback port
// =============================================================================

// startTestDaemon boots a REAL gRPC server on an ephemeral loopback port and
// registers the RuntimeService backed by the given runtime engine. engine may
// be nil — the RuntimeServiceServer is nil-safe (RPCs return
// codes.FailedPrecondition). This mirrors the standalone `cosca runtime start`
// daemon (FASE 1) and the pattern in api/grpcserver/server_test.go.
//
// Returns the listen address and a stop function that force-stops the server.
func startTestDaemon(t *testing.T, engine *runtime.Runtime) (addr string, stop func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral loopback port: %v", err)
	}

	srv := grpc.NewServer()
	cospb.RegisterRuntimeServiceServer(srv, grpcserver.NewRuntimeServiceServer(engine))

	go func() {
		_ = srv.Serve(lis)
	}()

	return lis.Addr().String(), func() { srv.Stop() }
}

// closedLoopbackAddr returns a loopback address that is very likely CLOSED:
// bind an ephemeral port and release it immediately. Connection attempts get
// an immediate ECONNREFUSED — no daemon is listening there.
func closedLoopbackAddr(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for closed-port probe: %v", err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()
	return addr
}

// =============================================================================
// NewRuntimeClient / Addr
// =============================================================================

// TestNewRuntimeClient_DefaultAddr verifies that an empty address defaults to
// the canonical standalone daemon address (FASE 1 — 127.0.0.1:14123).
func TestNewRuntimeClient_DefaultAddr(t *testing.T) {
	c := NewRuntimeClient("")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if got := c.Addr(); got != DefaultRuntimeAddr {
		t.Errorf("Addr() = %q, want %q", got, DefaultRuntimeAddr)
	}
}

// TestNewRuntimeClient_ExplicitAddr verifies an explicit address is preserved.
func TestNewRuntimeClient_ExplicitAddr(t *testing.T) {
	const addr = "127.0.0.1:14999"
	c := NewRuntimeClient(addr)
	if got := c.Addr(); got != addr {
		t.Errorf("Addr() = %q, want %q", got, addr)
	}
}

// =============================================================================
// Status / Health against a live daemon
// =============================================================================

// TestRuntimeClient_Status_WithRealDaemon verifies Status returns the daemon's
// real response (state, version) over a real gRPC round-trip.
func TestRuntimeClient_Status_WithRealDaemon(t *testing.T) {
	rt := runtime.New(runtime.WithConfig(runtime.RuntimeConfig{
		Version: "1.0.0-client-test",
	}))
	addr, stop := startTestDaemon(t, rt)
	defer stop()

	c := NewRuntimeClient(addr)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("Status() unexpected error: %v", err)
	}
	if resp.GetVersion() != "1.0.0-client-test" {
		t.Errorf("Version = %q, want %q", resp.GetVersion(), "1.0.0-client-test")
	}
	if resp.GetState() == "" {
		t.Error("State expected non-empty")
	}
	if resp.GetHealth() == "" {
		t.Error("Health expected non-empty")
	}
}

// TestRuntimeClient_Health_WithRealDaemon verifies Health returns the daemon's
// real health response over a real gRPC round-trip. A fresh runtime reports
// unknown health, which maps to healthy=true (matching runtimeHealthToPb).
func TestRuntimeClient_Health_WithRealDaemon(t *testing.T) {
	rt := runtime.New()
	addr, stop := startTestDaemon(t, rt)
	defer stop()

	c := NewRuntimeClient(addr)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.Health(ctx)
	if err != nil {
		t.Fatalf("Health() unexpected error: %v", err)
	}
	if !resp.GetHealthy() {
		t.Error("Healthy expected true for a fresh runtime (unknown health)")
	}
}

// TestRuntimeClient_ServerNilEngineNilSafe verifies the client survives a
// daemon whose engine is nil: the RuntimeServiceServer returns
// codes.FailedPrecondition (no panic), which the client maps to the canonical
// "daemon not active" error.
func TestRuntimeClient_ServerNilEngineNilSafe(t *testing.T) {
	addr, stop := startTestDaemon(t, nil) // nil engine — service is nil-safe
	defer stop()

	c := NewRuntimeClient(addr)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.Status(ctx)
	if err == nil {
		t.Fatal("expected error for daemon with nil engine")
	}
	if !strings.Contains(err.Error(), "não está ativo") {
		t.Errorf("error = %q, want canonical 'não está ativo'", err.Error())
	}

	_, err = c.Health(ctx)
	if err == nil {
		t.Fatal("expected error for daemon with nil engine")
	}
	if !strings.Contains(err.Error(), "não está ativo") {
		t.Errorf("error = %q, want canonical 'não está ativo'", err.Error())
	}
}

// =============================================================================
// Daemon down
// =============================================================================

// TestRuntimeClient_Status_DaemonDown verifies Status surfaces the canonical
// "não está ativo" error when no daemon is listening at the address. A closed
// loopback port fails in milliseconds (no 2s timeout hit).
func TestRuntimeClient_Status_DaemonDown(t *testing.T) {
	addr := closedLoopbackAddr(t)
	c := NewRuntimeClient(addr)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.Status(ctx)
	if err == nil {
		t.Fatal("expected error when daemon is down")
	}
	if !strings.Contains(err.Error(), "não está ativo") {
		t.Errorf("error = %q, want canonical 'não está ativo'", err.Error())
	}
	if !strings.Contains(err.Error(), addr) {
		t.Errorf("error = %q, want address %q in message", err.Error(), addr)
	}
}

// TestRuntimeClient_Health_DaemonDown verifies Health surfaces the same
// canonical error when the daemon is down.
func TestRuntimeClient_Health_DaemonDown(t *testing.T) {
	addr := closedLoopbackAddr(t)
	c := NewRuntimeClient(addr)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.Health(ctx)
	if err == nil {
		t.Fatal("expected error when daemon is down")
	}
	if !strings.Contains(err.Error(), "não está ativo") {
		t.Errorf("error = %q, want canonical 'não está ativo'", err.Error())
	}
}

// =============================================================================
// Close idempotence + redial
// =============================================================================

// TestRuntimeClient_CloseIdempotent verifies Close can be called multiple
// times without panicking or erroring — including when no connection was ever
// dialed (lazy client).
func TestRuntimeClient_CloseIdempotent(t *testing.T) {
	// Never dialed.
	c := NewRuntimeClient(closedLoopbackAddr(t))
	if err := c.Close(); err != nil {
		t.Errorf("first Close on never-dialed client: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second Close on never-dialed client: %v", err)
	}

	// Dialed against a live daemon, then closed twice.
	rt := runtime.New()
	addr, stop := startTestDaemon(t, rt)
	defer stop()

	c2 := NewRuntimeClient(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c2.Status(ctx); err != nil {
		t.Fatalf("Status() before Close: %v", err)
	}
	if err := c2.Close(); err != nil {
		t.Errorf("first Close after dial: %v", err)
	}
	if err := c2.Close(); err != nil {
		t.Errorf("second Close after dial: %v", err)
	}
}

// TestRuntimeClient_RedialAfterClose verifies the client recovers after
// Close: the next RPC lazily dials a fresh connection instead of panicking on
// the nil'ed internal state.
func TestRuntimeClient_RedialAfterClose(t *testing.T) {
	rt := runtime.New()
	addr, stop := startTestDaemon(t, rt)
	defer stop()

	c := NewRuntimeClient(addr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := c.Status(ctx); err != nil {
		t.Fatalf("Status() before Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	resp, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("Status() after Close (redial) failed: %v", err)
	}
	if resp.GetState() == "" {
		t.Error("redialed Status() returned empty state")
	}
	if err := c.Close(); err != nil {
		t.Errorf("final Close(): %v", err)
	}
}
