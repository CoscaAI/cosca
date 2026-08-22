package cosca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
)

// =============================================================================
// Types
// =============================================================================

// SearchOptions defines parameters for standard knowledge searches.
type SearchOptions struct {
	// Limit is the maximum number of results to return.
	Limit int `json:"limit,omitempty"`
	// Offset is the pagination offset.
	Offset int `json:"offset,omitempty"`
	// MinScore is the minimum relevance score threshold (0.0 - 1.0).
	MinScore float64 `json:"minScore,omitempty"`
	// Filters are field-level filters applied to the search.
	Filters map[string]interface{} `json:"filters,omitempty"`
	// Namespace restricts search to a specific namespace.
	Namespace string `json:"namespace,omitempty"`
	// IncludeFields restricts the returned fields.
	IncludeFields []string `json:"includeFields,omitempty"`
}

// FTSOptions defines parameters for full-text search queries.
type FTSOptions struct {
	// Limit is the maximum number of results to return.
	Limit int `json:"limit,omitempty"`
	// Offset is the pagination offset.
	Offset int `json:"offset,omitempty"`
	// MinScore is the minimum relevance score threshold (0.0 - 1.0).
	MinScore float64 `json:"minScore,omitempty"`
	// Filters are field-level filters applied to the search.
	Filters map[string]interface{} `json:"filters,omitempty"`
	// Namespace restricts search to a specific namespace.
	Namespace string `json:"namespace,omitempty"`
	// IncludeFields restricts the returned fields.
	IncludeFields []string `json:"includeFields,omitempty"`
}

// VectorOptions defines parameters for vector similarity search.
type VectorOptions struct {
	// Limit is the maximum number of results to return.
	Limit int `json:"limit,omitempty"`
	// Offset is the pagination offset.
	Offset int `json:"offset,omitempty"`
	// MinScore is the minimum relevance score threshold (0.0 - 1.0).
	MinScore float64 `json:"minScore,omitempty"`
	// Vector is an optional raw embedding vector for similarity search.
	Vector []float64 `json:"vector,omitempty"`
	// Filters are field-level filters applied to the search.
	Filters map[string]interface{} `json:"filters,omitempty"`
	// Namespace restricts search to a specific namespace.
	Namespace string `json:"namespace,omitempty"`
	// IncludeFields restricts the returned fields.
	IncludeFields []string `json:"includeFields,omitempty"`
}

// HybridOptions defines parameters for hybrid (vector + keyword) searches.
type HybridOptions struct {
	// Limit is the maximum number of results.
	Limit int `json:"limit,omitempty"`
	// Offset is the pagination offset.
	Offset int `json:"offset,omitempty"`
	// MinScore is the minimum relevance threshold.
	MinScore float64 `json:"minScore,omitempty"`
	// Alpha controls the weighting between vector (1.0) and keyword (0.0).
	// Default is 0.5 (equal weighting).
	Alpha float64 `json:"alpha,omitempty"`
	// Vector is an optional raw embedding vector for similarity search.
	Vector []float64 `json:"vector,omitempty"`
	// Filters are field-level filters.
	Filters map[string]interface{} `json:"filters,omitempty"`
	// Namespace restricts search to a namespace.
	Namespace string `json:"namespace,omitempty"`
}

// SearchResult represents a single knowledge search result.
type SearchResult struct {
	// ID is the unique result identifier.
	ID string `json:"id"`
	// Score is the relevance score (0.0 - 1.0).
	Score float64 `json:"score"`
	// Content is the result content or snippet.
	Content string `json:"content,omitempty"`
	// Title is the result title.
	Title string `json:"title,omitempty"`
	// Source is the result source type (file, url, database, etc.).
	Source string `json:"source,omitempty"`
	// SourceURI is the URI of the original source.
	SourceURI string `json:"sourceUri,omitempty"`
	// Metadata holds result-specific metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// Highlights are matching text fragments.
	Highlights []string `json:"highlights,omitempty"`
}

