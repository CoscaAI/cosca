package city

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/world"
)

// TestCityHierarchy valida a estrutura hierárquica completa:
// City → Districts → Streets → Buildings/Trees/Vehicles + Squares + River.
func TestCityHierarchy(t *testing.T) {
	cfg := DefaultConfig(42)
	res, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := res.World

	// City exists.
	if w.GetEntity(res.CityID) == nil {
		t.Fatal("city entity missing")
	}

	// Districts.
	if len(res.DistrictIDs) != cfg.Districts {
		t.Errorf("districts = %d, want %d", len(res.DistrictIDs), cfg.Districts)
	}
	for _, did := range res.DistrictIDs {
		d := w.GetEntity(did)
		if d == nil {
			t.Fatalf("district %s missing", did)
		}
		if d.Parent != res.CityID {
			t.Errorf("district %s parent = %s, want %s", did, d.Parent, res.CityID)
		}
	}

	// Streets: each belongs to a district.
	if len(res.StreetIDs) != cfg.Districts*cfg.StreetsPerD {
		t.Errorf("streets = %d, want %d", len(res.StreetIDs), cfg.Districts*cfg.StreetsPerD)
	}
	for _, sid := range res.StreetIDs {
		s := w.GetEntity(sid)
		if s == nil || s.Class != world.ClassRoad {
			t.Errorf("street %s missing or not road", sid)
		}
	}

	// River.
	river := w.GetEntity("river_001")
	if river == nil {
		t.Error("river missing")
	} else if river.Class != world.ClassWater {
		t.Errorf("river class = %s, want water", river.Class)
	}

	// Squares: one per district.
	squares := 0
	for i := range w.Entities {
		if w.Entities[i].Type == "square.public" {
			squares++
		}
	}
	if squares != cfg.Districts {
		t.Errorf("squares = %d, want %d", squares, cfg.Districts)
	}
}

// TestCityDeterminism verifica que mesma seed = mesma cidade (fingerprint).
func TestCityDeterminism(t *testing.T) {
	w1, err := Generate(DefaultConfig(123))
	if err != nil {
		t.Fatal(err)
	}
	w2, err := Generate(DefaultConfig(123))
	if err != nil {
		t.Fatal(err)
	}
	fp1, _ := w1.World.HashWorld()
	fp2, _ := w2.World.HashWorld()
	if fp1 != fp2 {
		t.Error("same seed produced different worlds")
	}

	// Different seed ⇒ different world.
	w3, _ := Generate(DefaultConfig(999))
	fp3, _ := w3.World.HashWorld()
	if fp3 == fp1 {
		t.Error("different seeds produced identical worlds")
	}
}

// TestCityEntityTypes verifica os tipos semânticos presentes.
func TestCityEntityTypes(t *testing.T) {
	w, err := Generate(DefaultConfig(7))
	if err != nil {
		t.Fatal(err)
	}
	classes := map[world.EntityClass]int{}
	for i := range w.World.Entities {
		classes[w.World.Entities[i].Class]++
	}
	for _, want := range []world.EntityClass{
		world.ClassStructure, world.ClassTerrain, world.ClassRoad,
		world.ClassVegetation, world.ClassVehicle, world.ClassWater,
	} {
		if classes[want] == 0 {
			t.Errorf("no entities of class %s", want)
		}
	}
}

// TestCityProvenance verifica que todo conteúdo gerado tem provenance+seed.
func TestCityProvenance(t *testing.T) {
	w, err := Generate(DefaultConfig(5))
	if err != nil {
		t.Fatal(err)
	}
	for i := range w.World.Entities {
		e := &w.World.Entities[i]
		if e.Provenance.Class != world.ClassGENERATED {
			t.Errorf("entity %s class = %s, want generated", e.ID, e.Provenance.Class)
		}
		if e.Provenance.Generation == nil || e.Provenance.Generation.Seed != 5 {
			t.Errorf("entity %s missing generation/seed", e.ID)
		}
	}
}

// TestCityGraphRelations verifica relações semânticas no grafo.
func TestCityGraphRelations(t *testing.T) {
	w, err := Generate(DefaultConfig(11))
	if err != nil {
		t.Fatal(err)
	}
	// Streets should be connected/located within the city.
	relCount := len(w.World.Relations)
	if relCount == 0 {
		t.Error("no relations in graph")
	}
}
