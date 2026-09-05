package cosca

import (
	"net/http"
	"strings"
	"testing"
)

// =============================================================================
// GraphSDK Tests
// =============================================================================

func TestGraphSDK_QueryRelations(t *testing.T) {
	t.Parallel()

	expected := []Relation{
		{
			ID:          "rel-1",
			SourceID:    "pkg-1",
			SourceType:  "package",
			SourceLabel: "cosca/pkg/core",
			TargetID:    "pkg-2",
			TargetType:  "package",
			TargetLabel: "cosca/pkg/sdk",
			Type:        "depends_on",
			Weight:      0.85,
			Properties: map[string]interface{}{
				"coupling": "high",
			},
		},
		{
			ID:         "rel-2",
			SourceID:   "fn-1",
			SourceType: "function",
			TargetID:   "fn-2",
			TargetType: "function",
			Type:       "calls",
			Weight:     0.5,
		},
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/graph/relations")
		if r.URL.Query().Get("entity") != "cosca/pkg/core" {
			t.Errorf("expected entity 'cosca/pkg/core', got %q", r.URL.Query().Get("entity"))
		}
		writeJSON(t, w, http.StatusOK, relationsResponse{
			Relations: expected,
			Total:     2,
		})
	})

	relations, err := c.Graph.QueryRelations("cosca/pkg/core")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(relations) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(relations))
	}

	r0 := relations[0]
	if r0.ID != "rel-1" {
		t.Errorf("expected ID %q, got %q", "rel-1", r0.ID)
	}
	if r0.SourceID != "pkg-1" {
		t.Errorf("expected SourceID %q, got %q", "pkg-1", r0.SourceID)
	}
	if r0.TargetID != "pkg-2" {
		t.Errorf("expected TargetID %q, got %q", "pkg-2", r0.TargetID)
	}
	if r0.Type != "depends_on" {
		t.Errorf("expected Type %q, got %q", "depends_on", r0.Type)
	}
	if r0.Weight != 0.85 {
		t.Errorf("expected Weight %f, got %f", 0.85, r0.Weight)
	}
	if r0.SourceLabel != "cosca/pkg/core" {
		t.Errorf("expected SourceLabel %q, got %q", "cosca/pkg/core", r0.SourceLabel)
	}
}

func TestGraphSDK_QueryRelations_EmptyEntity(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for empty entity")
	})

	_, err := c.Graph.QueryRelations("")
	if err == nil {
		t.Fatal("expected error for empty entity")
	}
	if !strings.Contains(err.Error(), "entity identifier is required") {
		t.Errorf("expected 'entity identifier is required', got %q", err.Error())
	}
}

func TestGraphSDK_QueryRelations_NotFound(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "entity not found in graph")
	})

	_, err := c.Graph.QueryRelations("nonexistent")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", coscaErr.StatusCode)
	}
	if coscaErr.Code != "NOT_FOUND" {
		t.Errorf("expected Code %q, got %q", "NOT_FOUND", coscaErr.Code)
	}
}

func TestGraphSDK_QueryRelations_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	})

	_, err := c.Graph.QueryRelations("test")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("expected 'failed to decode', got %q", err.Error())
	}
}

func TestGraphSDK_QueryRelations_EmptyResult(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, relationsResponse{
			Relations: []Relation{},
			Total:     0,
		})
	})

	relations, err := c.Graph.QueryRelations("isolated-entity")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(relations) != 0 {
		t.Errorf("expected 0 relations, got %d", len(relations))
	}
}

func TestGraphSDK_GetGraphStats(t *testing.T) {
	t.Parallel()

	expected := GraphStats{
		NodeCount:            150,
		EdgeCount:            423,
		NodeTypes:            map[string]int{"package": 80, "function": 50, "class": 20},
		EdgeTypes:            map[string]int{"depends_on": 200, "calls": 150, "implements": 73},
		AvgDegree:            5.64,
		Density:              0.037,
		ComponentCount:       3,
		LargestComponentSize: 142,
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/graph/stats")
		writeJSON(t, w, http.StatusOK, expected)
	})

	stats, err := c.Graph.GetGraphStats()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stats.NodeCount != 150 {
		t.Errorf("expected NodeCount %d, got %d", 150, stats.NodeCount)
	}
	if stats.EdgeCount != 423 {
		t.Errorf("expected EdgeCount %d, got %d", 423, stats.EdgeCount)
	}
	if stats.NodeTypes["package"] != 80 {
		t.Errorf("expected package node count 80, got %d", stats.NodeTypes["package"])
	}
	if stats.AvgDegree != 5.64 {
		t.Errorf("expected AvgDegree %f, got %f", 5.64, stats.AvgDegree)
	}
	if stats.Density != 0.037 {
		t.Errorf("expected Density %f, got %f", 0.037, stats.Density)
	}
	if stats.ComponentCount != 3 {
		t.Errorf("expected ComponentCount %d, got %d", 3, stats.ComponentCount)
	}
	if stats.LargestComponentSize != 142 {
		t.Errorf("expected LargestComponentSize %d, got %d", 142, stats.LargestComponentSize)
	}
}

func TestGraphSDK_GetGraphStats_ServerError(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := c.Graph.GetGraphStats()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestGraphSDK_GetGraphStats_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json`))
	})

	_, err := c.Graph.GetGraphStats()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode graph stats") {
		t.Errorf("expected 'failed to decode graph stats', got %q", err.Error())
	}
}

func TestGraphSDK_ExportGraph(t *testing.T) {
	t.Parallel()

	expectedData := []byte(`{"nodes":[{"id":"1","type":"package"}],"edges":[{"source":"1","target":"2"}]}`)

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/graph/export")
		if r.URL.Query().Get("format") != "json" {
			t.Errorf("expected format 'json', got %q", r.URL.Query().Get("format"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(expectedData)
	})

	data, err := c.Graph.ExportGraph("json")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty export data")
	}
	if string(data) != string(expectedData) {
		t.Errorf("expected %q, got %q", string(expectedData), string(data))
	}
}

func TestGraphSDK_ExportGraph_DOT(t *testing.T) {
	t.Parallel()

	expectedData := []byte(`digraph G { "node1" -> "node2"; }`)

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/graph/export")
		if r.URL.Query().Get("format") != "dot" {
			t.Errorf("expected format 'dot', got %q", r.URL.Query().Get("format"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write(expectedData)
	})

	data, err := c.Graph.ExportGraph("dot")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(data) != string(expectedData) {
		t.Errorf("expected %q, got %q", string(expectedData), string(data))
	}
}

func TestGraphSDK_ExportGraph_EmptyFormat(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for empty format")
	})

	_, err := c.Graph.ExportGraph("")
	if err == nil {
		t.Fatal("expected error for empty format")
	}
	if !strings.Contains(err.Error(), "export format is required") {
		t.Errorf("expected 'export format is required', got %q", err.Error())
	}
}

func TestGraphSDK_ExportGraph_ServerError(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := c.Graph.ExportGraph("json")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestGraphSDK_ExportGraph_NotFound(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "graph is empty")
	})

	_, err := c.Graph.ExportGraph("graphml")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", coscaErr.StatusCode)
	}
}

func TestGraphSDK_QueryRelations_ServerError(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := c.Graph.QueryRelations("test-entity")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
