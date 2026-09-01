package cosca

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// KnowledgeSDK Extended Tests
// =============================================================================

func TestKnowledgeSDK_IndexDocument(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/knowledge/index")

		var req indexDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.Path != "/path/to/document.md" {
			t.Errorf("expected Path %q, got %q", "/path/to/document.md", req.Path)
		}

		writeJSON(t, w, http.StatusOK, indexDocumentResponse{
			DocumentID: "doc-001",
			Chunks:     15,
			DurationMs: 234,
		})
	})

	err := c.Knowledge.IndexDocument("/path/to/document.md")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestKnowledgeSDK_IndexDocument_Created(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusCreated, indexDocumentResponse{
			DocumentID: "doc-002",
			Chunks:     8,
			DurationMs: 120,
		})
	})

	err := c.Knowledge.IndexDocument("/new/doc.md")
	if err != nil {
		t.Fatalf("expected no error for 201, got %v", err)
	}
}

func TestKnowledgeSDK_IndexDocument_EmptyPath(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	err := c.Knowledge.IndexDocument("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if !strings.Contains(err.Error(), "document path is required") {
		t.Errorf("expected 'document path is required', got %q", err.Error())
	}
}

func TestKnowledgeSDK_IndexDocument_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusBadRequest, "INVALID_PATH", "file does not exist")
	})

	err := c.Knowledge.IndexDocument("/nonexistent/file.md")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "INVALID_PATH" {
		t.Errorf("expected Code %q, got %q", "INVALID_PATH", coscaErr.Code)
	}
}

func TestKnowledgeSDK_GetStats(t *testing.T) {
	t.Parallel()

	expected := KnowledgeStats{
		DocCount:          1250,
		ChunkCount:        8750,
		IndexSize:         52428800,
		LastIndexed:       time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		LastBuildDuration: 45 * time.Second,
		Namespaces:        []string{"code", "docs", "issues"},
		ErrorCount:        3,
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/knowledge/stats")
		writeJSON(t, w, http.StatusOK, expected)
	})

	stats, err := c.Knowledge.GetStats()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stats.DocCount != 1250 {
		t.Errorf("expected DocCount %d, got %d", 1250, stats.DocCount)
	}
	if stats.ChunkCount != 8750 {
		t.Errorf("expected ChunkCount %d, got %d", 8750, stats.ChunkCount)
	}
	if stats.IndexSize != 52428800 {
		t.Errorf("expected IndexSize %d, got %d", 52428800, stats.IndexSize)
	}
	if stats.ErrorCount != 3 {
		t.Errorf("expected ErrorCount %d, got %d", 3, stats.ErrorCount)
	}
	if len(stats.Namespaces) != 3 {
		t.Errorf("expected 3 namespaces, got %d", len(stats.Namespaces))
	}
}

func TestKnowledgeSDK_GetStats_Error(t *testing.T) {
	t.Parallel()

	// Use a 4xx error to get CoscaError (5xx are retried by doRequest and return plain error).
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "INDEX_UNAVAILABLE", "knowledge index is not ready")
	})

	_, err := c.Knowledge.GetStats()
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "INDEX_UNAVAILABLE" {
		t.Errorf("expected Code %q, got %q", "INDEX_UNAVAILABLE", coscaErr.Code)
	}
}

func TestKnowledgeSDK_GetStats_ServerError(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := c.Knowledge.GetStats()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	// 5xx from doRequest returns a plain error, not CoscaError.
	if _, ok := err.(*CoscaError); ok {
		t.Errorf("expected non-CoscaError for 5xx, got *CoscaError")
	}
}

func TestKnowledgeSDK_Rebuild(t *testing.T) {
	t.Parallel()

	// Rebuild delega ao Sync (endpoint real do servidor) — /v1/knowledge/rebuild
	// NÃO existe no servidor atual.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/knowledge/sync")
		writeJSON(t, w, http.StatusOK, SyncResponse{Synced: 1})
	})

	err := c.Knowledge.Rebuild()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestKnowledgeSDK_Rebuild_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusConflict, "SYNC_IN_PROGRESS", "a sync is already running")
	})

	err := c.Knowledge.Rebuild()
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "SYNC_IN_PROGRESS" {
		t.Errorf("expected Code %q, got %q", "SYNC_IN_PROGRESS", coscaErr.Code)
	}
}

func TestKnowledgeSDK_SearchByType(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/knowledge/search")

		var req searchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.Query != "setup" {
			t.Errorf("expected Query %q, got %q", "setup", req.Query)
		}
		if len(req.Types) != 1 || req.Types[0] != "agent" {
			t.Errorf("expected Types ['agent'], got %v", req.Types)
		}

		writeJSON(t, w, http.StatusOK, searchResponse{
			Results: []SearchResult{
				{ID: "agent-1", Score: 0.99, Content: "Setup agent for CI/CD"},
			},
			Total:  1,
			TookMs: 12,
		})
	})

	results, err := c.Knowledge.SearchByType("agent", "setup")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "agent-1" {
		t.Errorf("expected ID %q, got %q", "agent-1", results[0].ID)
	}
}

func TestKnowledgeSDK_SearchByType_EmptyType(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Knowledge.SearchByType("", "query")
	if err == nil {
		t.Fatal("expected error for empty entity type")
	}
	if !strings.Contains(err.Error(), "entity type is required") {
		t.Errorf("expected 'entity type is required', got %q", err.Error())
	}
}

func TestKnowledgeSDK_SearchByType_EmptyQuery(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Knowledge.SearchByType("agent", "")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "search query is required") {
		t.Errorf("expected 'search query is required', got %q", err.Error())
	}
}

