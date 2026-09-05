package pipeline

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Checkpoint captures the minimal state needed to resume a workflow after a crash.
type Checkpoint struct {
	PlanID       string            `json:"plan_id"`
	LastSequence int64             `json:"last_sequence"`
	TaskStatuses map[string]string `json:"task_statuses"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Checksum     uint32            `json:"checksum,omitempty"` // CRC32 of serialized state
}

type CheckpointStore struct {
	dir string
	mu  sync.RWMutex
}

func NewCheckpointStore(dir string) (*CheckpointStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("checkpoint: create directory: %w", err)
	}
	return &CheckpointStore{dir: dir}, nil
}

// Save persists a checkpoint as {planID}.checkpoint.json with CRC32 integrity.
func (s *CheckpointStore) Save(cp Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cp.UpdatedAt = time.Now().UTC()

	// Compute CRC32 checksum of the serialized state (excluding checksum field)
	cp.Checksum = 0
	payload, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("checkpoint: marshal for checksum: %w", err)
	}
	cp.Checksum = crc32.ChecksumIEEE(payload)

	// Re-marshal with checksum included
	payload, err = json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("checkpoint: marshal: %w", err)
	}

	path := filepath.Join(s.dir, cp.PlanID+".checkpoint.json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("checkpoint: write: %w", err)
	}
	return nil
}

// Load reads a checkpoint for the given plan ID and verifies CRC32 integrity.
// Returns ErrChecksumMismatch if the state diverged (corruption, partial write).
func (s *CheckpointStore) Load(planID string) (*Checkpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.dir, planID+".checkpoint.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("checkpoint: read %q: %w", planID, err)
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("checkpoint: unmarshal %q: %w", planID, err)
	}

	// Verify integrity: re-compute checksum excluding the stored checksum field
	storedChecksum := cp.Checksum
	cp.Checksum = 0
	verifyPayload, _ := json.Marshal(cp)
	computed := crc32.ChecksumIEEE(verifyPayload)

	if storedChecksum != 0 && storedChecksum != computed {
		return &cp, fmt.Errorf("%w: plan %s (stored=%08x, computed=%08x)",
			ErrChecksumMismatch, planID, storedChecksum, computed)
	}

	return &cp, nil
}

// ErrChecksumMismatch is returned when checkpoint integrity verification fails.
var ErrChecksumMismatch = fmt.Errorf("checkpoint: checksum mismatch — state may be corrupted")
