package cosca

import (
	"net/http"
	"strings"
	"testing"
)

// =============================================================================
// MemorySDK Snapshots — comportamento honesto
//
// O servidor REST atual NÃO implementa /v1/memory/snapshots (nem restore).
// Os métodos ListSnapshots/CreateSnapshot/RestoreSnapshot retornam erro claro
// em vez de chamar endpoint inexistente. Estes testes validam esse contrato.
// Quando a superfície REST expuser snapshots (internal/memory tem o
// SnapshotManager no motor), os métodos voltam a ser funcionais e estes
// testes devem ser reescritos contra o endpoint real.
// =============================================================================

func TestMemorySDK_ListSnapshots_NotImplemented(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})
	_, err := c.Memory.ListSnapshots()
	if err == nil {
		t.Fatal("expected error: endpoint /v1/memory/snapshots not implemented")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got %q", err.Error())
	}
}

func TestMemorySDK_CreateSnapshot_NotImplemented(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})
	_, err := c.Memory.CreateSnapshot("any-name")
	if err == nil {
		t.Fatal("expected error: endpoint /v1/memory/snapshots not implemented")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got %q", err.Error())
	}
}

func TestMemorySDK_RestoreSnapshot_NotImplemented(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})
	err := c.Memory.RestoreSnapshot("any-id")
	if err == nil {
		t.Fatal("expected error: endpoint /v1/memory/snapshots/{id}/restore not implemented")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got %q", err.Error())
	}
}

// =============================================================================
// Memory Type Constants
// =============================================================================

func TestMemoryTypeConstants(t *testing.T) {
	t.Parallel()

	if MemoryTypeEphemeral != "ephemeral" {
		t.Errorf("expected MemoryTypeEphemeral 'ephemeral', got %q", MemoryTypeEphemeral)
	}
	if MemoryTypePersistent != "persistent" {
		t.Errorf("expected MemoryTypePersistent 'persistent', got %q", MemoryTypePersistent)
	}
	if MemoryTypeWorking != "working" {
		t.Errorf("expected MemoryTypeWorking 'working', got %q", MemoryTypeWorking)
	}
	if MemoryTypeLongTerm != "long_term" {
		t.Errorf("expected MemoryTypeLongTerm 'long_term', got %q", MemoryTypeLongTerm)
	}
	if MemoryTypeSemantic != "semantic" {
		t.Errorf("expected MemoryTypeSemantic 'semantic', got %q", MemoryTypeSemantic)
	}
	if MemoryTypeEpisodic != "episodic" {
		t.Errorf("expected MemoryTypeEpisodic 'episodic', got %q", MemoryTypeEpisodic)
	}
}
