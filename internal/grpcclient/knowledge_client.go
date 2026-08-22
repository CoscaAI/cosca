// Package grpcclient provides gRPC clients for the Cosca daemon services.
//
// FASE 2 (DDNA-2026-08-07-001): KnowledgeClient delegates knowledge
// operations (search, index, stats, sync) to the standalone runtime daemon
// via the KnowledgeService gRPC, eliminating the duplicated Knowledge Engine
// in the serve process.
package grpcclient

import (
	"context"
	"fmt"
	"sync"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// KnowledgeClient is a gRPC client for cosca.v1.KnowledgeService
// (Search/Index/Stats/Sync). It shares the same connection lifecycle as
// RuntimeClient: lazy dial, non-blocking, safe for concurrent use.
// Every method returns a clear error when the daemon is unreachable, so
// the REST server can answer honestly without blocking.
type KnowledgeClient struct {
	addr string

	mu   sync.Mutex
	conn *grpc.ClientConn
	svc  cospb.KnowledgeServiceClient
}

// NewKnowledgeClient creates a client for the given daemon address.
// An empty address defaults to DefaultRuntimeAddr.
func NewKnowledgeClient(addr string) *KnowledgeClient {
	if addr == "" {
		addr = DefaultRuntimeAddr
	}
	return &KnowledgeClient{addr: addr}
}

// Addr returns the daemon address this client talks to.
func (c *KnowledgeClient) Addr() string { return c.addr }

// service returns the lazily-dialed KnowledgeService client.
func (c *KnowledgeClient) service() cospb.KnowledgeServiceClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.svc == nil {
		conn, err := grpc.NewClient(c.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil
		}
		c.conn = conn
		c.svc = cospb.NewKnowledgeServiceClient(conn)
	}
	return c.svc
}

func (c *KnowledgeClient) daemonDownError(err error) error {
	return fmt.Errorf("runtime daemon não está ativo em %s: %w", c.addr, err)
}

// Search performs a hybrid knowledge search via the runtime daemon.
func (c *KnowledgeClient) Search(ctx context.Context, req *cospb.SearchRequest) (*cospb.SearchResponse, error) {
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

// Index indexes a document or directory tree via the runtime daemon.
func (c *KnowledgeClient) Index(ctx context.Context, req *cospb.IndexRequest) (*cospb.IndexResponse, error) {
	svc := c.service()
	if svc == nil {
		return nil, c.daemonDownError(fmt.Errorf("invalid daemon address %q", c.addr))
	}
	rctx, cancel := context.WithTimeout(ctx, DefaultRPCTimeout)
	defer cancel()
	resp, err := svc.Index(rctx, req)
	if err != nil {
		return nil, c.daemonDownError(err)
	}
	return resp, nil
}

// Stats returns knowledge engine statistics via the runtime daemon.
func (c *KnowledgeClient) Stats(ctx context.Context, req *cospb.StatsRequest) (*cospb.StatsResponse, error) {
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

// Sync triggers a filesystem sync via the runtime daemon.
func (c *KnowledgeClient) Sync(ctx context.Context, req *cospb.SyncRequest) (*cospb.SyncResponse, error) {
	svc := c.service()
	if svc == nil {
		return nil, c.daemonDownError(fmt.Errorf("invalid daemon address %q", c.addr))
	}
	rctx, cancel := context.WithTimeout(ctx, DefaultRPCTimeout)
	defer cancel()
	resp, err := svc.Sync(rctx, req)
	if err != nil {
		return nil, c.daemonDownError(err)
	}
	return resp, nil
}

// Close releases the underlying connection, if any.
func (c *KnowledgeClient) Close() error {
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
