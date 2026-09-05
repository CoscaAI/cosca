package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/sergi/go-diff/diffmatchpatch"
	"gopkg.in/yaml.v3"
)

// Snapshot represents a point-in-time copy of memory state.
type Snapshot struct {
	ID        string            `json:"id" yaml:"id"`
	Label     string            `json:"label,omitempty" yaml:"label,omitempty"`
	Layer     MemoryLayer       `json:"layer" yaml:"layer"`
	CreatedAt time.Time         `json:"created_at" yaml:"created_at"`
	RecordIDs []string          `json:"record_ids" yaml:"record_ids"`
	Size      int64             `json:"size" yaml:"size"`
	Metadata  map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// SnapshotDiff represents the differences between two snapshots.
type SnapshotDiff struct {
	SnapshotA string   `json:"snapshot_a"`
	SnapshotB string   `json:"snapshot_b"`
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Modified  []string `json:"modified"`
	Summary   string   `json:"summary"`
}

// SnapshotManager manages memory snapshots.
type SnapshotManager struct {
	mu           sync.RWMutex
	logger       zerolog.Logger
	dir          string
	engine       *MemoryEngine
	maxSnapshots int
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager(dir string, engine *MemoryEngine, logger zerolog.Logger) (*SnapshotManager, error) {
	snapDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating snapshot directory: %w", err)
	}

	return &SnapshotManager{
		logger:       logger,
		dir:          snapDir,
		engine:       engine,
		maxSnapshots: 50,
	}, nil
}

// Create creates a new snapshot of a memory layer.
func (sm *SnapshotManager) Create(ctx context.Context, layer MemoryLayer, label string) (*Snapshot, error) {
	sm.mu.Lock()

	// Get all records from the layer
	records, err := sm.engine.Search(ctx, "", SearchOptions{
		Layers: []MemoryLayer{layer},
		Limit:  10000,
	})
	if err != nil {
		sm.mu.Unlock()
		return nil, fmt.Errorf("searching records for snapshot: %w", err)
	}

	snapshot := &Snapshot{
		ID:        uuid.New().String(),
		Label:     label,
		Layer:     layer,
		CreatedAt: time.Now(),
		Size:      0,
	}

	for _, record := range records {
		snapshot.RecordIDs = append(snapshot.RecordIDs, record.ID)
		snapshot.Size += int64(len(record.Content))
	}

	// Marshal snapshot to YAML
	data, err := yaml.Marshal(snapshot)
	if err != nil {
		sm.mu.Unlock()
		return nil, fmt.Errorf("marshaling snapshot: %w", err)
	}

	// Write snapshot file
	fileName := fmt.Sprintf("%s_%s_%s.snapshot.yaml",
		layer,
		time.Now().Format("20060102_150405"),
		snapshot.ID[:8],
	)
	filePath := filepath.Join(sm.dir, fileName)

	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		sm.mu.Unlock()
		return nil, fmt.Errorf("writing snapshot file: %w", err)
	}

	// Also copy the actual memory files for the snapshot
	snapDataDir := filepath.Join(sm.dir, snapshot.ID)
	if err := os.MkdirAll(snapDataDir, 0o700); err != nil {
		sm.mu.Unlock()
		return nil, fmt.Errorf("creating snapshot data directory: %w", err)
	}

	for _, id := range snapshot.RecordIDs {
		_, err := sm.engine.Retrieve(ctx, id, layer)
		if err != nil {
			continue
		}
		// Save copy of record
		store, ok := sm.engine.stores[layer]
		if ok {
			if fileStore, ok := store.(*FileStore); ok {
				srcPath := filepath.Join(fileStore.dir, id+".md")
				if data, err := os.ReadFile(srcPath); err == nil {
					dstPath := filepath.Join(snapDataDir, id+".md")
					if err := os.WriteFile(dstPath, data, 0o600); err != nil {
						sm.mu.Unlock()
						return nil, fmt.Errorf("write snapshot: %w", err)
					}
				}
			}
		}
	}

	sm.logger.Info().
		Str("id", snapshot.ID[:8]).
		Str("layer", string(layer)).
		Str("label", label).
		Int("records", len(snapshot.RecordIDs)).
		Int64("size", snapshot.Size).
		Msg("snapshot created")

	// Release lock before cleanup — cleanup calls List() which acquires its own RLock,
	// and Go's sync.RWMutex is not reentrant. Holding the write lock during cleanup
	// would cause a deadlock.
	sm.mu.Unlock()

	// Cleanup old snapshots
	sm.cleanup(ctx)

	return snapshot, nil
}

