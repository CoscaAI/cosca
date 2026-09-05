package handler_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/api/grpcserver"
	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/api/stream"
	"github.com/CoscaAI/cosca/internal/grpcclient"
	"github.com/CoscaAI/cosca/internal/runtime"
)

// =============================================================================
// NewRuntimeHandler
// =============================================================================

// TestNewRuntimeHandlerNil verifies that NewRuntimeHandler with nil rt
// does not panic and returns a non-nil handler.
func TestNewRuntimeHandlerNil(t *testing.T) {
	h := handler.NewRuntimeHandler(nil)
	if h == nil {
		t.Fatal("expected non-nil handler when rt is nil")
	}
}

// TestNewRuntimeHandlerNonNil verifies that NewRuntimeHandler with a
// real runtime returns a non-nil handler.
func TestNewRuntimeHandlerNonNil(t *testing.T) {
	rt := runtime.New()
	h := handler.NewRuntimeHandler(rt)
	if h == nil {
		t.Fatal("expected non-nil handler when rt is non-nil")
	}
}

// =============================================================================
// SetHub
// =============================================================================

// TestSetHubNil verifies that SetHub with nil does not panic.
func TestSetHubNil(t *testing.T) {
	h := handler.NewRuntimeHandler(nil)
	// Should not panic.
	h.SetHub(nil)
}

// TestSetHubNonNil verifies that SetHub with a non-nil hub does not panic.
func TestSetHubNonNil(t *testing.T) {
	h := handler.NewRuntimeHandler(nil)
	hub := stream.NewHub(zerolog.Nop())
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hub.Shutdown(ctx)
	}()

	// Should not panic.
	h.SetHub(hub)
}

// =============================================================================
// Status Handler
// =============================================================================

// TestRuntimeHandlerStatus runs the Status handler for various runtime
// configurations and verifies the JSON response.
func TestRuntimeHandlerStatus(t *testing.T) {
	tests := []struct {
		name        string
		setupRT     func() *runtime.Runtime
		wantStatus  int
		wantState   string
		wantHealth  string
		wantUptime  string
		wantVersion string
		checkComp   bool // if true, verify components key exists
		wantCompCt  int
	}{
		{
			name:        "nil runtime",
			setupRT:     func() *runtime.Runtime { return nil },
			wantStatus:  http.StatusOK,
			wantState:   "unknown",
			wantHealth:  "unknown",
			wantUptime:  "0s",
			wantVersion: "0.0.0",
			checkComp:   false,
		},
		{
			name: "non-nil runtime default state",
			setupRT: func() *runtime.Runtime {
				return runtime.New(runtime.WithConfig(runtime.RuntimeConfig{
					Version: "1.0.0-test",
				}))
			},
			wantStatus:  http.StatusOK,
			wantState:   "uninitialized",
			wantHealth:  "unknown",
			wantUptime:  "0s",
			wantVersion: "1.0.0-test",
			checkComp:   false,
		},
		{
			name: "non-nil runtime ready with components",
			setupRT: func() *runtime.Runtime {
				rt := runtime.New(runtime.WithConfig(runtime.RuntimeConfig{
					Version: "2.0.0",
				}))
				// Valid transitions: Uninitialized -> Initializing -> Ready.
				if err := rt.State().TransitionTo(runtime.StateInitializing, "test init"); err != nil {
					panic("transition to initializing failed: " + err.Error())
				}
				if err := rt.State().TransitionTo(runtime.StateReady, "test ready"); err != nil {
					panic("transition to ready failed: " + err.Error())
				}
				// Add components.
				rt.State().SetComponentStatus("cache", runtime.StatusHealthy, "cache ok")
				rt.State().SetComponentStatus("memory", runtime.StatusDegraded, "memory slow")
				return rt
			},
			wantStatus:  http.StatusOK,
			wantState:   "ready",
			wantHealth:  "degraded", // recomputed from components
			wantUptime:  "0s",
			wantVersion: "2.0.0",
			checkComp:   true,
			wantCompCt:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := tt.setupRT()
			h := handler.NewRuntimeHandler(rt)

			req := httptest.NewRequest("GET", "/v1/status", nil)
			w := httptest.NewRecorder()

			h.Status(w, req)

			// Check status code.
			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			// Check Content-Type.
			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
			}

			// Unmarshal and verify.
			var resp handler.StatusResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if resp.State != tt.wantState {
				t.Errorf("expected state '%s', got '%s'", tt.wantState, resp.State)
			}
			if resp.Health != tt.wantHealth {
				t.Errorf("expected health '%s', got '%s'", tt.wantHealth, resp.Health)
			}
			if resp.Uptime != tt.wantUptime {
				t.Errorf("expected uptime '%s', got '%s'", tt.wantUptime, resp.Uptime)
			}
			if resp.Version != tt.wantVersion {
				t.Errorf("expected version '%s', got '%s'", tt.wantVersion, resp.Version)
			}

			if tt.checkComp {
				if len(resp.Components) != tt.wantCompCt {
					t.Errorf("expected %d components, got %d: %v", tt.wantCompCt, len(resp.Components), resp.Components)
				}
				// Verify specific components exist.
				for _, name := range []string{"cache", "memory"} {
					if _, ok := resp.Components[name]; !ok {
						t.Errorf("expected component '%s' in response", name)
					}
				}
			}
		})
	}
}

