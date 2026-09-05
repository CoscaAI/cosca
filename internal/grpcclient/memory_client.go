// Package grpcclient provides gRPC clients for the Cosca daemon services.
//
// FASE 2 (DDNA-2026-08-07-001): MemoryClient delegates memory operations
// (store, search, get, delete, promote, stats) to the standalone runtime
// daemon via the MemoryService gRPC, eliminating the duplicated Memory
// Engine in the serve process.
package grpcclient

import (
	"context"
	"fmt"
	"sync"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// MemoryClient is a gRPC client for cosca.v1.MemoryService
// (Store/Search/Get/Delete/Promote/Stats). It shares the same connection
// lifecycle as RuntimeClient: lazy dial, non-blocking, safe for concurrent
// use. Every method returns a clear error when the daemon is unreachable.
type MemoryClient struct {
	addr string

	mu   sync.Mutex
	conn *grpc.ClientConn
	svc  cospb.MemoryServiceClient
}

// NewMemoryClient creates a client for the given daemon address.
// An empty address defaults to DefaultRuntimeAddr.
func NewMemoryClient(addr string) *MemoryClient {
	if addr == "" {
		addr = DefaultRuntimeAddr
	}
	return &MemoryClient{addr: addr}
}

// Addr returns the daemon address this client talks to.
func (c *MemoryClient) Addr() string { return c.addr }

// service returns the lazily-dialed MemoryService client.
func (c *MemoryClient) service() cospb.MemoryServiceClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.svc == nil {
		conn, err := grpc.NewClient(c.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil
		}
		c.conn = conn
		c.svc = cospb.NewMemoryServiceClient(conn)
	}
	return c.svc
}

func (c *MemoryClient) daemonDownError(err error) error {
	return fmt.Errorf("runtime daemon não está ativo em %s: %w", c.addr, err)
}

// Store persists a memory record via the runtime daemon.
func (c *MemoryClient) Store(ctx context.Context, req *cospb.StoreRequest) (*cospb.StoreResponse, error) {
	svc := c.service()
	if svc == nil {
		return nil, c.daemonDownError(fmt.Errorf("invalid daemon address %q", c.addr))
	}
	rctx, cancel := context.WithTimeout(ctx, DefaultRPCTimeout)
	defer cancel()
	resp, err := svc.Store(rctx, req)
	if err != nil {
		return nil, c.daemonDownError(err)
	}
	return resp, nil
}

// Search queries memory across layers via the runtime daemon.
func (c *MemoryClient) Search(ctx context.Context, req *cospb.MemorySearchRequest) (*cospb.MemorySearchResponse, error) {
	svc := c.service()
	if svc == nil {
		return nil, c.daemonDownError(fmt.Errorf("invalid daemon address %q", c.addr))
	}
	rctx, cancel := context.WithTimeout(ctx, DefaultRPCTimeout)
	defer cancel()
	resp, err := svc.Search(rctx, req)
	if err != nil {
		return nil, c.daemonDownError(err)
	}
	return resp, nil
}

// Get retrieves a single memory record via the runtime daemon.
func (c *MemoryClient) Get(ctx context.Context, req *cospb.GetRequest) (*cospb.GetResponse, error) {
	svc := c.service()
	if svc == nil {
		return nil, c.daemonDownError(fmt.Errorf("invalid daemon address %q", c.addr))
	}
	rctx, cancel := context.WithTimeout(ctx, DefaultRPCTimeout)
	defer cancel()
	resp, err := svc.Get(rctx, req)
	if err != nil {
		return nil, c.daemonDownError(err)
	}
	return resp, nil
}

// Delete removes a memory record via the runtime daemon.
func (c *MemoryClient) Delete(ctx context.Context, req *cospb.DeleteRequest) (*cospb.DeleteResponse, error) {
	svc := c.service()
	if svc == nil {
		return nil, c.daemonDownError(fmt.Errorf("invalid daemon address %q", c.addr))
	}
	rctx, cancel := context.WithTimeout(ctx, DefaultRPCTimeout)
	defer cancel()
	resp, err := svc.Delete(rctx, req)
	if err != nil {
		return nil, c.daemonDownError(err)
	}
	return resp, nil
}

// Promote moves a memory record between layers via the runtime daemon.
func (c *MemoryClient) Promote(ctx context.Context, req *cospb.PromoteRequest) (*cospb.PromoteResponse, error) {
	svc := c.service()
	if svc == nil {
		return nil, c.daemonDownError(fmt.Errorf("invalid daemon address %q", c.addr))
	}
	rctx, cancel := context.WithTimeout(ctx, DefaultRPCTimeout)
	defer cancel()
	resp, err := svc.Promote(rctx, req)
	if err != nil {
		return nil, c.daemonDownError(err)
	}
	return resp, nil
}

// Stats returns per-layer memory statistics via the runtime daemon.
func (c *MemoryClient) Stats(ctx context.Context, req *cospb.MemoryStatsRequest) (*cospb.MemoryStatsResponse, error) {
	svc := c.service()
	if svc == nil {
		return nil, c.daemonDownError(fmt.Errorf("invalid daemon address %q", c.addr))
	}
	rctx, cancel := context.WithTimeout(ctx, DefaultRPCTimeout)
	defer cancel()
	resp, err := svc.Stats(rctx, req)
	if err != nil {
		return nil, c.daemonDownError(err)
	}
	return resp, nil
}

// Close releases the underlying connection, if any.
func (c *MemoryClient) Close() error {
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
