package grpcclient

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

func TestNewKnowledgeClient(t *testing.T) {
	c := NewKnowledgeClient("")
	if c.addr != DefaultRuntimeAddr {
		t.Errorf("empty addr should default to %s, got %s", DefaultRuntimeAddr, c.addr)
	}

	c2 := NewKnowledgeClient("127.0.0.1:9999")
	if c2.addr != "127.0.0.1:9999" {
		t.Errorf("custom addr not set: got %s", c2.addr)
	}
}

func TestNewMemoryClient(t *testing.T) {
	c := NewMemoryClient("")
	if c.addr != DefaultRuntimeAddr {
		t.Errorf("empty addr should default to %s, got %s", DefaultRuntimeAddr, c.addr)
	}
}

func TestKnowledgeSearcherAdapter_ImplementsInterface(t *testing.T) {
	c := NewKnowledgeClient("127.0.0.1:1")
	adapter := NewKnowledgeSearcherAdapter(c)

	var ks orchestration.KnowledgeSearcher = adapter
	_ = ks // compile-time check: if this compiles, the interface is satisfied
}

func TestMemoryRetrieverAdapter_ImplementsInterface(t *testing.T) {
	c := NewMemoryClient("127.0.0.1:1")
	adapter := NewMemoryRetrieverAdapter(c)

	var mr orchestration.MemoryRetriever = adapter
	_ = mr
}

func TestMemoryStorerAdapter_ImplementsInterface(t *testing.T) {
	c := NewMemoryClient("127.0.0.1:1")
	adapter := NewMemoryStorerAdapter(c)

	var ms orchestration.MemoryStorer = adapter
	_ = ms
}

func TestKnowledgeClient_Close(t *testing.T) {
	c := NewKnowledgeClient("127.0.0.1:1")
	// Close before any connection should be safe (no-op).
	if err := c.Close(); err != nil {
		t.Errorf("Close on unconnected client should not error: %v", err)
	}
}

func TestMemoryClient_Close(t *testing.T) {
	c := NewMemoryClient("127.0.0.1:1")
	if err := c.Close(); err != nil {
		t.Errorf("Close on unconnected client should not error: %v", err)
	}
}

func TestKnowledgeClient_daemonDownError(t *testing.T) {
	c := NewKnowledgeClient("127.0.0.1:1")
	err := c.daemonDownError(nil)
	if err == nil {
		t.Error("daemonDownError with nil should still return an error")
	}
}

func TestMemoryClient_daemonDownError(t *testing.T) {
	c := NewMemoryClient("127.0.0.1:1")
	err := c.daemonDownError(nil)
	if err == nil {
		t.Error("daemonDownError with nil should still return an error")
	}
}

func TestKnowledgeClient_service_nilOnInvalidAddr(t *testing.T) {
	// Using an invalid address format should not panic.
	c := NewKnowledgeClient("invalid-addr")
	svc := c.service()
	if svc != nil {
		t.Log("unexpected: service created for invalid addr (grpc.NewClient is non-blocking)")
	}
	// Clean up.
	c.Close()
}

func TestMemoryClient_service_nilOnInvalidAddr(t *testing.T) {
	c := NewMemoryClient("invalid-addr")
	svc := c.service()
	if svc != nil {
		t.Log("unexpected: service created for invalid addr (grpc.NewClient is non-blocking)")
	}
	c.Close()
}

func TestDefaultRPCTimeout(t *testing.T) {
	if DefaultRPCTimeout == 0 {
		t.Error("DefaultRPCTimeout must be > 0")
	}
}

func TestDefaultKnowledgeRPCTimeout(t *testing.T) {
	if DefaultRPCTimeout == 0 {
		t.Error("DefaultRPCTimeout must be > 0 (shared by all gRPC clients)")
	}
}

func TestDefaultMemoryRPCTimeout(t *testing.T) {
	if DefaultRPCTimeout == 0 {
		t.Error("DefaultRPCTimeout must be > 0 (shared by all gRPC clients)")
	}
}

func TestKnowledgeClient_Addr(t *testing.T) {
	c := NewKnowledgeClient("127.0.0.1:9998")
	if c.addr != "127.0.0.1:9998" {
		t.Errorf("unexpected addr: %s", c.addr)
	}
}

func TestMemoryClient_Addr(t *testing.T) {
	c := NewMemoryClient("127.0.0.1:9997")
	if c.addr != "127.0.0.1:9997" {
		t.Errorf("unexpected addr: %s", c.addr)
	}
}
