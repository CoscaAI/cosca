package ingest

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/world"
)

// TestInventoryRealWorld verifica o inventário semântico do mundo real.
func TestInventoryRealWorld(t *testing.T) {
	data := loadFixture(t)
	res, err := ParseOSM(data, DefaultConfig(world.GeoCoordinates{Latitude: -27.6375, Longitude: -48.6765}))
	if err != nil {
		t.Fatal(err)
	}
	inv := InventoryOf(res.World)

	if inv.Buildings == 0 {
		t.Error("inventory: no buildings")
	}
	if inv.Roads == 0 && inv.Avenues == 0 {
		t.Error("inventory: no roads")
	}
	// No invalid geometry, no orphan relations, no duplicates.
	if inv.InvalidGeometry != 0 {
		t.Errorf("inventory: %d invalid geometries", inv.InvalidGeometry)
	}
	if inv.OrphanRelations != 0 {
		t.Errorf("inventory: %d orphan relations", inv.OrphanRelations)
	}
	if inv.DuplicateEntities != 0 {
		t.Errorf("inventory: %d duplicate entities", inv.DuplicateEntities)
	}
}

// TestGoldenSliceFrozen verifica o Golden Slice determinístico.
func TestGoldenSliceFrozen(t *testing.T) {
	data := loadFixture(t)
	res, err := ParseOSM(data, DefaultConfig(world.GeoCoordinates{Latitude: -27.6375, Longitude: -48.6765}))
	if err != nil {
		t.Fatal(err)
	}
	inv := InventoryOf(res.World)
	golden, err := BuildGolden("palhoca-1km", res.World, inv)
	if err != nil {
		t.Fatal(err)
	}

	// Fingerprint must be stable.
	if golden.Fingerprint == "" {
		t.Fatal("empty fingerprint")
	}
	if golden.EntityCount != len(res.World.Entities) {
		t.Errorf("entity count = %d, want %d", golden.EntityCount, len(res.World.Entities))
	}

	// Rebuilding the golden from a re-parse must yield the same fingerprint.
	res2, err := ParseOSM(data, DefaultConfig(world.GeoCoordinates{Latitude: -27.6375, Longitude: -48.6765}))
	if err != nil {
		t.Fatal(err)
	}
	g2, _ := BuildGolden("palhoca-1km", res2.World, InventoryOf(res2.World))
	if golden.Fingerprint != g2.Fingerprint {
		t.Errorf("golden fingerprint unstable: %s != %s", golden.Fingerprint, g2.Fingerprint)
	}
}
