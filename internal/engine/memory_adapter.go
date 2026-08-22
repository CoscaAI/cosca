package engine

import (
	"context"
	"fmt"

	"github.com/CoscaAI/cosca/internal/memory"
)

// Compile-time interface checks.
var _ MemoryRetriever = (*memoryEngineAdapter)(nil)
var _ MemoryStorer = (*memoryEngineAdapter)(nil)

// memoryEngineAdapter wraps memory.MemoryEngine to implement MemoryRetriever
// and MemoryStorer. It translates between engine-level and memory-level types.
type memoryEngineAdapter struct {
	engine *memory.MemoryEngine
}

// NewMemoryEngineAdapter creates a new adapter backed by the given memory engine.
func NewMemoryEngineAdapter(engine *memory.MemoryEngine) *memoryEngineAdapter {
	return &memoryEngineAdapter{engine: engine}
}

// ── MemoryRetriever ──────────────────────────────────────────────────────────

// Search performs a memory search across layers and returns engine-level records.
func (a *memoryEngineAdapter) Search(ctx context.Context, query string, opts MemorySearchOptions) ([]MemoryRecord, error) {
	searchOpts := memory.SearchOptions{
		Types:    toMemoryTypes(opts.Types),
		Layers:   toMemoryLayers(opts.Layers),
		Limit:    opts.Limit,
		MinScore: opts.MinScore,
	}

	results, err := a.engine.Search(ctx, query, searchOpts)
	if err != nil {
		return nil, fmt.Errorf("memory adapter search: %w", err)
	}

	records := make([]MemoryRecord, len(results))
	for i, r := range results {
		records[i] = *toEngineMemoryRecord(&r)
	}
	return records, nil
}

// Retrieve fetches a specific memory record by ID and layer.
func (a *memoryEngineAdapter) Retrieve(ctx context.Context, id, layer string) (*MemoryRecord, error) {
	r, err := a.engine.Retrieve(ctx, id, memory.MemoryLayer(layer))
	if err != nil {
		return nil, fmt.Errorf("memory adapter retrieve: %w", err)
	}
	return toEngineMemoryRecord(r), nil
}

// ── MemoryStorer ──────────────────────────────────────────────────────────────

// Store saves a memory record and returns the persisted record.
func (a *memoryEngineAdapter) Store(ctx context.Context, record MemoryRecord) (*MemoryRecord, error) {
	memRecord := memory.MemoryRecord{
		Type:     memory.MemoryType(record.Type),
		Layer:    memory.MemoryLayer(record.Layer),
		Content:  record.Content,
		Metadata: record.Metadata,
		Priority: record.Priority,
	}

	saved, err := a.engine.Store(ctx, memRecord)
	if err != nil {
		return nil, fmt.Errorf("memory adapter store: %w", err)
	}
	return toEngineMemoryRecord(saved), nil
}

// ── Conversion: memory → engine ──────────────────────────────────────────────

// toEngineMemoryRecord converts a memory.MemoryRecord pointer to an engine.MemoryRecord pointer.
func toEngineMemoryRecord(r *memory.MemoryRecord) *MemoryRecord {
	if r == nil {
		return nil
	}
	return &MemoryRecord{
		ID:       r.ID,
		Type:     string(r.Type),
		Layer:    string(r.Layer),
		Content:  r.Content,
		Metadata: r.Metadata,
		Priority: r.Priority,
	}
}

// ── Conversion: engine → memory ──────────────────────────────────────────────

// toMemoryTypes converts a string slice to a memory.MemoryType slice.
func toMemoryTypes(types []string) []memory.MemoryType {
	if types == nil {
		return nil
	}
	result := make([]memory.MemoryType, len(types))
	for i, t := range types {
		result[i] = memory.MemoryType(t)
	}
	return result
}

// toMemoryLayers converts a string slice to a memory.MemoryLayer slice.
func toMemoryLayers(layers []string) []memory.MemoryLayer {
	if layers == nil {
		return nil
	}
	result := make([]memory.MemoryLayer, len(layers))
	for i, l := range layers {
		result[i] = memory.MemoryLayer(l)
	}
	return result
}
