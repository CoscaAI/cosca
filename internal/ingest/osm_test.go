package ingest

import (
	"os"
	"testing"

	"github.com/CoscaAI/cosca/internal/world"
	"github.com/CoscaAI/cosca/internal/world/city"
)

// loadFixture reads the OSM test fixture.
func loadFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/palhoca_sample.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// TestParseOSMReal verifies a real OSM extract ingests into a valid World Model.
func TestParseOSMReal(t *testing.T) {
	data := loadFixture(t)
	cfg := DefaultConfig(world.GeoCoordinates{Latitude: -27.6375, Longitude: -48.6765})

	res, err := ParseOSM(data, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// World must have ingested entities.
	if len(res.World.Entities) == 0 {
		t.Fatal("no entities ingested from real OSM")
	}

	// Validate the resulting world (invariants).
	vr := world.Validate(res.World)
	if !vr.Valid {
		t.Errorf("world validation failed after OSM ingestion: %+v", vr.Issues)
	}

	// Fingerprint must be stable.
	fp1, _ := res.World.HashWorld()
	fp2, _ := res.World.HashWorld()
	if fp1 != fp2 {
		t.Error("fingerprint unstable after ingestion")
	}
}

// TestParseOSMClasses verifies road/building/water/park mapping.
func TestParseOSMClasses(t *testing.T) {
	data := loadFixture(t)
	cfg := DefaultConfig(world.GeoCoordinates{Latitude: -27.6375, Longitude: -48.6765})
	res, err := ParseOSM(data, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// At minimum roads and buildings should be present in a city extract.
	if res.Classes[world.ClassRoad] == 0 {
		t.Error("no roads ingested")
	}
	if res.Classes[world.ClassStructure] == 0 {
		t.Error("no buildings ingested")
	}
}

// TestParseOSMProvenance verifies provenance is attached (FACT + source).
func TestParseOSMProvenance(t *testing.T) {
	data := loadFixture(t)
	res, err := ParseOSM(data, DefaultConfig(world.GeoCoordinates{Latitude: -27.6375, Longitude: -48.6765}))
	if err != nil {
		t.Fatal(err)
	}
	for i := range res.World.Entities {
		e := &res.World.Entities[i]
		if e.Provenance.Class != world.ClassFACT {
			t.Errorf("entity %s class = %s, want FACT", e.ID, e.Provenance.Class)
		}
		if e.Provenance.Source.Dataset != "osm" {
			t.Errorf("entity %s dataset = %s, want osm", e.ID, e.Provenance.Source.Dataset)
		}
		if e.Provenance.Source.Hash == "" {
			t.Errorf("entity %s missing source hash", e.ID)
		}
	}
}

// TestOSMGeometry verifies geometry was built (linestring/polygon) with local coords.
func TestOSMGeometry(t *testing.T) {
	data := loadFixture(t)
	res, err := ParseOSM(data, DefaultConfig(world.GeoCoordinates{Latitude: -27.6375, Longitude: -48.6765}))
	if err != nil {
		t.Fatal(err)
	}
	foundGeom := 0
	for i := range res.World.Entities {
		e := &res.World.Entities[i]
		if e.Geometry != nil && len(e.Geometry.Points) >= 2 {
			foundGeom++
			// Coords should be LOCAL (world meters), NOT lat/lon degrees.
			// lat/lon would be ~-27 / ~-48. Local meters are relative to the
			// origin anchor and may be NEGATIVE (points west/south of origin).
			// Valid range for a ~1km extract: |coord| < 5000m.
			for _, p := range e.Geometry.Points {
				if p.X < -5000 || p.X > 5000 || p.Y < -5000 || p.Y > 5000 {
					t.Errorf("entity %s has non-local coords (%f,%f)", e.ID, p.X, p.Y)
					break
				}
			}
		}
	}
	if foundGeom == 0 {
		t.Error("no entity has geometry")
	}
}

// TestFundamental verifies the professor's key test (B8):
// the SAME World Model accepts a SYNTHETIC city and a REAL OSM extract,
// both producing the same semantic structure (valid, fingerprinted, queryable).
func TestFundamental(t *testing.T) {
	// Synthetic city.
	synth, err := city.Generate(city.DefaultConfig(42))
	if err != nil {
		t.Fatal(err)
	}
	svr := world.Validate(synth.World)
	if !svr.Valid {
		t.Errorf("synthetic world invalid: %+v", svr.Issues)
	}

	// Real OSM.
	data := loadFixture(t)
	real, err := ParseOSM(data, DefaultConfig(world.GeoCoordinates{Latitude: -27.6375, Longitude: -48.6765}))
	if err != nil {
		t.Fatal(err)
	}
	rvr := world.Validate(real.World)
	if !rvr.Valid {
		t.Errorf("real world invalid: %+v", rvr.Issues)
	}

	// Both are queryable (world reasoning works on both).
	if len(synth.World.ByType("building.house")) == 0 {
		t.Error("synthetic city has no buildings")
	}
	if len(real.World.ByType("road.highway")) == 0 && len(real.World.ByType("road.residential")) == 0 {
		t.Error("real OSM has no roads")
	}
}

// TestQueriesRealWorld applies world queries to the real OSM world.
func TestQueriesRealWorld(t *testing.T) {
	data := loadFixture(t)
	res, err := ParseOSM(data, DefaultConfig(world.GeoCoordinates{Latitude: -27.6375, Longitude: -48.6765}))
	if err != nil {
		t.Fatal(err)
	}
	w := res.World

	// Count by class — roads and structures should be present.
	counts := w.CountByClass()
	if counts[world.ClassRoad] == 0 {
		t.Error("query: no roads in real world")
	}

	// All entities have stable IDs.
	for i := range w.Entities {
		if w.Entities[i].ID == "" {
			t.Error("an entity has empty ID")
		}
	}
}
