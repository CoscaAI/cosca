package worldmodel

import (
	"testing"
	"time"
)

func mkEntity(id string, t EntityType) WorldEntity {
	return WorldEntity{
		ID:         id,
		Type:       t,
		Label:      "ent_" + id,
		Confidence: 0.9,
		LastSeen:   time.Unix(100, 0),
		Position:   Vec3{X: 1, Y: 2, Z: 3},
	}
}

var cameraSrc = ObservationSource{SensorID: "cam0", Authenticated: true, ReplayProtected: true}

func TestLister_UpsertNew(t *testing.T) {
	l := NewLister()
	now := time.Unix(1000, 0)
	out := l.Upsert(mkEntity("a", EntityObject), cameraSrc, TrustKnown, now)
	if !out.Applied || !out.IsNew || !out.Changed || out.Revision != 1 {
		t.Fatalf("upsert novo inesperado: %+v", out)
	}
	se, ok := l.Get("a")
	if !ok {
		t.Fatal("entidade nao encontrada")
	}
	if se.Stamp.Revision != 1 || se.Stamp.Trust != TrustKnown || se.Stamp.AsOf != now {
		t.Fatalf("selo inesperado: %+v", se.Stamp)
	}
}

func TestLister_UpsertFencing(t *testing.T) {
	l := NewLister()
	now := time.Unix(1000, 0)
	l.Upsert(mkEntity("a", EntityObject), cameraSrc, TrustKnown, now)

	// Escrita com AsOf ANTERIOR → rejeitada (nunca regride em crença mais velha).
	out := l.Upsert(mkEntity("a", EntityObject), cameraSrc, TrustKnown, now.Add(-time.Minute))
	if out.Applied {
		t.Fatalf("escrita stale deveria ser rejeitada (fencing): %+v", out)
	}
	se, _ := l.Get("a")
	if se.Stamp.Revision != 1 {
		t.Fatalf("revision deveria permanecer 1, got %d", se.Stamp.Revision)
	}
}

func TestLister_UpsertMonotonicRevision(t *testing.T) {
	l := NewLister()
	base := time.Unix(1000, 0)
	l.Upsert(mkEntity("a", EntityObject), cameraSrc, TrustKnown, base)
	// Conteúdo igual → Changed=false, mas revision incrementa (escrita aceita).
	out := l.Upsert(mkEntity("a", EntityObject), cameraSrc, TrustKnown, base.Add(time.Second))
	if !out.Applied || out.Changed {
		t.Fatalf("upsert igual inesperado: %+v", out)
	}
	if out.Revision != 2 {
		t.Fatalf("revision deveria ser 2, got %d", out.Revision)
	}
	// Conteúdo DIFERENTE → Changed=true, revision incrementa.
	moved := mkEntity("a", EntityObject)
	moved.Position = Vec3{X: 9, Y: 9, Z: 9}
	out2 := l.Upsert(moved, cameraSrc, TrustKnown, base.Add(2*time.Second))
	if !out2.Applied || !out2.Changed || out2.Revision != 3 {
		t.Fatalf("upsert alterado inesperado: %+v", out2)
	}
}

func TestLister_ListSorted(t *testing.T) {
	l := NewLister()
	now := time.Unix(1000, 0)
	l.Upsert(mkEntity("c", EntityObject), cameraSrc, TrustKnown, now)
	l.Upsert(mkEntity("a", EntityObject), cameraSrc, TrustKnown, now)
	l.Upsert(mkEntity("b", EntityNPC), cameraSrc, TrustKnown, now)

	list := l.List()
	if len(list) != 3 {
		t.Fatalf("esperava 3, got %d", len(list))
	}
	// Ordenado por ID (determinístico).
	if list[0].Entity.ID != "a" || list[1].Entity.ID != "b" || list[2].Entity.ID != "c" {
		t.Fatalf("lista fora de ordem: %v %v %v", list[0].Entity.ID, list[1].Entity.ID, list[2].Entity.ID)
	}

	byType := l.ListByType(EntityObject)
	if len(byType) != 2 {
		t.Fatalf("esperava 2 objetos, got %d", len(byType))
	}
	if byType[0].Entity.ID != "a" || byType[1].Entity.ID != "c" {
		t.Fatalf("listByType fora de ordem: %v %v", byType[0].Entity.ID, byType[1].Entity.ID)
	}
}

func TestLister_Delete(t *testing.T) {
	l := NewLister()
	now := time.Unix(1000, 0)
	l.Upsert(mkEntity("a", EntityObject), cameraSrc, TrustKnown, now)
	if !l.Delete("a") {
		t.Fatal("delete deveria ter sucesso")
	}
	if _, ok := l.Get("a"); ok {
		t.Fatal("entidade ainda presente apos delete")
	}
	if l.Delete("a") {
		t.Fatal("delete repetido deveria falhar")
	}
}