// TestRuntimeHandlerStatusContentType verifies Content-Type header on success.
func TestRuntimeHandlerStatusContentType(t *testing.T) {
	h := handler.NewRuntimeHandler(nil)

	req := httptest.NewRequest("GET", "/v1/status", nil)
	w := httptest.NewRecorder()

	h.Status(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
	}
}

// TestRuntimeHandlerStatusCode verifies the HTTP status code is always 200.
func TestRuntimeHandlerStatusCode(t *testing.T) {
	// Both nil and non-nil runtime should return 200.
	cases := []struct {
		name string
		rt   *runtime.Runtime
	}{
		{"nil runtime", nil},
		{"non-nil runtime", runtime.New()},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := handler.NewRuntimeHandler(c.rt)
			req := httptest.NewRequest("GET", "/v1/status", nil)
			w := httptest.NewRecorder()

			h.Status(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}
		})
	}
}

// =============================================================================
// Health Handler
// =============================================================================

// TestRuntimeHandlerHealth runs the Health handler for various runtime
// health states and verifies the JSON response.
func TestRuntimeHandlerHealth(t *testing.T) {
	tests := []struct {
		name        string
		setupRT     func() *runtime.Runtime
		wantStatus  int
		wantHealthy bool
		wantWarning bool   // if true, at least one warning expected
		wantWarnSub string // substring to find in warnings
	}{
		{
			name: "nil runtime",
			setupRT: func() *runtime.Runtime {
				return nil
			},
			wantStatus:  http.StatusOK,
			wantHealthy: true,
			wantWarning: true,
			wantWarnSub: "runtime not available",
		},
		{
			name: "non-nil runtime default unknown health",
			setupRT: func() *runtime.Runtime {
				return runtime.New()
			},
			wantStatus:  http.StatusOK,
			wantHealthy: true,
			wantWarning: false,
		},
		{
			name: "non-nil runtime healthy",
			setupRT: func() *runtime.Runtime {
				rt := runtime.New()
				rt.State().SetHealth(runtime.StatusHealthy)
				return rt
			},
			wantStatus:  http.StatusOK,
			wantHealthy: true,
			wantWarning: false,
		},
		{
			name: "non-nil runtime degraded with components",
			setupRT: func() *runtime.Runtime {
				rt := runtime.New()
				rt.State().SetComponentStatus("cache", runtime.StatusDegraded, "Cache is degraded")
				rt.State().SetComponentStatus("memory", runtime.StatusHealthy, "")
				// recomputeHealth sets health to degraded because cache is degraded.
				return rt
			},
			wantStatus:  http.StatusOK,
			wantHealthy: false,
			wantWarning: true,
			wantWarnSub: "cache: Cache is degraded",
		},
		{
			name: "non-nil runtime multiple degraded components",
			setupRT: func() *runtime.Runtime {
				rt := runtime.New()
				rt.State().SetComponentStatus("cache", runtime.StatusDegraded, "Cache is slow")
				rt.State().SetComponentStatus("plugins", runtime.StatusDegraded, "Plugin loader crashed")
				return rt
			},
			wantStatus:  http.StatusOK,
			wantHealthy: false,
			wantWarning: true,
			wantWarnSub: "plugins: Plugin loader crashed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := tt.setupRT()
			h := handler.NewRuntimeHandler(rt)

			req := httptest.NewRequest("GET", "/v1/health", nil)
			w := httptest.NewRecorder()

			h.Health(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
			}

			var resp handler.HealthResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if resp.Healthy != tt.wantHealthy {
				t.Errorf("expected healthy=%v, got healthy=%v", tt.wantHealthy, resp.Healthy)
			}

			if tt.wantWarning {
				if len(resp.Warnings) == 0 {
					t.Errorf("expected at least one warning, got none")
				} else {
					found := false
					for _, w := range resp.Warnings {
						if strings.Contains(w, tt.wantWarnSub) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected warning containing '%s', got warnings: %v", tt.wantWarnSub, resp.Warnings)
					}
				}
			} else {
				if len(resp.Warnings) > 0 {
					t.Errorf("expected no warnings, got %d: %v", len(resp.Warnings), resp.Warnings)
				}
			}
		})
	}
}

