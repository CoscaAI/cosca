package adapter

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/orchestration"
)

// TestToMemorySearchOptions_ReaderAgent verifies that the ReaderAgent field on
// orchestration.MemorySearchOptions is mapped to the memory engine's
// AgentFilter according to the roles policy (A7): specialists scope to their
// own name; trusted roles (kernel/chiefs/executives) get full visibility.
func TestToMemorySearchOptions_ReaderAgent(t *testing.T) {
	tests := []struct {
		name       string
		reader     string
		wantFilter string
	}{
		{"specialist scopes to own name", "cosca-specialist-backend", "cosca-specialist-backend"},
		{"kernel full visibility", "cosca-kernel", ""},
		{"chief full visibility", "cosca-backend", ""},
		{"executive full visibility", "cosca-ceo", ""},
		{"empty reader full visibility", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toMemorySearchOptions(orchestration.MemorySearchOptions{
				ReaderAgent: tt.reader,
			})
			if got.AgentFilter != tt.wantFilter {
				t.Errorf("toMemorySearchOptions(ReaderAgent=%q).AgentFilter = %q, want %q", tt.reader, got.AgentFilter, tt.wantFilter)
			}
		})
	}
}

// TestMemoryAdapter_Search_AgentScoping is an end-to-end test proving a
// specialist's Search with ReaderAgent set never returns another specialist's
// records. Two records belonging to distinct specialists are stored via the
// adapter; searching as cosca-specialist-backend must return its own record
// but never the frontend specialist's record.
//
// Note: reader and record agents must be real Cosca specialist names
// (cosca-specialist-* prefix) — the roles policy (A7) resolves them to
// RoleSpecialist, otherwise they fall back to RoleChief (full visibility).
func TestMemoryAdapter_Search_AgentScoping(t *testing.T) {
	dir := t.TempDir()

	engine, err := memory.NewEngine(
		memory.WithConfig(memory.EngineConfig{
			DataDir:   dir,
			AutoPrune: false,
		}),
	)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	// Close releases the SQLite index handle; without it the TempDir cleanup
	// fails on Windows ("file already in use by another process").
	t.Cleanup(func() { _ = engine.Close() })

	adapter := NewMemoryAdapter(engine)
	ctx := context.Background()

	reader := "cosca-specialist-backend"
	other := "cosca-specialist-frontend"

	own := orchestration.MemoryRecord{
		Type:     "decision",
		Layer:    "session",
		Content:  "backend deployment plan",
		Metadata: map[string]string{"agent": reader},
	}
	otherRec := orchestration.MemoryRecord{
		Type:     "decision",
		Layer:    "session",
		Content:  "frontend deployment plan",
		Metadata: map[string]string{"agent": other},
	}

	if _, err := adapter.Store(ctx, own); err != nil {
		t.Fatalf("Store own: %v", err)
	}
	if _, err := adapter.Store(ctx, otherRec); err != nil {
		t.Fatalf("Store other: %v", err)
	}

	// As a specialist, the reader must never see the other specialist's record.
	records, err := adapter.Search(ctx, "deployment plan", orchestration.MemorySearchOptions{
		Limit:       10,
		ReaderAgent: reader,
	})
	if err != nil {
		t.Fatalf("Search as %s: %v", reader, err)
	}

	for _, r := range records {
		if agent := r.Metadata["agent"]; agent == other {
			t.Errorf("%s specialist leaked another specialist's record: %q", reader, r.Content)
		}
	}

	// The reader's own record must be present.
	foundOwn := false
	for _, r := range records {
		if r.Metadata["agent"] == reader {
			foundOwn = true
			break
		}
	}
	if !foundOwn {
		t.Errorf("%s specialist expected own record, got: %+v", reader, records)
	}
}
