package adapter

import (
	"context"
	"fmt"

	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/results"
)

// MemoryAdapter adapts a memory.MemoryEngine to implement both the
// orchestration.MemoryRetriever and orchestration.MemoryStorer port interfaces.
// It translates between the domain-level memory types (memory.MemoryRecord,
// memory.MemoryLayer, memory.SearchOptions) and the orchestration-level
// projections.
//
// This adapter ensures the memory package remains independent of the
// orchestration layer — all type mapping happens here, in the adapter.
type MemoryAdapter struct {
	engine *memory.MemoryEngine
	// degradedReason, quando não vazio, marca as operações bem-sucedidas como
	// Degraded (paridade D6) — ex.: backend opcional offline/fallback. Não é
	// um error; a operação teve efeito.
	degradedReason string
}

// NewMemoryAdapter creates a new MemoryAdapter backed by the given memory engine.
func NewMemoryAdapter(engine *memory.MemoryEngine) *MemoryAdapter {
	return &MemoryAdapter{engine: engine}
}

// SetDegraded marca o adapter como degradado com o motivo informado. Operações
// bem-sucedidas passam a reportar Degraded=true no envelope (aditivo; as
// assinaturas legadas `(*T, error)` continuam inalteradas).
func (a *MemoryAdapter) SetDegraded(reason string) {
	a.degradedReason = reason
}

// ── MemoryRetriever implementation ────────────────────────────────────────

// Search performs a memory search across one or more layers and returns
// orchestration-level MemoryRecords. It converts orchestration MemorySearchOptions
// to memory.SearchOptions and maps the results back.
func (a *MemoryAdapter) Search(ctx context.Context, query string, opts orchestration.MemorySearchOptions) ([]orchestration.MemoryRecord, error) {
	memOpts := toMemorySearchOptions(opts)

	records, err := a.engine.Search(ctx, query, memOpts)
	if err != nil {
		return nil, fmt.Errorf("memory adapter search: %w", err)
	}

	return toOrchMemoryRecords(records), nil
}

// SearchResult é a forma com envelope (paridade D6) de Search. Aditivo — Search
// continua disponível.
func (a *MemoryAdapter) SearchResult(ctx context.Context, query string, opts orchestration.MemorySearchOptions) *results.Result {
	data, err := a.Search(ctx, query, opts)
	return adapterResult(data, err, a.degradedReason)
}

// Retrieve fetches a specific memory record by ID from the given layer.
// The layer string is converted to a memory.MemoryLayer before delegation.
func (a *MemoryAdapter) Retrieve(ctx context.Context, id, layer string) (*orchestration.MemoryRecord, error) {
	memLayer := memory.MemoryLayer(layer)

	record, err := a.engine.Retrieve(ctx, id, memLayer)
	if err != nil {
		return nil, fmt.Errorf("memory adapter retrieve: %w", err)
	}

	return toOrchMemoryRecord(record), nil
}

// RetrieveResult é a forma com envelope (paridade D6) de Retrieve. Aditivo —
// Retrieve continua disponível.
func (a *MemoryAdapter) RetrieveResult(ctx context.Context, id, layer string) *results.Result {
	data, err := a.Retrieve(ctx, id, layer)
	return adapterResult(data, err, a.degradedReason)
}

// ── MemoryStorer implementation ───────────────────────────────────────────

// Store persists an orchestration-level MemoryRecord into the memory engine.
// The record is mapped to a memory.MemoryRecord before storage, and the
// returned persisted record is mapped back to an orchestration projection.
func (a *MemoryAdapter) Store(ctx context.Context, record orchestration.MemoryRecord) (*orchestration.MemoryRecord, error) {
	memRecord := toMemoryRecord(record)

	persisted, err := a.engine.Store(ctx, *memRecord)
	if err != nil {
		return nil, fmt.Errorf("memory adapter store: %w", err)
	}

	return toOrchMemoryRecord(persisted), nil
}

// StoreResult é a forma com envelope (paridade D6) de Store. Aditivo — Store
// continua disponível.
func (a *MemoryAdapter) StoreResult(ctx context.Context, record orchestration.MemoryRecord) *results.Result {
	data, err := a.Store(ctx, record)
	return adapterResult(data, err, a.degradedReason)
}

// ── Compile-time interface checks ─────────────────────────────────────────

var _ orchestration.MemoryRetriever = (*MemoryAdapter)(nil)
var _ orchestration.MemoryStorer = (*MemoryAdapter)(nil)

// ── Mapping: orchestration → memory ───────────────────────────────────────

// toMemorySearchOptions converts orchestration-level MemorySearchOptions to
// the internal memory.SearchOptions.
func toMemorySearchOptions(o orchestration.MemorySearchOptions) memory.SearchOptions {
	layers := make([]memory.MemoryLayer, 0, len(o.Layers))
	for _, l := range o.Layers {
		layers = append(layers, memory.MemoryLayer(l))
	}

	types := make([]memory.MemoryType, 0, len(o.Types))
	for _, t := range o.Types {
		types = append(types, memory.MemoryType(t))
	}

	return memory.SearchOptions{
		Types:       types,
		Layers:      layers,
		Limit:       o.Limit,
		MinScore:    o.MinScore,
		AgentFilter: memory.AgentFilterFor(o.ReaderAgent),
	}
}

// toMemoryRecord converts an orchestration-level MemoryRecord to a full
// memory.MemoryRecord. This is used when storing records from the orchestration
// engine into the memory subsystem.
func toMemoryRecord(r orchestration.MemoryRecord) *memory.MemoryRecord {
	return &memory.MemoryRecord{
		ID:       r.ID,
		Type:     memory.MemoryType(r.Type),
		Layer:    memory.MemoryLayer(r.Layer),
		Content:  r.Content,
		Metadata: r.Metadata,
		Priority: r.Priority,
	}
}

// ── Mapping: memory → orchestration ───────────────────────────────────────

// toOrchMemoryRecords converts a slice of memory.MemoryRecord to a slice of
// orchestration.MemoryRecord.
func toOrchMemoryRecords(records []memory.MemoryRecord) []orchestration.MemoryRecord {
	result := make([]orchestration.MemoryRecord, 0, len(records))
	for _, r := range records {
		if rec := toOrchMemoryRecord(&r); rec != nil {
			result = append(result, *rec)
		}
	}
	return result
}

// toOrchMemoryRecord converts a memory.MemoryRecord pointer to an
// orchestration.MemoryRecord pointer. Returns nil if the input is nil.
func toOrchMemoryRecord(r *memory.MemoryRecord) *orchestration.MemoryRecord {
	if r == nil {
		return nil
	}
	return &orchestration.MemoryRecord{
		ID:       r.ID,
		Type:     string(r.Type),
		Layer:    string(r.Layer),
		Content:  r.Content,
		Metadata: r.Metadata,
		Priority: r.Priority,
	}
}
