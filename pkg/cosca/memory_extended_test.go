package cosca

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// MemorySDK Extended Tests - Part 1: Retrieve, Delete, Promote, Stats
// =============================================================================

func TestMemorySDK_Retrieve(t *testing.T) {
	t.Parallel()

	expected := MemoryRecord{
		ID:         "mem-abc-123",
		Key:        "session-data",
		Value:      "important context for the session",
		Type:       MemoryTypeWorking,
		AgentID:    "agent-1",
		CreatedAt:  time.Date(2024, 5, 10, 14, 30, 0, 0, time.UTC),
		AccessedAt: time.Date(2024, 6, 1, 9, 0, 0, 0, time.UTC),
		Metadata: map[string]interface{}{
			"priority": "high",
		},
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/memory/get")
		writeJSON(t, w, http.StatusOK, map[string]MemoryRecord{"record": expected})
	})

	record, err := c.Memory.Retrieve("mem-abc-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if record.ID != "mem-abc-123" {
		t.Errorf("expected ID %q, got %q", "mem-abc-123", record.ID)
	}
	if record.Key != "session-data" {
		t.Errorf("expected Key %q, got %q", "session-data", record.Key)
	}
	if record.Value != "important context for the session" {
		t.Errorf("expected Value mismatch")
	}
	if record.Type != MemoryTypeWorking {
		t.Errorf("expected Type %q, got %q", MemoryTypeWorking, record.Type)
	}
	if record.AgentID != "agent-1" {
		t.Errorf("expected AgentID %q, got %q", "agent-1", record.AgentID)
	}
}

func TestMemorySDK_Retrieve_EmptyID(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Memory.Retrieve("")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !strings.Contains(err.Error(), "memory record ID is required") {
		t.Errorf("expected 'memory record ID is required', got %q", err.Error())
	}
}

func TestMemorySDK_Retrieve_NotFound(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "memory record not found")
	})

	_, err := c.Memory.Retrieve("nonexistent")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "NOT_FOUND" {
		t.Errorf("expected Code %q, got %q", "NOT_FOUND", coscaErr.Code)
	}
}

