package cosca

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// MemorySDK Extended Tests - Part 2: Snapshots and Restore
// =============================================================================

func TestMemorySDK_ListSnapshots(t *testing.T) {
	t.Parallel()

	expected := []Snapshot{
		{
			ID:         "snap-001",
			Name:       "pre-deploy-2024-06-01",
			AgentID:    "agent-main",
			Size:       2048000,
			EntryCount: 450,
			CreatedAt:  time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
			Labels:     []string{"deploy", "staging"},
		},
		{
			ID:         "snap-002",
			Name:       "weekly-backup",
			AgentID:    "agent-main",
			Size:       1980000,
			EntryCount: 445,
			CreatedAt:  time.Date(2024, 5, 25, 10, 0, 0, 0, time.UTC),
			Labels:     []string{"backup", "weekly"},
		},
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/memory/snapshots")
		writeJSON(t, w, http.StatusOK, snapshotListResponse{
			Snapshots: expected,
			Total:     2,
		})
	})

	snapshots, err := c.Memory.ListSnapshots()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}

	s0 := snapshots[0]
	if s0.ID != "snap-001" {
		t.Errorf("expected ID %q, got %q", "snap-001", s0.ID)
	}
	if s0.Name != "pre-deploy-2024-06-01" {
		t.Errorf("expected Name %q, got %q", "pre-deploy-2024-06-01", s0.Name)
	}
	if s0.Size != 2048000 {
		t.Errorf("expected Size %d, got %d", 2048000, s0.Size)
	}
	if s0.EntryCount != 450 {
		t.Errorf("expected EntryCount %d, got %d", 450, s0.EntryCount)
	}
	if len(s0.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(s0.Labels))
	}
}

func TestMemorySDK_ListSnapshots_Empty(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, snapshotListResponse{
			Snapshots: []Snapshot{},
			Total:     0,
		})
	})

	snapshots, err := c.Memory.ListSnapshots()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(snapshots) != 0 {
		t.Errorf("expected empty list, got %d snapshots", len(snapshots))
	}
}

func TestMemorySDK_ListSnapshots_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusInternalServerError, "SNAPSHOT_ERROR", "failed to list snapshots")
	})

	_, err := c.Memory.ListSnapshots()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMemorySDK_CreateSnapshot(t *testing.T) {
	t.Parallel()

	expected := Snapshot{
		ID:         "snap-new-001",
		Name:       "before-upgrade",
		AgentID:    "",
		Size:       512000,
		EntryCount: 100,
		CreatedAt:  time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		Labels:     nil,
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/memory/snapshots")

		var req createSnapshotRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.Name != "before-upgrade" {
			t.Errorf("expected Name %q, got %q", "before-upgrade", req.Name)
		}

		writeJSON(t, w, http.StatusCreated, expected)
	})

	snapshot, err := c.Memory.CreateSnapshot("before-upgrade")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if snapshot.ID != "snap-new-001" {
		t.Errorf("expected ID %q, got %q", "snap-new-001", snapshot.ID)
	}
	if snapshot.Name != "before-upgrade" {
		t.Errorf("expected Name %q, got %q", "before-upgrade", snapshot.Name)
	}
}

func TestMemorySDK_CreateSnapshot_OK(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, Snapshot{ID: "snap-ok", Name: "snap-ok", EntryCount: 1})
	})

	_, err := c.Memory.CreateSnapshot("snap-ok")
	if err != nil {
		t.Fatalf("expected no error for 200, got %v", err)
	}
}

func TestMemorySDK_CreateSnapshot_EmptyName(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Memory.CreateSnapshot("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !strings.Contains(err.Error(), "snapshot name is required") {
		t.Errorf("expected 'snapshot name is required', got %q", err.Error())
	}
}

func TestMemorySDK_CreateSnapshot_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusConflict, "SNAPSHOT_EXISTS", "snapshot with this name already exists")
	})

	_, err := c.Memory.CreateSnapshot("duplicate")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "SNAPSHOT_EXISTS" {
		t.Errorf("expected Code %q, got %q", "SNAPSHOT_EXISTS", coscaErr.Code)
	}
}

func TestMemorySDK_RestoreSnapshot(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/memory/snapshots/snap-restore-1/restore")
		w.WriteHeader(http.StatusOK)
	})

	err := c.Memory.RestoreSnapshot("snap-restore-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMemorySDK_RestoreSnapshot_EmptyID(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	err := c.Memory.RestoreSnapshot("")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !strings.Contains(err.Error(), "snapshot ID is required") {
		t.Errorf("expected 'snapshot ID is required', got %q", err.Error())
	}
}

func TestMemorySDK_RestoreSnapshot_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "snapshot not found")
	})

	err := c.Memory.RestoreSnapshot("nonexistent-snap")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "NOT_FOUND" {
		t.Errorf("expected Code %q, got %q", "NOT_FOUND", coscaErr.Code)
	}
}

// =============================================================================
// Memory Type Constants
// =============================================================================

func TestMemoryTypeConstants(t *testing.T) {
	t.Parallel()

	if MemoryTypeEphemeral != "ephemeral" {
		t.Errorf("expected MemoryTypeEphemeral 'ephemeral', got %q", MemoryTypeEphemeral)
	}
	if MemoryTypePersistent != "persistent" {
		t.Errorf("expected MemoryTypePersistent 'persistent', got %q", MemoryTypePersistent)
	}
	if MemoryTypeWorking != "working" {
		t.Errorf("expected MemoryTypeWorking 'working', got %q", MemoryTypeWorking)
	}
	if MemoryTypeLongTerm != "long_term" {
		t.Errorf("expected MemoryTypeLongTerm 'long_term', got %q", MemoryTypeLongTerm)
	}
	if MemoryTypeSemantic != "semantic" {
		t.Errorf("expected MemoryTypeSemantic 'semantic', got %q", MemoryTypeSemantic)
	}
	if MemoryTypeEpisodic != "episodic" {
		t.Errorf("expected MemoryTypeEpisodic 'episodic', got %q", MemoryTypeEpisodic)
	}
}