// KnowledgeStats provides statistics about the knowledge base.
type KnowledgeStats struct {
	// DocCount is the total number of indexed documents.
	DocCount int `json:"docCount"`
	// ChunkCount is the total number of indexed chunks.
	ChunkCount int `json:"chunkCount"`
	// IndexSize is the size of the index in bytes.
	IndexSize int64 `json:"indexSize"`
	// LastIndexed is the timestamp of the last indexing operation.
	LastIndexed time.Time `json:"lastIndexed"`
	// LastBuildDuration is the duration of the last index build.
	LastBuildDuration time.Duration `json:"lastBuildDuration"`
	// Namespaces lists the namespaces present in the index.
	Namespaces []string `json:"namespaces,omitempty"`
	// ErrorCount is the number of documents that failed to index.
	ErrorCount int `json:"errorCount"`
}

// =============================================================================
// Index Document Request / Response
// =============================================================================

type indexDocumentRequest struct {
	Path     string            `json:"path"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type indexDocumentResponse struct {
	DocumentID string `json:"documentId"`
	Chunks     int    `json:"chunks"`
	DurationMs int64  `json:"durationMs"`
}

// =============================================================================
// Search Request / Response
// =============================================================================

type searchRequest struct {
	Query         string                 `json:"query"`
	Type          string                 `json:"type"`
	Limit         int                    `json:"limit,omitempty"`
	Offset        int                    `json:"offset,omitempty"`
	MinScore      float64                `json:"minScore,omitempty"`
	Filters       map[string]interface{} `json:"filters,omitempty"`
	Namespace     string                 `json:"namespace,omitempty"`
	Alpha         float64                `json:"alpha,omitempty"`
	Vector        []float64              `json:"vector,omitempty"`
	IncludeFields []string               `json:"includeFields,omitempty"`
}

type searchResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
	TookMs  int64          `json:"tookMs"`
	HasMore bool           `json:"hasMore"`
}

// SyncResponse represents the response from a knowledge sync operation.
type SyncResponse struct {
	// Synced is the number of documents that were synced.
	Synced int `json:"synced"`
	// Updated is the number of documents that were updated.
	Updated int `json:"updated"`
	// Deleted is the number of documents that were removed.
	Deleted int `json:"deleted"`
	// Errors is the number of documents that failed to sync.
	Errors int `json:"errors"`
	// DurationMs is the sync duration in milliseconds.
	DurationMs int64 `json:"durationMs"`
	// Message is a summary message from the sync operation.
	Message string `json:"message,omitempty"`
}

// indexDocumentRecursiveRequest adds recursive support to the index request.
type indexDocumentRecursiveRequest struct {
	Path      string            `json:"path"`
	Recursive bool              `json:"recursive"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// KnowledgeSDK provides methods for interacting with the Cosca Knowledge Base.
// It supports document indexing, keyword/full-text search, vector/hybrid
// search, knowledge statistics, and index rebuild operations.
type KnowledgeSDK struct {
	client *Client
}

// IndexDocument indexes a document from the given file path into the
// knowledge base. The document is parsed, chunked, and embedded for
// subsequent search operations.
func (s *KnowledgeSDK) IndexDocument(path string) error {
	if path == "" {
		return fmt.Errorf("document path is required")
	}

	body := indexDocumentRequest{Path: path}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		"/v1/knowledge/index",
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

	var result indexDocumentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// Search performs a standard knowledge-base search using the given query
// string and options. It supports keyword, full-text, and vector search
// depending on the runtime configuration.
func (s *KnowledgeSDK) Search(query string, opts SearchOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	body := searchRequest{
		Query:         query,
		Type:          "keyword",
		Limit:         opts.Limit,
		Offset:        opts.Offset,
		MinScore:      opts.MinScore,
		Filters:       opts.Filters,
		Namespace:     opts.Namespace,
		IncludeFields: opts.IncludeFields,
	}

	return s.search(body)
}

// SearchByType performs a search scoped to a specific entity type (e.g.,
// "agent", "skill", "document", "plugin").
func (s *KnowledgeSDK) SearchByType(entityType string, query string) ([]SearchResult, error) {
	if entityType == "" {
		return nil, fmt.Errorf("entity type is required")
	}
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	body := searchRequest{
		Query: query,
		Type:  "keyword",
		Filters: map[string]interface{}{
			"entityType": entityType,
		},
	}

	return s.search(body)
}

// search is the internal search implementation.
func (s *KnowledgeSDK) search(body searchRequest) ([]SearchResult, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		"/v1/knowledge/search",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, s.decodeError(resp)
	}

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	return result.Results, nil
}

// GetStats returns statistics about the knowledge base, including document
// count, chunk count, index size, and namespace information.
func (s *KnowledgeSDK) GetStats() (KnowledgeStats, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/knowledge/stats",
		nil,
	)
	if err != nil {
		return KnowledgeStats{}, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return KnowledgeStats{}, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return KnowledgeStats{}, s.decodeError(resp)
	}

	var stats KnowledgeStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return KnowledgeStats{}, fmt.Errorf("failed to decode stats: %w", err)
	}

	return stats, nil
}

