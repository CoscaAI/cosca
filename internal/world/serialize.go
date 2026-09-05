package world

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// Serialization: canonical, deterministic, versioned, hashable.
//
// CANONICAL (regra do professor, item 10): the fingerprint must NOT depend
// on insertion order. So A,B,C and C,A,B → the SAME hash. The serializer
// sorts entities by ID and relations by (subject,object,relation) before
// producing the canonical payload.
//
// Determinism is guaranteed by:
//   - Entities sorted by ID
//   - Relations sorted by (subject, object, relation)
//   - SchemaVersion embedded
//   - No random/time elements injected by the serializer

// Error codes for serialization.
const (
	ErrSchemaMismatch = "schema_version_mismatch"
	ErrCorruptPayload = "corrupt_payload"
)

// canonicalWorld is a transient, order-independent projection of a World.
// It is produced on-the-fly and is NOT the stored representation; it exists
// solely to guarantee a canonical byte stream for hashing.
type canonicalWorld struct {
	WorldID       string          `json:"world_id"`
	SchemaVersion int             `json:"schema_version"`
	CoordSystem   CoordinateSystem `json:"coord_system"`
	Entities      []Entity        `json:"entities"`
	Relations     []Relation      `json:"relations"`
	Weather       WeatherState    `json:"weather"`
	Season        Season          `json:"season"`
	Time          SimulationTime  `json:"time"`
	Version       int             `json:"version"`
}

// buildCanonicalWorld constructs an order-independent projection of w.
func buildCanonicalWorld(w *World) canonicalWorld {
	entities := make([]Entity, len(w.Entities))
	copy(entities, w.Entities)
	sort.Slice(entities, func(i, j int) bool { return entities[i].ID < entities[j].ID })

	relations := make([]Relation, len(w.Relations))
	copy(relations, w.Relations)
	sort.Slice(relations, func(i, j int) bool {
		a, b := relations[i], relations[j]
		if a.Subject != b.Subject {
			return a.Subject < b.Subject
		}
		if a.Object != b.Object {
			return a.Object < b.Object
		}
		return a.Relation < b.Relation
	})

	return canonicalWorld{
		WorldID:       w.WorldID,
		SchemaVersion: w.SchemaVersion,
		CoordSystem:   w.CoordSystem,
		Entities:      entities,
		Relations:     relations,
		Weather:       w.Weather,
		Season:        w.Season,
		Time:          w.Time,
		Version:       w.Version,
	}
}

// MarshalWorldCanonical serializes a world deterministically and
// independent of insertion order. Same logical world ⇒ same bytes.
func MarshalWorldCanonical(w *World) ([]byte, error) {
	return json.Marshal(buildCanonicalWorld(w))
}

// UnmarshalWorld decodes a serialized world, validating schema version.
func UnmarshalWorld(data []byte) (*World, error) {
	var w World
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, err
	}
	if w.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("%s: got %d want %d", ErrSchemaMismatch, w.SchemaVersion, SchemaVersion)
	}
	return &w, nil
}

// MarshalEntityCanonical serializes a single entity deterministically.
func MarshalEntityCanonical(e Entity) ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalEntity decodes an entity.
func UnmarshalEntity(data []byte) (*Entity, error) {
	var e Entity
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// WorldFingerprint computes a deterministic SHA-256 over the canonical
// (order-independent) serialization. Same logical world ⇒ same fingerprint,
// regardless of insertion order.
func WorldFingerprint(w *World) (string, error) {
	data, err := MarshalWorldCanonical(w)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// HashWorld returns the fingerprint as a hex string (convenience).
func (w *World) HashWorld() (string, error) {
	return WorldFingerprint(w)
}
