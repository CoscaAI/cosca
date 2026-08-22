// Package memory provides the memory engine for the Cosca platform,
// supporting multi-layer storage, retrieval, promotion, and pruning of
// agent memory records with configurable TTL and persistence.
package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// LayerStats contains statistics for a memory layer.
type LayerStats struct {
	Name            string `json:"name"`
	Count           int    `json:"count"`
	TotalSize       int    `json:"total_size"`
	HighestPriority int    `json:"highest_priority"`
}

// LayerDefinition defines a memory layer's properties.
type LayerDefinition struct {
	Layer    MemoryLayer `json:"layer"`
	Priority int         `json:"priority"`
	MaxSize  int         `json:"max_size"`
	Persist  bool        `json:"persist"`
	TTL      string      `json:"ttl,omitempty"`
}

// LayerManager manages memory layer definitions and priorities.
type LayerManager struct {
	mu     sync.RWMutex
	layers map[MemoryLayer]LayerDefinition
	order  []MemoryLayer
}

// NewLayerManager creates a new layer manager with default layers.
func NewLayerManager() *LayerManager {
	lm := &LayerManager{
		layers: make(map[MemoryLayer]LayerDefinition),
	}

	// Register default layers in priority order.
	// Tier médio (7d) é o DEFAULT para tudo que persiste: o Don determinou
	// que todo o conhecimento entra como médio e o longo (LayerLong, 1 ano)
	// só recebe o que for promovido manualmente — "o longo a gente vai ver o
	// que coloca" (L338).
	defaultLayers := []LayerDefinition{
		{Layer: LayerTemp, Priority: 10, MaxSize: 100, Persist: false, TTL: "1h"},
		{Layer: LayerSession, Priority: 20, MaxSize: 500, Persist: false, TTL: "24h"},
		{Layer: LayerProject, Priority: 30, MaxSize: 1000, Persist: true, TTL: "168h"},
		{Layer: LayerWorkspace, Priority: 40, MaxSize: 2000, Persist: true, TTL: "168h"},
		{Layer: LayerGlobal, Priority: 50, MaxSize: 5000, Persist: true, TTL: "168h"},
		{Layer: LayerLong, Priority: 60, MaxSize: 10000, Persist: true, TTL: "8760h"},
	}

	for _, def := range defaultLayers {
		lm.layers[def.Layer] = def
		lm.order = append(lm.order, def.Layer)
	}

	return lm
}

// RegisterLayer registers a custom layer definition.
func (lm *LayerManager) RegisterLayer(def LayerDefinition) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if _, exists := lm.layers[def.Layer]; exists {
		return fmt.Errorf("layer %s already registered", def.Layer)
	}

	lm.layers[def.Layer] = def
	lm.order = append(lm.order, def.Layer)

	// Keep sorted by priority
	sort.Slice(lm.order, func(i, j int) bool {
		return lm.layers[lm.order[i]].Priority < lm.layers[lm.order[j]].Priority
	})

	return nil
}

// LayerPriority returns the priority of a layer.
func (lm *LayerManager) LayerPriority(layer MemoryLayer) int {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if def, ok := lm.layers[layer]; ok {
		return def.Priority
	}
	return 0
}

// Layers returns all registered layers, sorted by priority.
func (lm *LayerManager) Layers() []MemoryLayer {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make([]MemoryLayer, len(lm.order))
	copy(result, lm.order)
	return result
}

// LayerDefinition returns the definition for a specific layer.
func (lm *LayerManager) LayerDefinition(layer MemoryLayer) (LayerDefinition, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	def, ok := lm.layers[layer]
	return def, ok
}

// IsPersistent checks if a layer persists across sessions.
func (lm *LayerManager) IsPersistent(layer MemoryLayer) bool {
	def, ok := lm.LayerDefinition(layer)
	return ok && def.Persist
}

// CanPromote checks if a record can be promoted from one layer to another.
func (lm *LayerManager) CanPromote(from, to MemoryLayer) bool {
	fromPriority := lm.LayerPriority(from)
	toPriority := lm.LayerPriority(to)
	return toPriority > fromPriority
}

// HigherLayer returns the next higher persistence layer.
func (lm *LayerManager) HigherLayer(current MemoryLayer) (MemoryLayer, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	currentPriority := lm.LayerPriority(current)
	for _, layer := range lm.order {
		if lm.layers[layer].Priority > currentPriority && lm.layers[layer].Persist {
			return layer, true
		}
	}
	return "", false
}

// LowerLayer returns the next lower persistence layer.
func (lm *LayerManager) LowerLayer(current MemoryLayer) (MemoryLayer, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	currentPriority := lm.LayerPriority(current)
	// Iterate in reverse
	for i := len(lm.order) - 1; i >= 0; i-- {
		layer := lm.order[i]
		if lm.layers[layer].Priority < currentPriority {
			return layer, true
		}
	}
	return "", false
}

// Statistics returns statistics for all layers.
func (lm *LayerManager) Statistics(ctx context.Context, stores map[MemoryLayer]Store) map[MemoryLayer]LayerStats {
	stats := make(map[MemoryLayer]LayerStats)

	for _, layer := range lm.Layers() {
		def := lm.layers[layer]
		stats[layer] = LayerStats{
			Name:            string(layer),
			Count:           0,
			HighestPriority: def.Priority,
		}

		if store, ok := stores[layer]; ok {
			if s, err := store.Stats(ctx); err == nil {
				s.HighestPriority = def.Priority
				stats[layer] = s
			}
		}
	}

	return stats
}