// Rebuild triggers a full rebuild of the knowledge index. All documents
// are re-processed, re-chunked, and re-embedded from scratch.
func (s *KnowledgeSDK) Rebuild() error {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		"/v1/knowledge/rebuild",
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return s.decodeError(resp)
	}

	return nil
}

// SearchFTS performs a full-text search with the given query. It uses
// the FTS engine exclusively, bypassing vector similarity.
func (s *KnowledgeSDK) SearchFTS(query string, opts FTSOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	body := searchRequest{
		Query:         query,
		Type:          "fts",
		Limit:         opts.Limit,
		Offset:        opts.Offset,
		MinScore:      opts.MinScore,
		Filters:       opts.Filters,
		Namespace:     opts.Namespace,
		IncludeFields: opts.IncludeFields,
	}

	return s.search(body)
}

// SearchVector performs a vector-only similarity search. The query is
// embedded and matched against the vector index. If a raw Vector is
// supplied in opts, it is used directly instead of embedding the query.
func (s *KnowledgeSDK) SearchVector(query string, opts VectorOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	body := searchRequest{
		Query:         query,
		Type:          "vector",
		Limit:         opts.Limit,
		Offset:        opts.Offset,
		MinScore:      opts.MinScore,
		Vector:        opts.Vector,
		Filters:       opts.Filters,
		Namespace:     opts.Namespace,
		IncludeFields: opts.IncludeFields,
	}

	return s.search(body)
}

// HybridSearch performs a hybrid search combining vector similarity and
// keyword matching. The alpha parameter in HybridOptions controls the
// weighting between the two: 1.0 = pure vector, 0.0 = pure keyword.
func (s *KnowledgeSDK) HybridSearch(query string, opts HybridOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	alpha := opts.Alpha
	if alpha == 0 {
		alpha = 0.5 // default equal weighting
	}

	body := searchRequest{
		Query:     query,
		Type:      "hybrid",
		Limit:     opts.Limit,
		Offset:    opts.Offset,
		MinScore:  opts.MinScore,
		Alpha:     alpha,
		Vector:    opts.Vector,
		Filters:   opts.Filters,
		Namespace: opts.Namespace,
	}

	return s.search(body)
}

// Sync triggers a knowledge base synchronisation. It rescans configured
// source directories, adds new files, updates changed files, and removes
// entries for deleted files.
func (s *KnowledgeSDK) Sync() (*SyncResponse, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		"/v1/knowledge/sync",
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

	var result SyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode sync response: %w", err)
	}

	return &result, nil
}

// IndexDocumentRecursive indexes a document at the given path, recursively
// including all subdirectories when recursive is true.
func (s *KnowledgeSDK) IndexDocumentRecursive(path string, recursive bool) error {
	if path == "" {
		return fmt.Errorf("document path is required")
	}

	body := indexDocumentRecursiveRequest{
		Path:      path,
		Recursive: recursive,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		"/v1/knowledge/index",
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

// decodeError reads an error response body and returns it as a formatted error.
func (s *KnowledgeSDK) decodeError(resp *http.Response) error {
	return s.client.decodeError(resp)
}
