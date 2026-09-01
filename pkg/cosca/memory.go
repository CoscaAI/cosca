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
//
// ESPELHA o modelo canônico internal/memory.MemoryRecord — o wire do servidor
// REST usa exatamente estes campos. Não divergir: o servidor é a fonte de
// verdade (única fonte de verdade por domínio).
type MemoryRecord struct {
	// ID is the unique memory entry identifier.
	ID string `json:"id,omitempty"`
	// Type is the memory type classification.
	Type MemoryType `json:"type,omitempty"`
	// Layer is the memory layer (session, working, long_term, episodic...).
	Layer string `json:"layer,omitempty"`
	// Scope is the memory scope (project, workspace, global...).
	Scope string `json:"scope,omitempty"`
	// Owner is the record owner (anti-IDOR).
	Owner string `json:"owner,omitempty"`
	// Agent is the owning agent's identifier.
	Agent string `json:"agent,omitempty"`
	// Content is the memory payload (string).
	Content string `json:"content"`
	// Metadata holds additional structured metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
	// CreatedAt is when the memory was created.
	CreatedAt time.Time `json:"created_at,omitempty"`
	// UpdatedAt is when the memory was last updated.
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	// TTL is the record time-to-live.
	TTL time.Duration `json:"ttl,omitempty"`
	// Priority is the record priority.
	Priority int `json:"priority"`
	// Version is the record version.
	Version int `json:"version,omitempty"`
	// Score is the relevance score (populated in search results only).
	Score float64 `json:"score,omitempty"`
}

// Snapshot represents a point-in-time snapshot of agent memory.
//
// OBSERVAÇÃO: o servidor REST atual NÃO expõe endpoints de snapshot de
// memória. Estes tipos ficam para quando a superfície REST implementar
// /v1/memory/snapshots — hoje não são acionáveis.
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

// storeMemoryRequest é o corpo JSON de POST /v1/memory/store — espelha o
// handler.StoreRequest do servidor (api/rest/handler/memory.go).
type storeMemoryRequest struct {
	Type     string            `json:"type"`
	Layer    string            `json:"layer"`
	Scope    string            `json:"scope,omitempty"`
	Content  string            `json:"content"`
	Priority int               `json:"priority,omitempty"`
	TTL      string            `json:"ttl,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
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
// must contain at minimum a Content. If ID is omitted, the runtime generates
// one.
func (s *MemorySDK) Store(record MemoryRecord) error {
	if record.Content == "" {
		return fmt.Errorf("memory record content is required")
	}

	body := storeMemoryRequest{
		Type:     string(record.Type),
		Layer:    record.Layer,
		Scope:    record.Scope,
		Content:  record.Content,
		Priority: record.Priority,
		Metadata: record.Metadata,
	}
	if record.TTL > 0 {
		body.TTL = record.TTL.String()
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
//
// ATENÇÃO: o servidor REST atual NÃO implementa /v1/memory/snapshots — este
// método retorna erro claro em vez de chamar um endpoint inexistente. Volta a
// ser funcional quando a superfície REST expor o recurso (internal/memory tem
// SnapshotManager no motor; a exposição HTTP é que ainda não existe).
func (s *MemorySDK) ListSnapshots() ([]Snapshot, error) {
	return nil, fmt.Errorf("memory snapshots: endpoint /v1/memory/snapshots not implemented by the current server")
}

// CreateSnapshot captures a point-in-time snapshot of all memory for the
// given agent or for all agents. The snapshot can later be restored.
//
// ATENÇÃO: endpoint /v1/memory/snapshots não existe no servidor atual — ver
// ListSnapshots.
func (s *MemorySDK) CreateSnapshot(name string) (Snapshot, error) {
	return Snapshot{}, fmt.Errorf("memory snapshots: endpoint /v1/memory/snapshots not implemented by the current server")
}

// RestoreSnapshot restores agent memory from a previously created snapshot.
// This replaces the current memory state with the snapshot contents.
//
// ATENÇÃO: endpoint /v1/memory/snapshots/{id}/restore não existe no servidor
// atual — ver ListSnapshots.
func (s *MemorySDK) RestoreSnapshot(id string) error {
	return fmt.Errorf("memory snapshots: endpoint /v1/memory/snapshots/{id}/restore not implemented by the current server")
}

// RetrieveWithLayer fetches a memory record by ID, optionally restricting
// the lookup to a specific memory layer.
func (s *MemorySDK) RetrieveWithLayer(id string, layer string) (MemoryRecord, error) {
	if id == "" {
		return MemoryRecord{}, fmt.Errorf("memory record ID is required")
	}

	// Alinhado ao handler real: o servidor expõe /v1/memory/get?id=...&layer=...
	// (o endpoint legado /v1/memory/{id} não existe no servidor atual).
	path := memoryGetPath(id, layer)

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