// List returns all available snapshots.
func (sm *SnapshotManager) List(_ context.Context) ([]Snapshot, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	entries, err := os.ReadDir(sm.dir)
	if err != nil {
		return nil, err
	}

	var snapshots []Snapshot
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".snapshot.yaml") {
			continue
		}

		filePath := filepath.Join(sm.dir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var snapshot Snapshot
		if err := yaml.Unmarshal(data, &snapshot); err != nil {
			continue
		}
		snapshots = append(snapshots, snapshot)
	}

	// Sort by creation time, newest first
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})

	return snapshots, nil
}

// Restore restores memory state from a snapshot.
func (sm *SnapshotManager) Restore(ctx context.Context, snapshotID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	snapshot, err := sm.findSnapshot(ctx, snapshotID)
	if err != nil {
		return fmt.Errorf("finding snapshot %s: %w", snapshotID, err)
	}

	// Clear current records for the layer
	store, ok := sm.engine.stores[snapshot.Layer]
	if !ok {
		return fmt.Errorf("no store for layer %s", snapshot.Layer)
	}

	// Load records from snapshot data directory
	snapDataDir := filepath.Join(sm.dir, snapshot.ID)
	for _, id := range snapshot.RecordIDs {
		filePath := filepath.Join(snapDataDir, id+".md")
		if data, err := os.ReadFile(filePath); err == nil {
			// Parse and save the record
			if fileStore, ok := store.(*FileStore); ok {
				dstPath := filepath.Join(fileStore.dir, id+".md")
				if err := os.WriteFile(dstPath, data, 0644); err != nil {
					sm.logger.Warn().Err(err).Str("id", id).Msg("restoring record file")
				}
			}
		}
	}

	// Rebuild index
	sm.logger.Info().
		Str("id", snapshot.ID[:8]).
		Str("layer", string(snapshot.Layer)).
		Int("records", len(snapshot.RecordIDs)).
		Msg("snapshot restored")

	return nil
}

// Diff computes the difference between two snapshots.
func (sm *SnapshotManager) Diff(ctx context.Context, snapshotAID, snapshotBID string) (*SnapshotDiff, error) {
	snapshotA, err := sm.findSnapshot(ctx, snapshotAID)
	if err != nil {
		return nil, fmt.Errorf("finding snapshot A: %w", err)
	}

	snapshotB, err := sm.findSnapshot(ctx, snapshotBID)
	if err != nil {
		return nil, fmt.Errorf("finding snapshot B: %w", err)
	}

	setA := make(map[string]bool)
	for _, id := range snapshotA.RecordIDs {
		setA[id] = true
	}

	setB := make(map[string]bool)
	for _, id := range snapshotB.RecordIDs {
		setB[id] = true
	}

	diff := &SnapshotDiff{
		SnapshotA: snapshotA.ID,
		SnapshotB: snapshotB.ID,
	}

	// Find added and removed
	for id := range setB {
		if !setA[id] {
			diff.Added = append(diff.Added, id)
		}
	}
	for id := range setA {
		if !setB[id] {
			diff.Removed = append(diff.Removed, id)
		}
	}

	// Find modified (present in both but potentially different content)
	for id := range setA {
		if setB[id] {
			// Check if content changed
			contentA, _ := sm.readRecordContent(ctx, id, snapshotA.Layer)
			contentB, _ := sm.readRecordContent(ctx, id, snapshotB.Layer)
			if contentA != contentB {
				diff.Modified = append(diff.Modified, id)
			}
		}
	}

	// Generate summary
	diff.Summary = fmt.Sprintf("Added: %d, Removed: %d, Modified: %d",
		len(diff.Added), len(diff.Removed), len(diff.Modified))

	return diff, nil
}

