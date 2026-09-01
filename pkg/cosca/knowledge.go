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

// boolPtr é helper para os flags enable_fts/enable_vector do SearchRequest do
// servidor (campos *bool com omitempty).
func boolPtr(b bool) *bool { return &b }

// SearchOptions defines parameters for standard knowledge searches.
type SearchOptions struct {
	// Limit is the maximum number of results to return.
	Limit int `json:"limit,omitempty"`
	// Offset is the pagination offset.
	Offset int `json:"offset,omitempty"`
	// MinScore is the minimum relevance score threshold (0.0 - 1.0).
	MinScore float64 `json:"min_score,omitempty"`
	// Types restringe a busca por tipos de entidade.
	Types []string `json:"types,omitempty"`
	// PathFilter restringe a busca a um caminho.
	PathFilter string `json:"path_filter,omitempty"`
}

// FTSOptions defines parameters for full-text search queries.
type FTSOptions struct {
	// Limit is the maximum number of results to return.
	Limit int `json:"limit,omitempty"`
	// Offset is the pagination offset.
	Offset int `json:"offset,omitempty"`
	// MinScore is the minimum relevance score threshold (0.0 - 1.0).
	MinScore float64 `json:"min_score,omitempty"`
	// Types restringe a busca por tipos de entidade.
	Types []string `json:"types,omitempty"`
}

// VectorOptions defines parameters for vector similarity search.
type VectorOptions struct {
	// Limit is the maximum number of results to return.
	Limit int `json:"limit,omitempty"`
	// Offset is the pagination offset.
	Offset int `json:"offset,omitempty"`
	// MinScore is the minimum relevance score threshold (0.0 - 1.0).
	MinScore float64 `json:"min_score,omitempty"`
	// Types restringe a busca por tipos de entidade.
	Types []string `json:"types,omitempty"`
}

// HybridOptions defines parameters for hybrid (vector + keyword) searches.
type HybridOptions struct {
	// Limit is the maximum number of results.
	Limit int `json:"limit,omitempty"`
	// Offset is the pagination offset.
	Offset int `json:"offset,omitempty"`
	// MinScore is the minimum relevance threshold.
	MinScore float64 `json:"min_score,omitempty"`
	// Types restringe a busca por tipos de entidade.
	Types []string `json:"types,omitempty"`
	// PathFilter restringe a busca a um caminho.
	PathFilter string `json:"path_filter,omitempty"`
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

// searchRequest é o corpo JSON de POST /v1/knowledge/search — espelha o
// handler.SearchRequest do servidor (api/rest/handler/knowledge.go). Não
// divergir: o servidor é a fonte de verdade do contrato.
type searchRequest struct {
	Query       string            `json:"query"`
	Limit       int               `json:"limit,omitempty"`
	Offset      int               `json:"offset,omitempty"`
	Types       []string          `json:"types,omitempty"`
	PathFilter  string            `json:"path_filter,omitempty"`
	MinScore    float64           `json:"min_score,omitempty"`
	EnableFTS   *bool             `json:"enable_fts,omitempty"`
	EnableVec   *bool             `json:"enable_vector,omitempty"`
	EnableGraph *bool             `json:"enable_graph,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
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
		Query:      query,
		Limit:      opts.Limit,
		Offset:     opts.Offset,
		MinScore:   opts.MinScore,
		Types:      opts.Types,
		PathFilter: opts.PathFilter,
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
		Types: []string{entityType},
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
//
// O servidor atual não expõe /v1/knowledge/rebuild — a operação de
// reconstrução é feita por /v1/knowledge/sync (rescan + reindex) ou
// /v1/knowledge/index (documento a documento). Este método delega ao Sync
// para não prometer um endpoint inexistente.
func (s *KnowledgeSDK) Rebuild() error {
	_, err := s.Sync()
	return err
}

// SearchFTS performs a full-text search with the given query. It uses
// the FTS engine exclusively, bypassing vector similarity.
func (s *KnowledgeSDK) SearchFTS(query string, opts FTSOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	body := searchRequest{
		Query:    query,
		Limit:    opts.Limit,
		Offset:   opts.Offset,
		MinScore: opts.MinScore,
		Types:    opts.Types,
		EnableFTS: boolPtr(true),
		EnableVec: boolPtr(false),
	}

	return s.search(body)
}

// SearchVector performs a vector-only similarity search. The query is
// embedded and matched against the vector index.
func (s *KnowledgeSDK) SearchVector(query string, opts VectorOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	body := searchRequest{
		Query:    query,
		Limit:    opts.Limit,
		Offset:   opts.Offset,
		MinScore: opts.MinScore,
		Types:    opts.Types,
		EnableFTS: boolPtr(false),
		EnableVec: boolPtr(true),
	}

	return s.search(body)
}

// HybridSearch performs a hybrid search combining vector similarity and
// keyword matching. O servidor decide a fusão (meaning-first no espaço
// roteado); ambos os índices ficam habilitados.
func (s *KnowledgeSDK) HybridSearch(query string, opts HybridOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	body := searchRequest{
		Query:     query,
		Limit:     opts.Limit,
		Offset:    opts.Offset,
		MinScore:  opts.MinScore,
		Types:     opts.Types,
		PathFilter: opts.PathFilter,
		EnableFTS:  boolPtr(true),
		EnableVec:  boolPtr(true),
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
