package cosca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
)

// =============================================================================
// Types
// =============================================================================

// MemoryType categorises the type of memory storage.
type MemoryType string

// Predefined memory types.
const (
	MemoryTypeEphemeral  MemoryType = "ephemeral"
	MemoryTypePersistent MemoryType = "persistent"
	MemoryTypeWorking    MemoryType = "working"
	MemoryTypeLongTerm   MemoryType = "long_term"
	MemoryTypeSemantic   MemoryType = "semantic"
	MemoryTypeEpisodic   MemoryType = "episodic"
)

// MemoryRecord represents a single memory entry stored by an agent.
type MemoryRecord struct {
	// ID is the unique memory entry identifier.
	ID string `json:"id,omitempty"`
	// AgentID is the owning agent's identifier.
	AgentID string `json:"agentId,omitempty"`
	// Type is the memory type classification.
	Type MemoryType `json:"type,omitempty"`
	// Key is the memory key for lookups.
	Key string `json:"key"`
	// Value is the memory value. Can be any JSON-serialisable type.
	Value interface{} `json:"value"`
	// Embedding is an optional vector embedding.
	Embedding []float64 `json:"embedding,omitempty"`
	// Metadata holds additional structured metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// CreatedAt is when the memory was created.
	CreatedAt time.Time `json:"createdAt,omitempty"`
	// AccessedAt is when the memory was last read.
	AccessedAt time.Time `json:"accessedAt,omitempty"`
	// Score is the relevance score (populated in search results).
	Score float64 `json:"score,omitempty"`
}

// Snapshot represents a point-in-time snapshot of agent memory.
type Snapshot struct {
	// ID is the snapshot identifier.
	ID string `json:"id"`
	// Name is a human-readable snapshot name.
	Name string `json:"name"`
	// AgentID is the agent whose memory was snapshotted.
	AgentID string `json:"agentId,omitempty"`
	// Size is the snapshot size in bytes.
	Size int64 `json:"size"`
	// EntryCount is the number of memory entries in the snapshot.
	EntryCount int `json:"entryCount"`
	// CreatedAt is when the snapshot was taken.
	CreatedAt time.Time `json:"createdAt"`
	// Labels are optional labels for categorisation.
	Labels []string `json:"labels,omitempty"`
}

// =============================================================================
// Request / Response types
// =============================================================================

type storeMemoryRequest struct {
	AgentID   string                 `json:"agentId,omitempty"`
	Type      MemoryType             `json:"type,omitempty"`
	Key       string                 `json:"key"`
	Value     interface{}            `json:"value"`
	Embedding []float64              `json:"embedding,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type memorySearchRequest struct {
	Query string `json:"query"`
	Layer string `json:"layer,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type memorySearchResponse struct {
	Records []MemoryRecord `json:"records"`
	Total   int            `json:"total"`
}

type memoryGetResponse struct {
	Record MemoryRecord `json:"record"`
}

type snapshotListResponse struct {
	Snapshots []Snapshot `json:"snapshots"`
	Total     int        `json:"total"`
}

type createSnapshotRequest struct {
	Name   string   `json:"name"`
	Labels []string `json:"labels,omitempty"`
}

type deleteMemoryRequest struct {
	ID    string `json:"id"`
	Layer string `json:"layer,omitempty"`
}

type promoteMemoryRequest struct {
	ID        string `json:"id"`
	FromLayer string `json:"fromLayer"`
	ToLayer   string `json:"toLayer"`
}

// MemoryStats contains statistics about the memory subsystem.
type MemoryStats struct {
	// TotalEntries is the total number of memory entries.
	TotalEntries int `json:"totalEntries"`
	// Layers is the number of active memory layers.
	Layers int `json:"layers"`
	// SizeBytes is the approximate memory store size in bytes.
	SizeBytes int64 `json:"sizeBytes"`
	// LayerStats contains per-layer statistics.
	LayerStats map[string]struct {
		Entries int `json:"entries"`
	} `json:"layerStats,omitempty"`
}

// =============================================================================
// MemorySDK
// =============================================================================

// MemorySDK provides methods for interacting with the Cosca Memory subsystem.
// It enables agents to store, retrieve, search, and snapshot their memories
// across different memory types (ephemeral, persistent, working, etc.).
type MemorySDK struct {
	client *Client
}

// Store persists a memory record in the specified memory layer. The record
// must contain at minimum a Key and Value. If ID is omitted, the runtime
// generates one.
func (s *MemorySDK) Store(record MemoryRecord) error {
	if record.Key == "" {
		return fmt.Errorf("memory record key is required")
	}
	if record.Value == nil {
		return fmt.Errorf("memory record value is required")
	}

	body := storeMemoryRequest{
		AgentID:   record.AgentID,
		Type:      record.Type,
		Key:       record.Key,
		Value:     record.Value,
		Embedding: record.Embedding,
		Metadata:  record.Metadata,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal memory record: %w", err)
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		"/v1/memory/store",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.doRequest(req)
	if err != nil {
		return err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return s.decodeError(resp)
	}

	return nil
}

// Retrieve fetches a single memory record by its unique identifier.
func (s *MemorySDK) Retrieve(id string, layers ...string) (MemoryRecord, error) {
	if id == "" {
		return MemoryRecord{}, fmt.Errorf("memory record ID is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		memoryGetPath(id, firstLayer(layers)),
		nil,
	)
	if err != nil {
		return MemoryRecord{}, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return MemoryRecord{}, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return MemoryRecord{}, s.decodeError(resp)
	}

	var result memoryGetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return MemoryRecord{}, fmt.Errorf("failed to decode memory record: %w", err)
	}

	return result.Record, nil
}

// Search performs a semantic/keyword search across memory records in the
// specified memory layer (e.g., "working", "long_term", "episodic").
// If layer is empty, all layers are searched.
func (s *MemorySDK) Search(query string, layer string) ([]MemoryRecord, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/memory/search?"+url.Values{"query": []string{query}, "layer": []string{layer}, "layers": []string{layer}}.Encode(),
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

	var result memorySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode memory search results: %w", err)
	}

	return result.Records, nil
}