// TestRuntimeHandlerHealthStatusOk verifies Health always returns 200.
func TestRuntimeHandlerHealthStatusOk(t *testing.T) {
	cases := []struct {
		name string
		rt   *runtime.Runtime
	}{
		{"nil", nil},
		{"default", runtime.New()},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := handler.NewRuntimeHandler(c.rt)
			req := httptest.NewRequest("GET", "/v1/health", nil)
			w := httptest.NewRecorder()

			h.Health(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}
		})
	}
}

// =============================================================================
// StatusStream Handler (SSE)
// =============================================================================

// TestRuntimeHandlerStatusStreamInitialEvent verifies that the SSE
// StatusStream endpoint sends an initial status event and sets the
// correct content-type headers.
func TestRuntimeHandlerStatusStreamInitialEvent(t *testing.T) {
	rt := runtime.New(runtime.WithConfig(runtime.RuntimeConfig{
		Version: "3.0.0-sse",
	}))
	h := handler.NewRuntimeHandler(rt)

	tw := stream.NewSSETestWriter()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", "/v1/status/stream", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.StatusStream(tw, req)
	}()

	// Give the handler goroutine time to write the initial SSE event
	// and enter the select loop.
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop the event loop.
	cancel()

	// Wait for the handler to return.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return within 2s after context cancellation")
	}

	body := tw.Body().String()

	// Content-Type must be text/event-stream.
	ct := tw.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got '%s'", ct)
	}

	// Should contain at least one "status" event.
	if !strings.Contains(body, "status") {
		t.Errorf("expected 'status' event in SSE body, got: %s", body)
	}

	// Body should be non-empty.
	if body == "" {
		t.Error("expected non-empty SSE body")
	}
}

// TestRuntimeHandlerStatusStreamNilRT verifies that StatusStream does
// not write any events when the runtime is nil.
func TestRuntimeHandlerStatusStreamNilRT(t *testing.T) {
	h := handler.NewRuntimeHandler(nil)

	tw := stream.NewSSETestWriter()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", "/v1/status/stream", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.StatusStream(tw, req)
	}()

	// Give time to enter the loop.
	time.Sleep(50 * time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return within 2s after context cancellation")
	}

	body := tw.Body().String()

	// Content-Type should still be set by NewSSEWriter.
	ct := tw.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got '%s'", ct)
	}

	// With nil rt, sendStatus returns immediately — no events written.
	// The only output may be empty or just the SSE header.
	if strings.Contains(body, "status") {
		t.Errorf("expected no 'status' event with nil runtime, got: %s", body)
	}
}