func TestMemorySDK_Retrieve_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{broken`))
	})

	_, err := c.Memory.Retrieve("test-id")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("expected 'failed to decode', got %q", err.Error())
	}
}

func TestMemorySDK_RetrieveWithLayer(t *testing.T) {
	t.Parallel()

	expected := MemoryRecord{
		ID:    "mem-wl-1",
		Key:   "layer-data",
		Value: "data in layer",
		Type:  MemoryTypeSemantic,
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/memory/mem-wl-1")
		if r.URL.Query().Get("layer") != "semantic" {
			t.Errorf("expected layer 'semantic', got %q", r.URL.Query().Get("layer"))
		}
		writeJSON(t, w, http.StatusOK, expected)
	})

	record, err := c.Memory.RetrieveWithLayer("mem-wl-1", "semantic")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if record.ID != "mem-wl-1" {
		t.Errorf("expected ID %q, got %q", "mem-wl-1", record.ID)
	}
}

func TestMemorySDK_RetrieveWithLayer_Envelope(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/memory/mem-envelope-1")
		writeJSON(t, w, http.StatusOK, map[string]MemoryRecord{
			"record": {ID: "mem-envelope-1", Key: "wrapped", Value: "value"},
		})
	})

	record, err := c.Memory.RetrieveWithLayer("mem-envelope-1", "semantic")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if record.ID != "mem-envelope-1" || record.Key != "wrapped" {
		t.Fatalf("unexpected record: %+v", record)
	}
}

func TestMemorySDK_RetrieveWithLayer_EmptyID(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Memory.RetrieveWithLayer("", "working")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestMemorySDK_RetrieveWithLayer_NoLayer(t *testing.T) {
	t.Parallel()

	expected := MemoryRecord{
		ID:  "mem-no-layer",
		Key: "no-layer-key",
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/memory/mem-no-layer")
		if r.URL.Query().Get("layer") != "" {
			t.Errorf("expected no layer query param, got %q", r.URL.Query().Get("layer"))
		}
		writeJSON(t, w, http.StatusOK, expected)
	})

	_, err := c.Memory.RetrieveWithLayer("mem-no-layer", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMemorySDK_Delete(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodDelete, "/v1/memory/delete")

		var req deleteMemoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.ID != "mem-to-delete" {
			t.Errorf("expected ID %q, got %q", "mem-to-delete", req.ID)
		}
		if req.Layer != "working" {
			t.Errorf("expected Layer %q, got %q", "working", req.Layer)
		}

		w.WriteHeader(http.StatusOK)
	})

	err := c.Memory.Delete("mem-to-delete", "working")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMemorySDK_Delete_NoContent(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	err := c.Memory.Delete("mem-clean", "")
	if err != nil {
		t.Fatalf("expected no error for 204, got %v", err)
	}
}

func TestMemorySDK_Delete_EmptyID(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	err := c.Memory.Delete("", "working")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !strings.Contains(err.Error(), "memory record ID is required") {
		t.Errorf("expected 'memory record ID is required', got %q", err.Error())
	}
}

func TestMemorySDK_Delete_EmptyLayer(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req deleteMemoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.Layer != "" {
			t.Errorf("expected empty Layer, got %q", req.Layer)
		}
		w.WriteHeader(http.StatusOK)
	})

	err := c.Memory.Delete("mem-all-layers", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMemorySDK_Delete_NotFound(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "record does not exist")
	})

	err := c.Memory.Delete("ghost", "working")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "NOT_FOUND" {
		t.Errorf("expected Code %q, got %q", "NOT_FOUND", coscaErr.Code)
	}
}

func TestMemorySDK_Promote(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/memory/promote")

		var req promoteMemoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.ID != "mem-promote-1" {
			t.Errorf("expected ID %q, got %q", "mem-promote-1", req.ID)
		}
		if req.FromLayer != "working" {
			t.Errorf("expected FromLayer %q, got %q", "working", req.FromLayer)
		}
		if req.ToLayer != "long_term" {
			t.Errorf("expected ToLayer %q, got %q", "long_term", req.ToLayer)
		}

		w.WriteHeader(http.StatusOK)
	})

	err := c.Memory.Promote("mem-promote-1", "working", "long_term")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMemorySDK_Promote_EmptyID(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	err := c.Memory.Promote("", "working", "long_term")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestMemorySDK_Promote_EmptyFromLayer(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	err := c.Memory.Promote("id", "", "long_term")
	if err == nil {
		t.Fatal("expected error for empty fromLayer")
	}
	if !strings.Contains(err.Error(), "source layer is required") {
		t.Errorf("expected 'source layer is required', got %q", err.Error())
	}
}

func TestMemorySDK_Promote_EmptyToLayer(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	err := c.Memory.Promote("id", "working", "")
	if err == nil {
		t.Fatal("expected error for empty toLayer")
	}
	if !strings.Contains(err.Error(), "target layer is required") {
		t.Errorf("expected 'target layer is required', got %q", err.Error())
	}
}

func TestMemorySDK_Promote_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusBadRequest, "INVALID_LAYER", "target layer does not exist")
	})

	err := c.Memory.Promote("id", "working", "invalid")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "INVALID_LAYER" {
		t.Errorf("expected Code %q, got %q", "INVALID_LAYER", coscaErr.Code)
	}
}

func TestMemorySDK_Stats(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/memory/stats")
		writeJSON(t, w, http.StatusOK, MemoryStats{
			TotalEntries: 5432,
			Layers:       3,
			SizeBytes:    104857600,
		})
	})

	stats, err := c.Memory.Stats()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stats.TotalEntries != 5432 {
		t.Errorf("expected TotalEntries %d, got %d", 5432, stats.TotalEntries)
	}
	if stats.Layers != 3 {
		t.Errorf("expected Layers %d, got %d", 3, stats.Layers)
	}
	if stats.SizeBytes != 104857600 {
		t.Errorf("expected SizeBytes %d, got %d", 104857600, stats.SizeBytes)
	}
}

func TestMemorySDK_Stats_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusInternalServerError, "STATS_ERROR", "cannot compute memory stats")
	})

	_, err := c.Memory.Stats()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMemorySDK_Stats_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{bad`))
	})

	_, err := c.Memory.Stats()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("expected 'failed to decode', got %q", err.Error())
	}
}