// ListSnapshots returns all available memory snapshots.
func (s *MemorySDK) ListSnapshots() ([]Snapshot, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/memory/snapshots",
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

	var result snapshotListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot list: %w", err)
	}

	return result.Snapshots, nil
}

// CreateSnapshot captures a point-in-time snapshot of all memory for the
// given agent or for all agents. The snapshot can later be restored.
func (s *MemorySDK) CreateSnapshot(name string) (Snapshot, error) {
	if name == "" {
		return Snapshot{}, fmt.Errorf("snapshot name is required")
	}

	body := createSnapshotRequest{Name: name}
	payload, err := json.Marshal(body)
	if err != nil {
		return Snapshot{}, fmt.Errorf("failed to marshal snapshot request: %w", err)
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		"/v1/memory/snapshots",
		bytes.NewReader(payload),
	)
	if err != nil {
		return Snapshot{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.doRequest(req)
	if err != nil {
		return Snapshot{}, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return Snapshot{}, s.decodeError(resp)
	}

	var snapshot Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("failed to decode snapshot: %w", err)
	}

	return snapshot, nil
}

// RestoreSnapshot restores agent memory from a previously created snapshot.
// This replaces the current memory state with the snapshot contents.
func (s *MemorySDK) RestoreSnapshot(id string) error {
	if id == "" {
		return fmt.Errorf("snapshot ID is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		fmt.Sprintf("/v1/memory/snapshots/%s/restore", id),
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return s.decodeError(resp)
	}

	return nil
}

// RetrieveWithLayer fetches a memory record by ID, optionally restricting
// the lookup to a specific memory layer.
func (s *MemorySDK) RetrieveWithLayer(id string, layer string) (MemoryRecord, error) {
	if id == "" {
		return MemoryRecord{}, fmt.Errorf("memory record ID is required")
	}

	// Preserve the legacy endpoint as an alias; the primary Retrieve method
	// uses /v1/memory/get and supports the handler's layer query parameter.
	path := fmt.Sprintf("/v1/memory/%s", url.QueryEscape(id))
	if layer != "" {
		path += "?layer=" + url.QueryEscape(layer)
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		path,
		nil,
	)
	if err != nil {
		return MemoryRecord{}, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return MemoryRecord{}, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return MemoryRecord{}, s.decodeError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return MemoryRecord{}, fmt.Errorf("failed to decode memory record: %w", err)
	}

	// The legacy endpoint returned the record directly, while newer handlers
	// wrap it in {"record": ...}. Accept both representations so SDK clients
	// remain compatible with either server version.
	var envelope struct {
		Record json.RawMessage `json:"record"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return MemoryRecord{}, fmt.Errorf("failed to decode memory record: %w", err)
	}
	var record MemoryRecord
	if len(envelope.Record) > 0 && string(envelope.Record) != "null" {
		if err := json.Unmarshal(envelope.Record, &record); err != nil {
			return MemoryRecord{}, fmt.Errorf("failed to decode memory record: %w", err)
		}
	} else if err := json.Unmarshal(body, &record); err != nil {
		return MemoryRecord{}, fmt.Errorf("failed to decode memory record: %w", err)
	}

	return record, nil
}

func firstLayer(layers []string) string {
	if len(layers) == 0 {
		// Preserve the historical one-argument API while satisfying the REST
		// handler's required layer parameter.
		return "session"
	}
	return layers[0]
}

func memoryGetPath(id, layer string) string {
	path := fmt.Sprintf("/v1/memory/get?id=%s", url.QueryEscape(id))
	if layer != "" {
		path += "&layer=" + url.QueryEscape(layer)
	}
	return path
}

// Delete removes a memory record from the specified layer. If layer is empty,
// the record is removed from all layers.
func (s *MemorySDK) Delete(id string, layer string) error {
	if id == "" {
		return fmt.Errorf("memory record ID is required")
	}

	body := deleteMemoryRequest{
		ID:    id,
		Layer: layer,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal delete request: %w", err)
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodDelete,
		"/v1/memory/delete",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.doRequest(req)
	if err != nil {
		return err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return s.decodeError(resp)
	}

	return nil
}

// Promote moves a memory record from one layer to another (e.g., from
// "working" to "long_term").
func (s *MemorySDK) Promote(id string, fromLayer string, toLayer string) error {
	if id == "" {
		return fmt.Errorf("memory record ID is required")
	}
	if fromLayer == "" {
		return fmt.Errorf("source layer is required")
	}
	if toLayer == "" {
		return fmt.Errorf("target layer is required")
	}

	body := promoteMemoryRequest{
		ID:        id,
		FromLayer: fromLayer,
		ToLayer:   toLayer,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal promote request: %w", err)
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		"/v1/memory/promote",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.doRequest(req)
	if err != nil {
		return err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return s.decodeError(resp)
	}

	return nil
}

// Stats returns aggregate statistics about the memory subsystem, including
// total entries, layer counts, and total size.
func (s *MemorySDK) Stats() (*MemoryStats, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/memory/stats",
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

	var stats MemoryStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode memory stats: %w", err)
	}

	return &stats, nil
}

// decodeError reads an error response body and returns it as a formatted error.
func (s *MemorySDK) decodeError(resp *http.Response) error {
	return s.client.decodeError(resp)
}
