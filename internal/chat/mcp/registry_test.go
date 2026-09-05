package mcp

import (
	"encoding/json"
	"testing"
)

func TestRegistry_NamespacingResolvesConflict(t *testing.T) {
	r := NewRegistry()
	_ = r.Register("aws", []ToolInfo{{Name: "list_buckets", Description: "s3"}})
	_ = r.Register("git", []ToolInfo{{Name: "list_buckets", Description: "git"}}) // mesmo nome, server diferente

	// Conflito resolvido por namespace: "aws.list_buckets" != "git.list_buckets".
	if len(r.ToolNames()) != 2 {
		t.Fatalf("esperava 2 tools namespaced, got %d", len(r.ToolNames()))
	}
	s, tool, err := r.ResolveCall("aws.list_buckets")
	if err != nil || s != "aws" || tool != "list_buckets" {
		t.Fatalf("resolve aws.list_buckets: %s/%s %v", s, tool, err)
	}
}

func TestRegistry_DefaultDeny(t *testing.T) {
	r := NewRegistry()
	_ = r.Register("aws", []ToolInfo{{Name: "list_buckets"}})

	// Sem namespace → deny (I8: nunca chama por nome pelado).
	if _, _, err := r.ResolveCall("list_buckets"); err == nil {
		t.Fatal("tool sem namespace deve ser negada (default-deny)")
	}
	// Servidor não registrado → deny.
	if _, _, err := r.ResolveCall("google.search"); err == nil {
		t.Fatal("servidor não registrado deve ser negado")
	}
	// Tool inexistente no server → deny.
	if _, _, err := r.ResolveCall("aws.nao-existe"); err == nil {
		t.Fatal("tool inexistente deve ser negada")
	}
}

func TestRegistry_RegisterFailClosed(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("", nil); err == nil {
		t.Fatal("nome de servidor vazio deve dar erro (fail-closed)")
	}
}

func TestRegistry_DeterministicOrder(t *testing.T) {
	r := NewRegistry()
	_ = r.Register("zeta", []ToolInfo{{Name: "b"}, {Name: "a"}})
	_ = r.Register("alpha", []ToolInfo{{Name: "c"}})
	// Servers e toolNames ordenados (determinístico I1).
	got := r.ToolNames()
	// alpha.c vem antes de zeta.a/b (ordenado).
	if got[0] != "alpha.c" || got[len(got)-1] != "zeta.b" {
		t.Fatalf("ordem deve ser determinística: %v", got)
	}
	if r.Servers()[0] != "alpha" {
		t.Fatalf("servers ordenado: %v", r.Servers())
	}
}

func TestRegistry_ResolveTool(t *testing.T) {
	r := NewRegistry()
	_ = r.Register("db", []ToolInfo{{Name: "query", InputSchema: json.RawMessage(`{"type":"object"}`)}})
	ti, err := r.ResolveTool("db.query")
	if err != nil {
		t.Fatalf("resolve tool: %v", err)
	}
	if ti.Name != "query" || string(ti.InputSchema) != `{"type":"object"}` {
		t.Fatalf("descriptor errado: %+v", ti)
	}
}