// TestRuntimeHandlerStatusStreamCancel verifies that cancelling the
// request context terminates the SSE stream gracefully.
func TestRuntimeHandlerStatusStreamCancel(t *testing.T) {
	rt := runtime.New()
	h := handler.NewRuntimeHandler(rt)

	tw := stream.NewSSETestWriter()

	ctx, cancel := context.WithCancel(context.Background())

	req := httptest.NewRequest("GET", "/v1/status/stream", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.StatusStream(tw, req)
	}()

	// Allow the initial event to be sent, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Handler should return promptly after cancellation.
	select {
	case <-done:
		// Success: handler returned.
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return within 3s after cancellation")
	}
}

// TestRuntimeHandlerStatusStreamHeaders verifies that the SSE response
// headers are set correctly by NewSSEWriter.
func TestRuntimeHandlerStatusStreamHeaders(t *testing.T) {
	rt := runtime.New()
	h := handler.NewRuntimeHandler(rt)

	tw := stream.NewSSETestWriter()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", "/v1/status/stream", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.StatusStream(tw, req)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return within 2s")
	}

	// Verify SSE-specific headers.
	if ct := tw.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got '%s'", ct)
	}
	if cc := tw.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control 'no-cache', got '%s'", cc)
	}
	if conn := tw.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("expected Connection 'keep-alive', got '%s'", conn)
	}
}

// =============================================================================
// Remote daemon (API-only mode, FASE 2 — DDNA-2026-08-07-001)
// =============================================================================

// startRemoteDaemon boots a REAL gRPC RuntimeService server on an ephemeral
// loopback port, mirroring the standalone `cosca runtime start` daemon
// (FASE 1). Returns the listen address and a stop function.
func startRemoteDaemon(t *testing.T, rt *runtime.Runtime) (addr string, stop func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral loopback port: %v", err)
	}

	srv := grpc.NewServer()
	cospb.RegisterRuntimeServiceServer(srv, grpcserver.NewRuntimeServiceServer(rt))

	go func() {
		_ = srv.Serve(lis)
	}()

	return lis.Addr().String(), func() { srv.Stop() }
}

// closedLoopbackAddr returns a loopback address that is very likely CLOSED:
// bind an ephemeral port and release it immediately (immediate ECONNREFUSED).
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

