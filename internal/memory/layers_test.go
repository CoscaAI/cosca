package memory

import (
	"context"
	"testing"
)

func TestNewLayerManager(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	if lm == nil {
		t.Fatal("NewLayerManager returned nil")
	}
	layers := lm.Layers()
	if len(layers) != 6 {
		t.Errorf("expected 6 layers, got %d", len(layers))
	}
}

func TestLayerPriorities(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	tests := []struct {
		layer    MemoryLayer
		priority int
	}{
		{LayerTemp, 10},
		{LayerSession, 20},
		{LayerProject, 30},
		{LayerWorkspace, 40},
		{LayerGlobal, 50},
		{LayerLong, 60},
	}
	for _, tt := range tests {
		t.Run(string(tt.layer), func(t *testing.T) {
			got := lm.LayerPriority(tt.layer)
			if got != tt.priority {
				t.Errorf("LayerPriority(%s) = %d, want %d", tt.layer, got, tt.priority)
			}
		})
	}
}

func TestLayersSortedByPriority(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	layers := lm.Layers()
	for i := 1; i < len(layers); i++ {
		prev := lm.LayerPriority(layers[i-1])
		curr := lm.LayerPriority(layers[i])
		if prev > curr {
			t.Errorf("Layers not sorted by priority: %s (%d) > %s (%d)",
				layers[i-1], prev, layers[i], curr)
		}
	}
}

func TestLayerDefinition(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	def, ok := lm.LayerDefinition(LayerGlobal)
	if !ok {
		t.Fatal("LayerGlobal should exist")
	}
	if def.Layer != LayerGlobal {
		t.Errorf("Layer = %s", def.Layer)
	}
	if !def.Persist {
		t.Error("Global should be persistent")
	}
}

func TestLayerDefinitionNotFound(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	_, ok := lm.LayerDefinition(MemoryLayer("nonexistent"))
	if ok {
		t.Error("Should return false for nonexistent layer")
	}
}

func TestIsPersistent(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	if lm.IsPersistent(LayerTemp) {
		t.Error("Temp should not be persistent")
	}
	if !lm.IsPersistent(LayerGlobal) {
		t.Error("Global should be persistent")
	}
	if !lm.IsPersistent(LayerProject) {
		t.Error("Project should be persistent")
	}
}

func TestCanPromote(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	// Session -> Project should be valid (20 -> 30)
	if !lm.CanPromote(LayerSession, LayerProject) {
		t.Error("Session -> Project should be valid")
	}
	// Project -> Session should be invalid (30 -> 20)
	if lm.CanPromote(LayerProject, LayerSession) {
		t.Error("Project -> Session should be invalid")
	}
	// Temp -> Global should be valid
	if !lm.CanPromote(LayerTemp, LayerGlobal) {
		t.Error("Temp -> Global should be valid")
	}
}

func TestRegisterLayer(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	err := lm.RegisterLayer(LayerDefinition{
		Layer:    MemoryLayer("custom"),
		Priority: 25,
		MaxSize:  300,
		Persist:  true,
	})
	if err != nil {
		t.Fatalf("RegisterLayer error: %v", err)
	}
	if lm.LayerPriority(MemoryLayer("custom")) != 25 {
		t.Errorf("custom priority = %d, want 25", lm.LayerPriority(MemoryLayer("custom")))
	}
}

func TestRegisterLayerDuplicate(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	err := lm.RegisterLayer(LayerDefinition{Layer: LayerGlobal, Priority: 99})
	if err == nil {
		t.Error("Should error on duplicate registration")
	}
}

func TestHigherLayer(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	higher, ok := lm.HigherLayer(LayerSession)
	if !ok {
		t.Fatal("Should find higher layer")
	}
	if higher != LayerProject {
		t.Errorf("Higher from Session should be Project, got %s", higher)
	}
}

func TestHigherLayerFromGlobal(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	// Global (50) has a higher persistent layer: Long (60).
	higher, ok := lm.HigherLayer(LayerGlobal)
	if !ok {
		t.Fatal("Should find higher layer from Global (Long)")
	}
	if higher != LayerLong {
		t.Errorf("Higher from Global should be Long, got %s", higher)
	}
	// Long is the top layer — nothing above it.
	_, ok = lm.HigherLayer(LayerLong)
	if ok {
		t.Error("No higher layer from Long")
	}
}

func TestLowerLayer(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	lower, ok := lm.LowerLayer(LayerProject)
	if !ok {
		t.Fatal("Should find lower layer")
	}
	if lower != LayerSession {
		t.Errorf("Lower from Project should be Session, got %s", lower)
	}
}

func TestLayerStats(t *testing.T) {
	t.Parallel()
	stats := LayerStats{
		Name:            "test",
		Count:           10,
		TotalSize:       5000,
		HighestPriority: 30,
	}
	if stats.Name != "test" {
		t.Errorf("Name = %q", stats.Name)
	}
	if stats.Count != 10 {
		t.Errorf("Count = %d", stats.Count)
	}
	if stats.TotalSize != 5000 {
		t.Errorf("TotalSize = %d", stats.TotalSize)
	}
}

func TestLayerDefinitionDefaults(t *testing.T) {
	t.Parallel()
	def := LayerDefinition{
		Layer:    MemoryLayer("new"),
		Priority: 15,
		MaxSize:  200,
		Persist:  false,
		TTL:      "2h",
	}
	if def.MaxSize != 200 {
		t.Errorf("MaxSize = %d", def.MaxSize)
	}
	if def.TTL != "2h" {
		t.Errorf("TTL = %q", def.TTL)
	}
}

func TestStatistics(t *testing.T) {
	t.Parallel()
	lm := NewLayerManager()
	stats := lm.Statistics(context.Background(), nil)
	if len(stats) != 6 {
		t.Errorf("expected 6 layer stats, got %d", len(stats))
	}
}
