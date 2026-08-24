package world

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Serialization: canonical, deterministic, versioned, hashable.
//
// The world model serializes to a canonical JSON form. Determinism is
// guaranteed by:
//   - Stable field order (Go structs marshal in declaration order)
//   - SchemaVersion embedded in every payload
//   - Deterministic entity ordering (sorted by ID when required)
//   - No random/time elements in the serialized payloads (timestamps are
//     provided by the caller, not injected by the serializer)

// Error codes for serialization.
const (
	ErrSchemaMismatch      = "schema_version_mismatch"
	ErrCorruptPayload      = "corrupt_payload"
)

// MarshalWorldCanonical serializes a world deterministically.
// The output is byte-for-byte reproducible for the same logical world.
func MarshalWorldCanonical(w *World) ([]byte, error) {
	return json.Marshal(w)
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

// WorldFingerprint computes a deterministic SHA-256 over the whole world's
// canonical serialization. Same logical world ⇒ same fingerprint.
// This enables reproduction/verification: "is this the same world?"
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

// DeterministicEntities returns entities sorted by ID, so output is stable.
func deterministicEntities(entities []Entity) []Entity {
	out := make([]Entity, len(entities))
	copy(out, entities)
	// Simple insertion sort (stable, small n). Use sort.Slice for large n.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].ID < out[j-1].ID; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