func TestKnowledgeSDK_SearchFTS(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/knowledge/search")

		var req searchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.EnableFTS == nil || !*req.EnableFTS {
			t.Error("expected EnableFTS=true")
		}
		if req.EnableVec == nil || *req.EnableVec {
			t.Error("expected EnableVec=false")
		}

		writeJSON(t, w, http.StatusOK, searchResponse{
			Results: []SearchResult{
				{ID: "fts-1", Score: 0.88, Content: "Full-text match result"},
			},
			Total:  1,
			TookMs: 25,
		})
	})

	results, err := c.Knowledge.SearchFTS("full text search", FTSOptions{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestKnowledgeSDK_SearchFTS_EmptyQuery(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Knowledge.SearchFTS("", FTSOptions{})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "search query is required") {
		t.Errorf("expected 'search query is required', got %q", err.Error())
	}
}

func TestKnowledgeSDK_SearchVector(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/knowledge/search")

		var req searchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.EnableVec == nil || !*req.EnableVec {
			t.Error("expected EnableVec=true")
		}
		if req.EnableFTS == nil || *req.EnableFTS {
			t.Error("expected EnableFTS=false")
		}

		writeJSON(t, w, http.StatusOK, searchResponse{
			Results: []SearchResult{
				{ID: "vec-1", Score: 0.92, Content: "Similar vector result"},
			},
			Total:  1,
			TookMs: 18,
		})
	})

	results, err := c.Knowledge.SearchVector("concept search", VectorOptions{
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestKnowledgeSDK_SearchVector_EmptyQuery(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Knowledge.SearchVector("", VectorOptions{})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestKnowledgeSDK_HybridSearch(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/knowledge/search")

		var req searchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.EnableFTS == nil || !*req.EnableFTS {
			t.Error("expected EnableFTS=true (hybrid)")
		}
		if req.EnableVec == nil || !*req.EnableVec {
			t.Error("expected EnableVec=true (hybrid)")
		}

		writeJSON(t, w, http.StatusOK, searchResponse{
			Results: []SearchResult{
				{ID: "hyb-1", Score: 0.96, Content: "Hybrid search result"},
			},
			Total:  1,
			TookMs: 30,
		})
	})

	results, err := c.Knowledge.HybridSearch("hybrid query", HybridOptions{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestKnowledgeSDK_HybridSearch_EmptyQuery(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Knowledge.HybridSearch("", HybridOptions{})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestKnowledgeSDK_Sync(t *testing.T) {
	t.Parallel()

	expected := SyncResponse{
		Synced:     42,
		Updated:    15,
		Deleted:    3,
		Errors:     1,
		DurationMs: 4521,
		Message:    "Sync completed with 1 error",
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/knowledge/sync")
		writeJSON(t, w, http.StatusOK, expected)
	})

	resp, err := c.Knowledge.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Synced != 42 {
		t.Errorf("expected Synced %d, got %d", 42, resp.Synced)
	}
	if resp.Updated != 15 {
		t.Errorf("expected Updated %d, got %d", 15, resp.Updated)
	}
	if resp.Deleted != 3 {
		t.Errorf("expected Deleted %d, got %d", 3, resp.Deleted)
	}
	if resp.Errors != 1 {
		t.Errorf("expected Errors %d, got %d", 1, resp.Errors)
	}
	if resp.DurationMs != 4521 {
		t.Errorf("expected DurationMs %d, got %d", 4521, resp.DurationMs)
	}
	if resp.Message != "Sync completed with 1 error" {
		t.Errorf("expected Message %q, got %q", "Sync completed with 1 error", resp.Message)
	}
}

func TestKnowledgeSDK_Sync_Error(t *testing.T) {
	t.Parallel()

	// Use a 4xx error to get CoscaError (5xx are retried by doRequest and return plain error).
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusConflict, "SYNC_FAILED", "database locked")
	})

	_, err := c.Knowledge.Sync()
	if err == nil {
		t.Fatal("expected error")
	}
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "SYNC_FAILED" {
		t.Errorf("expected Code %q, got %q", "SYNC_FAILED", coscaErr.Code)
	}
}

func TestKnowledgeSDK_IndexDocumentRecursive(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/knowledge/index")

		var req indexDocumentRecursiveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.Path != "/docs" {
			t.Errorf("expected Path %q, got %q", "/docs", req.Path)
		}
		if !req.Recursive {
			t.Error("expected Recursive to be true")
		}

		w.WriteHeader(http.StatusCreated)
	})

	err := c.Knowledge.IndexDocumentRecursive("/docs", true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestKnowledgeSDK_IndexDocumentRecursive_NonRecursive(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req indexDocumentRecursiveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.Recursive {
			t.Error("expected Recursive to be false")
		}
		w.WriteHeader(http.StatusOK)
	})

	err := c.Knowledge.IndexDocumentRecursive("/single-file.md", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestKnowledgeSDK_IndexDocumentRecursive_EmptyPath(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	err := c.Knowledge.IndexDocumentRecursive("", true)
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if !strings.Contains(err.Error(), "document path is required") {
		t.Errorf("expected 'document path is required', got %q", err.Error())
	}
}

func TestKnowledgeSDK_IndexDocumentRecursive_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusForbidden, "ACCESS_DENIED", "cannot read directory")
	})

	err := c.Knowledge.IndexDocumentRecursive("/protected", true)
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 403 {
		t.Errorf("expected status 403, got %d", coscaErr.StatusCode)
	}
}
