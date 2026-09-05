package ingest

import (
	"crypto/sha256"
	"encoding/json"

	"github.com/CoscaAI/cosca/internal/world"
)

// Golden Slice — freeze the 1km×1km real-world dataset as a reference.
//
// The professor's idea: after any parser change, we must be able to ask
// "does the new version still interpret that area the same way?". The Golden
// Slice stores the canonical World Model + fingerprint + inventory counts.
// Any deviation in fingerprint/inventory signals a regression.

// GoldenSlice is an immutable snapshot of a reference ingestion.
type GoldenSlice struct {
	Name          string    `json:"name"`
	SourceDataset string    `json:"source_dataset"`
	SourceVersion string    `json:"source_version"`
	Origin        []float64 `json:"origin"` // [lat, lon, alt]
	EntityCount   int       `json:"entity_count"`
	Inventory     Inventory `json:"inventory"`
	Fingerprint   string    `json:"fingerprint"`
	WorldHash     string    `json:"world_hash"`
}

// BuildGolden freezes a world into a reference slice.
func BuildGolden(name string, w *world.World, inv Inventory) (*GoldenSlice, error) {
	fp, err := world.WorldFingerprint(w)
	if err != nil {
		return nil, err
	}
	return &GoldenSlice{
		Name:          name,
		SourceDataset: "osm",
		SourceVersion: "1.0",
		Origin:        []float64{w.CoordSystem.Origin.Latitude, w.CoordSystem.Origin.Longitude, w.CoordSystem.Origin.Altitude},
		EntityCount:   len(w.Entities),
		Inventory:     inv,
		Fingerprint:   fp,
		WorldHash:     fp,
	}, nil
}

// Marshal serializes the golden slice canonically.
func (g *GoldenSlice) Marshal() ([]byte, error) {
	return json.Marshal(g)
}

// Hash computes a content hash of the golden slice itself.
func (g *GoldenSlice) Hash() string {
	data, _ := json.Marshal(g)
	h := sha256.Sum256(data)
	return hexEncode(h[:])
}

func hexEncode(b []byte) string {
	const hexc = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexc[v>>4]
		out[i*2+1] = hexc[v&0x0f]
	}
	return string(out)
}