// DiffText generates a text representation of the diff between snapshot content.
func (sm *SnapshotManager) DiffText(ctx context.Context, snapshotAID, snapshotBID, recordID string) (string, error) {
	contentA, err := sm.readRecordContent(ctx, recordID, "")
	if err != nil {
		// Try to find which layer
		snapshotA, err := sm.findSnapshot(ctx, snapshotAID)
		if err != nil {
			return "", err
		}
		contentA, err = sm.readRecordContent(ctx, recordID, snapshotA.Layer)
		if err != nil {
			contentA = ""
		}
	}

	contentB, err := sm.readRecordContent(ctx, recordID, "")
	if err != nil {
		snapshotB, err := sm.findSnapshot(ctx, snapshotBID)
		if err != nil {
			return "", err
		}
		contentB, err = sm.readRecordContent(ctx, recordID, snapshotB.Layer)
		if err != nil {
			contentB = ""
		}
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(contentA, contentB, true)
	return dmp.DiffPrettyText(diffs), nil
}

// CreateOnEvent creates a snapshot automatically when a significant event occurs.
func (sm *SnapshotManager) CreateOnEvent(ctx context.Context, layer MemoryLayer, event string) (*Snapshot, error) {
	label := fmt.Sprintf("auto_%s_%s", event, time.Now().Format("20060102_150405"))
	sm.logger.Info().Str("event", event).Str("layer", string(layer)).Msg("creating automatic snapshot")
	return sm.Create(ctx, layer, label)
}

// PruneOldSnapshots removes old snapshots beyond the retention limit.
func (sm *SnapshotManager) PruneOldSnapshots(ctx context.Context, maxAge time.Duration) (int, error) {
	snapshots, err := sm.List(ctx)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	count := 0

	for _, snap := range snapshots {
		if snap.CreatedAt.Before(cutoff) {
			if err := sm.deleteSnapshot(ctx, snap.ID); err == nil {
				count++
			}
		}
	}

	return count, nil
}

// findSnapshot finds a snapshot by ID or label.
func (sm *SnapshotManager) findSnapshot(_ context.Context, idOrLabel string) (*Snapshot, error) {
	entries, err := os.ReadDir(sm.dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".snapshot.yaml") {
			continue
		}

		filePath := filepath.Join(sm.dir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var snapshot Snapshot
		if err := yaml.Unmarshal(data, &snapshot); err != nil {
			continue
		}

		if snapshot.ID == idOrLabel || snapshot.Label == idOrLabel {
			return &snapshot, nil
		}
	}

	return nil, fmt.Errorf("snapshot not found: %s", idOrLabel)
}

// deleteSnapshot deletes a snapshot by ID.
func (sm *SnapshotManager) deleteSnapshot(ctx context.Context, id string) error {
	snapshot, err := sm.findSnapshot(ctx, id)
	if err != nil {
		return err
	}

	// Delete the snapshot data directory
	snapDataDir := filepath.Join(sm.dir, snapshot.ID)
	_ = os.RemoveAll(snapDataDir)

	// Delete the snapshot file
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	entries, _ := os.ReadDir(sm.dir)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".snapshot.yaml") {
			filePath := filepath.Join(sm.dir, entry.Name())
			data, _ := os.ReadFile(filePath)
			var snap Snapshot
			if yaml.Unmarshal(data, &snap) == nil && snap.ID == id {
				return os.Remove(filePath)
			}
		}
	}

	return nil
}

// readRecordContent reads the content of a memory record by ID.
func (sm *SnapshotManager) readRecordContent(ctx context.Context, id string, layer MemoryLayer) (string, error) {
	if layer != "" {
		record, err := sm.engine.Retrieve(ctx, id, layer)
		if err == nil {
			return record.Content, nil
		}
	}

	// Try all layers
	for _, l := range sm.engine.layers.Layers() {
		record, err := sm.engine.Retrieve(ctx, id, l)
		if err == nil {
			return record.Content, nil
		}
	}

	return "", fmt.Errorf("record %s not found in any layer", id)
}

// cleanup removes old snapshots beyond the maximum count.
func (sm *SnapshotManager) cleanup(ctx context.Context) {
	snapshots, err := sm.List(ctx)
	if err != nil {
		return
	}

	if len(snapshots) <= sm.maxSnapshots {
		return
	}

	// Remove oldest snapshots
	toRemove := len(snapshots) - sm.maxSnapshots
	for i := len(snapshots) - 1; i >= 0 && toRemove > 0; i-- {
		if err := sm.deleteSnapshot(ctx, snapshots[i].ID); err == nil {
			toRemove--
		}
	}
}