// TestRuntimeHandlerRemote_Status verifies that when the handler is wired to
// a remote daemon client, Status reports the DAEMON's real state (remote
// source wins over the in-process runtime).
func TestRuntimeHandlerRemote_Status(t *testing.T) {
	daemonRT := runtime.New(runtime.WithConfig(runtime.RuntimeConfig{
		Version: "9.9.9-remote",
	}))
	addr, stop := startRemoteDaemon(t, daemonRT)
	defer stop()

	client := grpcclient.NewRuntimeClient(addr)
	defer client.Close()

	h := handler.NewRuntimeHandlerWithClient(nil, client)

	req := httptest.NewRequest("GET", "/v1/status", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.StatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Version != "9.9.9-remote" {
		t.Errorf("expected version from daemon '9.9.9-remote', got '%s'", resp.Version)
	}
	// Fresh daemon runtime: uninitialized — never the local "unknown"
	// placeholder nor the honest "stopped" fallback.
	if resp.State == "" || resp.State == "unknown" || resp.State == "stopped" {
		t.Errorf("expected real daemon state, got '%s'", resp.State)
	}
	if resp.Health == "" {
		t.Error("expected non-empty health from daemon")
	}
}

// TestRuntimeHandlerRemote_Health verifies Health reports the daemon's real
// health (healthy=true for a fresh runtime with unknown health).
func TestRuntimeHandlerRemote_Health(t *testing.T) {
	daemonRT := runtime.New()
	addr, stop := startRemoteDaemon(t, daemonRT)
	defer stop()

	client := grpcclient.NewRuntimeClient(addr)
	defer client.Close()

	h := handler.NewRuntimeHandlerWithClient(nil, client)

	req := httptest.NewRequest("GET", "/v1/health", nil)
	w := httptest.NewRecorder()
	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Healthy {
		t.Errorf("expected healthy=true from daemon, got %v (warnings: %v)", resp.Healthy, resp.Warnings)
	}
	if len(resp.Warnings) != 0 {
		t.Errorf("expected no warnings from a fresh daemon, got %v", resp.Warnings)
	}
}

// TestRuntimeHandlerRemote_DaemonDown verifies the HONEST fallback when the
// client is set but the daemon is not reachable: Status reports state
// "stopped" with the daemon component, Health reports healthy=false with a
// clear "não está ativo" warning. Never a generic placeholder.
func TestRuntimeHandlerRemote_DaemonDown(t *testing.T) {
	addr := closedLoopbackAddr(t)
	client := grpcclient.NewRuntimeClient(addr)
	defer client.Close()

	h := handler.NewRuntimeHandlerWithClient(nil, client)

	// ── Status ──────────────────────────────────────────────────────────
	req := httptest.NewRequest("GET", "/v1/status", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var statusResp handler.StatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("failed to unmarshal status: %v", err)
	}
	if statusResp.State != "stopped" {
		t.Errorf("expected state 'stopped', got '%s'", statusResp.State)
	}
	if statusResp.Health != "unavailable" {
		t.Errorf("expected health 'unavailable', got '%s'", statusResp.Health)
	}
	daemonComp, ok := statusResp.Components["daemon"]
	if !ok {
		t.Fatalf("expected 'daemon' component in response, got %v", statusResp.Components)
	}
	if !strings.Contains(daemonComp.Message, "não está ativo") {
		t.Errorf("expected daemon message containing 'não está ativo', got '%s'", daemonComp.Message)
	}

	// ── Health ──────────────────────────────────────────────────────────
	req = httptest.NewRequest("GET", "/v1/health", nil)
	w = httptest.NewRecorder()
	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var healthResp handler.HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &healthResp); err != nil {
		t.Fatalf("failed to unmarshal health: %v", err)
	}
	if healthResp.Healthy {
		t.Error("expected healthy=false when daemon is down")
	}
	found := false
	for _, warn := range healthResp.Warnings {
		if strings.Contains(warn, "não está ativo") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning containing 'não está ativo', got %v", healthResp.Warnings)
	}
}

// TestRuntimeHandlerRemote_StatusStream verifies the SSE StatusStream queries
// the remote daemon: an initial "status" event carrying the daemon's real
// state is emitted (the local rt is nil, so any event can only come from the
// remote source).
func TestRuntimeHandlerRemote_StatusStream(t *testing.T) {
	daemonRT := runtime.New(runtime.WithConfig(runtime.RuntimeConfig{
		Version: "3.1.0-stream",
	}))
	addr, stop := startRemoteDaemon(t, daemonRT)
	defer stop()

	client := grpcclient.NewRuntimeClient(addr)
	defer client.Close()

	h := handler.NewRuntimeHandlerWithClient(nil, client)

	tw := stream.NewSSETestWriter()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", "/v1/status/stream", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.StatusStream(tw, req)
	}()

	// Give the handler time to perform the first gRPC round-trip and write
	// the initial SSE events, then stop the stream.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return within 3s after context cancellation")
	}

	body := tw.Body().String()

	if ct := tw.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got '%s'", ct)
	}
	if !strings.Contains(body, "status") {
		t.Errorf("expected 'status' event from remote daemon in SSE body, got: %s", body)
	}
	// Fresh daemon runtime state — proves the event came from the daemon
	// and not from a local source (rt is nil).
	if !strings.Contains(body, "uninitialized") {
		t.Errorf("expected daemon state 'uninitialized' in SSE body, got: %s", body)
	}
}
