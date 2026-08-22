package cosca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/CoscaAI/cosca/internal/safe"
)

// =============================================================================
// Types
// =============================================================================

// Relation represents a relationship between two entities in the knowledge
// graph.
type Relation struct {
	// ID is the relation identifier.
	ID string `json:"id"`
	// SourceID is the source entity ID.
	SourceID string `json:"sourceId"`
	// SourceType is the source entity type.
	SourceType string `json:"sourceType"`
	// SourceLabel is the source entity display label.
	SourceLabel string `json:"sourceLabel,omitempty"`
	// TargetID is the target entity ID.
	TargetID string `json:"targetId"`
	// TargetType is the target entity type.
	TargetType string `json:"targetType"`
	// TargetLabel is the target entity display label.
	TargetLabel string `json:"targetLabel,omitempty"`
	// Type is the relationship type (depends_on, extends, implements, etc.).
	Type string `json:"type"`
	// Weight is the relationship weight / strength.
	Weight float64 `json:"weight,omitempty"`
	// Properties are additional edge properties.
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// GraphStats provides statistics about the knowledge graph.
type GraphStats struct {
	// NodeCount is the total number of nodes.
	NodeCount int `json:"nodeCount"`
	// EdgeCount is the total number of edges.
	EdgeCount int `json:"edgeCount"`
	// NodeTypes lists all distinct node types and their counts.
	NodeTypes map[string]int `json:"nodeTypes,omitempty"`
	// EdgeTypes lists all distinct edge types and their counts.
	EdgeTypes map[string]int `json:"edgeTypes,omitempty"`
	// AvgDegree is the average degree of nodes.
	AvgDegree float64 `json:"avgDegree"`
	// Density is the graph density (0.0 - 1.0).
	Density float64 `json:"density"`
	// ComponentCount is the number of connected components.
	ComponentCount int `json:"componentCount"`
	// LargestComponentSize is the size of the largest connected component.
	LargestComponentSize int `json:"largestComponentSize"`
}

// =============================================================================
// Response types
// =============================================================================

type relationsResponse struct {
	Relations []Relation `json:"relations"`
	Total     int        `json:"total"`
}

// =============================================================================
// GraphSDK
// =============================================================================

// GraphSDK provides methods for querying and exporting the Cosca knowledge
// graph. The graph tracks entities and their relationships across the
// codebase, providing insights into dependencies, structure, and impact
// analysis.
type GraphSDK struct {
	client *Client
}

// QueryRelations returns all relationships for a given entity. The entity
// can be any tracked node in the knowledge graph (package, class, function,
// module, etc.).
func (s *GraphSDK) QueryRelations(entity string) ([]Relation, error) {
	if entity == "" {
		return nil, fmt.Errorf("entity identifier is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("/v1/graph/relations?entity=%s", entity),
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, s.decodeError(resp)
	}

	var result relationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode relations: %w", err)
	}

	return result.Relations, nil
}

// GetGraphStats returns aggregate statistics about the knowledge graph
// including node and edge counts, type distributions, and graph metrics
// such as density and component analysis.
func (s *GraphSDK) GetGraphStats() (GraphStats, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/graph/stats",
		nil,
	)
	if err != nil {
		return GraphStats{}, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return GraphStats{}, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return GraphStats{}, s.decodeError(resp)
	}

	var stats GraphStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return GraphStats{}, fmt.Errorf("failed to decode graph stats: %w", err)
	}

	return stats, nil
}

// ExportGraph exports the knowledge graph in the specified format. Supported
// formats include:
//   - "json"     – full JSON representation of nodes and edges
//   - "graphml"  – GraphML XML format for import into graph tools
//   - "dot"      – Graphviz DOT format for visualisation
//   - "csv"      – CSV with nodes and edges
func (s *GraphSDK) ExportGraph(format string) ([]byte, error) {
	if format == "" {
		return nil, fmt.Errorf("export format is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("/v1/graph/export?format=%s", format),
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, s.decodeError(resp)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return buf.Bytes(), nil
}

// decodeError reads an error response body and returns it as a formatted error.
func (s *GraphSDK) decodeError(resp *http.Response) error {
	return s.client.decodeError(resp)
}
