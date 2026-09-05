package worldmodel

import (
	"testing"
	"time"
)

// TestOrchestratorListerMirrorsMerge: merge espelha a crença com selo (I3/I4).
func TestOrchestratorListerMirrorsMerge(t *testing.T) {
	orch := NewOrchestrator(DefaultOrchestratorConfig())
	now := time.Now()

	orch.mergeEntities([]WorldEntity{
		{ID: "e1", Position: Vec3{X: 1, Y: 0, Z: 0}, Label: "box", LastSeen: now},
	})

	se, ok := orch.Lister().Get("e1")
	if !ok {
		t.Fatal("crença deveria espelhar a entidade")
	}
	if se.Stamp.Trust != TrustKnown {
		t.Fatalf("trust esperado known, got %v", se.Stamp.Trust)
	}
	if !se.Stamp.Source.Authenticated {
		t.Fatal("fonte deveria estar autenticada (I3)")
	}
}

// TestOrchestratorBeliefRetainsStale: a crença (lister) retém a última posição
// conhecida mesmo quando o snapshot de visão poda (saber ≠ ver, I4).
func TestOrchestratorBeliefRetainsStale(t *testing.T) {
	orch := NewOrchestrator(DefaultOrchestratorConfig())
	now := time.Now()

	// e1 visto há 10s (velho) — será podado do snapshot de visão.
	orch.mergeEntities([]WorldEntity{
		{ID: "e1", Position: Vec3{X: 1, Y: 0, Z: 0}, LastSeen: now.Add(-10 * time.Second)},
	})
	orch.mergeEntities([]WorldEntity{
		{ID: "e2", Position: Vec3{X: 2, Y: 0, Z: 0}, LastSeen: now},
	})

	// Snapshot de visão: só e2 (e1 podado).
	state := orch.GetState()
	if len(state.Entities) != 1 || state.Entities[0].ID != "e2" {
		t.Fatalf("visão inesperada: %+v", state.Entities)
	}
	// Crença: e1 ainda conhecido (stale) — o mundo sabe onde esteve (I4).
	if _, ok := orch.Lister().Get("e1"); !ok {
		t.Fatal("crença deveria reter e1 como stale (saber ≠ ver)")
	}
	if _, ok := orch.Lister().Get("e2"); !ok {
		t.Fatal("crença deveria ter e2")
	}
}

// TestOrchestratorResyncWorldAutoHeals: resync contra snapshot autoritativo
// cura drops perdidos e expõe a divergência (I2, fail-closed).
func TestOrchestratorResyncWorldAutoHeals(t *testing.T) {
	orch := NewOrchestrator(DefaultOrchestratorConfig())
	now := time.Now()

	orch.mergeEntities([]WorldEntity{
		{ID: "e1", Position: Vec3{X: 1, Y: 0, Z: 0}, LastSeen: now},
		{ID: "e2", Position: Vec3{X: 2, Y: 0, Z: 0}, LastSeen: now},
		{ID: "e3", Position: Vec3{X: 3, Y: 0, Z: 0}, LastSeen: now},
	})

	e2Moved := WorldEntity{ID: "e2", Position: Vec3{X: 9, Y: 0, Z: 0}, LastSeen: now}
	snapshot := []WorldEntity{
		e2Moved,
		{ID: "e4", Position: Vec3{X: 4, Y: 0, Z: 0}, LastSeen: now},
	}

	diff := orch.ResyncWorld(snapshot, ObservationSource{SensorID: "cam0"}, TrustKnown, now.Add(time.Second))

	// e1 e e3 sumiram (drop curado), e4 entrou, e2 atualizado.
	if len(diff.Removed) != 2 {
		t.Fatalf("removidos inesperado: %v", diff.Removed)
	}
	if len(diff.Added) != 1 || diff.Added[0] != "e4" {
		t.Fatalf("adicionados inesperado: %v", diff.Added)
	}
	if len(diff.Updated) != 1 || diff.Updated[0] != "e2" {
		t.Fatalf("atualizados inesperado: %v", diff.Updated)
	}

	// Estado de visão reconciliado com o snapshot.
	state := orch.GetState()
	if len(state.Entities) != 2 {
		t.Fatalf("visão pós-resync deveria ter 2, got %d", len(state.Entities))
	}
	se, _ := orch.Lister().Get("e2")
	if se.Entity.Position.X != 9 {
		t.Fatalf("posição de e2 não atualizada: %+v", se.Entity.Position)
	}
	if _, ok := orch.Lister().Get("e1"); ok {
		t.Fatal("e1 deveria ter sido curado (removido) do mundo")
	}
}

// TestOrchestratorDestroyClearsBelief: destroy remove também da crença.
func TestOrchestratorDestroyClearsBelief(t *testing.T) {
	orch := NewOrchestrator(DefaultOrchestratorConfig())
	now := time.Now()
	orch.mergeEntities([]WorldEntity{{ID: "e1", Label: "box", LastSeen: now}})

	orch.InjectEvent(WorldEvent{Type: "destroy", EntityID: "e1"})

	if _, ok := orch.Lister().Get("e1"); ok {
		t.Fatal("destroy deveria limpar a crença (lister)")
	}
	if len(orch.GetState().Entities) != 0 {
		t.Fatal("destroy deveria limpar a visão")
	}
}