func TestLister_ResyncAutoHeal(t *testing.T) {
	l := NewLister()
	now := time.Unix(1000, 0)

	// Cache inicial: a, b, c.
	l.Upsert(mkEntity("a", EntityObject), cameraSrc, TrustKnown, now)
	l.Upsert(mkEntity("b", EntityObject), cameraSrc, TrustKnown, now)
	l.Upsert(mkEntity("c", EntityNPC), cameraSrc, TrustKnown, now)

	// Snapshot autoritativo: a, b (com b movido), d (novo) — 'c' sumiu (drop).
	bMoved := mkEntity("b", EntityObject)
	bMoved.Position = Vec3{X: 5, Y: 5, Z: 5}
	snapshot := []WorldEntity{
		mkEntity("a", EntityObject),
		bMoved,
		mkEntity("d", EntityVehicle),
	}

	diff := l.Resync(snapshot, cameraSrc, TrustKnown, now.Add(time.Second))
	if diff.Version != l.Version() {
		t.Fatalf("diff.Version %d != cache %d", diff.Version, l.Version())
	}
	if len(diff.Added) != 1 || diff.Added[0] != "d" {
		t.Fatalf("Added inesperado: %v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0] != "c" {
		t.Fatalf("Removed inesperado: %v", diff.Removed)
	}
	if len(diff.Updated) != 1 || diff.Updated[0] != "b" {
		t.Fatalf("Updated inesperado: %v", diff.Updated)
	}

	// 'c' foi curado (removido), 'd' entrou.
	if _, ok := l.Get("c"); ok {
		t.Fatal("drop perdido nao foi curado")
	}
	if _, ok := l.Get("d"); !ok {
		t.Fatal("nova entidade nao entrou no resync")
	}
	se, _ := l.Get("b")
	if se.Entity.Position != (Vec3{X: 5, Y: 5, Z: 5}) {
		t.Fatalf("posicao de b nao atualizada: %+v", se.Entity.Position)
	}
}

func TestLister_ResyncIfDue(t *testing.T) {
	l := NewLister(WithResyncPeriod(5 * time.Second))
	now := time.Unix(1000, 0)

	// Sem lastSync → deve resyncar.
	due, diff := l.ResyncIfDue(now, cameraSrc, TrustKnown, func() []WorldEntity {
		return []WorldEntity{mkEntity("a", EntityObject)}
	})
	if !due || len(diff.Added) != 1 {
		t.Fatalf("primeiro resync deveria rodar: due=%v diff=%+v", due, diff)
	}

	// Logo depois → NÃO deve resyncar (período não decorreu).
	due, _ = l.ResyncIfDue(now.Add(2*time.Second), cameraSrc, TrustKnown, func() []WorldEntity {
		return nil
	})
	if due {
		t.Fatal("resync nao deveria rodar antes do periodo")
	}

	// Passado o período → deve resyncar.
	due, diff = l.ResyncIfDue(now.Add(6*time.Second), cameraSrc, TrustKnown, func() []WorldEntity {
		return []WorldEntity{mkEntity("a", EntityObject)}
	})
	if !due {
		t.Fatal("resync deveria rodar apos o periodo")
	}
	if !diff.Empty() {
		t.Fatalf("nada deveria ter mudado, got %+v", diff)
	}
}

func TestLister_Stale(t *testing.T) {
	l := NewLister()
	now := time.Unix(1000, 0)
	l.Upsert(mkEntity("a", EntityObject), cameraSrc, TrustKnown, now.Add(-10*time.Second))
	l.Upsert(mkEntity("b", EntityObject), cameraSrc, TrustKnown, now.Add(-1*time.Second))

	stale := l.Stale(now, 5*time.Second)
	if len(stale) != 1 || stale[0].Entity.ID != "a" {
		t.Fatalf("stale inesperado: %+v", stale)
	}
}

func TestStampedEntity_KnownKindDegrade(t *testing.T) {
	// Selo Unknown → crença degradada (I4): o sistema NÃO age sobre "não sabe que não sabe".
	se := StampedEntity{
		Entity: WorldEntity{ID: "a", Type: EntityObject, Visibility: VisibilityCurrent},
		Stamp:  EntityStamp{Trust: TrustUnknown},
	}
	if se.KnownKind() != VisibilityStale {
		t.Fatalf("Unknown deveria degradar para stale, got %v", se.KnownKind())
	}

	// Selo known + entity current → current.
	se2 := StampedEntity{
		Entity: WorldEntity{ID: "a", Type: EntityObject, Visibility: VisibilityCurrent},
		Stamp:  EntityStamp{Trust: TrustKnown},
	}
	if se2.KnownKind() != VisibilityCurrent {
		t.Fatalf("known+current deveria ser current, got %v", se2.KnownKind())
	}
}
