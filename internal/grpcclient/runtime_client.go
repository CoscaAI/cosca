// Package grpcclient provides gRPC clients for the Cosca daemon services.
//
// FASE 2 (DDNA-2026-08-07-001): the REST server in API-only mode
// (`cosca serve --api-only`) does not run an internal daemon — it consults
// the standalone runtime daemon (`cosca runtime start`, which listens on
// 127.0.0.1:14123 since FASE 1) through the RuntimeService client defined
// here.
package grpcclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DefaultRuntimeAddr is the loopback address of the standalone runtime
// daemon (`cosca runtime start` — FASE 1).
const DefaultRuntimeAddr = "127.0.0.1:14123"

// DefaultRPCTimeout bounds every RPC so the caller NEVER blocks on an
// unreachable daemon. Connection-refused on loopback fails in milliseconds;
// this only guards against a hung or unresponsive daemon.
const DefaultRPCTimeout = 2 * time.Second

// RuntimeClient is a small gRPC client for the cosca.v1.RuntimeService
// (Status/Health) exposed by the standalone runtime daemon. It is safe for
// concurrent use: the connection is dialed lazily on the first RPC and
// reused afterwards. Every method returns a clear error when the daemon is
// not reachable, so the REST server can answer honestly without blocking.
type RuntimeClient struct {
	addr string

	mu   sync.Mutex
	conn *grpc.ClientConn
	svc  cospb.RuntimeServiceClient
}

// NewRuntimeClient creates a client for the given daemon address. An empty
// address defaults to DefaultRuntimeAddr.
func NewRuntimeClient(addr string) *RuntimeClient {
	if addr == "" {
		addr = DefaultRuntimeAddr
	}
	return &RuntimeClient{addr: addr}
}

// Addr returns the daemon address this client talks to.
func (c *RuntimeClient) Addr() string { return c.addr }

// service returns the lazily-dialed RuntimeService client. grpc.NewClient is
// non-blocking: the real connection is established on the first RPC, so an
// unreachable daemon surfaces as an RPC error — mapped to a clear message by
// the callers — never as a dial hang.
func (c *RuntimeClient) service() cospb.RuntimeServiceClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.svc == nil {
		conn, err := grpc.NewClient(c.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil
		}
		c.conn = conn
		c.svc = cospb.NewRuntimeServiceClient(conn)
	}
	return c.svc
}

// daemonDownError wraps an RPC error with the canonical "daemon not active"
// message, so callers can surface it verbatim.
func (c *RuntimeClient) daemonDownError(err error) error {
	return fmt.Errorf("runtime daemon não está ativo em %s: %w", c.addr, err)
}

// Status returns the daemon's runtime status (state, health, uptime,
// version, components). Returns an error when the daemon is not reachable.
func (c *RuntimeClient) Status(ctx context.Context) (*cospb.StatusResponse, error) {
	svc := c.service()
	if svc == nil {
		return nil, c.daemonDownError(fmt.Errorf("invalid daemon address %q", c.addr))
	}
	rctx, cancel := context.WithTimeout(ctx, DefaultRPCTimeout)
	defer cancel()
	resp, err := svc.Status(rctx, &cospb.StatusRequest{})
	if err != nil {
		return nil, c.daemonDownError(err)
	}
	return resp, nil
}

// Health returns the daemon's health check (healthy flag + warnings).
// Returns an error when the daemon is not reachable.
func (c *RuntimeClient) Health(ctx context.Context) (*cospb.HealthResponse, error) {
	svc := c.service()
	if svc == nil {
		return nil, c.daemonDownError(fmt.Errorf("invalid daemon address %q", c.addr))
	}
	rctx, cancel := context.WithTimeout(ctx, DefaultRPCTimeout)
	defer cancel()
	resp, err := svc.Health(rctx, &cospb.HealthRequest{})
	if err != nil {
		return nil, c.daemonDownError(err)
	}
	return resp, nil
}

// Close releases the underlying connection, if any. Safe to call multiple
// times and from any goroutine.
func (c *RuntimeClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.svc = nil
	return err
}
